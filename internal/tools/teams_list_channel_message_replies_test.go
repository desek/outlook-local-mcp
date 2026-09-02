// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains the handler tests for teams list_channel_message_replies.
// The parent identifier every reply carries is graded explicitly: it is what
// distinguishes a reply from the post it answers, and a caller merging threads
// from several channels has nothing else to sort by.
//
// @agents-index: Handler tests for teams.list_channel_message_replies covering
// per-identifier refusal, the requested path, the parent identifier on each
// reply, and the text rendering.
package tools

import (
	"net/http"
	"strings"
	"testing"
)

// teamsChannelRepliesJSON is a canned reply collection under one channel post,
// each reply naming the post it answers.
const teamsChannelRepliesJSON = `{
	"value": [
		{
			"id": "reply-1",
			"replyToId": "post-1",
			"messageType": "message",
			"createdDateTime": "2026-01-02T04:00:00Z",
			"from": {"user": {"displayName": "Robin Vale"}},
			"body": {"contentType": "text", "content": "checked, looks right to me"},
			"channelIdentity": {"teamId": "team-1", "channelId": "19:channel-1"}
		},
		{
			"id": "reply-2",
			"replyToId": "post-1",
			"messageType": "message",
			"createdDateTime": "2026-01-02T05:00:00Z",
			"from": {"user": {"displayName": "Alex Stone"}},
			"body": {"contentType": "text", "content": "shipping it then"},
			"channelIdentity": {"teamId": "team-1", "channelId": "19:channel-1"}
		}
	]
}`

// newTeamsChannelRepliesRecorder returns a recorder answering with the canned
// reply collection.
func newTeamsChannelRepliesRecorder() *teamsRecorder {
	return &teamsRecorder{response: teamsChannelRepliesJSON}
}

// channelRepliesArgs is the complete argument set, from which each refusal case
// removes exactly one entry.
var channelRepliesArgs = map[string]any{
	"team_id":    "team-1",
	"channel_id": "19:channel-1",
	"message_id": "post-1",
}

// TestListChannelMessageReplies_RefusesEachIdentifierSeparately validates that a
// caller missing one of the three identifiers is told which one, and that no
// refusal costs a Graph call.
func TestListChannelMessageReplies_RefusesEachIdentifierSeparately(t *testing.T) {
	for _, tc := range []struct {
		omit string
		want string
	}{
		{"team_id", "supply team_id"},
		{"channel_id", "supply channel_id"},
		{"message_id", "supply message_id"},
	} {
		t.Run(tc.omit, func(t *testing.T) {
			args := map[string]any{}
			for key, value := range channelRepliesArgs {
				if key != tc.omit {
					args[key] = value
				}
			}

			recorder := newTeamsChannelRepliesRecorder()
			result := runTeamsHandler(t, recorder, NewHandleListChannelMessageReplies, args)

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

// TestListChannelMessageReplies_ReadsTheNamedThread validates that exactly one
// request is issued and that it addresses the replies under the named post.
func TestListChannelMessageReplies_ReadsTheNamedThread(t *testing.T) {
	recorder := newTeamsChannelRepliesRecorder()
	result := runTeamsHandler(t, recorder, NewHandleListChannelMessageReplies, channelRepliesArgs)

	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	if got := recorder.callCount(); got != 1 {
		t.Fatalf("graph request count = %d, want 1", got)
	}
	if !strings.HasSuffix(recorder.paths[0], "/teams/team-1/channels/19:channel-1/messages/post-1/replies") {
		t.Errorf("path = %q, want the named post's replies", recorder.paths[0])
	}
}

// TestListChannelMessageReplies_EveryReplyNamesItsParent validates that the
// scanning tier carries replyToId, which is the field a caller merging threads
// sorts by and the one the reply serializer exists to preserve.
func TestListChannelMessageReplies_EveryReplyNamesItsParent(t *testing.T) {
	args := map[string]any{"output": "summary"}
	for key, value := range channelRepliesArgs {
		args[key] = value
	}

	records := decodeTeamsRecords(t, runTeamsHandler(t, newTeamsChannelRepliesRecorder(), NewHandleListChannelMessageReplies, args))
	if len(records) != 2 {
		t.Fatalf("record count = %d, want 2", len(records))
	}
	for i, record := range records {
		if got, _ := record["replyToId"].(string); got != "post-1" {
			t.Errorf("reply %d replyToId = %q, want post-1", i, got)
		}
	}
}

// TestListChannelMessageReplies_TextTierListsRepliesWithATotal validates the
// default rendering: a numbered listing naming each responder and closing with a
// count.
func TestListChannelMessageReplies_TextTierListsRepliesWithATotal(t *testing.T) {
	text := resultText(t, runTeamsHandler(t, newTeamsChannelRepliesRecorder(), NewHandleListChannelMessageReplies, channelRepliesArgs))

	for _, want := range []string{"1. Robin Vale", "2. Alex Stone", "Reply to: post-1", "2 message(s) total."} {
		if !strings.Contains(text, want) {
			t.Errorf("text output missing %q:\n%s", want, text)
		}
	}
}

// TestListChannelMessageReplies_GraphFailureCarriesFix validates that a refused
// read reaches the caller with the correction naming what to check.
func TestListChannelMessageReplies_GraphFailureCarriesFix(t *testing.T) {
	recorder := &teamsRecorder{response: teamsChannelRepliesJSON, status: http.StatusForbidden}
	result := runTeamsHandler(t, recorder, NewHandleListChannelMessageReplies, channelRepliesArgs)

	if !result.IsError {
		t.Fatalf("expected failure, got %q", resultText(t, result))
	}
	if !strings.Contains(resultText(t, result), "ChannelMessage.Read.All") {
		t.Errorf("failure carries no correction naming the scope: %q", resultText(t, result))
	}
}
