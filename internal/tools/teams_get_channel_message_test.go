// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains the handler tests for teams get_channel_message: each of
// the three identifiers refused separately before any request, and the body
// escalation, which is the property the verb's description promises and the one
// a caller spends a raw fetch on.
//
// @agents-index: Handler tests for teams.get_channel_message covering
// per-identifier refusal, the requested path, and the preview-to-full-body
// escalation.
package tools

import (
	"net/http"
	"strings"
	"testing"
)

// teamsSingleChannelMessageJSON is a canned single channel post whose body
// exceeds the preview bound, so a preview is distinguishable from a full body by
// length rather than by exact text.
var teamsSingleChannelMessageJSON = `{
	"id": "post-1",
	"subject": "Release checklist",
	"messageType": "message",
	"createdDateTime": "2026-01-02T03:04:05Z",
	"from": {"user": {"displayName": "Alex Stone"}},
	"body": {"contentType": "text", "content": "` + teamsLongBody + `"},
	"channelIdentity": {"teamId": "team-1", "channelId": "19:channel-1"}
}`

// newTeamsChannelMessageRecorder returns a recorder answering with the canned
// single channel post.
func newTeamsChannelMessageRecorder() *teamsRecorder {
	return &teamsRecorder{response: teamsSingleChannelMessageJSON}
}

// TestGetChannelMessage_RefusesEachIdentifierSeparately validates that a caller
// missing one of the three identifiers is told which one, and that no refusal
// costs a Graph call.
func TestGetChannelMessage_RefusesEachIdentifierSeparately(t *testing.T) {
	for _, tc := range []struct {
		name string
		args map[string]any
		want string
	}{
		{"no team", map[string]any{"channel_id": "19:channel-1", "message_id": "post-1"}, "supply team_id"},
		{"no channel", map[string]any{"team_id": "team-1", "message_id": "post-1"}, "supply channel_id"},
		{"no message", map[string]any{"team_id": "team-1", "channel_id": "19:channel-1"}, "supply message_id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := newTeamsChannelMessageRecorder()
			result := runTeamsHandler(t, recorder, NewHandleGetChannelMessage, tc.args)

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

// TestGetChannelMessage_ReadsTheNamedPost validates that the request addresses
// the post the caller named, once.
func TestGetChannelMessage_ReadsTheNamedPost(t *testing.T) {
	recorder := newTeamsChannelMessageRecorder()
	result := runTeamsHandler(t, recorder, NewHandleGetChannelMessage, map[string]any{
		"team_id":    "team-1",
		"channel_id": "19:channel-1",
		"message_id": "post-1",
	})

	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	if got := recorder.callCount(); got != 1 {
		t.Fatalf("graph request count = %d, want 1", got)
	}
	if !strings.HasSuffix(recorder.paths[0], "/teams/team-1/channels/19:channel-1/messages/post-1") {
		t.Errorf("path = %q, want the named post", recorder.paths[0])
	}
}

// TestGetChannelMessage_DefaultPreviewsAndRawEscalates validates the body
// escalation: the default tier truncates and says so, and the raw tier returns
// the whole post, both from the one request Graph serves either way.
func TestGetChannelMessage_DefaultPreviewsAndRawEscalates(t *testing.T) {
	args := map[string]any{
		"team_id":    "team-1",
		"channel_id": "19:channel-1",
		"message_id": "post-1",
		"output":     "summary",
	}
	summary := decodeTeamsRecord(t, runTeamsHandler(t, newTeamsChannelMessageRecorder(), NewHandleGetChannelMessage, args))

	preview, _ := summary["bodyPreview"].(string)
	if !strings.Contains(preview, "output=raw") {
		t.Errorf("preview does not name the escalation: %q", preview)
	}
	if len([]rune(preview)) >= len([]rune(teamsLongBody)) {
		t.Errorf("preview length = %d runes, want it shorter than the body", len([]rune(preview)))
	}

	args["output"] = "raw"
	raw := decodeTeamsRecord(t, runTeamsHandler(t, newTeamsChannelMessageRecorder(), NewHandleGetChannelMessage, args))
	if raw["body"] != teamsLongBody {
		t.Error("raw tier did not return the whole body")
	}
	if raw["teamId"] != "team-1" || raw["channelId"] != "19:channel-1" {
		t.Error("raw tier dropped the identifiers a reply read is addressed by")
	}
}

// TestGetChannelMessage_TextTierStatesSenderAndIdentifiers validates that the
// default rendering names who posted and the identifiers a follow-up read of its
// replies takes.
func TestGetChannelMessage_TextTierStatesSenderAndIdentifiers(t *testing.T) {
	text := resultText(t, runTeamsHandler(t, newTeamsChannelMessageRecorder(), NewHandleGetChannelMessage, map[string]any{
		"team_id":    "team-1",
		"channel_id": "19:channel-1",
		"message_id": "post-1",
	}))

	for _, want := range []string{"From: Alex Stone", "Subject: Release checklist", "Message ID: post-1", "Team ID: team-1", "Channel ID: 19:channel-1"} {
		if !strings.Contains(text, want) {
			t.Errorf("text output missing %q:\n%s", want, text)
		}
	}
}

// TestGetChannelMessage_GraphFailureCarriesFix validates that a refused read
// reaches the caller with the correction naming what to check.
func TestGetChannelMessage_GraphFailureCarriesFix(t *testing.T) {
	recorder := &teamsRecorder{response: teamsSingleChannelMessageJSON, status: http.StatusNotFound}
	result := runTeamsHandler(t, recorder, NewHandleGetChannelMessage, map[string]any{
		"team_id":    "team-1",
		"channel_id": "19:channel-1",
		"message_id": "post-1",
	})

	if !result.IsError {
		t.Fatalf("expected failure, got %q", resultText(t, result))
	}
	if !strings.Contains(resultText(t, result), "team_id, channel_id, and message_id") {
		t.Errorf("failure carries no correction: %q", resultText(t, result))
	}
}
