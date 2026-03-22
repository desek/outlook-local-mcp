// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the list_events tool, including tool
// registration validation, parameter handling, and handler construction.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// TestListEventsTool_Registration validates that the NewListEventsTool is
// properly defined with the expected name and read-only annotation.
func TestListEventsTool_Registration(t *testing.T) {
	tool := NewListEventsTool()
	if tool.Name != "calendar_list_events" {
		t.Errorf("tool name = %q, want %q", tool.Name, "calendar_list_events")
	}

	annotations := tool.Annotations
	if annotations.ReadOnlyHint == nil || !*annotations.ReadOnlyHint {
		t.Error("expected ReadOnlyHint to be true")
	}
}

// TestListEventsTool_NoRequiredParameters validates that no parameters are
// marked as required in the tool's input schema, since the date convenience
// parameter can substitute for start_datetime and end_datetime.
func TestListEventsTool_NoRequiredParameters(t *testing.T) {
	tool := NewListEventsTool()
	schema := tool.InputSchema

	if len(schema.Required) != 0 {
		t.Errorf("expected 0 required parameters, got %d: %v", len(schema.Required), schema.Required)
	}
}

// TestListEventsTool_HasEightParameters validates that the tool defines exactly
// eight parameters: date, start_datetime, end_datetime, calendar_id,
// max_results, timezone, account, and output.
func TestListEventsTool_HasEightParameters(t *testing.T) {
	tool := NewListEventsTool()
	schema := tool.InputSchema

	expected := []string{"date", "start_datetime", "end_datetime", "calendar_id", "max_results", "timezone", "account", "output"}
	if len(schema.Properties) != len(expected) {
		t.Errorf("expected %d properties, got %d", len(expected), len(schema.Properties))
	}
	for _, name := range expected {
		if _, ok := schema.Properties[name]; !ok {
			t.Errorf("expected property %q to be defined", name)
		}
	}
}

// TestNewHandleListEvents_ReturnsHandler validates that NewHandleListEvents
// returns a non-nil handler function.
func TestNewHandleListEvents_ReturnsHandler(t *testing.T) {
	handler := NewHandleListEvents(graph.RetryConfig{}, 0, "UTC", "")
	if handler == nil {
		t.Fatal("expected non-nil handler function")
	}
}

// TestListEventsToolCanBeAddedToServer validates that the NewListEventsTool
// and its handler can be registered on an MCP server without error or panic.
func TestListEventsToolCanBeAddedToServer(t *testing.T) {
	s := server.NewMCPServer("test-server", "0.0.1",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)
	// Must not panic.
	s.AddTool(NewListEventsTool(), NewHandleListEvents(graph.RetryConfig{}, 0, "UTC", ""))
}

// TestHandleListEvents_NoClientInContext validates that the handler returns
// a tool error when no Graph client is present in the context.
func TestHandleListEvents_NoClientInContext(t *testing.T) {
	handler := NewHandleListEvents(graph.RetryConfig{}, 0, "UTC", "")
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"start_datetime": "2026-03-12T00:00:00Z",
		"end_datetime":   "2026-03-13T00:00:00Z",
	}

	result, err := handler(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when no client in context")
	}
	text := result.Content[0].(mcp.TextContent).Text
	if text != "no account selected" {
		t.Errorf("error text = %q, want %q", text, "no account selected")
	}
}

// TestHandleListEvents_MissingStartDatetime validates that the handler returns
// a tool error when start_datetime is not provided.
func TestHandleListEvents_MissingStartDatetime(t *testing.T) {
	handler := NewHandleListEvents(graph.RetryConfig{}, 0, "UTC", "")
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"end_datetime": "2026-03-13T00:00:00Z",
	}

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when start_datetime is missing")
	}
}

