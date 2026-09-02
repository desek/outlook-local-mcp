// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains the handler tests for teams list_transcripts. The property
// worth grading beyond the request shape is what the listing does not carry: no
// tier of it inlines transcript text, because a meeting can hold several
// transcripts and each is the whole text of a call.
//
// @agents-index: Handler tests for teams.list_transcripts covering identifier
// refusal, the requested path, and the absence of transcript content at every
// output tier.
package tools

import (
	"strings"
	"testing"
)

// teamsTranscriptListJSON is a canned transcript collection for one meeting.
const teamsTranscriptListJSON = `{
	"value": [
		{
			"id": "transcript-1",
			"meetingId": "meeting-1",
			"callId": "call-1",
			"createdDateTime": "2026-01-02T10:05:00Z",
			"endDateTime": "2026-01-02T10:06:00Z",
			"transcriptContentUrl": "https://graph.microsoft.com/v1.0/transcript-1/content"
		},
		{
			"id": "transcript-2",
			"meetingId": "meeting-1",
			"createdDateTime": "2026-01-03T10:05:00Z"
		}
	]
}`

// newTeamsTranscriptListRecorder returns a recorder answering with the canned
// transcript collection.
func newTeamsTranscriptListRecorder() *teamsPathRecorder {
	return &teamsPathRecorder{respond: func(string) (string, string) {
		return teamsTranscriptListJSON, "application/json"
	}}
}

// TestListTranscripts_RequiresMeetingID validates that an unnamed meeting costs
// no Graph call and that the refusal names the verb that produces the id, which
// a caller holding a calendar event does not yet have.
func TestListTranscripts_RequiresMeetingID(t *testing.T) {
	recorder := newTeamsTranscriptListRecorder()
	result := runTeamsPathHandler(t, recorder, NewHandleListTranscripts, map[string]any{})

	if !result.IsError {
		t.Fatalf("expected refusal, got %q", resultText(t, result))
	}
	if got := recorder.callCount(); got != 0 {
		t.Errorf("graph request count = %d, want 0", got)
	}
	if !strings.Contains(resultText(t, result), "get_online_meeting") {
		t.Errorf("refusal = %q, want it to name the resolving operation", resultText(t, result))
	}
}

// TestListTranscripts_ReadsTheNamedMeeting validates that exactly one request is
// issued and that it addresses the transcripts of the meeting the caller named.
func TestListTranscripts_ReadsTheNamedMeeting(t *testing.T) {
	recorder := newTeamsTranscriptListRecorder()
	result := runTeamsPathHandler(t, recorder, NewHandleListTranscripts, map[string]any{
		"meeting_id": "meeting-1",
		"output":     "summary",
	})

	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	if got := recorder.callCount(); got != 1 {
		t.Fatalf("graph request count = %d, want 1", got)
	}
	if !strings.HasSuffix(recorder.paths[0], "/onlineMeetings/meeting-1/transcripts") {
		t.Errorf("path = %q, want the meeting's transcripts", recorder.paths[0])
	}

	records := decodeTeamsRecords(t, result)
	if len(records) != 2 {
		t.Fatalf("returned %d transcripts, want 2", len(records))
	}
	if records[0]["id"] != "transcript-1" {
		t.Errorf("first id = %v, want transcript-1", records[0]["id"])
	}
}

// TestListTranscripts_CarriesNoContentAtEitherTier validates the listing's
// bound: escalating a listing describes each record more fully, it does not
// inline the text of every transcript the meeting holds.
func TestListTranscripts_CarriesNoContentAtEitherTier(t *testing.T) {
	for _, mode := range []string{"summary", "raw"} {
		t.Run(mode, func(t *testing.T) {
			recorder := newTeamsTranscriptListRecorder()
			result := runTeamsPathHandler(t, recorder, NewHandleListTranscripts, map[string]any{
				"meeting_id": "meeting-1",
				"output":     mode,
			})

			if result.IsError {
				t.Fatalf("expected success, got %q", resultText(t, result))
			}
			for _, record := range decodeTeamsRecords(t, result) {
				for _, field := range []string{"content", "contentPreview"} {
					if _, present := record[field]; present {
						t.Errorf("listing carried %q, want transcript text only from the single-transcript read", field)
					}
				}
			}
		})
	}
}

// TestListTranscripts_TextModeStatesWhyAMeetingListsNone validates that an empty
// listing is an explanation rather than a bare zero, since a caller expecting a
// recap most often has a meeting that was simply never transcribed.
func TestListTranscripts_TextModeStatesWhyAMeetingListsNone(t *testing.T) {
	recorder := &teamsPathRecorder{respond: func(string) (string, string) {
		return `{"value": []}`, "application/json"
	}}
	result := runTeamsPathHandler(t, recorder, NewHandleListTranscripts, map[string]any{
		"meeting_id": "meeting-1",
	})

	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	if !strings.Contains(resultText(t, result), "transcribed") {
		t.Errorf("result = %q, want it to state why a meeting lists no transcripts", resultText(t, result))
	}
}
