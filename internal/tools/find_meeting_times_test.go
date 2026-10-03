// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains the handler tests for find_meeting_times: the posted
// request body, the refusals that must happen before any Graph request, and the
// three output tiers.
//
// @agents-index: Handler tests for calendar.find_meeting_times covering request
// body construction, pre-flight refusals, and output tiering.
package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
)

// findMeetingTimesResponseJSON is a canned findMeetingTimes response carrying
// two ranked suggestions, used wherever the test needs a successful call.
const findMeetingTimesResponseJSON = `{
	"emptySuggestionsReason": "",
	"meetingTimeSuggestions": [
		{
			"confidence": 100.0,
			"order": 1,
			"organizerAvailability": "free",
			"suggestionReason": "Suggested because everyone is free.",
			"meetingTimeSlot": {
				"start": {"dateTime": "2026-03-12T09:00:00", "timeZone": "UTC"},
				"end": {"dateTime": "2026-03-12T09:30:00", "timeZone": "UTC"}
			},
			"attendeeAvailability": [
				{"attendee": {"type": "required", "emailAddress": {"name": "A", "address": "a@example.com"}}, "availability": "free"}
			],
			"locations": []
		},
		{
			"confidence": 50.0,
			"order": 2,
			"organizerAvailability": "tentative",
			"meetingTimeSlot": {
				"start": {"dateTime": "2026-03-12T14:00:00", "timeZone": "UTC"},
				"end": {"dateTime": "2026-03-12T14:30:00", "timeZone": "UTC"}
			},
			"attendeeAvailability": [],
			"locations": []
		}
	]
}`

// findMeetingTimesRecorder is a test Graph endpoint that records every request
// body it receives and replies with the supplied JSON. Recording the count is
// what lets a test assert that a refusal happened before any Graph call.
type findMeetingTimesRecorder struct {
	calls    atomic.Int32
	lastBody string
	// lastPrefer is the Prefer header of the last request, recorded so a test
	// asserts what Graph actually received.
	lastPrefer string
	response   string
}

func (r *findMeetingTimesRecorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.calls.Add(1)
	r.lastPrefer = req.Header.Get("Prefer")
	if body, err := io.ReadAll(req.Body); err == nil {
		r.lastBody = string(body)
	}
	w.Header().Set("Content-Type", "application/json")
	//nolint:errcheck // test helper
	w.Write([]byte(r.response))
}

