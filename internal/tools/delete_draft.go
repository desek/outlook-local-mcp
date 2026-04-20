// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the mail_delete_draft MCP tool, which permanently
// deletes a draft message via DELETE /me/messages/{id} on the Microsoft
// Graph API. The handler first verifies isDraft=true and refuses to delete
// sent or received messages.
package tools

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
)

// NewDeleteDraftTool creates the MCP tool definition for mail_delete_draft.
// It requires a message_id identifying the draft to remove. Destructive:
// deletion is permanent.
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewDeleteDraftTool() mcp.Tool {
	return mcp.NewTool("mail_delete_draft",
		mcp.WithTitleAnnotation("Delete Email Draft"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithDescription(
			"Permanently delete a draft message from the Drafts folder. This only "+
				"operates on messages with isDraft=true; sent or received messages "+
				"are rejected.",
		),
		mcp.WithString("message_id", mcp.Required(),
			mcp.Description("The unique identifier of the draft message to delete."),
		),
		mcp.WithString("account",
			mcp.Description(AccountParamDescription),
		),
	)
}

// NewHandleDeleteDraft creates the MCP tool handler for mail_delete_draft. It
// verifies the target is a draft, then calls DELETE /me/messages/{id} on the
// Graph API.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for a single Graph API call.
//
// Returns a handler function compatible with the MCP server AddTool signature.
//
// Side effects: calls GET then DELETE /me/messages/{id} on the Microsoft
// Graph API.
func NewHandleDeleteDraft(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := slog.With("tool", "mail_delete_draft")

		client, err := GraphClient(ctx)
		if err != nil {
			return mcp.NewToolResultError("no account selected"), nil
		}

		messageID, err := request.RequireString("message_id")
		if err != nil {
			return mcp.NewToolResultError("missing required parameter: message_id"), nil
		}
		if err := validate.ValidateResourceID(messageID, "message_id"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if errResult := verifyIsDraft(ctx, client, retryCfg, timeout, messageID, logger); errResult != nil {
			return errResult, nil
		}

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			return client.Me().Messages().ByMessageId(messageID).Delete(timeoutCtx, nil)
		})
		if err != nil {
			if graph.IsTimeoutError(err) {
				logger.ErrorContext(ctx, "request timed out",
					"timeout_seconds", int(timeout.Seconds()),
					"error", err.Error())
				return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds()))), nil
			}
			logger.ErrorContext(ctx, "delete draft failed", "error", graph.FormatGraphError(err))
			return mcp.NewToolResultError(graph.RedactGraphError(err)), nil
		}

		logger.InfoContext(ctx, "draft deleted", "draft_id", messageID)

		response := fmt.Sprintf("Draft deleted: %s\nThe draft has been permanently removed from the Drafts folder.", messageID)
		if line := AccountInfoLine(ctx); line != "" {
			response += "\n" + line
		}
		return mcp.NewToolResultText(response), nil
	}
}
