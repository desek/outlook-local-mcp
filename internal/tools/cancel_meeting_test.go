package tools

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
)

// TestCancelEvent_Success_WithComment validates that HandleCancelEvent returns
// a plain text confirmation when the Graph API returns 202 Accepted and a
// comment is provided.
func TestCancelEvent_Success_WithComment(t *testing.T) {
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	handler := HandleCancelEvent(graph.RetryConfig{}, 30*time.Second)

	req := mcp.CallToolRequest{}
	req.Params.Name = "calendar_cancel_meeting"
	req.Params.Arguments = map[string]any{
		"event_id": "AAMkAGTest456",
		"comment":  "Meeting postponed to next week",
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %+v", result)
	}

	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "Event cancelled: AAMkAGTest456") {
		t.Errorf("response should contain event cancelled line, got: %q", text)
	}
	if !strings.Contains(text, "Cancellation message sent to all attendees.") {
		t.Errorf("response should contain cancellation message, got: %q", text)
	}
}

// TestCancelEvent_Success_WithoutComment validates that HandleCancelEvent
// returns a plain text confirmation when no comment is provided.
func TestCancelEvent_Success_WithoutComment(t *testing.T) {
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	handler := HandleCancelEvent(graph.RetryConfig{}, 30*time.Second)

	req := mcp.CallToolRequest{}
	req.Params.Name = "calendar_cancel_meeting"
	req.Params.Arguments = map[string]any{
		"event_id": "AAMkAGTest789",
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %+v", result)
	}

	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "Event cancelled: AAMkAGTest789") {
		t.Errorf("response should contain event cancelled line, got: %q", text)
	}
}

// TestCancelEvent_NonOrganizer validates that HandleCancelEvent returns an MCP
// tool error when the Graph API returns 400 (ErrorAccessDenied).
func TestCancelEvent_NonOrganizer(t *testing.T) {
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		//nolint:errcheck // test helper, write error is non-critical
		w.Write([]byte(`{"error":{"code":"ErrorAccessDenied","message":"Only the organizer can cancel a meeting."}}`))
	}))
	defer srv.Close()

	handler := HandleCancelEvent(graph.RetryConfig{}, 30*time.Second)

	req := mcp.CallToolRequest{}
	req.Params.Name = "calendar_cancel_meeting"
	req.Params.Arguments = map[string]any{
		"event_id": "AAMkAGTest456",
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result, got success")
	}
}

// TestCancelEvent_NotFound validates that HandleCancelEvent returns an MCP
// tool error when the Graph API returns 404 Not Found.
func TestCancelEvent_NotFound(t *testing.T) {
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		//nolint:errcheck // test helper, write error is non-critical
		w.Write([]byte(`{"error":{"code":"ErrorItemNotFound","message":"The specified object was not found in the store."}}`))
	}))
	defer srv.Close()

	handler := HandleCancelEvent(graph.RetryConfig{}, 30*time.Second)

	req := mcp.CallToolRequest{}
	req.Params.Name = "calendar_cancel_meeting"
	req.Params.Arguments = map[string]any{
		"event_id": "nonexistent-id",
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result, got success")
	}
}

// TestCancelEvent_MissingEventId validates that HandleCancelEvent returns an
// MCP tool error when the required event_id parameter is missing.
func TestCancelEvent_MissingEventId(t *testing.T) {
	handler := HandleCancelEvent(graph.RetryConfig{}, 30*time.Second)

	req := mcp.CallToolRequest{}
	req.Params.Name = "calendar_cancel_meeting"
	req.Params.Arguments = map[string]any{}

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for missing event_id")
	}

	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "missing required parameter: event_id") {
		t.Errorf("error text missing expected prefix, got: %q", text)
	}
	if !strings.Contains(text, "calendar_list_events or calendar_search_events") {
		t.Errorf("error text missing recovery hint, got: %q", text)
	}
}

// TestCancelEvent_NoClientInContext validates that HandleCancelEvent returns a
// tool error when no Graph client is present in the context.
func TestCancelEvent_NoClientInContext(t *testing.T) {
	handler := HandleCancelEvent(graph.RetryConfig{}, 30*time.Second)

	req := mcp.CallToolRequest{}
	req.Params.Name = "calendar_cancel_meeting"
	req.Params.Arguments = map[string]any{
		"event_id": "AAMkAGTest456",
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

// TestCancelEvent_Timeout validates that HandleCancelEvent returns a timeout
// error message when the Graph API call exceeds the configured deadline.
func TestCancelEvent_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	ctx = auth.WithGraphClient(ctx, client)

	handler := HandleCancelEvent(graph.RetryConfig{}, 30*time.Second)

	req := mcp.CallToolRequest{}
	req.Params.Name = "calendar_cancel_meeting"
	req.Params.Arguments = map[string]any{
		"event_id": "AAMkAGTest456",
	}

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for timed-out request")
	}

	text := result.Content[0].(mcp.TextContent).Text
	want := "request timed out after 30s"
	if text != want {
		t.Errorf("error text = %q, want %q", text, want)
	}
}

// TestNewCancelMeetingTool validates that the cancel_meeting tool definition has
// the correct name, parameters, and description content.
func TestNewCancelMeetingTool(t *testing.T) {
	tool := NewCancelMeetingTool()

	if tool.Name != "calendar_cancel_meeting" {
		t.Errorf("name = %q, want %q", tool.Name, "calendar_cancel_meeting")
	}

	desc := tool.Description
	if desc == "" {
		t.Fatal("expected non-empty description")
	}

	props := tool.InputSchema.Properties
	if _, exists := props["event_id"]; !exists {
		t.Error("expected event_id in properties")
	}
	if _, exists := props["comment"]; !exists {
		t.Error("expected comment in properties")
	}
	if _, exists := props["account"]; !exists {
		t.Error("expected account in properties")
	}

	required := tool.InputSchema.Required
	eventIDRequired := false
	commentRequired := false
	for _, r := range required {
		if r == "event_id" {
			eventIDRequired = true
		}
		if r == "comment" {
			commentRequired = true
		}
	}
	if !eventIDRequired {
		t.Error("event_id should be required")
	}
	if commentRequired {
		t.Error("comment should not be required")
	}
}
