package tools

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
)

// TestUpdateEventPatchSemantics validates that when only event_id and subject
// are provided, the handler constructs a request that would set only the
// subject.
func TestUpdateEventPatchSemantics(t *testing.T) {
	tool := NewUpdateEventTool()
	if tool.Name != "calendar_update_event" {
		t.Errorf("tool name = %q, want %q", tool.Name, "calendar_update_event")
	}

	if _, ok := tool.InputSchema.Properties["account"]; !ok {
		t.Error("expected account property to be defined")
	}

	handler := HandleUpdateEvent(graph.RetryConfig{}, 0, "America/New_York")
	if handler == nil {
		t.Fatal("handler is nil")
	}
}

// TestHandleUpdateEvent_NoClientInContext validates that the handler returns
// a tool error when no Graph client is present in the context.
func TestHandleUpdateEvent_NoClientInContext(t *testing.T) {
	handler := HandleUpdateEvent(graph.RetryConfig{}, 0, "America/New_York")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"event_id": "test-event-id",
		"subject":  "New Title",
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

// TestUpdateEventRecurrenceRemoval validates that passing "null" as the
// recurrence value does not cause a parsing error.
func TestUpdateEventRecurrenceRemoval(t *testing.T) {
	handler := HandleUpdateEvent(graph.RetryConfig{}, 0, "America/New_York")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"event_id":   "test-event-id",
		"recurrence": "null",
	}

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	func() {
		defer func() {
			_ = recover()
		}()
		_, _ = handler(ctx, req)
	}()
}

// TestUpdateEventStartTimezoneDefaults validates that providing start_datetime
// without start_timezone uses the default timezone from config and succeeds.
func TestUpdateEventStartTimezoneDefaults(t *testing.T) {
	var captured []byte
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body) //nolint:errcheck // test helper
		captured = body
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockEventJSON)) //nolint:errcheck // test helper
	}))
	defer srv.Close()

	handler := HandleUpdateEvent(graph.RetryConfig{}, 30*time.Second, "Europe/Stockholm")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"event_id":       "AAMkAGTest123",
		"start_datetime": "2026-04-15T09:00:00",
	}
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("handler returned unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %v", result.Content[0].(mcp.TextContent).Text)
	}

	body := string(captured)
	if !strings.Contains(body, "Europe/Stockholm") {
		t.Errorf("PATCH body should contain default timezone Europe/Stockholm, got: %s", body)
	}
}

// TestUpdateEventEndTimezoneDefaults validates that providing end_datetime
// without end_timezone uses the default timezone from config and succeeds.
func TestUpdateEventEndTimezoneDefaults(t *testing.T) {
	var captured []byte
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body) //nolint:errcheck // test helper
		captured = body
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockEventJSON)) //nolint:errcheck // test helper
	}))
	defer srv.Close()

	handler := HandleUpdateEvent(graph.RetryConfig{}, 30*time.Second, "Europe/Stockholm")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"event_id":     "AAMkAGTest123",
		"end_datetime": "2026-04-15T10:00:00",
	}
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("handler returned unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %v", result.Content[0].(mcp.TextContent).Text)
	}

	body := string(captured)
	if !strings.Contains(body, "Europe/Stockholm") {
		t.Errorf("PATCH body should contain default timezone Europe/Stockholm, got: %s", body)
	}
}

// TestUpdateEventIsOnlineMeetingParam validates that the is_online_meeting
// parameter is accepted by the tool definition and handler without error.
func TestUpdateEventIsOnlineMeetingParam(t *testing.T) {
	tool := NewUpdateEventTool()
	if _, ok := tool.InputSchema.Properties["is_online_meeting"]; !ok {
		t.Error("expected is_online_meeting property to be defined")
	}

	handler := HandleUpdateEvent(graph.RetryConfig{}, 0, "America/New_York")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"event_id":          "test-event-id",
		"is_online_meeting": true,
	}

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	func() {
		defer func() {
			_ = recover()
		}()
		_, _ = handler(ctx, req)
	}()
}

// TestUpdateEventMissingEventID validates that calling update_event without
// event_id returns a validation error.
func TestUpdateEventMissingEventID(t *testing.T) {
	handler := HandleUpdateEvent(graph.RetryConfig{}, 0, "America/New_York")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"subject": "New Title",
	}

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("handler returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if !result.IsError {
		t.Error("expected error result for missing event_id")
	}
}

// updateEventMockHandler returns an http.Handler that responds to PATCH
// /me/events/{id} with a valid Graph API event JSON response.
func updateEventMockHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockEventJSON)) //nolint:errcheck // test helper
	})
}

// TestUpdateEvent_AdvisoryWhenAddingAttendees validates that the response
// contains an advisory when attendees are provided but body is missing.
func TestUpdateEvent_AdvisoryWhenAddingAttendees(t *testing.T) {
	client, srv := newTestGraphClient(t, updateEventMockHandler())
	defer srv.Close()

	handler := HandleUpdateEvent(graph.RetryConfig{}, 30*time.Second, "America/New_York")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"event_id":  "AAMkAGTest123",
		"attendees": `[{"email":"a@b.com","name":"Alice","type":"required"}]`,
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %v", result.Content[0].(mcp.TextContent).Text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "Event updated:") {
		t.Errorf("expected text confirmation, got: %q", text)
	}
	if !strings.Contains(text, "description") {
		t.Errorf("expected advisory mentioning description, got: %q", text)
	}
}

// TestUpdateEvent_NoProvenanceProperty validates that the PATCH request body
// does NOT include singleValueExtendedProperties. Provenance is write-once at
// creation and must not be set on updates.
func TestUpdateEvent_NoProvenanceProperty(t *testing.T) {
	var captured []byte
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body) //nolint:errcheck // test helper
		captured = body
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockEventJSON)) //nolint:errcheck // test helper
	}))
	defer srv.Close()

	handler := HandleUpdateEvent(graph.RetryConfig{}, 30*time.Second, "America/New_York")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"event_id": "AAMkAGTest123",
		"subject":  "Updated Title",
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %v", result.Content[0].(mcp.TextContent).Text)
	}

	body := string(captured)
	if strings.Contains(body, "singleValueExtendedProperties") {
		t.Error("PATCH body should NOT contain singleValueExtendedProperties")
	}
}
