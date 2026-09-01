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
	"github.com/desek/outlook-local-mcp/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
)

// patchRecorder is a test handler that records every request it receives and
// replies with the supplied canned message JSON. It is shared by the mail
// property-write tests, which assert both on the PATCH body and on the number
// of requests issued.
type patchRecorder struct {
	// response is the canned Graph JSON returned for every request.
	response string
	// bodies holds the raw request body of each request received, in order.
	bodies []string
	// methods holds the HTTP method of each request received, in order.
	methods []string
}

// ServeHTTP records the request and writes the canned response.
func (p *patchRecorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	p.bodies = append(p.bodies, string(body))
	p.methods = append(p.methods, r.Method)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(p.response))
}

// resultText returns the text content of a tool result, failing the test when
// the result carries no text content.
func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("result carried no content")
	}
	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", result.Content[0])
	}
	return text.Text
}

// TestSetFlag_Success verifies that the follow-up flag is written and that the
// confirmation states the status the Graph response reported.
func TestSetFlag_Success(t *testing.T) {
	rec := &patchRecorder{response: `{"id":"msg-1","subject":"Quarterly report","flag":{"flagStatus":"flagged"}}`}
	client, srv := newTestGraphClient(t, rec)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleSetFlag(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"message_id":  "msg-1",
		"flag_status": "flagged",
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
	if !strings.Contains(rec.bodies[0], `"flagStatus":"flagged"`) {
		t.Errorf("expected PATCH body to carry the flag status, got: %q", rec.bodies[0])
	}
	text := resultText(t, result)
	for _, want := range []string{"Quarterly report", "msg-1", "Flag status", "flagged"} {
		if !strings.Contains(text, want) {
			t.Errorf("expected confirmation to name %q, got: %q", want, text)
		}
	}
}

// TestSetFlag_RejectsUnknownStatus verifies that only the three accepted
// statuses reach Graph, and that the refusal names all three.
func TestSetFlag_RejectsUnknownStatus(t *testing.T) {
	rec := &patchRecorder{response: `{"id":"msg-1"}`}
	client, srv := newTestGraphClient(t, rec)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleSetFlag(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"message_id":  "msg-1",
		"flag_status": "urgent",
	}
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error for an unaccepted flag status")
	}
	text := resultText(t, result)
	for _, want := range []string{"notFlagged", "flagged", "complete"} {
		if !strings.Contains(text, want) {
			t.Errorf("expected refusal to name %q, got: %q", want, text)
		}
	}
	if len(rec.methods) != 0 {
		t.Errorf("expected no Graph request, got %v", rec.methods)
	}
}

// TestSetFlag_RequiresFlagStatus verifies that a missing status is refused by
// name before any Graph request is issued.
func TestSetFlag_RequiresFlagStatus(t *testing.T) {
	rec := &patchRecorder{response: `{"id":"msg-1"}`}
	client, srv := newTestGraphClient(t, rec)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleSetFlag(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"message_id": "msg-1"}
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error for a missing flag_status")
	}
	if !strings.Contains(resultText(t, result), "flag_status") {
		t.Errorf("expected refusal to name flag_status, got: %q", resultText(t, result))
	}
	if len(rec.methods) != 0 {
		t.Errorf("expected no Graph request, got %v", rec.methods)
	}
}

// TestSetFlag_AcceptsNonDraftMessage verifies that no draft guard is applied:
// a received message is flagged rather than refused for not being a draft.
func TestSetFlag_AcceptsNonDraftMessage(t *testing.T) {
	rec := &patchRecorder{response: `{"id":"msg-1","subject":"Received","isDraft":false,"flag":{"flagStatus":"complete"}}`}
	client, srv := newTestGraphClient(t, rec)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleSetFlag(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"message_id":  "msg-1",
		"flag_status": "complete",
	}
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(t, result))
	}
	if strings.Contains(resultText(t, result), "not a draft") {
		t.Errorf("expected no draft refusal, got: %q", resultText(t, result))
	}
	// A read-modify-write would show as a GET preceding the PATCH.
	if len(rec.methods) != 1 || rec.methods[0] != http.MethodPatch {
		t.Errorf("expected exactly one PATCH and no verification GET, got %v", rec.methods)
	}
}

// TestSetFlag_InvalidMessageIDRejectedBeforeCall verifies that identifier
// validation precedes the Graph request. The invalid case is an over-length
// identifier because that is what validate.ValidateResourceID rejects: it
// bounds length and emptiness rather than character set.
func TestSetFlag_InvalidMessageIDRejectedBeforeCall(t *testing.T) {
	rec := &patchRecorder{response: `{"id":"msg-1"}`}
	client, srv := newTestGraphClient(t, rec)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleSetFlag(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"message_id":  strings.Repeat("a", validate.MaxResourceIDLen+1),
		"flag_status": "flagged",
	}
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error for a malformed message_id")
	}
	if len(rec.methods) != 0 {
		t.Errorf("expected no Graph request, got %v", rec.methods)
	}
}
