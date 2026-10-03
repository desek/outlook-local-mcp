// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the mail domain's mark_read handler, which writes the read
// state of an existing message via PATCH /me/messages/{id}. It writes in both
// directions: is_read is a required boolean, not a flag. It operates on
// received messages, so it applies no isDraft guard and performs no
// read-modify-write round trip.
//
// @agents-index: Handler constructor for the mail.mark_read verb, PATCHing a
// message's isRead state and confirming the state Graph returned.
package tools

import (
	"context"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/logging"
	"github.com/desek/outlook-local-mcp/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// NewHandleMarkRead creates the MCP tool handler for the mail.mark_read verb.
// It requires message_id and a boolean is_read, rejecting a missing or
// non-boolean value by name before any Graph request is issued, and PATCHes the
// message with the requested read state.
//
// The confirmation reports the state read from the Graph response rather than
// the requested one.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for a single Graph API call.
//
// Returns a handler function compatible with the MCP server AddTool signature.
//
// Side effects: calls PATCH /me/messages/{id} on the Microsoft Graph API.
func NewHandleMarkRead(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)

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

		isRead, err := request.RequireBool("is_read")
		if err != nil {
			return mcp.NewToolResultError(
				"missing or invalid parameter: is_read (supply the boolean true to mark read, false to mark unread)"), nil
		}

		read := isRead
		patch := models.NewMessage()
		patch.SetIsRead(&read)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		var updated models.Messageable
		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			var gErr error
			updated, gErr = client.Me().Messages().ByMessageId(messageID).Patch(timeoutCtx, patch, nil)
			return gErr
		})
		if err != nil {
			if graph.IsTimeoutError(err) {
				logger.ErrorContext(ctx, "request timed out",
					"timeout_seconds", int(timeout.Seconds()),
					"error", err.Error())
				return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds()))), nil
			}
			msg := graph.RedactGraphError(err) +
				"\nSupply a message_id from mail.list_messages and is_read as a boolean."
			logger.ErrorContext(ctx, "mark read failed",
				"error", graph.FormatGraphError(err),
				"fix", "supply a message_id from mail.list_messages and is_read as a boolean")
			return mcp.NewToolResultError(msg), nil
		}

		resultID := graph.SafeStr(updated.GetId())
		logger.InfoContext(ctx, "message read state updated", "message_id", resultID)

		response := FormatMailWriteConfirmation(
			"read state updated",
			graph.SafeStr(updated.GetSubject()),
			resultID,
			"Read state",
			responseReadState(updated),
			"",
		)
		if line := AccountInfoLine(ctx); line != "" {
			response += "\n" + line
		}
		return mcp.NewToolResultText(response), nil
	}
}

// responseReadState renders the read state from a Graph response message,
// reporting what the service stored rather than what was requested.
//
// Parameters:
//   - msg: the message Graph returned from the write.
//
// Returns "read" or "unread", or "unknown (not reported by Graph)" when the
// response carries no isRead value.
//
// Side effects: none.
func responseReadState(msg models.Messageable) string {
	if msg == nil || msg.GetIsRead() == nil {
		return "unknown (not reported by Graph)"
	}
	if *msg.GetIsRead() {
		return "read"
	}
	return "unread"
}
