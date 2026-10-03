// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the calendar domain's add_event_attachment handler, which
// clips a file to an existing event. The caller supplies the file as base64 in
// one argument, so the whole payload is held in memory and the configured
// maximum attachment size is what bounds that footprint: the size is measured
// from the decoded bytes and refused before any Graph request is issued.
//
// It is the event-side mirror of the mail add verb, and reuses that verb's
// threshold, its default content type, and its chunk transfer unchanged. Two
// things differ. There is no draft guard, because an event has no draft state
// to be in, so the one GET this handler pays exists solely to read the subject
// the confirmation names, and its failure refuses the call before any bytes
// move rather than being a guard the caller could bypass. And the upload
// session is created through the event navigation, which the SDK exposes as a
// distinct generated type.
//
// @agents-index: Handler constructor for the calendar.add_event_attachment
// verb, validating and sizing the payload, reading the event subject, and
// routing the bytes to the direct or chunked transfer path.
package tools

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/logging"
	"github.com/desek/outlook-local-mcp/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

// Corrections appended to this verb's refusals, whether it raises them itself
// or delegates to a shared helper. The identifier validator, the base64
// decoder, the timeout helper, and the redactor each state a diagnosis without
// a next step, because none of them knows which verb shaped the request, so the
// correction is authored here and travels to both the tool result and the log
// record.
const (
	addEventAttachmentEventIDFix = "supply event_id as an identifier returned by calendar list_events, search_events, or get_event"
	addEventAttachmentNameFix    = "supply name as the file name the attachment should carry, at most 255 characters"
	addEventAttachmentContentFix = "supply content_bytes as the file content encoded with standard base64"
	addEventAttachmentEventFix   = "check that event_id names an event this account can edit, re-resolving it with calendar search_events, then retry; nothing was uploaded"
	addEventAttachmentTimeoutFix = "retry the call, and if it times out again list the event's attachments with calendar list_event_attachments to check whether the file arrived before adding it a second time"
	addEventAttachmentUploadFix  = "the attachment was not added; retry the call, and confirm with calendar list_event_attachments before adding it a second time"
)

