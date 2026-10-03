// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the mail domain's move_message handler, which moves an
// existing message to another folder via POST /me/messages/{id}/move. The move
// mints a new identifier: the message Graph returns is the copy in the
// destination folder, and the identifier the caller supplied stops resolving,
// which is why the confirmation names all three identifiers rather than only
// the subject. It operates on received messages, so it applies no isDraft guard
// and performs no read-modify-write round trip.
//
// @agents-index: Handler constructor for the mail.move_message verb, POSTing a
// folder move and confirming the new identifier the move minted.
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
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

// moveFixInstruction is the correction appended to a failed move and logged
// alongside it, so a headless caller that cannot read an interactive surface
// still receives it. A move fails most often because the destination does not
// resolve, and a destination identifier is obtained from mail.list_folders.
const moveFixInstruction = "supply a message_id from mail.list_messages and a destination_folder_id from mail.list_folders"

// NewHandleMoveMessage creates the MCP tool handler for the mail.move_message
// verb. It requires message_id and destination_folder_id, validates both before
// any Graph request is issued, and POSTs the move with the destination supplied
// in the request body.
//
// The confirmation is built from the message Graph returns, which is the
// message in its new folder, so the new identifier is reported rather than the
// requested one. It also states explicitly that the original identifier no
// longer resolves, because that consequence is invisible to the caller and
// silently invalidates any handle it was holding.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for a single Graph API call.
//
// Returns a handler function compatible with the MCP server AddTool signature.
//
// Side effects: calls POST /me/messages/{id}/move on the Microsoft Graph API,
// which removes the message from its source folder.
func NewHandleMoveMessage(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

		destID, err := request.RequireString("destination_folder_id")
		if err != nil {
			return mcp.NewToolResultError(
				"missing required parameter: destination_folder_id (obtain one from mail.list_folders)"), nil
		}
		if err := validate.ValidateResourceID(destID, "destination_folder_id"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		body := users.NewItemMessagesItemMovePostRequestBody()
		body.SetDestinationId(&destID)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		var moved models.Messageable
		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			var gErr error
			moved, gErr = client.Me().Messages().ByMessageId(messageID).Move().Post(timeoutCtx, body, nil)
			return gErr
		})
		if err != nil {
			if graph.IsTimeoutError(err) {
				logger.ErrorContext(ctx, "request timed out",
					"timeout_seconds", int(timeout.Seconds()),
					"error", err.Error())
				return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds()))), nil
			}
			msg := graph.RedactGraphError(err) + "\nSupply a message_id from mail.list_messages and a " +
				"destination_folder_id from mail.list_folders."
			logger.ErrorContext(ctx, "move message failed",
				"error", graph.FormatGraphError(err),
				"fix", moveFixInstruction)
			return mcp.NewToolResultError(msg), nil
		}

		newID := graph.SafeStr(moved.GetId())
		logger.InfoContext(ctx, "message moved",
			"original_message_id", messageID,
			"message_id", newID,
			"destination_folder_id", destID)

		response := FormatMailWriteConfirmation(
			"moved",
			graph.SafeStr(moved.GetSubject()),
			newID,
			"Destination folder",
			destID,
			fmt.Sprintf("The move minted a new identifier: the original message_id %s no longer "+
				"resolves, so use the ID above for any further operation on this message.", messageID),
		)
		if line := AccountInfoLine(ctx); line != "" {
			response += "\n" + line
		}
		return mcp.NewToolResultText(response), nil
	}
}
