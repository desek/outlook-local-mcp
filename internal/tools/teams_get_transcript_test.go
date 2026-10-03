// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains the handler tests for teams get_transcript. The escalation
// is graded on both axes it has: the returned text grows under the raw tier, and
// the request count does not, because a callTranscript carries no service-side
// preview and the default preview is a truncation of content that had to be
// fetched either way.
//
// @agents-index: Handler tests for teams.get_transcript covering per-identifier
// refusal, the metadata-then-content request pair, and the preview-to-full-text
// escalation.
package tools

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
)

// teamsTranscriptText is a WEBVTT document longer than the preview bound, so a
// preview is distinguishable from the whole transcript by length.
var teamsTranscriptText = "WEBVTT\n\n" + strings.Repeat("00:00:01.000 --> 00:00:02.000\nAlex Stone: the release is ready\n\n", 20)

// teamsTranscriptMetadataJSON is the canned metadata for that transcript.
const teamsTranscriptMetadataJSON = `{
	"id": "transcript-1",
	"meetingId": "meeting-1",
	"callId": "call-1",
	"createdDateTime": "2026-01-02T10:05:00Z",
	"endDateTime": "2026-01-02T10:06:00Z"
}`

// newTeamsTranscriptRecorder returns a recorder answering the metadata read with
// the canned record and the content read with the WEBVTT text.
func newTeamsTranscriptRecorder() *teamsPathRecorder {
	return &teamsPathRecorder{respond: func(path string) (string, string) {
		if strings.HasSuffix(path, "/content") {
			return teamsTranscriptText, "text/vtt"
		}
		return teamsTranscriptMetadataJSON, "application/json"
	}}
}

