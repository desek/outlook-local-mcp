// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the mail domain's set_flag handler, which writes the
// follow-up flag of an existing message via PATCH /me/messages/{id}. It operates
// on received messages, so it applies no isDraft guard and performs no
// read-modify-write round trip: the new status is supplied in full by the caller.
//
// @agents-index: Handler constructor for the mail.set_flag verb, PATCHing a
// message's follow-up flag status and confirming the value Graph returned.
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

// NewHandleSetFlag creates the MCP tool handler for the mail.set_flag verb. It
// requires message_id and flag_status, validates both before any Graph request
// is issued, and PATCHes the message with a follow-up flag carrying the parsed
// status.
//
// The confirmation reports the flag status read from the Graph response rather
// than the requested one, so a value the service coerced is visible to the
// caller.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for a single Graph API call.
//
// Returns a handler function compatible with the MCP server AddTool signature.
//
// Side effects: calls PATCH /me/messages/{id} on the Microsoft Graph API.
func NewHandleSetFlag(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

		statusStr, err := request.RequireString("flag_status")
		if err != nil {
			return mcp.NewToolResultError(
				"missing required parameter: flag_status (accepted: notFlagged, flagged, complete)"), nil
		}
		// ValidateFlagStatus must precede ParseFlagStatus: the parser defaults an
		// unrecognised value to notFlagged, which clears a flag rather than
		// leaving it unchanged.
		if err := validate.ValidateFlagStatus(statusStr); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		status := graph.ParseFlagStatus(statusStr)

		flag := models.NewFollowupFlag()
		flag.SetFlagStatus(&status)
		patch := models.NewMessage()
		patch.SetFlag(flag)

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
				"\nSupply a message_id from mail.list_messages and a flag_status of notFlagged, flagged, or complete."
			logger.ErrorContext(ctx, "set flag failed",
				"error", graph.FormatGraphError(err),
				"fix", "supply a message_id from mail.list_messages and a flag_status of notFlagged, flagged, or complete")
			return mcp.NewToolResultError(msg), nil
		}

		resultID := graph.SafeStr(updated.GetId())
		logger.InfoContext(ctx, "message flag updated", "message_id", resultID)

		response := FormatMailWriteConfirmation(
			"flag updated",
			graph.SafeStr(updated.GetSubject()),
			resultID,
			"Flag status",
			responseFlagStatus(updated),
			"",
		)
		if line := AccountInfoLine(ctx); line != "" {
			response += "\n" + line
		}
		return mcp.NewToolResultText(response), nil
	}
}

// responseFlagStatus reads the follow-up flag status from a Graph response
// message. It reports what the service returned rather than what was requested,
// so a coerced or omitted value is visible to the caller.
//
// Parameters:
//   - msg: the message Graph returned from the write.
//
// Returns the status name, or "unknown (not reported by Graph)" when the
// response carries no flag status.
//
// Side effects: none.
func responseFlagStatus(msg models.Messageable) string {
	if msg == nil {
		return "unknown (not reported by Graph)"
	}
	flag := msg.GetFlag()
	if flag == nil || flag.GetFlagStatus() == nil {
		return "unknown (not reported by Graph)"
	}
	return flag.GetFlagStatus().String()
}
