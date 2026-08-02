// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the search_messages MCP tool, which performs full-text
// search across email messages using Microsoft Graph's KQL $search parameter.
// The tool supports optional folder scoping, pagination via max_results, and
// summary/raw output modes. Note: $search cannot be combined with $filter, and
// $orderby is restricted when $search is used (results are ranked by relevance).
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
	msgraphcore "github.com/microsoftgraph/msgraph-sdk-go-core"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

// searchMessagesSummarySelectFields defines the $select fields for
// search_messages in summary mode. These correspond to the summary fields from
// SerializeSummaryMessage.
var searchMessagesSummarySelectFields = []string{
	"id", "subject", "bodyPreview", "from", "toRecipients",
	"receivedDateTime", "importance", "isRead", "hasAttachments",
	"conversationId", "webLink", "categories", "flag",
}

// searchMessagesFullSelectFields defines the $select fields for
// search_messages in raw mode. These include all summary fields plus detail
// fields returned by SerializeMessage.
var searchMessagesFullSelectFields = []string{
	"id", "subject", "bodyPreview", "from", "toRecipients",
	"receivedDateTime", "importance", "isRead", "hasAttachments",
	"conversationId", "webLink", "categories", "flag",
	"body", "ccRecipients", "bccRecipients", "sentDateTime",
	"conversationIndex", "internetMessageId", "parentFolderId",
	"replyTo", "internetMessageHeaders",
}

// NewHandleSearchMessages creates a tool handler that searches email messages
// by calling the Graph API's messages endpoint with the $search parameter. The
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
//   - Validates the required query parameter (returns error when empty).
//   - Normalises the query via NormaliseSearchQuery once, before the folder
//     branch and before any Graph call, so both request paths send an identical
//     $search value and an unconvertible query is refused (with the error in
//     both the tool result and the log) rather than reaching Graph.
//   - Routes to /me/messages or /me/mailFolders/{id}/messages based on folder_id.
//   - Sets $search with the normalised KQL query string.
//   - Does NOT use $filter or $orderby (incompatible with $search).
//   - Uses PageIterator for pagination with a max_results cap.
//   - Serializes messages using SerializeSummaryMessage or SerializeMessage from
//     the graph package depending on the output mode.
//   - Returns Graph API errors via mcp.NewToolResultError with RedactGraphError.
//   - Returns timeout errors via mcp.NewToolResultError with TimeoutErrorMessage.
//   - Logs entry at debug level, completion at info level, errors at error level.
func NewHandleSearchMessages(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)
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

		// Extract and validate required query parameter.
		query := request.GetString("query", "")
		if query == "" {
			return mcp.NewToolResultError("query is required: provide a KQL search string (e.g., subject:\"Design Review\")"), nil
		}

		// Extract and validate optional folder_id.
		folderID := request.GetString("folder_id", "")
		if folderID != "" {
			if err := validate.ValidateResourceID(folderID, "folder_id"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}

		// Normalise the query into a value Microsoft Graph accepts as a $search
		// value. This runs once, before the folder branch below, so both request
		// paths send the same value and cannot diverge. It also runs before the
		// timeout context and the retry wrapper, so an unconvertible query is
		// refused here and no request is ever issued for it. The error reaches
		// both the tool result and the log record, so a headless caller that
		// cannot see an interactive surface still receives the correction.
		normalised, err := NormaliseSearchQuery(query)
		if err != nil {
			logger.Error("search query rejected", "error", err.Error())
			return mcp.NewToolResultError(err.Error()), nil
		}

		maxResultsFloat := request.GetFloat("max_results", 25)
		maxResults := int(maxResultsFloat)
		if maxResults < 1 {
			maxResults = 25
		}
		if maxResults > 100 {
			maxResults = 100
		}

		logger.Debug("tool called",
			"query", query,
			"folder_id", folderID,
			"max_results", maxResults)

		// Select fields based on output mode.
		selectFields := searchMessagesSummarySelectFields
		if outputMode == "raw" {
			selectFields = searchMessagesFullSelectFields
		}

		top := int32(maxResults)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		var resp models.MessageCollectionResponseable
		var graphErr error

		if folderID != "" {
			// Route to specific folder's messages with $search.
			qp := &users.ItemMailFoldersItemMessagesRequestBuilderGetQueryParameters{
				Select: selectFields,
				Search: &normalised,
				Top:    &top,
			}
			cfg := &users.ItemMailFoldersItemMessagesRequestBuilderGetRequestConfiguration{
				QueryParameters: qp,
			}
			logger.Debug("graph API request",
				"endpoint", "GET /me/mailFolders/{id}/messages",
				"folder_id", folderID,
				"search", normalised,
				"top", top)
			graphErr = graph.RetryGraphCall(ctx, retryCfg, func() error {
				var err error
				resp, err = client.Me().MailFolders().ByMailFolderId(folderID).Messages().Get(timeoutCtx, cfg)
				return err
			})
		} else {
			// Route to all messages with $search.
			qp := &users.ItemMessagesRequestBuilderGetQueryParameters{
				Select: selectFields,
				Search: &normalised,
				Top:    &top,
			}
			cfg := &users.ItemMessagesRequestBuilderGetRequestConfiguration{
				QueryParameters: qp,
			}
			logger.Debug("graph API request",
				"endpoint", "GET /me/messages",
				"search", normalised,
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
