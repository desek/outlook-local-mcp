// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the mail domain's add_attachment handler on its
// direct-upload path: parameter validation ahead of any Graph request, the size
// bound, the draft guard, the routing boundary, and the confirmation built from
// the service's response.
package tools

import (
	"context"
	"encoding/base64"
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

// attachmentRecorder records the requests a handler issues, so a test can
// assert not only what came back but how many round trips produced it. The
// counts are what pin "before any Graph request" and "no upload session": an
// assertion on the result alone cannot distinguish a refusal from a call that
// happened and was then discarded.
type attachmentRecorder struct {
	gets      int
	posts     int
	lastPost  map[string]any
	isDraft   bool
	subject   string
	returnsID string
}

// handler serves the isDraft verification GET and the attachment POST, and
// records both.
func (r *attachmentRecorder) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case http.MethodGet:
			r.gets++
			body, _ := json.Marshal(map[string]any{
				"id": "draft-1", "isDraft": r.isDraft, "subject": r.subject,
			})
			_, _ = w.Write(body)
		case http.MethodPost:
			r.posts++
			raw, _ := io.ReadAll(req.Body)
			r.lastPost = map[string]any{}
			_ = json.Unmarshal(raw, &r.lastPost)
			_, _ = w.Write([]byte(`{"@odata.type":"#microsoft.graph.fileAttachment","id":"` +
				r.returnsID + `","name":"report.pdf","contentType":"application/pdf","size":5}`))
		default:
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
		}
	})
}

// newAddAttachmentFixture wires a recorder-backed Graph client into a context
// and returns the handler under test alongside the recorder.
func newAddAttachmentFixture(t *testing.T, rec *attachmentRecorder, maxSize int64) (context.Context, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	t.Helper()
	client, srv := newTestGraphClient(t, rec.handler())
	t.Cleanup(srv.Close)
	ctx := auth.WithGraphClient(context.Background(), client)
	return ctx, NewHandleAddAttachment(graph.RetryConfig{}, 30*time.Second, maxSize)
}

// callAddAttachment invokes the handler with the given arguments.
func callAddAttachment(t *testing.T, ctx context.Context, handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), args map[string]any) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return result
}

// TestNewHandleAddAttachment_ReturnsHandler validates handler construction.
func TestNewHandleAddAttachment_ReturnsHandler(t *testing.T) {
	if NewHandleAddAttachment(graph.RetryConfig{}, 0, 10485760) == nil {
		t.Fatal("expected non-nil handler")
	}
}

// TestAddAttachment_DirectPath verifies that a small file is attached with a
// single POST, that no second GET is paid for the subject, and that the
// confirmation reports the service's identifier and the direct transfer path.
func TestAddAttachment_DirectPath(t *testing.T) {
	rec := &attachmentRecorder{isDraft: true, subject: "Quarterly report", returnsID: "att-from-service"}
	ctx, handler := newAddAttachmentFixture(t, rec, 10485760)

	result := callAddAttachment(t, ctx, handler, map[string]any{
		"message_id":    "draft-1",
		"name":          "report.pdf",
		"mime_type":     "application/pdf",
		"content_bytes": base64.StdEncoding.EncodeToString([]byte("hello")),
	})
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(t, result))
	}
	if rec.gets != 1 || rec.posts != 1 {
		t.Errorf("expected 1 GET and 1 POST, got %d GET and %d POST", rec.gets, rec.posts)
	}
	text := resultText(t, result)
	for _, want := range []string{
		`Attachment added: "report.pdf"`,
		`Message: "Quarterly report"`,
		"Message ID: draft-1",
		"Attachment ID: att-from-service",
		"Size: 5 bytes",
		"Transfer: " + TransferDirect,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("confirmation missing %q, got:\n%s", want, text)
		}
	}
}

// TestAddAttachment_IdentifierComesFromResponse verifies the confirmation
// reports the identifier the service minted rather than echoing any request
// argument, which is the only way the caller can address the attachment later.
func TestAddAttachment_IdentifierComesFromResponse(t *testing.T) {
	rec := &attachmentRecorder{isDraft: true, subject: "Notes", returnsID: "service-minted-id"}
	ctx, handler := newAddAttachmentFixture(t, rec, 10485760)

	result := callAddAttachment(t, ctx, handler, map[string]any{
		"message_id":    "draft-1",
		"name":          "notes.txt",
		"content_bytes": base64.StdEncoding.EncodeToString([]byte("hello")),
	})
	text := resultText(t, result)
	if !strings.Contains(text, "Attachment ID: service-minted-id") {
		t.Errorf("expected the service identifier in the confirmation, got:\n%s", text)
	}
	if strings.Contains(text, "Attachment ID: notes.txt") {
		t.Error("confirmation echoed a request argument as the attachment identifier")
	}
}

// TestAddAttachment_DefaultsMimeType verifies that an omitted mime_type is sent
// as the generic binary type rather than left empty.
func TestAddAttachment_DefaultsMimeType(t *testing.T) {
	rec := &attachmentRecorder{isDraft: true, subject: "Notes", returnsID: "att-1"}
	ctx, handler := newAddAttachmentFixture(t, rec, 10485760)

	callAddAttachment(t, ctx, handler, map[string]any{
		"message_id":    "draft-1",
		"name":          "notes.bin",
		"content_bytes": base64.StdEncoding.EncodeToString([]byte("hello")),
	})
	if got := rec.lastPost["contentType"]; got != defaultAttachmentMimeType {
		t.Errorf("contentType = %v, want %s", got, defaultAttachmentMimeType)
	}
	if got := rec.lastPost["@odata.type"]; got != "#microsoft.graph.fileAttachment" {
		t.Errorf("@odata.type = %v, want #microsoft.graph.fileAttachment", got)
	}
	if got := rec.lastPost["name"]; got != "notes.bin" {
		t.Errorf("name = %v, want notes.bin", got)
	}
}