// runFindMeetingTimes invokes the handler against a recording endpoint and
// returns the result together with the recorder, so a test can assert on both
// the response and what reached Graph.
func runFindMeetingTimes(t *testing.T, response string, args map[string]any) (*mcp.CallToolResult, *findMeetingTimesRecorder) {
	t.Helper()

	recorder := &findMeetingTimesRecorder{response: response}
	client, srv := newTestGraphClient(t, recorder)
	defer srv.Close()

	handler := NewHandleFindMeetingTimes(graph.RetryConfig{}, 30*time.Second, "UTC")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	result, err := handler(auth.WithGraphClient(context.Background(), client), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return result, recorder
}

// TestFindMeetingTimes_Success validates that the posted body carries the
// attendees and that the ranked suggestions reach the text output.
func TestFindMeetingTimes_Success(t *testing.T) {
	result, recorder := runFindMeetingTimes(t, findMeetingTimesResponseJSON, map[string]any{
		"attendees": `[{"email":"a@example.com","name":"A"},{"email":"b@example.com","type":"optional"}]`,
	})

	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	if got := recorder.calls.Load(); got != 1 {
		t.Fatalf("graph request count = %d, want 1", got)
	}
	if !strings.Contains(recorder.lastBody, "a@example.com") || !strings.Contains(recorder.lastBody, "b@example.com") {
		t.Errorf("posted body does not carry both attendees: %s", recorder.lastBody)
	}

	text := resultText(t, result)
	if !strings.Contains(text, "Confidence 100%") {
		t.Errorf("text output does not state the confidence: %s", text)
	}
	if !strings.Contains(text, "2 suggestion(s) total.") {
		t.Errorf("text output does not state the total: %s", text)
	}
}

// TestFindMeetingTimes_RequiresAttendees validates that a call naming no
// attendees is refused before any Graph request.
func TestFindMeetingTimes_RequiresAttendees(t *testing.T) {
	result, recorder := runFindMeetingTimes(t, findMeetingTimesResponseJSON, map[string]any{})

	if !result.IsError {
		t.Fatal("expected an error result when attendees is missing")
	}
	if !strings.Contains(resultText(t, result), "attendees") {
		t.Errorf("error does not name attendees: %s", resultText(t, result))
	}
	if got := recorder.calls.Load(); got != 0 {
		t.Errorf("graph request count = %d, want 0", got)
	}
}

// TestFindMeetingTimes_RejectsInvalidAttendeeEmail validates that every
// attendee email is validated before any Graph request.
func TestFindMeetingTimes_RejectsInvalidAttendeeEmail(t *testing.T) {
	result, recorder := runFindMeetingTimes(t, findMeetingTimesResponseJSON, map[string]any{
		"attendees": `[{"email":"a@example.com"},{"email":"not-an-email"}]`,
	})

	if !result.IsError {
		t.Fatal("expected an error result for a malformed attendee email")
	}
	if !strings.Contains(resultText(t, result), "not-an-email") {
		t.Errorf("error does not name the invalid email: %s", resultText(t, result))
	}
	if got := recorder.calls.Load(); got != 0 {
		t.Errorf("graph request count = %d, want 0", got)
	}
}

// TestFindMeetingTimes_RejectsUnparseableDuration validates that a duration
// that is not ISO 8601 is refused before any Graph request, with the correction
// stated in the error.
func TestFindMeetingTimes_RejectsUnparseableDuration(t *testing.T) {
	result, recorder := runFindMeetingTimes(t, findMeetingTimesResponseJSON, map[string]any{
		"attendees":        `[{"email":"a@example.com"}]`,
		"meeting_duration": "30m",
	})

	if !result.IsError {
		t.Fatal("expected an error result for an unparseable duration")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "meeting_duration") || !strings.Contains(text, "PT30M") {
		t.Errorf("error does not name the parameter and a correct value: %s", text)
	}
	if got := recorder.calls.Load(); got != 0 {
		t.Errorf("graph request count = %d, want 0", got)
	}
}

// TestFindMeetingTimes_RejectsInvalidDatetime validates that each supplied
// window bound is validated before any Graph request.
func TestFindMeetingTimes_RejectsInvalidDatetime(t *testing.T) {
	result, recorder := runFindMeetingTimes(t, findMeetingTimesResponseJSON, map[string]any{
		"attendees":      `[{"email":"a@example.com"}]`,
		"start_datetime": "not-a-datetime",
		"end_datetime":   "2026-03-13T00:00:00Z",
	})

	if !result.IsError {
		t.Fatal("expected an error result for a malformed window bound")
	}
	if !strings.Contains(resultText(t, result), "start_datetime") {
		t.Errorf("error does not name the invalid parameter: %s", resultText(t, result))
	}
	if got := recorder.calls.Load(); got != 0 {
		t.Errorf("graph request count = %d, want 0", got)
	}
}

// TestFindMeetingTimes_RejectsHalfOpenWindow validates that exactly one bound
// is refused, because a half-open window has no honest reading.
func TestFindMeetingTimes_RejectsHalfOpenWindow(t *testing.T) {
	result, recorder := runFindMeetingTimes(t, findMeetingTimesResponseJSON, map[string]any{
		"attendees":      `[{"email":"a@example.com"}]`,
		"start_datetime": "2026-03-12T00:00:00Z",
	})

	if !result.IsError {
		t.Fatal("expected an error result for a half-open window")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "start_datetime") || !strings.Contains(text, "end_datetime") {
		t.Errorf("error does not name both window parameters: %s", text)
	}
	if got := recorder.calls.Load(); got != 0 {
		t.Errorf("graph request count = %d, want 0", got)
	}
}

// TestFindMeetingTimes_OmitsTimeConstraintWhenNoBounds validates that Graph's
// own default window stays in force when the caller supplies neither bound.
func TestFindMeetingTimes_OmitsTimeConstraintWhenNoBounds(t *testing.T) {
	result, recorder := runFindMeetingTimes(t, findMeetingTimesResponseJSON, map[string]any{
		"attendees": `[{"email":"a@example.com"}]`,
	})

	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	if strings.Contains(recorder.lastBody, "timeConstraint") {
		t.Errorf("posted body carries a timeConstraint the caller did not ask for: %s", recorder.lastBody)
	}
}

// TestFindMeetingTimes_AppliesDurationAndCandidateDefaults validates that the
// published defaults reach the request body rather than being left to Graph.
func TestFindMeetingTimes_AppliesDurationAndCandidateDefaults(t *testing.T) {
	_, recorder := runFindMeetingTimes(t, findMeetingTimesResponseJSON, map[string]any{
		"attendees": `[{"email":"a@example.com"}]`,
	})

	if !strings.Contains(recorder.lastBody, `"meetingDuration":"PT30M"`) {
		t.Errorf("posted body does not carry the default duration: %s", recorder.lastBody)
	}
	if !strings.Contains(recorder.lastBody, `"maxCandidates":20`) {
		t.Errorf("posted body does not carry the default candidate count: %s", recorder.lastBody)
	}
}

// TestFindMeetingTimes_OmitsUnsuppliedOptionalsFromBody validates that a
// parameter with no default is left out of the body entirely, so the service's
// own default is not shadowed by one this server invents.
//
// isOrganizerOptional is deliberately not asserted here: the SDK's own request
// body constructor stamps it false before the handler sees the object, which is
// also the value Graph documents as its default, so the handler cannot omit it
// and gains nothing by trying.
func TestFindMeetingTimes_OmitsUnsuppliedOptionalsFromBody(t *testing.T) {
	_, recorder := runFindMeetingTimes(t, findMeetingTimesResponseJSON, map[string]any{
		"attendees": `[{"email":"a@example.com"}]`,
	})

	if strings.Contains(recorder.lastBody, "minimumAttendeePercentage") {
		t.Errorf("posted body carries minimumAttendeePercentage although the caller supplied none: %s", recorder.lastBody)
	}
}

// TestFindMeetingTimes_SuppliedOptionalsReachBody validates that the optional
// constraints the caller does supply are carried into the request body.
func TestFindMeetingTimes_SuppliedOptionalsReachBody(t *testing.T) {
	_, recorder := runFindMeetingTimes(t, findMeetingTimesResponseJSON, map[string]any{
		"attendees":                   `[{"email":"a@example.com"}]`,
		"minimum_attendee_percentage": float64(75),
		"is_organizer_optional":       true,
	})

	if !strings.Contains(recorder.lastBody, `"minimumAttendeePercentage":75`) {
		t.Errorf("posted body does not carry the supplied attendee percentage: %s", recorder.lastBody)
	}
	if !strings.Contains(recorder.lastBody, `"isOrganizerOptional":true`) {
		t.Errorf("posted body does not carry the supplied organizer flag: %s", recorder.lastBody)
	}
}

// TestFindMeetingTimes_RejectsOutOfBoundNumerics validates that each numeric
// bound is enforced before any Graph request, with the bound stated.
func TestFindMeetingTimes_RejectsOutOfBoundNumerics(t *testing.T) {
	cases := []struct {
		name  string
		args  map[string]any
		names string
	}{
		{
			name:  "candidate count below the floor",
			args:  map[string]any{"attendees": `[{"email":"a@example.com"}]`, "max_candidates": float64(0)},
			names: "max_candidates",
		},
		{
			name:  "attendee percentage above the ceiling",
			args:  map[string]any{"attendees": `[{"email":"a@example.com"}]`, "minimum_attendee_percentage": float64(101)},
			names: "minimum_attendee_percentage",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, recorder := runFindMeetingTimes(t, findMeetingTimesResponseJSON, tc.args)
			if !result.IsError {
				t.Fatal("expected an error result for an out-of-bound value")
			}
			if !strings.Contains(resultText(t, result), tc.names) {
				t.Errorf("error does not name the parameter: %s", resultText(t, result))
			}
			if got := recorder.calls.Load(); got != 0 {
				t.Errorf("graph request count = %d, want 0", got)
			}
		})
	}
}

