package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
)

// TestCreateForwardDraft_Success exercises the createForward path with
// optional recipients and comment, and validates that the POST body carries
// the parsed recipients.
func TestCreateForwardDraft_Success(t *testing.T) {
	var body []byte
	var paths []string
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.Method == http.MethodPost {
			b, _ := io.ReadAll(r.Body)
			body = b
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"fwd-draft-1","subject":"FW: Hello","isDraft":true}`))
	}))
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleCreateForwardDraft(graph.RetryConfig{}, 30*time.Second, "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"message_id":    "AAMkMessage",
		"to_recipients": "alice@example.com",
		"comment":       "FYI",
	}
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", result.Content[0].(mcp.TextContent).Text)
	}
	if !strings.Contains(strings.Join(paths, "|"), "createForward") {
		t.Errorf("expected createForward path, got: %v", paths)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("payload not JSON: %v", err)
	}
	to, _ := payload["ToRecipients"].([]any)
	if len(to) != 1 {
		t.Errorf("expected 1 ToRecipient, got %d: payload=%s", len(to), string(body))
	}
	if c, _ := payload["Comment"].(string); c != "FYI" {
		t.Errorf("Comment = %q, want FYI", c)
	}
}
