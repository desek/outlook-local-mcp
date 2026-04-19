// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the mail_get_conversation MCP tool, which retrieves all
// messages in an email conversation thread in chronological order via the
// Microsoft Graph API. The caller may supply either a message_id (from which
// the conversationId is resolved) or a conversation_id directly. Results are
// returned using the three-tier output model (text/summary/raw).
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
	msgraphcore "github.com/microsoftgraph/msgraph-sdk-go-core"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

// NewGetConversationTool creates the MCP tool definition for
// mail_get_conversation. The tool retrieves all messages in an email thread in
// chronological order (oldest first). Either a message_id (to resolve the
// conversationId) or a conversation_id may be supplied.
//
// Parameters:
//   - message_id: required unless conversation_id is supplied.
//   - conversation_id: optional; when present, skips the initial message fetch.
//   - max_results: optional maximum number of messages (default 50, max 100).
//   - account: optional account label for multi-account selection.
//   - output: optional output mode (text/summary/raw).
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewGetConversationTool() mcp.Tool {
	return mcp.NewTool("mail_get_conversation",
		mcp.WithDescription("Retrieve all messages in an email conversation thread in chronological order. Useful for understanding historical context before drafting a response. Supply either message_id (conversationId will be resolved) or conversation_id directly."),
		mcp.WithTitleAnnotation("Get Email Conversation"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithString("message_id",
			mcp.Description("The unique identifier of a message in the conversation. The conversationId is resolved from this message."),
		),
		mcp.WithString("conversation_id",
			mcp.Description("Conversation ID to retrieve directly (skips the initial message fetch)."),
		),
		mcp.WithNumber("max_results",
			mcp.Description("Maximum number of messages to return (default 50, max 100)."),
			mcp.Min(1),
			mcp.Max(100),
		),
		mcp.WithString("account",
			mcp.Description(AccountParamDescription),
		),
		mcp.WithString("output",
			mcp.Description("Output mode: 'text' (default) returns chronological plain-text thread, 'summary' returns compact JSON, 'raw' returns full Graph API fields per message."),
			mcp.Enum("text", "summary", "raw"),
		),
	)
}

// conversationSummarySelectFields defines $select fields for summary/text modes.
var conversationSummarySelectFields = []string{
	"id", "subject", "bodyPreview", "from", "toRecipients",
	"receivedDateTime", "importance", "isRead", "hasAttachments",
	"conversationId", "webLink", "categories", "flag",
}

// conversationFullSelectFields defines $select fields for raw mode.
var conversationFullSelectFields = []string{
	"id", "subject", "bodyPreview", "from", "toRecipients",
	"receivedDateTime", "importance", "isRead", "hasAttachments",
	"conversationId", "webLink", "categories", "flag",
	"body", "ccRecipients", "bccRecipients", "sentDateTime",
	"conversationIndex", "internetMessageId", "parentFolderId",
	"replyTo", "internetMessageHeaders",
}

// NewHandleGetConversation creates a tool handler that returns a full email
// conversation thread by calling GET /me/messages with a conversationId filter
// and chronological ordering.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for a single Graph API call.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method.
//
// The handler:
//   - Retrieves the Graph client from context via GraphClient.
//   - Requires either message_id or conversation_id; when only message_id is
//     provided, fetches the message to resolve conversationId.
//   - Queries /me/messages with $filter=conversationId eq '...' and
//     $orderby=receivedDateTime asc.
//   - Paginates with a max_results cap.
//   - Serializes each message per the requested output mode and returns the
//     thread via SerializeConversationThread.
func NewHandleGetConversation(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := slog.With("tool", "mail_get_conversation")
		start := time.Now()

		client, err := GraphClient(ctx)
		if err != nil {
			return mcp.NewToolResultError("no account selected"), nil
		}

		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		messageID := request.GetString("message_id", "")
		conversationID := request.GetString("conversation_id", "")
		if messageID == "" && conversationID == "" {
			return mcp.NewToolResultError("missing required parameter: provide either message_id or conversation_id"), nil
		}
		if messageID != "" {
			if err := validate.ValidateResourceID(messageID, "message_id"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
		if conversationID != "" {
			if err := validate.ValidateResourceID(conversationID, "conversation_id"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}

		maxResults := int(request.GetFloat("max_results", 50))
		if maxResults < 1 {
			maxResults = 50
		}
		if maxResults > 100 {
			maxResults = 100
		}

		// Resolve conversationId from the message if not provided directly.
		if conversationID == "" {
			timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
			msgCfg := &users.ItemMessagesMessageItemRequestBuilderGetRequestConfiguration{
				QueryParameters: &users.ItemMessagesMessageItemRequestBuilderGetQueryParameters{
					Select: []string{"id", "conversationId"},
				},
			}
			var msg models.Messageable
			err := graph.RetryGraphCall(ctx, retryCfg, func() error {
				var gErr error
				msg, gErr = client.Me().Messages().ByMessageId(messageID).Get(timeoutCtx, msgCfg)
				return gErr
			})
			cancel()
			if err != nil {
				if graph.IsTimeoutError(err) {
					logger.ErrorContext(ctx, "request timed out", "timeout_seconds", int(timeout.Seconds()), "error", err.Error())
					return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds()))), nil
				}
				logger.Error("graph API call failed (resolve conversation)", "error", graph.FormatGraphError(err), "message_id", messageID, "duration", time.Since(start))
				return mcp.NewToolResultError(graph.RedactGraphError(err)), nil
			}
			if cid := msg.GetConversationId(); cid != nil {
				conversationID = *cid
			}
			if conversationID == "" {
				return mcp.NewToolResultError("message has no conversationId"), nil
			}
		}

		logger.Debug("tool called", "message_id", messageID, "conversation_id", conversationID, "max_results", maxResults, "output", outputMode)

		selectFields := conversationSummarySelectFields
		if outputMode == "raw" {
			selectFields = conversationFullSelectFields
		}
		filter := fmt.Sprintf("conversationId eq '%s'", conversationID)
		orderby := []string{"receivedDateTime asc"}
		top := int32(maxResults)

		qp := &users.ItemMessagesRequestBuilderGetQueryParameters{
			Select:  selectFields,
			Orderby: orderby,
			Top:     &top,
			Filter:  &filter,
		}
		cfg := &users.ItemMessagesRequestBuilderGetRequestConfiguration{QueryParameters: qp}

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		var resp models.MessageCollectionResponseable
		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			var gErr error
			resp, gErr = client.Me().Messages().Get(timeoutCtx, cfg)
			return gErr
		})
		if err != nil {
			if graph.IsTimeoutError(err) {
				logger.ErrorContext(ctx, "request timed out", "timeout_seconds", int(timeout.Seconds()), "error", err.Error())
				return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds()))), nil
			}
			logger.Error("graph API call failed", "error", graph.FormatGraphError(err), "duration", time.Since(start))
			return mcp.NewToolResultError(graph.RedactGraphError(err)), nil
		}

		messages := make([]map[string]any, 0, maxResults)
		pageIterator, pErr := msgraphcore.NewPageIterator[models.Messageable](
			resp, client.GetAdapter(), models.CreateMessageCollectionResponseFromDiscriminatorValue,
		)
		if pErr != nil {
			logger.Error("page iterator creation failed", "error", pErr.Error())
			return mcp.NewToolResultError(fmt.Sprintf("failed to create page iterator: %s", pErr.Error())), nil
		}
		iterErr := pageIterator.Iterate(ctx, func(msg models.Messageable) bool {
			if outputMode == "raw" {
				messages = append(messages, graph.SerializeMessage(msg))
			} else {
				messages = append(messages, graph.SerializeSummaryMessage(msg))
			}
			return len(messages) < maxResults
		})
		if iterErr != nil {
			logger.Error("pagination failed", "error", iterErr.Error())
			return mcp.NewToolResultError(fmt.Sprintf("failed to iterate messages: %s", iterErr.Error())), nil
		}

		thread := graph.SerializeConversationThread(conversationID, messages)

		if outputMode == "text" {
			logger.Info("tool completed", "duration", time.Since(start), "count", len(messages))
			return mcp.NewToolResultText(FormatConversationText(thread)), nil
		}

		jsonBytes, err := json.Marshal(thread)
		if err != nil {
			logger.Error("json serialization failed", "error", err.Error())
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize thread: %s", err.Error())), nil
		}
		logger.Info("tool completed", "duration", time.Since(start), "count", len(messages))
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}
