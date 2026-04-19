// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the search_events MCP tool, which searches calendar events
// within a time range using optional OData filters and client-side substring
// matching. The tool builds a composite OData $filter string from the provided
// parameters (importance, sensitivity, is_all_day, show_as, is_cancelled) and
// applies case-insensitive substring matching on the subject field client-side.
// Categories filtering is also performed client-side.
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
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	msgraphcore "github.com/microsoftgraph/msgraph-sdk-go-core"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

// searchEventsSelectFields defines the $select fields for the search_events
// CalendarView endpoint. These match the list_events summary fields.
var searchEventsSelectFields = []string{
	"id", "subject", "start", "end", "location", "organizer",
	"isAllDay", "showAs", "importance", "sensitivity", "isCancelled",
	"categories", "webLink", "isOnlineMeeting", "onlineMeeting",
}

// NewSearchEventsTool creates the MCP tool definition for search_events. All
// parameters are optional. The tool is annotated as read-only since it only
// retrieves data.
//
// When provenanceEnabled is true, an additional optional created_by_mcp boolean
// parameter is registered, allowing callers to filter results to only
// MCP-created events using a server-side OData filter on the provenance
// extended property.
//
// Parameters:
//   - provenanceEnabled: whether provenance tagging is active. When false, the
//     created_by_mcp parameter is not included in the tool definition.
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewSearchEventsTool(provenanceEnabled bool) mcp.Tool {
	opts := []mcp.ToolOption{
		mcp.WithDescription(
			"Search calendar events by subject text, importance, sensitivity, " +
				"and other properties within a time range. All parameters are optional. " +
				"Defaults to searching the next 30 days from now.",
		),
		mcp.WithTitleAnnotation("Search Calendar Events"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithString("query",
			mcp.Description("Text to search for in event subjects (case-insensitive)."),
		),
		mcp.WithString("date",
			mcp.Description("Date shorthand: 'today', 'tomorrow', 'this_week', 'next_week', or ISO 8601 date (YYYY-MM-DD). Expands to start/end boundaries in the configured timezone. When start_datetime/end_datetime are also provided, they take precedence."),
		),
		mcp.WithString("start_datetime",
			mcp.Description("Start of the time range in ISO 8601 format. Defaults to current time."),
		),
		mcp.WithString("end_datetime",
			mcp.Description("End of the time range in ISO 8601 format. Defaults to 30 days from start."),
		),
		mcp.WithString("importance",
			mcp.Description("Filter by importance: low, normal, or high."),
		),
		mcp.WithString("sensitivity",
			mcp.Description("Filter by sensitivity: normal, personal, private, or confidential."),
		),
		mcp.WithBoolean("is_all_day",
			mcp.Description("Filter by all-day event status."),
		),
		mcp.WithString("show_as",
			mcp.Description("Filter by free/busy status: free, tentative, busy, oof, or workingElsewhere."),
		),
		mcp.WithBoolean("is_cancelled",
			mcp.Description("Filter by cancellation status."),
		),
		mcp.WithString("categories",
			mcp.Description("Comma-separated category names to filter by (matches any, client-side)."),
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
			mcp.Description(AccountParamDescription),
		),
		mcp.WithString("output",
			mcp.Description("Output mode: 'text' (default) returns plain-text listing, 'summary' returns compact JSON, 'raw' returns full Graph API fields."),
			mcp.Enum("text", "summary", "raw"),
		),
	}

	if provenanceEnabled {
		opts = append(opts, mcp.WithBoolean("created_by_mcp",
			mcp.Description("When true, only return events created by this MCP server (server-side filter)."),
		))
	}

	return mcp.NewTool("calendar_search_events", opts...)
}

// NewHandleSearchEvents creates a tool handler that searches calendar events
// using the CalendarView endpoint with composite OData filtering and client-side
// fallback logic. The Graph client is retrieved from the request context at
// invocation time.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//   - defaultTimezone: the IANA timezone name used to expand the date convenience
//     parameter to start/end boundaries.
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
//   - Defaults start_datetime to now and end_datetime to 30 days from start when
//     neither date nor explicit datetimes are provided.
//   - Builds a composite OData $filter from importance, sensitivity, is_all_day,
//     show_as, and is_cancelled (subject matching is always client-side).
//   - Applies client-side case-insensitive substring matching on subject when
//     query parameter is provided.
//   - Applies client-side category filtering when categories parameter is provided.
//   - Limits results to max_results.
//   - Serializes events using SerializeEvent and returns a JSON array.
func NewHandleSearchEvents(retryCfg graph.RetryConfig, timeout time.Duration, defaultTimezone, provenancePropertyID string) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := slog.With("tool", "calendar_search_events")
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

		// Extract optional parameters with defaults.
		query := request.GetString("query", "")
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

		importance := request.GetString("importance", "")
		sensitivity := request.GetString("sensitivity", "")
		showAs := request.GetString("show_as", "")
		categories := request.GetString("categories", "")
		timezone := request.GetString("timezone", "")

		// Validate optional parameters.
		if query != "" {
			if err := validate.ValidateStringLength(query, "query", validate.MaxQueryLen); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
		if startDatetime != "" {
			if err := validate.ValidateDatetime(startDatetime, "start_datetime"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
		if endDatetime != "" {
			if err := validate.ValidateDatetime(endDatetime, "end_datetime"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
		if importance != "" {
			if err := validate.ValidateImportance(importance); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
		if sensitivity != "" {
			if err := validate.ValidateSensitivity(sensitivity); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
		if showAs != "" {
			if err := validate.ValidateShowAs(showAs); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
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

		// Extract boolean parameters from arguments map.
		args := request.GetArguments()
		var isAllDay *bool
		if v, ok := args["is_all_day"].(bool); ok {
			isAllDay = &v
		}
		var isCancelled *bool
		if v, ok := args["is_cancelled"].(bool); ok {
			isCancelled = &v
		}
		var createdByMcp bool
		if v, ok := args["created_by_mcp"].(bool); ok {
			createdByMcp = v
		}

		// Default start_datetime to current time.
		if startDatetime == "" {
			startDatetime = time.Now().UTC().Format(time.RFC3339)
		}

		// Default end_datetime to 30 days from start.
		if endDatetime == "" {
			parsed, err := time.Parse(time.RFC3339, startDatetime)
			if err != nil {
				// Fallback: 30 days from now if parsing fails.
				endDatetime = time.Now().UTC().Add(30 * 24 * time.Hour).Format(time.RFC3339)
			} else {
				endDatetime = parsed.Add(30 * 24 * time.Hour).Format(time.RFC3339)
			}
		}

		logger.Debug("tool called",
			"query", query,
			"start_datetime", startDatetime,
			"end_datetime", endDatetime,
			"max_results", maxResults)

		// Build composite OData $filter expressions. Subject matching is always
		// performed client-side via case-insensitive substring, not via OData.
		var filters []string

		if importance != "" {
			filters = append(filters, fmt.Sprintf("importance eq '%s'", graph.EscapeOData(importance)))
		}
		if sensitivity != "" {
			filters = append(filters, fmt.Sprintf("sensitivity eq '%s'", graph.EscapeOData(sensitivity)))
		}
		if isAllDay != nil {
			filters = append(filters, fmt.Sprintf("isAllDay eq %v", *isAllDay))
		}
		if showAs != "" {
			filters = append(filters, fmt.Sprintf("showAs eq '%s'", graph.EscapeOData(showAs)))
		}
		if isCancelled != nil {
			filters = append(filters, fmt.Sprintf("isCancelled eq %v", *isCancelled))
		}
		if createdByMcp && provenancePropertyID != "" {
			filters = append(filters, fmt.Sprintf(
				"singleValueExtendedProperties/Any(ep: ep/id eq '%s' and ep/value eq 'true')",
				provenancePropertyID,
			))
		}

		filterStr := strings.Join(filters, " and ")

		// Execute CalendarView with filter using timeout context.
		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		// Build $expand for provenance extended property when enabled.
		var expandFilter string
		if provenancePropertyID != "" {
			expandFilter = graph.ProvenanceExpandFilter(provenancePropertyID)
		}

		// Always use raw serialization during collection so client-side
		// filtering (categories, subject fallback) has access to all fields.
		serializeFn := func(e models.Eventable) map[string]any {
			return graph.SerializeEvent(e, provenancePropertyID)
		}

		// Build client-side match filter applied during page iteration so
		// that events beyond the first page are scanned when needed.
		clientFilter := buildClientFilter(query, categories)

		events, err := executeSearchCalendarView(timeoutCtx, client, retryCfg, startDatetime, endDatetime, filterStr, timezone, maxResults, expandFilter, serializeFn, clientFilter)
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

		// Convert to summary format if needed. Raw events were collected
		// above to support client-side filtering on fields like categories.
		if outputMode == "summary" || outputMode == "text" {
			for i, e := range events {
				events[i] = graph.ToSummaryEventMap(e)
			}
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

// searchPageSize is the Graph API page size used when client-side filters
// (query, categories) are active. A larger page size reduces API round trips
// when scanning beyond the requested maxResults to find matching events.
const searchPageSize = 50

// executeSearchCalendarView executes a CalendarView request with the specified
// date range, OData filter, timezone, and result limit. It paginates through
// results using the Graph SDK's PageIterator and serializes events via the
// provided serialize function.
//
// When match is non-nil, it is applied during iteration so that only matching
// events count toward maxResults. This allows the iterator to scan beyond the
// first page when the target events are not in the initial results. A larger
// page size (searchPageSize) is used to reduce API round trips during scanning.
//
// Parameters:
//   - ctx: the request context.
//   - graphClient: the authenticated Microsoft Graph client.
//   - retryCfg: retry configuration for transient Graph API errors.
//   - startDatetime: the CalendarView start boundary in ISO 8601 format.
//   - endDatetime: the CalendarView end boundary in ISO 8601 format.
//   - filter: the OData $filter string, or empty for no filter.
//   - timezone: IANA timezone for the Prefer header, or empty for default.
//   - maxResults: maximum number of matching events to collect.
//   - expandFilter: the $expand clause for provenance extended property, or
//     empty when provenance tagging is disabled.
//   - serialize: function to convert an Eventable to a map for serialization.
//   - match: optional client-side filter applied during iteration. When non-nil,
//     only events for which match returns true are collected. When nil, all
//     events are collected up to maxResults.
//
// Returns a slice of serialized event maps, or an error from the Graph API.
//
// Side effects: makes HTTP requests to the Microsoft Graph API.
func executeSearchCalendarView(
	ctx context.Context,
	graphClient *msgraphsdk.GraphServiceClient,
	retryCfg graph.RetryConfig,
	startDatetime, endDatetime, filter, timezone string,
	maxResults int,
	expandFilter string,
	serialize func(models.Eventable) map[string]any,
	match func(map[string]any) bool,
) ([]map[string]any, error) {
	// When client-side filtering is active, use a larger page size to
	// reduce API round trips while scanning for matching events.
	top := int32(maxResults)
	if match != nil && top < searchPageSize {
		top = searchPageSize
	}
	orderby := []string{"start/dateTime"}

	var expandFields []string
	if expandFilter != "" {
		expandFields = []string{expandFilter}
	}

	qp := &users.ItemCalendarViewRequestBuilderGetQueryParameters{
		StartDateTime: &startDatetime,
		EndDateTime:   &endDatetime,
		Select:        searchEventsSelectFields,
		Orderby:       orderby,
		Top:           &top,
		Expand:        expandFields,
	}
	if filter != "" {
		qp.Filter = &filter
	}

	cfg := &users.ItemCalendarViewRequestBuilderGetRequestConfiguration{
		QueryParameters: qp,
	}
	if timezone != "" {
		headers := abstractions.NewRequestHeaders()
		headers.Add("Prefer", fmt.Sprintf("outlook.timezone=\"%s\"", timezone))
		cfg.Headers = headers
	}

	logger := slog.With("tool", "calendar_search_events")
	logger.Debug("graph API request",
		"endpoint", "GET /me/calendarView",
		"start_datetime", startDatetime,
		"end_datetime", endDatetime,
		"filter", filter,
		"top", top)

	var resp models.EventCollectionResponseable
	if graphErr := graph.RetryGraphCall(ctx, retryCfg, func() error {
		var err error
		resp, err = graphClient.Me().CalendarView().Get(ctx, cfg)
		return err
	}); graphErr != nil {
		return nil, graphErr
	}

	logger.Debug("graph API response",
		"endpoint", "GET /me/calendarView",
		"status", "ok")

	events := make([]map[string]any, 0, maxResults)
	pageIterator, err := msgraphcore.NewPageIterator[models.Eventable](
		resp,
		graphClient.GetAdapter(),
		models.CreateEventCollectionResponseFromDiscriminatorValue,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create page iterator: %w", err)
	}

	if timezone != "" {
		headers := abstractions.NewRequestHeaders()
		headers.Add("Prefer", fmt.Sprintf("outlook.timezone=\"%s\"", timezone))
		pageIterator.SetHeaders(headers)
	}

	err = pageIterator.Iterate(ctx, func(event models.Eventable) bool {
		serialized := serialize(event)
		if match != nil && !match(serialized) {
			return true // skip non-matching, continue scanning
		}
		events = append(events, serialized)
		return len(events) < maxResults
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate events: %w", err)
	}

	return events, nil
}

// buildClientFilter constructs a match function for client-side event filtering
// during page iteration. Returns nil when no client-side filters are active,
// signaling that all events should be collected without additional filtering.
//
// Parameters:
//   - query: case-insensitive substring to match against event subjects. Empty
//     string disables subject filtering.
//   - categories: comma-separated category names to match (any). Empty string
//     disables category filtering.
//
// Returns a function that accepts a serialized event map and returns true if
// the event matches all active filters, or nil when no filters are active.
func buildClientFilter(query, categories string) func(map[string]any) bool {
	if query == "" && categories == "" {
		return nil
	}

	var queryLower string
	if query != "" {
		queryLower = strings.ToLower(query)
	}

	var categorySet map[string]bool
	if categories != "" {
		targets := splitCategories(categories)
		categorySet = make(map[string]bool, len(targets))
		for _, t := range targets {
			categorySet[strings.ToLower(t)] = true
		}
	}

	return func(e map[string]any) bool {
		if queryLower != "" {
			subject, _ := e["subject"].(string)
			if !strings.Contains(strings.ToLower(subject), queryLower) {
				return false
			}
		}
		if categorySet != nil {
			cats, _ := e["categories"].([]string)
			matched := false
			for _, c := range cats {
				if categorySet[strings.ToLower(c)] {
					matched = true
					break
				}
			}
			if !matched {
				return false
			}
		}
		return true
	}
}
