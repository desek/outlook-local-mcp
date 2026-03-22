// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the get_free_busy tool, including tool
// registration validation, required parameter handling, free event filtering,
// response schema validation, and empty range behavior.
package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// TestGetFreeBusy_ReadOnlyAnnotation validates that the NewGetFreeBusyTool has
// its ReadOnlyHint annotation set to true.
func TestGetFreeBusy_ReadOnlyAnnotation(t *testing.T) {
	tool := NewGetFreeBusyTool()
	annotations := tool.Annotations
	if annotations.ReadOnlyHint == nil || !*annotations.ReadOnlyHint {
		t.Error("expected ReadOnlyHint to be true")
	}
}

// TestGetFreeBusy_ToolName validates that the tool is registered with the
// correct name "calendar_get_free_busy".
func TestGetFreeBusy_ToolName(t *testing.T) {
	tool := NewGetFreeBusyTool()
	if tool.Name != "calendar_get_free_busy" {
		t.Errorf("tool name = %q, want %q", tool.Name, "calendar_get_free_busy")
	}
}

// TestGetFreeBusy_NoRequiredParams validates that no parameters are marked as
// required in the tool schema, since the date convenience parameter can
// substitute for start_datetime and end_datetime.
func TestGetFreeBusy_NoRequiredParams(t *testing.T) {
	tool := NewGetFreeBusyTool()
	if len(tool.InputSchema.Required) != 0 {
		t.Errorf("expected 0 required parameters, got %d: %v", len(tool.InputSchema.Required), tool.InputSchema.Required)
	}

	// Verify all expected parameters exist.
	expected := []string{"date", "start_datetime", "end_datetime", "timezone", "account", "output"}
	for _, name := range expected {
		if _, ok := tool.InputSchema.Properties[name]; !ok {
			t.Errorf("expected property %q to be defined", name)
		}
	}
}

// TestGetFreeBusyToolCanBeAddedToServer validates that the NewGetFreeBusyTool
// and its handler can be registered on an MCP server without error or panic.
func TestGetFreeBusyToolCanBeAddedToServer(t *testing.T) {
	s := server.NewMCPServer("test-server", "0.0.1",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)
	s.AddTool(NewGetFreeBusyTool(), NewHandleGetFreeBusy(graph.RetryConfig{}, 0, "UTC"))
}

// TestGetFreeBusy_NoClientInContext validates that the handler returns a tool
// error when no Graph client is present in the context.
func TestGetFreeBusy_NoClientInContext(t *testing.T) {
	handler := NewHandleGetFreeBusy(graph.RetryConfig{}, 30*time.Second, "UTC")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"start_datetime": "2026-03-12T00:00:00Z",
		"end_datetime":   "2026-03-13T00:00:00Z",
	}

	result, err := handler(context.Background(), req)
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

// TestGetFreeBusy_MissingStartDatetime validates that the handler returns a
// tool error when start_datetime is not provided.
func TestGetFreeBusy_MissingStartDatetime(t *testing.T) {
	handler := NewHandleGetFreeBusy(graph.RetryConfig{}, 0, "UTC")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"end_datetime": "2026-03-13T00:00:00Z",
	}

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when start_datetime is missing")
	}
}

// TestGetFreeBusy_MissingEndDatetime validates that the handler returns a
// tool error when end_datetime is not provided.
func TestGetFreeBusy_MissingEndDatetime(t *testing.T) {
	handler := NewHandleGetFreeBusy(graph.RetryConfig{}, 0, "UTC")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"start_datetime": "2026-03-12T00:00:00Z",
	}

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when end_datetime is missing")
	}
}

// TestGetFreeBusy_FiltersOutFree validates that events with showAs="free" are
// excluded from the busyPeriods array.
func TestGetFreeBusy_FiltersOutFree(t *testing.T) {
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper
		w.Write([]byte(`{"value":[
			{"id":"1","subject":"Free Event","showAs":"free","start":{"dateTime":"2026-03-12T09:00:00","timeZone":"UTC"},"end":{"dateTime":"2026-03-12T10:00:00","timeZone":"UTC"}},
			{"id":"2","subject":"Busy Meeting","showAs":"busy","start":{"dateTime":"2026-03-12T10:00:00","timeZone":"UTC"},"end":{"dateTime":"2026-03-12T11:00:00","timeZone":"UTC"}},
			{"id":"3","subject":"Tentative","showAs":"tentative","start":{"dateTime":"2026-03-12T11:00:00","timeZone":"UTC"},"end":{"dateTime":"2026-03-12T12:00:00","timeZone":"UTC"}},
			{"id":"4","subject":"Out of Office","showAs":"oof","start":{"dateTime":"2026-03-12T13:00:00","timeZone":"UTC"},"end":{"dateTime":"2026-03-12T14:00:00","timeZone":"UTC"}}
		]}`))
	}))
	defer srv.Close()

	handler := NewHandleGetFreeBusy(graph.RetryConfig{}, 30*time.Second, "UTC")
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

	var resp FreeBusyResponse
	text := result.Content[0].(mcp.TextContent).Text
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(resp.BusyPeriods) != 3 {
		t.Fatalf("expected 3 busy periods, got %d", len(resp.BusyPeriods))
	}

	for _, bp := range resp.BusyPeriods {
		if bp.Status == "free" {
			t.Errorf("unexpected free event in busy periods: %+v", bp)
		}
	}
}

