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

// TestDeleteEvent_Success validates that HandleDeleteEvent returns a plain text
// confirmation when the Graph API returns 204 No Content.
func TestDeleteEvent_Success(t *testing.T) {
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	handler := HandleDeleteEvent(graph.RetryConfig{}, 30*time.Second)

	req := mcp.CallToolRequest{}
	req.Params.Name = "calendar_delete_event"
	req.Params.Arguments = map[string]any{
		"event_id": "AAMkAGTest123",
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
	if !strings.Contains(text, "Event deleted: AAMkAGTest123") {
		t.Errorf("response should contain event deleted line, got: %q", text)
	}
	if !strings.Contains(text, "Cancellation notices were sent to attendees if applicable.") {
		t.Errorf("response should contain cancellation message, got: %q", text)
	}
}

// TestDeleteEvent_NotFound validates that HandleDeleteEvent returns an MCP
// tool error when the Graph API returns 404 Not Found.
func TestDeleteEvent_NotFound(t *testing.T) {
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		//nolint:errcheck // test helper, write error is non-critical
		w.Write([]byte(`{"error":{"code":"ErrorItemNotFound","message":"The specified object was not found in the store."}}`))
	}))
	defer srv.Close()

	handler := HandleDeleteEvent(graph.RetryConfig{}, 30*time.Second)

	req := mcp.CallToolRequest{}
	req.Params.Name = "calendar_delete_event"
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

// TestDeleteEvent_MissingEventId validates that HandleDeleteEvent returns an
// MCP tool error when the required event_id parameter is missing.
func TestDeleteEvent_MissingEventId(t *testing.T) {
	handler := HandleDeleteEvent(graph.RetryConfig{}, 30*time.Second)

	req := mcp.CallToolRequest{}
	req.Params.Name = "calendar_delete_event"
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

// TestDeleteEvent_NoClientInContext validates that HandleDeleteEvent returns a
// tool error when no Graph client is present in the context.
func TestDeleteEvent_NoClientInContext(t *testing.T) {
	handler := HandleDeleteEvent(graph.RetryConfig{}, 30*time.Second)

	req := mcp.CallToolRequest{}
	req.Params.Name = "calendar_delete_event"
	req.Params.Arguments = map[string]any{
		"event_id": "AAMkAGTest123",
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

// TestDeleteEvent_Timeout validates that HandleDeleteEvent returns a timeout
// error message when the Graph API call exceeds the configured deadline.
func TestDeleteEvent_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	ctx = auth.WithGraphClient(ctx, client)

	handler := HandleDeleteEvent(graph.RetryConfig{}, 30*time.Second)

	req := mcp.CallToolRequest{}
	req.Params.Name = "calendar_delete_event"
	req.Params.Arguments = map[string]any{
		"event_id": "AAMkAGTest123",
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

// TestNewDeleteEventTool validates that the delete_event tool definition has
// the correct name, parameters, and description content.
func TestNewDeleteEventTool(t *testing.T) {
	tool := NewDeleteEventTool()

	if tool.Name != "calendar_delete_event" {
		t.Errorf("name = %q, want %q", tool.Name, "calendar_delete_event")
	}

	desc := tool.Description
	if desc == "" {
		t.Fatal("expected non-empty description")
	}

	props := tool.InputSchema.Properties
	if _, exists := props["event_id"]; !exists {
		t.Error("expected event_id in properties")
	}
	if _, exists := props["account"]; !exists {
		t.Error("expected account in properties")
	}

	required := tool.InputSchema.Required
	found := false
	for _, r := range required {
		if r == "event_id" {
			found = true
			break
		}
	}
	if !found {
		t.Error("event_id should be required")
	}
}
