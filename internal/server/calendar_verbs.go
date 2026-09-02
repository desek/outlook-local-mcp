// Package server — this file builds the calendar domain verb slice for the
// aggregate "calendar" MCP tool (CR-0060 Phase 3d).
//
// It lives in the server package rather than tools to avoid the import cycle
// that would arise from tools importing tools/help (which itself imports tools).
//
// Every calendar verb is always registered (no feature-flag gating):
// help, list_calendars, list_events, get_event, search_events, create_event,
// update_event, delete_event, respond_event, reschedule_event, create_meeting,
// update_meeting, cancel_meeting, reschedule_meeting, get_free_busy,
// find_meeting_times, get_schedule, list_event_attachments,
// get_event_attachment, add_event_attachment.
//
// The aggregate "calendar" tool is registered unconditionally per FR-1.
package server

import (
	"time"

	"github.com/desek/outlook-local-mcp/internal/audit"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/observability"
	"github.com/desek/outlook-local-mcp/internal/tools"
	"github.com/desek/outlook-local-mcp/internal/tools/help"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel/trace"
)

// calendarVerbsConfig holds the dependencies required to build the calendar
// domain verb slice. All fields are captured at server start.
type calendarVerbsConfig struct {
	// retryCfg is the Graph API retry configuration applied to all calendar handlers.
	retryCfg graph.RetryConfig

	// timeout is the maximum duration for a single Graph API call.
	timeout time.Duration

	// defaultTimezone is the IANA timezone used when the caller omits timezone.
	defaultTimezone string

	// provenancePropertyID is the fully-qualified MAPI extended property ID for
	// provenance tagging. Empty string disables tagging.
	provenancePropertyID string

	// m is the ToolMetrics instance for observability instrumentation.
	m *observability.ToolMetrics

	// tracer is the OTEL tracer for span creation.
	tracer trace.Tracer

	// authMW is the authentication middleware factory applied to every verb.
	authMW func(mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc

	// accountResolverMW is the account-resolver middleware applied to every verb.
	accountResolverMW func(mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc

	// readOnly controls whether write verbs are blocked by ReadOnlyGuard.
	readOnly bool

	// maxAttachmentSize is the ceiling in bytes applied to an event attachment
	// downloaded by get_event_attachment and uploaded by add_event_attachment.
	// It is the server's single attachment limit, shared with the mail domain,
	// so a caller reasons about one knob rather than one per domain.
	maxAttachmentSize int64
}

// buildCalendarVerbs constructs the ordered []tools.Verb slice for the calendar
// domain aggregate tool and returns a pointer to an initially empty VerbRegistry.
//
// Every calendar verb is always registered (no feature-flag gating). Each
// verb's Handler is pre-wrapped with authMW, accountResolverMW, observability,
// and audit middleware using the fully-qualified identity "calendar.<verb>" per
// CR-0060 FR-13 and FR-14. Write verbs additionally include ReadOnlyGuard
// between observability and audit.
//
// The returned registry pointer is empty at the time of return. The caller
// MUST call RegisterDomainTool with the returned verbs, then assign the returned
// VerbRegistry back through the pointer so that the help verb can introspect
// all registered verbs at call time.
//
// Parameters:
//   - c: calendarVerbsConfig with all required dependencies.
//
// Returns:
//   - verbs: ordered Verb slice for use with RegisterDomainTool.
//   - registryPtr: pointer whose value is assigned after registration.
func buildCalendarVerbs(c calendarVerbsConfig) ([]tools.Verb, *tools.VerbRegistry) {
	empty := make(tools.VerbRegistry)
	registryPtr := &empty

	// wrap builds the read-verb chain:
	// authMW -> accountResolverMW -> WithObservability -> AuditWrap -> Handler.
	wrap := func(name, auditOp string, h mcpserver.ToolHandlerFunc) tools.Handler {
		return tools.Handler(c.authMW(c.accountResolverMW(observability.WithObservability(name, c.m, c.tracer, audit.AuditWrap(name, auditOp, h)))))
	}

	// wrapWrite adds ReadOnlyGuard between observability and audit for write verbs.
	wrapWrite := func(name, auditOp string, h mcpserver.ToolHandlerFunc) tools.Handler {
		return tools.Handler(c.authMW(c.accountResolverMW(observability.WithObservability(name, c.m, c.tracer, ReadOnlyGuard(name, c.readOnly, audit.AuditWrap(name, auditOp, h))))))
	}

	rc := c.retryCfg
	tz := c.defaultTimezone
	prov := c.provenancePropertyID

	return []tools.Verb{
		help.NewHelpVerb(registryPtr),
		buildListCalendarsVerb(c, rc, wrap),
		buildListEventsVerb(c, rc, tz, prov, wrap),
		buildGetEventVerb(c, rc, tz, prov, wrap),
		buildSearchEventsVerb(c, rc, tz, prov, wrap),
		buildCreateEventVerb(c, rc, tz, prov, wrapWrite),
		buildUpdateEventVerb(c, rc, tz, wrapWrite),
		buildDeleteEventVerb(c, rc, wrapWrite),
		buildRespondEventVerb(c, rc, wrapWrite),
		buildRescheduleEventVerb(c, rc, tz, wrapWrite),
		buildCreateMeetingVerb(c, rc, tz, prov, wrapWrite),
		buildUpdateMeetingVerb(c, rc, tz, wrapWrite),
		buildCancelMeetingVerb(c, rc, wrapWrite),
		buildRescheduleMeetingVerb(c, rc, tz, wrapWrite),
		buildGetFreeBusyVerb(c, rc, tz, wrap),
		buildFindMeetingTimesVerb(c, rc, tz, wrap),
		buildGetScheduleVerb(c, rc, tz, wrap),
		buildListEventAttachmentsVerb(c, rc, wrap),
		buildGetEventAttachmentVerb(c, rc, wrap),
		buildAddEventAttachmentVerb(c, rc, wrapWrite),
	}, registryPtr
}

// buildListCalendarsVerb constructs the list_calendars Verb.
func buildListCalendarsVerb(c calendarVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "list_calendars",
		Summary:     "list all calendars accessible to the authenticated user",
		Description: "Returns all calendars accessible to the authenticated user, including the default calendar and any shared or delegated calendars. Use the returned calendar IDs with list_events to scope event queries to a specific calendar.",
		SeeDocs:     []string{"concepts#multi-account-model-and-upn-identity"},
		Handler:     wrap("calendar.list_calendars", "read", tools.NewHandleListCalendars(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Never assume a default account — always check account list first. Accepts a label (e.g. 'work') or UPN (e.g. 'user@contoso.com'). Disconnected accounts are listed but require login before use."),
			),
			mcp.WithString("output",
				mcp.Description("Output mode: 'text' (default), 'summary', or 'raw'."),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// buildListEventsVerb constructs the list_events Verb.
func buildListEventsVerb(c calendarVerbsConfig, rc graph.RetryConfig, tz, prov string, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "list_events",
		Summary:     "list events in a time window; expands recurring events into occurrences",
		Description: "Lists calendar events within a time window. Recurring events are expanded into individual occurrences. Use the 'date' shorthand for quick queries ('today', 'tomorrow', 'this_week', 'next_week') or provide explicit ISO 8601 start/end datetimes. Results default to the primary calendar; use calendar_id to scope to a specific calendar.",
		Examples: []tools.Example{
			{Args: map[string]any{"date": "today"}, Comment: "list today's events"},
			{Args: map[string]any{"date": "this_week", "max_results": 50}, Comment: "list this week's events, up to 50"},
			{Args: map[string]any{"start_datetime": "2026-04-28T00:00:00", "end_datetime": "2026-04-29T00:00:00"}, Comment: "list events for a specific day"},
		},
		SeeDocs: []string{"concepts#output-tiers"},
		Handler: wrap("calendar.list_events", "read", tools.NewHandleListEvents(rc, c.timeout, tz, prov)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
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
				mcp.Description("Account label or UPN to use. Never assume a default account — always check account list first. Accepts a label (e.g. 'work') or UPN (e.g. 'user@contoso.com'). Disconnected accounts are listed but require login before use."),
			),
			mcp.WithString("output",
				mcp.Description("Output mode: 'text' (default), 'summary', or 'raw'."),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// buildGetEventVerb constructs the get_event Verb.
func buildGetEventVerb(c calendarVerbsConfig, rc graph.RetryConfig, tz, prov string, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "get_event",
		Summary:     "get full event details by ID; bodyPreview by default, full body via output=raw",
		Description: "Fetches the full metadata of a single calendar event by its ID. Text and summary output include a bodyPreview (first 255 characters). To read the complete HTML body, use output=raw. Use list_events or search_events to obtain an event ID.",
		Examples: []tools.Example{
			{Args: map[string]any{"event_id": "<id>", "output": "raw"}, Comment: "fetch full event details including HTML body"},
		},
		SeeDocs: []string{"concepts#output-tiers"},
		Handler: wrap("calendar.get_event", "read", tools.NewHandleGetEvent(rc, c.timeout, tz, prov)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("event_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the event to retrieve."),
			),
			mcp.WithString("timezone",
				mcp.Description("IANA timezone name for returned event times (e.g., America/New_York)."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Never assume a default account — always check account list first. Accepts a label (e.g. 'work') or UPN (e.g. 'user@contoso.com'). Disconnected accounts are listed but require login before use."),
			),
			mcp.WithString("output",
				mcp.Description("Output mode: 'text' (default), 'summary', or 'raw' (includes full HTML body)."),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// buildSearchEventsVerb constructs the search_events Verb.
func buildSearchEventsVerb(c calendarVerbsConfig, rc graph.RetryConfig, tz, prov string, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	schema := []mcp.ToolOption{
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
			mcp.Description("Account label or UPN to use. Never assume a default account — always check account list first. Accepts a label (e.g. 'work') or UPN (e.g. 'user@contoso.com'). Disconnected accounts are listed but require login before use."),
		),
		mcp.WithString("output",
			mcp.Description("Output mode: 'text' (default), 'summary', or 'raw'."),
			mcp.Enum("text", "summary", "raw"),
		),
	}
	if prov != "" {
		schema = append(schema, mcp.WithBoolean("created_by_mcp",
			mcp.Description("When true, only return events created by this MCP server (server-side filter)."),
		))
	}
	return tools.Verb{
		Name:        "search_events",
		Summary:     "search events by subject, date range, importance, sensitivity, and other filters",
		Description: "Searches for calendar events matching the given filters. Subject search is case-insensitive and matches any substring. All filters are combined with AND semantics. Results are ordered by start time. Use get_event with the returned event ID to read full details.",
		Examples: []tools.Example{
			{Args: map[string]any{"query": "standup", "date": "this_week"}, Comment: "find this week's standup meetings"},
			{Args: map[string]any{"query": "review", "importance": "high"}, Comment: "find high-importance review events"},
		},
		SeeDocs: []string{"concepts#output-tiers"},
		Handler: wrap("calendar.search_events", "read", tools.NewHandleSearchEvents(rc, c.timeout, tz, prov)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: schema,
	}
}

// buildCreateEventVerb constructs the create_event Verb.
func buildCreateEventVerb(c calendarVerbsConfig, rc graph.RetryConfig, tz, prov string, wrapWrite func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "create_event",
		Summary:     "create a personal calendar event (no attendees); use create_meeting for meetings",
		Description: "Creates a new personal calendar event with no attendees. For events with attendees that require invitation emails, use create_meeting instead. Supports all-day events, recurrence, Teams online meetings (work/school accounts), categories, reminders, and custom importance/sensitivity. Times must be supplied in ISO 8601 format without a UTC offset (the server applies the configured or specified timezone).",
		Examples: []tools.Example{
			{Args: map[string]any{"subject": "Deep work", "start_datetime": "2026-04-28T09:00:00", "end_datetime": "2026-04-28T11:00:00"}, Comment: "create a two-hour focus block"},
			{Args: map[string]any{"subject": "Team lunch", "start_datetime": "2026-04-28T12:00:00", "is_all_day": false, "location": "Canteen"}, Comment: "create a lunch event with location"},
		},
		SeeDocs: []string{"concepts#output-tiers"},
		Handler: wrapWrite("calendar.create_event", "write", tools.HandleCreateEvent(rc, c.timeout, tz, prov)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(false),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("subject",
				mcp.Required(),
				mcp.Description("Event title."),
			),
			mcp.WithString("start_datetime",
				mcp.Required(),
				mcp.Description("Start time in ISO 8601 without offset, e.g. 2026-04-15T09:00:00."),
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
				mcp.Description("Event body content (HTML or plain text)."),
			),
			mcp.WithString("location",
				mcp.Description("Location display name (e.g. room name, office, or \"Microsoft Teams\")."),
			),
			mcp.WithBoolean("is_online_meeting",
				mcp.Description("Set true to create a Teams online meeting (work/school accounts only)."),
			),
			mcp.WithBoolean("is_all_day",
				mcp.Description("All-day event. Start/end must be midnight in the same timezone."),
			),
			mcp.WithString("importance",
				mcp.Description("Event importance: low, normal, or high."),
			),
			mcp.WithString("sensitivity",
				mcp.Description("Event sensitivity: normal, personal, private, or confidential."),
			),
			mcp.WithString("show_as",
				mcp.Description("Free/busy status: free, tentative, busy, oof, or workingElsewhere."),
			),
			mcp.WithString("categories",
				mcp.Description("Comma-separated category names."),
			),
			mcp.WithString("recurrence",
				mcp.Description(`JSON recurrence object, e.g. {"pattern":{"type":"weekly","interval":1,"daysOfWeek":["monday"]},"range":{"type":"endDate","startDate":"2026-04-15","endDate":"2026-12-31"}}.`),
			),
			mcp.WithNumber("reminder_minutes",
				mcp.Description("Reminder minutes before start."),
			),
			mcp.WithString("calendar_id",
				mcp.Description("Target calendar ID. Omit for default calendar."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Never assume a default account — always check account list first. Accepts a label (e.g. 'work') or UPN (e.g. 'user@contoso.com'). Disconnected accounts are listed but require login before use."),
			),
		},
	}
}

// buildUpdateEventVerb constructs the update_event Verb.
func buildUpdateEventVerb(c calendarVerbsConfig, rc graph.RetryConfig, tz string, wrapWrite func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "update_event",
		Summary:     "update a personal event (PATCH); use update_meeting to change attendees",
		Description: "Updates fields of an existing personal calendar event using PATCH semantics: only supplied fields are changed. Attendee changes are not supported; use update_meeting for meetings with attendees. Recurrence changes apply to the series master only.",
		SeeDocs:     []string{"concepts#output-tiers"},
		Handler:     wrapWrite("calendar.update_event", "write", tools.HandleUpdateEvent(rc, c.timeout, tz)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("event_id",
				mcp.Required(),
				mcp.Description("The unique ID of the event to update."),
			),
			mcp.WithString("subject",
				mcp.Description("New event title."),
			),
			mcp.WithString("start_datetime",
				mcp.Description("New start time in ISO 8601 without offset."),
			),
			mcp.WithString("start_timezone",
				mcp.Description("IANA timezone for new start time."),
			),
			mcp.WithString("end_datetime",
				mcp.Description("New end time in ISO 8601 without offset."),
			),
			mcp.WithString("end_timezone",
				mcp.Description("IANA timezone for new end time."),
			),
			mcp.WithString("body",
				mcp.Description("New event body (HTML or plain text)."),
			),
			mcp.WithString("location",
				mcp.Description("New location display name."),
			),
			mcp.WithBoolean("is_online_meeting",
				mcp.Description("Set true to make this a Teams online meeting, or false to remove it (work/school accounts only)."),
			),
			mcp.WithBoolean("is_all_day",
				mcp.Description("Change all-day status."),
			),
			mcp.WithString("importance",
				mcp.Description("New importance: low, normal, or high."),
			),
			mcp.WithString("sensitivity",
				mcp.Description("New sensitivity: normal, personal, private, or confidential."),
			),
			mcp.WithString("show_as",
				mcp.Description("New free/busy status: free, tentative, busy, oof, or workingElsewhere."),
			),
			mcp.WithString("categories",
				mcp.Description("New comma-separated category names (replaces all)."),
			),
			mcp.WithString("recurrence",
				mcp.Description(`New recurrence JSON object, or "null" to remove. Only for series masters.`),
			),
			mcp.WithNumber("reminder_minutes",
				mcp.Description("New reminder minutes before start."),
			),
			mcp.WithBoolean("is_reminder_on",
				mcp.Description("Enable or disable the reminder."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Never assume a default account — always check account list first. Accepts a label (e.g. 'work') or UPN (e.g. 'user@contoso.com'). Disconnected accounts are listed but require login before use."),
			),
		},
	}
}

// buildDeleteEventVerb constructs the delete_event Verb.
func buildDeleteEventVerb(c calendarVerbsConfig, rc graph.RetryConfig, wrapWrite func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "delete_event",
		Summary:     "permanently delete an event by ID (organizer deletions notify attendees)",
		Description: "Permanently deletes a calendar event. This operation is irreversible. When the authenticated user is the meeting organizer, all attendees receive a cancellation notification. To cancel a meeting without deleting it from the organizer's calendar, use cancel_meeting instead.",
		SeeDocs:     []string{"concepts#read-only-mode"},
		Handler:     wrapWrite("calendar.delete_event", "delete", tools.HandleDeleteEvent(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("event_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the event to delete."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Never assume a default account — always check account list first. Accepts a label (e.g. 'work') or UPN (e.g. 'user@contoso.com'). Disconnected accounts are listed but require login before use."),
			),
		},
	}
}

// buildRespondEventVerb constructs the respond_event Verb.
func buildRespondEventVerb(c calendarVerbsConfig, rc graph.RetryConfig, wrapWrite func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "respond_event",
		Summary:     "accept, tentatively accept, or decline an invitation; notifies organizer",
		Description: "Sends an accept, tentative, or decline response to a meeting invitation. By default the response is sent to the organizer; set send_response=false to update the local calendar state without notifying the organizer. An optional comment is included in the response email.",
		Examples: []tools.Example{
			{Args: map[string]any{"event_id": "<id>", "response": "accept"}, Comment: "accept a meeting invitation"},
			{Args: map[string]any{"event_id": "<id>", "response": "decline", "comment": "Conflict with another commitment"}, Comment: "decline with a message"},
		},
		Handler: wrapWrite("calendar.respond_event", "write", tools.HandleRespondEvent(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("event_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the event to respond to."),
			),
			mcp.WithString("response",
				mcp.Required(),
				mcp.Description("Response type: 'accept', 'tentative', or 'decline'."),
				mcp.Enum("accept", "tentative", "decline"),
			),
			mcp.WithString("comment",
				mcp.Description("Optional message to the organizer explaining your response."),
			),
			mcp.WithBoolean("send_response",
				mcp.Description("Whether to send the response to the organizer. Defaults to true."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Never assume a default account — always check account list first. Accepts a label (e.g. 'work') or UPN (e.g. 'user@contoso.com'). Disconnected accounts are listed but require login before use."),
			),
		},
	}
}

// buildRescheduleEventVerb constructs the reschedule_event Verb.
func buildRescheduleEventVerb(c calendarVerbsConfig, rc graph.RetryConfig, tz string, wrapWrite func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "reschedule_event",
		Summary:     "move a personal event to a new start time, preserving its original duration",
		Description: "Moves a personal calendar event to a new start time while preserving its original duration. For meetings with attendees, use reschedule_meeting so that update notifications are sent to attendees.",
		Examples: []tools.Example{
			{Args: map[string]any{"event_id": "<id>", "new_start_datetime": "2026-04-29T10:00:00"}, Comment: "reschedule an event to a new time"},
		},
		Handler: wrapWrite("calendar.reschedule_event", "write", tools.HandleRescheduleEvent(rc, c.timeout, tz)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("event_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the event to reschedule."),
			),
			mcp.WithString("new_start_datetime",
				mcp.Required(),
				mcp.Description("New start time in ISO 8601 without offset, e.g. 2026-04-17T14:00:00."),
			),
			mcp.WithString("new_start_timezone",
				mcp.Description("IANA timezone for the new start time. Defaults to the server's configured timezone."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Never assume a default account — always check account list first. Accepts a label (e.g. 'work') or UPN (e.g. 'user@contoso.com'). Disconnected accounts are listed but require login before use."),
			),
		},
	}
}

// buildCreateMeetingVerb constructs the create_meeting Verb.
func buildCreateMeetingVerb(c calendarVerbsConfig, rc graph.RetryConfig, tz, prov string, wrapWrite func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "create_meeting",
		Summary:     "create a meeting with attendees; sends invitations; confirm with user first",
		Description: "Creates a calendar event with attendees and sends invitation emails to all recipients. Always present a confirmation summary to the user before calling this verb. Invitation emails are sent immediately upon creation; there is no undo. Include an agenda, context, and location in the body field for external attendees. Attendee data quality affects delivery — verify email addresses before use.",
		Examples: []tools.Example{
			{Args: map[string]any{"subject": "Project kickoff", "start_datetime": "2026-05-01T10:00:00", "attendees": `[{"email":"alice@contoso.com","name":"Alice","type":"required"}]`}, Comment: "create a meeting with one required attendee"},
		},
		SeeDocs: []string{"concepts#mcp-elicitation-requirement"},
		Handler: wrapWrite("calendar.create_meeting", "write", tools.HandleCreateEvent(rc, c.timeout, tz, prov)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(false),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("subject",
				mcp.Required(),
				mcp.Description("Meeting title."),
			),
			mcp.WithString("start_datetime",
				mcp.Required(),
				mcp.Description("Start time in ISO 8601 without offset, e.g. 2026-04-15T09:00:00."),
			),
			mcp.WithString("start_timezone",
				mcp.Description("IANA timezone for start time. Defaults to server's configured timezone when omitted."),
			),
			mcp.WithString("end_datetime",
				mcp.Description("End time in ISO 8601 without offset. Defaults to start_datetime + 30 minutes when omitted."),
			),
			mcp.WithString("end_timezone",
				mcp.Description("IANA timezone for end time. Defaults to server's configured timezone when omitted."),
			),
			mcp.WithString("attendees",
				mcp.Required(),
				mcp.Description(`JSON array of attendees: [{"email":"a@b.com","name":"Name","type":"required|optional|resource"}].`),
			),
			mcp.WithString("body",
				mcp.Description("Meeting body content (HTML or plain text). Strongly recommended — include agenda, context, and location details."),
			),
			mcp.WithString("location",
				mcp.Description("Location display name (e.g. room name, office, or \"Microsoft Teams\"). Include in body for external attendees."),
			),
			mcp.WithBoolean("is_online_meeting",
				mcp.Description("Set true to create a Teams online meeting (work/school accounts only)."),
			),
			mcp.WithBoolean("is_all_day",
				mcp.Description("All-day event. Start/end must be midnight in the same timezone."),
			),
			mcp.WithString("importance",
				mcp.Description("Event importance: low, normal, or high."),
			),
			mcp.WithString("sensitivity",
				mcp.Description("Event sensitivity: normal, personal, private, or confidential."),
			),
			mcp.WithString("show_as",
				mcp.Description("Free/busy status: free, tentative, busy, oof, or workingElsewhere."),
			),
			mcp.WithString("categories",
				mcp.Description("Comma-separated category names."),
			),
			mcp.WithString("recurrence",
				mcp.Description(`JSON recurrence object, e.g. {"pattern":{"type":"weekly","interval":1,"daysOfWeek":["monday"]},"range":{"type":"endDate","startDate":"2026-04-15","endDate":"2026-12-31"}}.`),
			),
			mcp.WithNumber("reminder_minutes",
				mcp.Description("Reminder minutes before start."),
			),
			mcp.WithString("calendar_id",
				mcp.Description("Target calendar ID. Omit for default calendar."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Never assume a default account — always check account list first. Accepts a label (e.g. 'work') or UPN (e.g. 'user@contoso.com'). Disconnected accounts are listed but require login before use."),
			),
		},
	}
}

// buildUpdateMeetingVerb constructs the update_meeting Verb.
func buildUpdateMeetingVerb(c calendarVerbsConfig, rc graph.RetryConfig, tz string, wrapWrite func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "update_meeting",
		Summary:     "update a meeting including attendee changes; sends notifications; confirm first",
		Description: "Updates a meeting and sends update notification emails to all attendees. PATCH semantics: only supplied fields are changed. The attendees field, when supplied, replaces the entire attendee list. Always confirm the changes with the user before calling this verb.",
		SeeDocs:     []string{"concepts#mcp-elicitation-requirement"},
		Handler:     wrapWrite("calendar.update_meeting", "write", tools.HandleUpdateEvent(rc, c.timeout, tz)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("event_id",
				mcp.Required(),
				mcp.Description("The unique ID of the meeting to update."),
			),
			mcp.WithString("subject",
				mcp.Description("New meeting title."),
			),
			mcp.WithString("start_datetime",
				mcp.Description("New start time in ISO 8601 without offset."),
			),
			mcp.WithString("start_timezone",
				mcp.Description("IANA timezone for new start time."),
			),
			mcp.WithString("end_datetime",
				mcp.Description("New end time in ISO 8601 without offset."),
			),
			mcp.WithString("end_timezone",
				mcp.Description("IANA timezone for new end time."),
			),
			mcp.WithString("attendees",
				mcp.Description(`New attendees JSON array (replaces entire list): [{"email":"a@b.com","name":"Name","type":"required"}].`),
			),
			mcp.WithString("body",
				mcp.Description("New meeting body (HTML or plain text). Include agenda and location for external attendees."),
			),
			mcp.WithString("location",
				mcp.Description("New location display name."),
			),
			mcp.WithBoolean("is_online_meeting",
				mcp.Description("Set true to make this a Teams online meeting, or false to remove it (work/school accounts only)."),
			),
			mcp.WithBoolean("is_all_day",
				mcp.Description("Change all-day status."),
			),
			mcp.WithString("importance",
				mcp.Description("New importance: low, normal, or high."),
			),
			mcp.WithString("sensitivity",
				mcp.Description("New sensitivity: normal, personal, private, or confidential."),
			),
			mcp.WithString("show_as",
				mcp.Description("New free/busy status: free, tentative, busy, oof, or workingElsewhere."),
			),
			mcp.WithString("categories",
				mcp.Description("New comma-separated category names (replaces all)."),
			),
			mcp.WithString("recurrence",
				mcp.Description(`New recurrence JSON object, or "null" to remove. Only for series masters.`),
			),
			mcp.WithNumber("reminder_minutes",
				mcp.Description("New reminder minutes before start."),
			),
			mcp.WithBoolean("is_reminder_on",
				mcp.Description("Enable or disable the reminder."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Never assume a default account — always check account list first. Accepts a label (e.g. 'work') or UPN (e.g. 'user@contoso.com'). Disconnected accounts are listed but require login before use."),
			),
		},
	}
}

// buildCancelMeetingVerb constructs the cancel_meeting Verb.
func buildCancelMeetingVerb(c calendarVerbsConfig, rc graph.RetryConfig, wrapWrite func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "cancel_meeting",
		Summary:     "cancel a meeting and notify attendees; organizer only; confirm with user first",
		Description: "Cancels a meeting and sends cancellation notices to all attendees. Only the meeting organizer can cancel. An optional comment is included in the cancellation email. Always confirm with the user before calling this verb. To remove the event without sending notices, use delete_event.",
		SeeDocs:     []string{"concepts#mcp-elicitation-requirement"},
		Handler:     wrapWrite("calendar.cancel_meeting", "delete", tools.HandleCancelEvent(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("event_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the meeting to cancel."),
			),
			mcp.WithString("comment",
				mcp.Description("Optional custom cancellation message sent to all attendees."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Never assume a default account — always check account list first. Accepts a label (e.g. 'work') or UPN (e.g. 'user@contoso.com'). Disconnected accounts are listed but require login before use."),
			),
		},
	}
}

// buildRescheduleMeetingVerb constructs the reschedule_meeting Verb.
func buildRescheduleMeetingVerb(c calendarVerbsConfig, rc graph.RetryConfig, tz string, wrapWrite func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "reschedule_meeting",
		Summary:     "move a meeting to a new start time, preserving duration; notifies attendees",
		Description: "Moves a meeting to a new start time, preserving its original duration, and sends rescheduling notifications to all attendees. Always confirm the new time with the user before calling. For personal events without attendees, use reschedule_event.",
		SeeDocs:     []string{"concepts#mcp-elicitation-requirement"},
		Handler:     wrapWrite("calendar.reschedule_meeting", "write", tools.HandleRescheduleEvent(rc, c.timeout, tz)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("event_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the meeting to reschedule."),
			),
			mcp.WithString("new_start_datetime",
				mcp.Required(),
				mcp.Description("New start time in ISO 8601 without offset, e.g. 2026-04-17T14:00:00."),
			),
			mcp.WithString("new_start_timezone",
				mcp.Description("IANA timezone for the new start time. Defaults to the server's configured timezone."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Never assume a default account — always check account list first. Accepts a label (e.g. 'work') or UPN (e.g. 'user@contoso.com'). Disconnected accounts are listed but require login before use."),
			),
		},
	}
}

// buildGetFreeBusyVerb constructs the get_free_busy Verb.
func buildGetFreeBusyVerb(c calendarVerbsConfig, rc graph.RetryConfig, tz string, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "get_free_busy",
		Summary:     "get free/busy availability for a time range; returns busy periods with times",
		Description: "Returns the free/busy schedule for the authenticated user's calendar within the specified time range. Each busy period includes a start time, end time, and status (busy, tentative, oof, workingElsewhere). Use this before creating a meeting to find available slots.",
		Examples: []tools.Example{
			{Args: map[string]any{"date": "tomorrow"}, Comment: "check availability for tomorrow"},
		},
		Handler: wrap("calendar.get_free_busy", "read", tools.NewHandleGetFreeBusy(rc, c.timeout, tz)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
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
				mcp.Description("Account label or UPN to use. Never assume a default account — always check account list first. Accepts a label (e.g. 'work') or UPN (e.g. 'user@contoso.com'). Disconnected accounts are listed but require login before use."),
			),
			mcp.WithString("output",
				mcp.Description("Output mode: 'text' (default), 'summary', or 'raw'."),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// buildFindMeetingTimesVerb constructs the find_meeting_times Verb.
//
// It is registered on the read chain because the call writes nothing: the
// request body carries the query rather than a mutation, which is the Graph
// convention for a read whose input is too large for a query string.
func buildFindMeetingTimesVerb(c calendarVerbsConfig, rc graph.RetryConfig, tz string, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "find_meeting_times",
		Summary:     "propose meeting slots that suit a set of attendees, ranked by confidence",
		Description: "Asks Microsoft Graph to propose meeting slots that suit a set of attendees, returning candidates ranked by confidence with the organizer's availability and, where Graph supplies one, the reason a slot was suggested. Prefer this when the question is \"when could these people meet?\"; prefer get_schedule when you need each mailbox's actual free/busy blocks and working hours, and get_free_busy when you only need the signed-in user's own busy periods with their subjects. Requires attendees. meeting_duration defaults to PT30M and max_candidates to 20; start_datetime and end_datetime bound the search and must be supplied together or not at all, and when both are omitted Graph applies its own window. Read-only, non-destructive, idempotent, and open-world: it queries Graph and changes nothing, so repeating it with the same arguments over an unchanged mailbox returns the same result. Returns a numbered listing by default; use output=summary or output=raw for structured results.",
		Examples: []tools.Example{
			{Args: map[string]any{"attendees": `[{"email":"alice@contoso.com","type":"required"}]`}, Comment: "propose slots for one required attendee using Graph's default window"},
			{Args: map[string]any{"attendees": `[{"email":"alice@contoso.com","type":"required"},{"email":"bob@contoso.com","type":"optional"}]`, "meeting_duration": "PT1H", "start_datetime": "2026-05-01T09:00:00", "end_datetime": "2026-05-02T17:00:00"}, Comment: "propose hour-long slots inside an explicit window"},
		},
		SeeDocs: []string{"concepts#output-tiers"},
		Handler: wrap("calendar.find_meeting_times", "read", tools.NewHandleFindMeetingTimes(rc, c.timeout, tz)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("attendees",
				mcp.Required(),
				mcp.Description(`JSON array of attendees: [{"email":"a@b.com","name":"Name","type":"required|optional|resource"}]. The name field is accepted and ignored, so one attendee list can be passed to both this verb and create_meeting.`),
			),
			mcp.WithString("meeting_duration",
				mcp.Description("Meeting length as an ISO 8601 duration, e.g. PT30M or PT1H30M. Defaults to PT30M when omitted."),
			),
			mcp.WithString("start_datetime",
				mcp.Description("Start of the search window in ISO 8601 without offset, e.g. 2026-05-01T09:00:00. Supply together with end_datetime, or omit both to let Graph apply its own window."),
			),
			mcp.WithString("end_datetime",
				mcp.Description("End of the search window in ISO 8601 without offset. Supply together with start_datetime, or omit both to let Graph apply its own window."),
			),
			mcp.WithNumber("max_candidates",
				mcp.Description("Maximum number of suggestions to return. Defaults to 20 when omitted."),
				mcp.Min(1),
				mcp.Max(100),
			),
			mcp.WithNumber("minimum_attendee_percentage",
				mcp.Description("Minimum percentage of attendees that must be free for a slot to be suggested. Omitted from the request when not supplied, leaving Graph's own default in force."),
				mcp.Min(0),
				mcp.Max(100),
			),
			mcp.WithBoolean("is_organizer_optional",
				mcp.Description("Set true to let the search ignore the organizer's own availability. Omitted from the request when not supplied."),
			),
			mcp.WithString("timezone",
				mcp.Description("IANA timezone name for the search window bounds (e.g., America/New_York). Defaults to the server's configured timezone."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Never assume a default account — always check account list first. Accepts a label (e.g. 'work') or UPN (e.g. 'user@contoso.com'). Disconnected accounts are listed but require login before use."),
			),
			mcp.WithString("output",
				mcp.Description("Output mode: 'text' (default), 'summary', or 'raw'."),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// buildGetScheduleVerb constructs the get_schedule Verb.
//
// Like find_meeting_times it is registered on the read chain: the POST body
// carries the mailbox list and the window, not a mutation.
func buildGetScheduleVerb(c calendarVerbsConfig, rc graph.RetryConfig, tz string, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "get_schedule",
		Summary:     "read free/busy blocks and working hours for one or more mailboxes",
		Description: "Returns the free/busy blocks and the working hours of one or more mailboxes the signed-in user is permitted to view, attributing every block to the mailbox it belongs to. Prefer this over get_free_busy when the question concerns somebody other than the signed-in user, names more than one mailbox, or needs working hours; prefer get_free_busy for the signed-in user's own busy periods, which alone carry the subject of each period; prefer find_meeting_times when you want Graph to propose slots rather than to report availability. Requires schedules and a resolvable window, supplied either as start_datetime and end_datetime or as the date shorthand, with the explicit datetimes taking precedence. A mailbox the caller may not view is reported as an error against that mailbox while the other mailboxes still return their blocks. Read-only, non-destructive, idempotent, and open-world: it queries Graph and changes nothing. Returns labeled per-mailbox sections by default; use output=summary or output=raw for structured results.",
		Examples: []tools.Example{
			{Args: map[string]any{"schedules": "alice@contoso.com,bob@contoso.com", "date": "tomorrow"}, Comment: "read two mailboxes' availability for tomorrow"},
			{Args: map[string]any{"schedules": "room-oslo@contoso.com", "start_datetime": "2026-05-01T08:00:00", "end_datetime": "2026-05-01T18:00:00", "availability_view_interval": 15}, Comment: "read one room's availability at fifteen-minute granularity"},
		},
		SeeDocs: []string{"concepts#output-tiers"},
		Handler: wrap("calendar.get_schedule", "read", tools.NewHandleGetSchedule(rc, c.timeout, tz)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("schedules",
				mcp.Required(),
				mcp.Description("Comma-separated SMTP addresses of the mailboxes to query, e.g. alice@contoso.com,room-oslo@contoso.com. At most 20 mailboxes per call, which is Graph's own ceiling."),
			),
			mcp.WithString("date",
				mcp.Description("Date shorthand: 'today', 'tomorrow', 'this_week', 'next_week', or ISO 8601 date (YYYY-MM-DD). Expands to start/end boundaries in the configured timezone. When start_datetime/end_datetime are also provided, they take precedence."),
			),
			mcp.WithString("start_datetime",
				mcp.Description("Start of the time range in ISO 8601 format (e.g., 2026-05-01T08:00:00). Required unless 'date' is provided."),
			),
			mcp.WithString("end_datetime",
				mcp.Description("End of the time range in ISO 8601 format (e.g., 2026-05-01T18:00:00). Required unless 'date' is provided."),
			),
			mcp.WithNumber("availability_view_interval",
				mcp.Description("Granularity in minutes of the returned availability view string. Defaults to 30 when omitted."),
				mcp.Min(5),
				mcp.Max(1440),
			),
			mcp.WithString("timezone",
				mcp.Description("IANA timezone name for the queried window and the returned times (e.g., America/New_York). Defaults to the server's configured timezone."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Never assume a default account — always check account list first. Accepts a label (e.g. 'work') or UPN (e.g. 'user@contoso.com'). Disconnected accounts are listed but require login before use."),
			),
			mcp.WithString("output",
				mcp.Description("Output mode: 'text' (default), 'summary', or 'raw'."),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// buildListEventAttachmentsVerb constructs the list_event_attachments Verb.
//
// It is registered on the read chain and unconditionally: an event attachment
// is calendar data, so neither mail gate governs it, and a caller reading a
// meeting's agenda must not have to enable the mail surface to see it.
func buildListEventAttachmentsVerb(c calendarVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "list_event_attachments",
		Summary:     "list attachment metadata (id, name, contentType, size) for an event",
		Description: "Lists the attachments of a calendar event, returning metadata only: attachment ID, name, content type, and size in bytes. No content bytes are fetched, so the response stays small however large the attachments are; pass a returned attachment_id to get_event_attachment to download one. Requires event_id. Read-only, non-destructive, idempotent, and open-world: it queries Graph and changes nothing. Returns a numbered list by default; use output=summary or output=raw for structured results.",
		Examples: []tools.Example{
			{Args: map[string]any{"event_id": "AAMkAGI2..."}, Comment: "list what is attached to an event"},
			{Args: map[string]any{"event_id": "AAMkAGI2...", "output": "raw"}, Comment: "read the full attachment metadata as structured JSON"},
		},
		SeeDocs: []string{"concepts#output-tiers"},
		Handler: wrap("calendar.list_event_attachments", "read", tools.NewHandleListEventAttachments(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("event_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the event whose attachments are listed."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Never assume a default account — always check account list first. Accepts a label (e.g. 'work') or UPN (e.g. 'user@contoso.com'). Disconnected accounts are listed but require login before use."),
			),
			mcp.WithString("output",
				mcp.Description("Output mode: 'text' (default), 'summary', or 'raw'."),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// buildGetEventAttachmentVerb constructs the get_event_attachment Verb.
//
// The size ceiling is stated in the published description because it is the one
// thing about this verb a caller cannot discover from the schema: the refusal
// arrives only after a request, and a caller that knows the bound in advance
// can choose to list first rather than spend the call.
func buildGetEventAttachmentVerb(c calendarVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "get_event_attachment",
		Summary:     "download an event attachment; returns metadata and base64 content",
		Description: "Downloads one attachment of a calendar event and returns its metadata plus the content as a standard base64-encoded string. Requires event_id and attachment_id; obtain the attachment_id from list_event_attachments. The full content is returned only within the server's attachment size limit (OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES, 10 MB by default); a larger attachment is refused with an error naming the limit, and no partial content is returned, so read the size from list_event_attachments first when the attachment may be large. Read-only, non-destructive, idempotent, and open-world: it queries Graph and changes nothing. Returns labeled fields by default; use output=summary or output=raw for structured results.",
		Examples: []tools.Example{
			{Args: map[string]any{"event_id": "AAMkAGI2...", "attachment_id": "AAMkAGI2ZGY..."}, Comment: "download an attachment of an event"},
			{Args: map[string]any{"event_id": "AAMkAGI2...", "attachment_id": "AAMkAGI2ZGY...", "output": "raw"}, Comment: "read the attachment and its metadata as structured JSON"},
		},
		SeeDocs: []string{"concepts#output-tiers"},
		Handler: wrap("calendar.get_event_attachment", "read", tools.NewHandleGetEventAttachment(rc, c.timeout, c.maxAttachmentSize)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("event_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the event the attachment belongs to."),
			),
			mcp.WithString("attachment_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the attachment, as returned by list_event_attachments."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Never assume a default account — always check account list first. Accepts a label (e.g. 'work') or UPN (e.g. 'user@contoso.com'). Disconnected accounts are listed but require login before use."),
			),
			mcp.WithString("output",
				mcp.Description("Output mode: 'text' (default), 'summary', or 'raw'."),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// buildAddEventAttachmentVerb constructs the add_event_attachment Verb.
//
// The MIME parameter is named mime_type rather than content_type for the same
// reason the mail domain names it so: the aggregate tool publishes the union of
// its verbs' parameters, and one concept keeps one name across the surface.
//
// No is_inline parameter is published. An event attachment the caller supplies
// as a file has no body to be inline within, so the flag would name a choice
// this verb does not offer.
func buildAddEventAttachmentVerb(c calendarVerbsConfig, rc graph.RetryConfig, wrapWrite func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "add_event_attachment",
		Summary:     "attach a file to an existing event; bytes supplied base64 in content_bytes",
		Description: "Attaches a file to an existing calendar event. Requires event_id, name, and content_bytes; content_bytes carries the file content as a standard base64-encoded string, and the optional mime_type sets the attachment content type (defaults to application/octet-stream). The transfer mechanism is chosen from the decoded size, small files in a single request and larger ones through a chunked upload session, so the caller never selects it. Content above the server's attachment size limit (OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES, 10 MB by default) is refused before any upload begins. Attaching to an event that already has attendees does not itself notify them; use update_event when the change should reach them. Non-read-only, non-destructive (it only adds, leaving the event and its existing attachments untouched), non-idempotent (a repeated call adds a second attachment with a new ID), and open-world. The confirmation names the event, the attachment, its size, and the attachment ID the service assigned. Returns a text confirmation and takes no output parameter.",
		Examples: []tools.Example{
			{Args: map[string]any{"event_id": "AAMkAGI2...", "name": "agenda.pdf", "content_bytes": "JVBERi0xLjQK...", "mime_type": "application/pdf"}, Comment: "attach a PDF agenda to a meeting"},
			{Args: map[string]any{"event_id": "AAMkAGI2...", "name": "notes.txt", "content_bytes": "aGVsbG8gd29ybGQ="}, Comment: "attach a small file without naming its MIME type"},
		},
		SeeDocs: []string{"concepts#read-only-mode"},
		Handler: wrapWrite("calendar.add_event_attachment", "write", tools.NewHandleAddEventAttachment(rc, c.timeout, c.maxAttachmentSize)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(false),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("event_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the event the attachment is added to."),
			),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("File name the attachment carries on the event, for example agenda.pdf. At most 255 characters."),
			),
			mcp.WithString("content_bytes",
				mcp.Required(),
				mcp.Description("File content as a standard base64-encoded string. The decoded size selects the transfer path and is checked against the server's attachment size limit."),
			),
			mcp.WithString("mime_type",
				mcp.Description("MIME type of the attachment, for example application/pdf. Omit to default to application/octet-stream. This is the attachment content type, not the event body content type."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Never assume a default account — always check account list first. Accepts a label (e.g. 'work') or UPN (e.g. 'user@contoso.com'). Disconnected accounts are listed but require login before use."),
			),
		},
	}
}