// NewHandleAddEventAttachment creates the MCP tool handler for the
// calendar.add_event_attachment verb. It requires event_id, name, and
// content_bytes, validates and decodes all three before any Graph request is
// issued, refuses a payload larger than the configured bound, reads the event's
// subject, and then transfers the bytes on the path their size selects.
//
// Every value the confirmation reports about the created attachment comes from
// the Graph response, because the service mints the attachment identifier and
// the caller cannot otherwise learn it.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for a single Graph API call.
//   - maxSize: the maximum allowed decoded attachment size in bytes, from
//     config.MaxAttachmentSizeBytes. Values <=0 are treated as unlimited.
//
// Returns a handler function compatible with the MCP server AddTool signature.
//
// Side effects: calls GET /me/events/{id} and then, on the direct path,
// POST /me/events/{id}/attachments, or on the chunked path
// POST /me/events/{id}/attachments/createUploadSession followed by the chunk
// PUTs the session's upload URL accepts.
func NewHandleAddEventAttachment(retryCfg graph.RetryConfig, timeout time.Duration, maxSize int64) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)

		client, err := GraphClient(ctx)
		if err != nil {
			return mcp.NewToolResultError("no account selected"), nil
		}

		eventID, err := request.RequireString("event_id")
		if err != nil || eventID == "" {
			logger.Warn("parameter validation failed", "parameter", "event_id", "fix", addEventAttachmentEventIDFix)
			return mcp.NewToolResultError(fmt.Sprintf("missing required parameter: event_id: %s", addEventAttachmentEventIDFix)), nil
		}
		if err := validate.ValidateResourceID(eventID, "event_id"); err != nil {
			logger.Warn("parameter validation failed", "parameter", "event_id", "fix", addEventAttachmentEventIDFix, "error", err.Error())
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), addEventAttachmentEventIDFix)), nil
		}

		name, err := request.RequireString("name")
		if err != nil || name == "" {
			logger.Warn("parameter validation failed", "parameter", "name", "fix", addEventAttachmentNameFix)
			return mcp.NewToolResultError(fmt.Sprintf("missing required parameter: name: %s", addEventAttachmentNameFix)), nil
		}
		if err := validate.ValidateStringLength(name, "name", validate.MaxAttachmentNameLen); err != nil {
			logger.Warn("parameter validation failed", "parameter", "name", "fix", addEventAttachmentNameFix, "error", err.Error())
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), addEventAttachmentNameFix)), nil
		}

		encoded, err := request.RequireString("content_bytes")
		if err != nil {
			logger.Warn("parameter validation failed", "parameter", "content_bytes", "fix", addEventAttachmentContentFix)
			return mcp.NewToolResultError(fmt.Sprintf("missing required parameter: content_bytes: %s", addEventAttachmentContentFix)), nil
		}
		decoded, err := validate.ValidateBase64(encoded, "content_bytes")
		if err != nil {
			logger.Warn("parameter validation failed", "parameter", "content_bytes", "fix", addEventAttachmentContentFix, "error", err.Error())
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), addEventAttachmentContentFix)), nil
		}

		// The ceiling is measured from the decoded bytes rather than the base64
		// text, because the decoded length is what the service stores and what
		// the handler holds in memory.
		size := len(decoded)
		// The service ceiling applies whatever the configured bound is,
		// including an unlimited one, so no out-of-range upload session is
		// ever requested.
		if msg := attachmentOverServiceCeiling(size); msg != "" {
			logger.Warn("attachment exceeds service ceiling", "size", size, "max", int64(graphAttachmentCeilingBytes), "fix", msg)
			return mcp.NewToolResultError(msg), nil
		}
		if maxSize > 0 && int64(size) > maxSize {
			fix := fmt.Sprintf("raise OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES above %d bytes and restart the server to upload a file this large", size)
			logger.Warn("attachment exceeds maximum size", "size", size, "max", maxSize, "fix", fix)
			return mcp.NewToolResultError(fmt.Sprintf(
				"attachment size %d bytes exceeds maximum allowed %d bytes; %s", size, maxSize, fix)), nil
		}

		mimeType, _ := request.GetArguments()["mime_type"].(string)
		if mimeType == "" {
			mimeType = defaultAttachmentMimeType
		}

		subject, errResult := fetchEventSubject(ctx, client, retryCfg, timeout, eventID, logger)
		if errResult != nil {
			return errResult, nil
		}

		if size >= inlineAttachmentThresholdBytes {
			// The chunked upload session is the only transfer the service
			// accepts at this size: a direct POST of these bytes is rejected.
			uploadURL, err := createEventAttachmentUploadSession(ctx, client, retryCfg, timeout, eventID, name, mimeType, int64(size))
			var attachmentID string
			if err == nil {
				attachmentID, err = transferAttachmentChunks(ctx, uploadURL, decoded, timeout)
			}
			if err != nil {
				if graph.IsTimeoutError(err) {
					logger.ErrorContext(ctx, "request timed out",
						"timeout_seconds", int(timeout.Seconds()),
						"fix", addEventAttachmentTimeoutFix,
						"error", err.Error())
					return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
						graph.TimeoutErrorMessage(int(timeout.Seconds())), addEventAttachmentTimeoutFix)), nil
				}
				logger.ErrorContext(ctx, "event attachment upload session failed",
					"event_id", eventID,
					"size", size,
					"error", graph.FormatGraphError(err),
					"fix", addEventAttachmentUploadFix)
				return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
					graph.RedactGraphError(err), addEventAttachmentUploadFix)), nil
			}

			logger.InfoContext(ctx, "event attachment added",
				"event_id", eventID,
				"attachment_id", attachmentID,
				"size", size,
				"transfer", TransferUploadSession)

			response := FormatEventAttachmentConfirmation(name, subject, eventID, attachmentID, size, TransferUploadSession)
			if line := AccountInfoLine(ctx); line != "" {
				response += "\n" + line
			}
			return mcp.NewToolResultText(response), nil
		}

		attachment := models.NewFileAttachment()
		attachment.SetName(&name)
		attachment.SetContentType(&mimeType)
		attachment.SetContentBytes(decoded)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		var created models.Attachmentable
		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			var gErr error
			created, gErr = client.Me().Events().ByEventId(eventID).Attachments().Post(timeoutCtx, attachment, nil)
			return gErr
		})
		if err != nil {
			if graph.IsTimeoutError(err) {
				logger.ErrorContext(ctx, "request timed out",
					"timeout_seconds", int(timeout.Seconds()),
					"fix", addEventAttachmentTimeoutFix,
					"error", err.Error())
				return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
					graph.TimeoutErrorMessage(int(timeout.Seconds())), addEventAttachmentTimeoutFix)), nil
			}
			logger.ErrorContext(ctx, "add event attachment failed",
				"event_id", eventID,
				"error", graph.FormatGraphError(err),
				"fix", addEventAttachmentUploadFix)
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
				graph.RedactGraphError(err), addEventAttachmentUploadFix)), nil
		}

		attachmentID := graph.SafeStr(created.GetId())
		logger.InfoContext(ctx, "event attachment added",
			"event_id", eventID,
			"attachment_id", attachmentID,
			"size", size,
			"transfer", TransferDirect)

		response := FormatEventAttachmentConfirmation(name, subject, eventID, attachmentID, size, TransferDirect)
		if line := AccountInfoLine(ctx); line != "" {
			response += "\n" + line
		}
		return mcp.NewToolResultText(response), nil
	}
}

