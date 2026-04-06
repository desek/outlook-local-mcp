// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the reschedule_meeting MCP tool definition, which moves an
// existing calendar meeting (event with attendees) to a new time while preserving
// its original duration. The tool reuses HandleRescheduleEvent for its handler
// logic; only the tool definition differs from reschedule_event. The description
// includes unconditional confirmation guidance warning about attendee update
// notifications.
package tools

import (
	"github.com/mark3labs/mcp-go/mcp"
)

// NewRescheduleMeetingTool creates the MCP tool definition for
// calendar_reschedule_meeting. The tool has the same parameters as
// NewRescheduleEventTool. The description includes unconditional confirmation
// guidance referencing attendee notifications and AskUserQuestion reference.
//
// The handler for this tool is HandleRescheduleEvent -- meeting tools reuse
// existing event handlers since the underlying Graph API calls are identical.
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewRescheduleMeetingTool() mcp.Tool {
	return mcp.NewTool("calendar_reschedule_meeting",
		mcp.WithTitleAnnotation("Reschedule Meeting"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithDescription(
			"Move a meeting to a new time, preserving its original duration. "+
				"Only the new start time is required; the end time is computed "+
				"automatically. Sends update notifications to all attendees.\n\n"+
				"IMPORTANT: Rescheduling sends update notifications to all attendees. "+
				"You MUST present a draft summary to the user showing the event subject, "+
				"current time, proposed new time, and attendee list, then wait for "+
				"explicit confirmation before calling this tool. "+
				"If the AskUserQuestion tool is available, use it to present the summary "+
				"and collect confirmation for a better user experience.",
		),
		mcp.WithString("event_id", mcp.Required(),
			mcp.Description("The unique identifier of the event to reschedule."),
		),
		mcp.WithString("new_start_datetime", mcp.Required(),
			mcp.Description("New start time in ISO 8601 without offset, e.g. 2026-04-17T14:00:00"),
		),
		mcp.WithString("new_start_timezone",
			mcp.Description("IANA timezone for the new start time. Defaults to the server's configured timezone."),
		),
		mcp.WithString("account",
			mcp.Description("Account label to use. If omitted, the default account is used. Use account_list to see available accounts."),
		),
	)
}