// TestGetTranscript_RefusesEachIdentifierSeparately validates that a caller
// missing one of the two identifiers is told which one, and that no refusal
// costs a Graph call.
func TestGetTranscript_RefusesEachIdentifierSeparately(t *testing.T) {
	for _, tc := range []struct {
		name string
		args map[string]any
		want string
	}{
		{"no meeting", map[string]any{"transcript_id": "transcript-1"}, "supply meeting_id"},
		{"no transcript", map[string]any{"meeting_id": "meeting-1"}, "supply transcript_id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := newTeamsTranscriptRecorder()
			result := runTeamsPathHandler(t, recorder, NewHandleGetTranscript, tc.args)

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

// TestGetTranscript_ContentOnlyUnderRaw validates the verb's escalation: the
// default tier returns metadata and a truncated preview, the raw tier returns
// the whole WEBVTT document, and both pay the same two requests, the metadata
// read followed by the content read.
func TestGetTranscript_ContentOnlyUnderRaw(t *testing.T) {
	summaryRecorder := newTeamsTranscriptRecorder()
	summary := runTeamsPathHandler(t, summaryRecorder, NewHandleGetTranscript, map[string]any{
		"meeting_id":    "meeting-1",
		"transcript_id": "transcript-1",
		"output":        "summary",
	})
	if summary.IsError {
		t.Fatalf("expected success, got %q", resultText(t, summary))
	}

	rawRecorder := newTeamsTranscriptRecorder()
	raw := runTeamsPathHandler(t, rawRecorder, NewHandleGetTranscript, map[string]any{
		"meeting_id":    "meeting-1",
		"transcript_id": "transcript-1",
		"output":        "raw",
	})
	if raw.IsError {
		t.Fatalf("expected success, got %q", resultText(t, raw))
	}

	for name, recorder := range map[string]*teamsPathRecorder{"summary": summaryRecorder, "raw": rawRecorder} {
		if got := recorder.callCount(); got != 2 {
			t.Fatalf("%s graph request count = %d, want 2", name, got)
		}
		if !strings.HasSuffix(recorder.paths[0], "/transcripts/transcript-1") {
			t.Errorf("%s first path = %q, want the transcript metadata", name, recorder.paths[0])
		}
		if !strings.HasSuffix(recorder.paths[1], "/transcripts/transcript-1/content") {
			t.Errorf("%s second path = %q, want the transcript content", name, recorder.paths[1])
		}
	}

	summaryRecord := decodeTeamsRecord(t, summary)
	if _, present := summaryRecord["content"]; present {
		t.Error("summary tier carried the full content, want a preview only")
	}
	preview, _ := summaryRecord["contentPreview"].(string)
	if preview == "" {
		t.Fatal("summary tier carried no preview")
	}
	if truncated, _ := summaryRecord["contentTruncated"].(bool); !truncated {
		t.Error("contentTruncated = false, want true for a transcript longer than the preview bound")
	}

	rawRecord := decodeTeamsRecord(t, raw)
	content, _ := rawRecord["content"].(string)
	if content != teamsTranscriptText {
		t.Errorf("raw content is %d characters, want the whole %d-character transcript", len(content), len(teamsTranscriptText))
	}
	if len(preview) >= len(content) {
		t.Errorf("preview is %d characters and content %d, want the preview to be shorter", len(preview), len(content))
	}
}

// TestGetTranscript_TextModeMarksThePreview validates that a reader of the text
// tier is told the transcript was cut, since a WEBVTT document truncated
// mid-sentence otherwise reads as a complete record of a shorter meeting.
func TestGetTranscript_TextModeMarksThePreview(t *testing.T) {
	recorder := newTeamsTranscriptRecorder()
	result := runTeamsPathHandler(t, recorder, NewHandleGetTranscript, map[string]any{
		"meeting_id":    "meeting-1",
		"transcript_id": "transcript-1",
	})

	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	if !strings.Contains(resultText(t, result), "output=raw") {
		t.Errorf("result = %q, want it to name the escalation", resultText(t, result))
	}
}

// TestGetTranscript_ContentRefusalIsReportedWithItsFix validates that a
// transcript whose metadata is readable but whose content the service declines
// is reported as a failure carrying the correction, rather than as a transcript
// with empty text, which a caller would read as a meeting nobody spoke in.
func TestGetTranscript_ContentRefusalIsReportedWithItsFix(t *testing.T) {
	recorder := &teamsPathRecorder{
		respond:    func(string) (string, string) { return teamsTranscriptMetadataJSON, "application/json" },
		failSuffix: "/content",
	}
	result := runTeamsPathHandler(t, recorder, NewHandleGetTranscript, map[string]any{
		"meeting_id":    "meeting-1",
		"transcript_id": "transcript-1",
	})

	if !result.IsError {
		t.Fatalf("expected a failure, got %q", resultText(t, result))
	}
	if got := recorder.callCount(); got != 2 {
		t.Fatalf("graph request count = %d, want 2", got)
	}
	if !strings.Contains(resultText(t, result), "OnlineMeetingTranscript.Read.All") {
		t.Errorf("failure = %q, want it to name the consent to check", resultText(t, result))
	}
}

// transcriptInnerErrorServer serves the transcript endpoints and refuses chosen
// content formats with a 403 carrying an inner-error code, recording the Accept
// header of every content request so a test reads what was actually sent.
type transcriptInnerErrorServer struct {
	mu sync.Mutex
	// accepts holds the Accept header of every content request, in order.
	accepts []string
	// paths holds every request path, in order.
	paths []string
	// refuse maps an Accept value on a content request, or "list" for the
	// listing, to the inner-error code returned for it.
	refuse map[string]string
}

// ServeHTTP records the request and answers with a 403 or the canned body.
func (s *transcriptInnerErrorServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	s.mu.Lock()
	s.paths = append(s.paths, req.URL.Path)
	key := "list"
	if strings.HasSuffix(req.URL.Path, "/content") {
		key = req.Header.Get("Accept")
		s.accepts = append(s.accepts, key)
	}
	s.mu.Unlock()

	if strings.HasSuffix(req.URL.Path, "/transcripts/transcript-1") {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(teamsTranscriptMetadataJSON))
		return
	}
	if code, ok := s.refuse[key]; ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprintf(w, `{"error":{"code":"Forbidden","message":"denied","innerError":{"code":%q}}}`, code)
		return
	}
	if key == "list" {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(teamsTranscriptListJSON))
		return
	}
	w.Header().Set("Content-Type", key)
	_, _ = w.Write([]byte(teamsTranscriptText))
}

