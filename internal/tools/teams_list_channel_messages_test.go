// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains the handler tests for teams list_channel_messages, and the
// canned channel-message bodies the domain's other channel tests share. A
// channel is addressed by two identifiers rather than one, so each refusal is
// graded separately: a caller told only that "an identifier is missing" cannot
// tell which of the two to go and find.
//
// @agents-index: Handler tests for teams.list_channel_messages covering
// per-identifier refusal, the requested path, output tiering, and the text
// rendering, plus the shared channel-message fixtures.
package tools

import (
	"net/http"
	"strings"
	"testing"
)

// teamsChannelMessagesJSON is a canned channel listing carrying two top-level
// posts, each with the channelIdentity the channel serializers key on.
const teamsChannelMessagesJSON = `{
	"value": [
		{
			"id": "post-1",
			"subject": "Release checklist",
			"messageType": "message",
			"createdDateTime": "2026-01-02T03:04:05Z",
			"from": {"user": {"displayName": "Alex Stone"}},
			"body": {"contentType": "text", "content": "the checklist is ready for review"},
			"channelIdentity": {"teamId": "team-1", "channelId": "19:channel-1"}
		},
		{
			"id": "post-2",
			"messageType": "message",
			"createdDateTime": "2026-01-03T09:00:00Z",
			"from": {"user": {"displayName": "Robin Vale"}},
			"body": {"contentType": "text", "content": "moving the review to Thursday"},
			"channelIdentity": {"teamId": "team-1", "channelId": "19:channel-1"}
		}
	]
}`

// newTeamsChannelMessagesRecorder returns a recorder answering with the canned
// channel listing.
func newTeamsChannelMessagesRecorder() *teamsRecorder {
	return &teamsRecorder{response: teamsChannelMessagesJSON}
}

// TestListChannelMessages_RefusesEachIdentifierSeparately validates that a
// caller missing one of the two channel identifiers is told which one, and that
// neither refusal costs a Graph call.
func TestListChannelMessages_RefusesEachIdentifierSeparately(t *testing.T) {
	for _, tc := range []struct {
		name string
		args map[string]any
		want string
	}{
		{"no team", map[string]any{"channel_id": "19:channel-1"}, "supply team_id"},
		{"no channel", map[string]any{"team_id": "team-1"}, "supply channel_id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := newTeamsChannelMessagesRecorder()
			result := runTeamsHandler(t, recorder, NewHandleListChannelMessages, tc.args)

			if !result.IsError {
				t.Fatalf("expected refusal, got %q", resultText(t, result))
			}
			if got := recorder.callCount(); got != 0 {
				t.Errorf("graph request count = %d, want 0", got)
			}
			if !strings.Contains(resultText(t, result), tc.want) {
				t.Errorf("refusal = %q, want it to name %q", resultText(t, result), tc.want)
			}
		})
	}
}

// TestListChannelMessages_ReadsTheNamedChannel validates that exactly one
// request is issued and that it addresses the channel the caller named.
func TestListChannelMessages_ReadsTheNamedChannel(t *testing.T) {
	recorder := newTeamsChannelMessagesRecorder()
	result := runTeamsHandler(t, recorder, NewHandleListChannelMessages, map[string]any{
		"team_id":    "team-1",
		"channel_id": "19:channel-1",
	})

	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	if got := recorder.callCount(); got != 1 {
		t.Fatalf("graph request count = %d, want 1", got)
	}
	if !strings.HasSuffix(recorder.paths[0], "/teams/team-1/channels/19:channel-1/messages") {
		t.Errorf("path = %q, want the named channel's messages", recorder.paths[0])
	}
	if recorder.methods[0] != http.MethodGet {
		t.Errorf("method = %q, want GET", recorder.methods[0])
	}
}

