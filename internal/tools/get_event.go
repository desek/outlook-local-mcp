// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the get_event MCP tool, which retrieves full details of a
// single calendar event by its ID via the Microsoft Graph API. The response
// includes all event fields including body content, attendees with response
// status, recurrence patterns, and metadata timestamps.
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
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
)

// getEventSelectFields defines the $select fields for the get_event endpoint.
// This is the comprehensive field set including body, attendees, recurrence,
// and metadata fields not included in the list_events summary view.
var getEventSelectFields = []string{
	"id", "subject", "body", "bodyPreview", "start", "end",
	"location", "locations", "organizer", "attendees",
	"isAllDay", "showAs", "importance", "sensitivity", "isCancelled",
	"recurrence", "categories", "webLink",
	"isOnlineMeeting", "onlineMeeting",
	"responseStatus", "seriesMasterId", "type",
	"hasAttachments", "createdDateTime", "lastModifiedDateTime",
}

// NewGetEventTool creates the MCP tool definition for get_event. The tool
// accepts one required parameter (event_id) and two optional parameters
// (timezone, account). It is annotated as read-only since it only retrieves data.
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewGetEventTool() mcp.Tool {
	return mcp.NewTool("calendar_get_event",
		mcp.WithDescription("Get full details of a single calendar event by its ID. Default output includes bodyPreview (plain-text snippet); full HTML body is only available via output=raw."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithString("event_id",
			mcp.Required(),
			mcp.Description("The unique identifier of the event to retrieve."),
		),
		mcp.WithString("timezone",
			mcp.Description("IANA timezone name for returned event times (e.g., America/New_York)."),
		),
		mcp.WithString("account",
			mcp.Description("Account label to use. If omitted, the default account is used. Use account_list to see available accounts."),
		),
		mcp.WithString("output",
			mcp.Description("Output mode: 'text' (default) shows body preview in plain text, 'summary' returns compact JSON with bodyPreview field, 'raw' returns full Graph API fields including full body with HTML content."),
			mcp.Enum("text", "summary", "raw"),
		),
	)
}

// NewHandleGetEvent creates a tool handler that retrieves full event details
// by ID by calling GET /me/events/{id} via the Graph SDK. The Graph client
// is retrieved from the request context at invocation time.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//   - provenancePropertyID: the full provenance property ID string, built once at
//     startup. When non-empty, $expand is added to request the provenance extended
//     property, and the serialized event includes "createdByMcp" when tagged.
//
// Returns a tool handler function compatible with the MCP server's AddTool method.
//
// The handler:
//   - Retrieves the Graph client from context via GraphClient.
//   - Extracts and validates the required event_id parameter.
//   - Wraps the Graph API call with RetryGraphCall for transient error handling.
//   - Applies a comprehensive $select and optional $expand covering all event fields.
//   - Sets the Prefer header for timezone when provided.
//   - Serializes the event including body, attendees, recurrence, and metadata.
//   - Returns Graph API errors via mcp.NewToolResultError with FormatGraphError.
//   - Logs entry at debug level, completion at info level, errors at error level.
func NewHandleGetEvent(retryCfg graph.RetryConfig, timeout time.Duration, provenancePropertyID string) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := slog.With("tool", "calendar_get_event")
		start := time.Now()

		client, err := GraphClient(ctx)
		if err != nil {
			return mcp.NewToolResultError("no account selected"), nil
		}

		// Extract required parameter.
		eventID, err := request.RequireString("event_id")
		if err != nil || eventID == "" {
			return mcp.NewToolResultError("missing required parameter: event_id. Tip: Use calendar_list_events or calendar_search_events to find the event ID."), nil
		}
		if err := validate.ValidateResourceID(eventID, "event_id"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Validate output mode.
		outputMode, err := ValidateOutputMode(request)
		if err != nil {
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
			"event_id", eventID,
			"timezone", timezone)

		// Build request configuration.
		var expandFields []string
		if provenancePropertyID != "" {
			expandFields = []string{graph.ProvenanceExpandFilter(provenancePropertyID)}
		}
		cfg := &users.ItemEventsEventItemRequestBuilderGetRequestConfiguration{
			QueryParameters: &users.ItemEventsEventItemRequestBuilderGetQueryParameters{
				Select: getEventSelectFields,
				Expand: expandFields,
			},
		}
		if timezone != "" {
			headers := abstractions.NewRequestHeaders()
			headers.Add("Prefer", fmt.Sprintf("outlook.timezone=\"%s\"", timezone))
			cfg.Headers = headers
		}

		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		logger.Debug("graph API request",
			"endpoint", "GET /me/events/{id}",
			"event_id", eventID)

		var event models.Eventable
		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			var graphErr error
			event, graphErr = client.Me().Events().ByEventId(eventID).Get(timeoutCtx, cfg)
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
				"event_id", eventID,
				"duration", time.Since(start))
			return mcp.NewToolResultError(graph.RedactGraphError(err)), nil
		}

		logger.Debug("graph API response",
			"endpoint", "GET /me/events/{id}",
			"event_id", eventID,
			"subject", graph.SafeStr(event.GetSubject()))

		var result map[string]any
		if outputMode == "summary" || outputMode == "text" {
			result = graph.SerializeSummaryGetEvent(event, provenancePropertyID)
		} else {
			// Start from the base serialization and add full-detail fields.
			result = graph.SerializeEvent(event, provenancePropertyID)

			// Body: nested ItemBodyable with contentType and content.
			if body := event.GetBody(); body != nil {
				bodyMap := map[string]string{
					"content": graph.SafeStr(body.GetContent()),
				}
				if ct := body.GetContentType(); ct != nil {
					bodyMap["contentType"] = ct.String()
				} else {
					bodyMap["contentType"] = ""
				}
				result["body"] = bodyMap
			}

			// BodyPreview: plain text preview of the body.
			result["bodyPreview"] = graph.SafeStr(event.GetBodyPreview())

			// Locations: array of location objects.
			if locs := event.GetLocations(); locs != nil {
				locList := make([]map[string]string, 0, len(locs))
				for _, loc := range locs {
					locList = append(locList, map[string]string{
						"displayName": graph.SafeStr(loc.GetDisplayName()),
					})
				}
				result["locations"] = locList
			} else {
				result["locations"] = []map[string]string{}
			}

			// Attendees: array with nil-safe extraction of email, type, and response.
			if attendees := event.GetAttendees(); attendees != nil {
				attList := make([]map[string]string, 0, len(attendees))
				for _, att := range attendees {
					attMap := map[string]string{
						"name":     "",
						"email":    "",
						"type":     "",
						"response": "",
					}
					if ea := att.GetEmailAddress(); ea != nil {
						attMap["name"] = graph.SafeStr(ea.GetName())
						attMap["email"] = graph.SafeStr(ea.GetAddress())
					}
					if t := att.GetTypeEscaped(); t != nil {
						attMap["type"] = t.String()
					}
					if status := att.GetStatus(); status != nil {
						if resp := status.GetResponse(); resp != nil {
							attMap["response"] = resp.String()
						}
					}
					attList = append(attList, attMap)
				}
				result["attendees"] = attList
			} else {
				result["attendees"] = []map[string]string{}
			}

			// Recurrence: nested PatternedRecurrenceable.
			if rec := event.GetRecurrence(); rec != nil {
				recMap := map[string]any{}
				if pattern := rec.GetPattern(); pattern != nil {
					patternMap := map[string]any{}
					if t := pattern.GetTypeEscaped(); t != nil {
						patternMap["type"] = t.String()
					}
					if interval := pattern.GetInterval(); interval != nil {
						patternMap["interval"] = *interval
					}
					if daysOfWeek := pattern.GetDaysOfWeek(); daysOfWeek != nil {
						days := make([]string, 0, len(daysOfWeek))
						for _, d := range daysOfWeek {
							days = append(days, d.String())
						}
						patternMap["daysOfWeek"] = days
					}
					if dom := pattern.GetDayOfMonth(); dom != nil {
						patternMap["dayOfMonth"] = *dom
					}
					recMap["pattern"] = patternMap
				}
				if rng := rec.GetRangeEscaped(); rng != nil {
					rangeMap := map[string]string{}
					if t := rng.GetTypeEscaped(); t != nil {
						rangeMap["type"] = t.String()
					}
					if sd := rng.GetStartDate(); sd != nil {
						rangeMap["startDate"] = sd.String()
					} else {
						rangeMap["startDate"] = ""
					}
					if ed := rng.GetEndDate(); ed != nil {
						rangeMap["endDate"] = ed.String()
					} else {
						rangeMap["endDate"] = ""
					}
					recMap["range"] = rangeMap
				}
				result["recurrence"] = recMap
			}

			// ResponseStatus: the user's response to the event.
			if rs := event.GetResponseStatus(); rs != nil {
				rsMap := map[string]string{}
				if resp := rs.GetResponse(); resp != nil {
					rsMap["response"] = resp.String()
				} else {
					rsMap["response"] = ""
				}
				if t := rs.GetTime(); t != nil {
					rsMap["time"] = t.Format(time.RFC3339)
				}
				result["responseStatus"] = rsMap
			}

			// SeriesMasterId: ID of the series master for recurring event occurrences.
			result["seriesMasterId"] = graph.SafeStr(event.GetSeriesMasterId())

			// Type: event type enum (singleInstance, occurrence, exception, seriesMaster).
			if et := event.GetTypeEscaped(); et != nil {
				result["type"] = et.String()
			} else {
				result["type"] = ""
			}

			// HasAttachments: boolean indicating if the event has attachments.
			result["hasAttachments"] = graph.SafeBool(event.GetHasAttachments())

			// CreatedDateTime and LastModifiedDateTime: ISO 8601 timestamps.
			if created := event.GetCreatedDateTime(); created != nil {
				result["createdDateTime"] = created.Format(time.RFC3339)
			} else {
				result["createdDateTime"] = ""
			}
			if modified := event.GetLastModifiedDateTime(); modified != nil {
				result["lastModifiedDateTime"] = modified.Format(time.RFC3339)
			} else {
				result["lastModifiedDateTime"] = ""
			}
		}

		// Return text output when requested.
		if outputMode == "text" {
			logger.Info("tool completed",
				"duration", time.Since(start),
				"event_id", eventID)
			return mcp.NewToolResultText(FormatEventDetailText(result)), nil
		}

		jsonBytes, err := json.Marshal(result)
		if err != nil {
			logger.Error("json serialization failed",
				"error", err.Error(),
				"duration", time.Since(start))
			return mcp.NewToolResultError(fmt.Sprintf("failed to serialize event: %s", err.Error())), nil
		}

		logger.Info("tool completed",
			"duration", time.Since(start),
			"event_id", eventID)
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}
