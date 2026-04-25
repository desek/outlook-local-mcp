// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the update_event MCP tool, which modifies an existing
// personal calendar event (without attendee changes) via PATCH /me/events/{id}
// on the Microsoft Graph API. Only fields explicitly provided in the request are
// set on the request body, preserving PATCH semantics where omitted fields
// retain their current values. Supports converting events to/from Teams online
// meetings via is_online_meeting. To update attendees on an event, use
// calendar_update_meeting instead.
package tools

import (
	"context"
	"strings"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/logging"
	"github.com/desek/outlook-local-mcp/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// NewUpdateEventTool creates the MCP tool definition for update_event. The tool
// accepts a required event_id and optional fields for mutable event properties,
// including Teams online meeting toggling. Only specified fields are changed
// (PATCH semantics).
//
// This tool does not accept attendees. To update attendees on an event,
// use calendar_update_meeting instead.
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewUpdateEventTool() mcp.Tool {
	return mcp.NewTool("calendar_update_event",
		mcp.WithTitleAnnotation("Update Calendar Event"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithDescription(
			"Update an existing calendar event. Only specified fields are changed "+
				"(PATCH semantics).\n\n"+
				"To update attendees on an event, use calendar_update_meeting instead.",
		),
		mcp.WithString("event_id", mcp.Required(),
			mcp.Description("The unique ID of the event to update"),
		),
		mcp.WithString("subject",
			mcp.Description("New event title"),
		),
		mcp.WithString("start_datetime",
			mcp.Description("New start time in ISO 8601 without offset"),
		),
		mcp.WithString("start_timezone",
			mcp.Description("IANA timezone for new start time"),
		),
		mcp.WithString("end_datetime",
			mcp.Description("New end time in ISO 8601 without offset"),
		),
		mcp.WithString("end_timezone",
			mcp.Description("IANA timezone for new end time"),
		),
		mcp.WithString("body",
			mcp.Description("New event body (HTML or plain text)."),
		),
		mcp.WithString("location",
			mcp.Description("New location display name (e.g. room name, office, or \"Microsoft Teams\")."),
		),
		mcp.WithBoolean("is_online_meeting",
			mcp.Description("Set true to make this a Teams online meeting, or false to remove it (work/school accounts only)"),
		),
		mcp.WithBoolean("is_all_day",
			mcp.Description("Change all-day status"),
		),
		mcp.WithString("importance",
			mcp.Description("New importance: low, normal, or high"),
		),
		mcp.WithString("sensitivity",
			mcp.Description("New sensitivity: normal, personal, private, or confidential"),
		),
		mcp.WithString("show_as",
			mcp.Description("New free/busy status: free, tentative, busy, oof, or workingElsewhere"),
		),
		mcp.WithString("categories",
			mcp.Description("New comma-separated category names (replaces all)"),
		),
		mcp.WithString("recurrence",
			mcp.Description(`New recurrence JSON object, or "null" to remove. Only for series masters.`),
		),
		mcp.WithNumber("reminder_minutes",
			mcp.Description("New reminder minutes before start"),
		),
		mcp.WithBoolean("is_reminder_on",
			mcp.Description("Enable or disable the reminder"),
		),
		mcp.WithString("account",
			mcp.Description(AccountParamDescription),
		),
	)
}

// HandleUpdateEvent is the MCP tool handler for update_event. It extracts the
// event_id and only the provided optional fields, constructs a models.Event
// with PATCH semantics, calls PATCH /me/events/{id}, and returns a concise
// text confirmation. The Graph client is retrieved from the request context at
// invocation time.
//
// When start_datetime or end_datetime is provided without a corresponding
// timezone, the handler defaults the timezone to the server's configured
// default timezone rather than returning an error.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//   - defaultTimezone: the IANA timezone from server config, used as the default
//     when start_timezone or end_timezone is omitted alongside a datetime update.
//
// Returns a closure matching the MCP tool handler function signature.
//
// Side effects: calls PATCH /me/events/{id} on the Microsoft Graph API. Logs at
// debug level on entry, error level on failure, and info level on success.
func HandleUpdateEvent(retryCfg graph.RetryConfig, timeout time.Duration, defaultTimezone string) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)
		args := request.GetArguments()
		logger.DebugContext(ctx, "tool called", "params", args)

		client, err := GraphClient(ctx)
		if err != nil {
			return mcp.NewToolResultError("no account selected"), nil
		}

		// Extract required event_id.
		eventID, err := request.RequireString("event_id")
		if err != nil {
			return mcp.NewToolResultError("missing required parameter: event_id. Tip: Use calendar_list_events or calendar_search_events to find the event ID."), nil
		}
		if err := validate.ValidateResourceID(eventID, "event_id"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Validate optional parameters before processing.
		if subject, ok := args["subject"].(string); ok {
			if err := validate.ValidateStringLength(subject, "subject", validate.MaxSubjectLen); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
		if startDT, ok := args["start_datetime"].(string); ok {
			if err := validate.ValidateDatetime(startDT, "start_datetime"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if startTZ, tzOK := args["start_timezone"].(string); tzOK && startTZ != "" {
				if err := validate.ValidateTimezone(startTZ, "start_timezone"); err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
			}
		}
		if endDT, ok := args["end_datetime"].(string); ok {
			if err := validate.ValidateDatetime(endDT, "end_datetime"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if endTZ, tzOK := args["end_timezone"].(string); tzOK && endTZ != "" {
				if err := validate.ValidateTimezone(endTZ, "end_timezone"); err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
			}
		}
		if bodyStr, ok := args["body"].(string); ok {
			if err := validate.ValidateStringLength(bodyStr, "body", validate.MaxBodyLen); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
		if locationStr, ok := args["location"].(string); ok {
			if err := validate.ValidateStringLength(locationStr, "location", validate.MaxLocationLen); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
		if impStr, ok := args["importance"].(string); ok {
			if err := validate.ValidateImportance(impStr); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
		if sensStr, ok := args["sensitivity"].(string); ok {
			if err := validate.ValidateSensitivity(sensStr); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
		if showStr, ok := args["show_as"].(string); ok {
			if err := validate.ValidateShowAs(showStr); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
		if catStr, ok := args["categories"].(string); ok {
			if err := validate.ValidateStringLength(catStr, "categories", validate.MaxCategoriesLen); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}

		event := models.NewEvent()

		// Optional: subject.
		if subject, ok := args["subject"].(string); ok {
			event.SetSubject(&subject)
		}

		// Optional: start datetime + timezone. When start_datetime is provided
		// without start_timezone, the server's configured default timezone is used.
		if startDT, ok := args["start_datetime"].(string); ok {
			startTZ, _ := args["start_timezone"].(string)
			if startTZ == "" {
				startTZ = defaultTimezone
			}
			start := models.NewDateTimeTimeZone()
			start.SetDateTime(&startDT)
			start.SetTimeZone(&startTZ)
			event.SetStart(start)
		}

		// Optional: end datetime + timezone. When end_datetime is provided
		// without end_timezone, the server's configured default timezone is used.
		if endDT, ok := args["end_datetime"].(string); ok {
			endTZ, _ := args["end_timezone"].(string)
			if endTZ == "" {
				endTZ = defaultTimezone
			}
			end := models.NewDateTimeTimeZone()
			end.SetDateTime(&endDT)
			end.SetTimeZone(&endTZ)
			event.SetEnd(end)
		}

		// Optional: body with content type auto-detection.
		if bodyStr, ok := args["body"].(string); ok {
			body := models.NewItemBody()
			if strings.Contains(bodyStr, "<") {
				contentType := models.HTML_BODYTYPE
				body.SetContentType(&contentType)
			} else {
				contentType := models.TEXT_BODYTYPE
				body.SetContentType(&contentType)
			}
			body.SetContent(&bodyStr)
			event.SetBody(body)
		}

		// Optional: location.
		if locationStr, ok := args["location"].(string); ok {
			loc := models.NewLocation()
			loc.SetDisplayName(&locationStr)
			event.SetLocation(loc)
		}

		// Optional: attendees (replaces entire list).
		if attendeesJSON, ok := args["attendees"].(string); ok {
			attendees, attErr := parseAttendees(attendeesJSON)
			if attErr != nil {
				return mcp.NewToolResultError(attErr.Error()), nil
			}
			event.SetAttendees(attendees)
		}

		// Optional: online meeting.
		if isOnline, ok := args["is_online_meeting"].(bool); ok {
			event.SetIsOnlineMeeting(&isOnline)
			if isOnline {
				provider := models.TEAMSFORBUSINESS_ONLINEMEETINGPROVIDERTYPE
				event.SetOnlineMeetingProvider(&provider)
			}
		}

		// Optional: all-day event.
		if isAllDay, ok := args["is_all_day"].(bool); ok {
			event.SetIsAllDay(&isAllDay)
		}

		// Optional: importance.
		if impStr, ok := args["importance"].(string); ok {
			imp := graph.ParseImportance(impStr)
			event.SetImportance(&imp)
		}

		// Optional: sensitivity.
		if sensStr, ok := args["sensitivity"].(string); ok {
			sens := graph.ParseSensitivity(sensStr)
			event.SetSensitivity(&sens)
		}

		// Optional: show as (free/busy status).
		if showStr, ok := args["show_as"].(string); ok {
			sa := graph.ParseShowAs(showStr)
			event.SetShowAs(&sa)
		}

		// Optional: categories (comma-separated string, replaces all).
		if catStr, ok := args["categories"].(string); ok {
			cats := splitCategories(catStr)
			event.SetCategories(cats)
		}

		// Optional: recurrence. "null" string removes recurrence.
		if recStr, ok := args["recurrence"].(string); ok {
			if recStr == "null" {
				event.SetRecurrence(nil)
			} else {
				rec, recErr := graph.BuildRecurrence(recStr)
				if recErr != nil {
					return mcp.NewToolResultError(recErr.Error()), nil
				}
				event.SetRecurrence(rec)
			}
		}

		// Optional: reminder minutes.
		if remVal, ok := args["reminder_minutes"].(float64); ok {
			rem := int32(remVal)
			event.SetReminderMinutesBeforeStart(&rem)
		}

		// Optional: is_reminder_on.
		if isRemOn, ok := args["is_reminder_on"].(bool); ok {
			event.SetIsReminderOn(&isRemOn)
		}

		// Call PATCH /me/events/{id} with timeout.
		logger.DebugContext(ctx, "updating event", "event_id", eventID)
		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		var updatedEvent models.Eventable
		err = graph.RetryGraphCall(ctx, retryCfg, func() error {
			var graphErr error
			updatedEvent, graphErr = client.Me().Events().ByEventId(eventID).Patch(timeoutCtx, event, nil)
			return graphErr
		})
		if err != nil {
			if graph.IsTimeoutError(err) {
				logger.ErrorContext(ctx, "request timed out",
					"timeout_seconds", int(timeout.Seconds()),
					"error", err.Error())
				return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds()))), nil
			}
			logger.ErrorContext(ctx, "update event failed", "event_id", eventID, "error", graph.FormatGraphError(err))
			return mcp.NewToolResultError(graph.RedactGraphError(err)), nil
		}

		logger.InfoContext(ctx, "event updated", "event_id", eventID)

		// Extract fields for text confirmation.
		eventSubject := graph.SafeStr(updatedEvent.GetSubject())
		displayTime := formatEventDisplayTime(updatedEvent)
		eventLocation := extractEventLocation(updatedEvent)

		response := FormatWriteConfirmation("updated", eventSubject, eventID, displayTime, eventLocation)

		// Append advisory when attendees parameter was provided and non-empty
		// but body or location is missing.
		attendeesStr, _ := args["attendees"].(string)
		bodyStr, _ := args["body"].(string)
		locationStr, _ := args["location"].(string)
		isOnline, _ := args["is_online_meeting"].(bool)
		if advisory := buildAdvisory(attendeesStr != "", bodyStr != "", locationStr != "", isOnline); advisory != "" {
			response += "\n" + advisory
		}

		if line := AccountInfoLine(ctx); line != "" {
			response += "\n" + line
		}

		return mcp.NewToolResultText(response), nil
	}
}
