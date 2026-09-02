// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains the handler tests for teams compose_reply. The load-bearing
// assertion is negative: every request the handler issued is graded, and any
// method other than GET fails the test, because the verb's name invites the
// reading that it communicates and nothing else in the running system would make
// a stray write visible before it reached a real conversation.
//
// @agents-index: Handler tests for teams.compose_reply covering the absence of
// any write request, the not-sent statement in the returned text, the quoted
// parent, and the parent-location refusals.
package tools

import (
	"net/http"
	"strings"
	"testing"
)

// composeReplyChatArgs answers a chat message, the shape addressed by a single
// chat identifier.
var composeReplyChatArgs = map[string]any{
	"chat_id":    "19:chat-1",
	"message_id": "msg-1",
	"body":       "agreed, I will take the checklist",
}

// composeReplyChannelArgs answers a channel post, the shape addressed by a team
// and a channel identifier.
var composeReplyChannelArgs = map[string]any{
	"team_id":    "team-1",
	"channel_id": "19:channel-1",
	"message_id": "post-1",
	"body":       "agreed, I will take the checklist",
}

// TestComposeReply_PostsNothing validates that preparing a reply issues one GET
// and nothing else: no POST, PATCH, PUT, or DELETE reaches the service in either
// the chat or the channel shape.
func TestComposeReply_PostsNothing(t *testing.T) {
	for name, args := range map[string]map[string]any{
		"chat":    composeReplyChatArgs,
		"channel": composeReplyChannelArgs,
	} {
		t.Run(name, func(t *testing.T) {
			recorder := &teamsRecorder{response: teamsSingleChatMessageJSON}
			result := runTeamsHandler(t, recorder, NewHandleComposeReply, args)

			if result.IsError {
				t.Fatalf("expected success, got %q", resultText(t, result))
			}
			if got := recorder.callCount(); got != 1 {
				t.Fatalf("graph request count = %d, want 1 read", got)
			}
			for i, method := range recorder.methods {
				if method != http.MethodGet {
					t.Errorf("request %d used %s %s, want GET only", i, method, recorder.paths[i])
				}
			}
			for i, body := range recorder.bodies {
				if body != "" {
					t.Errorf("request %d carried a body %q, want none", i, body)
				}
			}
		})
	}
}

// TestComposeReply_ReadsTheParentItQuotes validates that the one request the
// handler issues addresses the message the draft answers, in the collection the
// caller's identifiers name.
func TestComposeReply_ReadsTheParentItQuotes(t *testing.T) {
	for _, tc := range []struct {
		name string
		args map[string]any
		want string
	}{
		{"chat", composeReplyChatArgs, "/chats/19:chat-1/messages/msg-1"},
		{"channel", composeReplyChannelArgs, "/teams/team-1/channels/19:channel-1/messages/post-1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &teamsRecorder{response: teamsSingleChatMessageJSON}
			runTeamsHandler(t, recorder, NewHandleComposeReply, tc.args)

			if got := recorder.callCount(); got != 1 {
				t.Fatalf("graph request count = %d, want 1", got)
			}
			if !strings.HasSuffix(recorder.paths[0], tc.want) {
				t.Errorf("path = %q, want it to address %q", recorder.paths[0], tc.want)
			}
		})
	}
}

// TestComposeReply_StatesNotSent validates that the returned text says the reply
// was not sent and says what the user must do to send it, since the tool result
// is the only place a caller learns this.
func TestComposeReply_StatesNotSent(t *testing.T) {
	text := resultText(t, runTeamsHandler(t, &teamsRecorder{response: teamsSingleChatMessageJSON}, NewHandleComposeReply, composeReplyChatArgs))

	for _, want := range []string{"NOT SENT", "nothing was posted", "has not been sent", "paste the reply text"} {
		if !strings.Contains(text, want) {
			t.Errorf("prepared text missing %q:\n%s", want, text)
		}
	}
}

// TestComposeReply_QuotesTheParentAndReturnsTheBody validates that the draft
// carries both halves a person needs to judge it: what is being answered, quoted
// in full rather than previewed, and the reply text unchanged.
func TestComposeReply_QuotesTheParentAndReturnsTheBody(t *testing.T) {
	text := resultText(t, runTeamsHandler(t, &teamsRecorder{response: teamsSingleChatMessageJSON}, NewHandleComposeReply, composeReplyChatArgs))

	if !strings.Contains(text, "In reply to Alex Stone") {
		t.Errorf("prepared text does not name who is being answered:\n%s", text)
	}
	if !strings.Contains(text, "> "+strings.TrimSpace(teamsLongBody)) {
		t.Error("prepared text does not quote the whole parent body")
	}
	if !strings.Contains(text, "agreed, I will take the checklist") {
		t.Error("prepared text does not carry the reply body unchanged")
	}
}

// TestComposeReply_RefusesAnIncompleteRequest validates that every refusal
// happens before any request and names what is missing. A caller naming both a
// chat and a channel has named two different messages, which is refused rather
// than resolved by preferring one.
func TestComposeReply_RefusesAnIncompleteRequest(t *testing.T) {
	for _, tc := range []struct {
		name string
		args map[string]any
		want string
	}{
		{"no body", map[string]any{"chat_id": "19:chat-1", "message_id": "msg-1"}, "body must not be empty"},
		{"no message", map[string]any{"chat_id": "19:chat-1", "body": "text"}, "supply message_id"},
		{"no location", map[string]any{"message_id": "msg-1", "body": "text"}, "must be given"},
		{"partial channel", map[string]any{"team_id": "team-1", "message_id": "post-1", "body": "text"}, "channel_id must not be empty"},
		{"both locations", map[string]any{"chat_id": "19:chat-1", "team_id": "team-1", "channel_id": "19:channel-1", "message_id": "msg-1", "body": "text"}, "must not be combined"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &teamsRecorder{response: teamsSingleChatMessageJSON}
			result := runTeamsHandler(t, recorder, NewHandleComposeReply, tc.args)

			if !result.IsError {
				t.Fatalf("expected refusal, got %q", resultText(t, result))
			}
			if got := recorder.callCount(); got != 0 {
				t.Errorf("graph request count = %d, want 0", got)
			}
			if !strings.Contains(resultText(t, result), tc.want) {
				t.Errorf("refusal = %q, want it to state %q", resultText(t, result), tc.want)
			}
		})
	}
}

// TestComposeReply_GraphFailureCarriesFix validates that a parent that cannot be
// read reaches the caller with the correction naming what to check.
func TestComposeReply_GraphFailureCarriesFix(t *testing.T) {
	recorder := &teamsRecorder{response: teamsSingleChatMessageJSON, status: http.StatusNotFound}
	result := runTeamsHandler(t, recorder, NewHandleComposeReply, composeReplyChatArgs)

	if !result.IsError {
		t.Fatalf("expected failure, got %q", resultText(t, result))
	}
	if !strings.Contains(resultText(t, result), "OUTLOOK_MCP_TEAMS_ENABLED") {
		t.Errorf("failure carries no correction: %q", resultText(t, result))
	}
}
