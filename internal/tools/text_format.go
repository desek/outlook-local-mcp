// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides plain-text formatters for the "text" output mode on read
// tools. Each formatter takes serialized data (maps from the graph serialization
// layer) and produces a human-readable plain-text string with numbered listings,
// formatted times, and summary totals.
package tools

import (
	"fmt"
	"strings"
	"time"

	"github.com/desek/outlook-local-mcp/internal/buildinfo"
)

// FormatEventsText formats a slice of serialized summary event maps into a
// numbered plain-text listing with human-readable times and a total count.
// Each event shows subject, displayTime, location, showAs status, and organizer.
//
// Parameters:
//   - events: slice of summary event maps (from SerializeSummaryEvent or
//     ToSummaryEventMap), each expected to contain "subject", "displayTime",
//     "location", "showAs", and "organizer" keys.
//
// Returns a formatted plain-text string. Returns "No events found." when
// the slice is empty.
//
// Side effects: none.
func FormatEventsText(events []map[string]any) string {
	if len(events) == 0 {
		return "No events found."
	}

	var b strings.Builder
	for i, e := range events {
		subject, _ := e["subject"].(string)
		if subject == "" {
			subject = "(No subject)"
		}
		fmt.Fprintf(&b, "%d. %s\n", i+1, subject)

		// Time | Location | Status line.
		displayTime, _ := e["displayTime"].(string)
		location, _ := e["location"].(string)
		showAs, _ := e["showAs"].(string)

		var details []string
		if displayTime != "" {
			details = append(details, displayTime)
		}
		if location != "" {
			details = append(details, location)
		}
		if showAs != "" {
			details = append(details, strings.Title(showAs)) //nolint:staticcheck // strings.Title is sufficient for single-word enum values
		}
		if len(details) > 0 {
			fmt.Fprintf(&b, "   %s\n", strings.Join(details, " | "))
		}

		// Organizer line.
		organizer, _ := e["organizer"].(string)
		if organizer != "" {
			fmt.Fprintf(&b, "   Organizer: %s\n", organizer)
		}

		// Blank line between events.
		if i < len(events)-1 {
			b.WriteString("\n")
		}
	}

	// Summary total.
	fmt.Fprintf(&b, "\n%d event(s) total.", len(events))

	return b.String()
}

// FormatEventDetailText formats a single serialized event map into a
// human-readable plain-text detail view. Includes subject, time, location,
// organizer, status, attendees, and body preview when available.
//
// Parameters:
//   - event: a summary-get event map (from SerializeSummaryGetEvent), expected
//     to contain "subject", "displayTime", "location", "organizer", "showAs",
//     "attendees", and "bodyPreview" keys.
//
// Returns a formatted plain-text string.
//
// Side effects: none.
func FormatEventDetailText(event map[string]any) string {
	var b strings.Builder

	subject, _ := event["subject"].(string)
	if subject == "" {
		subject = "(No subject)"
	}
	b.WriteString(subject)
	b.WriteString("\n")

	displayTime, _ := event["displayTime"].(string)
	if displayTime != "" {
		fmt.Fprintf(&b, "Time: %s\n", displayTime)
	}

	location, _ := event["location"].(string)
	if location != "" {
		fmt.Fprintf(&b, "Location: %s\n", location)
	}

	organizer, _ := event["organizer"].(string)
	if organizer != "" {
		fmt.Fprintf(&b, "Organizer: %s\n", organizer)
	}

	showAs, _ := event["showAs"].(string)
	if showAs != "" {
		fmt.Fprintf(&b, "Status: %s\n", strings.Title(showAs)) //nolint:staticcheck // strings.Title is sufficient for single-word enum values
	}

	// Attendees list.
	if attendees, ok := event["attendees"].([]map[string]string); ok && len(attendees) > 0 {
		b.WriteString("Attendees:\n")
		for _, att := range attendees {
			name := att["name"]
			resp := att["response"]
			if name != "" {
				if resp != "" {
					fmt.Fprintf(&b, "  - %s (%s)\n", name, resp)
				} else {
					fmt.Fprintf(&b, "  - %s\n", name)
				}
			}
		}
	}

	// Body preview.
	bodyPreview, _ := event["bodyPreview"].(string)
	if bodyPreview != "" {
		fmt.Fprintf(&b, "\n%s\n", bodyPreview)
	}

	return b.String()
}

// FormatCalendarsText formats a slice of serialized calendar maps into a
// numbered plain-text listing.
//
// Parameters:
//   - calendars: slice of calendar maps (from SerializeCalendar), each expected
//     to contain "name", "owner", "isDefaultCalendar", and "canEdit" keys.
//
// Returns a formatted plain-text string. Returns "No calendars found." when
// the slice is empty.
//
// Side effects: none.
func FormatCalendarsText(calendars []map[string]any) string {
	if len(calendars) == 0 {
		return "No calendars found."
	}

	var b strings.Builder
	for i, cal := range calendars {
		name, _ := cal["name"].(string)
		if name == "" {
			name = "(Unnamed)"
		}

		var tags []string
		if isDefault, _ := cal["isDefaultCalendar"].(bool); isDefault {
			tags = append(tags, "default")
		}
		if canEdit, _ := cal["canEdit"].(bool); !canEdit {
			tags = append(tags, "read-only")
		}

		line := fmt.Sprintf("%d. %s", i+1, name)
		if len(tags) > 0 {
			line += fmt.Sprintf(" (%s)", strings.Join(tags, ", "))
		}
		b.WriteString(line)
		b.WriteString("\n")

		// Owner line.
		if ownerObj, ok := cal["owner"].(map[string]string); ok {
			ownerName := ownerObj["name"]
			if ownerName != "" {
				fmt.Fprintf(&b, "   Owner: %s\n", ownerName)
			}
		}
	}

	fmt.Fprintf(&b, "\n%d calendar(s) total.", len(calendars))

	return b.String()
}

