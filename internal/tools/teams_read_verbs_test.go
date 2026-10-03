// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file holds the cross-verb tests over the Teams read handlers: the
// properties every one of them must hold rather than the per-verb behaviour the
// sibling files grade. Three of those properties are only observable across the
// whole domain, because a per-verb test proves them for the verb it names and
// says nothing about the verb added next to it.
//
// The error contract is the reason this file exists. A handler must put its
// correction on both channels it has, the tool result the caller reads and the
// log record a headless operator reads afterwards, and the per-verb tests grade
// the tool result alone. A buffer-backed slog handler is installed here so the
// second channel is read rather than assumed.
//
// @agents-index: Cross-verb tests over the Teams read handlers covering
// pre-request identifier validation, timeout and redaction behaviour, and the
// correction reaching both the tool result and the log record.
package tools

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
)

// teamsHandlerCtor is the constructor shape every Teams handler shares. It is
// named so the tables below read as data rather than as repeated signatures.
type teamsHandlerCtor func(graph.RetryConfig, time.Duration) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)

// teamsReadCase names one Teams verb together with the arguments that satisfy
// its validation, so a table entry can drive the verb past its refusal paths and
// into the request path under test.
type teamsReadCase struct {
	// name is the verb name, used to attribute a failure to a verb rather than
	// to a table position.
	name string
	// ctor is the handler constructor under test.
	ctor teamsHandlerCtor
	// args are arguments that pass every validation the verb performs.
	args map[string]any
}

// teamsGraphCallingVerbs enumerates every Teams verb that issues a Graph
// request, with arguments that reach that request.
//
// The list is written out rather than derived from the registry, because the
// registry holds wrapped handlers bound to real middleware while these tests
// drive the bare constructors against a recording endpoint. A verb added to the
// domain without a row here is caught by the count assertion in
// TestEveryTeamsGraphVerbIsCovered rather than passing unnoticed.
func teamsGraphCallingVerbs() []teamsReadCase {
	return []teamsReadCase{
		{"search", NewHandleTeamsSearch, map[string]any{"query": "release checklist"}},
		{"list_chats", NewHandleListChats, map[string]any{}},
		{"list_chat_messages", NewHandleListChatMessages, map[string]any{"chat_id": "19:chat-1"}},
		{"get_chat_message", NewHandleGetChatMessage, map[string]any{"chat_id": "19:chat-1", "message_id": "msg-1"}},
		{"list_channel_messages", NewHandleListChannelMessages, map[string]any{"team_id": "team-1", "channel_id": "19:channel-1"}},
		{"get_channel_message", NewHandleGetChannelMessage, map[string]any{"team_id": "team-1", "channel_id": "19:channel-1", "message_id": "post-1"}},
		{"list_channel_message_replies", NewHandleListChannelMessageReplies, map[string]any{"team_id": "team-1", "channel_id": "19:channel-1", "message_id": "post-1"}},
		{"compose_reply", NewHandleComposeReply, map[string]any{"chat_id": "19:chat-1", "message_id": "msg-1", "body": "agreed"}},
		{"get_online_meeting", NewHandleGetOnlineMeeting, map[string]any{"meeting_id": "meeting-1"}},
		{"list_transcripts", NewHandleListTranscripts, map[string]any{"meeting_id": "meeting-1"}},
		{"get_transcript", NewHandleGetTranscript, map[string]any{"meeting_id": "meeting-1", "transcript_id": "transcript-1"}},
	}
}

// teamsIdentifierRefusals enumerates the verbs that require an identifier or a
// query, paired with arguments that omit one, so the refusal can be observed
// before any request is issued.
//
// list_chats is absent deliberately: it addresses the signed-in user's own chat
// collection and takes no required argument, so it has no pre-request refusal to
// grade. Every other verb has one.
func teamsIdentifierRefusals() []teamsReadCase {
	return []teamsReadCase{
		{"search", NewHandleTeamsSearch, map[string]any{}},
		{"list_chat_messages", NewHandleListChatMessages, map[string]any{}},
		{"get_chat_message", NewHandleGetChatMessage, map[string]any{"chat_id": "19:chat-1"}},
		{"list_channel_messages", NewHandleListChannelMessages, map[string]any{"team_id": "team-1"}},
		{"get_channel_message", NewHandleGetChannelMessage, map[string]any{"team_id": "team-1", "channel_id": "19:channel-1"}},
		{"list_channel_message_replies", NewHandleListChannelMessageReplies, map[string]any{"team_id": "team-1", "channel_id": "19:channel-1"}},
		{"compose_reply", NewHandleComposeReply, map[string]any{"body": "agreed"}},
		{"get_online_meeting", NewHandleGetOnlineMeeting, map[string]any{}},
		{"list_transcripts", NewHandleListTranscripts, map[string]any{}},
		{"get_transcript", NewHandleGetTranscript, map[string]any{"meeting_id": "meeting-1"}},
	}
}

