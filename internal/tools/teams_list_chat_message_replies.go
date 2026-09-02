// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the handler for the teams list_chat_message_replies verb,
// which reads the replies hanging under one chat message. A thread listing
// returns top-level messages, so the answers to a question are only reachable
// through this verb; each reply carries the replyToId naming the parent, so a
// caller merging several threads can still tell which is which.
//
// @agents-index: Handler constructor for the teams.list_chat_message_replies
// verb, reading one chat message's replies and projecting them across the three
// output tiers.
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

// Corrections appended to this verb's refusals, each naming the listing that
// supplies the identifier it is about.
const (
	listChatRepliesAccountFix   = "no account is selected: run the account domain's list operation and retry naming one with the account parameter"
	listChatRepliesChatIDFix    = "supply chat_id as the id field of a teams list_chats entry or the chatId of a teams search hit"
	listChatRepliesMessageIDFix = "supply message_id as the id field of the parent message, from a teams list_chat_messages entry or a teams search hit"
	listChatRepliesTimeoutFix   = "retry, and if the deadline is hit again confirm the identifiers with the teams list_chat_messages operation"
	listChatRepliesGraphFix     = "check that chat_id and message_id name a message in a chat this account is a member of and that consent was granted after OUTLOOK_MCP_TEAMS_ENABLED was set, so Chat.Read was requested, then retry; see docs/troubleshooting.md#authentication-failures"
)

// NewHandleListChatMessageReplies creates the handler for the teams
// list_chat_message_replies verb. It reads
// GET /me/chats/{chat-id}/messages/{message-id}/replies and returns the replies
// in the order Graph returned them.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method.
//
// Both identifiers are validated before any request is issued, so a malformed
// one costs no Graph call, and exactly one request is issued on the success
// path without following the collection's next link.
func NewHandleListChatMessageReplies(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)
		start := time.Now()

		client, err := GraphClient(ctx)
		if err != nil {
			logger.Error("no account selected", "fix", listChatRepliesAccountFix)
			return mcp.NewToolResultError(listChatRepliesAccountFix), nil
		}

		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		chatID := request.GetString("chat_id", "")
		if err := validate.ValidateResourceID(chatID, "chat_id"); err != nil {
			logger.Error("chat_id rejected", "error", err.Error(), "fix", listChatRepliesChatIDFix)
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), listChatRepliesChatIDFix)), nil
		}

		messageID := request.GetString("message_id", "")
		if err := validate.ValidateResourceID(messageID, "message_id"); err != nil {
			logger.Error("message_id rejected", "error", err.Error(), "fix", listChatRepliesMessageIDFix)
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), listChatRepliesMessageIDFix)), nil
		}

		logger.Debug("tool called", "chat_id", chatID, "message_id", messageID)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		logger.Debug("graph API request", "endpoint", "GET /me/chats/{chat-id}/messages/{message-id}/replies")

		var resp models.ChatMessageCollectionResponseable
		graphErr := graph.RetryGraphCall(ctx, retryCfg, func() error {
			var callErr error
			resp, callErr = client.Me().Chats().ByChatId(chatID).Messages().ByChatMessageId(messageID).Replies().Get(timeoutCtx, nil)
			return callErr
		})
		if graphErr != nil {
			if graph.IsTimeoutError(graphErr) {
				logger.ErrorContext(ctx, "request timed out",
					"endpoint", "GET /me/chats/{chat-id}/messages/{message-id}/replies",
					"timeout_seconds", int(timeout.Seconds()),
					"fix", listChatRepliesTimeoutFix,
					"error", graphErr.Error())
				return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
					graph.TimeoutErrorMessage(int(timeout.Seconds())), listChatRepliesTimeoutFix)), nil
			}
			logger.Error("graph API call failed",
				"endpoint", "GET /me/chats/{chat-id}/messages/{message-id}/replies",
				"error", graph.FormatGraphError(graphErr),
				"fix", listChatRepliesGraphFix,
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
				graph.RedactGraphError(graphErr), listChatRepliesGraphFix)), nil
		}

		replies := serializeMessageReplyCollection(resp, outputMode)

		logger.Debug("graph API response",
			"endpoint", "GET /me/chats/{chat-id}/messages/{message-id}/replies",
			"count", len(replies))

		if outputMode == "text" {
			logger.Info("tool completed", "duration", time.Since(start), "count", len(replies))
			return mcp.NewToolResultText(FormatTeamsMessagesText(replies)), nil
		}

		jsonBytes, err := json.Marshal(replies)
		if err != nil {
			logger.Error("json serialization failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize replies: %s", err.Error())), nil
		}

		logger.Info("tool completed", "duration", time.Since(start), "count", len(replies))
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}

// serializeMessageReplyCollection projects a reply collection through the
// tier's serializer, preserving the order Graph returned them in. The reply
// serializers are used rather than the chat-message ones because a reply's
// distinguishing field is the parent it hangs under, and this is the one shape
// that carries it in the summary tier.
//
// Parameters:
//   - resp: the collection response, which may be nil when Graph returned no body.
//   - outputMode: the resolved output tier.
//
// Returns one serialized map per reply, in the order received.
func serializeMessageReplyCollection(resp models.ChatMessageCollectionResponseable, outputMode string) []map[string]any {
	if resp == nil {
		return []map[string]any{}
	}
	replies := make([]map[string]any, 0, len(resp.GetValue()))
	for _, msg := range resp.GetValue() {
		if outputMode == "raw" {
			replies = append(replies, SerializeMessageReply(msg))
		} else {
			replies = append(replies, SerializeSummaryMessageReply(msg))
		}
	}
	return replies
}
