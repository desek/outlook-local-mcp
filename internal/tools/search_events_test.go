// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the search_events tool, including tool
// registration validation, parameter handling, default date range behavior,
// OData filter building, client-side substring matching on subject,
// client-side category filtering, and max_results enforcement.
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

// TestSearchEvents_ReadOnlyAnnotation validates that the NewSearchEventsTool has
// its ReadOnlyHint annotation set to true.
func TestSearchEvents_ReadOnlyAnnotation(t *testing.T) {
	tool := NewSearchEventsTool(false)
	annotations := tool.Annotations
	if annotations.ReadOnlyHint == nil || !*annotations.ReadOnlyHint {
		t.Error("expected ReadOnlyHint to be true")
	}
}

// TestSearchEvents_ToolName validates that the tool is registered with the
// correct name "calendar_search_events".
func TestSearchEvents_ToolName(t *testing.T) {
	tool := NewSearchEventsTool(false)
	if tool.Name != "calendar_search_events" {
		t.Errorf("tool name = %q, want %q", tool.Name, "calendar_search_events")
	}
}

// TestSearchEvents_HasAllParameters validates that the tool defines the expected
// 14 parameters (11 original + date + account + output).
func TestSearchEvents_HasAllParameters(t *testing.T) {
	tool := NewSearchEventsTool(false)
	expected := []string{
		"query", "date", "start_datetime", "end_datetime", "importance",
		"sensitivity", "is_all_day", "show_as", "is_cancelled",
		"categories", "max_results", "timezone", "account", "output",
	}
	props := tool.InputSchema.Properties
	if len(props) != len(expected) {
		t.Errorf("expected %d properties, got %d", len(expected), len(props))
	}
	for _, name := range expected {
		if _, ok := props[name]; !ok {
			t.Errorf("expected property %q to be defined", name)
		}
	}
}

// TestSearchEvents_NoRequiredParameters validates that all parameters are
// optional (no required parameters).
func TestSearchEvents_NoRequiredParameters(t *testing.T) {
	tool := NewSearchEventsTool(false)
	if len(tool.InputSchema.Required) != 0 {
		t.Errorf("expected no required parameters, got %v", tool.InputSchema.Required)
	}
}

// TestSearchEventsToolCanBeAddedToServer validates that the NewSearchEventsTool
// and its handler can be registered on an MCP server without error or panic.
func TestSearchEventsToolCanBeAddedToServer(t *testing.T) {
	s := server.NewMCPServer("test-server", "0.0.1",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)
	s.AddTool(NewSearchEventsTool(false), NewHandleSearchEvents(graph.RetryConfig{}, 0, "UTC", ""))
}

// TestSearchEvents_NoClientInContext validates that the handler returns a tool
// error when no Graph client is present in the context.
func TestSearchEvents_NoClientInContext(t *testing.T) {
	handler := NewHandleSearchEvents(graph.RetryConfig{}, 30*time.Second, "UTC", "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{}

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

// TestSearchEvents_DefaultDateRange validates that when start_datetime and
// end_datetime are not provided, the handler defaults to now and now+30 days.
func TestSearchEvents_DefaultDateRange(t *testing.T) {
	var capturedStartDT, capturedEndDT string
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedStartDT = r.URL.Query().Get("startDateTime")
		capturedEndDT = r.URL.Query().Get("endDateTime")
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper
		w.Write([]byte(`{"value":[]}`))
	}))
	defer srv.Close()

	handler := NewHandleSearchEvents(graph.RetryConfig{}, 30*time.Second, "UTC", "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %+v", result)
	}

	// Verify start is close to now.
	parsedStart, err := time.Parse(time.RFC3339, capturedStartDT)
	if err != nil {
		t.Fatalf("failed to parse captured start: %v", err)
	}
	if time.Since(parsedStart) > 5*time.Second {
		t.Errorf("start_datetime %v is not close to now", parsedStart)
	}

	// Verify end is close to now + 30 days.
	parsedEnd, err := time.Parse(time.RFC3339, capturedEndDT)
	if err != nil {
		t.Fatalf("failed to parse captured end: %v", err)
	}
	expected30Days := parsedStart.Add(30 * 24 * time.Hour)
	diff := parsedEnd.Sub(expected30Days)
	if diff < -5*time.Second || diff > 5*time.Second {
		t.Errorf("end_datetime %v is not 30 days from start %v", parsedEnd, parsedStart)
	}
}

