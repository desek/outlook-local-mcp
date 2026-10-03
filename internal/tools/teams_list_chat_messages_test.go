// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains the handler tests for teams list_chat_messages: the
// identifier refusal that must happen before any request, the path the request
// takes, and the three output tiers.
//
// @agents-index: Handler tests for teams.list_chat_messages covering pre-flight
// identifier validation, request path, and output tiering.
package tools

import (
	"net/http"
	"strings"
	"testing"
)

// teamsChatMessagesResponseJSON is a canned chat-message collection carrying two
// messages from different senders.
const teamsChatMessagesResponseJSON = `{
	"value": [
		{
			"id": "msg-1",
			"chatId": "19:chat-1",
			"messageType": "message",
			"createdDateTime": "2026-01-02T03:04:05Z",
			"from": {"user": {"displayName": "Alex Stone"}},
			"body": {"contentType": "text", "content": "the release checklist is ready"}
		},
		{
			"id": "msg-2",
			"chatId": "19:chat-1",
			"messageType": "message",
			"createdDateTime": "2026-01-02T04:04:05Z",
			"from": {"user": {"displayName": "Bo Lee"}},
			"body": {"contentType": "text", "content": "reviewing it now"},
			"attachments": [{"id": "att-1", "name": "checklist.md", "contentType": "text/markdown"}]
		}
	]
}`

// newTeamsChatMessagesRecorder returns a recorder answering with the canned
// chat messages.
func newTeamsChatMessagesRecorder() *teamsRecorder {
	return &teamsRecorder{response: teamsChatMessagesResponseJSON}
}

// TestListChatMessages_RejectsMissingChatIDBeforeAnyRequest validates that an
// absent chat_id is refused with the correction naming both ways one is
// obtained, and costs no Graph call.
func TestListChatMessages_RejectsMissingChatIDBeforeAnyRequest(t *testing.T) {
	recorder := newTeamsChatMessagesRecorder()
	result := runTeamsHandler(t, recorder, NewHandleListChatMessages, map[string]any{})

	if !result.IsError {
		t.Fatalf("expected refusal, got %q", resultText(t, result))
	}
	if got := recorder.callCount(); got != 0 {
		t.Errorf("graph request count = %d, want 0", got)
	}
	text := resultText(t, result)
	if !strings.Contains(text, "list_chats") || !strings.Contains(text, "search") {
		t.Errorf("refusal names no way to obtain a chat_id: %q", text)
	}
}

// TestListChatMessages_ReadsTheNamedChat validates that the request reaches the
// messages collection of the chat the caller named, once.
func TestListChatMessages_ReadsTheNamedChat(t *testing.T) {
	recorder := newTeamsChatMessagesRecorder()
	result := runTeamsHandler(t, recorder, NewHandleListChatMessages, map[string]any{"chat_id": "19:chat-1"})

	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	if got := recorder.callCount(); got != 1 {
		t.Fatalf("graph request count = %d, want 1", got)
	}
	if !strings.Contains(recorder.paths[0], "/chats/19:chat-1/messages") {
		t.Errorf("path = %q, want the named chat's messages", recorder.paths[0])
	}
}

// TestListChatMessages_SummaryPreviewsEachBody validates that the scanning tier
// carries a preview and the chat identifier, and withholds the full body, which
// is what makes reading a thread affordable.
func TestListChatMessages_SummaryPreviewsEachBody(t *testing.T) {
	messages := decodeTeamsRecords(t, runTeamsHandler(t, newTeamsChatMessagesRecorder(), NewHandleListChatMessages, map[string]any{
		"chat_id": "19:chat-1",
		"output":  "summary",
	}))

	if len(messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(messages))
	}
	if messages[0]["bodyPreview"] != "the release checklist is ready" {
		t.Errorf("preview = %v, want the message text", messages[0]["bodyPreview"])
	}
	if messages[0]["chatId"] != "19:chat-1" {
		t.Errorf("chatId = %v, want the chat identifier", messages[0]["chatId"])
	}
	if _, present := messages[0]["body"]; present {
		t.Error("summary tier carries a full body, which only the raw tier promises")
	}
	if messages[1]["attachmentCount"] != float64(1) {
		t.Errorf("attachmentCount = %v, want 1", messages[1]["attachmentCount"])
	}
}

