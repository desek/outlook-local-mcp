package tools

import (
	"context"
	"log/slog"
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

// TestVerifyIsDraftReturnsSubject verifies that the shared draft guard asks for
// the subject in its projection and hands the fetched message back to its
// caller.
//
// The projection is the load-bearing half: a caller that names the draft in its
// confirmation reads the subject from this one GET rather than paying a second.
// Dropping subject from the $select would leave every such confirmation reading
// the no-subject placeholder against a real mailbox, and no handler-level test
// can detect that, because a fixture answers with the subject whether or not it
// was asked for.
func TestVerifyIsDraftReturnsSubject(t *testing.T) {
	var selects []string
	gets := 0
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
			return
		}
		gets++
		selects = append(selects, r.URL.Query().Get("$select"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"draft-1","isDraft":true,"subject":"Quarterly report"}`))
	}))
	defer srv.Close()

	msg, errResult := verifyIsDraft(context.Background(), client, graph.RetryConfig{}, 30*time.Second, "draft-1", slog.Default())
	if errResult != nil {
		t.Fatalf("unexpected refusal: %s", errResult.Content[0].(mcp.TextContent).Text)
	}
	if gets != 1 {
		t.Errorf("verification GETs = %d, want 1", gets)
	}
	if len(selects) != 1 || !strings.Contains(selects[0], "subject") {
		t.Errorf("projection = %v, want one containing subject", selects)
	}
	if msg == nil {
		t.Fatal("the guard returned no message, so its caller cannot read the subject from it")
	}
	if got := graph.SafeStr(msg.GetSubject()); got != "Quarterly report" {
		t.Errorf("subject = %q, want %q", got, "Quarterly report")
	}
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