// FormatFreeBusyText formats a FreeBusyResponse into a human-readable
// plain-text listing of busy periods.
//
// Parameters:
//   - data: a FreeBusyResponse struct containing timeRange and busyPeriods.
//
// Returns a formatted plain-text string with a numbered list of busy periods.
// Returns "No busy periods found." when there are no busy periods.
//
// Side effects: none.
func FormatFreeBusyText(data FreeBusyResponse) string {
	if len(data.BusyPeriods) == 0 {
		return "No busy periods found."
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Busy periods (%s to %s):\n\n", data.TimeRange.Start, data.TimeRange.End)

	for i, bp := range data.BusyPeriods {
		subject := bp.Subject
		if subject == "" {
			subject = "(No subject)"
		}
		fmt.Fprintf(&b, "%d. %s\n", i+1, subject)

		// Prefer the localised rendering so free/busy text matches the
		// convention event listings use. Falling back to the raw ISO values
		// keeps output useful when Graph supplied no usable timezone.
		when := bp.DisplayTime
		if when == "" {
			when = fmt.Sprintf("%s - %s", bp.Start, bp.End)
		}
		fmt.Fprintf(&b, "   %s | %s\n", when, strings.Title(bp.Status)) //nolint:staticcheck // strings.Title is sufficient for single-word enum values

		if i < len(data.BusyPeriods)-1 {
			b.WriteString("\n")
		}
	}

	fmt.Fprintf(&b, "\n%d busy period(s) total.", len(data.BusyPeriods))

	return b.String()
}

// FormatMeetingTimeSuggestionsText formats a FindMeetingTimesResponse into a
// numbered plain-text listing of candidate meeting slots.
//
// Parameters:
//   - data: the response envelope carrying summary-serialized suggestions and,
//     when Graph offered none, its own reason for the empty result.
//
// Returns a formatted plain-text string with a numbered list of suggestions and
// a total count. When there are no suggestions, returns Graph's stated reason
// so the caller learns why no slot was offered rather than only that none was.
//
// Side effects: none.
func FormatMeetingTimeSuggestionsText(data FindMeetingTimesResponse) string {
	if len(data.Suggestions) == 0 {
		if data.EmptySuggestionsReason != "" {
			return fmt.Sprintf("No meeting times suggested. Reason: %s", data.EmptySuggestionsReason)
		}
		return "No meeting times suggested."
	}

	var b strings.Builder
	for i, s := range data.Suggestions {
		fmt.Fprintf(&b, "%d. %s\n", i+1, meetingSlotDisplay(s))

		var details []string
		if confidence, ok := s["confidence"].(float64); ok {
			details = append(details, fmt.Sprintf("Confidence %.0f%%", confidence))
		}
		if organizer, ok := s["organizerAvailability"].(string); ok && organizer != "" {
			details = append(details, fmt.Sprintf("Organizer %s", organizer))
		}
		if len(details) > 0 {
			fmt.Fprintf(&b, "   %s\n", strings.Join(details, " | "))
		}
		if reason, ok := s["suggestionReason"].(string); ok && reason != "" {
			fmt.Fprintf(&b, "   %s\n", reason)
		}

		if i < len(data.Suggestions)-1 {
			b.WriteString("\n")
		}
	}

	fmt.Fprintf(&b, "\n%d suggestion(s) total.", len(data.Suggestions))

	return b.String()
}

// meetingSlotDisplay renders the time slot of a summary-serialized meeting-time
// suggestion, preferring the localised displayTime and falling back to the raw
// ISO bounds when Graph supplied no usable timezone.
func meetingSlotDisplay(suggestion map[string]any) string {
	slot, ok := suggestion["meetingTimeSlot"].(map[string]string)
	if !ok {
		return "(No time slot)"
	}
	if slot["displayTime"] != "" {
		return slot["displayTime"]
	}
	if slot["start"] == "" && slot["end"] == "" {
		return "(No time slot)"
	}
	return fmt.Sprintf("%s - %s", slot["start"], slot["end"])
}

// FormatScheduleText formats a GetScheduleResponse into a labeled per-mailbox
// plain-text listing of free/busy blocks, working hours, and, where Graph
// reported one, that mailbox's error.
//
// Parameters:
//   - data: the response envelope carrying the resolved window and one
//     summary-serialized record per mailbox, in the order they were requested.
//
// Returns a formatted plain-text string with one labeled section per mailbox and
// a total count. A mailbox Graph could not read states its error rather than
// being omitted, so a caller can tell a mailbox with no meetings from one it may
// not view.
//
// Side effects: none.
func FormatScheduleText(data GetScheduleResponse) string {
	if len(data.Schedules) == 0 {
		return "No schedules returned."
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Schedules (%s to %s):\n\n", data.TimeRange.Start, data.TimeRange.End)

	for i, record := range data.Schedules {
		fmt.Fprintf(&b, "%s\n", scheduleMailboxLabel(record))

		if failure := scheduleErrorLine(record); failure != "" {
			fmt.Fprintf(&b, "  Error: %s\n", failure)
		}
		if hours := scheduleWorkingHoursLine(record); hours != "" {
			fmt.Fprintf(&b, "  Working hours: %s\n", hours)
		}
		writeScheduleItems(&b, record)

		if i < len(data.Schedules)-1 {
			b.WriteString("\n")
		}
	}

	fmt.Fprintf(&b, "\n%d mailbox(es) total.", len(data.Schedules))

	return b.String()
}

// writeScheduleItems renders one mailbox's busy blocks, or states that it has
// none, so an empty schedule reads as a finding rather than a missing section.
func writeScheduleItems(b *strings.Builder, record map[string]any) {
	items, _ := record["scheduleItems"].([]map[string]any)
	if len(items) == 0 {
		b.WriteString("  No busy periods.\n")
		return
	}
	for _, item := range items {
		when, _ := item["displayTime"].(string)
		if when == "" {
			start, _ := item["start"].(string)
			end, _ := item["end"].(string)
			when = fmt.Sprintf("%s - %s", start, end)
		}
		status, _ := item["status"].(string)
		if status == "" {
			status = "unknown"
		}
		fmt.Fprintf(b, "  %s | %s\n", when, status)
	}
}

// scheduleErrorLine renders the per-mailbox error Graph reported, naming both
// the message and the response code so the caller can act on either. Returns
// the empty string when the mailbox was read successfully.
func scheduleErrorLine(record map[string]any) string {
	failure, ok := record["error"].(map[string]any)
	if !ok {
		return ""
	}
	message, _ := failure["message"].(string)
	code, _ := failure["responseCode"].(string)
	switch {
	case message != "" && code != "":
		return fmt.Sprintf("%s (%s)", message, code)
	case message != "":
		return message
	default:
		return code
	}
}

// scheduleWorkingHoursLine renders a mailbox's working hours as its days and
// its daily bounds. Returns the empty string when Graph supplied none, so the
// section is omitted rather than shown blank.
func scheduleWorkingHoursLine(record map[string]any) string {
	hours, ok := record["workingHours"].(map[string]any)
	if !ok {
		return ""
	}
	days, _ := hours["daysOfWeek"].([]string)
	startTime, _ := hours["startTime"].(string)
	endTime, _ := hours["endTime"].(string)
	timeZone, _ := hours["timeZone"].(string)

	if len(days) == 0 && startTime == "" && endTime == "" {
		return ""
	}

	line := fmt.Sprintf("%s - %s", startTime, endTime)
	if len(days) > 0 {
		line = fmt.Sprintf("%s, %s", strings.Join(days, ", "), line)
	}
	if timeZone != "" {
		line += " (" + timeZone + ")"
	}
	return line
}

// FormatMessagesText formats a slice of serialized summary message maps into a
// numbered plain-text listing. Each message shows subject, sender address, date,
// read/attachment status flags, and body preview.
//
// Parameters:
//   - messages: slice of summary message maps (from SerializeSummaryMessage),
//     each expected to contain "subject", "from" (map with "address"),
//     "receivedDateTime", "isRead", "hasAttachments", and "bodyPreview" keys.
//
// Returns a formatted plain-text string. Returns "No messages found." when the
// slice is nil or empty.
//
// Side effects: none.
func FormatMessagesText(messages []map[string]any) string {
	if len(messages) == 0 {
		return "No messages found."
	}

	var b strings.Builder
	for i, m := range messages {
		subject, _ := m["subject"].(string)
		if subject == "" {
			subject = "(No subject)"
		}
		fmt.Fprintf(&b, "%d. %s\n", i+1, subject)

		// From and date line.
		fromAddr := extractFromAddress(m)
		receivedDT, _ := m["receivedDateTime"].(string)
		displayDate := formatReceivedDate(receivedDT)

		var parts []string
		if fromAddr != "" {
			parts = append(parts, "From: "+fromAddr)
		}
		if displayDate != "" {
			parts = append(parts, displayDate)
		}
		if len(parts) > 0 {
			fmt.Fprintf(&b, "   %s\n", strings.Join(parts, " | "))
		}

		// Status flags line.
		var flags []string
		if isRead, ok := m["isRead"].(bool); ok && !isRead {
			flags = append(flags, "[Unread]")
		}
		if hasAtt, ok := m["hasAttachments"].(bool); ok && hasAtt {
			flags = append(flags, "[Has attachments]")
		}
		if len(flags) > 0 {
			fmt.Fprintf(&b, "   %s\n", strings.Join(flags, " "))
		}

		// Body preview.
		bodyPreview, _ := m["bodyPreview"].(string)
		if bodyPreview != "" {
			fmt.Fprintf(&b, "   Preview: %s\n", bodyPreview)
		}

		// Blank line between messages.
		if i < len(messages)-1 {
			b.WriteString("\n")
		}
	}

	fmt.Fprintf(&b, "\n%d message(s) total.", len(messages))

	return b.String()
}

// extractFromAddress extracts the sender email address from a message map.
// The "from" field may be a map[string]string (from SerializeSummaryMessage)
// or a map[string]any (from JSON round-trip).
//
// Parameters:
//   - m: a message map containing an optional "from" key.
//
// Returns the sender email address, or "" if not available.
//
// Side effects: none.
func extractFromAddress(m map[string]any) string {
	switch from := m["from"].(type) {
	case map[string]string:
		return from["address"]
	case map[string]any:
		addr, _ := from["address"].(string)
		return addr
	}
	return ""
}

// formatReceivedDate parses an RFC3339 datetime string and returns a
// human-readable date string in "Mon Jan 02, 2006 3:04 PM" format.
// Returns an empty string if the input is empty or cannot be parsed.
//
// Parameters:
//   - rfc3339: an RFC3339-formatted datetime string.
//
// Returns the formatted date string, or "" on empty/invalid input.
//
// Side effects: none.
func formatReceivedDate(rfc3339 string) string {
	if rfc3339 == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return rfc3339
	}
	return t.Format("Mon Jan 2, 2006 3:04 PM")
}

// FormatMessageDetailText formats a single serialized message map into a
// human-readable plain-text detail view. Includes subject, sender, recipients,
// date, importance (when not "normal"), attachment indicator, and body preview.
//
// Parameters:
//   - message: a message map (from SerializeSummaryMessage or SerializeMessage),
//     expected to contain "subject", "from", "toRecipients", "receivedDateTime",
//     "importance", "hasAttachments", and "bodyPreview" keys.
//
// Returns a formatted plain-text string.
//
// Side effects: none.
func FormatMessageDetailText(message map[string]any) string {
	var b strings.Builder

	subject, _ := message["subject"].(string)
	if subject == "" {
		subject = "(No subject)"
	}
	b.WriteString(subject)
	b.WriteString("\n")

	// From line.
	fromAddr := extractFromAddress(message)
	if fromAddr != "" {
		fmt.Fprintf(&b, "From: %s\n", fromAddr)
	}

	// To line.
	toLine := formatRecipientAddresses(message["toRecipients"])
	if toLine != "" {
		fmt.Fprintf(&b, "To: %s\n", toLine)
	}

	// Date line.
	receivedDT, _ := message["receivedDateTime"].(string)
	displayDate := formatReceivedDate(receivedDT)
	if displayDate != "" {
		fmt.Fprintf(&b, "Date: %s\n", displayDate)
	}

	// Importance (only if not normal).
	importance, _ := message["importance"].(string)
	if importance != "" && importance != "normal" {
		fmt.Fprintf(&b, "Importance: %s\n", importance)
	}

	// Attachment indicator.
	if hasAtt, ok := message["hasAttachments"].(bool); ok && hasAtt {
		b.WriteString("[Has attachments]\n")
	}

	// Provenance indicator (only when the server has provenance tagging
	// configured and the field is present on the message map).
	if prov, ok := message["provenance"].(bool); ok && prov {
		b.WriteString("[Created by this MCP server]\n")
	}

	// Body preview.
	bodyPreview, _ := message["bodyPreview"].(string)
	if bodyPreview != "" {
		fmt.Fprintf(&b, "\n%s\n", bodyPreview)
	}

	return b.String()
}

// formatRecipientAddresses extracts email addresses from a recipients field
// and joins them with ", ". Handles both []map[string]string (direct
// serialization) and []any (from JSON round-trip).
//
// Parameters:
//   - recipients: the "toRecipients" (or similar) field from a message map.
//
// Returns a comma-separated string of email addresses, or "" if empty.
//
// Side effects: none.
func formatRecipientAddresses(recipients any) string {
	var addrs []string
	switch rs := recipients.(type) {
	case []map[string]string:
		for _, r := range rs {
			if addr := r["address"]; addr != "" {
				addrs = append(addrs, addr)
			}
		}
	case []any:
		for _, r := range rs {
			if rm, ok := r.(map[string]any); ok {
				if addr, _ := rm["address"].(string); addr != "" {
					addrs = append(addrs, addr)
				}
			}
		}
	}
	return strings.Join(addrs, ", ")
}

// FormatConversationText formats a serialized conversation thread into a
// numbered plain-text listing ordered chronologically. Each entry shows the
// message subject, sender address, received date, and a body preview.
//
// Parameters:
//   - thread: a map produced by graph.SerializeConversationThread, expected to
//     contain "conversationId", "count", and "messages" keys.
//
// Returns a formatted plain-text string. Returns "No messages found." when the
// thread contains no messages.
//
// Side effects: none.
func FormatConversationText(thread map[string]any) string {
	messages, _ := thread["messages"].([]map[string]any)
	if len(messages) == 0 {
		return "No messages found."
	}

	var b strings.Builder
	convoID, _ := thread["conversationId"].(string)
	if convoID != "" {
		fmt.Fprintf(&b, "Conversation: %s\n\n", convoID)
	}
	for i, m := range messages {
		subject, _ := m["subject"].(string)
		if subject == "" {
			subject = "(No subject)"
		}
		fmt.Fprintf(&b, "%d. %s\n", i+1, subject)

		fromAddr := extractFromAddress(m)
		receivedDT, _ := m["receivedDateTime"].(string)
		displayDate := formatReceivedDate(receivedDT)
		var parts []string
		if fromAddr != "" {
			parts = append(parts, "From: "+fromAddr)
		}
		if displayDate != "" {
			parts = append(parts, displayDate)
		}
		if len(parts) > 0 {
			fmt.Fprintf(&b, "   %s\n", strings.Join(parts, " | "))
		}

		if prov, ok := m["provenance"].(bool); ok && prov {
			b.WriteString("   [Created by this MCP server]\n")
		}

		bodyPreview, _ := m["bodyPreview"].(string)
		if bodyPreview != "" {
			fmt.Fprintf(&b, "   Preview: %s\n", bodyPreview)
		}

		if i < len(messages)-1 {
			b.WriteString("\n")
		}
	}
	fmt.Fprintf(&b, "\n%d message(s) in thread.", len(messages))
	return b.String()
}

// FormatAttachmentText formats a serialized attachment map into a human-readable
// plain-text detail view showing name, content type, size, and an indicator
// that the content is delivered as base64 in summary/raw output modes.
//
// Parameters:
//   - att: a map produced by graph.SerializeAttachment.
//
// Returns a formatted plain-text string.
//
// Side effects: none.
func FormatAttachmentText(att map[string]any) string {
	var b strings.Builder
	name, _ := att["name"].(string)
	if name == "" {
		name = "(Unnamed attachment)"
	}
	b.WriteString(name)
	b.WriteString("\n")
	if ct, _ := att["contentType"].(string); ct != "" {
		fmt.Fprintf(&b, "Content-Type: %s\n", ct)
	}
	fmt.Fprintf(&b, "Size: %d bytes\n", toInt(att["size"]))
	if inline, ok := att["isInline"].(bool); ok && inline {
		b.WriteString("Inline: true\n")
	}
	if _, ok := att["contentBytes"]; ok {
		b.WriteString("Content available as base64 via output=summary or output=raw.")
	} else {
		b.WriteString("This attachment has no downloadable file content (not a file attachment).")
	}
	return b.String()
}

// FormatAttachmentsText formats a slice of attachment metadata maps into a
// numbered plain-text listing showing id, name, content type, size, and inline
// flag. Used by mail_list_attachments.
//
// Parameters:
//   - atts: slice of attachment maps (from graph.SerializeSummaryAttachment or
//     SerializeAttachment). Each is expected to contain "id", "name",
//     "contentType", "size", and optionally "isInline".
//
// Returns a formatted plain-text string. Returns "No attachments." when the
// slice is empty.
//
// Side effects: none.
func FormatAttachmentsText(atts []map[string]any) string {
	if len(atts) == 0 {
		return "No attachments."
	}
	var b strings.Builder
	for i, a := range atts {
		name, _ := a["name"].(string)
		if name == "" {
			name = "(Unnamed)"
		}
		fmt.Fprintf(&b, "%d. %s\n", i+1, name)
		if id, _ := a["id"].(string); id != "" {
			fmt.Fprintf(&b, "   ID: %s\n", id)
		}
		if ct, _ := a["contentType"].(string); ct != "" {
			fmt.Fprintf(&b, "   Content-Type: %s\n", ct)
		}
		fmt.Fprintf(&b, "   Size: %d bytes\n", toInt(a["size"]))
		if inline, ok := a["isInline"].(bool); ok && inline {
			b.WriteString("   Inline: true\n")
		}
	}
	fmt.Fprintf(&b, "\n%d attachment(s).", len(atts))
	return b.String()
}

// FormatMailFoldersText formats a slice of serialized mail folder maps into a
// numbered plain-text listing with unread and total item counts.
//
// Parameters:
//   - folders: slice of folder maps (from serializeMailFolder), each expected
//     to contain "displayName", "unreadItemCount", and "totalItemCount" keys.
//
// Returns a formatted plain-text string. Returns "No folders found." when the
// slice is nil or empty.
//
// Side effects: none.
func FormatMailFoldersText(folders []map[string]any) string {
	if len(folders) == 0 {
		return "No folders found."
	}

	var b strings.Builder
	for i, f := range folders {
		name, _ := f["displayName"].(string)
		if name == "" {
			name = "(Unnamed)"
		}
		unread := toInt(f["unreadItemCount"])
		total := toInt(f["totalItemCount"])
		fmt.Fprintf(&b, "%d. %s (%d unread, %d total)\n", i+1, name, unread, total)
	}

	fmt.Fprintf(&b, "\n%d folder(s) total.", len(folders))

	return b.String()
}

// toInt converts a numeric value from a map[string]any to int. Handles int32
// (from direct serialization) and float64 (from JSON round-trip).
//
// Parameters:
//   - v: a value that may be int32, float64, int, or other numeric type.
//
// Returns the integer value, or 0 for unsupported types.
//
// Side effects: none.
func toInt(v any) int {
	switch n := v.(type) {
	case int32:
		return int(n)
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return 0
}

// FormatAccountsText formats a slice of account maps into a numbered
// plain-text listing showing each account's label, User Principal Name (UPN)
// when available, authentication state, and auth_method (CR-0056).
//
// Format: "N. label — upn (state, auth_method)". When the UPN is empty the
// em-dash and UPN portion are omitted; when auth_method is empty only the
// state is shown inside the parentheses.
//
// Parameters:
//   - accounts: slice of account maps, each expected to contain "label"
//     (string), "authenticated" (bool), and optionally "email" (string) and
//     "auth_method" (string) keys.
//
// Returns a formatted plain-text string. Returns "No accounts registered."
// when the slice is nil or empty.
//
// Side effects: none.
func FormatAccountsText(accounts []map[string]any) string {
	if len(accounts) == 0 {
		return "No accounts registered."
	}

	var b strings.Builder
	for i, a := range accounts {
		label, _ := a["label"].(string)
		if label == "" {
			label = "(unnamed)"
		}
		authed, _ := a["authenticated"].(bool)
		state := "disconnected"
		if authed {
			state = "authenticated"
		}
		email, _ := a["email"].(string)
		method, _ := a["auth_method"].(string)
		parenthetical := state
		if method != "" {
			parenthetical = state + ", " + method
		}
		if email != "" {
			fmt.Fprintf(&b, "%d. %s — %s (%s)\n", i+1, label, email, parenthetical)
		} else {
			fmt.Fprintf(&b, "%d. %s (%s)\n", i+1, label, parenthetical)
		}
	}

	fmt.Fprintf(&b, "\n%d account(s) total.", len(accounts))

	return b.String()
}

// FormatAccountLine returns a formatted "Account: label (upn)" line suitable
// for appending to write-tool confirmation responses. UPN is always included
// when non-empty (after CR-0056, UPN is persisted so it should almost always
// be available). An optional disconnected-account advisory is appended on a
// trailing line so the LLM raises the broader account landscape rather than
// silently operating on the auto-selected account (CR-0056 FR-52 / AC-17).
//
// Parameters:
//   - label: the account label (e.g., "default", "work").
//   - email: the account email/UPN address; may be empty.
//   - advisory: optional advisory text produced by the AccountResolver when
//     auto-selection coexists with disconnected accounts. Empty when no
//     advisory applies.
//
// Returns a single- or two-line string, or the empty string when label is
// empty.
//
// Side effects: none.
func FormatAccountLine(label, email string, advisory ...string) string {
	if label == "" {
		return ""
	}
	var base string
	if email != "" {
		base = fmt.Sprintf("Account: %s (%s)", label, email)
	} else {
		base = "Account: " + label
	}
	for _, a := range advisory {
		if a != "" {
			return base + "\n" + a
		}
	}
	return base
}

// FormatStatusText formats a statusResponse struct into a human-readable
// plain-text summary showing server version, timezone, uptime, account list
// with authentication state, and feature flags. This is the text-mode output
// for the status tool; full configuration is available via output=summary or
// output=raw.
//
// Parameters:
//   - status: the statusResponse struct from the status tool handler.
//
// Returns a formatted plain-text string.
//
// Side effects: none.
func FormatStatusText(status statusResponse) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Server: outlook-local-mcp v%s\n", status.Version)
	fmt.Fprintf(&b, "Timezone: %s\n", status.Timezone)
	fmt.Fprintf(&b, "Uptime: %s\n", formatUptime(status.ServerUptimeSeconds))

	// Accounts section. Disconnected accounts are first-class entries and are
	// rendered with their persisted UPN and auth_method so the LLM and user
	// can see the full landscape (CR-0056 FR-31/FR-32).
	if len(status.Accounts) > 0 {
		b.WriteString("\nAccounts:\n")
		for _, acct := range status.Accounts {
			state := "disconnected"
			if acct.Authenticated {
				state = "authenticated"
			}
			switch {
			case acct.UPN != "" && acct.AuthMethod != "":
				fmt.Fprintf(&b, "  %s: %s — %s (%s)\n", acct.Label, state, acct.UPN, acct.AuthMethod)
			case acct.UPN != "":
				fmt.Fprintf(&b, "  %s: %s — %s\n", acct.Label, state, acct.UPN)
			case acct.AuthMethod != "":
				fmt.Fprintf(&b, "  %s: %s (%s)\n", acct.Label, state, acct.AuthMethod)
			default:
				fmt.Fprintf(&b, "  %s: %s\n", acct.Label, state)
			}
		}
	}

	// Features line.
	readOnly := "off"
	if status.Config.Features.ReadOnly {
		readOnly = "on"
	}
	mail := "off"
	if status.Config.Features.MailEnabled {
		mail = "on"
	}
	mailManage := "off"
	if status.Config.Features.MailManageEnabled {
		mailManage = "on"
	}
	fmt.Fprintf(&b, "\nFeatures: read-only=%s, mail=%s, mail-manage=%s, provenance=%s", readOnly, mail, mailManage, status.Config.Features.ProvenanceTag)

	return b.String()
}

// FormatAboutText formats a buildinfo.Info into a labelled single-screen
// plain-text rendering suitable for the LLM to read once and remember.
// The output stays under 24 lines as required by CR-0067 FR-9.
//
// Parameters:
//   - info: the buildinfo.Info snapshot from buildinfo.Snapshot.
//
// Returns a formatted plain-text string.
//
// Side effects: none.
func FormatAboutText(info buildinfo.Info) string {
	var b strings.Builder
	fmt.Fprintf(&b, "outlook-local-mcp %s\n", info.Version)
	fmt.Fprintf(&b, "  commit: %s\n", info.Commit)
	fmt.Fprintf(&b, "  built:  %s\n", info.BuildDate)
	fmt.Fprintf(&b, "  go:     %s\n", info.GoVersion)
	b.WriteString("Host\n")
	fmt.Fprintf(&b, "  os/arch:      %s/%s\n", info.OS, info.Arch)
	fmt.Fprintf(&b, "  runtime:      %s\n", info.Runtime)
	fmt.Fprintf(&b, "  distribution: %s\n", info.Distribution)
	fmt.Fprintf(&b, "  auth backend: %s\n", info.AuthBackend)
	b.WriteString("Links\n")
	fmt.Fprintf(&b, "  homepage: %s\n", info.Homepage)
	fmt.Fprintf(&b, "  issues:   %s\n", info.IssueTracker)
	fmt.Fprintf(&b, "  docs:     %s\n", info.DocsBase)
	return b.String()
}

// formatUptime converts seconds to a human-readable duration string
// (e.g., "3h 42m", "5m", "45s").
//
// Parameters:
//   - seconds: the uptime duration in seconds.
//
// Returns a human-readable duration string.
//
// Side effects: none.
func formatUptime(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// FormatWriteConfirmation formats a concise text confirmation for write tool
// responses (create, update, reschedule). The output includes the action verb,
// subject, event ID, display time, and optionally the location.
//
// Parameters:
//   - action: the action verb (e.g., "created", "updated", "rescheduled").
//   - subject: the event subject/title.
//   - eventID: the Graph API event ID.
//   - displayTime: the human-readable time range (e.g., "Wed Mar 25, 2:00 PM - 3:00 PM").
//   - location: the event location. When empty, the Location line is omitted.
//
// Returns a multi-line text confirmation that does not exceed 5 lines.
//
// Side effects: none.
func FormatWriteConfirmation(action, subject, eventID, displayTime, location string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Event %s: %q\n", action, subject)
	fmt.Fprintf(&b, "ID: %s\n", eventID)
	fmt.Fprintf(&b, "Time: %s", displayTime)
	if location != "" {
		fmt.Fprintf(&b, "\nLocation: %s", location)
	}
	return b.String()
}

// FormatContactMatchesText formats merged contacts-domain search matches into a
// numbered plain-text listing with a total count. Each entry states the source
// the match came from, because a saved contact and a person Graph inferred from
// correspondence carry different confidence and a caller choosing an address
// needs to see which it is looking at.
//
// Parameters:
//   - matches: slice of summary contact or person maps carrying "displayName",
//     "emailAddress", "id", and the "source" label.
//
// Returns a formatted plain-text string. Returns a stated no-match line when
// the slice is empty, naming what to try next.
//
// Side effects: none.
func FormatContactMatchesText(matches []map[string]any) string {
	if len(matches) == 0 {
		return "No contacts or people matched. Try a shorter query, a surname, or a company name."
	}

	var b strings.Builder
	for i, match := range matches {
		source, _ := match["source"].(string)
		fmt.Fprintf(&b, "%d. %s [%s]\n", i+1, contactDisplayLabel(match), source)
		if address, _ := match["emailAddress"].(string); address != "" {
			fmt.Fprintf(&b, "   Email: %s\n", address)
		}
		if id, _ := match["id"].(string); id != "" {
			fmt.Fprintf(&b, "   ID: %s\n", id)
		}
	}

	fmt.Fprintf(&b, "\n%d match(es) total.", len(matches))

	return b.String()
}

// FormatPeopleText formats relevance-ranked people into a numbered plain-text
// listing with a total count. The listing order is Graph's relevance order, so
// the position of an entry is itself information and is preserved.
//
// Parameters:
//   - people: slice of summary person maps carrying "displayName",
//     "emailAddress", and "id".
//
// Returns a formatted plain-text string. Returns "No people found." when the
// slice is empty.
//
// Side effects: none.
func FormatPeopleText(people []map[string]any) string {
	if len(people) == 0 {
		return "No people found."
	}

	var b strings.Builder
	for i, person := range people {
		fmt.Fprintf(&b, "%d. %s\n", i+1, contactDisplayLabel(person))
		if address, _ := person["emailAddress"].(string); address != "" {
			fmt.Fprintf(&b, "   Email: %s\n", address)
		}
		if id, _ := person["id"].(string); id != "" {
			fmt.Fprintf(&b, "   ID: %s\n", id)
		}
	}

	fmt.Fprintf(&b, "\n%d person/people total, most relevant first.", len(people))

	return b.String()
}

// FormatContactDetailText formats one saved contact as labelled fields. Every
// address the contact holds is listed, not only the leading one, because a
// contact commonly carries a work and a personal address and picking between
// them is the caller's decision.
//
// Parameters:
//   - contact: a summary contact map carrying "displayName", "id", and
//     "emailAddresses".
//
// Returns a formatted plain-text string.
//
// Side effects: none.
func FormatContactDetailText(contact map[string]any) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Contact: %s\n", contactDisplayLabel(contact))
	if id, _ := contact["id"].(string); id != "" {
		fmt.Fprintf(&b, "ID: %s\n", id)
	}
	writeContactAddressLines(&b, contact)
	return strings.TrimRight(b.String(), "\n")
}

// FormatPersonDetailText formats one relevance-ranked person as labelled
// fields. A person carries no per-address name in Graph, so the addresses are
// listed under the person's own display name and the relevance score of the
// leading address is stated, since it is the only confidence signal the
// resource offers.
//
// Parameters:
//   - person: a summary person map carrying "displayName", "id",
//     "emailAddresses", and "relevanceScore".
//
// Returns a formatted plain-text string.
//
// Side effects: none.
func FormatPersonDetailText(person map[string]any) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Person: %s\n", contactDisplayLabel(person))
	if id, _ := person["id"].(string); id != "" {
		fmt.Fprintf(&b, "ID: %s\n", id)
	}
	writeContactAddressLines(&b, person)
	if score, ok := person["relevanceScore"].(float64); ok {
		fmt.Fprintf(&b, "Relevance: %.2f\n", score)
	}
	return strings.TrimRight(b.String(), "\n")
}

