// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the handler for the teams compose_reply verb, which drafts
// a reply and sends nothing. It reads the parent message so the draft quotes
// what it answers, then returns prepared text; no send scope is requested in any
// configuration, so the absence of a write here is backed by the absence of the
// permission to perform one. Posting the text is a manual action the user takes
// in Microsoft Teams.
//
// @agents-index: Handler constructor for the teams.compose_reply verb, reading
// the parent message for context and returning prepared reply text without
// posting anything.
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
)

// Corrections appended to this verb's refusals. The parent is addressed either
// as a chat message or as a channel post, and the two identifier sets are
// disjoint, so the correction for a missing location names both shapes rather
// than guessing which one the caller meant.
const (
	composeReplyAccountFix   = "no account is selected: run the account domain's list operation and retry naming one with the account parameter"
	composeReplyBodyFix      = "supply body as the reply text to prepare; this operation returns the text and posts nothing"
	composeReplyMessageIDFix = "supply message_id as the id field of the message being answered, from a teams list_chat_messages or list_channel_messages entry or a teams search hit"
	composeReplyLocationFix  = "supply either chat_id for a chat message or both team_id and channel_id for a channel post, as carried by the teams search hit or listing entry the message came from"
	composeReplyTimeoutFix   = "retry, and if the deadline is hit again confirm the identifiers by reading the parent with the teams get_chat_message or get_channel_message operation"
	composeReplyGraphFix     = "check that the identifiers name a message this account can read and that consent was granted after OUTLOOK_MCP_TEAMS_ENABLED was set, so Chat.Read and ChannelMessage.Read.All were requested, then retry; see docs/troubleshooting.md#authentication-failures"
)

// The maximum reply body this verb will prepare. The bound exists so a runaway
// caller cannot turn one tool result into an unbounded response; it is generous
// against any reply a person would write by hand.
const composeReplyMaxBodyLen = 32000

// NewHandleComposeReply creates the handler for the teams compose_reply verb. It
// reads the parent message named by the caller, quotes it, and returns the
// prepared reply text.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method.
//
// The only Graph request the handler issues is the GET that reads the parent.
// It posts, patches, and sends nothing, and returns its prepared text
// unconditionally rather than across output tiers, because there is no resource
// behind it to project.
func NewHandleComposeReply(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)
		start := time.Now()

		client, err := GraphClient(ctx)
		if err != nil {
			logger.Error("no account selected", "fix", composeReplyAccountFix)
			return mcp.NewToolResultError(composeReplyAccountFix), nil
		}

		body := request.GetString("body", "")
		if body == "" {
			logger.Error("body rejected", "error", "body must not be empty", "fix", composeReplyBodyFix)
			return mcp.NewToolResultError(fmt.Sprintf("body must not be empty: %s", composeReplyBodyFix)), nil
		}
		if err := validate.ValidateStringLength(body, "body", composeReplyMaxBodyLen); err != nil {
			logger.Error("body rejected", "error", err.Error(), "fix", composeReplyBodyFix)
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), composeReplyBodyFix)), nil
		}

		messageID := request.GetString("message_id", "")
		if err := validate.ValidateResourceID(messageID, "message_id"); err != nil {
			logger.Error("message_id rejected", "error", err.Error(), "fix", composeReplyMessageIDFix)
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), composeReplyMessageIDFix)), nil
		}

		chatID := request.GetString("chat_id", "")
		teamID := request.GetString("team_id", "")
		channelID := request.GetString("channel_id", "")
		if err := validateComposeReplyLocation(chatID, teamID, channelID); err != nil {
			logger.Error("parent location rejected", "error", err.Error(), "fix", composeReplyLocationFix)
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s", err.Error(), composeReplyLocationFix)), nil
		}

		endpoint := "GET /me/chats/{chat-id}/messages/{message-id}"
		if chatID == "" {
			endpoint = "GET /teams/{team-id}/channels/{channel-id}/messages/{message-id}"
		}

		logger.Debug("tool called", "chat_id", chatID, "team_id", teamID, "channel_id", channelID, "message_id", messageID)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		logger.Debug("graph API request", "endpoint", endpoint)

		var parent models.ChatMessageable
		graphErr := graph.RetryGraphCall(ctx, retryCfg, func() error {
			var callErr error
			if chatID != "" {
				parent, callErr = client.Me().Chats().ByChatId(chatID).Messages().ByChatMessageId(messageID).Get(timeoutCtx, nil)
				return callErr
			}
			parent, callErr = client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).
				Messages().ByChatMessageId(messageID).Get(timeoutCtx, nil)
			return callErr
		})
		if graphErr != nil {
			if graph.IsTimeoutError(graphErr) {
				logger.ErrorContext(ctx, "request timed out",
					"endpoint", endpoint,
					"timeout_seconds", int(timeout.Seconds()),
					"fix", composeReplyTimeoutFix,
					"error", graphErr.Error())
				return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
					graph.TimeoutErrorMessage(int(timeout.Seconds())), composeReplyTimeoutFix)), nil
			}
			logger.Error("graph API call failed",
				"endpoint", endpoint,
				"error", graph.FormatGraphError(graphErr),
				"fix", composeReplyGraphFix,
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("%s: %s",
				graph.RedactGraphError(graphErr), composeReplyGraphFix)), nil
		}

		logger.Debug("graph API response", "endpoint", endpoint, "status", "ok")

		// The raw serialization is used for the quote even though this verb has no
		// output tiers: a quote of a truncated preview would misrepresent what the
		// draft is answering.
		quoted := SerializeChatMessage(parent)
		if chatID == "" {
			quoted = SerializeChannelMessage(parent)
		}

		logger.Info("tool completed", "duration", time.Since(start), "sent", false)
		return mcp.NewToolResultText(FormatPreparedTeamsReplyText(quoted, body)), nil
	}
}

// validateComposeReplyLocation checks that the parent message is addressed as
// exactly one of a chat message or a channel post. A caller supplying both has
// named two different messages and no reading of the request is safe; a caller
// supplying neither has named none.
//
// Parameters:
//   - chatID: the chat identifier, empty when the parent is a channel post.
//   - teamID: the team identifier, empty when the parent is a chat message.
//   - channelID: the channel identifier, empty when the parent is a chat message.
//
// Returns nil when exactly one complete location is supplied, or an error
// stating which reading of the request failed.
func validateComposeReplyLocation(chatID, teamID, channelID string) error {
	inChat := chatID != ""
	inChannel := teamID != "" || channelID != ""

	switch {
	case inChat && inChannel:
		return fmt.Errorf("chat_id must not be combined with team_id or channel_id")
	case inChat:
		return validate.ValidateResourceID(chatID, "chat_id")
	case inChannel:
		if err := validate.ValidateResourceID(teamID, "team_id"); err != nil {
			return err
		}
		return validate.ValidateResourceID(channelID, "channel_id")
	default:
		return fmt.Errorf("the parent message location must be given")
	}
}
