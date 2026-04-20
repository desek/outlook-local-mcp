// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the mail_list_attachments MCP tool, which lists attachment
// metadata (id, name, contentType, size, isInline) for a given message via the
// Microsoft Graph API. It exists because mail_get_message does not expose
// attachment IDs, leaving callers unable to subsequently invoke
// mail_get_attachment. This tool fills that gap with a lightweight enumeration
// endpoint that does not download attachment content.
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
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

// listAttachmentsSelectFields enumerates the lightweight metadata returned by
// this tool. Content bytes are intentionally excluded — callers fetch those via
// mail_get_attachment using the returned id.
var listAttachmentsSelectFields = []string{"id", "name", "contentType", "size", "isInline"}

// NewListAttachmentsTool creates the MCP tool definition for
// mail_list_attachments. The tool enumerates attachment metadata for a single
// message without downloading any content bytes, enabling callers to discover
// attachment IDs needed by mail_get_attachment.
//
// Parameters:
//   - message_id: required parent message ID.
//   - account: optional account label for multi-account selection.
//   - output: optional output mode (text/summary/raw).
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewListAttachmentsTool() mcp.Tool {
	return mcp.NewTool("mail_list_attachments",
		mcp.WithDescription("List attachments for a message (metadata only — id, name, contentType, size, isInline). Use the returned id with mail_get_attachment to download content. This tool exists because mail_get_message does not include attachment IDs in its output."),
		mcp.WithTitleAnnotation("List Email Attachments"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithString("message_id",
			mcp.Required(),
			mcp.Description("The unique identifier of the parent message."),
		),
		mcp.WithString("account",
			mcp.Description(AccountParamDescription),
		),
		mcp.WithString("output",
			mcp.Description("Output mode: 'text' (default) returns a numbered list, 'summary' returns compact JSON, 'raw' returns full Graph API attachment fields."),
			mcp.Enum("text", "summary", "raw"),
		),
	)
}

// NewHandleListAttachments creates a tool handler that lists attachment
// metadata for a message by calling GET /me/messages/{id}/attachments with a
// $select restricted to lightweight fields (no contentBytes).
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method.
func NewHandleListAttachments(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := slog.With("tool", "mail_list_attachments")
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

		logger.Debug("tool called", "message_id", messageID, "output", outputMode)

		qp := &users.ItemMessagesItemAttachmentsRequestBuilderGetQueryParameters{
			Select: listAttachmentsSelectFields,
		}
		cfg := &users.ItemMessagesItemAttachmentsRequestBuilderGetRequestConfiguration{QueryParameters: qp}

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		var resp models.AttachmentCollectionResponseable
		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			var gErr error
			resp, gErr = client.Me().Messages().ByMessageId(messageID).Attachments().Get(timeoutCtx, cfg)
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

		raw := resp.GetValue()
		items := make([]map[string]any, 0, len(raw))
		for _, att := range raw {
			if outputMode == "raw" {
				items = append(items, graph.SerializeAttachment(att))
			} else {
				items = append(items, graph.SerializeSummaryAttachment(att))
			}
		}

		if outputMode == "text" {
			logger.Info("tool completed", "duration", time.Since(start), "count", len(items))
			return mcp.NewToolResultText(FormatAttachmentsText(items)), nil
		}

		jsonBytes, jErr := json.Marshal(items)
		if jErr != nil {
			logger.Error("json serialization failed", "error", jErr.Error())
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize attachments: %s", jErr.Error())), nil
		}
		logger.Info("tool completed", "duration", time.Since(start), "count", len(items))
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}
