package tools

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
)

// TestSetCategories_ReplacesFullSet verifies that the PATCH body carries the
// full replacement set rather than an append.
func TestSetCategories_ReplacesFullSet(t *testing.T) {
	rec := &patchRecorder{response: `{"id":"msg-1","subject":"Report","categories":["Project","Urgent"]}`}
	client, srv := newTestGraphClient(t, rec)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleSetCategories(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"message_id": "msg-1",
		"categories": "Project, Urgent",
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
	if !strings.Contains(rec.bodies[0], `"categories":["Project","Urgent"]`) {
		t.Errorf("expected PATCH body to carry exactly two trimmed entries, got: %q", rec.bodies[0])
	}
}

// TestSetCategories_ConfirmationListsResult verifies that the resulting list is
// read from the Graph response rather than echoed from the request.
func TestSetCategories_ConfirmationListsResult(t *testing.T) {
	rec := &patchRecorder{response: `{"id":"msg-1","subject":"Report","categories":["Project","Urgent"]}`}
	client, srv := newTestGraphClient(t, rec)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleSetCategories(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"message_id": "msg-1",
		"categories": "Ignored",
	}
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(t, result)
	for _, want := range []string{"Categories", "Project", "Urgent"} {
		if !strings.Contains(text, want) {
			t.Errorf("expected confirmation to name %q, got: %q", want, text)
		}
	}
	if strings.Contains(text, "Ignored") {
		t.Errorf("expected the response values, not the request argument, got: %q", text)
	}
}

// TestSetCategories_EmptyValueClears verifies that a whitespace-only value
// clears every category and that the confirmation says so in words.
func TestSetCategories_EmptyValueClears(t *testing.T) {
	rec := &patchRecorder{response: `{"id":"msg-1","subject":"Report","categories":[]}`}
	client, srv := newTestGraphClient(t, rec)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleSetCategories(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"message_id": "msg-1",
		"categories": "   ",
	}
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(t, result))
	}
	if !strings.Contains(rec.bodies[0], `"categories":[]`) {
		t.Errorf("expected PATCH body to carry an empty array, got: %q", rec.bodies[0])
	}
	if !strings.Contains(resultText(t, result), "no categories") {
		t.Errorf("expected the no-categories sentence, got: %q", resultText(t, result))
	}
}

// TestSetCategories_OverLengthRejected verifies that the shared category length
// bound applies and is enforced before any Graph request.
func TestSetCategories_OverLengthRejected(t *testing.T) {
	rec := &patchRecorder{response: `{"id":"msg-1"}`}
	client, srv := newTestGraphClient(t, rec)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleSetCategories(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"message_id": "msg-1",
		"categories": strings.Repeat("a", validate.MaxCategoriesLen+1),
	}
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error for an over-length categories value")
	}
	if !strings.Contains(resultText(t, result), "categories") {
		t.Errorf("expected refusal to name categories, got: %q", resultText(t, result))
	}
	if len(rec.methods) != 0 {
		t.Errorf("expected no Graph request, got %v", rec.methods)
	}
}

// TestSetCategories_RequiresCategories verifies that a missing value is refused
// by name rather than treated as a clear instruction, since clearing must be
// asked for explicitly.
func TestSetCategories_RequiresCategories(t *testing.T) {
	rec := &patchRecorder{response: `{"id":"msg-1"}`}
	client, srv := newTestGraphClient(t, rec)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleSetCategories(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"message_id": "msg-1"}
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error for a missing categories parameter")
	}
	if !strings.Contains(resultText(t, result), "categories") {
		t.Errorf("expected refusal to name categories, got: %q", resultText(t, result))
	}
	if len(rec.methods) != 0 {
		t.Errorf("expected no Graph request, got %v", rec.methods)
	}
}

// TestSetCategories_AcceptsNonDraftMessage verifies that no draft guard and no
// read-modify-write round trip is applied to a received message.
func TestSetCategories_AcceptsNonDraftMessage(t *testing.T) {
	rec := &patchRecorder{response: `{"id":"msg-1","subject":"Received","isDraft":false,"categories":["Project"]}`}
	client, srv := newTestGraphClient(t, rec)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleSetCategories(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"message_id": "msg-1",
		"categories": "Project",
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