// TestSearchEvents_FilterBuildImportance validates that providing the importance
// parameter generates the correct OData filter expression.
func TestSearchEvents_FilterBuildImportance(t *testing.T) {
	var capturedFilter string
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedFilter = r.URL.Query().Get("$filter")
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper
		w.Write([]byte(`{"value":[]}`))
	}))
	defer srv.Close()

	handler := NewHandleSearchEvents(graph.RetryConfig{}, 30*time.Second, "UTC", "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"start_datetime": "2026-03-12T00:00:00Z",
		"end_datetime":   "2026-03-13T00:00:00Z",
		"importance":     "high",
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %+v", result)
	}

	if capturedFilter != "importance eq 'high'" {
		t.Errorf("filter = %q, want %q", capturedFilter, "importance eq 'high'")
	}
}

// TestSearchEvents_FilterBuildComposite validates that multiple filter
// parameters are combined with " and " in the OData $filter string, and that
// the query parameter does NOT produce an OData filter (subject matching is
// always client-side).
func TestSearchEvents_FilterBuildComposite(t *testing.T) {
	var capturedFilter string
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedFilter = r.URL.Query().Get("$filter")
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper
		w.Write([]byte(`{"value":[]}`))
	}))
	defer srv.Close()

	handler := NewHandleSearchEvents(graph.RetryConfig{}, 30*time.Second, "UTC", "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"start_datetime": "2026-03-12T00:00:00Z",
		"end_datetime":   "2026-03-13T00:00:00Z",
		"importance":     "high",
		"sensitivity":    "private",
		"query":          "Team",
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %+v", result)
	}

	// Verify OData filters are present for importance and sensitivity.
	if !strings.Contains(capturedFilter, "importance eq 'high'") {
		t.Errorf("filter missing importance clause: %q", capturedFilter)
	}
	if !strings.Contains(capturedFilter, "sensitivity eq 'private'") {
		t.Errorf("filter missing sensitivity clause: %q", capturedFilter)
	}
	// Verify no subject filter in OData (client-side substring matching).
	if strings.Contains(capturedFilter, "startsWith") {
		t.Errorf("filter should not contain startsWith (subject matching is client-side): %q", capturedFilter)
	}
	// Verify the two OData filters are joined by " and ".
	if strings.Count(capturedFilter, " and ") != 1 {
		t.Errorf("expected 1 ' and ' join, got %d in: %q", strings.Count(capturedFilter, " and "), capturedFilter)
	}
}

// TestSearchEvents_SubstringMatch validates that client-side substring matching
// finds events where the query is a substring of the subject (not just a
// prefix). This verifies FR-13: case-insensitive substring matching.
func TestSearchEvents_SubstringMatch(t *testing.T) {
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify no subject filter in OData (FR-14).
		filter := r.URL.Query().Get("$filter")
		if strings.Contains(filter, "startsWith") {
			t.Error("OData filter should not contain startsWith")
		}

		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper
		w.Write([]byte(`{"value":[{"id":"1","subject":"Q2 Budget Review","start":{"dateTime":"2026-03-12T09:00:00","timeZone":"UTC"},"end":{"dateTime":"2026-03-12T10:00:00","timeZone":"UTC"}},{"id":"2","subject":"Lunch Break","start":{"dateTime":"2026-03-12T12:00:00","timeZone":"UTC"},"end":{"dateTime":"2026-03-12T13:00:00","timeZone":"UTC"}}]}`))
	}))
	defer srv.Close()

	handler := NewHandleSearchEvents(graph.RetryConfig{}, 30*time.Second, "UTC", "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"start_datetime": "2026-03-12T00:00:00Z",
		"end_datetime":   "2026-03-13T00:00:00Z",
		"query":          "budget",
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
	if events[0]["subject"] != "Q2 Budget Review" {
		t.Errorf("subject = %q, want %q", events[0]["subject"], "Q2 Budget Review")
	}
}

