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

// replyStubHandler records paths of each request and responds to createReply /
// createReplyAll calls with a canned draft.
func replyStubHandler(paths *[]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*paths = append(*paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"reply-draft-1","subject":"RE: Hello","isDraft":true}`))
	})
}

// TestCreateReplyDraft_Reply exercises the createReply path (reply_all omitted).
func TestCreateReplyDraft_Reply(t *testing.T) {
	var paths []string
	client, srv := newTestGraphClient(t, replyStubHandler(&paths))
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleCreateReplyDraft(graph.RetryConfig{}, 30*time.Second, "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"message_id": "AAMkMessage",
		"comment":    "Thanks!",
	}
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", result.Content[0].(mcp.TextContent).Text)
	}
	joined := strings.Join(paths, "|")
	if !strings.Contains(joined, "createReply") || strings.Contains(joined, "createReplyAll") {
		t.Errorf("expected createReply path (not createReplyAll), got: %q", joined)
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "reply-draft-1") {
		t.Errorf("expected draft id in response, got: %q", text)
	}
}

// TestCreateReplyDraft_ReplyAll exercises the createReplyAll path.
func TestCreateReplyDraft_ReplyAll(t *testing.T) {
	var paths []string
	client, srv := newTestGraphClient(t, replyStubHandler(&paths))
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleCreateReplyDraft(graph.RetryConfig{}, 30*time.Second, "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"message_id": "AAMkMessage",
		"reply_all":  true,
	}
	result, err := handler(ctx, req)
	if err != nil || result.IsError {
		t.Fatalf("unexpected error: %v", err)
	}
	joined := strings.Join(paths, "|")
	if !strings.Contains(joined, "createReplyAll") {
		t.Errorf("expected createReplyAll path, got: %q", joined)
	}
}

// TestCreateReplyDraft_WithProvenance verifies that when provenancePropertyID
// is configured the handler issues a follow-up PATCH on the returned draft.
func TestCreateReplyDraft_WithProvenance(t *testing.T) {
	var methods []string
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"reply-draft-1","subject":"RE: x","isDraft":true}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"reply-draft-1"}`))
	}))
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	propID := graph.BuildProvenancePropertyID("com.test.outlook-mcp.created")
	handler := NewHandleCreateReplyDraft(graph.RetryConfig{}, 30*time.Second, propID)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"message_id": "AAMkMessage"}
	result, err := handler(ctx, req)
	if err != nil || result.IsError {
		t.Fatalf("unexpected error: %v", err)
	}
	// Expect a POST for createReply then a PATCH for provenance.
	sawPatch := false
	for _, m := range methods {
		if m == http.MethodPatch {
			sawPatch = true
		}
	}
	if !sawPatch {
		t.Errorf("expected follow-up PATCH, got methods: %v", methods)
	}
}