// TestHandleListEvents_MissingEndDatetime validates that the handler returns
// a tool error when end_datetime is not provided.
func TestHandleListEvents_MissingEndDatetime(t *testing.T) {
	handler := NewHandleListEvents(graph.RetryConfig{}, 0, "UTC", "")
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"start_datetime": "2026-03-12T00:00:00Z",
	}

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when end_datetime is missing")
	}
}

// TestListEvents_OutputSummary validates that the summary output mode returns
// summary format with 8 keys per event and flattened start/end/organizer/location.
func TestListEvents_OutputSummary(t *testing.T) {
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper
		w.Write([]byte(`{"value":[{"id":"evt-1","subject":"Sync","start":{"dateTime":"2026-03-12T09:00:00","timeZone":"UTC"},"end":{"dateTime":"2026-03-12T10:00:00","timeZone":"UTC"},"location":{"displayName":"Room A"},"organizer":{"emailAddress":{"name":"Jane","address":"jane@example.com"}},"showAs":"busy","isOnlineMeeting":true,"isAllDay":false,"isCancelled":false,"webLink":"https://outlook.office.com/event/123","importance":"normal","sensitivity":"normal","categories":[]}]}`))
	}))
	defer srv.Close()

	handler := NewHandleListEvents(graph.RetryConfig{}, 30*time.Second, "UTC", "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"start_datetime": "2026-03-12T00:00:00Z",
		"end_datetime":   "2026-03-13T00:00:00Z",
		"output":         "summary",
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %+v", result)
	}

	var events []map[string]any
	text := result.Content[0].(mcp.TextContent).Text
	if err := json.Unmarshal([]byte(text), &events); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]

	// Summary format should have exactly 9 keys (8 base + displayTime).
	if len(event) != 9 {
		t.Errorf("expected 9 keys in summary, got %d: %v", len(event), keys(event))
	}

	// Verify flat strings (not nested objects).
	if event["start"] != "2026-03-12T09:00:00" {
		t.Errorf("start = %v, want %q", event["start"], "2026-03-12T09:00:00")
	}
	if event["end"] != "2026-03-12T10:00:00" {
		t.Errorf("end = %v, want %q", event["end"], "2026-03-12T10:00:00")
	}
	if event["location"] != "Room A" {
		t.Errorf("location = %v, want %q", event["location"], "Room A")
	}
	if event["organizer"] != "Jane" {
		t.Errorf("organizer = %v, want %q", event["organizer"], "Jane")
	}

	// Verify excluded fields are absent.
	for _, excluded := range []string{"webLink", "importance", "sensitivity", "categories", "isAllDay", "isCancelled"} {
		if _, ok := event[excluded]; ok {
			t.Errorf("summary should not contain %q", excluded)
		}
	}
}

// TestListEvents_OutputRaw validates that output=raw returns the full field set
// with nested start/end objects and all metadata fields.
func TestListEvents_OutputRaw(t *testing.T) {
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper
		w.Write([]byte(`{"value":[{"id":"evt-1","subject":"Sync","start":{"dateTime":"2026-03-12T09:00:00","timeZone":"UTC"},"end":{"dateTime":"2026-03-12T10:00:00","timeZone":"UTC"},"location":{"displayName":"Room A"},"organizer":{"emailAddress":{"name":"Jane","address":"jane@example.com"}},"showAs":"busy","isOnlineMeeting":true,"isAllDay":false,"isCancelled":false,"webLink":"https://outlook.office.com/event/123","importance":"normal","sensitivity":"normal","categories":["Work"]}]}`))
	}))
	defer srv.Close()

	handler := NewHandleListEvents(graph.RetryConfig{}, 30*time.Second, "UTC", "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"start_datetime": "2026-03-12T00:00:00Z",
		"end_datetime":   "2026-03-13T00:00:00Z",
		"output":         "raw",
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %+v", result)
	}

	var events []map[string]any
	text := result.Content[0].(mcp.TextContent).Text
	if err := json.Unmarshal([]byte(text), &events); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]

	// Raw format should have nested start object.
	startObj, ok := event["start"].(map[string]any)
	if !ok {
		t.Fatalf("start should be a nested object in raw mode, got %T", event["start"])
	}
	if startObj["dateTime"] != "2026-03-12T09:00:00" {
		t.Errorf("start.dateTime = %v, want %q", startObj["dateTime"], "2026-03-12T09:00:00")
	}

	// Raw format should include webLink and categories.
	if _, ok := event["webLink"]; !ok {
		t.Error("raw format should contain webLink")
	}
	if _, ok := event["categories"]; !ok {
		t.Error("raw format should contain categories")
	}
}

