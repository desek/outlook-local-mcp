// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the list_calendars MCP tool, which retrieves all calendars
// accessible to the authenticated user via the Microsoft Graph API. The tool
// returns a JSON array of calendar objects, each containing identification,
// display, and ownership metadata.
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
)

// NewListCalendarsTool creates the MCP tool definition for list_calendars.
// The tool accepts an optional account parameter for multi-account selection
// and is annotated as read-only since it only retrieves data from the Graph
// API without making any modifications.
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewListCalendarsTool() mcp.Tool {
	return mcp.NewTool("calendar_list",
		mcp.WithDescription("List all calendars accessible to the authenticated user."),
		mcp.WithTitleAnnotation("List Calendars"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithString("account",
			mcp.Description(AccountParamDescription),
		),
		mcp.WithString("output",
			mcp.Description("Output mode: 'text' (default) returns plain-text listing, 'summary' returns compact JSON, 'raw' returns full Graph API fields."),
			mcp.Enum("text", "summary", "raw"),
		),
	)
}

// NewHandleListCalendars creates a tool handler that lists all calendars for
// the authenticated user by calling GET /me/calendars via the Graph SDK.
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
//   - Calls client.Me().Calendars().Get(ctx, nil) to retrieve all calendars.
//   - Serializes each calendar into a map with keys: id, name, color, hexColor,
//     isDefaultCalendar, canEdit, and owner (containing name and address).
//   - Returns the JSON array via mcp.NewToolResultText.
//   - Returns timeout errors via mcp.NewToolResultError with timeoutErrorMessage.
//   - Returns Graph API errors via mcp.NewToolResultError with formatGraphError.
//   - Logs entry at debug level, completion at info level with duration and count,
//     and errors at error level.
func NewHandleListCalendars(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := slog.With("tool", "calendar_list")
		start := time.Now()

		// Validate output mode.
		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		logger.Debug("tool called")

		client, err := GraphClient(ctx)
		if err != nil {
			return mcp.NewToolResultError("no account selected"), nil
		}

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		logger.Debug("graph API request",
			"endpoint", "GET /me/calendars")

		var resp models.CalendarCollectionResponseable
		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			var graphErr error
			resp, graphErr = client.Me().Calendars().Get(timeoutCtx, nil)
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
			"endpoint", "GET /me/calendars",
			"count", len(resp.GetValue()))

		calendars := resp.GetValue()
		results := make([]map[string]any, 0, len(calendars))
		for _, cal := range calendars {
			results = append(results, SerializeCalendar(cal))
		}

		// Return text output when requested.
		if outputMode == "text" {
			logger.Info("tool completed",
				"duration", time.Since(start),
				"count", len(results))
			return mcp.NewToolResultText(FormatCalendarsText(results)), nil
		}

		jsonBytes, err := json.Marshal(results)
		if err != nil {
			logger.Error("json serialization failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize calendars: %s", err.Error())), nil
		}

		logger.Info("tool completed",
			"duration", time.Since(start),
			"count", len(results))
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}

// SerializeCalendar converts a models.Calendarable into a map[string]any suitable
// for JSON serialization in MCP tool responses. All pointer fields are accessed
// through SafeStr and SafeBool helpers to prevent nil dereference panics.
//
// Parameters:
//   - cal: a models.Calendarable representing a calendar from the Microsoft Graph API.
//
// Returns a map containing keys: id, name, color, hexColor, isDefaultCalendar,
// canEdit, and owner (with name and address sub-keys). The color field is
// extracted from the CalendarColor enum via its String() method.
//
// Side effects: none.
func SerializeCalendar(cal models.Calendarable) map[string]any {
	result := map[string]any{
		"id":                graph.SafeStr(cal.GetId()),
		"name":              graph.SafeStr(cal.GetName()),
		"hexColor":          graph.SafeStr(cal.GetHexColor()),
		"isDefaultCalendar": graph.SafeBool(cal.GetIsDefaultCalendar()),
		"canEdit":           graph.SafeBool(cal.GetCanEdit()),
	}

	// Color: CalendarColor enum with String() method.
	if c := cal.GetColor(); c != nil {
		result["color"] = c.String()
	} else {
		result["color"] = ""
	}

	// Owner: nested EmailAddressable with name and address.
	if owner := cal.GetOwner(); owner != nil {
		result["owner"] = map[string]string{
			"name":    graph.SafeStr(owner.GetName()),
			"address": graph.SafeStr(owner.GetAddress()),
		}
	} else {
		result["owner"] = map[string]string{
			"name":    "",
			"address": "",
		}
	}

	return result
}
