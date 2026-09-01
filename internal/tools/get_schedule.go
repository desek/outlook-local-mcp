// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the handler for the calendar get_schedule verb, which reads
// free/busy blocks and working hours for one or more mailboxes the caller is
// permitted to see. It is a pure read: the request body carries the mailbox list
// and the window, and the response is projected through the scheduling
// serializers.
//
// @agents-index: Handler constructor for the calendar.get_schedule verb, posting
// a mailbox list and a resolved window to getSchedule and projecting per-mailbox
// availability, working hours, and errors across the three output tiers.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/logging"
	"github.com/desek/outlook-local-mcp/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

// maxScheduleMailboxes is the number of mailboxes one getSchedule request may
// name. It is the service's own documented ceiling, enforced here so a caller
// naming too many mailboxes is told the limit rather than shown a Graph error
// that does not name it.
const maxScheduleMailboxes = 20

// Availability-view interval bounds, in minutes. The interval decides how
// coarsely the availability string divides the window; the bounds are the
// service's own, and the default keeps a day's view readable.
const (
	defaultAvailabilityViewInterval = 30
	minAvailabilityViewInterval     = 5
	maxAvailabilityViewInterval     = 1440
)

// GetScheduleResponse is the JSON envelope the summary and raw output tiers
// return. The per-mailbox records are carried as already-serialized maps so the
// tier decides the field set, and TimeRange restates the resolved window so a
// caller reading a shorthand-expanded request can see what was actually queried.
type GetScheduleResponse struct {
	// TimeRange holds the resolved start and end of the queried window.
	TimeRange FreeBusyTimeRange `json:"timeRange"`
	// Schedules holds one serialized record per mailbox, in the order the
	// caller named them.
	Schedules []map[string]any `json:"schedules"`
}

