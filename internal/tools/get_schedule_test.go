// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains the handler tests for get_schedule: the posted request
// body, the refusals that must happen before any Graph request, the per-mailbox
// error surfacing, and the three output tiers.
//
// @agents-index: Handler tests for calendar.get_schedule covering request body
// construction, pre-flight refusals, per-mailbox error surfacing, and output
// tiering.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
)

// getScheduleResponseJSON is a canned getSchedule response carrying two
// mailboxes, one with a busy block and working hours and one entirely free.
const getScheduleResponseJSON = `{
	"value": [
		{
			"scheduleId": "a@example.com",
			"availabilityView": "0220",
			"scheduleItems": [
				{
					"start": {"dateTime": "2026-03-12T09:00:00", "timeZone": "UTC"},
					"end": {"dateTime": "2026-03-12T10:00:00", "timeZone": "UTC"},
					"status": "busy",
					"subject": "Budget review",
					"location": "Room 1",
					"isPrivate": false
				}
			],
			"workingHours": {
				"daysOfWeek": ["monday", "tuesday"],
				"startTime": "08:00:00.0000000",
				"endTime": "17:00:00.0000000",
				"timeZone": {"name": "UTC"}
			}
		},
		{
			"scheduleId": "b@example.com",
			"availabilityView": "0000",
			"scheduleItems": [],
			"workingHours": {
				"daysOfWeek": ["monday"],
				"startTime": "09:00:00.0000000",
				"endTime": "18:00:00.0000000",
				"timeZone": {"name": "UTC"}
			}
		}
	]
}`

// getScheduleErrorResponseJSON is a canned response where Graph could read one
// mailbox and refused the other, which is how it reports a mailbox the caller
// may not view: inside the response, not as a failed call.
const getScheduleErrorResponseJSON = `{
	"value": [
		{
			"scheduleId": "a@example.com",
			"availabilityView": "0220",
			"scheduleItems": [
				{
					"start": {"dateTime": "2026-03-12T09:00:00", "timeZone": "UTC"},
					"end": {"dateTime": "2026-03-12T10:00:00", "timeZone": "UTC"},
					"status": "busy"
				}
			]
		},
		{
			"scheduleId": "denied@example.com",
			"availabilityView": "",
			"scheduleItems": [],
			"error": {"message": "Access is denied.", "responseCode": "ErrorAccessDenied"}
		}
	]
}`

// scheduleWindow is the explicit window most tests pass, so a test that is not
// about window resolution states one line rather than two.
var scheduleWindow = map[string]any{
	"start_datetime": "2026-03-12T00:00:00Z",
	"end_datetime":   "2026-03-13T00:00:00Z",
}

// withScheduleWindow returns the supplied arguments with the standard explicit
// window merged in, leaving any window the caller already stated in place.
func withScheduleWindow(args map[string]any) map[string]any {
	merged := map[string]any{}
	for k, v := range scheduleWindow {
		merged[k] = v
	}
	for k, v := range args {
		merged[k] = v
	}
	return merged
}

// runGetSchedule invokes the handler against a recording endpoint and returns
// the result together with the recorder, so a test can assert on both the
// response and what reached Graph.
func runGetSchedule(t *testing.T, response string, args map[string]any) (*mcp.CallToolResult, *findMeetingTimesRecorder) {
	t.Helper()

	recorder := &findMeetingTimesRecorder{response: response}
	client, srv := newTestGraphClient(t, recorder)
	defer srv.Close()

	handler := NewHandleGetSchedule(graph.RetryConfig{}, 30*time.Second, "UTC")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	result, err := handler(auth.WithGraphClient(context.Background(), client), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return result, recorder
}

// TestGetSchedule_Success validates that the posted body carries the mailboxes
// and the window, and that each mailbox's blocks are attributed to it in the
// text output.
func TestGetSchedule_Success(t *testing.T) {
	result, recorder := runGetSchedule(t, getScheduleResponseJSON, withScheduleWindow(map[string]any{
		"schedules": "a@example.com, b@example.com",
	}))

	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}
	if got := recorder.calls.Load(); got != 1 {
		t.Fatalf("graph request count = %d, want 1", got)
	}
	if !strings.Contains(recorder.lastBody, "a@example.com") || !strings.Contains(recorder.lastBody, "b@example.com") {
		t.Errorf("posted body does not carry both mailboxes: %s", recorder.lastBody)
	}
	if !strings.Contains(recorder.lastBody, "2026-03-12T00:00:00Z") {
		t.Errorf("posted body does not carry the window: %s", recorder.lastBody)
	}

	text := resultText(t, result)
	if !strings.Contains(text, "a@example.com") || !strings.Contains(text, "b@example.com") {
		t.Errorf("text output does not attribute blocks to each mailbox: %s", text)
	}
	if !strings.Contains(text, "2 mailbox(es) total.") {
		t.Errorf("text output does not state the total: %s", text)
	}
	if !strings.Contains(text, "No busy periods.") {
		t.Errorf("text output does not state that the free mailbox has no blocks: %s", text)
	}
}