// TestFindMeetingTimes_EmptySuggestionsReasonSurfaced validates that an empty
// result explains itself rather than only reporting that nothing was found.
func TestFindMeetingTimes_EmptySuggestionsReasonSurfaced(t *testing.T) {
	const emptyResponse = `{"emptySuggestionsReason":"AttendeesUnavailable","meetingTimeSuggestions":[]}`

	result, _ := runFindMeetingTimes(t, emptyResponse, map[string]any{
		"attendees": `[{"email":"a@example.com"}]`,
	})

	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	if !strings.Contains(resultText(t, result), "AttendeesUnavailable") {
		t.Errorf("text output does not state the empty-suggestions reason: %s", resultText(t, result))
	}
}

// TestFindMeetingTimes_SummaryAndRawTiersDiffer validates that the three output
// tiers are honoured and that summary is a chosen field set rather than raw.
func TestFindMeetingTimes_SummaryAndRawTiersDiffer(t *testing.T) {
	args := func(mode string) map[string]any {
		return map[string]any{"attendees": `[{"email":"a@example.com"}]`, "output": mode}
	}

	textResult, _ := runFindMeetingTimes(t, findMeetingTimesResponseJSON, args("text"))
	if !strings.Contains(resultText(t, textResult), "1. ") {
		t.Errorf("text tier is not a numbered listing: %s", resultText(t, textResult))
	}

	summaryResult, _ := runFindMeetingTimes(t, findMeetingTimesResponseJSON, args("summary"))
	rawResult, _ := runFindMeetingTimes(t, findMeetingTimesResponseJSON, args("raw"))
	summaryText := resultText(t, mustSucceed(t, summaryResult))
	rawText := resultText(t, mustSucceed(t, rawResult))

	if summaryText == rawText {
		t.Fatal("summary and raw tiers returned identical payloads")
	}

	var summary, raw FindMeetingTimesResponse
	if err := json.Unmarshal([]byte(summaryText), &summary); err != nil {
		t.Fatalf("summary tier is not valid JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(rawText), &raw); err != nil {
		t.Fatalf("raw tier is not valid JSON: %v", err)
	}
	if len(summary.Suggestions) != 2 || len(raw.Suggestions) != 2 {
		t.Fatalf("suggestion counts = %d summary, %d raw, want 2 and 2", len(summary.Suggestions), len(raw.Suggestions))
	}
	if _, ok := raw.Suggestions[0]["attendeeAvailability"]; !ok {
		t.Error("raw tier omits attendeeAvailability")
	}
	if _, ok := summary.Suggestions[0]["attendeeAvailability"]; ok {
		t.Error("summary tier carries attendeeAvailability, which belongs to raw")
	}
}

// mustSucceed fails the test when the tool result is an error, and returns it
// otherwise so tier assertions read as one expression.
func mustSucceed(t *testing.T, result *mcp.CallToolResult) *mcp.CallToolResult {
	t.Helper()
	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	return result
}

// TestFindMeetingTimes_NoClientInContext validates that the handler refuses
// when no account has been resolved into the request context.
func TestFindMeetingTimes_NoClientInContext(t *testing.T) {
	handler := NewHandleFindMeetingTimes(graph.RetryConfig{}, 30*time.Second, "UTC")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"attendees": `[{"email":"a@example.com"}]`}

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected an error result when no client is in context")
	}
}

