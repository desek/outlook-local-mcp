// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the list_events MCP tool, which retrieves calendar events
// within a specified time range using the CalendarView endpoint. The CalendarView
// endpoint expands recurring events into individual occurrences, providing an
// accurate schedule representation. Pagination is handled via the Graph SDK's
// PageIterator with a configurable max_results cap.
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
	abstractions "github.com/microsoft/kiota-abstractions-go"
	msgraphcore "github.com/microsoftgraph/msgraph-sdk-go-core"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

// listEventsSelectFields defines the $select fields for the list_events CalendarView
// endpoint. These fields represent the summary view of an event, excluding heavy
// fields like body and attendees to minimize data transfer.
var listEventsSelectFields = []string{
	"id", "subject", "start", "end", "location", "organizer",
	"isAllDay", "showAs", "importance", "sensitivity", "isCancelled",
	"categories", "webLink", "isOnlineMeeting", "onlineMeeting",
}

// NewListEventsTool creates the MCP tool definition for list_events. The tool
// accepts an optional date convenience parameter that expands to start/end of
// day, plus explicit start_datetime/end_datetime for fine-grained control.
// When date is provided, start_datetime and end_datetime are derived
// automatically. When both are provided, explicit values take precedence.
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewListEventsTool() mcp.Tool {
	return mcp.NewTool("calendar_list_events",
		mcp.WithDescription("List calendar events within a time range. Expands recurring events into individual occurrences."),
		mcp.WithTitleAnnotation("List Calendar Events"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithString("date",
			mcp.Description("Date shorthand: 'today', 'tomorrow', 'this_week', 'next_week', or ISO 8601 date (YYYY-MM-DD). Expands to start/end boundaries in the configured timezone. When start_datetime/end_datetime are also provided, they take precedence."),
		),
		mcp.WithString("start_datetime",
			mcp.Description("Start of the time range in ISO 8601 format (e.g., 2026-03-12T00:00:00Z). Required unless 'date' is provided."),
		),
		mcp.WithString("end_datetime",
			mcp.Description("End of the time range in ISO 8601 format (e.g., 2026-03-13T00:00:00Z). Required unless 'date' is provided."),
		),
		mcp.WithString("calendar_id",
			mcp.Description("Optional calendar ID. If omitted, uses the default calendar."),
		),
		mcp.WithNumber("max_results",
			mcp.Description("Maximum number of events to return (default 25, max 100)."),
			mcp.Min(1),
			mcp.Max(100),
		),
		mcp.WithString("timezone",
			mcp.Description("IANA timezone name for returned event times (e.g., America/New_York)."),
		),
		mcp.WithString("account",
			mcp.Description("Account label to use. If omitted, the default account is used. Use account_list to see available accounts."),
		),
		mcp.WithString("output",
			mcp.Description("Output mode: 'text' (default) returns plain-text listing, 'summary' returns compact JSON, 'raw' returns full Graph API fields."),
			mcp.Enum("text", "summary", "raw"),
		),
	)
}

// NewHandleListEvents creates a tool handler that lists calendar events within
// a time range by calling the CalendarView endpoint via the Graph SDK. The
// Graph client is retrieved from the request context at invocation time.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//   - defaultTimezone: the IANA timezone name used to expand the date convenience
//     parameter to start/end-of-day boundaries.
//   - provenancePropertyID: the full provenance property ID string, built once at
//     startup. When non-empty, $expand is added to request the provenance extended
//     property, and serialized events include "createdByMcp" when tagged.
//
// Returns a tool handler function compatible with the MCP server's AddTool method.
//
// The handler:
//   - Retrieves the Graph client from context via GraphClient.
//   - Resolves the date convenience parameter to start_datetime/end_datetime when
//     explicit values are not provided.
//   - Extracts and validates required parameters (start_datetime, end_datetime).
//   - Routes to /me/calendarView or /me/calendars/{id}/calendarView based on calendar_id.
//   - Applies $select, $orderby, $top, and $expand query parameters.
//   - Sets the Prefer header for timezone when provided.
//   - Uses PageIterator for pagination with a max_results cap.
//   - Serializes events using SerializeEvent from the graph package.
//   - Returns Graph API errors via mcp.NewToolResultError with FormatGraphError.
//   - Logs entry at debug level, completion at info level, errors at error level.
func NewHandleListEvents(retryCfg graph.RetryConfig, timeout time.Duration, defaultTimezone, provenancePropertyID string) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := slog.With("tool", "calendar_list_events")
		start := time.Now()

		client, err := GraphClient(ctx)
		if err != nil {
			return mcp.NewToolResultError("no account selected"), nil
		}

		// Resolve date convenience parameter to start/end datetimes.
		startDatetime := request.GetString("start_datetime", "")
		endDatetime := request.GetString("end_datetime", "")

		if startDatetime == "" || endDatetime == "" {
			dateParam := request.GetString("date", "")
			if dateParam != "" {
				resolvedStart, resolvedEnd, dateErr := expandDateParam(dateParam, defaultTimezone)
				if dateErr != nil {
					return mcp.NewToolResultError(dateErr.Error()), nil
				}
				if startDatetime == "" {
					startDatetime = resolvedStart
				}
				if endDatetime == "" {
					endDatetime = resolvedEnd
				}
			}
		}

		// Validate that we have both start and end datetimes.
		if startDatetime == "" {
			return mcp.NewToolResultError("start_datetime is required (or provide 'date' parameter)"), nil
		}
		if err := validate.ValidateDatetime(startDatetime, "start_datetime"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if endDatetime == "" {
			return mcp.NewToolResultError("end_datetime is required (or provide 'date' parameter)"), nil
		}
		if err := validate.ValidateDatetime(endDatetime, "end_datetime"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Validate output mode.
		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Extract optional parameters.
		calendarID := request.GetString("calendar_id", "")
		if calendarID != "" {
			if err := validate.ValidateResourceID(calendarID, "calendar_id"); err != nil {
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
		timezone := request.GetString("timezone", "")
		if timezone != "" {
			if err := validate.ValidateTimezone(timezone, "timezone"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}

		logger.Debug("tool called",
			"start_datetime", startDatetime,
			"end_datetime", endDatetime,
			"calendar_id", calendarID,
			"max_results", maxResults,
			"timezone", timezone)

		// Build the query and make the Graph API call.
		top := int32(maxResults)
		orderby := []string{"start/dateTime"}

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		var resp models.EventCollectionResponseable
		var graphErr error
		// Build $expand for provenance extended property when enabled.
		var expandFields []string
		if provenancePropertyID != "" {
			expandFields = []string{graph.ProvenanceExpandFilter(provenancePropertyID)}
		}

		if calendarID != "" {
			// Route to specific calendar's CalendarView.
			qp := &users.ItemCalendarsItemCalendarViewRequestBuilderGetQueryParameters{
				StartDateTime: &startDatetime,
				EndDateTime:   &endDatetime,
				Select:        listEventsSelectFields,
				Orderby:       orderby,
				Top:           &top,
				Expand:        expandFields,
			}
			cfg := &users.ItemCalendarsItemCalendarViewRequestBuilderGetRequestConfiguration{
				QueryParameters: qp,
			}
			if timezone != "" {
				headers := abstractions.NewRequestHeaders()
				headers.Add("Prefer", fmt.Sprintf("outlook.timezone=\"%s\"", timezone))
				cfg.Headers = headers
			}
			logger.Debug("graph API request",
				"endpoint", "GET /me/calendars/{id}/calendarView",
				"calendar_id", calendarID,
				"start_datetime", startDatetime,
				"end_datetime", endDatetime,
				"top", top)
			graphErr = graph.RetryGraphCall(ctx, retryCfg, func() error {
				var err error
				resp, err = client.Me().Calendars().ByCalendarId(calendarID).CalendarView().Get(timeoutCtx, cfg)
				return err
			})
		} else {
			// Route to default CalendarView.
			qp := &users.ItemCalendarViewRequestBuilderGetQueryParameters{
				StartDateTime: &startDatetime,
				EndDateTime:   &endDatetime,
				Select:        listEventsSelectFields,
				Orderby:       orderby,
				Top:           &top,
				Expand:        expandFields,
			}
			cfg := &users.ItemCalendarViewRequestBuilderGetRequestConfiguration{
				QueryParameters: qp,
			}
			if timezone != "" {
				headers := abstractions.NewRequestHeaders()
				headers.Add("Prefer", fmt.Sprintf("outlook.timezone=\"%s\"", timezone))
				cfg.Headers = headers
			}
			logger.Debug("graph API request",
				"endpoint", "GET /me/calendarView",
				"start_datetime", startDatetime,
				"end_datetime", endDatetime,
				"top", top)
			graphErr = graph.RetryGraphCall(ctx, retryCfg, func() error {
				var err error
				resp, err = client.Me().CalendarView().Get(timeoutCtx, cfg)
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
			"endpoint", "GET /me/calendarView",
			"status", "ok")

		// Paginate through results using PageIterator with max_results cap.
		events := make([]map[string]any, 0, maxResults)
		pageIterator, err := msgraphcore.NewPageIterator[models.Eventable](
			resp,
			client.GetAdapter(),
			models.CreateEventCollectionResponseFromDiscriminatorValue,
		)
		if err != nil {
			logger.Error("page iterator creation failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to create page iterator: %s", err.Error())), nil
		}

		// Set Prefer header on subsequent page requests when timezone is specified.
		if timezone != "" {
			headers := abstractions.NewRequestHeaders()
			headers.Add("Prefer", fmt.Sprintf("outlook.timezone=\"%s\"", timezone))
			pageIterator.SetHeaders(headers)
		}

		err = pageIterator.Iterate(ctx, func(event models.Eventable) bool {
			if outputMode == "raw" {
				events = append(events, graph.SerializeEvent(event, provenancePropertyID))
			} else {
				events = append(events, graph.SerializeSummaryEvent(event, provenancePropertyID))
			}
			// Stop iteration once max_results events are collected.
			return len(events) < maxResults
		})
		if err != nil {
			logger.Error("pagination failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to iterate events: %s", err.Error())), nil
		}

		// Return text output when requested.
		if outputMode == "text" {
			logger.Info("tool completed",
				"duration", time.Since(start),
				"count", len(events))
			return mcp.NewToolResultText(FormatEventsText(events)), nil
		}

		jsonBytes, err := json.Marshal(events)
		if err != nil {
			logger.Error("json serialization failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize events: %s", err.Error())), nil
		}

		logger.Info("tool completed",
			"duration", time.Since(start),
			"count", len(events))
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}

// expandDateParam resolves the date convenience parameter to start and end
// datetime strings in the given IANA timezone. End boundaries use exclusive
// midnight-of-next-period semantics to align with Graph CalendarView's
// exclusive endDateTime and avoid DST bugs. The dateParam accepts:
//   - "today": resolves to start-of-day through midnight of next day.
//   - "tomorrow": resolves to tomorrow's start-of-day through midnight of the
//     day after.
//   - "this_week": resolves to Monday 00:00:00 through the following Monday
//     00:00:00 of the current ISO week.
//   - "next_week": resolves to Monday 00:00:00 through the Monday after
//     (00:00:00) of the following ISO week.
//   - An ISO 8601 date string (e.g., "2026-03-17"): resolves to that date's
//     start-of-day through midnight of the next day.
//
// Parameters:
//   - dateParam: the date value from the request.
//   - tz: the IANA timezone name for computing day boundaries.
//
// Returns the start and end as ISO 8601 datetime strings, or an error if the
// date or timezone is invalid.
func expandDateParam(dateParam, tz string) (string, string, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return "", "", fmt.Errorf("invalid timezone %q for date expansion", tz)
	}

	now := time.Now().In(loc)

	switch dateParam {
	case "today":
		date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		return formatDayRange(date)

	case "tomorrow":
		tomorrow := now.AddDate(0, 0, 1)
		date := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, loc)
		return formatDayRange(date)

	case "this_week":
		return formatWeekRange(now, loc, 0)

	case "next_week":
		return formatWeekRange(now, loc, 1)

	default:
		parsed, err := time.Parse("2006-01-02", dateParam)
		if err != nil {
			return "", "", fmt.Errorf("date must be 'today', 'tomorrow', 'this_week', 'next_week', or ISO 8601 date (YYYY-MM-DD), got %q", dateParam)
		}
		date := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, loc)
		return formatDayRange(date)
	}
}

// formatDayRange returns the start-of-day and start-of-next-day ISO 8601
// strings for the given time. The end boundary is midnight of the following
// day (exclusive), which avoids DST bugs from adding absolute durations and
// aligns with the Graph CalendarView exclusive endDateTime semantics.
func formatDayRange(date time.Time) (string, string, error) {
	startOfDay := date.Format("2006-01-02T15:04:05")
	nextDay := time.Date(date.Year(), date.Month(), date.Day()+1, 0, 0, 0, 0, date.Location())
	return startOfDay, nextDay.Format("2006-01-02T15:04:05"), nil
}

// formatWeekRange returns the Monday 00:00:00 through the following Monday
// 00:00:00 (exclusive) ISO 8601 strings for the ISO week containing the given
// time, offset by weekOffset weeks (0 = current week, 1 = next week). The end
// boundary is midnight of the Monday after the target week, which aligns with
// the Graph CalendarView exclusive endDateTime semantics and avoids DST issues.
func formatWeekRange(now time.Time, loc *time.Location, weekOffset int) (string, string, error) {
	// Compute days since Monday (ISO: Monday=1, Sunday=7).
	weekday := now.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	daysSinceMonday := int(weekday) - 1

	monday := now.AddDate(0, 0, -daysSinceMonday+weekOffset*7)
	monday = time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, loc)

	nextMonday := monday.AddDate(0, 0, 7)
	nextMonday = time.Date(nextMonday.Year(), nextMonday.Month(), nextMonday.Day(), 0, 0, 0, 0, loc)

	return monday.Format("2006-01-02T15:04:05"), nextMonday.Format("2006-01-02T15:04:05"), nil
}
