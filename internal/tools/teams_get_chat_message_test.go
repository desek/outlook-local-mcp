// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains the handler tests for teams get_chat_message: each
// identifier refused separately before any request, and the body escalation,
// which is the property the verb's description promises and the one a caller
// spends a raw fetch on.
//
// @agents-index: Handler tests for teams.get_chat_message covering per-identifier
// refusal, request count, and the preview-to-full-body escalation.
package tools

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// teamsLongBody is a body longer than the preview bound, so a test can tell a
// preview from a full body by length rather than by exact text.
var teamsLongBody = strings.Repeat("release note ", 80)

// teamsSingleChatMessageJSON is a canned single chat message whose body exceeds
// the preview bound.
var teamsSingleChatMessageJSON = `{
	"id": "msg-1",
	"chatId": "19:chat-1",
	"messageType": "message",
	"createdDateTime": "2026-01-02T03:04:05Z",
	"lastEditedDateTime": "2026-01-02T03:10:00Z",
	"from": {"user": {"displayName": "Alex Stone"}},
	"body": {"contentType": "text", "content": "` + teamsLongBody + `"}
}`

// newTeamsChatMessageRecorder returns a recorder answering with the canned
// single message.
func newTeamsChatMessageRecorder() *teamsRecorder {
	return &teamsRecorder{response: teamsSingleChatMessageJSON}
}

// decodeTeamsRecord decodes a JSON tool result into the single record it carries.
func decodeTeamsRecord(t *testing.T, result *mcp.CallToolResult) map[string]any {
	t.Helper()
	var record map[string]any
	if err := json.Unmarshal([]byte(resultText(t, result)), &record); err != nil {
		t.Fatalf("result is not a JSON object: %v", err)
	}
	return record
}

// TestGetChatMessage_RefusesEachIdentifierSeparately validates that a caller
// missing one identifier is told which one, since the two are obtained from
// different listings, and that neither refusal costs a Graph call.
func TestGetChatMessage_RefusesEachIdentifierSeparately(t *testing.T) {
	for _, tc := range []struct {
		name string
		args map[string]any
		want string
	}{
		{"no chat", map[string]any{"message_id": "msg-1"}, "supply chat_id"},
		{"no message", map[string]any{"chat_id": "19:chat-1"}, "supply message_id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := newTeamsChatMessageRecorder()
			result := runTeamsHandler(t, recorder, NewHandleGetChatMessage, tc.args)

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

// TestGetChatMessage_ReadsTheNamedMessage validates that the request addresses
// the message the caller named, once.
func TestGetChatMessage_ReadsTheNamedMessage(t *testing.T) {
	recorder := newTeamsChatMessageRecorder()
	result := runTeamsHandler(t, recorder, NewHandleGetChatMessage, map[string]any{
		"chat_id":    "19:chat-1",
		"message_id": "msg-1",
	})

	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	if got := recorder.callCount(); got != 1 {
		t.Fatalf("graph request count = %d, want 1", got)
	}
	if !strings.Contains(recorder.paths[0], "/chats/19:chat-1/messages/msg-1") {
		t.Errorf("path = %q, want the named message", recorder.paths[0])
	}
}

// TestGetChatMessage_DefaultPreviewsAndRawEscalates validates the body
// escalation: the default tier truncates and says so, and the raw tier returns
// the whole body, both from the one request Graph serves either way.
func TestGetChatMessage_DefaultPreviewsAndRawEscalates(t *testing.T) {
	summary := decodeTeamsRecord(t, runTeamsHandler(t, newTeamsChatMessageRecorder(), NewHandleGetChatMessage, map[string]any{
		"chat_id":    "19:chat-1",
		"message_id": "msg-1",
		"output":     "summary",
	}))

	preview, _ := summary["bodyPreview"].(string)
	if !strings.Contains(preview, "output=raw") {
		t.Errorf("preview does not name the escalation: %q", preview)
	}
	if len([]rune(preview)) >= len([]rune(teamsLongBody)) {
		t.Errorf("preview length = %d runes, want it shorter than the body", len([]rune(preview)))
	}

	raw := decodeTeamsRecord(t, runTeamsHandler(t, newTeamsChatMessageRecorder(), NewHandleGetChatMessage, map[string]any{
		"chat_id":    "19:chat-1",
		"message_id": "msg-1",
		"output":     "raw",
	}))
	if raw["body"] != teamsLongBody {
		t.Error("raw tier did not return the whole body")
	}
}

// TestGetChatMessage_TextTierStatesSenderAndIdentifiers validates that the
// default rendering names who wrote the message and the identifiers a follow-up
// read of its replies takes.
func TestGetChatMessage_TextTierStatesSenderAndIdentifiers(t *testing.T) {
	text := resultText(t, runTeamsHandler(t, newTeamsChatMessageRecorder(), NewHandleGetChatMessage, map[string]any{
		"chat_id":    "19:chat-1",
		"message_id": "msg-1",
	}))

	for _, want := range []string{"From: Alex Stone", "Message ID: msg-1", "Chat ID: 19:chat-1", "Sent: 2026-01-02T03:04:05Z"} {
		if !strings.Contains(text, want) {
			t.Errorf("text output missing %q:\n%s", want, text)
		}
	}
}

// TestGetChatMessage_GraphFailureCarriesFix validates that a refused read
// reaches the caller with the correction naming what to check.
func TestGetChatMessage_GraphFailureCarriesFix(t *testing.T) {
	recorder := &teamsRecorder{response: teamsSingleChatMessageJSON, status: http.StatusNotFound}
	result := runTeamsHandler(t, recorder, NewHandleGetChatMessage, map[string]any{
		"chat_id":    "19:chat-1",
		"message_id": "msg-1",
	})

	if !result.IsError {
		t.Fatalf("expected failure, got %q", resultText(t, result))
	}
	if !strings.Contains(resultText(t, result), "chat_id and message_id") {
		t.Errorf("failure carries no correction: %q", resultText(t, result))
	}
}
