// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the handler for the teams list_chats verb, which reads the
// conversations the signed-in user is a member of. It is the only way this
// domain resolves a chat identifier without a search: reading one chat by
// identifier is deliberately not exposed, so the listing carries enough of each
// chat to choose from, including the preview of its last message, which is often
// all that distinguishes one untitled one-to-one chat from another.
//
// @agents-index: Handler constructor for the teams.list_chats verb, reading the
// signed-in user's chats and projecting them across the three output tiers.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/logging"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// Corrections appended to this verb's refusals, authored at the verb because the
// shared helpers state a diagnosis without naming what to do next, and emitted
// to both the tool result and the log record.
const (
	listChatsAccountFix = "no account is selected: run the account domain's list operation and retry naming one with the account parameter"
	listChatsTimeoutFix = "retry, and if the deadline is hit again find the conversation with the teams search operation instead of listing every chat"
	listChatsGraphFix   = "check that consent was granted after OUTLOOK_MCP_TEAMS_ENABLED was set, so Chat.Read was requested, then retry; see docs/troubleshooting.md#authentication-failures"
)

// NewHandleListChats creates the handler for the teams list_chats verb. It reads
// GET /me/chats and returns the chats in the order Graph returned them.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method.
//
// The handler issues exactly one Graph request and reads only the page Graph
// returns for it; it does not follow the collection's next link, so the cost of
// the verb is bounded by one round trip.
func NewHandleListChats(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)
		start := time.Now()

		client, err := GraphClient(ctx)
		if err != nil {
			logger.Error("no account selected", "fix", listChatsAccountFix)
			return mcp.NewToolResultError(listChatsAccountFix), nil
		}

		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		logger.Debug("tool called")

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		logger.Debug("graph API request", "endpoint", "GET /me/chats")

		var resp models.ChatCollectionResponseable
		graphErr := graph.RetryGraphCall(ctx, retryCfg, func() error {
			var callErr error
			resp, callErr = client.Me().Chats().Get(timeoutCtx, nil)
			return callErr
		})
		if graphErr != nil {
			if graph.IsTimeoutError(graphErr) {
				logger.ErrorContext(ctx, "request timed out",
					"endpoint", "GET /me/chats",
					"timeout_seconds", int(timeout.Seconds()),
					"fix", listChatsTimeoutFix,
					"error", graphErr.Error())
				return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
					graph.TimeoutErrorMessage(int(timeout.Seconds())), listChatsTimeoutFix)), nil
			}
			logger.Error("graph API call failed",
				"endpoint", "GET /me/chats",
				"error", graph.FormatGraphError(graphErr),
				"fix", listChatsGraphFix,
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
				graph.RedactGraphError(graphErr), listChatsGraphFix)), nil
		}

		chats := serializeChatCollection(resp, outputMode)

		logger.Debug("graph API response", "endpoint", "GET /me/chats", "count", len(chats))

		if outputMode == "text" {
			logger.Info("tool completed", "duration", time.Since(start), "count", len(chats))
			return mcp.NewToolResultText(FormatChatsText(chats)), nil
		}

		jsonBytes, err := json.Marshal(chats)
		if err != nil {
			logger.Error("json serialization failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize chats: %s", err.Error())), nil
		}

		logger.Info("tool completed", "duration", time.Since(start), "count", len(chats))
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}

// serializeChatCollection projects a chat collection through the tier's
// serializer, preserving Graph's own order, which puts the most recently active
// conversations first.
//
// Parameters:
//   - resp: the collection response, which may be nil when Graph returned no body.
//   - outputMode: the resolved output tier.
//
// Returns one serialized map per chat, in the order received.
func serializeChatCollection(resp models.ChatCollectionResponseable, outputMode string) []map[string]any {
	if resp == nil {
		return []map[string]any{}
	}
	chats := make([]map[string]any, 0, len(resp.GetValue()))
	for _, chat := range resp.GetValue() {
		if outputMode == "raw" {
			chats = append(chats, SerializeChat(chat))
		} else {
			chats = append(chats, SerializeSummaryChat(chat))
		}
	}
	return chats
}
