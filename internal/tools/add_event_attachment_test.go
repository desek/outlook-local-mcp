// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the calendar add_event_attachment handler: both
// transfer paths and their request counts, the subject fetch that precedes any
// byte moving, the validation that precedes any Graph request at all, and the
// two properties that make the chunked path safe to expose, namely that the
// confirmed identifier comes from the service and that the pre-authenticated
// upload URL reaches neither channel.
//
// @agents-index: Tests for the calendar.add_event_attachment handler, covering
// the direct and chunked paths, the subject fetch, the validation bounds, and
// upload-URL redaction.
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

// eventAttachmentRecorder records the requests the event add handler issues, so
// a test can assert not only what came back but how many round trips produced
// it. The counts are what pin "before any Graph request" and "no bytes moved":
// an assertion on the result alone cannot distinguish a refusal from a call
// that happened and was then discarded.
type eventAttachmentRecorder struct {
	// gets counts event lookups, which pins the subject fetch at exactly one.
	gets int

	// posts counts attachment POSTs, excluding createUploadSession, so the two
	// transfer paths are distinguishable from the request record alone.
	posts int

	// lastPost holds the decoded body of the last attachment POST.
	lastPost map[string]any

	// subject is the subject the event lookup reports.
	subject string

	// returnsID is the attachment identifier the direct POST responds with.
	returnsID string

	// failGetStatus, when non-zero, answers the event lookup with that status,
	// which is the failure that must refuse the call before any byte moves.
	failGetStatus int

	// upload serves the chunked path when a test exercises it, and is nil for
	// the direct-path tests.
	upload *uploadServer
}

// handler serves the event lookup, the attachment POST, the upload-session
// creation, and the chunk PUTs, recording each.
func (r *eventAttachmentRecorder) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case http.MethodGet:
			r.gets++
			if r.failGetStatus != 0 {
				w.WriteHeader(r.failGetStatus)
				_, _ = w.Write([]byte(`{"error":{"code":"ErrorItemNotFound","message":"The specified object was not found in the store."}}`))
				return
			}
			body, _ := json.Marshal(map[string]any{"id": "evt-1", "subject": r.subject})
			_, _ = w.Write(body)
		case http.MethodPost:
			if strings.HasSuffix(req.URL.Path, "/createUploadSession") {
				r.upload.serveSession(w)
				return
			}
			r.posts++
			raw, _ := io.ReadAll(req.Body)
			r.lastPost = map[string]any{}
			_ = json.Unmarshal(raw, &r.lastPost)
			_, _ = w.Write([]byte(`{"@odata.type":"#microsoft.graph.fileAttachment","id":"` +
				r.returnsID + `","name":"agenda.pdf","contentType":"application/pdf","size":5}`))
		case http.MethodPut:
			r.upload.serveChunk(w, req)
		default:
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
		}
	})
}

// newAddEventAttachmentFixture wires a recorder-backed Graph client into a
// context and returns the handler under test.
func newAddEventAttachmentFixture(t *testing.T, rec *eventAttachmentRecorder, maxSize int64) (context.Context, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	t.Helper()
	client, srv := newTestGraphClient(t, rec.handler())
	t.Cleanup(srv.Close)
	if rec.upload != nil {
		// The upload URL is only knowable once the test server is listening,
		// and it carries a query string because the real one carries an access
		// token there, which the redaction test must be able to look for.
		rec.upload.url = srv.URL + "/upload/event-session-1?token=upload-secret"
	}
	ctx := auth.WithGraphClient(context.Background(), client)
	return ctx, NewHandleAddEventAttachment(graph.RetryConfig{}, 30*time.Second, maxSize)
}

// callAddEventAttachment invokes the handler with the given arguments.
func callAddEventAttachment(t *testing.T, ctx context.Context, handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), args map[string]any) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return result
}

// TestAddEventAttachment_DirectUploadPath asserts a small file is clipped to
// the event with one subject GET and one POST, and that the confirmation names
// the subject, the identifiers, the size, and the direct transfer path.
func TestAddEventAttachment_DirectUploadPath(t *testing.T) {
	rec := &eventAttachmentRecorder{subject: "Quarterly review", returnsID: "att-from-service"}
	ctx, handler := newAddEventAttachmentFixture(t, rec, 10485760)

	result := callAddEventAttachment(t, ctx, handler, map[string]any{
		"event_id":      "evt-1",
		"name":          "agenda.pdf",
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
		`Attachment added: "agenda.pdf"`,
		`Event: "Quarterly review"`,
		"Event ID: evt-1",
		"Attachment ID: att-from-service",
		"Size: 5 bytes",
		"Transfer: " + TransferDirect,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("confirmation missing %q, got:\n%s", want, text)
		}
	}
	if got := rec.lastPost["@odata.type"]; got != "#microsoft.graph.fileAttachment" {
		t.Errorf("@odata.type = %v, want #microsoft.graph.fileAttachment", got)
	}
	if _, present := rec.lastPost["isInline"]; present {
		t.Error("the POST set isInline, which the verb publishes no parameter for")
	}
}

