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

// createDraftStubHandler serves POST /me/messages and records the most
// recent request body for assertion.
func createDraftStubHandler(captureBody *[]byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "expected POST", http.StatusMethodNotAllowed)
			return
		}
		if captureBody != nil {
			b, _ := io.ReadAll(r.Body)
			*captureBody = b
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"draft-123","subject":"Hello draft","isDraft":true}`))
	})
}

// TestCreateDraft_Success validates that the handler creates a draft and
// returns a confirmation containing the draft ID and subject.
func TestCreateDraft_Success(t *testing.T) {
	client, srv := newTestGraphClient(t, createDraftStubHandler(nil))
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleCreateDraft(graph.RetryConfig{}, 30*time.Second, "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"subject": "Hello draft",
		"body":    "Hi there",
	}
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", result.Content[0].(mcp.TextContent).Text)
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "draft-123") {
		t.Errorf("expected draft ID in response, got: %q", text)
	}
	if !strings.Contains(text, "Drafts folder") {
		t.Errorf("expected Drafts folder advisory, got: %q", text)
	}
}

// TestCreateDraft_AllFields verifies that recipients, body, content_type,
// and importance are all serialized onto the POST request body.
func TestCreateDraft_AllFields(t *testing.T) {
	var body []byte
	client, srv := newTestGraphClient(t, createDraftStubHandler(&body))
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleCreateDraft(graph.RetryConfig{}, 30*time.Second, "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"to_recipients":  "alice@example.com, bob@example.com",
		"cc_recipients":  "carol@example.com",
		"bcc_recipients": "dan@example.com",
		"subject":        "Greetings",
		"body":           "<p>Hello</p>",
		"content_type":   "html",
		"importance":     "high",
	}
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", result.Content[0].(mcp.TextContent).Text)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("payload not JSON: %v", err)
	}
	if s, _ := payload["subject"].(string); s != "Greetings" {
		t.Errorf("subject = %q, want Greetings", s)
	}
	if imp, _ := payload["importance"].(string); imp != "high" {
		t.Errorf("importance = %q, want high", imp)
	}
	to, _ := payload["toRecipients"].([]any)
	if len(to) != 2 {
		t.Errorf("toRecipients count = %d, want 2", len(to))
	}
	cc, _ := payload["ccRecipients"].([]any)
	if len(cc) != 1 {
		t.Errorf("ccRecipients count = %d, want 1", len(cc))
	}
	bb, _ := payload["bccRecipients"].([]any)
	if len(bb) != 1 {
		t.Errorf("bccRecipients count = %d, want 1", len(bb))
	}
	if bodyField, _ := payload["body"].(map[string]any); bodyField != nil {
		if ct, _ := bodyField["contentType"].(string); !strings.EqualFold(ct, "html") {
			t.Errorf("body.contentType = %q, want html", ct)
		}
	} else {
		t.Error("expected body in payload")
	}
}

// TestCreateDraft_WithProvenance verifies that provenance extended property
// is stamped on the POST body when provenancePropertyID is configured.
func TestCreateDraft_WithProvenance(t *testing.T) {
	var body []byte
	client, srv := newTestGraphClient(t, createDraftStubHandler(&body))
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	propID := graph.BuildProvenancePropertyID("com.test.outlook-mcp.created")
	handler := NewHandleCreateDraft(graph.RetryConfig{}, 30*time.Second, propID)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"subject": "provenance test"}
	result, err := handler(ctx, req)
	if err != nil || result.IsError {
		t.Fatalf("unexpected error: %v / isErr=%v", err, result != nil && result.IsError)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("payload not JSON: %v", err)
	}
	props, _ := payload["singleValueExtendedProperties"].([]any)
	if len(props) == 0 {
		t.Fatal("expected singleValueExtendedProperties in payload")
	}
	first, _ := props[0].(map[string]any)
	if id, _ := first["id"].(string); id != propID {
		t.Errorf("property id = %q, want %q", id, propID)
	}
	if val, _ := first["value"].(string); val != "true" {
		t.Errorf("property value = %q, want true", val)
	}
}
