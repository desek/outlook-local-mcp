// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the handler for the teams get_chat_message verb, which
// reads one message in a conversation. It is the escalation target of a search
// hit or a thread listing: both return a preview, and this verb returns the
// whole message behind it, but only under output=raw. A Teams body is
// arbitrarily long and commonly HTML, so the default tier still returns the
// preview and the verb's description names the escalation.
//
// @agents-index: Handler constructor for the teams.get_chat_message verb,
// reading one chat message by chat and message identifier with the full body
// gated behind the raw output tier.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/logging"
	"github.com/desek/outlook-local-mcp/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// Corrections appended to this verb's refusals. Both identifiers are named
// separately, because a caller that has one and not the other needs to know
// which of the two is missing to know which listing to run.
const (
	getChatMessageAccountFix   = "no account is selected: run the account domain's list operation and retry naming one with the account parameter"
	getChatMessageChatIDFix    = "supply chat_id as the id field of a teams list_chats entry or the chatId of a teams search hit"
	getChatMessageMessageIDFix = "supply message_id as the id field of a teams list_chat_messages entry or a teams search hit"
	getChatMessageTimeoutFix   = "retry, and if the deadline is hit again confirm the identifiers with the teams list_chat_messages operation"
	getChatMessageGraphFix     = "check that chat_id and message_id name a message in a chat this account is a member of and that consent was granted after OUTLOOK_MCP_TEAMS_ENABLED was set, so Chat.Read was requested, then retry; see docs/troubleshooting.md#authentication-failures"
)

// NewHandleGetChatMessage creates the handler for the teams get_chat_message
// verb. It reads GET /me/chats/{chat-id}/messages/{message-id} and returns the
// message with a body preview by default and the full body under output=raw.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method.
//
// Both identifiers are validated before any request is issued, so a malformed
// one costs no Graph call. The escalation the verb offers is over the size of
// the returned text, not over the number of requests: one request serves every
// tier, because Graph returns the whole body whether or not the caller asked
// for it.
func NewHandleGetChatMessage(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)
		start := time.Now()

		client, err := GraphClient(ctx)
		if err != nil {
			logger.Error("no account selected", "fix", getChatMessageAccountFix)
			return mcp.NewToolResultError(getChatMessageAccountFix), nil
		}

		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		chatID := request.GetString("chat_id", "")
		if err := validate.ValidateResourceID(chatID, "chat_id"); err != nil {
			logger.Error("chat_id rejected", "error", err.Error(), "fix", getChatMessageChatIDFix)
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), getChatMessageChatIDFix)), nil
		}

		messageID := request.GetString("message_id", "")
		if err := validate.ValidateResourceID(messageID, "message_id"); err != nil {
			logger.Error("message_id rejected", "error", err.Error(), "fix", getChatMessageMessageIDFix)
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), getChatMessageMessageIDFix)), nil
		}

		logger.Debug("tool called", "chat_id", chatID, "message_id", messageID)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		logger.Debug("graph API request", "endpoint", "GET /me/chats/{chat-id}/messages/{message-id}")

		var msg models.ChatMessageable
		graphErr := graph.RetryGraphCall(ctx, retryCfg, func() error {
			var callErr error
			msg, callErr = client.Me().Chats().ByChatId(chatID).Messages().ByChatMessageId(messageID).Get(timeoutCtx, nil)
			return callErr
		})
		if graphErr != nil {
			if graph.IsTimeoutError(graphErr) {
				logger.ErrorContext(ctx, "request timed out",
					"endpoint", "GET /me/chats/{chat-id}/messages/{message-id}",
					"timeout_seconds", int(timeout.Seconds()),
					"fix", getChatMessageTimeoutFix,
					"error", graphErr.Error())
				return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
					graph.TimeoutErrorMessage(int(timeout.Seconds())), getChatMessageTimeoutFix)), nil
			}
			logger.Error("graph API call failed",
				"endpoint", "GET /me/chats/{chat-id}/messages/{message-id}",
				"error", graph.FormatGraphError(graphErr),
				"fix", getChatMessageGraphFix,
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
				graph.RedactGraphError(graphErr), getChatMessageGraphFix)), nil
		}

		logger.Debug("graph API response",
			"endpoint", "GET /me/chats/{chat-id}/messages/{message-id}",
			"status", "ok")

		result := SerializeSummaryChatMessage(msg)
		if outputMode == "raw" {
			result = SerializeChatMessage(msg)
		}

		if outputMode == "text" {
			logger.Info("tool completed", "duration", time.Since(start))
			return mcp.NewToolResultText(FormatTeamsMessageDetailText(result)), nil
		}

		jsonBytes, err := json.Marshal(result)
		if err != nil {
			logger.Error("json serialization failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize message: %s", err.Error())), nil
		}

		logger.Info("tool completed", "duration", time.Since(start))
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}