// writeContactAddressLines writes one Email line per address a summary record
// holds, or a stated absence, so a record with no address reads as an answer
// rather than as a truncated one.
func writeContactAddressLines(b *strings.Builder, record map[string]any) {
	addresses, _ := record["emailAddresses"].([]string)
	if len(addresses) == 0 {
		b.WriteString("Email: (none recorded)\n")
		return
	}
	for _, address := range addresses {
		fmt.Fprintf(b, "Email: %s\n", address)
	}
}

// contactDisplayLabel returns the name a contacts-domain record should be
// listed under, falling back to its leading address and then to a stated
// placeholder, so a record Graph returned without a display name is still
// identifiable rather than rendered as a blank line.
func contactDisplayLabel(record map[string]any) string {
	if name, _ := record["displayName"].(string); name != "" {
		return name
	}
	if address, _ := record["emailAddress"].(string); address != "" {
		return address
	}
	return "(Unnamed)"
}

// FormatTeamsSearchHitsText formats ranked Teams search hits into a numbered
// plain-text listing with a total count. Hits arrive in relevance order and are
// listed in it, since the ranking is what the search adds over an enumeration.
// Each entry names the collection its message came from, because a chat hit and
// a channel hit are read back by different verbs taking different identifiers.
//
// Parameters:
//   - hits: slice of hit maps carrying "source", "from", "createdDateTime",
//     "bodyPreview", and whichever of "chatId", "teamId", and "channelId" the
//     hit's collection supplies.
//
// Returns a formatted plain-text string. Returns a stated no-result line when
// the slice is empty.
//
// Side effects: none.
func FormatTeamsSearchHitsText(hits []map[string]any) string {
	if len(hits) == 0 {
		return "No Teams messages matched."
	}

	var b strings.Builder
	for i, hit := range hits {
		fmt.Fprintf(&b, "%d. [%s] %s\n", i+1, teamsFieldOr(hit, "source", "unknown"), teamsMessageLabel(hit))
		if created, _ := hit["createdDateTime"].(string); created != "" {
			fmt.Fprintf(&b, "   Sent: %s\n", created)
		}
		if preview, _ := hit["bodyPreview"].(string); preview != "" {
			fmt.Fprintf(&b, "   %s\n", teamsSingleLine(preview))
		}
		writeTeamsIdentifierLines(&b, hit, "   ")
	}

	fmt.Fprintf(&b, "\n%d match(es) total.", len(hits))

	return b.String()
}