// TestSearchEvents_CategoryClientSide validates that client-side category
// filtering correctly matches events with any of the specified categories.
func TestSearchEvents_CategoryClientSide(t *testing.T) {
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper
		w.Write([]byte(`{"value":[{"id":"1","subject":"Work Event","categories":["Work"]},{"id":"2","subject":"Personal Event","categories":["Personal"]},{"id":"3","subject":"Both","categories":["Work","Personal"]},{"id":"4","subject":"Uncategorized","categories":[]}]}`))
	}))
	defer srv.Close()

	handler := NewHandleSearchEvents(graph.RetryConfig{}, 30*time.Second, "UTC", "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"start_datetime": "2026-03-12T00:00:00Z",
		"end_datetime":   "2026-03-13T00:00:00Z",
		"categories":     "Work,Personal",
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

	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
}

// TestSearchEvents_MaxResultsDefault validates that without a max_results
// parameter, at most 25 events are returned.
func TestSearchEvents_MaxResultsDefault(t *testing.T) {
	var eventsJSON strings.Builder
	eventsJSON.WriteString(`{"value":[`)
	for i := range 30 {
		if i > 0 {
			eventsJSON.WriteString(",")
		}
		eventsJSON.WriteString(fmt.Sprintf(`{"id":"%d","subject":"Event %d"}`, i, i))
	}
	eventsJSON.WriteString(`]}`)

	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper
		w.Write([]byte(eventsJSON.String()))
	}))
	defer srv.Close()

	handler := NewHandleSearchEvents(graph.RetryConfig{}, 30*time.Second, "UTC", "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"start_datetime": "2026-03-12T00:00:00Z",
		"end_datetime":   "2026-04-12T00:00:00Z",
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

	if len(events) > 25 {
		t.Errorf("expected at most 25 events, got %d", len(events))
	}
}

// TestSearchEvents_MaxResultsClamp validates that max_results values outside
// the 1-100 range are clamped.
func TestSearchEvents_MaxResultsClamp(t *testing.T) {
	var capturedTop string
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTop = r.URL.Query().Get("$top")
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper
		w.Write([]byte(`{"value":[]}`))
	}))
	defer srv.Close()

	handler := NewHandleSearchEvents(graph.RetryConfig{}, 30*time.Second, "UTC", "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"start_datetime": "2026-03-12T00:00:00Z",
		"end_datetime":   "2026-03-13T00:00:00Z",
		"max_results":    float64(150),
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %+v", result)
	}

	if capturedTop != "100" {
		t.Errorf("$top = %q, want %q", capturedTop, "100")
	}
}

// TestSearchEvents_NoFilters validates that when only date range is provided
// (no other filters), no $filter parameter is sent to the Graph API.
func TestSearchEvents_NoFilters(t *testing.T) {
	var capturedFilter string
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedFilter = r.URL.Query().Get("$filter")
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper
		w.Write([]byte(`{"value":[]}`))
	}))
	defer srv.Close()

	handler := NewHandleSearchEvents(graph.RetryConfig{}, 30*time.Second, "UTC", "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"start_datetime": "2026-03-12T00:00:00Z",
		"end_datetime":   "2026-03-13T00:00:00Z",
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %+v", result)
	}

	if capturedFilter != "" {
		t.Errorf("expected no $filter, got %q", capturedFilter)
	}
}

