// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the handler for the teams get_channel_message verb, which
// reads one post in a channel. It is the escalation target of a search hit or a
// channel listing: both return a preview, and this verb returns the whole post
// behind it, but only under output=raw, because a channel post is arbitrarily
// long and commonly HTML.
//
// @agents-index: Handler constructor for the teams.get_channel_message verb,
// reading one channel message by team, channel, and message identifier with the
// full body gated behind the raw output tier.
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

// Corrections appended to this verb's refusals. All three identifiers are named
// separately, because a caller holding some of them needs to know which one is
// missing to know which listing to run for it.
const (
	getChannelMessageAccountFix   = "no account is selected: run the account domain's list operation and retry naming one with the account parameter"
	getChannelMessageTeamIDFix    = "supply team_id as the teamId field of a teams search hit for a channel message"
	getChannelMessageChannelIDFix = "supply channel_id as the channelId field of a teams search hit for a channel message"
	getChannelMessageMessageIDFix = "supply message_id as the id field of a teams list_channel_messages entry or a teams search hit"
	getChannelMessageTimeoutFix   = "retry, and if the deadline is hit again confirm the identifiers with the teams list_channel_messages operation"
	getChannelMessageGraphFix     = "check that team_id, channel_id, and message_id name a post in a channel this account can read and that consent was granted after OUTLOOK_MCP_TEAMS_ENABLED was set, so ChannelMessage.Read.All was requested, then retry; see docs/troubleshooting.md#authentication-failures"
)

// The endpoint this verb reads, stated once so the log records and the timeout
// and failure paths cannot drift from each other.
const getChannelMessageEndpoint = "GET /teams/{team-id}/channels/{channel-id}/messages/{message-id}"

// NewHandleGetChannelMessage creates the handler for the teams
// get_channel_message verb. It reads
// GET /teams/{team-id}/channels/{channel-id}/messages/{message-id} and returns
// the post with a body preview by default and the full body under output=raw.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method.
//
// Every identifier is validated before any request is issued, so a malformed one
// costs no Graph call. The escalation the verb offers is over the size of the
// returned text, not over the number of requests: one request serves every tier,
// because Graph returns the whole body whether or not the caller asked for it.
func NewHandleGetChannelMessage(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)
		start := time.Now()

		client, err := GraphClient(ctx)
		if err != nil {
			logger.Error("no account selected", "fix", getChannelMessageAccountFix)
			return mcp.NewToolResultError(getChannelMessageAccountFix), nil
		}

		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		teamID := request.GetString("team_id", "")
		if err := validate.ValidateResourceID(teamID, "team_id"); err != nil {
			logger.Error("team_id rejected", "error", err.Error(), "fix", getChannelMessageTeamIDFix)
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), getChannelMessageTeamIDFix)), nil
		}

		channelID := request.GetString("channel_id", "")
		if err := validate.ValidateResourceID(channelID, "channel_id"); err != nil {
			logger.Error("channel_id rejected", "error", err.Error(), "fix", getChannelMessageChannelIDFix)
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), getChannelMessageChannelIDFix)), nil
		}

		messageID := request.GetString("message_id", "")
		if err := validate.ValidateResourceID(messageID, "message_id"); err != nil {
			logger.Error("message_id rejected", "error", err.Error(), "fix", getChannelMessageMessageIDFix)
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), getChannelMessageMessageIDFix)), nil
		}

		logger.Debug("tool called", "team_id", teamID, "channel_id", channelID, "message_id", messageID)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		logger.Debug("graph API request", "endpoint", getChannelMessageEndpoint)

		var msg models.ChatMessageable
		graphErr := graph.RetryGraphCall(ctx, retryCfg, func() error {
			var callErr error
			msg, callErr = client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).
				Messages().ByChatMessageId(messageID).Get(timeoutCtx, nil)
			return callErr
		})
		if graphErr != nil {
			if graph.IsTimeoutError(graphErr) {
				logger.ErrorContext(ctx, "request timed out",
					"endpoint", getChannelMessageEndpoint,
					"timeout_seconds", int(timeout.Seconds()),
					"fix", getChannelMessageTimeoutFix,
					"error", graphErr.Error())
				return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
					graph.TimeoutErrorMessage(int(timeout.Seconds())), getChannelMessageTimeoutFix)), nil
			}
			logger.Error("graph API call failed",
				"endpoint", getChannelMessageEndpoint,
				"error", graph.FormatGraphError(graphErr),
				"fix", getChannelMessageGraphFix,
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
				graph.RedactGraphError(graphErr), getChannelMessageGraphFix)), nil
		}

		logger.Debug("graph API response", "endpoint", getChannelMessageEndpoint, "status", "ok")

		result := SerializeSummaryChannelMessage(msg)
		if outputMode == "raw" {
			result = SerializeChannelMessage(msg)
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