// TestListEvents_OutputInvalid validates that an invalid output value returns
// the expected error message.
func TestListEvents_OutputInvalid(t *testing.T) {
	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()

	handler := NewHandleListEvents(graph.RetryConfig{}, 30*time.Second, "UTC", "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"start_datetime": "2026-03-12T00:00:00Z",
		"end_datetime":   "2026-03-13T00:00:00Z",
		"output":         "verbose",
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for invalid output mode")
	}
	text := result.Content[0].(mcp.TextContent).Text
	if text != "output must be 'summary', 'raw', or 'text'" {
		t.Errorf("error text = %q, want %q", text, "output must be 'summary', 'raw', or 'text'")
	}
}

// keys returns the key names from a map for diagnostic output.
func keys(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

// TestHandleListEvents_MissingBothRequired validates that the handler returns
// a tool error when both required parameters are missing.
func TestHandleListEvents_MissingBothRequired(t *testing.T) {
	handler := NewHandleListEvents(graph.RetryConfig{}, 0, "UTC", "")
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{}

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when both required params are missing")
	}
}

// TestListEvents_DateToday_ExpandsToStartEndOfDay validates that date="today"
// expands to start-of-day (00:00:00) and end-of-day (23:59:59) in the
// configured timezone and makes a valid Graph API request.
func TestListEvents_DateToday_ExpandsToStartEndOfDay(t *testing.T) {
	var capturedStart, capturedEnd string
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedStart = r.URL.Query().Get("startDateTime")
		capturedEnd = r.URL.Query().Get("endDateTime")
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper
		w.Write([]byte(`{"value":[]}`))
	}))
	defer srv.Close()

	handler := NewHandleListEvents(graph.RetryConfig{}, 30*time.Second, "Europe/Stockholm", "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"date": "today",
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("expected success, got error: %s", text)
	}

	// Compute expected date boundaries in the configured timezone.
	loc, _ := time.LoadLocation("Europe/Stockholm")
	today := time.Now().In(loc)
	expectedDate := today.Format("2006-01-02")
	nextDay := time.Date(today.Year(), today.Month(), today.Day()+1, 0, 0, 0, 0, loc)
	expectedEndDate := nextDay.Format("2006-01-02")

	if capturedStart == "" || capturedEnd == "" {
		t.Fatal("expected Graph API call with startDateTime/endDateTime query params")
	}

	// Verify start is at 00:00:00 of today.
	if !strings.HasPrefix(capturedStart, expectedDate+"T00:00:00") {
		t.Errorf("startDateTime = %q, want prefix %q", capturedStart, expectedDate+"T00:00:00")
	}
	// Verify end is at 00:00:00 of next day (exclusive).
	if !strings.HasPrefix(capturedEnd, expectedEndDate+"T00:00:00") {
		t.Errorf("endDateTime = %q, want prefix %q", capturedEnd, expectedEndDate+"T00:00:00")
	}
}