// TestGetSchedule_ReturnsWorkingHours validates that working hours reach the
// text output, which is the capability get_free_busy has no concept of.
func TestGetSchedule_ReturnsWorkingHours(t *testing.T) {
	result, _ := runGetSchedule(t, getScheduleResponseJSON, withScheduleWindow(map[string]any{
		"schedules": "a@example.com",
	}))

	text := resultText(t, result)
	if !strings.Contains(text, "Working hours:") {
		t.Fatalf("text output states no working hours: %s", text)
	}
	if !strings.Contains(text, "08:00:00") || !strings.Contains(text, "monday") {
		t.Errorf("text output does not state the working days and bounds: %s", text)
	}
}

// TestGetSchedule_RequiresSchedules validates that a call naming no mailbox is
// refused before any Graph request.
func TestGetSchedule_RequiresSchedules(t *testing.T) {
	result, recorder := runGetSchedule(t, getScheduleResponseJSON, withScheduleWindow(map[string]any{}))

	if !result.IsError {
		t.Fatal("expected an error result when schedules is missing")
	}
	if !strings.Contains(resultText(t, result), "schedules") {
		t.Errorf("error does not name schedules: %s", resultText(t, result))
	}
	if got := recorder.calls.Load(); got != 0 {
		t.Errorf("graph request count = %d, want 0", got)
	}
}

// TestGetSchedule_RejectsInvalidAddress validates that every mailbox address is
// validated before any Graph request.
func TestGetSchedule_RejectsInvalidAddress(t *testing.T) {
	result, recorder := runGetSchedule(t, getScheduleResponseJSON, withScheduleWindow(map[string]any{
		"schedules": "a@example.com,not-an-email",
	}))

	if !result.IsError {
		t.Fatal("expected an error result for a malformed address")
	}
	if !strings.Contains(resultText(t, result), "not-an-email") {
		t.Errorf("error does not name the invalid address: %s", resultText(t, result))
	}
	// The shared email validator names the offending value and stops there, so
	// the refusal is required to name the parameter it came from and the
	// correction to apply.
	if !strings.Contains(resultText(t, result), "schedules") {
		t.Errorf("error does not name the parameter the address came from: %s", resultText(t, result))
	}
	if !strings.Contains(resultText(t, result), getScheduleAddressFix) {
		t.Errorf("error states no correction to apply: %s", resultText(t, result))
	}
	if got := recorder.calls.Load(); got != 0 {
		t.Errorf("graph request count = %d, want 0", got)
	}
}

// TestGetSchedule_RequiresWindow validates that a request resolving to no window
// is refused before any Graph request, with both routes to a window named.
func TestGetSchedule_RequiresWindow(t *testing.T) {
	result, recorder := runGetSchedule(t, getScheduleResponseJSON, map[string]any{
		"schedules": "a@example.com",
	})

	if !result.IsError {
		t.Fatal("expected an error result when no window resolves")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "start_datetime") || !strings.Contains(text, "end_datetime") || !strings.Contains(text, "date") {
		t.Errorf("error does not name the window parameters: %s", text)
	}
	if got := recorder.calls.Load(); got != 0 {
		t.Errorf("graph request count = %d, want 0", got)
	}
}

// TestGetSchedule_EnforcesMailboxCeiling validates that a list longer than the
// service accepts is refused before any Graph request, with the ceiling stated.
func TestGetSchedule_EnforcesMailboxCeiling(t *testing.T) {
	addresses := make([]string, maxScheduleMailboxes+1)
	for i := range addresses {
		addresses[i] = fmt.Sprintf("user%d@example.com", i)
	}

	result, recorder := runGetSchedule(t, getScheduleResponseJSON, withScheduleWindow(map[string]any{
		"schedules": strings.Join(addresses, ","),
	}))

	if !result.IsError {
		t.Fatal("expected an error result for a list beyond the ceiling")
	}
	if !strings.Contains(resultText(t, result), "20") {
		t.Errorf("error does not state the ceiling of twenty: %s", resultText(t, result))
	}
	if got := recorder.calls.Load(); got != 0 {
		t.Errorf("graph request count = %d, want 0", got)
	}
}

