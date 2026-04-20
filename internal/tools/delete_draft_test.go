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

// deleteDraftHandler stubs GET (isDraft verification) then DELETE.
func deleteDraftHandler(isDraft bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			if isDraft {
				_, _ = w.Write([]byte(`{"id":"draft-1","isDraft":true}`))
			} else {
				_, _ = w.Write([]byte(`{"id":"draft-1","isDraft":false}`))
			}
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
		}
	})
}

// TestDeleteDraft_Success verifies that a draft can be deleted.
func TestDeleteDraft_Success(t *testing.T) {
	client, srv := newTestGraphClient(t, deleteDraftHandler(true))
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleDeleteDraft(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"message_id": "draft-1"}
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", result.Content[0].(mcp.TextContent).Text)
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "Draft deleted") {
		t.Errorf("expected deletion confirmation, got: %q", text)
	}
}

// TestDeleteDraft_NotDraft verifies that deletion is refused for non-draft
// messages.
func TestDeleteDraft_NotDraft(t *testing.T) {
	client, srv := newTestGraphClient(t, deleteDraftHandler(false))
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleDeleteDraft(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"message_id": "msg-1"}
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error for non-draft message")
	}
}
