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

// updateDraftHandler stubs GET /me/messages/{id} (for isDraft verification)
// followed by PATCH. The isDraft parameter controls the GET response.
func updateDraftHandler(isDraft bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			if isDraft {
				_, _ = w.Write([]byte(`{"id":"draft-1","isDraft":true}`))
			} else {
				_, _ = w.Write([]byte(`{"id":"draft-1","isDraft":false}`))
			}
		case http.MethodPatch:
			_, _ = w.Write([]byte(`{"id":"draft-1","subject":"Updated","isDraft":true}`))
		default:
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
		}
	})
}

// TestUpdateDraft_Success verifies that the handler verifies isDraft=true
// and PATCHes the message.
func TestUpdateDraft_Success(t *testing.T) {
	client, srv := newTestGraphClient(t, updateDraftHandler(true))
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleUpdateDraft(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"message_id": "draft-1",
		"subject":    "Updated",
	}
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", result.Content[0].(mcp.TextContent).Text)
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "updated") {
		t.Errorf("expected updated confirmation, got: %q", text)
	}
}

// TestUpdateDraft_NotDraft verifies that the handler rejects non-draft
// messages with a tool error.
func TestUpdateDraft_NotDraft(t *testing.T) {
	client, srv := newTestGraphClient(t, updateDraftHandler(false))
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleUpdateDraft(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"message_id": "msg-1",
		"subject":    "No can do",
	}
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error for non-draft message")
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "not a draft") {
		t.Errorf("expected 'not a draft' message, got: %q", text)
	}
}
