// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the create_event MCP tool, which creates a new calendar
// event via the Microsoft Graph API. It supports the full range of event
// properties: subject, start/end with timezones, body (HTML/text auto-detection),
// location, attendees (with automatic invitation sending), Teams online meetings,
// all-day events, importance, sensitivity, free/busy status, categories,
// recurrence patterns, and reminders.
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
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// maxAttendees is the Exchange Online limit for the number of attendees on a
// single event. The Graph API rejects requests exceeding this limit.
const maxAttendees = 500

// NewCreateEventTool creates the MCP tool definition for create_event. The tool
// accepts required parameters for subject and start_datetime, plus optional
// parameters for start/end timezone (default to server's configured timezone),
// end_datetime (default to start + 30 minutes, or + 24 hours for all-day
// events), body, location, attendees, online meeting, all-day, importance,
// sensitivity, show_as, categories, recurrence, reminder_minutes, and
// calendar_id.
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewCreateEventTool() mcp.Tool {
	return mcp.NewTool("calendar_create_event",
		mcp.WithTitleAnnotation("Create Calendar Event"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithDescription(
			"Create a new calendar event. Supports attendees (sends invitations "+
				"automatically), Teams online meetings, recurrence, and all standard "+
				"event properties.\n\n"+
				"Only subject and start_datetime are required. Timezones default to "+
				"the server's configured timezone, and end_datetime defaults to "+
				"start + 30 minutes (or + 24 hours for all-day events).\n\n"+
				"IMPORTANT: When attendees are included, always provide a body "+
				"(agenda or description) and location so recipients understand the "+
				"purpose and place of the meeting. Ask the user for these details "+
				"or suggest appropriate values before creating the event.\n\n"+
				"IMPORTANT: When the event includes attendees, you MUST present a draft "+
				"summary to the user and wait for explicit confirmation before calling "+
				"this tool. The summary MUST include: subject, date/time, attendee list, "+
				"location, and body preview. If any attendee email domain differs from the "+
				"user's own domain, add an explicit warning that external recipients will "+
				"receive the invitation. Only call this tool after the user confirms.",
		),
		mcp.WithString("subject", mcp.Required(),
			mcp.Description("Event title"),
		),
		mcp.WithString("start_datetime", mcp.Required(),
			mcp.Description("Start time in ISO 8601 without offset, e.g. 2026-04-15T09:00:00"),
		),
		mcp.WithString("start_timezone",
			mcp.Description("IANA timezone for start time, e.g. America/New_York. Defaults to server's configured timezone when omitted."),
		),
		mcp.WithString("end_datetime",
			mcp.Description("End time in ISO 8601 without offset. Defaults to start_datetime + 30 minutes (or + 24 hours for all-day events) when omitted."),
		),
		mcp.WithString("end_timezone",
			mcp.Description("IANA timezone for end time. Defaults to server's configured timezone when omitted."),
		),
		mcp.WithString("body",
			mcp.Description("Event body content (HTML or plain text). Strongly recommended when "+
				"attendees are invited -- include the meeting agenda, purpose, or discussion topics. "+
				"Attendees receive this in their invitation."),
		),
		mcp.WithString("location",
			mcp.Description("Location display name (e.g. room name, office, or \"Microsoft Teams\"). "+
				"Strongly recommended when attendees are invited. If an online meeting is enabled, "+
				"you may use \"Microsoft Teams\" or omit this."),
		),
		mcp.WithString("attendees",
			mcp.Description(`JSON array of attendees: [{"email":"a@b.com","name":"Name","type":"required|optional|resource"}]`),
		),
		mcp.WithBoolean("is_online_meeting",
			mcp.Description("Set true to create a Teams online meeting (work/school accounts only)"),
		),
		mcp.WithBoolean("is_all_day",
			mcp.Description("All-day event. Start/end must be midnight in the same timezone."),
		),
		mcp.WithString("importance",
			mcp.Description("Event importance: low, normal, or high"),
		),
		mcp.WithString("sensitivity",
			mcp.Description("Event sensitivity: normal, personal, private, or confidential"),
		),
		mcp.WithString("show_as",
			mcp.Description("Free/busy status: free, tentative, busy, oof, or workingElsewhere"),
		),
		mcp.WithString("categories",
			mcp.Description("Comma-separated category names"),
		),
		mcp.WithString("recurrence",
			mcp.Description(`JSON recurrence object, e.g. {"pattern":{"type":"weekly","interval":1,"daysOfWeek":["monday"]},"range":{"type":"endDate","startDate":"2026-04-15","endDate":"2026-12-31"}}`),
		),
		mcp.WithNumber("reminder_minutes",
			mcp.Description("Reminder minutes before start"),
		),
		mcp.WithString("calendar_id",
			mcp.Description("Target calendar ID. Omit for default calendar."),
		),
		mcp.WithString("account",
			mcp.Description("Account label to use. If omitted, the default account is used. Use account_list to see available accounts."),
		),
	)
}

// HandleCreateEvent is the MCP tool handler for create_event. It extracts
// parameters from the request, constructs a models.Event, calls the appropriate
// Graph API endpoint, and returns a concise text confirmation. The Graph client
// is retrieved from the request context at invocation time.
//
// When provenancePropertyID is non-empty, the handler stamps a single-value
// extended property on the event before the POST call, tagging it as
// MCP-created. When empty, provenance tagging is skipped entirely.
//
// Parameters:
//   - retryCfg: retry configuration for transient Graph API errors.
//   - timeout: the maximum duration for the Graph API call.
//   - defaultTimezone: the IANA timezone from server config, used as the default
//     when start_timezone or end_timezone is omitted by the caller.
//   - provenancePropertyID: the full MAPI property ID for provenance tagging,
//     built once at startup via graph.BuildProvenancePropertyID. Empty string
//     disables provenance tagging.
//
// Returns a closure matching the MCP tool handler function signature.
//
// Side effects: calls POST /me/events or POST /me/calendars/{id}/events on the
// Microsoft Graph API. Logs at debug level on entry, error level on failure,
// and info level on success.
func HandleCreateEvent(retryCfg graph.RetryConfig, timeout time.Duration, defaultTimezone string, provenancePropertyID string) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := slog.With("tool", "calendar_create_event")
		args := request.GetArguments()
		logger.DebugContext(ctx, "tool called", "params", args)

		client, err := GraphClient(ctx)
		if err != nil {
			return mcp.NewToolResultError("no account selected"), nil
		}

		// Extract required parameters.
		subject, err := request.RequireString("subject")
		if err != nil {
			return mcp.NewToolResultError("missing required parameter: subject. Tip: Ask the user what they'd like to name the event."), nil
		}
		startDT, err := request.RequireString("start_datetime")
		if err != nil {
			return mcp.NewToolResultError("missing required parameter: start_datetime"), nil
		}

		// Extract optional timezone parameters, defaulting to server config.
		startTZ, _ := args["start_timezone"].(string)
		if startTZ == "" {
			startTZ = defaultTimezone
		}
		endTZ, _ := args["end_timezone"].(string)
		if endTZ == "" {
			endTZ = defaultTimezone
		}

		// Extract optional end_datetime, defaulting to start + 30 min (or + 24h for all-day).
		endDT, _ := args["end_datetime"].(string)
		if endDT == "" {
			isAllDay, _ := args["is_all_day"].(bool)
			computed, computeErr := computeDefaultEndTime(startDT, isAllDay)
			if computeErr != nil {
				return mcp.NewToolResultError(computeErr.Error()), nil
			}
			endDT = computed
		}

		// Validate required and defaulted parameters.
		if err := validate.ValidateStringLength(subject, "subject", validate.MaxSubjectLen); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := validate.ValidateDatetime(startDT, "start_datetime"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := validate.ValidateTimezone(startTZ, "start_timezone"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := validate.ValidateDatetime(endDT, "end_datetime"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := validate.ValidateTimezone(endTZ, "end_timezone"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Validate optional parameters.
		if bodyStr, ok := args["body"].(string); ok && bodyStr != "" {
			if err := validate.ValidateStringLength(bodyStr, "body", validate.MaxBodyLen); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
		if locationStr, ok := args["location"].(string); ok && locationStr != "" {
			if err := validate.ValidateStringLength(locationStr, "location", validate.MaxLocationLen); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
		if impStr, ok := args["importance"].(string); ok && impStr != "" {
			if err := validate.ValidateImportance(impStr); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
		if sensStr, ok := args["sensitivity"].(string); ok && sensStr != "" {
			if err := validate.ValidateSensitivity(sensStr); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
		if showStr, ok := args["show_as"].(string); ok && showStr != "" {
			if err := validate.ValidateShowAs(showStr); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
		if catStr, ok := args["categories"].(string); ok && catStr != "" {
			if err := validate.ValidateStringLength(catStr, "categories", validate.MaxCategoriesLen); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}
		if calID, ok := args["calendar_id"].(string); ok && calID != "" {
			if err := validate.ValidateResourceID(calID, "calendar_id"); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}

		// Construct the event.
		event := models.NewEvent()
		event.SetSubject(&subject)

		// Start time.
		start := models.NewDateTimeTimeZone()
		start.SetDateTime(&startDT)
		start.SetTimeZone(&startTZ)
		event.SetStart(start)

		// End time.
		end := models.NewDateTimeTimeZone()
		end.SetDateTime(&endDT)
		end.SetTimeZone(&endTZ)
		event.SetEnd(end)

		// Optional: body with content type auto-detection.
		if bodyStr, ok := args["body"].(string); ok && bodyStr != "" {
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
		if locationStr, ok := args["location"].(string); ok && locationStr != "" {
			loc := models.NewLocation()
			loc.SetDisplayName(&locationStr)
			event.SetLocation(loc)
		}

		// Optional: attendees.
		if attendeesJSON, ok := args["attendees"].(string); ok && attendeesJSON != "" {
			attendees, attErr := parseAttendees(attendeesJSON)
			if attErr != nil {
				return mcp.NewToolResultError(attErr.Error()), nil
			}
			event.SetAttendees(attendees)
		}

		// Optional: online meeting.
		if isOnline, ok := args["is_online_meeting"].(bool); ok && isOnline {
			event.SetIsOnlineMeeting(&isOnline)
			provider := models.TEAMSFORBUSINESS_ONLINEMEETINGPROVIDERTYPE
			event.SetOnlineMeetingProvider(&provider)
		}

		// Optional: all-day event.
		if isAllDay, ok := args["is_all_day"].(bool); ok {
			event.SetIsAllDay(&isAllDay)
		}

		// Optional: importance.
		if impStr, ok := args["importance"].(string); ok && impStr != "" {
			imp := graph.ParseImportance(impStr)
			event.SetImportance(&imp)
		}

		// Optional: sensitivity.
		if sensStr, ok := args["sensitivity"].(string); ok && sensStr != "" {
			sens := graph.ParseSensitivity(sensStr)
			event.SetSensitivity(&sens)
		}

		// Optional: show as (free/busy status).
		if showStr, ok := args["show_as"].(string); ok && showStr != "" {
			sa := graph.ParseShowAs(showStr)
			event.SetShowAs(&sa)
		}

		// Optional: categories (comma-separated string).
		if catStr, ok := args["categories"].(string); ok && catStr != "" {
			cats := splitCategories(catStr)
			event.SetCategories(cats)
		}

		// Optional: recurrence.
		if recStr, ok := args["recurrence"].(string); ok && recStr != "" {
			rec, recErr := graph.BuildRecurrence(recStr)
			if recErr != nil {
				return mcp.NewToolResultError(recErr.Error()), nil
			}
			event.SetRecurrence(rec)
		}

		// Optional: reminder minutes.
		if remVal, ok := args["reminder_minutes"].(float64); ok {
			rem := int32(remVal)
			event.SetReminderMinutesBeforeStart(&rem)
			isOn := true
			event.SetIsReminderOn(&isOn)
		}

		// Stamp provenance extended property when enabled.
		if provenancePropertyID != "" {
			event.SetSingleValueExtendedProperties(
				[]models.SingleValueLegacyExtendedPropertyable{
					graph.NewProvenanceProperty(provenancePropertyID),
				},
			)
		}

		// Route to the appropriate Graph API endpoint.
		timeoutCtx, cancel := graph.WithTimeout(ctx, timeout)
		defer cancel()

		var createdEvent models.Eventable
		if calID, ok := args["calendar_id"].(string); ok && calID != "" {
			logger.DebugContext(ctx, "creating event in calendar", "calendar_id", calID)
			err = graph.RetryGraphCall(ctx, retryCfg, func() error {
				var graphErr error
				createdEvent, graphErr = client.Me().Calendars().ByCalendarId(calID).Events().Post(timeoutCtx, event, nil)
				return graphErr
			})
		} else {
			logger.DebugContext(ctx, "creating event in default calendar")
			err = graph.RetryGraphCall(ctx, retryCfg, func() error {
				var graphErr error
				createdEvent, graphErr = client.Me().Events().Post(timeoutCtx, event, nil)
				return graphErr
			})
		}
		if err != nil {
			if graph.IsTimeoutError(err) {
				logger.ErrorContext(ctx, "request timed out",
					"timeout_seconds", int(timeout.Seconds()),
					"error", err.Error())
				return mcp.NewToolResultError(graph.TimeoutErrorMessage(int(timeout.Seconds()))), nil
			}
			logger.ErrorContext(ctx, "create event failed", "error", graph.FormatGraphError(err))
			return mcp.NewToolResultError(graph.RedactGraphError(err)), nil
		}

		eventID := graph.SafeStr(createdEvent.GetId())
		logger.InfoContext(ctx, "event created", "event_id", eventID)

		// Extract fields for text confirmation.
		eventSubject := graph.SafeStr(createdEvent.GetSubject())
		displayTime := formatEventDisplayTime(createdEvent)
		eventLocation := extractEventLocation(createdEvent)

		response := FormatWriteConfirmation("created", eventSubject, eventID, displayTime, eventLocation)

		// Append advisory when attendees are present but body or location is missing.
		attendeesStr, _ := args["attendees"].(string)
		bodyStr, _ := args["body"].(string)
		locationStr, _ := args["location"].(string)
		isOnline, _ := args["is_online_meeting"].(bool)
		if advisory := buildAdvisory(attendeesStr != "", bodyStr != "", locationStr != "", isOnline); advisory != "" {
			response += "\n" + advisory
		}

		return mcp.NewToolResultText(response), nil
	}
}

// defaultMeetingDuration is the default duration for a new event when
// end_datetime is omitted: 30 minutes.
const defaultMeetingDuration = 30 * time.Minute

// defaultAllDayDuration is the default duration for an all-day event when
// end_datetime is omitted: 24 hours.
const defaultAllDayDuration = 24 * time.Hour

// computeDefaultEndTime computes the default end_datetime from start_datetime
// when the caller omits it. For regular events the default is start + 30
// minutes; for all-day events it is start + 24 hours.
//
// Parameters:
//   - startDT: the start datetime in ISO 8601 local format (2006-01-02T15:04:05).
//   - isAllDay: true when the event is an all-day event.
//
// Returns the computed end datetime as an ISO 8601 local string, or an error
// if startDT cannot be parsed.
func computeDefaultEndTime(startDT string, isAllDay bool) (string, error) {
	t, err := time.Parse("2006-01-02T15:04:05", startDT)
	if err != nil {
		return "", fmt.Errorf("cannot compute default end time: invalid start_datetime %q", startDT)
	}
	if isAllDay {
		return t.Add(defaultAllDayDuration).Format("2006-01-02T15:04:05"), nil
	}
	return t.Add(defaultMeetingDuration).Format("2006-01-02T15:04:05"), nil
}

// parseAttendees parses a JSON array of attendee objects and constructs a slice
// of models.Attendeeable. Each object must have "email" and "name" fields, with
// an optional "type" field (defaults to "required").
//
// Parameters:
//   - jsonStr: a JSON array string of attendee objects.
//
// Returns the constructed attendee slice, or an error if JSON parsing fails or
// the attendee count exceeds the 500 limit.
//
// Side effects: none.
func parseAttendees(jsonStr string) ([]models.Attendeeable, error) {
	var attendeeList []struct {
		Email string `json:"email"`
		Name  string `json:"name"`
		Type  string `json:"type"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &attendeeList); err != nil {
		return nil, fmt.Errorf("invalid attendees JSON: %w", err)
	}
	if len(attendeeList) > maxAttendees {
		return nil, fmt.Errorf("attendee count %d exceeds maximum of %d", len(attendeeList), maxAttendees)
	}

	result := make([]models.Attendeeable, len(attendeeList))
	for i, a := range attendeeList {
		if err := validate.ValidateEmail(a.Email); err != nil {
			return nil, fmt.Errorf("attendee %d: %w", i, err)
		}
		if a.Type != "" {
			if err := validate.ValidateAttendeeType(a.Type); err != nil {
				return nil, fmt.Errorf("attendee %d: %w", i, err)
			}
		}
		att := models.NewAttendee()
		email := models.NewEmailAddress()
		addr := a.Email
		email.SetAddress(&addr)
		name := a.Name
		email.SetName(&name)
		att.SetEmailAddress(email)
		attType := graph.ParseAttendeeType(a.Type)
		att.SetTypeEscaped(&attType)
		result[i] = att
	}
	return result, nil
}

// splitCategories splits a comma-separated string into a trimmed string slice.
// Empty entries after trimming are excluded.
//
// Parameters:
//   - s: the comma-separated categories string.
//
// Returns a string slice with trimmed, non-empty category names.
func splitCategories(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// formatEventDisplayTime computes a human-readable display time string from a
// Graph API event's start/end datetime and timezone fields.
//
// Parameters:
//   - event: a models.Eventable with start and end DateTimeTimeZone fields.
//
// Returns the formatted display time string, or empty string when the event
// has no start/end.
//
// Side effects: none.
func formatEventDisplayTime(event models.Eventable) string {
	startDT, startTZ, endDT, endTZ := "", "", "", ""
	if s := event.GetStart(); s != nil {
		startDT = graph.SafeStr(s.GetDateTime())
		startTZ = graph.SafeStr(s.GetTimeZone())
	}
	if e := event.GetEnd(); e != nil {
		endDT = graph.SafeStr(e.GetDateTime())
		endTZ = graph.SafeStr(e.GetTimeZone())
	}
	return graph.FormatDisplayTime(startDT, endDT, startTZ, endTZ, graph.SafeBool(event.GetIsAllDay()))
}

// extractEventLocation returns the display name of an event's location, or
// empty string when no location is set.
//
// Parameters:
//   - event: a models.Eventable with an optional Location field.
//
// Returns the location display name or empty string.
//
// Side effects: none.
func extractEventLocation(event models.Eventable) string {
	if loc := event.GetLocation(); loc != nil {
		return graph.SafeStr(loc.GetDisplayName())
	}
	return ""
}
