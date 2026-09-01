// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the chunked upload-session transfer used when a draft
// attachment is too large for a single POST. It is the one place in the server
// that issues a raw HTTP request outside the SDK request-builder chain: the
// session is created through the SDK, but the bytes are written by PUT to the
// pre-authenticated upload URL the service returns, which the SDK does not
// transfer for us.
//
// That upload URL carries an access token in its query string, so every error
// this file returns is passed through a redactor before it leaves: the URL must
// reach neither the tool result nor the log record. A transfer that does not
// complete, including one whose final response names no attachment, is reported
// as a failure, because the service discards an unfinished session and an
// identifier reported for it would not resolve.
//
// @agents-index: Chunked upload-session transfer for a large draft attachment,
// creating the session through the SDK and PUTting the bytes to the upload URL
// in fixed-size chunks with Content-Range.
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

const (
	// uploadChunkSizeBytes is the size of every chunk but the last. The service
	// requires a multiple of 320 KiB, and this is five of them (1638400 bytes),
	// which keeps the round-trip count low for the sizes this bound admits
	// while staying far below the memory the fully buffered payload already
	// costs.
	uploadChunkSizeBytes = 5 * 320 * 1024

	// redactedUploadURLPlaceholder replaces the pre-authenticated upload URL
	// wherever it would otherwise appear in an error. The URL carries an access
	// token, so it is redacted rather than truncated: a prefix of it is still a
	// credential.
	redactedUploadURLPlaceholder = "[upload URL redacted]"

	// uploadErrorBodyLimit bounds how much of a failing chunk response is read
	// into the error message, so a large error page cannot flood the result.
	uploadErrorBodyLimit = 2048
)

// uploadAttachmentSession creates an upload session for a draft and transfers
// the attachment bytes to it in chunks, returning the identifier the service
// assigns to the created attachment.
//
// The session is created through the SDK so that retry, timeout, and error
// redaction behave exactly as they do for every other Graph call. The chunk
// transfer is raw HTTP against the URL the session returns, because that URL is
// pre-authenticated and outside the request-builder chain; each chunk carries
// its own timeout so a stalled transfer cannot hang the handler.
//
// Parameters:
//   - client: the resolved Graph client for the active account.
//   - retryCfg: retry configuration applied to the session-creation POST.
//   - timeout: the bound applied to the session-creation POST and, separately,
//     to each individual chunk PUT.
//   - messageID: the draft the attachment is added to.
//   - name: the attachment file name.
//   - mimeType: the attachment content type.
//   - content: the decoded attachment bytes.
//
// Returns the service-assigned attachment identifier.
//
// Side effects: POSTs to /me/messages/{id}/attachments/createUploadSession and
// then PUTs the content to the returned upload URL.
//
// Errors: a failed session creation is returned unwrapped so the caller can
// still classify it as a timeout; a failed or incomplete transfer is returned
// with the upload URL already redacted.
func uploadAttachmentSession(
	ctx context.Context,
	client *msgraphsdk.GraphServiceClient,
	retryCfg graph.RetryConfig,
	timeout time.Duration,
	messageID, name, mimeType string,
	content []byte,
) (string, error) {
	uploadURL, err := createAttachmentUploadSession(ctx, client, retryCfg, timeout, messageID, name, mimeType, int64(len(content)))
	if err != nil {
		return "", err
	}

	return transferAttachmentChunks(ctx, uploadURL, content, timeout)
}

// createAttachmentUploadSession asks the service for an upload session for a
// file attachment of the given name, type, and size, and returns the upload URL
// it hands back.
//
// Errors: the Graph error is returned unwrapped, so a timeout stays
// recognisable to the caller; a session that carries no upload URL is an error
// rather than an empty transfer.
func createAttachmentUploadSession(
	ctx context.Context,
	client *msgraphsdk.GraphServiceClient,
	retryCfg graph.RetryConfig,
	timeout time.Duration,
	messageID, name, mimeType string,
	size int64,
) (string, error) {
	item := models.NewAttachmentItem()
	fileType := models.FILE_ATTACHMENTTYPE
	item.SetAttachmentType(&fileType)
	item.SetName(&name)
	item.SetContentType(&mimeType)
	item.SetSize(&size)

	body := users.NewItemMessagesItemAttachmentsCreateUploadSessionPostRequestBody()
	body.SetAttachmentItem(item)

	timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
	defer cancel()

	var session models.UploadSessionable
	err := graph.RetryGraphCall(ctx, retryCfg, func() error {
		var gErr error
		session, gErr = client.Me().Messages().ByMessageId(messageID).
			Attachments().CreateUploadSession().Post(timeoutCtx, body, nil)
		return gErr
	})
	if err != nil {
		return "", err
	}
	if session == nil || graph.SafeStr(session.GetUploadUrl()) == "" {
		return "", errors.New("the upload session the service created carries no upload URL, so the attachment cannot be transferred")
	}
	return graph.SafeStr(session.GetUploadUrl()), nil
}

