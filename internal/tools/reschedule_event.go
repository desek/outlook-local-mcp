// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the reschedule_event MCP tool, which moves an existing
// calendar event to a new time while preserving its original duration. The
// handler performs two Graph API calls: GET to retrieve the current event
// start/end, and PATCH to update with the new computed times.
package tools

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

// NewRescheduleEventTool creates the MCP tool definition for reschedule_event.
// The tool accepts a required event_id and new_start_datetime, plus optional
// new_start_timezone (defaults to server config) and optional account selector.
// It preserves the original event duration, computing the new end time
// automatically.
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewRescheduleEventTool() mcp.Tool {
	return mcp.NewTool("calendar_reschedule_event",
		mcp.WithTitleAnnotation("Reschedule Event"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithDescription(
			"Move an event to a new time, preserving its original duration. "+
				"Only the new start time is required; the end time is computed automatically. "+
				"Sends update notifications to attendees if applicable.\n\n"+
				"IMPORTANT: When the event has attendees, rescheduling sends update "+
				"notifications to all attendees. You MUST present a draft summary to the "+
				"user showing the event subject, current time, proposed new time, and "+
				"attendee list, then wait for explicit confirmation before calling this tool.",
		),
		mcp.WithString("event_id", mcp.Required(),
			mcp.Description("The unique identifier of the event to reschedule."),
		),
		mcp.WithString("new_start_datetime", mcp.Required(),
			mcp.Description("New start time in ISO 8601 without offset, e.g. 2026-04-17T14:00:00"),
		),
		mcp.WithString("new_start_timezone",
			mcp.Description("IANA timezone for the new start time. Defaults to the server's configured timezone."),
		),
		mcp.WithString("account",
			mcp.Description("Account label to use. If omitted, the default account is used. Use account_list to see available accounts."),
		),
	)
}

// rescheduleSelectFields defines the minimal $select fields needed for
// reschedule: only start and end times are required to compute the duration.
var rescheduleSelectFields = []string{"id", "start", "end"}

// HandleRescheduleEvent is the MCP tool handler for reschedule_event. It
// retrieves the existing event's start/end times via GET /me/events/{id},
// computes the original duration, adds it to the new start time to produce
// the new end time, and updates the event via PATCH /me/events/{id}.
//
// When new_start_timezone is omitted, the handler defaults to the server's
// configured default timezone.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for each Graph API call.
//   - defaultTimezone: the IANA timezone from server config, used as the default
//     when new_start_timezone is omitted.
//
// Returns a closure matching the MCP tool handler function signature. The
// closure returns an *mcp.CallToolResult with either a text confirmation
// containing the rescheduled event details, or an error result when validation
// or Graph API calls fail.
//
// Side effects: calls GET /me/events/{id} and PATCH /me/events/{id} on the
// Microsoft Graph API (at most 2 calls per invocation). Logs at debug level on
// entry, error level on failure, and info level on success.
func HandleRescheduleEvent(retryCfg graph.RetryConfig, timeout time.Duration, defaultTimezone string) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := slog.With("tool", "calendar_reschedule_event")

		client, err := GraphClient(ctx)
		if err != nil {
			return mcp.NewToolResultError("no account selected"), nil
		}

		// Extract and validate required parameters.
		eventID, err := request.RequireString("event_id")
		if err != nil {
			return mcp.NewToolResultError("missing required parameter: event_id. Tip: Use calendar_list_events or calendar_search_events to find the event ID."), nil
		}
		if err := validate.ValidateResourceID(eventID, "event_id"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		newStartDT, err := request.RequireString("new_start_datetime")
		if err != nil {
			return mcp.NewToolResultError("missing required parameter: new_start_datetime"), nil
		}
		if err := validate.ValidateDatetime(newStartDT, "new_start_datetime"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Default timezone from config when omitted.
		newStartTZ := request.GetString("new_start_timezone", "")
		if newStartTZ == "" {
			newStartTZ = defaultTimezone
		} else {
			if err := validate.ValidateTimezone(newStartTZ, "new_start_timezone"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}

		logger.DebugContext(ctx, "rescheduling event",
			"event_id", eventID,
			"new_start_datetime", newStartDT,
			"new_start_timezone", newStartTZ,
		)

		// Step 1: GET the existing event to retrieve current start/end.
		getCfg := &users.ItemEventsEventItemRequestBuilderGetRequestConfiguration{
			QueryParameters: &users.ItemEventsEventItemRequestBuilderGetQueryParameters{
				Select: rescheduleSelectFields,
			},
		}

		getCtx, getCancel := graph.WithTimeout(ctx, timeout)
		defer getCancel()

		var existing models.Eventable
		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			var graphErr error
			existing, graphErr = client.Me().Events().ByEventId(eventID).Get(getCtx, getCfg)
			return graphErr
		})
		if err != nil {
			if graph.IsTimeoutError(err) {
				logger.ErrorContext(ctx, "GET request timed out",
					"timeout_seconds", int(timeout.Seconds()),
					"error", err.Error())
				return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds()))), nil
			}
			logger.ErrorContext(ctx, "get event failed",
				"event_id", eventID,
				"error", graph.FormatGraphError(err))
			return mcp.NewToolResultError(
				fmt.Sprintf("failed to retrieve event: %s. Tip: Use calendar_list_events or calendar_search_events to verify the event ID.",
					graph.RedactGraphError(err))), nil
		}

		// Extract current start/end datetimes and compute duration.
		oldStart := existing.GetStart()
		oldEnd := existing.GetEnd()
		if oldStart == nil || oldEnd == nil {
			return mcp.NewToolResultError("event has no start or end time"), nil
		}

		oldStartStr := graph.SafeStr(oldStart.GetDateTime())
		oldEndStr := graph.SafeStr(oldEnd.GetDateTime())
		if oldStartStr == "" || oldEndStr == "" {
			return mcp.NewToolResultError("event has empty start or end datetime"), nil
		}

		duration, err := computeEventDuration(oldStartStr, oldEndStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to compute event duration: %s", err.Error())), nil
		}

		// Compute new end = new start + duration.
		newEndDT, err := addDuration(newStartDT, duration)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to compute new end time: %s", err.Error())), nil
		}

		// Step 2: PATCH the event with new start/end.
		event := models.NewEvent()
		start := models.NewDateTimeTimeZone()
		start.SetDateTime(&newStartDT)
		start.SetTimeZone(&newStartTZ)
		event.SetStart(start)

		end := models.NewDateTimeTimeZone()
		end.SetDateTime(&newEndDT)
		end.SetTimeZone(&newStartTZ)
		event.SetEnd(end)

		patchCtx, patchCancel := graph.WithTimeout(ctx, timeout)
		defer patchCancel()

		var updatedEvent models.Eventable
		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			var graphErr error
			updatedEvent, graphErr = client.Me().Events().ByEventId(eventID).Patch(patchCtx, event, nil)
			return graphErr
		})
		if err != nil {
			if graph.IsTimeoutError(err) {
				logger.ErrorContext(ctx, "PATCH request timed out",
					"timeout_seconds", int(timeout.Seconds()),
					"error", err.Error())
				return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds()))), nil
			}
			logger.ErrorContext(ctx, "reschedule event failed",
				"event_id", eventID,
				"error", graph.FormatGraphError(err))
			return mcp.NewToolResultError(graph.RedactGraphError(err)), nil
		}

		logger.InfoContext(ctx, "event rescheduled",
			"event_id", eventID,
			"new_start", newStartDT,
			"new_end", newEndDT,
		)

		// Extract fields for text confirmation.
		eventSubject := graph.SafeStr(updatedEvent.GetSubject())
		displayTime := formatEventDisplayTime(updatedEvent)
		eventLocation := extractEventLocation(updatedEvent)

		return mcp.NewToolResultText(FormatWriteConfirmation("rescheduled", eventSubject, eventID, displayTime, eventLocation)), nil
	}
}

