// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the chunked upload-session path of the mail
// domain's add_attachment handler: the session creation, the chunk sizing and
// Content-Range headers, the identifier read from the completion response, and
// the two properties that make the path safe to expose, namely that a failed
// transfer is not reported as a success and that the pre-authenticated upload
// URL never reaches the result.
package tools

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

// uploadServer serves the createUploadSession response and the chunk PUTs that
// follow it, recording what the handler sent. It records the Content-Range of
// every chunk and the byte count in total, because the sizing contract is not
// observable from the confirmation the handler returns.
type uploadServer struct {
	// url is the upload URL handed back in the session response. The fixture
	// fills it in once the test server is listening.
	url string

	// sessions counts createUploadSession requests, which pins that the large
	// path creates exactly one.
	sessions int

	// ranges holds the Content-Range header of every chunk, in order.
	ranges []string

	// received counts the bytes the chunks carried in total.
	received int

	// locationID is the attachment identifier named in the Location header of
	// the response that completes the transfer.
	locationID string

	// location, when set, is the full Location header value, overriding the
	// default documented message form built from locationID. Event tests set
	// the documented event form, and one case sets the plain path form.
	location string

	// omitLocation suppresses the Location header on the completing response,
	// leaving the transfer with no identifier.
	omitLocation bool

	// failStatus, when non-zero, makes every chunk fail with that status and a
	// body quoting the upload URL, which is how a service error can carry the
	// credential the redaction must remove.
	failStatus int

	// stallChunk holds every chunk PUT open for this long, so a test can drive
	// the transfer past the per-chunk bound.
	stallChunk time.Duration
}

// serveSession answers a createUploadSession request. A nil receiver means the
// test did not set up the chunked path, and refusing is the honest answer.
func (u *uploadServer) serveSession(w http.ResponseWriter) {
	if u == nil {
		http.Error(w, "no upload session configured", http.StatusMethodNotAllowed)
		return
	}
	u.sessions++
	_, _ = fmt.Fprintf(w, `{"uploadUrl":%q,"expirationDateTime":"2030-01-01T00:00:00Z"}`, u.url)
}

// serveChunk answers one chunk PUT, recording its range and size, and completes
// the transfer on the chunk whose range ends at the total.
func (u *uploadServer) serveChunk(w http.ResponseWriter, req *http.Request) {
	if u == nil {
		http.Error(w, "no upload session configured", http.StatusMethodNotAllowed)
		return
	}
	contentRange := req.Header.Get("Content-Range")
	u.ranges = append(u.ranges, contentRange)
	u.received += int(req.ContentLength)

	// The body is drained before stalling so the server can notice the client
	// abandoning the chunk, rather than waiting out its own stall.
	_, _ = io.Copy(io.Discard, req.Body)
	stall(req, u.stallChunk)
	if u.failStatus != 0 {
		http.Error(w, "the upload to "+u.url+" was rejected", u.failStatus)
		return
	}
	if !isFinalContentRange(contentRange) {
		w.WriteHeader(http.StatusOK)
		return
	}
	if !u.omitLocation {
		location := u.location
		if location == "" {
			// The documented completion names the attachment as an OData key
			// segment, the identifier quoted inside the parentheses.
			location = "https://outlook.office.com/api/v2.0/Users('u-1')/Messages('draft-1')/Attachments('" + u.locationID + "')"
		}
		w.Header().Set("Location", location)
	}
	w.WriteHeader(http.StatusCreated)
}

// isFinalContentRange reports whether a "bytes start-end/total" header names the
// chunk that completes the transfer, which is the one whose inclusive end is the
// last byte of the total.
func isFinalContentRange(header string) bool {
	span, total, ok := strings.Cut(strings.TrimPrefix(header, "bytes "), "/")
	if !ok {
		return false
	}
	_, end, ok := strings.Cut(span, "-")
	if !ok {
		return false
	}
	endValue, err := strconv.Atoi(end)
	if err != nil {
		return false
	}
	totalValue, err := strconv.Atoi(total)
	if err != nil {
		return false
	}
	return endValue+1 == totalValue
}

