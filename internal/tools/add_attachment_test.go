// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the mail domain's add_attachment handler on its
// direct-upload path: parameter validation ahead of any Graph request, the size
// bound, the draft guard, the routing boundary, and the confirmation built from
// the service's response.
package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
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

	// upload serves the chunked path when a test exercises it, and is nil for
	// the direct-path tests, whose fixture must answer an upload request with a
	// failure rather than a canned success.
	upload *uploadServer

	// stallPost holds the attachment POST open for this long, so a test can
	// drive the handler past its own request timeout. The stall is abandoned as
	// soon as the client gives up, so the test costs the timeout rather than
	// the stall.
	stallPost time.Duration

	// failPostStatus, when non-zero, answers the attachment POST with that
	// status and a Graph error body, which is the failure the direct path
	// reports through the shared redactor.
	failPostStatus int
}

// stall blocks until the duration elapses or the caller abandons the request,
// whichever comes first.
func stall(req *http.Request, d time.Duration) {
	if d <= 0 {
		return
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-req.Context().Done():
	}
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
			if strings.HasSuffix(req.URL.Path, "/createUploadSession") {
				r.upload.serveSession(w)
				return
			}
			r.posts++
			raw, _ := io.ReadAll(req.Body)
			// The body is drained before stalling so the server can notice the
			// client abandoning the request; otherwise the fixture waits out its
			// own stall after the handler has already given up.
			stall(req, r.stallPost)
			if r.failPostStatus != 0 {
				w.WriteHeader(r.failPostStatus)
				_, _ = w.Write([]byte(`{"error":{"code":"ErrorAccessDenied","message":"Access is denied."}}`))
				return
			}
			r.lastPost = map[string]any{}
			_ = json.Unmarshal(raw, &r.lastPost)
			_, _ = w.Write([]byte(`{"@odata.type":"#microsoft.graph.fileAttachment","id":"` +
				r.returnsID + `","name":"report.pdf","contentType":"application/pdf","size":5}`))
		case http.MethodPut:
			r.upload.serveChunk(w, req)
		default:
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
		}
	})
}

// newAddAttachmentFixture wires a recorder-backed Graph client into a context
// and returns the handler under test alongside the recorder.
func newAddAttachmentFixture(t *testing.T, rec *attachmentRecorder, maxSize int64) (context.Context, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	t.Helper()
	return newAddAttachmentFixtureWithTimeout(t, rec, maxSize, 30*time.Second)
}