// TestListChatMessages_RawTierCarriesFullBodyAndAttachments validates the
// escalation: the whole body plus the attachment records the summary only
// counted.
func TestListChatMessages_RawTierCarriesFullBodyAndAttachments(t *testing.T) {
	messages := decodeTeamsRecords(t, runTeamsHandler(t, newTeamsChatMessagesRecorder(), NewHandleListChatMessages, map[string]any{
		"chat_id": "19:chat-1",
		"output":  "raw",
	}))

	if messages[0]["body"] != "the release checklist is ready" {
		t.Errorf("raw body = %v, want the full message body", messages[0]["body"])
	}
	attachments, ok := messages[1]["attachments"].([]any)
	if !ok || len(attachments) != 1 {
		t.Fatalf("raw attachments = %v, want one attachment", messages[1]["attachments"])
	}
}

// TestListChatMessages_TextTierNumbersTheThread validates that the default tier
// renders a numbered listing closing with a total.
func TestListChatMessages_TextTierNumbersTheThread(t *testing.T) {
	text := resultText(t, runTeamsHandler(t, newTeamsChatMessagesRecorder(), NewHandleListChatMessages, map[string]any{
		"chat_id": "19:chat-1",
	}))

	for _, want := range []string{"1. Alex Stone", "2. Bo Lee", "Attachments: 1", "2 message(s) total."} {
		if !strings.Contains(text, want) {
			t.Errorf("text output missing %q:\n%s", want, text)
		}
	}
}

// TestListChatMessages_GraphFailureCarriesFix validates that a refused read
// reaches the caller with the correction naming both the identifier and the
// consent to check.
func TestListChatMessages_GraphFailureCarriesFix(t *testing.T) {
	recorder := &teamsRecorder{response: teamsChatMessagesResponseJSON, status: http.StatusForbidden}
	result := runTeamsHandler(t, recorder, NewHandleListChatMessages, map[string]any{"chat_id": "19:chat-1"})

	if !result.IsError {
		t.Fatalf("expected failure, got %q", resultText(t, result))
	}
	if !strings.Contains(resultText(t, result), "OUTLOOK_MCP_TEAMS_ENABLED") {
		t.Errorf("failure carries no correction: %q", resultText(t, result))
	}
}

// TestListChatMessages_RequestsOrderTopAndEnumPreference validates the recorded
// request: creation order instead of the reaction-sensitive default, a bounded
// page, and the Prefer header that makes Graph name system events.
func TestListChatMessages_RequestsOrderTopAndEnumPreference(t *testing.T) {
	_, q, prefer := runTeamsQueryHandler(t, teamsChatMessagesResponseJSON, NewHandleListChatMessages, map[string]any{"chat_id": "19:chat-group", "max_results": float64(5)})
	if got := q.Get("$orderby"); got != "createdDateTime desc" {
		t.Errorf("$orderby = %q", got)
	}
	if got := q.Get("$top"); got != "5" {
		t.Errorf("$top = %q, want 5", got)
	}
	if !strings.Contains(prefer, "include-unknown-enum-members") {
		t.Errorf("Prefer = %q, want include-unknown-enum-members", prefer)
	}
}

// TestListChatMessages_LabelsSystemEvents validates that a system event, which
// has no sender and a placeholder body, is labelled by its event.
func TestListChatMessages_LabelsSystemEvents(t *testing.T) {
	body := `{"value":[{"id":"e1","chatId":"19:c","messageType":"systemEventMessage","from":null,
		"body":{"contentType":"html","content":"<systemEventMessage/>"},
		"eventDetail":{"@odata.type":"#microsoft.graph.membersAddedEventMessageDetail"}}]}`
	result, _, _ := runTeamsQueryHandler(t, body, NewHandleListChatMessages, map[string]any{"chat_id": "19:c", "output": "summary"})
	records := decodeTeamsRecords(t, result)
	if records[0]["from"] != "(system event)" || records[0]["bodyPreview"] != "System event: membersAdded" || records[0]["eventType"] != "membersAdded" {
		t.Errorf("system event record = %v", records[0])
	}
}

// TestListChatMessages_MarksTruncatedPage validates the truncation note.
func TestListChatMessages_MarksTruncatedPage(t *testing.T) {
	result, _, _ := runTeamsQueryHandler(t, withNextLink(teamsChatMessagesResponseJSON), NewHandleListChatMessages, map[string]any{"chat_id": "19:chat-group", "output": "text"})
	if got := resultText(t, result); strings.Contains(got, "total.") || !strings.Contains(got, "truncated: true") {
		t.Errorf("text = %q", got)
	}
}
