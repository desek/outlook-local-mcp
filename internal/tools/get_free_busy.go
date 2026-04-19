// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the get_free_busy MCP tool, which queries the CalendarView
// endpoint to determine when a user is busy within a specified time range. Events
// where showAs equals "free" are filtered out, and the remaining events are
// returned as compact busy periods with start, end, status, and subject fields.
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

// freeBusySelectFields defines the $select fields for the get_free_busy
// CalendarView request. Only the fields needed for busy period extraction
// are requested to minimize data transfer.
var freeBusySelectFields = []string{
	"id", "subject", "start", "end", "showAs",
}

// NewGetFreeBusyTool creates the MCP tool definition for get_free_busy. It
// accepts start_datetime and end_datetime for fine-grained control, or a date
// convenience parameter that expands to start/end boundaries. When date is
// provided, start_datetime and end_datetime become optional. The tool is
// annotated as read-only since it only retrieves data.
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewGetFreeBusyTool() mcp.Tool {
	return mcp.NewTool("calendar_get_free_busy",
		mcp.WithDescription(
			"Get free/busy availability for a time range. Returns busy periods "+
				"(events where showAs is not 'free') with start, end, status, and subject. "+
				"Provide either 'date' for day/week shorthand, or explicit start/end datetimes.",
		),
		mcp.WithTitleAnnotation("Get Free/Busy Schedule"),
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
		mcp.WithString("timezone",
			mcp.Description("IANA timezone name for returned event times (e.g., America/New_York)."),
		),
		mcp.WithString("account",
			mcp.Description(AccountParamDescription),
		),
		mcp.WithString("output",
			mcp.Description("Output mode: 'text' (default) returns plain-text listing, 'summary' returns compact JSON, 'raw' returns full Graph API fields."),
			mcp.Enum("text", "summary", "raw"),
		),
	)
}

// FreeBusyResponse represents the JSON response structure for the get_free_busy
// tool. It contains the queried time range and an array of busy periods.
type FreeBusyResponse struct {
	// TimeRange contains the queried start and end times.
	TimeRange FreeBusyTimeRange `json:"timeRange"`
	// BusyPeriods contains the list of events where showAs is not "free".
	BusyPeriods []BusyPeriod `json:"busyPeriods"`
}

// FreeBusyTimeRange represents the queried time range in the free/busy response.
type FreeBusyTimeRange struct {
	// Start is the start of the queried time range in ISO 8601 format.
	Start string `json:"start"`
	// End is the end of the queried time range in ISO 8601 format.
	End string `json:"end"`
}

// BusyPeriod represents a single busy period in the free/busy response.
type BusyPeriod struct {
	// Start is the event start time in ISO 8601 format.
	Start string `json:"start"`
	// End is the event end time in ISO 8601 format.
	End string `json:"end"`
	// Status is the showAs value for the event (e.g., "busy", "tentative", "oof").
	Status string `json:"status"`
	// Subject is the event subject line.
	Subject string `json:"subject"`
}

// NewHandleGetFreeBusy creates a tool handler that retrieves busy periods
// within a time range by querying CalendarView and filtering out events with
// showAs equal to "free". The Graph client is retrieved from the request
// context at invocation time.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//   - defaultTimezone: the IANA timezone name used to expand the date convenience
//     parameter to start/end boundaries.
//
// Returns a tool handler function compatible with the MCP server's AddTool method.
//
// The handler:
//   - Retrieves the Graph client from context via GraphClient.
//   - Resolves the date convenience parameter to start_datetime/end_datetime when
//     explicit values are not provided.
//   - Validates that start_datetime and end_datetime are present (either explicit
//     or expanded from date).
//   - Queries CalendarView with pagination.
//   - Filters out events where showAs equals "free".
//   - Returns a JSON object with timeRange and busyPeriods fields.
func NewHandleGetFreeBusy(retryCfg graph.RetryConfig, timeout time.Duration, defaultTimezone string) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := slog.With("tool", "calendar_get_free_busy")
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

		// Extract datetime parameters with date convenience fallback.
		startDatetime := request.GetString("start_datetime", "")
		endDatetime := request.GetString("end_datetime", "")

		// Resolve date convenience parameter to start/end datetimes.
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

		// Extract optional parameter.
		timezone := request.GetString("timezone", "")
		if timezone != "" {
			if err := validate.ValidateTimezone(timezone, "timezone"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}

		logger.Debug("tool called",
			"start_datetime", startDatetime,
			"end_datetime", endDatetime,
			"timezone", timezone)

		// Execute CalendarView request.
		top := int32(100)
		orderby := []string{"start/dateTime"}

		qp := &users.ItemCalendarViewRequestBuilderGetQueryParameters{
			StartDateTime: &startDatetime,
			EndDateTime:   &endDatetime,
			Select:        freeBusySelectFields,
			Orderby:       orderby,
			Top:           &top,
		}
		cfg := &users.ItemCalendarViewRequestBuilderGetRequestConfiguration{
			QueryParameters: qp,
		}
		if timezone != "" {
			headers := abstractions.NewRequestHeaders()
			headers.Add("Prefer", fmt.Sprintf("outlook.timezone=\"%s\"", timezone))
			cfg.Headers = headers
		}

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		logger.Debug("graph API request",
			"endpoint", "GET /me/calendarView",
			"start_datetime", startDatetime,
			"end_datetime", endDatetime)

		var resp models.EventCollectionResponseable
		graphErr := graph.RetryGraphCall(ctx, retryCfg, func() error {
			var err error
			resp, err = client.Me().CalendarView().Get(timeoutCtx, cfg)
			return err
		})
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

		// Paginate through all results.
		var busyPeriods []BusyPeriod
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

		if timezone != "" {
			headers := abstractions.NewRequestHeaders()
			headers.Add("Prefer", fmt.Sprintf("outlook.timezone=\"%s\"", timezone))
			pageIterator.SetHeaders(headers)
		}

		err = pageIterator.Iterate(ctx, func(event models.Eventable) bool {
			// Filter out events where showAs equals "free".
			if sa := event.GetShowAs(); sa != nil {
				if *sa == models.FREE_FREEBUSYSTATUS {
					return true // Skip free events, continue iteration.
				}
				bp := BusyPeriod{
					Subject: graph.SafeStr(event.GetSubject()),
					Status:  sa.String(),
				}
				if s := event.GetStart(); s != nil {
					bp.Start = graph.SafeStr(s.GetDateTime())
				}
				if e := event.GetEnd(); e != nil {
					bp.End = graph.SafeStr(e.GetDateTime())
				}
				busyPeriods = append(busyPeriods, bp)
			}
			return true
		})
		if err != nil {
			logger.Error("pagination failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to iterate events: %s", err.Error())), nil
		}

		// Ensure busyPeriods is never nil for consistent JSON output.
		if busyPeriods == nil {
			busyPeriods = []BusyPeriod{}
		}

		result := FreeBusyResponse{
			TimeRange: FreeBusyTimeRange{
				Start: startDatetime,
				End:   endDatetime,
			},
			BusyPeriods: busyPeriods,
		}

		// Return text output when requested.
		if outputMode == "text" {
			logger.Info("tool completed",
				"duration", time.Since(start),
				"busy_periods", len(busyPeriods))
			return mcp.NewToolResultText(FormatFreeBusyText(result)), nil
		}

		jsonBytes, err := json.Marshal(result)
		if err != nil {
			logger.Error("json serialization failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize response: %s", err.Error())), nil
		}

		logger.Info("tool completed",
			"duration", time.Since(start),
			"busy_periods", len(busyPeriods))
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}