// TestBuildClientFilter validates the client-side filter function with
// case-insensitive category matching and subject substring matching.
func TestBuildClientFilter(t *testing.T) {
	events := []map[string]any{
		{"subject": "Work Event", "categories": []string{"Work"}},
		{"subject": "Personal Event", "categories": []string{"Personal"}},
		{"subject": "Both", "categories": []string{"Work", "Personal"}},
		{"subject": "Other", "categories": []string{"Other"}},
		{"subject": "Empty", "categories": []string{}},
	}

	// Categories-only filter.
	match := buildClientFilter("", "work,personal")
	var result []map[string]any
	for _, e := range events {
		if match(e) {
			result = append(result, e)
		}
	}
	if len(result) != 3 {
		t.Errorf("categories filter: expected 3 events, got %d", len(result))
	}

	// Query-only filter.
	match = buildClientFilter("personal", "")
	result = nil
	for _, e := range events {
		if match(e) {
			result = append(result, e)
		}
	}
	if len(result) != 1 {
		t.Errorf("query filter: expected 1 event, got %d", len(result))
	}

	// Combined filter.
	match = buildClientFilter("event", "work")
	result = nil
	for _, e := range events {
		if match(e) {
			result = append(result, e)
		}
	}
	if len(result) != 1 {
		t.Errorf("combined filter: expected 1 event (Work Event), got %d", len(result))
	}

	// No filters returns nil.
	if buildClientFilter("", "") != nil {
		t.Error("expected nil when no filters active")
	}
}

// TestSearchEvents_QueryScansBeyondPageSize validates that client-side query
// filtering scans beyond the initial maxResults to find matching events that
// appear later in the CalendarView result set.
func TestSearchEvents_QueryScansBeyondPageSize(t *testing.T) {
	// Build 30 events where only the last one matches the query.
	var eventsJSON strings.Builder
	eventsJSON.WriteString(`{"value":[`)
	for i := range 30 {
		if i > 0 {
			eventsJSON.WriteString(",")
		}
		subject := fmt.Sprintf("Event %d", i)
		if i == 29 {
			subject = "Target Match"
		}
		fmt.Fprintf(&eventsJSON,
			`{"id":"%d","subject":"%s","start":{"dateTime":"2026-03-12T%02d:00:00","timeZone":"UTC"},"end":{"dateTime":"2026-03-12T%02d:30:00","timeZone":"UTC"}}`,
			i, subject, i%24, i%24)
	}
	eventsJSON.WriteString(`]}`)

	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper
		w.Write([]byte(eventsJSON.String()))
	}))
	defer srv.Close()

	handler := NewHandleSearchEvents(graph.RetryConfig{}, 30*time.Second, "UTC", "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"start_datetime": "2026-03-12T00:00:00Z",
		"end_datetime":   "2026-03-13T00:00:00Z",
		"query":          "Target",
		"max_results":    float64(25),
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
	if events[0]["subject"] != "Target Match" {
		t.Errorf("subject = %q, want %q", events[0]["subject"], "Target Match")
	}
}

// TestSearchEvents_CreatedByMcpFilter validates that when created_by_mcp=true
// is provided and provenance tagging is enabled, the OData $filter includes the
// extended property clause for server-side filtering of MCP-created events.
func TestSearchEvents_CreatedByMcpFilter(t *testing.T) {
	var capturedFilter string
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedFilter = r.URL.Query().Get("$filter")
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper
		w.Write([]byte(`{"value":[]}`))
	}))
	defer srv.Close()

	propID := graph.BuildProvenancePropertyID("com.github.desek.outlook-local-mcp.created")
	handler := NewHandleSearchEvents(graph.RetryConfig{}, 30*time.Second, "UTC", propID)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"start_datetime": "2026-03-12T00:00:00Z",
		"end_datetime":   "2026-03-13T00:00:00Z",
		"created_by_mcp": true,
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %+v", result)
	}

	expectedClause := fmt.Sprintf(
		"singleValueExtendedProperties/Any(ep: ep/id eq '%s' and ep/value eq 'true')",
		propID,
	)
	if !strings.Contains(capturedFilter, expectedClause) {
		t.Errorf("filter missing provenance clause:\ngot:  %q\nwant: %q", capturedFilter, expectedClause)
	}
}

