// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the handler for the teams list_channel_message_replies
// verb, which reads the replies hanging under one channel post. A channel
// listing returns thread openers only, so the discussion under a post is
// reachable only here; each reply carries the replyToId naming the parent, so a
// caller merging several threads can still tell which is which.
//
// @agents-index: Handler constructor for the
// teams.list_channel_message_replies verb, reading one channel post's replies
// and projecting them across the three output tiers.
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

// Corrections appended to this verb's refusals, each naming where the identifier
// it is about is obtained.
const (
	listChannelRepliesAccountFix   = "no account is selected: run the account domain's list operation and retry naming one with the account parameter"
	listChannelRepliesTeamIDFix    = "supply team_id as the teamId field of a teams search hit for a channel message"
	listChannelRepliesChannelIDFix = "supply channel_id as the channelId field of a teams search hit for a channel message"
	listChannelRepliesMessageIDFix = "supply message_id as the id field of the parent post, from a teams list_channel_messages entry or a teams search hit"
	listChannelRepliesTimeoutFix   = "retry, and if the deadline is hit again confirm the identifiers with the teams list_channel_messages operation"
	listChannelRepliesGraphFix     = "check that team_id, channel_id, and message_id name a post in a channel this account can read and that consent was granted after OUTLOOK_MCP_TEAMS_ENABLED was set, so ChannelMessage.Read.All was requested, then retry; see docs/troubleshooting.md#authentication-failures"
)

// The endpoint this verb reads, stated once so the log records and the timeout
// and failure paths cannot drift from each other.
const listChannelRepliesEndpoint = "GET /teams/{team-id}/channels/{channel-id}/messages/{message-id}/replies"

// NewHandleListChannelMessageReplies creates the handler for the teams
// list_channel_message_replies verb. It reads
// GET /teams/{team-id}/channels/{channel-id}/messages/{message-id}/replies and
// returns the replies in the order Graph returned them.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method.
//
// Every identifier is validated before any request is issued, so a malformed one
// costs no Graph call, and exactly one request of at most max_results replies
// (1 to 50) is issued on the success path without following the collection's
// next link; when Graph signals more pages, the result says so. Only $top is
// sent because the replies collection supports no other query option.
func NewHandleListChannelMessageReplies(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)
		start := time.Now()

		client, err := GraphClient(ctx)
		if err != nil {
			logger.Error("no account selected", "fix", listChannelRepliesAccountFix)
			return mcp.NewToolResultError(listChannelRepliesAccountFix), nil
		}

		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		teamID := request.GetString("team_id", "")
		if err := validate.ValidateResourceID(teamID, "team_id"); err != nil {
			logger.Error("team_id rejected", "error", err.Error(), "fix", listChannelRepliesTeamIDFix)
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), listChannelRepliesTeamIDFix)), nil
		}

		channelID := request.GetString("channel_id", "")
		if err := validate.ValidateResourceID(channelID, "channel_id"); err != nil {
			logger.Error("channel_id rejected", "error", err.Error(), "fix", listChannelRepliesChannelIDFix)
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), listChannelRepliesChannelIDFix)), nil
		}

		messageID := request.GetString("message_id", "")
		if err := validate.ValidateResourceID(messageID, "message_id"); err != nil {
			logger.Error("message_id rejected", "error", err.Error(), "fix", listChannelRepliesMessageIDFix)
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), listChannelRepliesMessageIDFix)), nil
		}

		logger.Debug("tool called", "team_id", teamID, "channel_id", channelID, "message_id", messageID)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		logger.Debug("graph API request", "endpoint", listChannelRepliesEndpoint)

		var resp models.ChatMessageCollectionResponseable
		graphErr := graph.RetryGraphCall(ctx, retryCfg, func() error {
			var callErr error
			resp, callErr = client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).
				Messages().ByChatMessageId(messageID).Replies().Get(timeoutCtx, listChannelRepliesRequestConfig(teamsPageSize(request)))
			return callErr
		})
		if graphErr != nil {
			if graph.IsTimeoutError(graphErr) {
				logger.ErrorContext(ctx, "request timed out",
					"endpoint", listChannelRepliesEndpoint,
					"timeout_seconds", int(timeout.Seconds()),
					"fix", listChannelRepliesTimeoutFix,
					"error", graphErr.Error())
				return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
					graph.TimeoutErrorMessage(int(timeout.Seconds())), listChannelRepliesTimeoutFix)), nil
			}
			logger.Error("graph API call failed",
				"endpoint", listChannelRepliesEndpoint,
				"error", graph.FormatGraphError(graphErr),
				"fix", listChannelRepliesGraphFix,
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
				graph.RedactGraphError(graphErr), listChannelRepliesGraphFix)), nil
		}

		replies := serializeMessageReplyCollection(resp, outputMode)

		logger.Debug("graph API response",
			"endpoint", listChannelRepliesEndpoint,
			"count", len(replies))

		more := resp != nil && resp.GetOdataNextLink() != nil

		if outputMode == "text" {
			logger.Info("tool completed", "duration", time.Since(start), "count", len(replies), "truncated", more)
			return mcp.NewToolResultText(teamsPagedText(FormatTeamsMessagesText(replies), "replies", len(replies), more)), nil
		}

		jsonBytes, err := json.Marshal(replies)
		if err != nil {
			logger.Error("json serialization failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize replies: %s", err.Error())), nil
		}

		logger.Info("tool completed", "duration", time.Since(start), "count", len(replies), "truncated", more)
		return teamsPagedJSON(string(jsonBytes), "replies", len(replies), more), nil
	}
}

// listChannelRepliesRequestConfig builds the query for one page of replies;
// $top is the only option the replies collection accepts.
func listChannelRepliesRequestConfig(top int32) *msteams.ItemChannelsItemMessagesItemRepliesRequestBuilderGetRequestConfiguration {
	return &msteams.ItemChannelsItemMessagesItemRepliesRequestBuilderGetRequestConfiguration{
		QueryParameters: &msteams.ItemChannelsItemMessagesItemRepliesRequestBuilderGetQueryParameters{Top: &top},
	}
}