// TestListEvents_DateISO_ExpandsToStartEndOfDay validates that a specific ISO
// date string expands to the correct start-of-day and end-of-day boundaries.
func TestListEvents_DateISO_ExpandsToStartEndOfDay(t *testing.T) {
	var capturedStart, capturedEnd string
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedStart = r.URL.Query().Get("startDateTime")
		capturedEnd = r.URL.Query().Get("endDateTime")
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper
		w.Write([]byte(`{"value":[]}`))
	}))
	defer srv.Close()

	handler := NewHandleListEvents(graph.RetryConfig{}, 30*time.Second, "America/New_York", "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"date": "2026-03-17",
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("expected success, got error: %s", text)
	}

	if capturedStart == "" || capturedEnd == "" {
		t.Fatal("expected Graph API call with startDateTime/endDateTime query params")
	}

	if capturedStart != "2026-03-17T00:00:00" {
		t.Errorf("startDateTime = %q, want %q", capturedStart, "2026-03-17T00:00:00")
	}
	if capturedEnd != "2026-03-18T00:00:00" {
		t.Errorf("endDateTime = %q, want %q", capturedEnd, "2026-03-18T00:00:00")
	}
}

// TestListEvents_DateAndExplicitRange_ExplicitWins validates that when both
// date and explicit start_datetime/end_datetime are provided, the explicit
// values take precedence (backward-compatible).
func TestListEvents_DateAndExplicitRange_ExplicitWins(t *testing.T) {
	var capturedStart, capturedEnd string
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedStart = r.URL.Query().Get("startDateTime")
		capturedEnd = r.URL.Query().Get("endDateTime")
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper
		w.Write([]byte(`{"value":[]}`))
	}))
	defer srv.Close()

	handler := NewHandleListEvents(graph.RetryConfig{}, 30*time.Second, "UTC", "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"date":           "today",
		"start_datetime": "2026-06-01T08:00:00Z",
		"end_datetime":   "2026-06-01T18:00:00Z",
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("expected success, got error: %s", text)
	}

	if capturedStart == "" || capturedEnd == "" {
		t.Fatal("expected Graph API call with startDateTime/endDateTime query params")
	}

	// Explicit values should be used, not today's date.
	if !strings.HasPrefix(capturedStart, "2026-06-01T08") {
		t.Errorf("startDateTime = %q, want prefix %q", capturedStart, "2026-06-01T08")
	}
	if !strings.HasPrefix(capturedEnd, "2026-06-01T18") {
		t.Errorf("endDateTime = %q, want prefix %q", capturedEnd, "2026-06-01T18")
	}
}

// TestListEvents_NoDateNoRange_ReturnsError validates that when neither date
// nor start_datetime/end_datetime are provided, a validation error is returned.
func TestListEvents_NoDateNoRange_ReturnsError(t *testing.T) {
	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()

	handler := NewHandleListEvents(graph.RetryConfig{}, 30*time.Second, "UTC", "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"max_results": float64(10),
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when neither date nor start/end provided")
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "start_datetime is required") {
		t.Errorf("error text should mention start_datetime requirement, got: %q", text)
	}
}

// TestListEvents_CreatedByMcpInResponse validates that when list_events returns
// events with the provenance extended property, the serialized response includes
// "createdByMcp": true.
func TestListEvents_CreatedByMcpInResponse(t *testing.T) {
	propID := graph.BuildProvenancePropertyID("com.github.desek.outlook-local-mcp.created")

	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper
		fmt.Fprintf(w, `{"value":[{"id":"evt-1","subject":"Tagged","start":{"dateTime":"2026-03-12T09:00:00","timeZone":"UTC"},"end":{"dateTime":"2026-03-12T10:00:00","timeZone":"UTC"},"singleValueExtendedProperties":[{"id":"%s","value":"true"}]}]}`, propID)
	}))
	defer srv.Close()

	handler := NewHandleListEvents(graph.RetryConfig{}, 30*time.Second, "UTC", propID)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"start_datetime": "2026-03-12T00:00:00Z",
		"end_datetime":   "2026-03-13T00:00:00Z",
		"output":         "summary",
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %v", result.Content[0].(mcp.TextContent).Text)
	}

	var events []map[string]any
	text := result.Content[0].(mcp.TextContent).Text
	if err := json.Unmarshal([]byte(text), &events); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	got, exists := events[0]["createdByMcp"]
	if !exists {
		t.Fatal("createdByMcp key not present in response")
	}
	if got != true {
		t.Errorf("createdByMcp = %v, want true", got)
	}
}

