// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the list_messages MCP tool, which retrieves email messages
// from the authenticated user's mailbox via the Microsoft Graph API. The tool
// supports filtering by folder, date range, sender, and conversation ID using
// OData $filter. Results are ordered by receivedDateTime descending and support
// pagination via max_results. Output modes (summary/raw) control the level of
// detail returned.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
	abstractions "github.com/microsoft/kiota-abstractions-go"
	msgraphcore "github.com/microsoftgraph/msgraph-sdk-go-core"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

// listMessagesSummarySelectFields defines the $select fields for list_messages
// in summary mode. These correspond to the summary fields from
// SerializeSummaryMessage, excluding heavy fields like body and headers.
var listMessagesSummarySelectFields = []string{
	"id", "subject", "bodyPreview", "from", "toRecipients",
	"receivedDateTime", "importance", "isRead", "hasAttachments",
	"conversationId", "webLink", "categories", "flag",
}

// listMessagesFullSelectFields defines the $select fields for list_messages
// in raw mode. These include all summary fields plus detail fields returned
// by SerializeMessage.
var listMessagesFullSelectFields = []string{
	"id", "subject", "bodyPreview", "from", "toRecipients",
	"receivedDateTime", "importance", "isRead", "hasAttachments",
	"conversationId", "webLink", "categories", "flag",
	"body", "ccRecipients", "bccRecipients", "sentDateTime",
	"conversationIndex", "internetMessageId", "parentFolderId",
	"replyTo", "internetMessageHeaders",
}

