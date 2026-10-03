// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the mail domain's set_categories handler, which replaces
// the full category set of an existing message via PATCH /me/messages/{id}.
// The categories array is replaced rather than appended to, and an empty or
// whitespace-only value clears every category. It operates on received
// messages, so it applies no isDraft guard and performs no read-modify-write.
//
// @agents-index: Handler constructor for the mail.set_categories verb,
// replacing a message's category set and confirming the resulting list.
package tools

import (
	"context"
	"strings"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/logging"
	"github.com/desek/outlook-local-mcp/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// NewHandleSetCategories creates the MCP tool handler for the
// mail.set_categories verb. It requires message_id and categories, validates
// the identifier and the category string length before any Graph request, and
// PATCHes the message with the full replacement set.
//
// The categories value is a comma-separated list; entries are trimmed and empty
// entries are dropped. An empty or whitespace-only value is an instruction to
// clear every category, and the confirmation then states that the message
// carries none rather than printing an empty list.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for a single Graph API call.
//
// Returns a handler function compatible with the MCP server AddTool signature.
//
// Side effects: calls PATCH /me/messages/{id} on the Microsoft Graph API.
func NewHandleSetCategories(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

		catStr, err := request.RequireString("categories")
		if err != nil {
			return mcp.NewToolResultError(
				"missing required parameter: categories (a comma-separated list; " +
					"supply an empty string to clear every category)"), nil
		}
		if err := validate.ValidateStringLength(catStr, "categories", validate.MaxCategoriesLen); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// The full set is replaced, never appended to; an empty slice clears it.
		cats := splitCategories(catStr)
		if cats == nil {
			cats = []string{}
		}
		patch := models.NewMessage()
		patch.SetCategories(cats)

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
				"\nSupply a message_id from mail.list_messages and categories as a comma-separated list."
			logger.ErrorContext(ctx, "set categories failed",
				"error", graph.FormatGraphError(err),
				"fix", "supply a message_id from mail.list_messages and categories as a comma-separated list")
			return mcp.NewToolResultError(msg), nil
		}

		resultID := graph.SafeStr(updated.GetId())
		logger.InfoContext(ctx, "message categories updated", "message_id", resultID)

		value, consequence := responseCategories(updated)
		response := FormatMailWriteConfirmation(
			"categories updated",
			graph.SafeStr(updated.GetSubject()),
			resultID,
			"Categories",
			value,
			consequence,
		)
		if line := AccountInfoLine(ctx); line != "" {
			response += "\n" + line
		}
		return mcp.NewToolResultText(response), nil
	}
}

// responseCategories renders the resulting category list from a Graph response
// message, reporting what the service stored rather than what was requested.
//
// Parameters:
//   - msg: the message Graph returned from the write.
//
// Returns the comma-separated list and an empty consequence line, or the value
// "none" together with the sentence stating that the message now carries no
// categories, so an empty result is spelled out rather than printed as an
// empty list.
//
// Side effects: none.
func responseCategories(msg models.Messageable) (value, consequence string) {
	if msg == nil || len(msg.GetCategories()) == 0 {
		return "none", "The message now carries no categories."
	}
	return strings.Join(msg.GetCategories(), ", "), ""
}