// transferAttachmentChunks writes the content to the upload URL in
// uploadChunkSizeBytes chunks, the last one short, and returns the attachment
// identifier the service reports when the final chunk completes the transfer.
//
// Each chunk carries a Content-Range naming its inclusive byte span and the
// total size, which is how the service reassembles the file and how it knows
// the transfer is finished.
//
// Errors: every error is redacted of the upload URL before it is returned. A
// transfer whose final response names no attachment is an error, not a partial
// success.
func transferAttachmentChunks(ctx context.Context, uploadURL string, content []byte, timeout time.Duration) (string, error) {
	httpClient := &http.Client{}
	total := len(content)
	attachmentID := ""

	for start := 0; start < total; start += uploadChunkSizeBytes {
		end := start + uploadChunkSizeBytes
		if end > total {
			end = total
		}
		id, err := putAttachmentChunk(ctx, httpClient, uploadURL, content[start:end], start, end, total, timeout)
		if err != nil {
			return "", fmt.Errorf("attachment upload failed for bytes %d-%d of %d: %s", start, end-1, total, redactUploadURL(err.Error(), uploadURL))
		}
		if end == total {
			attachmentID = id
		}
	}

	if attachmentID == "" {
		return "", errors.New("the attachment upload completed without an identifier, so the attachment cannot be confirmed as added")
	}
	return attachmentID, nil
}

// putAttachmentChunk writes one chunk to the upload URL under its own timeout
// and returns the attachment identifier when the response is the one that
// completes the transfer.
//
// Parameters end is exclusive; the Content-Range header the service expects is
// inclusive, so the header names end-1.
//
// Errors: a non-2xx response is an error naming the status and the body the
// service returned, both of which the caller redacts.
func putAttachmentChunk(
	ctx context.Context,
	httpClient *http.Client,
	uploadURL string,
	chunk []byte,
	start, end, total int,
	timeout time.Duration,
) (string, error) {
	reqCtx, cancel := graph.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPut, uploadURL, bytes.NewReader(chunk))
	if err != nil {
		return "", err
	}
	req.ContentLength = int64(len(chunk))
	req.Header.Set("Content-Type", defaultAttachmentMimeType)
	req.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end-1, total))

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	payload, _ := io.ReadAll(io.LimitReader(resp.Body, uploadErrorBodyLimit))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("the service answered %s: %s", resp.Status, strings.TrimSpace(string(payload)))
	}

	if end != total {
		return "", nil
	}
	return completedAttachmentID(resp, payload), nil
}

// completedAttachmentID reads the identifier of the created attachment from the
// response that completed the transfer. The service reports it in the Location
// header of the 201 it returns on the final chunk; the body is consulted only
// where a response carries one, because not every completion does.
//
// Returns an empty string when neither source names an identifier, which the
// caller treats as a failed transfer rather than a success.
func completedAttachmentID(resp *http.Response, payload []byte) string {
	if location := resp.Header.Get("Location"); location != "" {
		if id := attachmentIDFromLocation(location); id != "" {
			return id
		}
	}
	var decoded struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(payload, &decoded); err == nil {
		return decoded.ID
	}
	return ""
}

// attachmentIDFromLocation extracts the attachment identifier from the resource
// URL the Location header names. The header addresses the created attachment,
// so the identifier is its final path segment; it is percent-decoded because a
// Graph attachment identifier contains characters that are escaped there.
func attachmentIDFromLocation(location string) string {
	parsed, err := url.Parse(location)
	if err != nil {
		return ""
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	last := segments[len(segments)-1]
	if unescaped, err := url.PathUnescape(last); err == nil {
		return unescaped
	}
	return last
}

// redactUploadURL removes the pre-authenticated upload URL from a message. The
// URL and its query string are replaced separately, because a transport error
// quotes the whole URL while a service error body may quote only part of it,
// and the query string is where the access token lives.
func redactUploadURL(message, uploadURL string) string {
	redacted := strings.ReplaceAll(message, uploadURL, redactedUploadURLPlaceholder)
	if parsed, err := url.Parse(uploadURL); err == nil {
		if parsed.RawQuery != "" {
			redacted = strings.ReplaceAll(redacted, parsed.RawQuery, redactedUploadURLPlaceholder)
		}
		parsed.RawQuery = ""
		redacted = strings.ReplaceAll(redacted, parsed.String(), redactedUploadURLPlaceholder)
	}
	return redacted
}
