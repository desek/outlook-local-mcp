// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the cancel_meeting MCP tool, which cancels a meeting on
// the authenticated user's calendar via the Microsoft Graph API. Only the
// meeting organizer can cancel; non-organizers receive an access denied error
// from the Graph API. An optional comment parameter allows the organizer to
// include a custom cancellation message sent to all attendees.
package tools

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
	graphusers "github.com/microsoftgraph/msgraph-sdk-go/users"
)

// NewCancelMeetingTool creates the MCP tool definition for cancel_meeting. The
// tool accepts a required event_id and an optional comment string parameter. It
// does not carry a ReadOnlyHintAnnotation because it is a destructive write
// operation.
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewCancelMeetingTool() mcp.Tool {
	return mcp.NewTool("calendar_cancel_meeting",
		mcp.WithTitleAnnotation("Cancel Calendar Meeting"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithDescription(
			"Cancel a meeting and send a cancellation message to all attendees. "+
				"Only the meeting organizer can cancel; non-organizers will receive an "+
				"access denied error. Cancelling a series master cancels all future "+
				"instances. For non-meeting events or when no custom cancellation "+
				"message is needed, use calendar_delete_event instead.\n\n"+
				"IMPORTANT: When the event has attendees, cancelling sends a cancellation "+
				"notice to ALL attendees immediately. You MUST present a summary to the "+
				"user showing the event subject, time, and full attendee list, then wait "+
				"for explicit confirmation before calling this tool. If any attendee is "+
				"external to the user's organization, add an explicit warning about "+
				"external cancellation notices. "+
				"If the AskUserQuestion tool is available, use it to present the summary "+
				"and collect confirmation for a better user experience.",
		),
		mcp.WithString("event_id",
			mcp.Required(),
			mcp.Description("The unique identifier of the meeting to cancel."),
		),
		mcp.WithString("comment",
			mcp.Description("Optional custom cancellation message sent to all attendees."),
		),
		mcp.WithString("account",
			mcp.Description(AccountParamDescription),
		),
	)
}

// HandleCancelEvent is the MCP tool handler for cancel_meeting. It extracts the
// event_id and optional comment from the request arguments, builds the cancel
// request body, calls the Graph API POST /cancel endpoint, and returns a
// plain text confirmation on success. The Graph client is retrieved from the
// request context at invocation time.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//
// Returns a closure matching the MCP tool handler function signature. The
// closure returns an *mcp.CallToolResult with either a plain text confirmation
// containing the event ID and a cancellation message, or an error result
// formatted via FormatGraphError when the Graph API call fails.
//
// Side effects: calls POST /me/events/{id}/cancel on the Microsoft Graph API.
// Logs at debug level on entry, error level on failure, and info level on success.
func HandleCancelEvent(retryCfg graph.RetryConfig, timeout time.Duration) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := slog.With("tool", "calendar_cancel_meeting")

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

		comment := request.GetString("comment", "")
		if comment != "" {
			if err := validate.ValidateStringLength(comment, "comment", validate.MaxCommentLen); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}

		logger.DebugContext(ctx, "cancelling event", "event_id", eventID)

		cancelBody := graphusers.NewItemEventsItemCancelPostRequestBody()
		if comment != "" {
			cancelBody.SetComment(&comment)
		}

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		if timeoutCtx.Err() != nil {
			logger.ErrorContext(ctx, "request timed out",
				"timeout_seconds", int(timeout.Seconds()),
				"error", timeoutCtx.Err().Error())
			return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds()))), nil
		}

		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			return client.Me().Events().ByEventId(eventID).Cancel().Post(timeoutCtx, cancelBody, nil)
		})
		if err != nil {
			if graph.IsTimeoutError(err) {
				logger.ErrorContext(ctx, "request timed out",
					"timeout_seconds", int(timeout.Seconds()),
					"error", err.Error())
				return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds()))), nil
			}
			logger.ErrorContext(ctx, "cancel event failed", "event_id", eventID, "error", graph.FormatGraphError(err))
			return mcp.NewToolResultError(graph.RedactGraphError(err)), nil
		}

		logger.InfoContext(ctx, "event cancelled", "event_id", eventID)

		response := fmt.Sprintf("Event cancelled: %s\nCancellation message sent to all attendees.", eventID)
		if line := AccountInfoLine(ctx); line != "" {
			response += "\n" + line
		}
		return mcp.NewToolResultText(response), nil
	}
}