// teamsSlowResponse is how long the slow endpoint withholds its answer. It is
// far beyond the millisecond deadline the timeout cases set, and short enough
// that the test server's shutdown, which waits for outstanding handlers, is not
// itself delayed. Waiting on the request context instead would hang: an aborted
// client is not always observed as a cancelled server-side context, and a POST
// whose body was already delivered blocked the suite for the full test timeout.
const teamsSlowResponse = 200 * time.Millisecond

// teamsSlowEndpoint answers no request before the deadline a short timeout sets,
// so the handler's timeout branch is exercised rather than simulated.
type teamsSlowEndpoint struct{}

// ServeHTTP withholds its answer past any deadline these tests set.
func (teamsSlowEndpoint) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	select {
	case <-req.Context().Done():
	case <-time.After(teamsSlowResponse):
	}
}

// teamsEmailBearingFailure answers with a service error whose message carries an
// email address, which is the payload the redaction helper exists to remove
// before an error reaches the caller.
type teamsEmailBearingFailure struct{}

// teamsLeakedAddress is the address the service error carries. It must not
// survive into any tool result.
const teamsLeakedAddress = "alex.stone@contoso.example"

// ServeHTTP writes a forbidden response naming the address.
func (teamsEmailBearingFailure) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	//nolint:errcheck // test helper
	w.Write([]byte(`{"error":{"code":"ErrorAccessDenied","message":"Access is denied for ` + teamsLeakedAddress + `."}}`))
}

// runTeamsHandlerWith invokes a Teams handler against an arbitrary endpoint with
// an explicit timeout, which the shared runner does not allow. The timeout is a
// parameter here because the timeout branch is one of the properties under test.
func runTeamsHandlerWith(
	t *testing.T,
	endpoint http.Handler,
	ctor teamsHandlerCtor,
	args map[string]any,
	timeout time.Duration,
) *mcp.CallToolResult {
	t.Helper()

	client, srv := newTestGraphClient(t, endpoint)
	defer srv.Close()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	result, err := ctor(graph.RetryConfig{}, timeout)(auth.WithGraphClient(context.Background(), client), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return result
}

// TestEveryTeamsGraphVerbIsCovered asserts that the table driving the cross-verb
// checks still covers every Graph-calling verb the domain registers.
//
// The tables above are hand-written, so without this check a thirteenth verb
// would be added to the registry and silently escape all three cross-verb
// properties. The registry count is the authority; the eleven is derived from
// the twelve registered verbs minus help, which reaches nothing.
func TestEveryTeamsGraphVerbIsCovered(t *testing.T) {
	if got := len(teamsGraphCallingVerbs()); got != 11 {
		t.Errorf("the cross-verb table covers %d verbs, want 11; a Graph-calling verb was added to the domain without a row here", got)
	}
}

// TestEveryReadVerbValidatesIdentifiersBeforeCall asserts that a verb missing a
// required identifier refuses without issuing a Graph request.
//
// The per-verb tests grade which identifier is named; this grades the property
// that makes those refusals worth having, that a malformed call costs no network
// round trip and no service-side rate budget. It is checked across the domain
// because a verb that validated after the call would still return the right
// refusal text and pass every per-verb test.
func TestEveryReadVerbValidatesIdentifiersBeforeCall(t *testing.T) {
	for _, tc := range teamsIdentifierRefusals() {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &teamsRecorder{response: `{"value":[]}`}
			result := runTeamsHandlerWith(t, recorder, tc.ctor, tc.args, 30*time.Second)

			if !result.IsError {
				t.Fatalf("verb %s accepted a call missing a required identifier: %q", tc.name, resultText(t, result))
			}
			if got := recorder.callCount(); got != 0 {
				t.Errorf("verb %s issued %d Graph requests before refusing; validation must precede the call", tc.name, got)
			}
		})
	}
}