// TestExpandDateParam_Tomorrow validates that "tomorrow" resolves to the next
// day's start-of-day through midnight of the day after (exclusive end).
func TestExpandDateParam_Tomorrow(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Stockholm")
	tomorrow := time.Now().In(loc).AddDate(0, 0, 1)
	dayAfter := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day()+1, 0, 0, 0, 0, loc)
	expectedDate := tomorrow.Format("2006-01-02")
	expectedEnd := dayAfter.Format("2006-01-02") + "T00:00:00"

	start, end, err := expandDateParam("tomorrow", "Europe/Stockholm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if start != expectedDate+"T00:00:00" {
		t.Errorf("start = %q, want %q", start, expectedDate+"T00:00:00")
	}
	if end != expectedEnd {
		t.Errorf("end = %q, want %q", end, expectedEnd)
	}
}

// TestExpandDateParam_ThisWeek validates that "this_week" resolves to Monday
// 00:00:00 through the following Monday 00:00:00 (exclusive end) of the
// current ISO week.
func TestExpandDateParam_ThisWeek(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Stockholm")
	now := time.Now().In(loc)

	// Compute expected Monday of this week.
	weekday := now.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	daysSinceMonday := int(weekday) - 1
	monday := now.AddDate(0, 0, -daysSinceMonday)
	nextMonday := monday.AddDate(0, 0, 7)

	expectedStart := time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, loc).Format("2006-01-02T15:04:05")
	expectedEnd := time.Date(nextMonday.Year(), nextMonday.Month(), nextMonday.Day(), 0, 0, 0, 0, loc).Format("2006-01-02T15:04:05")

	start, end, err := expandDateParam("this_week", "Europe/Stockholm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if start != expectedStart {
		t.Errorf("start = %q, want %q", start, expectedStart)
	}
	if end != expectedEnd {
		t.Errorf("end = %q, want %q", end, expectedEnd)
	}
}

// TestExpandDateParam_NextWeek validates that "next_week" resolves to Monday
// 00:00:00 through the Monday after (00:00:00, exclusive end) of the following
// ISO week.
func TestExpandDateParam_NextWeek(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Stockholm")
	now := time.Now().In(loc)

	// Compute expected Monday of next week.
	weekday := now.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	daysSinceMonday := int(weekday) - 1
	nextMonday := now.AddDate(0, 0, -daysSinceMonday+7)
	mondayAfter := nextMonday.AddDate(0, 0, 7)

	expectedStart := time.Date(nextMonday.Year(), nextMonday.Month(), nextMonday.Day(), 0, 0, 0, 0, loc).Format("2006-01-02T15:04:05")
	expectedEnd := time.Date(mondayAfter.Year(), mondayAfter.Month(), mondayAfter.Day(), 0, 0, 0, 0, loc).Format("2006-01-02T15:04:05")

	start, end, err := expandDateParam("next_week", "Europe/Stockholm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if start != expectedStart {
		t.Errorf("start = %q, want %q", start, expectedStart)
	}
	if end != expectedEnd {
		t.Errorf("end = %q, want %q", end, expectedEnd)
	}
}

// TestExpandDateParam_Invalid validates that an unrecognized date string
// returns an error.
func TestExpandDateParam_Invalid(t *testing.T) {
	_, _, err := expandDateParam("yesterday", "UTC")
	if err == nil {
		t.Fatal("expected error for invalid date param 'yesterday'")
	}
	if !strings.Contains(err.Error(), "yesterday") {
		t.Errorf("error should mention the invalid value, got: %q", err.Error())
	}
}
