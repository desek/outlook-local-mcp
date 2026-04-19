// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the mail_get_conversation tool: registration,
// parameter schema, handler construction, and happy-path integration through
// a test HTTP server stub to verify conversationId resolution, direct
// conversation_id usage, and chronological ordering.
package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
)

// conversationStubHandler returns a handler that serves a message GET (to
// resolve conversationId) and a messages list GET (the thread). The captured
// URLs can be inspected via the returned pointer.
func conversationStubHandler(capture *[]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*capture = append(*capture, r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/messages/") && !strings.HasSuffix(r.URL.Path, "/messages"):
			// Single message fetch — return conversationId.
			_, _ = w.Write([]byte(`{"id":"m1","conversationId":"convo-123"}`))
		default:
			// Message list — return three messages in ascending receivedDateTime.
			_, _ = w.Write([]byte(`{"value":[
				{"id":"a","subject":"First","conversationId":"convo-123","receivedDateTime":"2026-04-01T09:00:00Z","bodyPreview":"hi"},
				{"id":"b","subject":"Second","conversationId":"convo-123","receivedDateTime":"2026-04-01T10:00:00Z","bodyPreview":"hello"},
				{"id":"c","subject":"Third","conversationId":"convo-123","receivedDateTime":"2026-04-01T11:00:00Z","bodyPreview":"goodbye"}
			]}`))
		}
	})
}

// TestGetConversationTool_Registration validates tool identity and annotations.
func TestGetConversationTool_Registration(t *testing.T) {
	tool := NewGetConversationTool()
	if tool.Name != "mail_get_conversation" {
		t.Errorf("tool name = %q, want mail_get_conversation", tool.Name)
	}
	if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint {
		t.Error("expected ReadOnlyHint true")
	}
}

// TestGetConversationTool_HasParameters validates tool parameters are defined.
func TestGetConversationTool_HasParameters(t *testing.T) {
	schema := NewGetConversationTool().InputSchema
	for _, p := range []string{"message_id", "conversation_id", "max_results", "account", "output"} {
		if _, ok := schema.Properties[p]; !ok {
			t.Errorf("missing property %q", p)
		}
	}
}

// TestNewHandleGetConversation_ReturnsHandler validates handler construction.
func TestNewHandleGetConversation_ReturnsHandler(t *testing.T) {
	if NewHandleGetConversation(graph.RetryConfig{}, 0) == nil {
		t.Fatal("expected non-nil handler")
	}
}

// TestGetConversation_ByMessageID exercises the conversationId resolution path:
// given message_id only, the handler fetches the message first, then queries
// the thread.
func TestGetConversation_ByMessageID(t *testing.T) {
	var urls []string
	client, srv := newTestGraphClient(t, conversationStubHandler(&urls))
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleGetConversation(graph.RetryConfig{}, 0)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"message_id": "AAMkAGI2TGULAAA=",
		"output":     "summary",
	}
	result, err := handler(ctx, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", result.Content[0].(mcp.TextContent).Text)
	}
	// First URL should be the single-message GET, second the thread list.
	if len(urls) < 2 {
		t.Fatalf("expected >=2 graph calls, got %d: %v", len(urls), urls)
	}
	if !strings.Contains(urls[1], "conversationId%20eq%20%27convo-123%27") && !strings.Contains(urls[1], "conversationId+eq+%27convo-123%27") {
		t.Errorf("expected thread query filter for convo-123, got %q", urls[1])
	}
}

// TestGetConversation_ByConversationID exercises the direct path: when
// conversation_id is supplied, no message fetch is made.
func TestGetConversation_ByConversationID(t *testing.T) {
	var urls []string
	client, srv := newTestGraphClient(t, conversationStubHandler(&urls))
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleGetConversation(graph.RetryConfig{}, 0)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"conversation_id": "convo-123",
		"output":          "summary",
	}
	result, err := handler(ctx, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", result.Content[0].(mcp.TextContent).Text)
	}
	// Only the thread list call should be made (no message GET).
	if len(urls) != 1 {
		t.Fatalf("expected exactly 1 graph call, got %d: %v", len(urls), urls)
	}
}

// TestGetConversation_ChronologicalOrder validates that the response preserves
// the chronological order returned by the Graph API (oldest first) and that
// the orderby clause is requested.
func TestGetConversation_ChronologicalOrder(t *testing.T) {
	var urls []string
	client, srv := newTestGraphClient(t, conversationStubHandler(&urls))
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleGetConversation(graph.RetryConfig{}, 0)
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"conversation_id": "convo-123",
		"output":          "summary",
	}
	result, err := handler(ctx, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", result.Content[0].(mcp.TextContent).Text)
	}
	// Verify orderby parameter in the URL.
	joined := strings.Join(urls, "|")
	if !strings.Contains(joined, "orderby=receivedDateTime") {
		t.Errorf("expected orderby=receivedDateTime in request URLs, got %q", joined)
	}

	// Decode JSON response and confirm message order.
	text := result.Content[0].(mcp.TextContent).Text
	var payload struct {
		Messages []map[string]any `json:"messages"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if len(payload.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(payload.Messages))
	}
	expected := []string{"First", "Second", "Third"}
	for i, want := range expected {
		got, _ := payload.Messages[i]["subject"].(string)
		if got != want {
			t.Errorf("messages[%d].subject = %q, want %q", i, got, want)
		}
	}
}

// TestGetConversation_MissingParams validates that omitting both message_id and
// conversation_id returns a tool error.
func TestGetConversation_MissingParams(t *testing.T) {
	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleGetConversation(graph.RetryConfig{}, 0)
	result, err := handler(ctx, mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when both identifiers missing")
	}
}