// TestListChannelMessages_SummaryCarriesChannelIdentifiers validates that the
// scanning tier carries the identifiers a follow-up read of one post takes,
// which a channel message supplies through channelIdentity rather than a chatId.
func TestListChannelMessages_SummaryCarriesChannelIdentifiers(t *testing.T) {
	records := decodeTeamsRecords(t, runTeamsHandler(t, newTeamsChannelMessagesRecorder(), NewHandleListChannelMessages, map[string]any{
		"team_id":    "team-1",
		"channel_id": "19:channel-1",
		"output":     "summary",
	}))

	if len(records) != 2 {
		t.Fatalf("record count = %d, want 2", len(records))
	}
	for _, want := range []struct{ key, value string }{
		{"id", "post-1"},
		{"teamId", "team-1"},
		{"channelId", "19:channel-1"},
		{"subject", "Release checklist"},
		{"from", "Alex Stone"},
	} {
		if got, _ := records[0][want.key].(string); got != want.value {
			t.Errorf("%s = %q, want %q", want.key, got, want.value)
		}
	}
	if _, present := records[0]["body"]; present {
		t.Error("summary tier carries the full body, want the preview only")
	}
}

// TestListChannelMessages_RawCarriesTheWholeBody validates that the raw tier
// returns what the summary tier only previews.
func TestListChannelMessages_RawCarriesTheWholeBody(t *testing.T) {
	records := decodeTeamsRecords(t, runTeamsHandler(t, newTeamsChannelMessagesRecorder(), NewHandleListChannelMessages, map[string]any{
		"team_id":    "team-1",
		"channel_id": "19:channel-1",
		"output":     "raw",
	}))

	if len(records) != 2 {
		t.Fatalf("record count = %d, want 2", len(records))
	}
	if got, _ := records[0]["body"].(string); got != "the checklist is ready for review" {
		t.Errorf("body = %q, want the whole post text", got)
	}
}

// TestListChannelMessages_TextTierListsPostsWithATotal validates the default
// rendering: a numbered listing naming each poster and closing with a count.
func TestListChannelMessages_TextTierListsPostsWithATotal(t *testing.T) {
	text := resultText(t, runTeamsHandler(t, newTeamsChannelMessagesRecorder(), NewHandleListChannelMessages, map[string]any{
		"team_id":    "team-1",
		"channel_id": "19:channel-1",
	}))

	for _, want := range []string{"1. Alex Stone - Release checklist", "2. Robin Vale", "Channel ID: 19:channel-1", "2 message(s) total."} {
		if !strings.Contains(text, want) {
			t.Errorf("text output missing %q:\n%s", want, text)
		}
	}
}

// TestListChannelMessages_GraphFailureCarriesFix validates that a refused read
// reaches the caller with the correction naming what to check.
func TestListChannelMessages_GraphFailureCarriesFix(t *testing.T) {
	recorder := &teamsRecorder{response: teamsChannelMessagesJSON, status: http.StatusForbidden}
	result := runTeamsHandler(t, recorder, NewHandleListChannelMessages, map[string]any{
		"team_id":    "team-1",
		"channel_id": "19:channel-1",
	})

	if !result.IsError {
		t.Fatalf("expected failure, got %q", resultText(t, result))
	}
	if !strings.Contains(resultText(t, result), "ChannelMessage.Read.All") {
		t.Errorf("failure carries no correction naming the scope: %q", resultText(t, result))
	}
}

// TestListChannelMessages_SendsTopOnly validates the recorded query: the page
// is bounded and no ordering is sent, since the collection documents none.
func TestListChannelMessages_SendsTopOnly(t *testing.T) {
	_, q, _ := runTeamsQueryHandler(t, teamsChannelMessagesJSON, NewHandleListChannelMessages, map[string]any{"team_id": "t1", "channel_id": "c1", "max_results": float64(99)})
	if got := q.Get("$top"); got != "50" {
		t.Errorf("$top = %q, want the clamp to 50", got)
	}
	if q.Has("$orderby") {
		t.Errorf("$orderby sent: %q", q.Get("$orderby"))
	}
}

// TestListChannelMessages_MarksTruncatedPage validates that a next link stops
// the text tier from presenting the page as the channel total.
func TestListChannelMessages_MarksTruncatedPage(t *testing.T) {
	result, _, _ := runTeamsQueryHandler(t, withNextLink(teamsChannelMessagesJSON), NewHandleListChannelMessages, map[string]any{"team_id": "t1", "channel_id": "c1", "output": "text"})
	if got := resultText(t, result); strings.Contains(got, "total.") || !strings.Contains(got, "truncated: true") {
		t.Errorf("text = %q", got)
	}
}