// TestAddEventAttachment_UploadSessionPath asserts a payload at the inline
// threshold is carried by one upload session and a sequence of chunks, with no
// direct POST and no further metadata request beyond the subject fetch.
func TestAddEventAttachment_UploadSessionPath(t *testing.T) {
	const size = uploadChunkSizeBytes + inlineAttachmentThresholdBytes
	// The documented event completion names the attachment as a key segment
	// under the event, so the confirmation must report the quoted identifier.
	up := &uploadServer{location: "https://outlook.office.com/api/v2.0/Users('u-1')/Events('evt-1')/Attachments('att-large')"}
	rec := &eventAttachmentRecorder{subject: "Board meeting", upload: up}
	ctx, handler := newAddEventAttachmentFixture(t, rec, 0)

	result := callAddEventAttachment(t, ctx, handler, map[string]any{
		"event_id":      "evt-1",
		"name":          "deck.pptx",
		"content_bytes": base64.StdEncoding.EncodeToString(make([]byte, size)),
	})
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(t, result))
	}
	if rec.gets != 1 {
		t.Errorf("expected exactly one event GET, got %d", rec.gets)
	}
	if rec.posts != 0 {
		t.Errorf("expected no direct attachment POST on the chunked path, got %d", rec.posts)
	}
	if up.sessions != 1 {
		t.Errorf("expected exactly one createUploadSession, got %d", up.sessions)
	}
	if up.received != size {
		t.Errorf("chunks carried %d bytes, want %d", up.received, size)
	}
	for _, r := range up.ranges {
		if !strings.HasPrefix(r, "bytes ") {
			t.Errorf("chunk carried no Content-Range span: %q", r)
		}
	}
	text := resultText(t, result)
	for _, want := range []string{
		`Event: "Board meeting"`,
		"Attachment ID: att-large",
		"Transfer: " + TransferUploadSession,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("confirmation missing %q, got:\n%s", want, text)
		}
	}
}

// TestAddEventAttachment_SubjectFetchPrecedesTransfer asserts the event lookup
// runs before any byte moves and that its failure refuses the call, so a
// caller naming an unreachable event never starts a transfer it would then
// have to reason about the state of.
func TestAddEventAttachment_SubjectFetchPrecedesTransfer(t *testing.T) {
	up := &uploadServer{locationID: "att-large"}
	rec := &eventAttachmentRecorder{failGetStatus: http.StatusNotFound, upload: up}
	ctx, handler := newAddEventAttachmentFixture(t, rec, 0)

	result := callAddEventAttachment(t, ctx, handler, map[string]any{
		"event_id":      "evt-missing",
		"name":          "agenda.pdf",
		"content_bytes": base64.StdEncoding.EncodeToString([]byte("hello")),
	})
	if !result.IsError {
		t.Fatalf("an attachment to a missing event was confirmed as added: %s", resultText(t, result))
	}
	if !strings.Contains(resultText(t, result), "event_id") {
		t.Errorf("the refusal does not name event_id, got: %s", resultText(t, result))
	}
	if rec.posts != 0 || up.sessions != 0 || len(up.ranges) != 0 {
		t.Errorf("bytes moved after a failed event lookup: %d POST, %d sessions, %d chunks",
			rec.posts, up.sessions, len(up.ranges))
	}
}

// TestAddEventAttachment_RejectsBadInputBeforeAnyRequest asserts each required
// and well-formedness rule refuses the call without a single Graph request, and
// that the error names the parameter to supply or correct.
func TestAddEventAttachment_RejectsBadInputBeforeAnyRequest(t *testing.T) {
	valid := base64.StdEncoding.EncodeToString([]byte("hello"))
	cases := []struct {
		name     string
		args     map[string]any
		wantText []string
	}{
		{"missing event id", map[string]any{"name": "a.txt", "content_bytes": valid}, []string{"event_id"}},
		{"oversize event id", map[string]any{"event_id": strings.Repeat("i", 5000), "name": "a.txt", "content_bytes": valid}, []string{"event_id"}},
		{"missing name", map[string]any{"event_id": "evt-1", "content_bytes": valid}, []string{"name"}},
		{"empty name", map[string]any{"event_id": "evt-1", "name": "", "content_bytes": valid}, []string{"name"}},
		{"over-length name", map[string]any{"event_id": "evt-1", "name": strings.Repeat("a", 256), "content_bytes": valid}, []string{"name", "255"}},
		{"missing content bytes", map[string]any{"event_id": "evt-1", "name": "a.txt"}, []string{"content_bytes"}},
		{"invalid base64", map[string]any{"event_id": "evt-1", "name": "a.txt", "content_bytes": "not base64!!"}, []string{"content_bytes"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &eventAttachmentRecorder{subject: "Notes", returnsID: "att-1"}
			ctx, handler := newAddEventAttachmentFixture(t, rec, 10485760)

			result := callAddEventAttachment(t, ctx, handler, tc.args)
			if !result.IsError {
				t.Fatal("expected a validation error")
			}
			for _, want := range tc.wantText {
				if !strings.Contains(resultText(t, result), want) {
					t.Errorf("error does not name %q, got: %s", want, resultText(t, result))
				}
			}
			if rec.gets != 0 || rec.posts != 0 {
				t.Errorf("expected no Graph request, got %d GET and %d POST", rec.gets, rec.posts)
			}
		})
	}
}