// largeAttachmentArgs builds the handler arguments for a payload of the given
// decoded size.
func largeAttachmentArgs(size int) map[string]any {
	return map[string]any{
		"message_id":    "draft-1",
		"name":          "archive.zip",
		"mime_type":     "application/zip",
		"content_bytes": base64.StdEncoding.EncodeToString(make([]byte, size)),
	}
}

// TestUploadSession_LargeFileChunks verifies that a payload at or above the
// inline threshold is carried by one upload session and a sequence of chunks
// that are all the fixed chunk size except the last, each announcing its
// inclusive byte span and the total.
func TestUploadSession_LargeFileChunks(t *testing.T) {
	const size = 2*uploadChunkSizeBytes + 12345
	up := &uploadServer{locationID: "att-large"}
	rec := &attachmentRecorder{isDraft: true, subject: "Quarterly", upload: up}
	ctx, handler := newAddAttachmentFixture(t, rec, 0)

	result := callAddAttachment(t, ctx, handler, largeAttachmentArgs(size))
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	if up.sessions != 1 {
		t.Errorf("createUploadSession requests = %d, want 1", up.sessions)
	}
	if rec.posts != 0 {
		t.Errorf("direct attachment POSTs = %d, want 0", rec.posts)
	}
	want := []string{
		fmt.Sprintf("bytes 0-%d/%d", uploadChunkSizeBytes-1, size),
		fmt.Sprintf("bytes %d-%d/%d", uploadChunkSizeBytes, 2*uploadChunkSizeBytes-1, size),
		fmt.Sprintf("bytes %d-%d/%d", 2*uploadChunkSizeBytes, size-1, size),
	}
	if len(up.ranges) != len(want) {
		t.Fatalf("chunks = %d (%v), want %d", len(up.ranges), up.ranges, len(want))
	}
	for i, w := range want {
		if up.ranges[i] != w {
			t.Errorf("chunk %d Content-Range = %q, want %q", i, up.ranges[i], w)
		}
	}
	if up.received != size {
		t.Errorf("bytes transferred = %d, want %d", up.received, size)
	}
	if text := resultText(t, result); !strings.Contains(text, TransferUploadSession) {
		t.Errorf("confirmation does not name the chunked path: %s", text)
	}
}

// TestUploadSession_AttachmentIDFromLocationHeader verifies that the identifier
// the confirmation reports is the one the completing response names, and that a
// completion naming none is a failure rather than a success with a blank id.
func TestUploadSession_AttachmentIDFromLocationHeader(t *testing.T) {
	t.Run("the identifier comes from the Location header", func(t *testing.T) {
		up := &uploadServer{locationID: "AAMkAGI%3D%3D"}
		rec := &attachmentRecorder{isDraft: true, subject: "Quarterly", upload: up}
		ctx, handler := newAddAttachmentFixture(t, rec, 0)

		result := callAddAttachment(t, ctx, handler, largeAttachmentArgs(inlineAttachmentThresholdBytes))
		if result.IsError {
			t.Fatalf("expected success, got error: %s", resultText(t, result))
		}
		// The header percent-escapes the identifier, and the confirmation must
		// report the identifier a caller can pass back, not the escaped form.
		if text := resultText(t, result); !strings.Contains(text, "Attachment ID: AAMkAGI==") {
			t.Errorf("confirmation does not report the identifier from the Location header: %s", text)
		}
	})

	t.Run("an identifier containing both = and %3D is decoded", func(t *testing.T) {
		up := &uploadServer{locationID: "AAMkADI5MAAIT3drCAAABEgAQ=%3D"}
		rec := &attachmentRecorder{isDraft: true, subject: "Quarterly", upload: up}
		ctx, handler := newAddAttachmentFixture(t, rec, 0)

		result := callAddAttachment(t, ctx, handler, largeAttachmentArgs(inlineAttachmentThresholdBytes))
		if text := resultText(t, result); result.IsError || !strings.Contains(text, "Attachment ID: AAMkADI5MAAIT3drCAAABEgAQ==") {
			t.Errorf("confirmation does not report the decoded key: %s", text)
		}
		if text := resultText(t, result); strings.Contains(text, "Attachments(") {
			t.Errorf("confirmation reports the OData key segment, not the identifier: %s", text)
		}
	})

	t.Run("the plain path form is still read", func(t *testing.T) {
		up := &uploadServer{location: "https://graph.microsoft.com/v1.0/me/messages/draft-1/attachments/att-plain"}
		rec := &attachmentRecorder{isDraft: true, subject: "Quarterly", upload: up}
		ctx, handler := newAddAttachmentFixture(t, rec, 0)

		result := callAddAttachment(t, ctx, handler, largeAttachmentArgs(inlineAttachmentThresholdBytes))
		if text := resultText(t, result); result.IsError || !strings.Contains(text, "Attachment ID: att-plain") {
			t.Errorf("confirmation does not report the plain path identifier: %s", text)
		}
	})

	t.Run("a completion naming no attachment is an error", func(t *testing.T) {
		up := &uploadServer{omitLocation: true}
		rec := &attachmentRecorder{isDraft: true, subject: "Quarterly", upload: up}
		ctx, handler := newAddAttachmentFixture(t, rec, 0)

		result := callAddAttachment(t, ctx, handler, largeAttachmentArgs(inlineAttachmentThresholdBytes))
		if !result.IsError {
			t.Fatalf("expected an error, got: %s", resultText(t, result))
		}
		if text := resultText(t, result); strings.Contains(text, "Attachment added") {
			t.Errorf("a transfer without an identifier was confirmed as added: %s", text)
		}
	})
}