// FormatChatsText formats the signed-in user's chats into a numbered plain-text
// listing with a total count. A one-to-one chat commonly carries no topic, so
// the preview of its last message is listed under it: without that, a page of
// untitled chats is indistinguishable rows of identifiers.
//
// Parameters:
//   - chats: slice of summary chat maps carrying "topic", "chatType", "id",
//     "lastUpdatedDateTime", and "lastMessagePreview".
//
// Returns a formatted plain-text string. Returns a stated no-result line when
// the slice is empty.
//
// Side effects: none.
func FormatChatsText(chats []map[string]any) string {
	if len(chats) == 0 {
		return "No chats found."
	}

	var b strings.Builder
	for i, chat := range chats {
		fmt.Fprintf(&b, "%d. %s\n", i+1, teamsChatLabel(chat))
		if chatType, _ := chat["chatType"].(string); chatType != "" {
			fmt.Fprintf(&b, "   Type: %s\n", chatType)
		}
		if updated, _ := chat["lastUpdatedDateTime"].(string); updated != "" {
			fmt.Fprintf(&b, "   Last updated: %s\n", updated)
		}
		if preview, _ := chat["lastMessagePreview"].(string); preview != "" {
			fmt.Fprintf(&b, "   Latest: %s\n", teamsSingleLine(preview))
		}
		if id, _ := chat["id"].(string); id != "" {
			fmt.Fprintf(&b, "   ID: %s\n", id)
		}
	}

	fmt.Fprintf(&b, "\n%d chat(s) total.", len(chats))

	return b.String()
}

