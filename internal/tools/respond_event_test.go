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

// TestRespondEvent_Accept validates that HandleRespondEvent routes an "accept"
// response to the /accept endpoint on the Graph API and returns a plain text
// confirmation.
func TestRespondEvent_Accept(t *testing.T) {
	var capturedPath string
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	handler := HandleRespondEvent(graph.RetryConfig{}, 30*time.Second)

	req := mcp.CallToolRequest{}
	req.Params.Name = "calendar_respond_event"
	req.Params.Arguments = map[string]any{
		"event_id": "AAMkAGTest123",
		"response": "accept",
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %+v", result)
	}

	if !strings.Contains(capturedPath, "/accept") {
		t.Errorf("expected path to contain /accept, got %q", capturedPath)
	}

	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "Event accepted: AAMkAGTest123") {
		t.Errorf("response should contain 'Event accepted: AAMkAGTest123', got: %q", text)
	}
	if !strings.Contains(text, "Response sent to organizer.") {
		t.Errorf("response should contain organizer message, got: %q", text)
	}
}

// TestRespondEvent_Tentative validates that HandleRespondEvent routes a
// "tentative" response to the /tentativelyAccept endpoint on the Graph API.
func TestRespondEvent_Tentative(t *testing.T) {
	var capturedPath string
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	handler := HandleRespondEvent(graph.RetryConfig{}, 30*time.Second)

	req := mcp.CallToolRequest{}
	req.Params.Name = "calendar_respond_event"
	req.Params.Arguments = map[string]any{
		"event_id": "AAMkAGTest123",
		"response": "tentative",
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %+v", result)
	}

	if !strings.Contains(capturedPath, "/tentativelyAccept") {
		t.Errorf("expected path to contain /tentativelyAccept, got %q", capturedPath)
	}

	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "Event tentatively accepted: AAMkAGTest123") {
		t.Errorf("response should contain 'Event tentatively accepted: AAMkAGTest123', got: %q", text)
	}
}

// TestRespondEvent_Decline validates that HandleRespondEvent routes a "decline"
// response to the /decline endpoint on the Graph API.
func TestRespondEvent_Decline(t *testing.T) {
	var capturedPath string
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	handler := HandleRespondEvent(graph.RetryConfig{}, 30*time.Second)

	req := mcp.CallToolRequest{}
	req.Params.Name = "calendar_respond_event"
	req.Params.Arguments = map[string]any{
		"event_id": "AAMkAGTest123",
		"response": "decline",
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %+v", result)
	}

	if !strings.Contains(capturedPath, "/decline") {
		t.Errorf("expected path to contain /decline, got %q", capturedPath)
	}

	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "Event declined: AAMkAGTest123") {
		t.Errorf("response should contain 'Event declined: AAMkAGTest123', got: %q", text)
	}
}

// TestRespondEvent_InvalidResponse validates that HandleRespondEvent rejects an
// invalid response value ("maybe") with a validation error listing valid values.
func TestRespondEvent_InvalidResponse(t *testing.T) {
	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()

	handler := HandleRespondEvent(graph.RetryConfig{}, 30*time.Second)

	req := mcp.CallToolRequest{}
	req.Params.Name = "calendar_respond_event"
	req.Params.Arguments = map[string]any{
		"event_id": "AAMkAGTest123",
		"response": "maybe",
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for invalid response")
	}

	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "maybe") {
		t.Errorf("error text should mention invalid value, got %q", text)
	}
	if !strings.Contains(text, "accept") || !strings.Contains(text, "tentative") || !strings.Contains(text, "decline") {
		t.Errorf("error text should list valid values, got %q", text)
	}
}

// TestRespondEvent_WithComment validates that the comment parameter is passed
// through to the Graph API request body.
func TestRespondEvent_WithComment(t *testing.T) {
	var capturedBody string
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capturedBody = string(body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	handler := HandleRespondEvent(graph.RetryConfig{}, 30*time.Second)

	req := mcp.CallToolRequest{}
	req.Params.Name = "calendar_respond_event"
	req.Params.Arguments = map[string]any{
		"event_id": "AAMkAGTest123",
		"response": "accept",
		"comment":  "Running late",
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %+v", result)
	}

	if !strings.Contains(capturedBody, "Running late") {
		t.Errorf("request body should contain comment, got %q", capturedBody)
	}
}

// TestRespondEvent_SendResponseFalse validates that when send_response is set
// to false, the SendResponse field in the request body is false.
func TestRespondEvent_SendResponseFalse(t *testing.T) {
	var capturedBody string
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capturedBody = string(body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	handler := HandleRespondEvent(graph.RetryConfig{}, 30*time.Second)

	req := mcp.CallToolRequest{}
	req.Params.Name = "calendar_respond_event"
	req.Params.Arguments = map[string]any{
		"event_id":      "AAMkAGTest123",
		"response":      "decline",
		"send_response": false,
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %+v", result)
	}

	// The Graph SDK serializes SendResponse as a JSON field in the request body.
	// When false, it should appear as false in the body.
	if !strings.Contains(capturedBody, "false") {
		t.Errorf("request body should contain SendResponse=false, got %q", capturedBody)
	}
}

// TestNewRespondEventTool validates that the respond_event tool definition has
// the correct name, parameters, and required fields.
func TestNewRespondEventTool(t *testing.T) {
	tool := NewRespondEventTool()

	if tool.Name != "calendar_respond_event" {
		t.Errorf("name = %q, want %q", tool.Name, "calendar_respond_event")
	}

	if tool.Description == "" {
		t.Fatal("expected non-empty description")
	}

	props := tool.InputSchema.Properties
	for _, name := range []string{"event_id", "response", "comment", "send_response", "account"} {
		if _, exists := props[name]; !exists {
			t.Errorf("expected %s in properties", name)
		}
	}

	requiredSet := make(map[string]bool)
	for _, r := range tool.InputSchema.Required {
		requiredSet[r] = true
	}
	if !requiredSet["event_id"] {
		t.Error("event_id should be required")
	}
	if !requiredSet["response"] {
		t.Error("response should be required")
	}
	if requiredSet["comment"] {
		t.Error("comment should not be required")
	}
	if requiredSet["send_response"] {
		t.Error("send_response should not be required")
	}
}
