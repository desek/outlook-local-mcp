// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the update_meeting MCP tool definition, which modifies an
// existing calendar meeting (event with attendees) via the Microsoft Graph API.
// The tool reuses HandleUpdateEvent for its handler logic; only the tool
// definition differs from update_event. The attendees parameter is present as
// optional (updates may modify other fields while attendees exist on the event),
// and the description includes unconditional confirmation guidance for
// attendee-affecting operations.
package tools

import (
	"github.com/mark3labs/mcp-go/mcp"
)

// NewUpdateMeetingTool creates the MCP tool definition for calendar_update_meeting.
// The tool has the same parameters as NewUpdateEventTool. The attendees parameter
// remains optional because updates may modify other fields while attendees exist
// on the event. The description includes unconditional confirmation guidance,
// CR-0039 body/location advisory, external domain warning, and AskUserQuestion
// reference.
//
// The handler for this tool is HandleUpdateEvent -- meeting tools reuse
// existing event handlers since the underlying Graph API calls are identical.
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewUpdateMeetingTool() mcp.Tool {
	return mcp.NewTool("calendar_update_meeting",
		mcp.WithTitleAnnotation("Update Calendar Meeting"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithDescription(
			"Update an existing calendar meeting. Only specified fields are changed "+
				"(PATCH semantics). Automatically sends update notifications to "+
				"attendees.\n\n"+
				"IMPORTANT: When attendees are included, always provide a body "+
				"(agenda or description) and location so attendees understand the "+
				"purpose and place of the meeting. Ask the user for these details "+
				"or suggest appropriate values before updating the meeting.\n\n"+
				"IMPORTANT: You MUST present a draft summary of the changes to the user "+
				"and wait for explicit confirmation before calling this tool. The summary "+
				"MUST include the fields being changed and the affected attendees. If any "+
				"new attendee email domain differs from the user's own domain, add an "+
				"explicit warning that external recipients will receive update "+
				"notifications. Only call this tool after the user confirms. "+
				"If the AskUserQuestion tool is available, use it to present the summary "+
				"and collect confirmation for a better user experience.",
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
			mcp.Description("New event body (HTML or plain text). Strongly recommended -- include "+
				"the meeting agenda, purpose, or discussion topics. Attendees receive "+
				"this in update notifications."),
		),
		mcp.WithString("location",
			mcp.Description("New location display name (e.g. room name, office, or \"Microsoft Teams\"). "+
				"Strongly recommended. If an online meeting is enabled, you may use "+
				"\"Microsoft Teams\" or omit this."),
		),
		mcp.WithString("attendees",
			mcp.Description(`New attendees JSON array (replaces entire list): [{"email":"a@b.com","name":"Name","type":"required"}]`),
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