// TestUploadSession_TransferFailureIsNotPartialSuccess verifies that a chunk the
// service rejects fails the whole verb: the bytes the service holds are
// discarded when the session expires, so a confirmation would name an
// attachment that does not resolve.
func TestUploadSession_TransferFailureIsNotPartialSuccess(t *testing.T) {
	up := &uploadServer{locationID: "att-large", failStatus: http.StatusInternalServerError}
	rec := &attachmentRecorder{isDraft: true, subject: "Quarterly", upload: up}
	ctx, handler := newAddAttachmentFixture(t, rec, 0)

	result := callAddAttachment(t, ctx, handler, largeAttachmentArgs(inlineAttachmentThresholdBytes))
	if !result.IsError {
		t.Fatalf("expected an error, got: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if strings.Contains(text, "Attachment added") {
		t.Errorf("a failed transfer was confirmed as added: %s", text)
	}
	if !strings.Contains(text, "not added") {
		t.Errorf("the error does not state the attachment was not added: %s", text)
	}
}

// TestUploadSession_ErrorRedactsUploadURL verifies that neither the upload URL
// nor the credential in its query string survives into the tool result or the
// log record, even when the service quotes the URL back in its own error body.
//
// Both channels are asserted because the log record is safe only by an ordering
// invariant: the transfer redacts before the handler logs. Moving the redaction,
// or adding a second log call on the raw error, would write a live access token
// into a persisted file and pass a result-only assertion.
func TestUploadSession_ErrorRedactsUploadURL(t *testing.T) {
	var logged bytes.Buffer
	restore := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	defer slog.SetDefault(restore)

	up := &uploadServer{locationID: "att-large", failStatus: http.StatusForbidden}
	rec := &attachmentRecorder{isDraft: true, subject: "Quarterly", upload: up}
	ctx, handler := newAddAttachmentFixture(t, rec, 0)

	result := callAddAttachment(t, ctx, handler, largeAttachmentArgs(inlineAttachmentThresholdBytes))
	if !result.IsError {
		t.Fatalf("expected an error, got: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if strings.Contains(text, up.url) {
		t.Errorf("the upload URL reached the result: %s", text)
	}
	if strings.Contains(text, "upload-secret") {
		t.Errorf("the upload credential reached the result: %s", text)
	}
	if !strings.Contains(text, redactedUploadURLPlaceholder) {
		t.Errorf("the error does not mark the redaction: %s", text)
	}
	if strings.Contains(logged.String(), up.url) {
		t.Errorf("the upload URL reached the log record: %s", logged.String())
	}
	if strings.Contains(logged.String(), "upload-secret") {
		t.Errorf("the upload credential reached the log record: %s", logged.String())
	}
	if !strings.Contains(logged.String(), attachmentFixInstruction) {
		t.Errorf("expected the log record to carry the correction, got: %s", logged.String())
	}
}