// TestGetFreeBusy_ResponseSchema validates that the response contains timeRange
// and busyPeriods fields with the correct structure.
func TestGetFreeBusy_ResponseSchema(t *testing.T) {
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper
		w.Write([]byte(`{"value":[{"id":"1","subject":"Meeting","showAs":"busy","start":{"dateTime":"2026-03-12T10:00:00","timeZone":"UTC"},"end":{"dateTime":"2026-03-12T11:00:00","timeZone":"UTC"}}]}`))
	}))
	defer srv.Close()

	handler := NewHandleGetFreeBusy(graph.RetryConfig{}, 30*time.Second, "UTC")
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

	var parsed map[string]any
	text := result.Content[0].(mcp.TextContent).Text
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	timeRange, ok := parsed["timeRange"].(map[string]any)
	if !ok {
		t.Fatal("expected timeRange in response")
	}
	if timeRange["start"] != "2026-03-12T00:00:00Z" {
		t.Errorf("timeRange.start = %q, want %q", timeRange["start"], "2026-03-12T00:00:00Z")
	}
	if timeRange["end"] != "2026-03-13T00:00:00Z" {
		t.Errorf("timeRange.end = %q, want %q", timeRange["end"], "2026-03-13T00:00:00Z")
	}

	periods, ok := parsed["busyPeriods"].([]any)
	if !ok {
		t.Fatal("expected busyPeriods array in response")
	}
	if len(periods) != 1 {
		t.Fatalf("expected 1 busy period, got %d", len(periods))
	}

	period := periods[0].(map[string]any)
	for _, field := range []string{"start", "end", "status", "subject"} {
		if _, exists := period[field]; !exists {
			t.Errorf("expected %q field in busy period", field)
		}
	}
}

// TestGetFreeBusy_EmptyRange validates that when all events have showAs="free",
// the busyPeriods array is empty (not nil).
func TestGetFreeBusy_EmptyRange(t *testing.T) {
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper
		w.Write([]byte(`{"value":[{"id":"1","subject":"Free Time","showAs":"free","start":{"dateTime":"2026-03-12T09:00:00","timeZone":"UTC"},"end":{"dateTime":"2026-03-12T10:00:00","timeZone":"UTC"}}]}`))
	}))
	defer srv.Close()

	handler := NewHandleGetFreeBusy(graph.RetryConfig{}, 30*time.Second, "UTC")
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

	var resp FreeBusyResponse
	text := result.Content[0].(mcp.TextContent).Text
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.BusyPeriods == nil {
		t.Fatal("expected non-nil busyPeriods")
	}
	if len(resp.BusyPeriods) != 0 {
		t.Errorf("expected 0 busy periods, got %d", len(resp.BusyPeriods))
	}
}

// TestGetFreeBusy_DateParam validates that the date convenience parameter
// expands correctly and allows omitting start_datetime/end_datetime.
func TestGetFreeBusy_DateParam(t *testing.T) {
	var capturedStart, capturedEnd string
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedStart = r.URL.Query().Get("startDateTime")
		capturedEnd = r.URL.Query().Get("endDateTime")
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper
		w.Write([]byte(`{"value":[]}`))
	}))
	defer srv.Close()

	handler := NewHandleGetFreeBusy(graph.RetryConfig{}, 30*time.Second, "Europe/Stockholm")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"date": "tomorrow",
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
	tomorrow := time.Now().In(loc).AddDate(0, 0, 1)
	expectedDate := tomorrow.Format("2006-01-02")
	nextDay := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day()+1, 0, 0, 0, 0, loc)
	expectedEndDate := nextDay.Format("2006-01-02")

	if capturedStart == "" || capturedEnd == "" {
		t.Fatal("expected Graph API call with startDateTime/endDateTime query params")
	}

	if !strings.HasPrefix(capturedStart, expectedDate+"T00:00:00") {
		t.Errorf("startDateTime = %q, want prefix %q", capturedStart, expectedDate+"T00:00:00")
	}
	if !strings.HasPrefix(capturedEnd, expectedEndDate+"T00:00:00") {
		t.Errorf("endDateTime = %q, want prefix %q", capturedEnd, expectedEndDate+"T00:00:00")
	}
}

// TestGetFreeBusy_MissingBothDateAndDatetimes validates that the handler returns
// a tool error when neither date nor start_datetime/end_datetime are provided.
func TestGetFreeBusy_MissingBothDateAndDatetimes(t *testing.T) {
	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()

	handler := NewHandleGetFreeBusy(graph.RetryConfig{}, 30*time.Second, "UTC")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{}

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