// TestAddEventAttachment_RefusesOverSizeCeiling asserts a payload past the
// configured bound is refused before any Graph request, with an error naming
// the measured size, the bound, and the variable that raises it.
func TestAddEventAttachment_RefusesOverSizeCeiling(t *testing.T) {
	rec := &eventAttachmentRecorder{subject: "Notes", returnsID: "att-1"}
	ctx, handler := newAddEventAttachmentFixture(t, rec, 8)

	result := callAddEventAttachment(t, ctx, handler, map[string]any{
		"event_id":      "evt-1",
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

// TestAddEventAttachment_ConfirmationUsesGraphResponse asserts the confirmation
// reports the identifier the service minted rather than echoing any request
// argument, which is the only way the caller can address the attachment later.
func TestAddEventAttachment_ConfirmationUsesGraphResponse(t *testing.T) {
	rec := &eventAttachmentRecorder{subject: "Notes", returnsID: "service-minted-id"}
	ctx, handler := newAddEventAttachmentFixture(t, rec, 10485760)

	result := callAddEventAttachment(t, ctx, handler, map[string]any{
		"event_id":      "evt-1",
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

// TestAddEventAttachment_UploadURLNeverEscapes asserts the pre-authenticated
// upload URL reaches neither the tool result nor the log record when a chunk
// fails. Both channels are asserted because the log is the only one a headless
// caller reading a persisted file has, and the URL's query string is where the
// access token lives.
func TestAddEventAttachment_UploadURLNeverEscapes(t *testing.T) {
	var logged bytes.Buffer
	restore := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	defer slog.SetDefault(restore)

	up := &uploadServer{locationID: "att-large", failStatus: http.StatusForbidden}
	rec := &eventAttachmentRecorder{subject: "Board meeting", upload: up}
	ctx, handler := newAddEventAttachmentFixture(t, rec, 0)

	result := callAddEventAttachment(t, ctx, handler, map[string]any{
		"event_id":      "evt-1",
		"name":          "deck.pptx",
		"content_bytes": base64.StdEncoding.EncodeToString(make([]byte, inlineAttachmentThresholdBytes)),
	})
	if !result.IsError {
		t.Fatalf("a failed chunk transfer was confirmed as added: %s", resultText(t, result))
	}
	for channel, text := range map[string]string{"tool result": resultText(t, result), "log record": logged.String()} {
		if strings.Contains(text, up.url) || strings.Contains(text, "upload-secret") {
			t.Errorf("the upload URL survived into the %s: %s", channel, text)
		}
	}
	if !strings.Contains(resultText(t, result), addEventAttachmentUploadFix) {
		t.Errorf("the failure carries no fix instruction: %s", resultText(t, result))
	}
}

// TestAddEventAttachment_RefusesAboveServiceCeiling verifies a payload above
// the documented 150 MB upload limit is refused before any Graph request even
// when the configured bound is unlimited.
func TestAddEventAttachment_RefusesAboveServiceCeiling(t *testing.T) {
	if testing.Short() {
		t.Skip("allocates a payload above the service ceiling")
	}
	rec := &eventAttachmentRecorder{subject: "Notes", returnsID: "att-1"}
	ctx, handler := newAddEventAttachmentFixture(t, rec, 0)

	result := callAddEventAttachment(t, ctx, handler, map[string]any{
		"event_id":      "evt-1",
		"name":          "huge.bin",
		"content_bytes": base64.StdEncoding.EncodeToString(make([]byte, graphAttachmentCeilingBytes+1)),
	})
	if !result.IsError {
		t.Fatal("expected a refusal above the service ceiling")
	}
	if text := resultText(t, result); !strings.Contains(text, "150 MB") {
		t.Errorf("error does not name the documented limit: %s", text)
	}
	if rec.gets != 0 || rec.posts != 0 {
		t.Errorf("expected no Graph request, got %d GET and %d POST", rec.gets, rec.posts)
	}
}