// FormatTeamsMessagesText formats a collection of Teams messages, whether chat
// messages, channel messages, or replies, into a numbered plain-text listing
// with a total count. One formatter serves all three because the reading task is
// the same, scanning a thread for the message worth opening; the identifier
// lines differ per shape and are written from whichever the record carries.
//
// Parameters:
//   - messages: slice of summary message maps carrying "from",
//     "createdDateTime", "bodyPreview", "id", and whichever of "chatId",
//     "teamId", "channelId", and "replyToId" apply.
//
// Returns a formatted plain-text string. Returns a stated no-result line when
// the slice is empty.
//
// Side effects: none.
func FormatTeamsMessagesText(messages []map[string]any) string {
	if len(messages) == 0 {
		return "No Teams messages found."
	}

	var b strings.Builder
	for i, msg := range messages {
		fmt.Fprintf(&b, "%d. %s\n", i+1, teamsMessageLabel(msg))
		if created, _ := msg["createdDateTime"].(string); created != "" {
			fmt.Fprintf(&b, "   Sent: %s\n", created)
		}
		if preview, _ := msg["bodyPreview"].(string); preview != "" {
			fmt.Fprintf(&b, "   %s\n", teamsSingleLine(preview))
		}
		if count, ok := msg["attachmentCount"].(int); ok && count > 0 {
			fmt.Fprintf(&b, "   Attachments: %d\n", count)
		}
		writeTeamsIdentifierLines(&b, msg, "   ")
	}

	fmt.Fprintf(&b, "\n%d message(s) total.", len(messages))

	return b.String()
}