// fetchEventSubject reads the subject of the event the attachment is destined
// for, selecting that one property so the fetch stays cheap.
//
// The fetch is not decoration. It runs before a single byte moves, so an event
// identifier that names nothing this account can reach refuses the call rather
// than failing partway through a chunked transfer that would leave the caller
// unsure whether the file arrived.
//
// Returns the subject, or a non-nil tool result carrying the refusal, never
// both.
func fetchEventSubject(
	ctx context.Context,
	client *msgraphsdk.GraphServiceClient,
	retryCfg graph.RetryConfig,
	timeout time.Duration,
	eventID string,
	logger *slog.Logger,
) (string, *mcp.CallToolResult) {
	timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
	defer cancel()

	config := &users.ItemEventsEventItemRequestBuilderGetRequestConfiguration{
		QueryParameters: &users.ItemEventsEventItemRequestBuilderGetQueryParameters{
			Select: []string{"subject"},
		},
	}

	var event models.Eventable
	err := graph.RetryGraphCall(ctx, retryCfg, func() error {
		var gErr error
		event, gErr = client.Me().Events().ByEventId(eventID).Get(timeoutCtx, config)
		return gErr
	})
	if err != nil {
		if graph.IsTimeoutError(err) {
			logger.ErrorContext(ctx, "request timed out",
				"timeout_seconds", int(timeout.Seconds()),
				"fix", addEventAttachmentTimeoutFix,
				"error", err.Error())
			return "", mcp.NewToolResultError(fmt.Sprintf("%s: %s",
				graph.TimeoutErrorMessage(int(timeout.Seconds())), addEventAttachmentTimeoutFix))
		}
		logger.ErrorContext(ctx, "event lookup failed before attachment upload",
			"event_id", eventID,
			"error", graph.FormatGraphError(err),
			"fix", addEventAttachmentEventFix)
		return "", mcp.NewToolResultError(fmt.Sprintf("%s: %s",
			graph.RedactGraphError(err), addEventAttachmentEventFix))
	}
	return graph.SafeStr(event.GetSubject()), nil
}
