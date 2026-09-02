// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains the handler tests for teams list_chat_message_replies: the
// two identifiers refused before any request, the replies path, and the parent
// identifier every reply carries, which is what keeps replies from several
// threads distinguishable once a caller has merged them.
//
// @agents-index: Handler tests for teams.list_chat_message_replies covering
// pre-flight identifier validation, request path, and the parent identifier in
// every tier.
package tools

import (
	"net/http"
	"strings"
	"testing"
)

// teamsRepliesResponseJSON is a canned reply collection under one parent
// message.
const teamsRepliesResponseJSON = `{
	"value": [
		{
			"id": "reply-1",
			"replyToId": "msg-1",
			"chatId": "19:chat-1",
			"messageType": "message",
			"createdDateTime": "2026-01-02T05:04:05Z",
			"from": {"user": {"displayName": "Bo Lee"}},
			"body": {"contentType": "text", "content": "agreed"}
		},
		{
			"id": "reply-2",
			"replyToId": "msg-1",
			"chatId": "19:chat-1",
			"messageType": "message",
			"createdDateTime": "2026-01-02T06:04:05Z",
			"from": {"user": {"displayName": "Cam Diaz"}},
			"body": {"contentType": "text", "content": "shipping tomorrow"}
		}
	]
}`

// newTeamsRepliesRecorder returns a recorder answering with the canned replies.
func newTeamsRepliesRecorder() *teamsRecorder {
	return &teamsRecorder{response: teamsRepliesResponseJSON}
}

// TestListChatMessageReplies_RefusesEachIdentifierSeparately validates that a
// caller missing either identifier is told which one and pays no Graph call.
func TestListChatMessageReplies_RefusesEachIdentifierSeparately(t *testing.T) {
	for _, tc := range []struct {
		name string
		args map[string]any
		want string
	}{
		{"no chat", map[string]any{"message_id": "msg-1"}, "supply chat_id"},
		{"no message", map[string]any{"chat_id": "19:chat-1"}, "supply message_id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := newTeamsRepliesRecorder()
			result := runTeamsHandler(t, recorder, NewHandleListChatMessageReplies, tc.args)

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

// TestListChatMessageReplies_ReadsTheRepliesPath validates that the request
// reaches the parent message's replies collection, once.
func TestListChatMessageReplies_ReadsTheRepliesPath(t *testing.T) {
	recorder := newTeamsRepliesRecorder()
	result := runTeamsHandler(t, recorder, NewHandleListChatMessageReplies, map[string]any{
		"chat_id":    "19:chat-1",
		"message_id": "msg-1",
	})

	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	if got := recorder.callCount(); got != 1 {
		t.Fatalf("graph request count = %d, want 1", got)
	}
	if !strings.HasSuffix(recorder.paths[0], "/chats/19:chat-1/messages/msg-1/replies") {
		t.Errorf("path = %q, want the parent message's replies", recorder.paths[0])
	}
}

// TestListChatMessageReplies_EveryTierNamesTheParent validates that the parent
// identifier is present at the scanning tier as well as the raw one, since a
// caller holding replies from several threads has nothing else to sort them by.
func TestListChatMessageReplies_EveryTierNamesTheParent(t *testing.T) {
	for _, mode := range []string{"summary", "raw"} {
		t.Run(mode, func(t *testing.T) {
			replies := decodeTeamsRecords(t, runTeamsHandler(t, newTeamsRepliesRecorder(), NewHandleListChatMessageReplies, map[string]any{
				"chat_id":    "19:chat-1",
				"message_id": "msg-1",
				"output":     mode,
			}))

			if len(replies) != 2 {
				t.Fatalf("reply count = %d, want 2", len(replies))
			}
			for i, reply := range replies {
				if reply["replyToId"] != "msg-1" {
					t.Errorf("reply %d replyToId = %v, want the parent identifier", i, reply["replyToId"])
				}
				if reply["chatId"] != "19:chat-1" {
					t.Errorf("reply %d chatId = %v, want the chat identifier", i, reply["chatId"])
				}
			}
		})
	}
}

// TestListChatMessageReplies_TextTierNumbersTheReplies validates that the
// default tier renders a numbered listing closing with a total.
func TestListChatMessageReplies_TextTierNumbersTheReplies(t *testing.T) {
	text := resultText(t, runTeamsHandler(t, newTeamsRepliesRecorder(), NewHandleListChatMessageReplies, map[string]any{
		"chat_id":    "19:chat-1",
		"message_id": "msg-1",
	}))

	for _, want := range []string{"1. Bo Lee", "2. Cam Diaz", "Reply to: msg-1", "2 message(s) total."} {
		if !strings.Contains(text, want) {
			t.Errorf("text output missing %q:\n%s", want, text)
		}
	}
}

// TestListChatMessageReplies_GraphFailureCarriesFix validates that a refused
// read reaches the caller with the correction naming what to check.
func TestListChatMessageReplies_GraphFailureCarriesFix(t *testing.T) {
	recorder := &teamsRecorder{response: teamsRepliesResponseJSON, status: http.StatusForbidden}
	result := runTeamsHandler(t, recorder, NewHandleListChatMessageReplies, map[string]any{
		"chat_id":    "19:chat-1",
		"message_id": "msg-1",
	})

	if !result.IsError {
		t.Fatalf("expected failure, got %q", resultText(t, result))
	}
	if !strings.Contains(resultText(t, result), "OUTLOOK_MCP_TEAMS_ENABLED") {
		t.Errorf("failure carries no correction: %q", resultText(t, result))
	}
}
