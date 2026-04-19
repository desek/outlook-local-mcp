// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the create_meeting MCP tool definition, which creates a
// new calendar meeting (event with attendees) via the Microsoft Graph API. The
// tool reuses HandleCreateEvent for its handler logic; only the tool definition
// differs from create_event. The attendees parameter is required, and the
// description includes unconditional confirmation guidance for attendee-affecting
// operations.
package tools

import (
	"github.com/mark3labs/mcp-go/mcp"
)

// NewCreateMeetingTool creates the MCP tool definition for calendar_create_meeting.
// The tool has the same parameters as NewCreateEventTool except the attendees
// parameter is required (not optional). The description includes unconditional
// confirmation guidance, CR-0039 body/location advisory, external domain
// warning, and AskUserQuestion reference.
//
// The handler for this tool is HandleCreateEvent -- meeting tools reuse
// existing event handlers since the underlying Graph API calls are identical.
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewCreateMeetingTool() mcp.Tool {
	return mcp.NewTool("calendar_create_meeting",
		mcp.WithTitleAnnotation("Create Calendar Meeting"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithDescription(
			"Create a new calendar meeting with attendees. Sends invitations "+
				"automatically. Supports Teams online meetings, recurrence, and all "+
				"standard event properties.\n\n"+
				"Only subject, start_datetime, and attendees are required. Timezones "+
				"default to the server's configured timezone, and end_datetime defaults "+
				"to start + 30 minutes (or + 24 hours for all-day events).\n\n"+
				"IMPORTANT: Always provide a body (agenda or description) and location "+
				"so attendees understand the purpose and place of the meeting. Ask the "+
				"user for these details or suggest appropriate values before creating "+
				"the meeting.\n\n"+
				"IMPORTANT: You MUST present a draft summary to the user and wait for "+
				"explicit confirmation before calling this tool. The summary MUST include: "+
				"subject, date/time, attendee list, location, and body preview. If any "+
				"attendee email domain differs from the user's own domain, add an explicit "+
				"warning that external recipients will receive the invitation. Only call "+
				"this tool after the user confirms. "+
				"If the AskUserQuestion tool is available, use it to present the summary "+
				"and collect confirmation for a better user experience.",
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
			mcp.Description("Event body content (HTML or plain text). Strongly recommended -- include "+
				"the meeting agenda, purpose, or discussion topics. Attendees receive this "+
				"in their invitation."),
		),
		mcp.WithString("location",
			mcp.Description("Location display name (e.g. room name, office, or \"Microsoft Teams\"). "+
				"Strongly recommended. If an online meeting is enabled, you may use "+
				"\"Microsoft Teams\" or omit this."),
		),
		mcp.WithString("attendees", mcp.Required(),
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
			mcp.Description(AccountParamDescription),
		),
	)
}