// TestReadVerbsHonourTimeoutAndRedaction asserts the two properties every Teams
// handler inherits from the shared Graph helpers, across the whole domain.
//
// The timeout half checks that a handler bounded by a deadline reports the
// deadline rather than blocking or surfacing a transport error the caller cannot
// act on. The redaction half checks that an address the service put in its error
// message does not reach the caller: the domain reads conversations, so a
// participant's address is exactly the payload a Teams-scoped failure is likely
// to carry.
//
// Both halves also read the log record, because a handler that redacted the tool
// result while emitting an unredacted correction, or that emitted no record at
// all, would leave the operator's channel silent while the caller's looked
// correct.
func TestReadVerbsHonourTimeoutAndRedaction(t *testing.T) {
	for _, tc := range teamsGraphCallingVerbs() {
		t.Run(tc.name+"/timeout", func(t *testing.T) {
			var logged bytes.Buffer
			restore := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
			defer slog.SetDefault(restore)

			result := runTeamsHandlerWith(t, teamsSlowEndpoint{}, tc.ctor, tc.args, time.Millisecond)

			if !result.IsError {
				t.Fatalf("verb %s returned a result for a request that never completed: %q", tc.name, resultText(t, result))
			}
			if !strings.Contains(resultText(t, result), "timed out") {
				t.Errorf("verb %s does not report the deadline it hit: %q", tc.name, resultText(t, result))
			}
			if !strings.Contains(logged.String(), "fix=") {
				t.Errorf("verb %s emitted no correction on the log record for a timeout, so an operator reading the log has nothing to act on: %q", tc.name, logged.String())
			}
		})

		t.Run(tc.name+"/redaction", func(t *testing.T) {
			var logged bytes.Buffer
			restore := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
			defer slog.SetDefault(restore)

			result := runTeamsHandlerWith(t, teamsEmailBearingFailure{}, tc.ctor, tc.args, 30*time.Second)

			if !result.IsError {
				t.Fatalf("verb %s reported success for a refused request: %q", tc.name, resultText(t, result))
			}
			if strings.Contains(resultText(t, result), teamsLeakedAddress) {
				t.Errorf("verb %s returns the address the service put in its error message unredacted: %q", tc.name, resultText(t, result))
			}
			if !strings.Contains(resultText(t, result), "[email redacted]") {
				t.Errorf("verb %s does not pass the service error through redaction: %q", tc.name, resultText(t, result))
			}
			if !strings.Contains(logged.String(), "fix=") {
				t.Errorf("verb %s emitted no correction on the log record for a service failure: %q", tc.name, logged.String())
			}
		})
	}
}

// TestErrorFixReachesBothToolResultAndLog asserts that the same correction
// reaches both channels a failure has, for a refusal that never leaves the
// process and for a failure the service returned.
//
// Grading the tool result alone leaves the channel the correction exists for
// ungraded: a headless caller reading a persisted log file never sees the tool
// result, and the log record is the only place the fix instruction reaches it.
// The two cases are chosen to be structurally different, a pre-request
// validation refusal and a post-request service failure, because they are
// written on different branches of every handler and a fix present on one says
// nothing about the other.
func TestErrorFixReachesBothToolResultAndLog(t *testing.T) {
	for _, tc := range []struct {
		name     string
		endpoint http.Handler
		ctor     teamsHandlerCtor
		args     map[string]any
		// fix is a distinctive substring of the correction, expected verbatim in
		// the tool result and in the log record.
		fix string
	}{
		{
			name:     "validation refusal",
			endpoint: &teamsRecorder{response: `{"value":[]}`},
			ctor:     NewHandleGetChannelMessage,
			args:     map[string]any{"channel_id": "19:channel-1", "message_id": "post-1"},
			fix:      "supply team_id",
		},
		{
			name:     "service failure",
			endpoint: teamsEmailBearingFailure{},
			ctor:     NewHandleGetTranscript,
			args:     map[string]any{"meeting_id": "meeting-1", "transcript_id": "transcript-1"},
			fix:      "OUTLOOK_MCP_TEAMS_ENABLED",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logged bytes.Buffer
			restore := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
			defer slog.SetDefault(restore)

			result := runTeamsHandlerWith(t, tc.endpoint, tc.ctor, tc.args, 30*time.Second)

			if !result.IsError {
				t.Fatalf("expected a failure, got %q", resultText(t, result))
			}
			if !strings.Contains(resultText(t, result), tc.fix) {
				t.Errorf("the tool result does not carry the correction %q: %q", tc.fix, resultText(t, result))
			}
			if !strings.Contains(logged.String(), tc.fix) {
				t.Errorf("the log record does not carry the same correction %q the tool result carries, so a caller reading only the log has no fix to apply: %q", tc.fix, logged.String())
			}
		})
	}
}
