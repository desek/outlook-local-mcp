// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the handler for the teams list_chat_messages verb, which
// reads the messages in one conversation. It is the step between resolving a
// chat and reading a single message: the listing carries a body preview per
// message so the caller can decide which one is worth the full read, rather than
// escalating every message in the thread.
//
// @agents-index: Handler constructor for the teams.list_chat_messages verb,
// reading one chat's messages by chat identifier and projecting them across the
// three output tiers.
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

// Corrections appended to this verb's refusals. The identifier correction names
// both ways a chat identifier is obtained, since neither subsumes the other: a
// search finds the conversation a remembered phrase is in, and the chat listing
// enumerates the ones the user is a member of.
const (
	listChatMessagesAccountFix = "no account is selected: run the account domain's list operation and retry naming one with the account parameter"
	listChatMessagesChatIDFix  = "supply chat_id as the id field of a teams list_chats entry or the chatId of a teams search hit"
	listChatMessagesTimeoutFix = "retry, and if the deadline is hit again locate the message with the teams search operation instead of reading the whole thread"
	listChatMessagesGraphFix   = "check that chat_id names a chat this account is a member of and that consent was granted after OUTLOOK_MCP_TEAMS_ENABLED was set, so Chat.Read was requested, then retry; see docs/troubleshooting.md#authentication-failures"
)

// NewHandleListChatMessages creates the handler for the teams
// list_chat_messages verb. It reads GET /me/chats/{chat-id}/messages and returns
// the messages in the order Graph returned them.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method.
//
// The handler validates chat_id before issuing any request, so a malformed
// identifier costs no Graph call, and issues exactly one request on the success
// path without following the collection's next link.
func NewHandleListChatMessages(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)
		start := time.Now()

		client, err := GraphClient(ctx)
		if err != nil {
			logger.Error("no account selected", "fix", listChatMessagesAccountFix)
			return mcp.NewToolResultError(listChatMessagesAccountFix), nil
		}

		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		chatID := request.GetString("chat_id", "")
		if err := validate.ValidateResourceID(chatID, "chat_id"); err != nil {
			logger.Error("chat_id rejected", "error", err.Error(), "fix", listChatMessagesChatIDFix)
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), listChatMessagesChatIDFix)), nil
		}

		logger.Debug("tool called", "chat_id", chatID)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		logger.Debug("graph API request", "endpoint", "GET /me/chats/{chat-id}/messages")

		var resp models.ChatMessageCollectionResponseable
		graphErr := graph.RetryGraphCall(ctx, retryCfg, func() error {
			var callErr error
			resp, callErr = client.Me().Chats().ByChatId(chatID).Messages().Get(timeoutCtx, nil)
			return callErr
		})
		if graphErr != nil {
			if graph.IsTimeoutError(graphErr) {
				logger.ErrorContext(ctx, "request timed out",
					"endpoint", "GET /me/chats/{chat-id}/messages",
					"timeout_seconds", int(timeout.Seconds()),
					"fix", listChatMessagesTimeoutFix,
					"error", graphErr.Error())
				return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
					graph.TimeoutErrorMessage(int(timeout.Seconds())), listChatMessagesTimeoutFix)), nil
			}
			logger.Error("graph API call failed",
				"endpoint", "GET /me/chats/{chat-id}/messages",
				"error", graph.FormatGraphError(graphErr),
				"fix", listChatMessagesGraphFix,
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
				graph.RedactGraphError(graphErr), listChatMessagesGraphFix)), nil
		}

		messages := serializeChatMessageCollection(resp, outputMode)

		logger.Debug("graph API response",
			"endpoint", "GET /me/chats/{chat-id}/messages",
			"count", len(messages))

		if outputMode == "text" {
			logger.Info("tool completed", "duration", time.Since(start), "count", len(messages))
			return mcp.NewToolResultText(FormatTeamsMessagesText(messages)), nil
		}

		jsonBytes, err := json.Marshal(messages)
		if err != nil {
			logger.Error("json serialization failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize messages: %s", err.Error())), nil
		}

		logger.Info("tool completed", "duration", time.Since(start), "count", len(messages))
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}

// serializeChatMessageCollection projects a chat's messages through the tier's
// serializer, preserving the order Graph returned them in.
//
// Parameters:
//   - resp: the collection response, which may be nil when Graph returned no body.
//   - outputMode: the resolved output tier.
//
// Returns one serialized map per message, in the order received.
func serializeChatMessageCollection(resp models.ChatMessageCollectionResponseable, outputMode string) []map[string]any {
	if resp == nil {
		return []map[string]any{}
	}
	messages := make([]map[string]any, 0, len(resp.GetValue()))
	for _, msg := range resp.GetValue() {
		if outputMode == "raw" {
			messages = append(messages, SerializeChatMessage(msg))
		} else {
			messages = append(messages, SerializeSummaryChatMessage(msg))
		}
	}
	return messages
}
