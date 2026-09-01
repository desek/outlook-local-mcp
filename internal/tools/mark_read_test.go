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

// TestMarkRead_SetsRead verifies that the read state is written and confirmed.
func TestMarkRead_SetsRead(t *testing.T) {
	rec := &patchRecorder{response: `{"id":"msg-1","subject":"Report","isRead":true}`}
	client, srv := newTestGraphClient(t, rec)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleMarkRead(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"message_id": "msg-1",
		"is_read":    true,
	}
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(t, result))
	}
	if len(rec.methods) != 1 || rec.methods[0] != http.MethodPatch {
		t.Fatalf("expected exactly one PATCH, got %v", rec.methods)
	}
	if !strings.Contains(rec.bodies[0], `"isRead":true`) {
		t.Errorf("expected PATCH body to set isRead true, got: %q", rec.bodies[0])
	}
	text := resultText(t, result)
	if !strings.Contains(text, "Read state") || !strings.Contains(text, "read") {
		t.Errorf("expected confirmation to state the read state, got: %q", text)
	}
}

// TestMarkRead_SetsUnread verifies that the inverse is written, so the verb is
// a state set rather than a one-way flag.
func TestMarkRead_SetsUnread(t *testing.T) {
	rec := &patchRecorder{response: `{"id":"msg-1","subject":"Report","isRead":false}`}
	client, srv := newTestGraphClient(t, rec)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleMarkRead(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"message_id": "msg-1",
		"is_read":    false,
	}
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(t, result))
	}
	if !strings.Contains(rec.bodies[0], `"isRead":false`) {
		t.Errorf("expected PATCH body to set isRead false, got: %q", rec.bodies[0])
	}
	if !strings.Contains(resultText(t, result), "unread") {
		t.Errorf("expected confirmation to state unread, got: %q", resultText(t, result))
	}
}

// TestMarkRead_RequiresIsRead verifies that the boolean is required and named
// in the refusal, and that no Graph request is issued without it.
func TestMarkRead_RequiresIsRead(t *testing.T) {
	rec := &patchRecorder{response: `{"id":"msg-1"}`}
	client, srv := newTestGraphClient(t, rec)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleMarkRead(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"message_id": "msg-1"}
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error for a missing is_read")
	}
	if !strings.Contains(resultText(t, result), "is_read") {
		t.Errorf("expected refusal to name is_read, got: %q", resultText(t, result))
	}
	if len(rec.methods) != 0 {
		t.Errorf("expected no Graph request, got %v", rec.methods)
	}
}

// TestMarkRead_RejectsNonBooleanIsRead verifies that a string is refused rather
// than coerced, since a coerced value would silently write the wrong state.
func TestMarkRead_RejectsNonBooleanIsRead(t *testing.T) {
	rec := &patchRecorder{response: `{"id":"msg-1"}`}
	client, srv := newTestGraphClient(t, rec)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleMarkRead(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"message_id": "msg-1",
		"is_read":    "yes",
	}
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error for a non-boolean is_read")
	}
	if len(rec.methods) != 0 {
		t.Errorf("expected no Graph request, got %v", rec.methods)
	}
}

// TestMarkRead_AcceptsNonDraftMessage verifies that no draft guard and no
// read-modify-write round trip is applied to a received message.
func TestMarkRead_AcceptsNonDraftMessage(t *testing.T) {
	rec := &patchRecorder{response: `{"id":"msg-1","subject":"Received","isDraft":false,"isRead":true}`}
	client, srv := newTestGraphClient(t, rec)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleMarkRead(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"message_id": "msg-1",
		"is_read":    true,
	}
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(t, result))
	}
	if len(rec.methods) != 1 || rec.methods[0] != http.MethodPatch {
		t.Errorf("expected exactly one PATCH and no verification GET, got %v", rec.methods)
	}
}