// TestGetSchedule_RejectsOutOfBoundInterval validates that the interval bounds
// are enforced before any Graph request, with the bounds stated.
func TestGetSchedule_RejectsOutOfBoundInterval(t *testing.T) {
	cases := []struct {
		name     string
		interval float64
	}{
		{name: "below the floor", interval: 4},
		{name: "above the ceiling", interval: 1441},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, recorder := runGetSchedule(t, getScheduleResponseJSON, withScheduleWindow(map[string]any{
				"schedules":                  "a@example.com",
				"availability_view_interval": tc.interval,
			}))

			if !result.IsError {
				t.Fatal("expected an error result for an out-of-bound interval")
			}
			text := resultText(t, result)
			if !strings.Contains(text, "availability_view_interval") || !strings.Contains(text, "1440") {
				t.Errorf("error does not name the parameter and its bounds: %s", text)
			}
			if got := recorder.calls.Load(); got != 0 {
				t.Errorf("graph request count = %d, want 0", got)
			}
		})
	}
}

// TestGetSchedule_AppliesIntervalDefault validates that the published default
// reaches the request body rather than being left to the service.
func TestGetSchedule_AppliesIntervalDefault(t *testing.T) {
	_, recorder := runGetSchedule(t, getScheduleResponseJSON, withScheduleWindow(map[string]any{
		"schedules": "a@example.com",
	}))

	// The key is PascalCase because that is what the pinned SDK writes for this
	// request body, unlike the camelCase it writes for findMeetingTimes. The
	// assertion states the wire form that actually leaves the process rather
	// than the one the Graph reference documents.
	if !strings.Contains(recorder.lastBody, `"AvailabilityViewInterval":30`) {
		t.Errorf("posted body does not carry the default interval: %s", recorder.lastBody)
	}
}