// NewListMessagesTool creates the MCP tool definition for list_messages. The
// tool lists messages in a specific mail folder or across all folders, with
// OData $filter support for date ranges, sender filtering, and conversation
// threading. It is annotated as read-only since it only retrieves data from
// the Graph API without making any modifications.
//
// Parameters:
//   - folder_id: optional mail folder ID to scope the query.
//   - start_datetime: optional start of date range filter (ISO 8601).
//   - end_datetime: optional end of date range filter (ISO 8601).
//   - from: optional sender email address filter.
//   - conversation_id: optional conversation ID for thread retrieval.
//   - max_results: optional maximum number of messages (default 25, max 100).
//   - timezone: optional IANA timezone for the Prefer header.
//   - account: optional account label for multi-account selection.
//   - output: optional output mode (summary/raw).
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewListMessagesTool() mcp.Tool {
	return mcp.NewTool("mail_list_messages",
		mcp.WithDescription("List email messages in a mail folder or across all folders. Supports filtering by date range, sender, and conversation ID via OData $filter. Use mail_list_folders to discover folder IDs. For full-text search, use mail_search_messages instead."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithString("folder_id",
			mcp.Description("Mail folder ID to list messages from. If omitted, lists from all folders. Use mail_list_folders to discover folder IDs."),
		),
		mcp.WithString("start_datetime",
			mcp.Description("Start of date range filter in ISO 8601 format (e.g., 2026-03-12T00:00:00Z). Filters by receivedDateTime >= this value."),
		),
		mcp.WithString("end_datetime",
			mcp.Description("End of date range filter in ISO 8601 format (e.g., 2026-03-13T00:00:00Z). Filters by receivedDateTime <= this value."),
		),
		mcp.WithString("from",
			mcp.Description("Sender email address to filter by (e.g., alice@contoso.com). Filters by from/emailAddress/address."),
		),
		mcp.WithString("conversation_id",
			mcp.Description("Conversation ID to retrieve all messages in a thread. Use a conversationId from a previous message result."),
		),
		mcp.WithNumber("max_results",
			mcp.Description("Maximum number of messages to return (default 25, max 100)."),
			mcp.Min(1),
			mcp.Max(100),
		),
		mcp.WithString("timezone",
			mcp.Description("IANA timezone name for the Prefer: outlook.timezone header (e.g., America/New_York)."),
		),
		mcp.WithString("account",
			mcp.Description("Account label to use. If omitted, the default account is used. Use account_list to see available accounts."),
		),
		mcp.WithString("output",
			mcp.Description("Output mode: 'text' (default) returns plain-text listing, 'summary' returns compact JSON, 'raw' returns full Graph API fields including body and headers."),
			mcp.Enum("text", "summary", "raw"),
		),
	)
}

// NewHandleListMessages creates a tool handler that lists email messages by
// calling the Graph API's messages endpoint with OData $filter support. The
// Graph client is retrieved from the request context at invocation time.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//
// Returns a tool handler function compatible with the MCP server's AddTool method.
//
// The handler:
//   - Retrieves the Graph client from context via GraphClient.
//   - Validates optional parameters (folder_id, start_datetime, end_datetime,
//     from, conversation_id, timezone).
//   - Builds an OData $filter string from provided filter parameters, ANDing
//     multiple conditions together.
//   - Routes to /me/messages or /me/mailFolders/{id}/messages based on folder_id.
//   - Orders results by receivedDateTime desc.
//   - Uses PageIterator for pagination with a max_results cap.
//   - Serializes messages using SerializeSummaryMessage or SerializeMessage from
//     the graph package depending on the output mode.
//   - Returns Graph API errors via mcp.NewToolResultError with RedactGraphError.
//   - Returns timeout errors via mcp.NewToolResultError with TimeoutErrorMessage.
//   - Logs entry at debug level, completion at info level, errors at error level.
func NewHandleListMessages(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := slog.With("tool", "mail_list_messages")
		start := time.Now()

		client, err := GraphClient(ctx)
		if err != nil {
			return mcp.NewToolResultError("no account selected"), nil
		}

		// Validate output mode.
		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Extract and validate optional parameters.
		folderID := request.GetString("folder_id", "")
		if folderID != "" {
			if err := validate.ValidateResourceID(folderID, "folder_id"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}

		startDatetime := request.GetString("start_datetime", "")
		if startDatetime != "" {
			if err := validate.ValidateDatetime(startDatetime, "start_datetime"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}

		endDatetime := request.GetString("end_datetime", "")
		if endDatetime != "" {
			if err := validate.ValidateDatetime(endDatetime, "end_datetime"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}

		fromEmail := request.GetString("from", "")
		if fromEmail != "" {
			if err := validate.ValidateEmail(fromEmail); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}

		conversationID := request.GetString("conversation_id", "")
		if conversationID != "" {
			if err := validate.ValidateResourceID(conversationID, "conversation_id"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}

		timezone := request.GetString("timezone", "")
		if timezone != "" {
			if err := validate.ValidateTimezone(timezone, "timezone"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}

		maxResultsFloat := request.GetFloat("max_results", 25)
		maxResults := int(maxResultsFloat)
		if maxResults < 1 {
			maxResults = 25
		}
		if maxResults > 100 {
			maxResults = 100
		}

		// Build OData $filter from provided parameters.
		filter := buildMessageFilter(startDatetime, endDatetime, fromEmail, conversationID)

		logger.Debug("tool called",
			"folder_id", folderID,
			"start_datetime", startDatetime,
			"end_datetime", endDatetime,
			"from", fromEmail,
			"conversation_id", conversationID,
			"max_results", maxResults,
			"filter", filter)

		// Select fields based on output mode.
		selectFields := listMessagesSummarySelectFields
		if outputMode == "raw" {
			selectFields = listMessagesFullSelectFields
		}

		top := int32(maxResults)
		orderby := []string{"receivedDateTime desc"}

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		var resp models.MessageCollectionResponseable
		var graphErr error

		if folderID != "" {
			// Route to specific folder's messages.
			qp := &users.ItemMailFoldersItemMessagesRequestBuilderGetQueryParameters{
				Select:  selectFields,
				Orderby: orderby,
				Top:     &top,
			}
			if filter != "" {
				qp.Filter = &filter
			}
			cfg := &users.ItemMailFoldersItemMessagesRequestBuilderGetRequestConfiguration{
				QueryParameters: qp,
			}
			if timezone != "" {
				headers := abstractions.NewRequestHeaders()
				headers.Add("Prefer", fmt.Sprintf("outlook.timezone=\"%s\"", timezone))
				cfg.Headers = headers
			}
			logger.Debug("graph API request",
				"endpoint", "GET /me/mailFolders/{id}/messages",
				"folder_id", folderID,
				"filter", filter,
				"top", top)
			graphErr = graph.RetryGraphCall(ctx, retryCfg, func() error {
				var err error
				resp, err = client.Me().MailFolders().ByMailFolderId(folderID).Messages().Get(timeoutCtx, cfg)
				return err
			})
		} else {
			// Route to all messages.
			qp := &users.ItemMessagesRequestBuilderGetQueryParameters{
				Select:  selectFields,
				Orderby: orderby,
				Top:     &top,
			}
			if filter != "" {
				qp.Filter = &filter
			}
			cfg := &users.ItemMessagesRequestBuilderGetRequestConfiguration{
				QueryParameters: qp,
			}
			if timezone != "" {
				headers := abstractions.NewRequestHeaders()
				headers.Add("Prefer", fmt.Sprintf("outlook.timezone=\"%s\"", timezone))
				cfg.Headers = headers
			}
			logger.Debug("graph API request",
				"endpoint", "GET /me/messages",
				"filter", filter,
				"top", top)
			graphErr = graph.RetryGraphCall(ctx, retryCfg, func() error {
				var err error
				resp, err = client.Me().Messages().Get(timeoutCtx, cfg)
				return err
			})
		}

		if graphErr != nil {
			if graph.IsTimeoutError(graphErr) {
				logger.ErrorContext(ctx, "request timed out",
					"timeout_seconds", int(timeout.Seconds()),
					"error", graphErr.Error())
				return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds()))), nil
			}
			logger.Error("graph API call failed",
				"error", graph.FormatGraphError(graphErr),
				"duration", time.Since(start))
			return mcp.NewToolResultError(graph.RedactGraphError(graphErr)), nil
		}

		logger.Debug("graph API response",
			"endpoint", "GET /me/messages",
			"status", "ok")

		// Paginate through results using PageIterator with max_results cap.
		messages := make([]map[string]any, 0, maxResults)
		pageIterator, err := msgraphcore.NewPageIterator[models.Messageable](
			resp,
			client.GetAdapter(),
			models.CreateMessageCollectionResponseFromDiscriminatorValue,
		)
		if err != nil {
			logger.Error("page iterator creation failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to create page iterator: %s", err.Error())), nil
		}

		if timezone != "" {
			headers := abstractions.NewRequestHeaders()
			headers.Add("Prefer", fmt.Sprintf("outlook.timezone=\"%s\"", timezone))
			pageIterator.SetHeaders(headers)
		}

		err = pageIterator.Iterate(ctx, func(msg models.Messageable) bool {
			if outputMode == "raw" {
				messages = append(messages, graph.SerializeMessage(msg))
			} else {
				messages = append(messages, graph.SerializeSummaryMessage(msg))
			}
			return len(messages) < maxResults
		})
		if err != nil {
			logger.Error("pagination failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to iterate messages: %s", err.Error())), nil
		}

		// Return text output when requested.
		if outputMode == "text" {
			logger.Info("tool completed",
				"duration", time.Since(start),
				"count", len(messages))
			return mcp.NewToolResultText(FormatMessagesText(messages)), nil
		}

		jsonBytes, err := json.Marshal(messages)
		if err != nil {
			logger.Error("json serialization failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize messages: %s", err.Error())), nil
		}

		logger.Info("tool completed",
			"duration", time.Since(start),
			"count", len(messages))
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}

