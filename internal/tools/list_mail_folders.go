// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the list_mail_folders MCP tool, which retrieves the
// authenticated user's mail folders via the Microsoft Graph API. The tool
// returns a JSON array of folder objects, each containing the folder ID,
// display name, unread item count, and total item count.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

// NewListMailFoldersTool creates the MCP tool definition for list_mail_folders.
// The tool accepts an optional account parameter for multi-account selection
// and an optional max_results parameter (default 25) to limit the number of
// folders returned. It is annotated as read-only since it only retrieves data
// from the Graph API without making any modifications.
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewListMailFoldersTool() mcp.Tool {
	return mcp.NewTool("mail_list_folders",
		mcp.WithDescription("List the user's mail folders (Inbox, Sent Items, Drafts, etc.) with display name, unread count, and total count."),
		mcp.WithTitleAnnotation("List Mail Folders"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithString("account",
			mcp.Description("Account label to use. If omitted, the default account is used. Use account_list to see available accounts."),
		),
		mcp.WithNumber("max_results",
			mcp.Description("Maximum number of folders to return (default 25)."),
			mcp.Min(1),
		),
		mcp.WithString("output",
			mcp.Description("Output mode: 'text' (default) returns plain-text listing, 'summary' returns compact JSON, 'raw' returns full Graph API fields."),
			mcp.Enum("text", "summary", "raw"),
		),
	)
}

// NewHandleListMailFolders creates a tool handler that lists mail folders for
// the authenticated user by calling GET /me/mailFolders via the Graph SDK.
// The Graph client is retrieved from the request context at invocation time.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//
// Returns a tool handler function compatible with the MCP server's AddTool method.
//
// The handler:
//   - Retrieves the Graph client from context via GraphClient.
//   - Applies a timeout context before the Graph API call.
//   - Calls client.Me().MailFolders().Get(ctx, cfg) with $top and $select.
//   - Serializes each folder into a map with keys: id, displayName,
//     unreadItemCount, and totalItemCount.
//   - Returns the JSON array via mcp.NewToolResultText.
//   - Returns timeout errors via mcp.NewToolResultError with TimeoutErrorMessage.
//   - Returns Graph API errors via mcp.NewToolResultError with RedactGraphError.
//   - Logs entry at debug level, completion at info level with duration and count,
//     and errors at error level.
func NewHandleListMailFolders(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := slog.With("tool", "mail_list_folders")
		start := time.Now()

		logger.Debug("tool called")

		client, err := GraphClient(ctx)
		if err != nil {
			return mcp.NewToolResultError("no account selected"), nil
		}

		// Validate output mode.
		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		maxResultsFloat := request.GetFloat("max_results", 25)
		maxResults := int32(maxResultsFloat)
		if maxResults < 1 {
			maxResults = 25
		}

		selectFields := []string{"id", "displayName", "unreadItemCount", "totalItemCount"}
		qp := &users.ItemMailFoldersRequestBuilderGetQueryParameters{
			Top:    &maxResults,
			Select: selectFields,
		}
		cfg := &users.ItemMailFoldersRequestBuilderGetRequestConfiguration{
			QueryParameters: qp,
		}

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		logger.Debug("graph API request",
			"endpoint", "GET /me/mailFolders",
			"top", maxResults)

		var resp models.MailFolderCollectionResponseable
		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			var graphErr error
			resp, graphErr = client.Me().MailFolders().Get(timeoutCtx, cfg)
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
				"duration", time.Since(start))
			return mcp.NewToolResultError(graph.RedactGraphError(err)), nil
		}

		logger.Debug("graph API response",
			"endpoint", "GET /me/mailFolders",
			"count", len(resp.GetValue()))

		folders := resp.GetValue()
		results := make([]map[string]any, 0, len(folders))
		for _, folder := range folders {
			results = append(results, serializeMailFolder(folder))
		}

		// Return text output when requested.
		if outputMode == "text" {
			logger.Info("tool completed",
				"duration", time.Since(start),
				"count", len(results))
			return mcp.NewToolResultText(FormatMailFoldersText(results)), nil
		}

		jsonBytes, err := json.Marshal(results)
		if err != nil {
			logger.Error("json serialization failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize mail folders: %s", err.Error())), nil
		}

		logger.Info("tool completed",
			"duration", time.Since(start),
			"count", len(results))
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}

// serializeMailFolder converts a models.MailFolderable into a map[string]any
// suitable for JSON serialization in MCP tool responses. All pointer fields
// are accessed through graph.SafeStr and graph.SafeInt32 helpers to prevent
// nil dereference panics.
//
// Parameters:
//   - folder: a models.MailFolderable representing a mail folder from the
//     Microsoft Graph API.
//
// Returns a map containing keys: id, displayName, unreadItemCount, and
// totalItemCount.
//
// Side effects: none.
func serializeMailFolder(folder models.MailFolderable) map[string]any {
	return map[string]any{
		"id":              graph.SafeStr(folder.GetId()),
		"displayName":     graph.SafeStr(folder.GetDisplayName()),
		"unreadItemCount": graph.SafeInt32(folder.GetUnreadItemCount()),
		"totalItemCount":  graph.SafeInt32(folder.GetTotalItemCount()),
	}
}