// TestGetSchedule_PerMailboxErrorSurfaced validates that a mailbox the caller
// may not view is reported with its message and response code, and that the
// mailbox that succeeded keeps its blocks.
func TestGetSchedule_PerMailboxErrorSurfaced(t *testing.T) {
	result, _ := runGetSchedule(t, getScheduleErrorResponseJSON, withScheduleWindow(map[string]any{
		"schedules": "a@example.com,denied@example.com",
	}))

	if result.IsError {
		t.Fatalf("a per-mailbox error must not fail the whole call, got %q", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, "Access is denied.") || !strings.Contains(text, "ErrorAccessDenied") {
		t.Errorf("text output does not state the mailbox error and its response code: %s", text)
	}
	if !strings.Contains(text, "denied@example.com") {
		t.Errorf("text output does not name the mailbox that failed: %s", text)
	}
	if !strings.Contains(text, "busy") {
		t.Errorf("text output dropped the blocks of the mailbox that succeeded: %s", text)
	}
}

// TestGetSchedule_DateShorthandResolvesWindow validates that the shorthand
// resolves to a posted window through the same expansion the other calendar
// reads use.
func TestGetSchedule_DateShorthandResolvesWindow(t *testing.T) {
	result, recorder := runGetSchedule(t, getScheduleResponseJSON, map[string]any{
		"schedules": "a@example.com",
		"date":      "tomorrow",
	})

	if result.IsError {
		t.Fatalf("expected success, got %q", resultText(t, result))
	}

	expectedStart, expectedEnd, err := expandDateParam("tomorrow", "UTC")
	if err != nil {
		t.Fatalf("date expansion failed: %v", err)
	}
	if !strings.Contains(recorder.lastBody, expectedStart) || !strings.Contains(recorder.lastBody, expectedEnd) {
		t.Errorf("posted body does not carry the expanded window %s to %s: %s", expectedStart, expectedEnd, recorder.lastBody)
	}
	if !strings.Contains(resultText(t, result), "a@example.com") {
		t.Errorf("text output lists no availability: %s", resultText(t, result))
	}
}

// TestGetSchedule_ExplicitDatetimesBeatDateShorthand validates the documented
// precedence: an explicit window wins over the shorthand.
func TestGetSchedule_ExplicitDatetimesBeatDateShorthand(t *testing.T) {
	_, recorder := runGetSchedule(t, getScheduleResponseJSON, withScheduleWindow(map[string]any{
		"schedules": "a@example.com",
		"date":      "tomorrow",
	}))

	if !strings.Contains(recorder.lastBody, "2026-03-12T00:00:00Z") {
		t.Errorf("posted body does not carry the explicit window: %s", recorder.lastBody)
	}
}

// TestGetSchedule_RejectsInvalidDatetime validates that a malformed explicit
// bound is refused before any Graph request.
func TestGetSchedule_RejectsInvalidDatetime(t *testing.T) {
	result, recorder := runGetSchedule(t, getScheduleResponseJSON, map[string]any{
		"schedules":      "a@example.com",
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

// TestGetSchedule_SummaryAndRawTiersDiffer validates that the three output tiers
// are honoured and that summary is a chosen field set rather than raw.
func TestGetSchedule_SummaryAndRawTiersDiffer(t *testing.T) {
	args := func(mode string) map[string]any {
		return withScheduleWindow(map[string]any{"schedules": "a@example.com,b@example.com", "output": mode})
	}

	textResult, _ := runGetSchedule(t, getScheduleResponseJSON, args("text"))
	if !strings.Contains(resultText(t, textResult), "Working hours:") {
		t.Errorf("text tier is not a labeled per-mailbox listing: %s", resultText(t, textResult))
	}

	summaryResult, _ := runGetSchedule(t, getScheduleResponseJSON, args("summary"))
	rawResult, _ := runGetSchedule(t, getScheduleResponseJSON, args("raw"))
	summaryText := resultText(t, mustSucceed(t, summaryResult))
	rawText := resultText(t, mustSucceed(t, rawResult))

	if summaryText == rawText {
		t.Fatal("summary and raw tiers returned identical payloads")
	}

	var summary, raw GetScheduleResponse
	if err := json.Unmarshal([]byte(summaryText), &summary); err != nil {
		t.Fatalf("summary tier is not valid JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(rawText), &raw); err != nil {
		t.Fatalf("raw tier is not valid JSON: %v", err)
	}
	if len(summary.Schedules) != 2 || len(raw.Schedules) != 2 {
		t.Fatalf("mailbox counts = %d summary, %d raw, want 2 and 2", len(summary.Schedules), len(raw.Schedules))
	}
	if _, ok := raw.Schedules[0]["error"]; !ok {
		t.Error("raw tier omits the error key, so its shape varies between mailboxes")
	}
	if _, ok := summary.Schedules[0]["error"]; ok {
		t.Error("summary tier carries an error key for a mailbox Graph read successfully")
	}
}

// TestGetSchedule_NoClientInContext validates that the handler refuses when no
// account has been resolved into the request context.
func TestGetSchedule_NoClientInContext(t *testing.T) {
	handler := NewHandleGetSchedule(graph.RetryConfig{}, 30*time.Second, "UTC")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = withScheduleWindow(map[string]any{"schedules": "a@example.com"})

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected an error result when no client is in context")
	}
}

// TestGetSchedule_SendsTimezonePreferHeader validates that the request asks
// Graph for response times in the window's zone, read from the recorded header.
func TestGetSchedule_SendsTimezonePreferHeader(t *testing.T) {
	_, recorder := runGetSchedule(t, getScheduleResponseJSON, withScheduleWindow(map[string]any{
		"schedules": "a@example.com",
		"timezone":  "Europe/Stockholm",
	}))
	if want := `outlook.timezone="Europe/Stockholm"`; recorder.lastPrefer != want {
		t.Errorf("Prefer header = %q, want %q", recorder.lastPrefer, want)
	}

	_, recorder = runGetSchedule(t, getScheduleResponseJSON, withScheduleWindow(map[string]any{
		"schedules": "a@example.com",
	}))
	if want := `outlook.timezone="UTC"`; recorder.lastPrefer != want {
		t.Errorf("default Prefer header = %q, want %q", recorder.lastPrefer, want)
	}
}

// TestGetSchedule_RejectsInvalidWindowSpan validates that an inverted, empty,
// or 62-day-or-longer window is refused before any request, naming the limit.
func TestGetSchedule_RejectsInvalidWindowSpan(t *testing.T) {
	cases := map[string][2]string{
		"inverted": {"2026-03-13T00:00:00Z", "2026-03-12T00:00:00Z"},
		"empty":    {"2026-03-12T00:00:00Z", "2026-03-12T00:00:00Z"},
		"62 days":  {"2026-03-01T00:00:00Z", "2026-05-02T00:00:00Z"},
	}
	for name, window := range cases {
		t.Run(name, func(t *testing.T) {
			result, recorder := runGetSchedule(t, getScheduleResponseJSON, map[string]any{
				"schedules":      "a@example.com",
				"start_datetime": window[0],
				"end_datetime":   window[1],
			})
			if !result.IsError {
				t.Fatal("expected an error")
			}
			text := result.Content[0].(mcp.TextContent).Text
			if !strings.Contains(text, "62 days") || !strings.Contains(text, "end_datetime") {
				t.Errorf("error does not name the ceiling and the bounds: %s", text)
			}
			if got := recorder.calls.Load(); got != 0 {
				t.Errorf("Graph calls = %d, want 0", got)
			}
		})
	}
}

// TestGetSchedule_AcceptsWindowJustUnderCeiling validates that a window one
// second shorter than 62 days still reaches Graph.
func TestGetSchedule_AcceptsWindowJustUnderCeiling(t *testing.T) {
	result, recorder := runGetSchedule(t, getScheduleResponseJSON, map[string]any{
		"schedules":      "a@example.com",
		"start_datetime": "2026-03-01T00:00:00Z",
		"end_datetime":   "2026-05-01T23:59:59Z",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content[0].(mcp.TextContent).Text)
	}
	if got := recorder.calls.Load(); got != 1 {
		t.Errorf("Graph calls = %d, want 1", got)
	}
}