// buildMessageFilter constructs an OData $filter string from the provided
// filter parameters. Multiple conditions are ANDed together. Returns an empty
// string when no filter parameters are provided.
//
// Parameters:
//   - startDatetime: ISO 8601 datetime for receivedDateTime >= filter. Empty to skip.
//   - endDatetime: ISO 8601 datetime for receivedDateTime <= filter. Empty to skip.
//   - fromEmail: sender email address for from/emailAddress/address eq filter. Empty to skip.
//   - conversationID: conversation ID for conversationId eq filter. Empty to skip.
//
// Returns the constructed OData $filter string, or "" if no parameters are set.
//
// Side effects: none.
func buildMessageFilter(startDatetime, endDatetime, fromEmail, conversationID string) string {
	var parts []string

	if startDatetime != "" {
		parts = append(parts, fmt.Sprintf("receivedDateTime ge %s", startDatetime))
	}
	if endDatetime != "" {
		parts = append(parts, fmt.Sprintf("receivedDateTime le %s", endDatetime))
	}
	if fromEmail != "" {
		parts = append(parts, fmt.Sprintf("from/emailAddress/address eq '%s'", fromEmail))
	}
	if conversationID != "" {
		parts = append(parts, fmt.Sprintf("conversationId eq '%s'", conversationID))
	}

	return strings.Join(parts, " and ")
}
