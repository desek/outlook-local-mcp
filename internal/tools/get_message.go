// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the get_message MCP tool, which retrieves the full details
// of a single email message by ID via the Microsoft Graph API. The response
// includes all message fields: body content, all recipient fields (to, cc,
// bcc), internet message headers, and attachment metadata. Output modes
// (summary/raw) control the level of detail returned.
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
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

// getMessageFullSelectFields defines the $select fields for get_message in raw
// mode. This is the comprehensive field set including body, all recipient
// fields, internet message headers, and attachment metadata — the full set
// returned by SerializeMessage.
var getMessageFullSelectFields = []string{
	"id", "subject", "bodyPreview", "from", "toRecipients",
	"receivedDateTime", "importance", "isRead", "hasAttachments",
	"conversationId", "webLink", "categories", "flag",
	"body", "ccRecipients", "bccRecipients", "sentDateTime",
	"conversationIndex", "internetMessageId", "parentFolderId",
	"replyTo", "internetMessageHeaders",
}

// getMessageSummarySelectFields defines the $select fields for get_message
// in summary mode. These correspond to the summary fields from
// SerializeSummaryMessage.
var getMessageSummarySelectFields = []string{
	"id", "subject", "bodyPreview", "from", "toRecipients",
	"receivedDateTime", "importance", "isRead", "hasAttachments",
	"conversationId", "webLink", "categories", "flag",
}

// NewGetMessageTool creates the MCP tool definition for get_message. The tool
// retrieves the full details of a single email message by its ID, including
// body content, all recipient fields, internet message headers, and attachment
// metadata. It is annotated as read-only since it only retrieves data from the
// Graph API without making any modifications.
//
// Parameters:
//   - message_id: required unique identifier of the message to retrieve.
//   - account: optional account label for multi-account selection.
//   - output: optional output mode (summary/raw).
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewGetMessageTool() mcp.Tool {
	return mcp.NewTool("mail_get_message",
		mcp.WithDescription("Get full details of a single email message by its ID. Default output includes bodyPreview (plain-text snippet); full HTML body and headers are only available via output=raw."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithString("message_id",
			mcp.Required(),
			mcp.Description("The unique identifier of the message to retrieve."),
		),
		mcp.WithString("account",
			mcp.Description("Account label to use. If omitted, the default account is used. Use account_list to see available accounts."),
		),
		mcp.WithString("output",
			mcp.Description("Output mode: 'text' (default) shows body preview in plain text, 'summary' returns compact JSON with bodyPreview field, 'raw' returns full Graph API fields including full body with HTML content and headers."),
			mcp.Enum("text", "summary", "raw"),
		),
	)
}

// NewHandleGetMessage creates a tool handler that retrieves full message
// details by ID by calling GET /me/messages/{id} via the Graph SDK. The Graph
// client is retrieved from the request context at invocation time.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//
// Returns a tool handler function compatible with the MCP server's AddTool method.
//
// The handler:
//   - Retrieves the Graph client from context via GraphClient.
//   - Extracts and validates the required message_id parameter.
//   - Validates the optional output mode parameter.
//   - Wraps the Graph API call with RetryGraphCall for transient error handling.
//   - Applies a comprehensive $select covering all message fields (raw) or
//     summary fields (summary) depending on output mode.
//   - Serializes the message using SerializeMessage (raw) or
//     SerializeSummaryMessage (summary) from the graph package.
//   - Returns Graph API errors via mcp.NewToolResultError with RedactGraphError.
//   - Returns timeout errors via mcp.NewToolResultError with TimeoutErrorMessage.
//   - Logs entry at debug level, completion at info level, errors at error level.
func NewHandleGetMessage(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := slog.With("tool", "mail_get_message")
		start := time.Now()

		client, err := GraphClient(ctx)
		if err != nil {
			return mcp.NewToolResultError("no account selected"), nil
		}

		// Extract and validate required message_id parameter.
		messageID, err := request.RequireString("message_id")
		if err != nil || messageID == "" {
			return mcp.NewToolResultError("missing required parameter: message_id. Tip: Use mail_list_messages or mail_search_messages to find the message ID."), nil
		}
		if err := validate.ValidateResourceID(messageID, "message_id"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Validate output mode.
		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		logger.Debug("tool called",
			"message_id", messageID,
			"output", outputMode)

		// Select fields based on output mode. Text and summary use summary fields;
		// raw uses the full field set.
		selectFields := getMessageSummarySelectFields
		if outputMode == "raw" {
			selectFields = getMessageFullSelectFields
		}

		// Build request configuration.
		cfg := &users.ItemMessagesMessageItemRequestBuilderGetRequestConfiguration{
			QueryParameters: &users.ItemMessagesMessageItemRequestBuilderGetQueryParameters{
				Select: selectFields,
			},
		}

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		logger.Debug("graph API request",
			"endpoint", "GET /me/messages/{id}",
			"message_id", messageID)

		var msg models.Messageable
		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			var graphErr error
			msg, graphErr = client.Me().Messages().ByMessageId(messageID).Get(timeoutCtx, cfg)
			return graphErr
		})
		if err != nil {
			if graph.IsTimeoutError(err) {
				logger.ErrorContext(ctx, "request timed out",
					"timeout_seconds", int(timeout.Seconds()),
					"error", err.Error())
				return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds()))), nil
			}
			logger.Error("graph API call failed",
				"error", graph.FormatGraphError(err),
				"message_id", messageID,
				"duration", time.Since(start))
			return mcp.NewToolResultError(graph.RedactGraphError(err)), nil
		}

		logger.Debug("graph API response",
			"endpoint", "GET /me/messages/{id}",
			"message_id", messageID)

		// Serialize message based on output mode.
		var result map[string]any
		if outputMode == "raw" {
			result = graph.SerializeMessage(msg)
		} else {
			result = graph.SerializeSummaryMessage(msg)
		}

		// Return text output when requested.
		if outputMode == "text" {
			logger.Info("tool completed",
				"duration", time.Since(start),
				"message_id", messageID)
			return mcp.NewToolResultText(FormatMessageDetailText(result)), nil
		}

		jsonBytes, err := json.Marshal(result)
		if err != nil {
			logger.Error("json serialization failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize message: %s", err.Error())), nil
		}

		logger.Info("tool completed",
			"duration", time.Since(start),
			"message_id", messageID)
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}