// TestAddAttachment_NonDraftRefused verifies the draft guard refuses a message
// whose isDraft is false, and that no attachment request follows the refusal.
func TestAddAttachment_NonDraftRefused(t *testing.T) {
	rec := &attachmentRecorder{isDraft: false, subject: "Received mail", returnsID: "att-1"}
	ctx, handler := newAddAttachmentFixture(t, rec, 10485760)

	result := callAddAttachment(t, ctx, handler, map[string]any{
		"message_id":    "msg-1",
		"name":          "notes.txt",
		"content_bytes": base64.StdEncoding.EncodeToString([]byte("hello")),
	})
	if !result.IsError {
		t.Fatal("expected a refusal for a non-draft message")
	}
	if !strings.Contains(resultText(t, result), "message is not a draft") {
		t.Errorf("expected the established not-a-draft refusal, got: %s", resultText(t, result))
	}
	if rec.posts != 0 {
		t.Errorf("expected no attachment request after the refusal, got %d POST", rec.posts)
	}
}

// TestAddAttachment_RejectsBadInputBeforeAnyRequest verifies each required and
// well-formedness rule refuses the call without issuing a single Graph request,
// and that the error names the parameter to supply or correct.
func TestAddAttachment_RejectsBadInputBeforeAnyRequest(t *testing.T) {
	valid := base64.StdEncoding.EncodeToString([]byte("hello"))
	cases := []struct {
		name     string
		args     map[string]any
		wantText string
	}{
		{"missing message id", map[string]any{"name": "a.txt", "content_bytes": valid}, "message_id"},
		{"oversize message id", map[string]any{"message_id": strings.Repeat("i", 5000), "name": "a.txt", "content_bytes": valid}, "message_id"},
		{"missing name", map[string]any{"message_id": "draft-1", "content_bytes": valid}, "name"},
		{"empty name", map[string]any{"message_id": "draft-1", "name": "", "content_bytes": valid}, "name"},
		{"oversize name", map[string]any{"message_id": "draft-1", "name": strings.Repeat("a", 300), "content_bytes": valid}, "name"},
		{"missing content bytes", map[string]any{"message_id": "draft-1", "name": "a.txt"}, "content_bytes"},
		{"empty content bytes", map[string]any{"message_id": "draft-1", "name": "a.txt", "content_bytes": ""}, "content_bytes"},
		{"malformed content bytes", map[string]any{"message_id": "draft-1", "name": "a.txt", "content_bytes": "not base64!!"}, "content_bytes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &attachmentRecorder{isDraft: true, subject: "Notes", returnsID: "att-1"}
			ctx, handler := newAddAttachmentFixture(t, rec, 10485760)

			result := callAddAttachment(t, ctx, handler, tc.args)
			if !result.IsError {
				t.Fatal("expected a validation error")
			}
			if !strings.Contains(resultText(t, result), tc.wantText) {
				t.Errorf("error does not name %q, got: %s", tc.wantText, resultText(t, result))
			}
			if rec.gets != 0 || rec.posts != 0 {
				t.Errorf("expected no Graph request, got %d GET and %d POST", rec.gets, rec.posts)
			}
		})
	}
}

// TestAddAttachment_OversizeRefusedBeforeUpload verifies a payload past the
// configured bound is refused before any Graph request, with an error naming
// the measured size, the bound, and the variable that raises it.
func TestAddAttachment_OversizeRefusedBeforeUpload(t *testing.T) {
	rec := &attachmentRecorder{isDraft: true, subject: "Notes", returnsID: "att-1"}
	ctx, handler := newAddAttachmentFixture(t, rec, 8)

	result := callAddAttachment(t, ctx, handler, map[string]any{
		"message_id":    "draft-1",
		"name":          "big.bin",
		"content_bytes": base64.StdEncoding.EncodeToString(make([]byte, 16)),
	})
	if !result.IsError {
		t.Fatal("expected a refusal for an oversize attachment")
	}
	text := resultText(t, result)
	for _, want := range []string{"16 bytes", "8 bytes", "OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES"} {
		if !strings.Contains(text, want) {
			t.Errorf("error missing %q, got: %s", want, text)
		}
	}
	if rec.gets != 0 || rec.posts != 0 {
		t.Errorf("expected no Graph request, got %d GET and %d POST", rec.gets, rec.posts)
	}
}

// TestAddAttachment_RoutingBoundary pins the transfer-path boundary at the
// inline threshold. One byte below it the payload goes out as a single POST; at
// exactly the threshold it does not, because the service rejects a direct POST
// of that size and only an upload session carries it.
func TestAddAttachment_RoutingBoundary(t *testing.T) {
	cases := []struct {
		name      string
		size      int
		wantPosts int
	}{
		{"one byte below the threshold posts directly", inlineAttachmentThresholdBytes - 1, 1},
		{"exactly at the threshold does not post directly", inlineAttachmentThresholdBytes, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &attachmentRecorder{isDraft: true, subject: "Notes", returnsID: "att-1"}
			ctx, handler := newAddAttachmentFixture(t, rec, 0)

			callAddAttachment(t, ctx, handler, map[string]any{
				"message_id":    "draft-1",
				"name":          "payload.bin",
				"content_bytes": base64.StdEncoding.EncodeToString(make([]byte, tc.size)),
			})
			if rec.posts != tc.wantPosts {
				t.Errorf("attachment POSTs = %d, want %d", rec.posts, tc.wantPosts)
			}
		})
	}
}