// TestSearchEvents_CreatedByMcpFilterDisabled validates that created_by_mcp=true
// is ignored when provenance tagging is disabled (empty provenancePropertyID).
func TestSearchEvents_CreatedByMcpFilterDisabled(t *testing.T) {
	var capturedFilter string
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedFilter = r.URL.Query().Get("$filter")
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper
		w.Write([]byte(`{"value":[]}`))
	}))
	defer srv.Close()

	handler := NewHandleSearchEvents(graph.RetryConfig{}, 30*time.Second, "UTC", "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"start_datetime": "2026-03-12T00:00:00Z",
		"end_datetime":   "2026-03-13T00:00:00Z",
		"created_by_mcp": true,
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %+v", result)
	}

	if capturedFilter != "" {
		t.Errorf("expected no $filter when provenance disabled, got %q", capturedFilter)
	}
}

// TestSearchEvents_CreatedByMcpParam validates that NewSearchEventsTool includes
// the created_by_mcp parameter when provenance is enabled, and omits it when
// disabled.
func TestSearchEvents_CreatedByMcpParam(t *testing.T) {
	toolEnabled := NewSearchEventsTool(true)
	if _, ok := toolEnabled.InputSchema.Properties["created_by_mcp"]; !ok {
		t.Error("expected created_by_mcp parameter when provenance enabled")
	}

	toolDisabled := NewSearchEventsTool(false)
	if _, ok := toolDisabled.InputSchema.Properties["created_by_mcp"]; ok {
		t.Error("expected no created_by_mcp parameter when provenance disabled")
	}
}

// TestSearchEvents_CreatedByMcpInResponse validates that when search_events
// returns events with the provenance extended property, the serialized response
// includes "createdByMcp": true.
func TestSearchEvents_CreatedByMcpInResponse(t *testing.T) {
	propID := graph.BuildProvenancePropertyID("com.github.desek.outlook-local-mcp.created")

	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper
		fmt.Fprintf(w, `{"value":[{"id":"evt-1","subject":"Tagged","start":{"dateTime":"2026-03-12T09:00:00","timeZone":"UTC"},"end":{"dateTime":"2026-03-12T10:00:00","timeZone":"UTC"},"singleValueExtendedProperties":[{"id":"%s","value":"true"}]}]}`, propID)
	}))
	defer srv.Close()

	handler := NewHandleSearchEvents(graph.RetryConfig{}, 30*time.Second, "UTC", propID)
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

// TestSearchEvents_DateParam validates that the date convenience parameter
// expands correctly and is used when start_datetime/end_datetime are omitted.
func TestSearchEvents_DateParam(t *testing.T) {
	var capturedStart, capturedEnd string
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedStart = r.URL.Query().Get("startDateTime")
		capturedEnd = r.URL.Query().Get("endDateTime")
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper
		w.Write([]byte(`{"value":[]}`))
	}))
	defer srv.Close()

	handler := NewHandleSearchEvents(graph.RetryConfig{}, 30*time.Second, "Europe/Stockholm", "")
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

	if !strings.HasPrefix(capturedStart, expectedDate+"T00:00:00") {
		t.Errorf("startDateTime = %q, want prefix %q", capturedStart, expectedDate+"T00:00:00")
	}
	if !strings.HasPrefix(capturedEnd, expectedEndDate+"T00:00:00") {
		t.Errorf("endDateTime = %q, want prefix %q", capturedEnd, expectedEndDate+"T00:00:00")
	}
}