// computeEventDuration parses two ISO 8601 datetime strings and returns the
// duration between them.
//
// Parameters:
//   - startStr: the event start datetime in ISO 8601 format.
//   - endStr: the event end datetime in ISO 8601 format.
//
// Returns the duration and nil on success, or zero duration and an error if
// either string cannot be parsed.
func computeEventDuration(startStr, endStr string) (time.Duration, error) {
	startTime, err := parseGraphDateTime(startStr)
	if err != nil {
		return 0, fmt.Errorf("parse start: %w", err)
	}
	endTime, err := parseGraphDateTime(endStr)
	if err != nil {
		return 0, fmt.Errorf("parse end: %w", err)
	}
	return endTime.Sub(startTime), nil
}

// addDuration parses an ISO 8601 datetime string, adds the given duration, and
// returns the result formatted in the same layout.
//
// Parameters:
//   - dtStr: the base datetime in ISO 8601 format.
//   - d: the duration to add.
//
// Returns the resulting datetime string and nil on success, or empty string and
// an error if the input cannot be parsed.
func addDuration(dtStr string, d time.Duration) (string, error) {
	t, err := parseGraphDateTime(dtStr)
	if err != nil {
		return "", err
	}
	return t.Add(d).Format("2006-01-02T15:04:05"), nil
}

// graphDateTimeFormats lists the datetime formats returned by the Graph API.
// The Graph API may return fractional seconds with varying precision.
var graphDateTimeFormats = []string{
	"2006-01-02T15:04:05.0000000",
	"2006-01-02T15:04:05",
}

// parseGraphDateTime parses a datetime string from the Graph API, handling both
// formats with and without fractional seconds.
//
// Parameters:
//   - s: the datetime string to parse.
//
// Returns the parsed time.Time and nil on success, or zero time and an error if
// no format matches.
func parseGraphDateTime(s string) (time.Time, error) {
	for _, layout := range graphDateTimeFormats {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse datetime %q", s)
}