// FormatTeamsMessageDetailText formats one Teams message as labelled fields. The
// whole body is written rather than a preview, because this formatter renders
// what the caller escalated to see.
//
// Parameters:
//   - msg: a message map carrying "from", "subject", "createdDateTime", and
//     either "body" or "bodyPreview", plus its locating identifiers.
//
// Returns a formatted plain-text string.
//
// Side effects: none.
func FormatTeamsMessageDetailText(msg map[string]any) string {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\n", teamsFieldOr(msg, "from", "(unknown sender)"))
	if subject, _ := msg["subject"].(string); subject != "" {
		fmt.Fprintf(&b, "Subject: %s\n", subject)
	}
	if created, _ := msg["createdDateTime"].(string); created != "" {
		fmt.Fprintf(&b, "Sent: %s\n", created)
	}
	if edited, _ := msg["lastEditedDateTime"].(string); edited != "" {
		fmt.Fprintf(&b, "Edited: %s\n", edited)
	}
	writeTeamsIdentifierLines(&b, msg, "")
	fmt.Fprintf(&b, "\n%s\n", teamsMessageBody(msg))
	if names := teamsAttachmentNames(msg); len(names) > 0 {
		fmt.Fprintf(&b, "\nAttachments: %s\n", strings.Join(names, ", "))
	}
	return strings.TrimRight(b.String(), "\n")
}

