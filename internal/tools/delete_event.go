// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the delete_event MCP tool, which removes a calendar event
// from the authenticated user's calendar via the Microsoft Graph API. When the
// organizer deletes a meeting, cancellation notices are automatically sent to
// attendees. When an attendee deletes, the event is removed only from their
// own calendar.
package tools

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
)

// NewDeleteEventTool creates the MCP tool definition for delete_event. The tool
// accepts a required event_id string parameter. It does not carry a
// ReadOnlyHintAnnotation because it is a destructive write operation.
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewDeleteEventTool() mcp.Tool {
	return mcp.NewTool("calendar_delete_event",
		mcp.WithTitleAnnotation("Delete Calendar Event"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithDescription(
			"Delete a calendar event by ID. When the organizer deletes a meeting, "+
				"cancellation notices are automatically sent to all attendees. When an "+
				"attendee deletes, the event is removed only from their own calendar. "+
				"Deleting a series master deletes all occurrences. For sending a custom "+
				"cancellation message to attendees, use calendar_cancel_meeting instead.",
		),
		mcp.WithString("event_id",
			mcp.Required(),
			mcp.Description("The unique identifier of the event to delete."),
		),
		mcp.WithString("account",
			mcp.Description("Account label to use. If omitted, the default account is used. Use account_list to see available accounts."),
		),
	)
}

// HandleDeleteEvent is the MCP tool handler for delete_event. It extracts the
// event_id from the request arguments, calls the Graph API DELETE endpoint, and
// returns a plain text confirmation on success. The Graph client is retrieved
// from the request context at invocation time.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//
// Returns a closure matching the MCP tool handler function signature. The
// closure returns an *mcp.CallToolResult with either a plain text confirmation
// containing the event ID and a cancellation notice message, or an error result
// formatted via FormatGraphError when the Graph API call fails.
//
// Side effects: calls DELETE /me/events/{id} on the Microsoft Graph API. Logs
// at debug level on entry, error level on failure, and info level on success.
func HandleDeleteEvent(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := slog.With("tool", "calendar_delete_event")

		client, err := GraphClient(ctx)
		if err != nil {
			return mcp.NewToolResultError("no account selected"), nil
		}

		eventID, err := request.RequireString("event_id")
		if err != nil {
			return mcp.NewToolResultError("missing required parameter: event_id. Tip: Use calendar_list_events or calendar_search_events to find the event ID."), nil
		}
		if err := validate.ValidateResourceID(eventID, "event_id"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		logger.DebugContext(ctx, "deleting event", "event_id", eventID)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		if timeoutCtx.Err() != nil {
			logger.ErrorContext(ctx, "request timed out",
				"timeout_seconds", int(timeout.Seconds()),
				"error", timeoutCtx.Err().Error())
			return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds()))), nil
		}

		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			return client.Me().Events().ByEventId(eventID).Delete(timeoutCtx, nil)
		})
		if err != nil {
			if graph.IsTimeoutError(err) {
				logger.ErrorContext(ctx, "request timed out",
					"timeout_seconds", int(timeout.Seconds()),
					"error", err.Error())
				return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds()))), nil
			}
			logger.ErrorContext(ctx, "delete event failed", "event_id", eventID, "error", graph.FormatGraphError(err))
			return mcp.NewToolResultError(graph.RedactGraphError(err)), nil
		}

		logger.InfoContext(ctx, "event deleted", "event_id", eventID)

		response := fmt.Sprintf("Event deleted: %s\nCancellation notices were sent to attendees if applicable.", eventID)
		if line := AccountInfoLine(ctx); line != "" {
			response += "\n" + line
		}
		return mcp.NewToolResultText(response), nil
	}
}