// TestFindMeetingTimes_FormatterDeterministic validates that identical Graph
// responses format identically in every tier, including suggestion ordering.
func TestFindMeetingTimes_FormatterDeterministic(t *testing.T) {
	for _, mode := range []string{"text", "summary", "raw"} {
		args := map[string]any{"attendees": `[{"email":"a@example.com"}]`, "output": mode}
		first, _ := runFindMeetingTimes(t, findMeetingTimesResponseJSON, args)
		second, _ := runFindMeetingTimes(t, findMeetingTimesResponseJSON, args)
		if resultText(t, first) != resultText(t, second) {
			t.Errorf("output mode %q is not deterministic across identical responses", mode)
		}
	}
}

// TestFindMeetingTimes_RejectsZeroDuration validates that a zero duration is
// refused before any request, because it would serialize as the invalid "P".
func TestFindMeetingTimes_RejectsZeroDuration(t *testing.T) {
	for _, value := range []string{"PT0M", "PT0S", "P0D"} {
		t.Run(value, func(t *testing.T) {
			result, recorder := runFindMeetingTimes(t, findMeetingTimesResponseJSON, map[string]any{
				"attendees":        `[{"email":"a@example.com"}]`,
				"meeting_duration": value,
			})
			if !result.IsError {
				t.Fatal("expected an error for a zero duration")
			}
			text := result.Content[0].(mcp.TextContent).Text
			if !strings.Contains(text, "meeting_duration") || !strings.Contains(text, "PT30M") {
				t.Errorf("error does not name the parameter and an example: %s", text)
			}
			if got := recorder.calls.Load(); got != 0 {
				t.Errorf("Graph calls = %d, want 0", got)
			}
		})
	}
}

// TestFindMeetingTimes_GraphErrorNamesPersonalAccountCase validates that a
// Graph refusal carries the fix text naming the personal-account limitation.
//
// Graph does not support findMeetingTimes for personal Microsoft accounts and
// the server cannot know the account type before the call, so the only place
// the caller learns the cause is the error returned here. The fake endpoint
// answers with an OData error, which is what Graph returns in that case.
func TestFindMeetingTimes_GraphErrorNamesPersonalAccountCase(t *testing.T) {
	refusal := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		//nolint:errcheck // test helper
		w.Write([]byte(`{"error":{"code":"ErrorInvalidRequest","message":"The request is not supported for this account type."}}`))
	})
	client, srv := newTestGraphClient(t, refusal)
	defer srv.Close()

	handler := NewHandleFindMeetingTimes(graph.RetryConfig{}, 30*time.Second, "UTC")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"attendees": `[{"email":"a@example.com"}]`}

	result, err := handler(auth.WithGraphClient(context.Background(), client), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected an error result for a Graph refusal")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "work or school account") {
		t.Errorf("error does not name the required account type: %s", text)
	}
	if !strings.Contains(text, "personal Microsoft accounts are not supported") {
		t.Errorf("error does not name the personal-account case: %s", text)
	}
}