// FormatOnlineMeetingText formats one online meeting as labelled fields. The
// meeting identifier is stated on its own line because it is the key the
// transcript verbs take, and resolving it is the whole purpose of the verb this
// formatter renders.
//
// Parameters:
//   - meeting: a meeting map carrying "subject", "id", "startDateTime",
//     "endDateTime", and "joinWebUrl".
//
// Returns a formatted plain-text string.
//
// Side effects: none.
func FormatOnlineMeetingText(meeting map[string]any) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Meeting: %s\n", teamsFieldOr(meeting, "subject", "(no subject)"))
	if id, _ := meeting["id"].(string); id != "" {
		fmt.Fprintf(&b, "Meeting ID: %s\n", id)
	}
	if start, _ := meeting["startDateTime"].(string); start != "" {
		fmt.Fprintf(&b, "Start: %s\n", start)
	}
	if end, _ := meeting["endDateTime"].(string); end != "" {
		fmt.Fprintf(&b, "End: %s\n", end)
	}
	if organizer, _ := meeting["organizer"].(string); organizer != "" {
		fmt.Fprintf(&b, "Organizer: %s\n", organizer)
	}
	if join, _ := meeting["joinWebUrl"].(string); join != "" {
		fmt.Fprintf(&b, "Join URL: %s\n", join)
	}
	if allowed, ok := meeting["allowTranscription"].(bool); ok && !allowed {
		b.WriteString("Transcription: not enabled for this meeting\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// FormatTranscriptsText formats a meeting's transcripts into a numbered
// plain-text listing with a total count. No content is shown: the listing exists
// to choose which transcript to fetch, and each entry states the identifier that
// fetch is keyed by.
//
// Parameters:
//   - transcripts: slice of summary transcript maps carrying "id",
//     "createdDateTime", and "endDateTime".
//
// Returns a formatted plain-text string. Returns a stated no-result line naming
// why a transcribed meeting may still list none.
//
// Side effects: none.
func FormatTranscriptsText(transcripts []map[string]any) string {
	if len(transcripts) == 0 {
		return "No transcripts found for this meeting. A meeting has transcripts only if it was transcribed while it ran."
	}

	var b strings.Builder
	for i, transcript := range transcripts {
		fmt.Fprintf(&b, "%d. Transcript %s\n", i+1, teamsFieldOr(transcript, "id", "(no id)"))
		if created, _ := transcript["createdDateTime"].(string); created != "" {
			fmt.Fprintf(&b, "   Created: %s\n", created)
		}
		if end, _ := transcript["endDateTime"].(string); end != "" {
			fmt.Fprintf(&b, "   Ended: %s\n", end)
		}
	}

	fmt.Fprintf(&b, "\n%d transcript(s) total.", len(transcripts))

	return b.String()
}

// FormatTranscriptDetailText formats one transcript as labelled fields followed
// by its text. When the text was truncated the reader is told so explicitly,
// because a WEBVTT transcript cut mid-sentence otherwise reads as a complete
// record of a shorter meeting.
//
// Parameters:
//   - transcript: a transcript map carrying "id", "meetingId",
//     "createdDateTime", "contentTruncated", and either "content" or
//     "contentPreview".
//
// Returns a formatted plain-text string.
//
// Side effects: none.
func FormatTranscriptDetailText(transcript map[string]any) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Transcript: %s\n", teamsFieldOr(transcript, "id", "(no id)"))
	if meetingID, _ := transcript["meetingId"].(string); meetingID != "" {
		fmt.Fprintf(&b, "Meeting ID: %s\n", meetingID)
	}
	if created, _ := transcript["createdDateTime"].(string); created != "" {
		fmt.Fprintf(&b, "Created: %s\n", created)
	}

	text, _ := transcript["content"].(string)
	if text == "" {
		text, _ = transcript["contentPreview"].(string)
	}
	if text == "" {
		b.WriteString("\n(no transcript text returned)\n")
		return strings.TrimRight(b.String(), "\n")
	}

	fmt.Fprintf(&b, "\n%s\n", text)
	if truncated, _ := transcript["contentTruncated"].(bool); truncated {
		b.WriteString("\nThis is a preview. Request output=raw for the full transcript.\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// writeTeamsIdentifierLines writes an ID line for each locating identifier a
// Teams message record carries, skipping the ones its shape does not have, so a
// chat message is not padded with empty channel coordinates and a channel
// message is not padded with an empty chat one.
func writeTeamsIdentifierLines(b *strings.Builder, msg map[string]any, indent string) {
	for _, field := range []struct {
		key   string
		label string
	}{
		{"id", "Message ID"},
		{"replyToId", "Reply to"},
		{"chatId", "Chat ID"},
		{"teamId", "Team ID"},
		{"channelId", "Channel ID"},
	} {
		if value, _ := msg[field.key].(string); value != "" {
			fmt.Fprintf(b, "%s%s: %s\n", indent, field.label, value)
		}
	}
}

// teamsChatLabel returns the name a chat should be listed under, falling back
// from its topic to its type and then to a stated placeholder, so a chat with no
// topic is still identifiable rather than rendered as a blank line.
func teamsChatLabel(chat map[string]any) string {
	if topic, _ := chat["topic"].(string); topic != "" {
		return topic
	}
	if chatType, _ := chat["chatType"].(string); chatType != "" {
		return "(untitled " + chatType + " chat)"
	}
	return "(untitled chat)"
}

// teamsMessageLabel returns the heading a message is listed under: the sender,
// with the subject appended when the message has one, which a channel post
// commonly does and a chat message commonly does not.
func teamsMessageLabel(msg map[string]any) string {
	sender := teamsFieldOr(msg, "from", "(unknown sender)")
	if subject, _ := msg["subject"].(string); subject != "" {
		return sender + " - " + subject
	}
	return sender
}

// teamsMessageBody returns the text a detail rendering should show, preferring
// the full body and falling back to the preview, so the same formatter serves a
// raw-tier record and a summary-tier one.
func teamsMessageBody(msg map[string]any) string {
	if body, _ := msg["body"].(string); body != "" {
		return body
	}
	if preview, _ := msg["bodyPreview"].(string); preview != "" {
		return preview
	}
	return "(no message text)"
}

// teamsAttachmentNames reduces a raw-tier message's attachments to their names,
// which is all a plain-text rendering can usefully state about them.
func teamsAttachmentNames(msg map[string]any) []string {
	attachments, _ := msg["attachments"].([]map[string]any)
	names := make([]string, 0, len(attachments))
	for _, att := range attachments {
		if name, _ := att["name"].(string); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// teamsFieldOr returns a record's string field, or the given placeholder when it
// is absent or empty, so a missing value reads as a stated absence rather than
// leaving a label with nothing after it.
func teamsFieldOr(record map[string]any, key, placeholder string) string {
	if value, _ := record[key].(string); value != "" {
		return value
	}
	return placeholder
}

// teamsSingleLine collapses a message body onto one line for a listing entry.
// A Teams body carries newlines and HTML markup, and a multi-line entry inside a
// numbered list breaks the alignment the list's readability depends on.
func teamsSingleLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}