// runTranscriptServerHandler drives a handler against the inner-error server.
func runTranscriptServerHandler(
	t *testing.T,
	srvHandler http.Handler,
	newHandler func(graph.RetryConfig, time.Duration) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error),
	args map[string]any,
) *mcp.CallToolResult {
	t.Helper()
	client, srv := newTestGraphClient(t, srvHandler)
	defer srv.Close()
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	result, err := newHandler(graph.RetryConfig{}, 30*time.Second)(auth.WithGraphClient(context.Background(), client), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return result
}

// TestGetTranscript_RequestsWebVTT validates that the content read names the
// WEBVTT format in Accept, rather than the SDK's generic default, and that the
// result states the format it holds.
func TestGetTranscript_RequestsWebVTT(t *testing.T) {
	server := &transcriptInnerErrorServer{}
	result := runTranscriptServerHandler(t, server, NewHandleGetTranscript, map[string]any{
		"meeting_id": "meeting-1", "transcript_id": "transcript-1", "output": "summary",
	})
	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	if len(server.accepts) != 1 || server.accepts[0] != "text/vtt" {
		t.Errorf("content Accept headers = %v, want [text/vtt]", server.accepts)
	}
	if got := decodeTeamsRecord(t, result)["contentFormat"]; got != "text/vtt" {
		t.Errorf("contentFormat = %v, want text/vtt", got)
	}
}

// TestGetTranscript_FallsBackWhenAttributionNotAllowed validates that a tenant
// refusing speaker attribution still yields the transcript: the handler retries
// once in the unattributed format and says so in the result.
func TestGetTranscript_FallsBackWhenAttributionNotAllowed(t *testing.T) {
	server := &transcriptInnerErrorServer{refuse: map[string]string{"text/vtt": "SpeakerAttributionNotAllowed"}}
	result := runTranscriptServerHandler(t, server, NewHandleGetTranscript, map[string]any{
		"meeting_id": "meeting-1", "transcript_id": "transcript-1", "output": "summary",
	})
	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	want := []string{"text/vtt", "application/vnd.microsoft.graph.transcript+text"}
	if strings.Join(server.accepts, ",") != strings.Join(want, ",") {
		t.Errorf("content Accept headers = %v, want %v", server.accepts, want)
	}
	if got := decodeTeamsRecord(t, result)["contentFormat"]; got != want[1] {
		t.Errorf("contentFormat = %v, want %s", got, want[1])
	}
}

// TestGetTranscript_AccessDisabledSaysRetryCannotWork validates that a tenant
// that turned off transcript API access gets a correction naming the tenant
// policy, not the consent fix, and that the unattributed retry is not tried.
func TestGetTranscript_AccessDisabledSaysRetryCannotWork(t *testing.T) {
	server := &transcriptInnerErrorServer{refuse: map[string]string{"text/vtt": "GraphAccessToTranscriptsDisabled"}}
	result := runTranscriptServerHandler(t, server, NewHandleGetTranscript, map[string]any{
		"meeting_id": "meeting-1", "transcript_id": "transcript-1",
	})
	if !result.IsError {
		t.Fatalf("expected a failure, got %q", resultText(t, result))
	}
	if len(server.accepts) != 1 {
		t.Errorf("content requests = %d, want 1", len(server.accepts))
	}
	text := resultText(t, result)
	for _, want := range []string{"Set-CsTeamsMeetingConfiguration", "no retry"} {
		if !strings.Contains(text, want) {
			t.Errorf("failure = %q, want it to contain %q", text, want)
		}
	}
	if strings.Contains(text, "OnlineMeetingTranscript.Read.All") {
		t.Errorf("failure = %q, want no consent fix for a tenant policy", text)
	}
}
