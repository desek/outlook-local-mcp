// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the mail_get_attachment MCP tool, which downloads an email
// attachment's metadata and base64-encoded content via the Microsoft Graph API.
// A configurable size limit prevents unbounded memory allocation.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// NewGetAttachmentTool creates the MCP tool definition for mail_get_attachment.
// The tool downloads a single attachment's metadata and base64 content.
//
// Parameters:
//   - message_id: required parent message ID.
//   - attachment_id: required attachment ID.
//   - account: optional account label for multi-account selection.
//   - output: optional output mode (text/summary/raw).
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewGetAttachmentTool() mcp.Tool {
	return mcp.NewTool("mail_get_attachment",
		mcp.WithDescription("Download an email attachment. Returns metadata (name, content type, size) and, for file attachments, base64-encoded content. A configurable maximum size limit (default 10 MB) protects server memory; larger attachments return an error."),
		mcp.WithTitleAnnotation("Get Email Attachment"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithString("message_id",
			mcp.Required(),
			mcp.Description("The unique identifier of the parent message."),
		),
		mcp.WithString("attachment_id",
			mcp.Required(),
			mcp.Description("The unique identifier of the attachment."),
		),
		mcp.WithString("account",
			mcp.Description(AccountParamDescription),
		),
		mcp.WithString("output",
			mcp.Description("Output mode: 'text' (default) returns metadata summary, 'summary' returns compact JSON with base64 content, 'raw' returns full Graph API fields including base64 content."),
			mcp.Enum("text", "summary", "raw"),
		),
	)
}

// NewHandleGetAttachment creates a tool handler that retrieves an attachment's
// metadata and base64-encoded content by calling
// GET /me/messages/{message_id}/attachments/{attachment_id} via the Graph SDK.
// The maxSize parameter enforces an upper bound on the attachment's reported
// size. Attachments exceeding the limit cause the handler to return an error
// without returning content.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//   - maxSize: the maximum allowed attachment size in bytes. Values <=0 are
//     treated as unlimited.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method.
func NewHandleGetAttachment(retryCfg graph.RetryConfig, timeout time.Duration, maxSize int64) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := slog.With("tool", "mail_get_attachment")
		start := time.Now()

		client, err := GraphClient(ctx)
		if err != nil {
			return mcp.NewToolResultError("no account selected"), nil
		}

		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		messageID, err := request.RequireString("message_id")
		if err != nil || messageID == "" {
			return mcp.NewToolResultError("missing required parameter: message_id"), nil
		}
		if err := validate.ValidateResourceID(messageID, "message_id"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		attachmentID, err := request.RequireString("attachment_id")
		if err != nil || attachmentID == "" {
			return mcp.NewToolResultError("missing required parameter: attachment_id"), nil
		}
		if err := validate.ValidateResourceID(attachmentID, "attachment_id"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		logger.Debug("tool called", "message_id", messageID, "attachment_id", attachmentID, "output", outputMode)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		var att models.Attachmentable
		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			var gErr error
			att, gErr = client.Me().Messages().ByMessageId(messageID).Attachments().ByAttachmentId(attachmentID).Get(timeoutCtx, nil)
			return gErr
		})
		if err != nil {
			if graph.IsTimeoutError(err) {
				logger.ErrorContext(ctx, "request timed out", "timeout_seconds", int(timeout.Seconds()))
				return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds()))), nil
			}
			logger.Error("graph API call failed", "error", graph.FormatGraphError(err), "duration", time.Since(start))
			return mcp.NewToolResultError(graph.RedactGraphError(err)), nil
		}

		// Enforce maximum attachment size from the reported metadata.
		if maxSize > 0 {
			if sz := att.GetSize(); sz != nil && int64(*sz) > maxSize {
				logger.Warn("attachment exceeds maximum size", "size", int64(*sz), "max", maxSize)
				return mcp.NewToolResultError(fmt.Sprintf("attachment size %d bytes exceeds maximum allowed %d bytes; raise OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES to download", int64(*sz), maxSize)), nil
			}
		}

		result := graph.SerializeAttachment(att)

		if outputMode == "text" {
			logger.Info("tool completed", "duration", time.Since(start), "attachment_id", attachmentID)
			return mcp.NewToolResultText(FormatAttachmentText(result)), nil
		}

		if outputMode == "summary" {
			result = graph.SerializeSummaryAttachment(att)
		}

		jsonBytes, jErr := json.Marshal(result)
		if jErr != nil {
			logger.Error("json serialization failed", "error", jErr.Error())
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize attachment: %s", jErr.Error())), nil
		}
		logger.Info("tool completed", "duration", time.Since(start), "attachment_id", attachmentID)
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}