// NewHandleGetSchedule creates the handler for the get_schedule verb. It posts
// the caller's mailbox list and resolved window to
// POST /me/calendar/getSchedule and returns each mailbox's free/busy blocks,
// working hours, and, where Graph reported one, that mailbox's error.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//   - defaultTimezone: the IANA timezone used to expand the date shorthand into
//     day boundaries and to stamp the posted window when the caller names no
//     timezone, so the request is never zone-ambiguous.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method.
//
// The handler refuses every invalid input before issuing any Graph request: a
// missing or malformed mailbox list, more mailboxes than the service accepts, a
// window that resolves to nothing, a malformed bound, and an out-of-bound
// availability view interval.
func NewHandleGetSchedule(retryCfg graph.RetryConfig, timeout time.Duration, defaultTimezone string) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)
		start := time.Now()

		client, err := GraphClient(ctx)
		if err != nil {
			return mcp.NewToolResultError("no account selected"), nil
		}

		outputMode, err := ValidateOutputMode(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		mailboxes, err := parseScheduleMailboxes(request.GetString("schedules", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		startDatetime, endDatetime, err := resolveScheduleWindow(request, defaultTimezone)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		timezone := request.GetString("timezone", "")
		if timezone != "" {
			if err := validate.ValidateTimezone(timezone, "timezone"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
		windowTimezone := timezone
		if windowTimezone == "" {
			windowTimezone = defaultTimezone
		}

		interval := int32(request.GetFloat("availability_view_interval", defaultAvailabilityViewInterval))
		if _, supplied := request.GetArguments()["availability_view_interval"]; supplied {
			if interval < minAvailabilityViewInterval || interval > maxAvailabilityViewInterval {
				return mcp.NewToolResultError(fmt.Sprintf("availability_view_interval must be between %d and %d minutes, got %d", minAvailabilityViewInterval, maxAvailabilityViewInterval, interval)), nil
			}
		}

		body := users.NewItemCalendarGetSchedulePostRequestBody()
		body.SetSchedules(mailboxes)
		body.SetStartTime(newDateTimeTimeZone(startDatetime, windowTimezone))
		body.SetEndTime(newDateTimeTimeZone(endDatetime, windowTimezone))
		body.SetAvailabilityViewInterval(&interval)

		logger.Debug("tool called",
			"mailboxes", len(mailboxes),
			"start_datetime", startDatetime,
			"end_datetime", endDatetime,
			"availability_view_interval", interval)

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		logger.Debug("graph API request", "endpoint", "POST /me/calendar/getSchedule")

		var resp users.ItemCalendarGetSchedulePostResponseable
		graphErr := graph.RetryGraphCall(ctx, retryCfg, func() error {
			var callErr error
			resp, callErr = client.Me().Calendar().GetSchedule().PostAsGetSchedulePostResponse(timeoutCtx, body, nil)
			return callErr
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

		logger.Debug("graph API response", "endpoint", "POST /me/calendar/getSchedule", "status", "ok")

		// Graph returns the mailboxes in the order they were requested, and the
		// projection preserves that order rather than sorting or grouping, so
		// identical responses project identically.
		result := GetScheduleResponse{
			TimeRange: FreeBusyTimeRange{Start: startDatetime, End: endDatetime},
			Schedules: []map[string]any{},
		}
		if resp != nil {
			for _, info := range resp.GetValue() {
				// A mailbox Graph could not read carries an error instead of
				// blocks. It is projected like any other mailbox, so the
				// failure is stated for that mailbox and the mailboxes that
				// succeeded keep their blocks.
				if outputMode == "raw" {
					result.Schedules = append(result.Schedules, graph.SerializeScheduleInformation(info))
				} else {
					result.Schedules = append(result.Schedules, graph.SerializeSummaryScheduleInformation(info))
				}
			}
		}

		if outputMode == "text" {
			logger.Info("tool completed",
				"duration", time.Since(start),
				"mailboxes", len(result.Schedules))
			return mcp.NewToolResultText(FormatScheduleText(result)), nil
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
			"mailboxes", len(result.Schedules))
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}

// parseScheduleMailboxes splits the comma-separated mailbox list into validated
// SMTP addresses, preserving the order the caller named them so the projection
// is deterministic.
//
// Parameters:
//   - raw: the schedules parameter as supplied, a comma-separated address list.
//
// Returns the address slice, or an error naming what to supply when the list is
// empty, when an address fails validation, or when it names more mailboxes than
// the service accepts. Each error states the correction.
//
// Side effects: none.
func parseScheduleMailboxes(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("schedules is required: supply one or more mailbox SMTP addresses separated by commas, such as a@example.com,b@example.com")
	}

	var mailboxes []string
	for _, field := range strings.Split(raw, ",") {
		address := strings.TrimSpace(field)
		if address == "" {
			continue
		}
		mailboxes = append(mailboxes, address)
	}
	if len(mailboxes) == 0 {
		return nil, fmt.Errorf("schedules must name at least one mailbox SMTP address")
	}
	if len(mailboxes) > maxScheduleMailboxes {
		return nil, fmt.Errorf("schedules names %d mailboxes, which exceeds the ceiling of %d the service accepts in one request: split the list across several calls", len(mailboxes), maxScheduleMailboxes)
	}
	for _, address := range mailboxes {
		if err := validate.ValidateEmail(address); err != nil {
			return nil, err
		}
	}
	return mailboxes, nil
}

// resolveScheduleWindow resolves the queried window from explicit datetimes or
// the date shorthand, with the explicit values taking precedence over the
// shorthand, and validates both bounds.
//
// Parameters:
//   - request: the tool request carrying start_datetime, end_datetime, and date.
//   - defaultTimezone: the IANA timezone in which the shorthand's day boundaries
//     are computed.
//
// Returns the resolved start and end as ISO 8601 strings, or an error naming the
// parameters to supply when the request resolves to no window, and naming the
// offending parameter when a supplied bound is malformed.
//
// Side effects: none.
func resolveScheduleWindow(request mcp.CallToolRequest, defaultTimezone string) (string, string, error) {
	startDatetime := request.GetString("start_datetime", "")
	endDatetime := request.GetString("end_datetime", "")

	if startDatetime == "" || endDatetime == "" {
		if dateParam := request.GetString("date", ""); dateParam != "" {
			resolvedStart, resolvedEnd, err := expandDateParam(dateParam, defaultTimezone)
			if err != nil {
				return "", "", err
			}
			if startDatetime == "" {
				startDatetime = resolvedStart
			}
			if endDatetime == "" {
				endDatetime = resolvedEnd
			}
		}
	}

	if startDatetime == "" || endDatetime == "" {
		return "", "", fmt.Errorf("a window is required: supply both start_datetime and end_datetime, or the date shorthand ('today', 'tomorrow', 'this_week', 'next_week', or YYYY-MM-DD)")
	}
	if err := validate.ValidateDatetime(startDatetime, "start_datetime"); err != nil {
		return "", "", err
	}
	if err := validate.ValidateDatetime(endDatetime, "end_datetime"); err != nil {
		return "", "", err
	}
	return startDatetime, endDatetime, nil
}

// scheduleMailboxLabel returns the mailbox address a serialized schedule record
// belongs to, falling back to a stated placeholder so a record Graph returned
// without a schedule id is still attributable to a position in the listing.
func scheduleMailboxLabel(record map[string]any) string {
	if id, _ := record["scheduleId"].(string); id != "" {
		return id
	}
	return "(unnamed mailbox)"
}
