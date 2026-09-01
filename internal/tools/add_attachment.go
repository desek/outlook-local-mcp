// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the mail domain's add_attachment handler, which attaches a
// file to an existing draft. The caller supplies the file as base64 in one
// argument, so the whole payload is held in memory and the configured maximum
// attachment size is what bounds that footprint: the size is measured from the
// decoded bytes and refused before any Graph request is issued.
//
// The transfer path is chosen from the decoded size rather than exposed as a
// parameter, because the choice is a property of the Graph API and not of the
// caller's intent: below the inline threshold the bytes fit in a single POST,
// and at or above it the service requires an upload session.
//
// @agents-index: Handler constructor for the mail.add_attachment verb,
// validating and sizing the payload, guarding the draft, and routing it to the
// direct or chunked transfer path.
package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/logging"
	"github.com/desek/outlook-local-mcp/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

const (
	// inlineAttachmentThresholdBytes is the decoded payload size at which the
	// Graph API stops accepting a file attachment as a single POST body. It is
	// a decimal three million rather than a power-of-two multiple because that
	// is the figure the service documents, and the comparison is strict: a
	// payload of exactly this size already needs an upload session.
	inlineAttachmentThresholdBytes = 3 * 1000 * 1000

	// defaultAttachmentMimeType is the content type applied when the caller
	// omits mime_type. It is the generic binary type, which the service accepts
	// for arbitrary bytes, so an omitted parameter never turns into a rejected
	// upload.
	defaultAttachmentMimeType = "application/octet-stream"
)

// attachmentFixInstruction is the correction appended to a failed attachment
// request and logged alongside it, so a headless caller that cannot read an
// interactive surface still receives it. The two things a caller most often has
// wrong are the draft it named and the encoding of the bytes it sent.
const attachmentFixInstruction = "supply a message_id for a draft from mail.list_messages and file content base64-encoded in content_bytes"

// NewHandleAddAttachment creates the MCP tool handler for the
// mail.add_attachment verb. It requires message_id, name, and content_bytes,
// validates and decodes all three before any Graph request is issued, refuses a
// payload larger than the configured bound, verifies the target is a draft, and
// then transfers the bytes on the path its size selects.
//
// The draft guard supplies the subject the confirmation names, so the handler
// pays one GET rather than two. Every value the confirmation reports about the
// created attachment comes from the Graph response, because the service mints
// the attachment identifier and the caller cannot otherwise learn it.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for a single Graph API call.
//   - maxSize: the maximum allowed decoded attachment size in bytes, from
//     config.MaxAttachmentSizeBytes. Values <=0 are treated as unlimited.
//
// Returns a handler function compatible with the MCP server AddTool signature.
//
// Side effects: calls GET /me/messages/{id} and then, on the direct path,
// POST /me/messages/{id}/attachments, or on the chunked path
// POST /me/messages/{id}/attachments/createUploadSession followed by the chunk
// PUTs the session's upload URL accepts.
func NewHandleAddAttachment(retryCfg graph.RetryConfig, timeout time.Duration, maxSize int64) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)

		client, err := GraphClient(ctx)
		if err != nil {
			return mcp.NewToolResultError("no account selected"), nil
		}

		messageID, err := request.RequireString("message_id")
		if err != nil || messageID == "" {
			return mcp.NewToolResultError(
				"missing required parameter: message_id (obtain a draft ID from mail.list_messages)"), nil
		}
		if err := validate.ValidateResourceID(messageID, "message_id"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		name, err := request.RequireString("name")
		if err != nil || name == "" {
			return mcp.NewToolResultError(
				"missing required parameter: name (supply the file name the attachment should carry)"), nil
		}
		if err := validate.ValidateStringLength(name, "name", validate.MaxAttachmentNameLen); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		encoded, err := request.RequireString("content_bytes")
		if err != nil {
			return mcp.NewToolResultError(
				"missing required parameter: content_bytes (supply the file content as a standard base64-encoded string)"), nil
		}
		decoded, err := validate.ValidateBase64(encoded, "content_bytes")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		size := len(decoded)
		if maxSize > 0 && int64(size) > maxSize {
			logger.WarnContext(ctx, "attachment exceeds maximum size", "size", size, "max", maxSize)
			return mcp.NewToolResultError(fmt.Sprintf(
				"attachment size %d bytes exceeds maximum allowed %d bytes; raise OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES to upload a file this large",
				size, maxSize)), nil
		}

		mimeType, _ := request.GetArguments()["mime_type"].(string)
		if mimeType == "" {
			mimeType = defaultAttachmentMimeType
		}

		draft, errResult := verifyIsDraft(ctx, client, retryCfg, timeout, messageID, logger)
		if errResult != nil {
			return errResult, nil
		}
		subject := graph.SafeStr(draft.GetSubject())

		if size >= inlineAttachmentThresholdBytes {
			// The chunked upload session is the only transfer the service
			// accepts at this size: a direct POST of these bytes is rejected.
			attachmentID, err := uploadAttachmentSession(ctx, client, retryCfg, timeout, messageID, name, mimeType, decoded)
			if err != nil {
				if graph.IsTimeoutError(err) {
					logger.ErrorContext(ctx, "request timed out",
						"timeout_seconds", int(timeout.Seconds()),
						"error", err.Error())
					return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds()))), nil
				}
				logger.ErrorContext(ctx, "attachment upload session failed",
					"message_id", messageID,
					"size", size,
					"error", graph.FormatGraphError(err),
					"fix", attachmentFixInstruction)
				msg := graph.RedactGraphError(err) + "\nThe attachment was not added. Retry the upload, " +
					"or supply a message_id for a draft from mail.list_messages."
				return mcp.NewToolResultError(msg), nil
			}

			logger.InfoContext(ctx, "attachment added",
				"message_id", messageID,
				"attachment_id", attachmentID,
				"size", size,
				"transfer", TransferUploadSession)

			response := FormatAttachmentConfirmation(name, subject, messageID, attachmentID, size, TransferUploadSession)
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
			created, gErr = client.Me().Messages().ByMessageId(messageID).Attachments().Post(timeoutCtx, attachment, nil)
			return gErr
		})
		if err != nil {
			if graph.IsTimeoutError(err) {
				logger.ErrorContext(ctx, "request timed out",
					"timeout_seconds", int(timeout.Seconds()),
					"error", err.Error())
				return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds()))), nil
			}
			logger.ErrorContext(ctx, "add attachment failed",
				"error", graph.FormatGraphError(err),
				"fix", attachmentFixInstruction)
			msg := graph.RedactGraphError(err) + "\nSupply a message_id for a draft from mail.list_messages " +
				"and file content base64-encoded in content_bytes."
			return mcp.NewToolResultError(msg), nil
		}

		attachmentID := graph.SafeStr(created.GetId())
		logger.InfoContext(ctx, "attachment added",
			"message_id", messageID,
			"attachment_id", attachmentID,
			"size", size,
			"transfer", TransferDirect)

		response := FormatAttachmentConfirmation(name, subject, messageID, attachmentID, size, TransferDirect)
		if line := AccountInfoLine(ctx); line != "" {
			response += "\n" + line
		}
		return mcp.NewToolResultText(response), nil
	}
}
