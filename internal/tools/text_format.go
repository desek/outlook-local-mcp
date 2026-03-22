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
		fmt.Fprintf(&b, "   %s - %s | %s\n", bp.Start, bp.End, strings.Title(bp.Status)) //nolint:staticcheck // strings.Title is sufficient for single-word enum values

		if i < len(data.BusyPeriods)-1 {
			b.WriteString("\n")
		}
	}

	fmt.Fprintf(&b, "\n%d busy period(s) total.", len(data.BusyPeriods))

	return b.String()
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
// plain-text listing showing each account's label and authentication state.
//
// Parameters:
//   - accounts: slice of account maps, each expected to contain "label"
//     (string) and "authenticated" (bool) keys.
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
		state := "not authenticated"
		if authed {
			state = "authenticated"
		}
		fmt.Fprintf(&b, "%d. %s (%s)\n", i+1, label, state)
	}

	fmt.Fprintf(&b, "\n%d account(s) total.", len(accounts))

	return b.String()
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

	// Accounts section.
	if len(status.Accounts) > 0 {
		b.WriteString("\nAccounts:\n")
		for _, acct := range status.Accounts {
			state := "not authenticated"
			if acct.Authenticated {
				state = "authenticated"
			}
			fmt.Fprintf(&b, "  %s: %s\n", acct.Label, state)
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
	fmt.Fprintf(&b, "\nFeatures: read-only=%s, mail=%s, provenance=%s", readOnly, mail, status.Config.Features.ProvenanceTag)

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
