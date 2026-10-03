// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains the handler tests for teams list_chats: the single request
// on the success path, the three output tiers, and the untitled one-to-one chat,
// which is the case the listing's field set was chosen for.
//
// @agents-index: Handler tests for teams.list_chats covering request count,
// output tiering, and the untitled chat's fallback label.
package tools

import (
	"net/http"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// teamsChatsResponseJSON is a canned chats response carrying a named group chat
// and an untitled one-to-one chat, the pair that exercises both listing labels.
const teamsChatsResponseJSON = `{
	"value": [
		{
			"id": "19:chat-group",
			"topic": "Release crew",
			"chatType": "group",
			"lastUpdatedDateTime": "2026-01-04T09:00:00Z",
			"webUrl": "https://teams.microsoft.com/l/chat/19:chat-group",
			"members": [{"id": "m1", "displayName": "Alex Stone", "roles": ["owner"]}]
		},
		{
			"id": "19:chat-oneOnOne",
			"chatType": "oneOnOne",
			"lastUpdatedDateTime": "2026-01-03T09:00:00Z",
			"lastMessagePreview": {"body": {"contentType": "text", "content": "sounds good"}}
		}
	]
}`

// newTeamsChatsRecorder returns a recorder answering with the canned chats.
func newTeamsChatsRecorder() *teamsRecorder {
	return &teamsRecorder{response: teamsChatsResponseJSON}
}

// TestListChats_IssuesOneRequest validates that the listing reaches the chats
// collection once and does not follow the collection's next link.
func TestListChats_IssuesOneRequest(t *testing.T) {
	recorder := newTeamsChatsRecorder()
	result := runTeamsHandler(t, recorder, NewHandleListChats, map[string]any{})

	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	if got := recorder.callCount(); got != 1 {
		t.Fatalf("graph request count = %d, want 1", got)
	}
	if !strings.HasSuffix(recorder.paths[0], "/chats") {
		t.Errorf("path = %q, want the chats collection", recorder.paths[0])
	}
}

// TestListChats_SummaryCarriesTheChoosingFields validates that the summary tier
// states what a caller choosing a conversation needs, including the last-message
// preview an untitled chat is otherwise indistinguishable without.
func TestListChats_SummaryCarriesTheChoosingFields(t *testing.T) {
	chats := decodeTeamsRecords(t, runTeamsHandler(t, newTeamsChatsRecorder(), NewHandleListChats, map[string]any{
		"output": "summary",
	}))

	if len(chats) != 2 {
		t.Fatalf("chat count = %d, want 2", len(chats))
	}
	if chats[0]["topic"] != "Release crew" || chats[0]["chatType"] != "group" {
		t.Errorf("first chat = %v, want the named group chat", chats[0])
	}
	if chats[1]["lastMessagePreview"] != "sounds good" {
		t.Errorf("untitled chat preview = %v, want the last message", chats[1]["lastMessagePreview"])
	}
	if _, present := chats[0]["members"]; present {
		t.Error("summary tier carries members, which only the raw tier promises")
	}
}

// TestListChats_RawTierCarriesMembership validates that the raw tier adds the
// membership that identifies a chat once its topic is empty.
func TestListChats_RawTierCarriesMembership(t *testing.T) {
	chats := decodeTeamsRecords(t, runTeamsHandler(t, newTeamsChatsRecorder(), NewHandleListChats, map[string]any{
		"output": "raw",
	}))

	members, ok := chats[0]["members"].([]any)
	if !ok || len(members) != 1 {
		t.Fatalf("raw chat members = %v, want one member", chats[0]["members"])
	}
	member, _ := members[0].(map[string]any)
	if member["displayName"] != "Alex Stone" {
		t.Errorf("member = %v, want the named member", member)
	}
}

// TestListChats_TextTierLabelsTheUntitledChat validates that a chat with no
// topic still renders as something a reader can pick, rather than as a bare
// identifier.
func TestListChats_TextTierLabelsTheUntitledChat(t *testing.T) {
	text := resultText(t, runTeamsHandler(t, newTeamsChatsRecorder(), NewHandleListChats, map[string]any{
		"output": "text",
	}))

	for _, want := range []string{"Release crew", "(untitled oneOnOne chat)", "Latest: sounds good", "2 chat(s) total."} {
		if !strings.Contains(text, want) {
			t.Errorf("text output missing %q:\n%s", want, text)
		}
	}
}

// TestListChats_GraphFailureCarriesFix validates that a refused listing reaches
// the caller with the correction naming the consent to check.
func TestListChats_GraphFailureCarriesFix(t *testing.T) {
	recorder := &teamsRecorder{response: teamsChatsResponseJSON, status: http.StatusForbidden}
	result := runTeamsHandler(t, recorder, NewHandleListChats, map[string]any{})

	if !result.IsError {
		t.Fatalf("expected failure, got %q", resultText(t, result))
	}
	if !strings.Contains(resultText(t, result), "Chat.Read") {
		t.Errorf("failure carries no correction: %q", resultText(t, result))
	}
}

// TestListChats_RequestsExpandOrderAndTop validates the recorded query: the
// preview is a relationship Graph omits unless expanded, the ordering is the
// only one the collection supports, and the page is bounded.
func TestListChats_RequestsExpandOrderAndTop(t *testing.T) {
	_, q, _ := runTeamsQueryHandler(t, teamsChatsResponseJSON, NewHandleListChats, map[string]any{"max_results": float64(10)})
	if got := q.Get("$expand"); got != "lastMessagePreview" {
		t.Errorf("$expand = %q, want lastMessagePreview without members", got)
	}
	if got := q.Get("$orderby"); got != "lastMessagePreview/createdDateTime desc" {
		t.Errorf("$orderby = %q", got)
	}
	if got := q.Get("$top"); got != "10" {
		t.Errorf("$top = %q, want 10", got)
	}
}

// TestListChats_RawExpandsMembers validates that only the raw tier, which
// returns members, pays for expanding them.
func TestListChats_RawExpandsMembers(t *testing.T) {
	_, q, _ := runTeamsQueryHandler(t, teamsChatsResponseJSON, NewHandleListChats, map[string]any{"output": "raw"})
	if got := q.Get("$expand"); got != "lastMessagePreview,members" {
		t.Errorf("$expand = %q, want lastMessagePreview,members", got)
	}
	if got := q.Get("$top"); got != "50" {
		t.Errorf("$top = %q, want the default 50", got)
	}
}

// TestListChats_MarksTruncatedPage validates that a next link turns into a
// truncation note in both the JSON and text tiers.
func TestListChats_MarksTruncatedPage(t *testing.T) {
	result, _, _ := runTeamsQueryHandler(t, withNextLink(teamsChatsResponseJSON), NewHandleListChats, map[string]any{"output": "summary"})
	if len(result.Content) != 2 || !strings.Contains(result.Content[1].(mcp.TextContent).Text, "truncated: true") {
		t.Errorf("summary result lacks the truncation note: %v", result.Content)
	}
	text, _, _ := runTeamsQueryHandler(t, withNextLink(teamsChatsResponseJSON), NewHandleListChats, map[string]any{"output": "text"})
	if got := resultText(t, text); strings.Contains(got, "total.") || !strings.Contains(got, "truncated: true") {
		t.Errorf("text tier claims a total on a truncated page: %q", got)
	}
}
