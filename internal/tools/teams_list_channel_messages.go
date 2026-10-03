// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the handler for the teams list_channel_messages verb, which
// reads the top-level posts in one channel. Channel conversations are threaded:
// this listing returns the thread openers only, and the answers under each are
// reached through the replies verb, so a caller scanning a channel is not handed
// every reply in the team at once.
//
// @agents-index: Handler constructor for the teams.list_channel_messages verb,
// reading one channel's top-level messages by team and channel identifier and
// projecting them across the three output tiers.
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
	msteams "github.com/microsoftgraph/msgraph-sdk-go/teams"
)

// Corrections appended to this verb's refusals. The two identifiers are named
// separately because a channel is addressed by both, and a caller holding one
// cannot infer the other; the search hit is named as the source because this
// domain exposes no team or channel enumeration.
const (
	listChannelMessagesAccountFix   = "no account is selected: run the account domain's list operation and retry naming one with the account parameter"
	listChannelMessagesTeamIDFix    = "supply team_id as the teamId field of a teams search hit for a channel message"
	listChannelMessagesChannelIDFix = "supply channel_id as the channelId field of a teams search hit for a channel message"
	listChannelMessagesTimeoutFix   = "retry, and if the deadline is hit again locate the post with the teams search operation instead of reading the whole channel"
	listChannelMessagesGraphFix     = "check that team_id and channel_id name a channel this account can read and that consent was granted after OUTLOOK_MCP_TEAMS_ENABLED was set, so ChannelMessage.Read.All was requested, then retry; see docs/troubleshooting.md#authentication-failures"
)

// The endpoint this verb reads, stated once so the log records and the timeout
// and failure paths cannot drift from each other.
const listChannelMessagesEndpoint = "GET /teams/{team-id}/channels/{channel-id}/messages"

// NewHandleListChannelMessages creates the handler for the teams
// list_channel_messages verb. It reads
// GET /teams/{team-id}/channels/{channel-id}/messages and returns the top-level
// posts in the order Graph returned them, which is by the last modification of
// each whole reply chain. No $orderby is sent because the collection documents
// none.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method.
//
// Both identifiers are validated before any request is issued, so a malformed
// one costs no Graph call, and exactly one request of at most max_results posts
// (1 to 50; the service default is 20) is issued on the success path without
// following the collection's next link. When Graph signals more pages the
// result says so, because older threads are then reachable only through search.
func NewHandleListChannelMessages(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)
		start := time.Now()

		client, err := GraphClient(ctx)
		if err != nil {
			logger.Error("no account selected", "fix", listChannelMessagesAccountFix)
			return mcp.NewToolResultError(listChannelMessagesAccountFix), nil
		}

		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		teamID := request.GetString("team_id", "")
		if err := validate.ValidateResourceID(teamID, "team_id"); err != nil {
			logger.Error("team_id rejected", "error", err.Error(), "fix", listChannelMessagesTeamIDFix)
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), listChannelMessagesTeamIDFix)), nil
		}

		channelID := request.GetString("channel_id", "")
		if err := validate.ValidateResourceID(channelID, "channel_id"); err != nil {
			logger.Error("channel_id rejected", "error", err.Error(), "fix", listChannelMessagesChannelIDFix)
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), listChannelMessagesChannelIDFix)), nil
		}

		logger.Debug("tool called", "team_id", teamID, "channel_id", channelID)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		logger.Debug("graph API request", "endpoint", listChannelMessagesEndpoint)

		var resp models.ChatMessageCollectionResponseable
		graphErr := graph.RetryGraphCall(ctx, retryCfg, func() error {
			var callErr error
			resp, callErr = client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Messages().Get(timeoutCtx, listChannelMessagesRequestConfig(teamsPageSize(request)))
			return callErr
		})
		if graphErr != nil {
			if graph.IsTimeoutError(graphErr) {
				logger.ErrorContext(ctx, "request timed out",
					"endpoint", listChannelMessagesEndpoint,
					"timeout_seconds", int(timeout.Seconds()),
					"fix", listChannelMessagesTimeoutFix,
					"error", graphErr.Error())
				return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
					graph.TimeoutErrorMessage(int(timeout.Seconds())), listChannelMessagesTimeoutFix)), nil
			}
			logger.Error("graph API call failed",
				"endpoint", listChannelMessagesEndpoint,
				"error", graph.FormatGraphError(graphErr),
				"fix", listChannelMessagesGraphFix,
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
				graph.RedactGraphError(graphErr), listChannelMessagesGraphFix)), nil
		}

		messages := serializeChannelMessageCollection(resp, outputMode)

		logger.Debug("graph API response",
			"endpoint", listChannelMessagesEndpoint,
			"count", len(messages))

		more := resp != nil && resp.GetOdataNextLink() != nil

		if outputMode == "text" {
			logger.Info("tool completed", "duration", time.Since(start), "count", len(messages), "truncated", more)
			return mcp.NewToolResultText(teamsPagedText(FormatTeamsMessagesText(messages), "channel posts", len(messages), more)), nil
		}

		jsonBytes, err := json.Marshal(messages)
		if err != nil {
			logger.Error("json serialization failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize messages: %s", err.Error())), nil
		}

		logger.Info("tool completed", "duration", time.Since(start), "count", len(messages), "truncated", more)
		return teamsPagedJSON(string(jsonBytes), "channel posts", len(messages), more), nil
	}
}

// listChannelMessagesRequestConfig builds the query for one page of channel
// posts; only the page size is set, since the collection supports no ordering.
func listChannelMessagesRequestConfig(top int32) *msteams.ItemChannelsItemMessagesRequestBuilderGetRequestConfiguration {
	return &msteams.ItemChannelsItemMessagesRequestBuilderGetRequestConfiguration{
		QueryParameters: &msteams.ItemChannelsItemMessagesRequestBuilderGetQueryParameters{Top: &top},
	}
}

// serializeChannelMessageCollection projects a channel message collection
// through the tier's serializer, preserving the order Graph returned them in.
//
// Parameters:
//   - resp: the collection response, which may be nil when Graph returned no body.
//   - outputMode: the resolved output tier.
//
// Returns one serialized map per message, in the order received.
func serializeChannelMessageCollection(resp models.ChatMessageCollectionResponseable, outputMode string) []map[string]any {
	if resp == nil {
		return []map[string]any{}
	}
	messages := make([]map[string]any, 0, len(resp.GetValue()))
	for _, msg := range resp.GetValue() {
		if outputMode == "raw" {
			messages = append(messages, SerializeChannelMessage(msg))
		} else {
			messages = append(messages, SerializeSummaryChannelMessage(msg))
		}
	}
	return messages
}