// newAddAttachmentFixtureWithTimeout is newAddAttachmentFixture with the
// per-call Graph timeout under the test's control, which is what a stalled
// request needs in order to be abandoned rather than waited out.
func newAddAttachmentFixtureWithTimeout(t *testing.T, rec *attachmentRecorder, maxSize int64, timeout time.Duration) (context.Context, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	t.Helper()
	client, srv := newTestGraphClient(t, rec.handler())
	t.Cleanup(srv.Close)
	if rec.upload != nil {
		// The upload URL is only knowable once the test server is listening,
		// and it carries a query string because the real one carries an access
		// token there, which the redaction tests must be able to look for.
		rec.upload.url = srv.URL + "/upload/session-1?token=upload-secret"
	}
	ctx := auth.WithGraphClient(context.Background(), client)
	return ctx, NewHandleAddAttachment(graph.RetryConfig{}, timeout, maxSize)
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

// TestAddAttachment_StalledTransferIsBounded verifies that neither transfer path
// can outlast the configured request timeout: a service that never answers is
// abandoned and reported, rather than holding the handler open indefinitely.
//
// The two paths are asserted separately because they are bounded by different
// code: the direct POST by the SDK call's timeout context, and the chunked path
// by the per-chunk bound the PUT carries, which the SDK never sees.
func TestAddAttachment_StalledTransferIsBounded(t *testing.T) {
	const bound = 50 * time.Millisecond
	const stallFor = 10 * time.Second

	t.Run("a stalled direct post reports the timeout and its bound", func(t *testing.T) {
		rec := &attachmentRecorder{isDraft: true, subject: "Notes", returnsID: "att-1", stallPost: stallFor}
		ctx, handler := newAddAttachmentFixtureWithTimeout(t, rec, 0, bound)

		result := callAddAttachment(t, ctx, handler, map[string]any{
			"message_id":    "draft-1",
			"name":          "notes.txt",
			"content_bytes": base64.StdEncoding.EncodeToString([]byte("hello")),
		})
		if !result.IsError {
			t.Fatalf("a stalled upload was confirmed as added: %s", resultText(t, result))
		}
		if want := graph.TimeoutErrorMessage(int(bound.Seconds())); !strings.Contains(resultText(t, result), want) {
			t.Errorf("expected the timeout message %q, got: %s", want, resultText(t, result))
		}
	})

	t.Run("a stalled chunk fails the transfer without leaking the upload URL", func(t *testing.T) {
		up := &uploadServer{locationID: "att-large", stallChunk: stallFor}
		rec := &attachmentRecorder{isDraft: true, subject: "Notes", upload: up}
		ctx, handler := newAddAttachmentFixtureWithTimeout(t, rec, 0, bound)

		result := callAddAttachment(t, ctx, handler, largeAttachmentArgs(inlineAttachmentThresholdBytes))
		if !result.IsError {
			t.Fatalf("a stalled chunk was confirmed as added: %s", resultText(t, result))
		}
		text := resultText(t, result)
		if !strings.Contains(text, "not added") {
			t.Errorf("the error does not state the attachment was not added: %s", text)
		}
		if strings.Contains(text, up.url) || strings.Contains(text, "upload-secret") {
			t.Errorf("the upload URL survived an abandoned chunk: %s", text)
		}
	})
}

// TestAddAttachment_GraphFailureCarriesFixOnBothChannels verifies that a refused
// direct POST returns the redacted service error with the correction appended,
// and that the same correction appears on the emitted log record.
//
// Both halves are asserted because the log record is the only channel a headless
// caller reading a persisted log has; grading the tool result alone leaves the
// channel the fix instruction exists for ungraded.
func TestAddAttachment_GraphFailureCarriesFixOnBothChannels(t *testing.T) {
	var logged bytes.Buffer
	restore := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	defer slog.SetDefault(restore)

	rec := &attachmentRecorder{isDraft: true, subject: "Notes", failPostStatus: http.StatusForbidden}
	ctx, handler := newAddAttachmentFixture(t, rec, 0)

	result := callAddAttachment(t, ctx, handler, map[string]any{
		"message_id":    "draft-1",
		"name":          "notes.txt",
		"content_bytes": base64.StdEncoding.EncodeToString([]byte("hello")),
	})
	if !result.IsError {
		t.Fatalf("a refused upload was confirmed as added: %s", resultText(t, result))
	}
	if !strings.Contains(resultText(t, result), "content_bytes") {
		t.Errorf("the failure does not name what to correct, got: %s", resultText(t, result))
	}
	if !strings.Contains(logged.String(), attachmentFixInstruction) {
		t.Errorf("expected the log record to carry the same correction as the tool result, got: %q", logged.String())
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
			rec := &attachmentRecorder{
				isDraft: true, subject: "Notes", returnsID: "att-1",
				upload: &uploadServer{locationID: "att-large"},
			}
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

// TestAddAttachment_RefusesAboveServiceCeiling verifies a payload above the
// documented 150 MB upload limit is refused before any Graph request even when
// the configured bound is unlimited, since the service would reject the upload
// session anyway.
func TestAddAttachment_RefusesAboveServiceCeiling(t *testing.T) {
	if testing.Short() {
		t.Skip("allocates a payload above the service ceiling")
	}
	rec := &attachmentRecorder{isDraft: true, subject: "Notes", returnsID: "att-1"}
	ctx, handler := newAddAttachmentFixture(t, rec, 0)

	result := callAddAttachment(t, ctx, handler, map[string]any{
		"message_id":    "draft-1",
		"name":          "huge.bin",
		"content_bytes": base64.StdEncoding.EncodeToString(make([]byte, graphAttachmentCeilingBytes+1)),
	})
	if !result.IsError {
		t.Fatal("expected a refusal above the service ceiling")
	}
	text := resultText(t, result)
	for _, want := range []string{"150 MB", "35 MB"} {
		if !strings.Contains(text, want) {
			t.Errorf("error missing %q, got: %s", want, text)
		}
	}
	if rec.gets != 0 || rec.posts != 0 {
		t.Errorf("expected no Graph request, got %d GET and %d POST", rec.gets, rec.posts)
	}
}

// TestAttachmentOverServiceCeiling_Boundary verifies the ceiling is inclusive:
// a payload of exactly 150 MB is allowed and one byte more is refused.
func TestAttachmentOverServiceCeiling_Boundary(t *testing.T) {
	if msg := attachmentOverServiceCeiling(graphAttachmentCeilingBytes); msg != "" {
		t.Errorf("payload at the ceiling refused: %s", msg)
	}
	if msg := attachmentOverServiceCeiling(graphAttachmentCeilingBytes + 1); msg == "" {
		t.Error("payload above the ceiling allowed")
	}
}
