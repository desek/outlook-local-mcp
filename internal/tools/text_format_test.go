// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the plain-text formatters used by the "text"
// output mode on read tools.
package tools

import (
	"strings"
	"testing"
)

// TestFormatEventsText_MultipleEvents verifies that multiple events are
// formatted as a numbered list with displayTime, location, showAs, organizer,
// and a total count (FR-18).
func TestFormatEventsText_MultipleEvents(t *testing.T) {
	events := []map[string]any{
		{
			"subject":     "Team Sync",
			"displayTime": "Wed Mar 19, 2:00 PM - 3:00 PM",
			"location":    "Conference Room A",
			"showAs":      "busy",
			"organizer":   "Alice Smith",
		},
		{
			"subject":     "1:1 with Bob",
			"displayTime": "Wed Mar 19, 4:00 PM - 4:30 PM",
			"location":    "Microsoft Teams",
			"showAs":      "busy",
			"organizer":   "Bob Jones",
		},
		{
			"subject":     "All Hands",
			"displayTime": "Wed Mar 19, 5:00 PM - 6:00 PM",
			"location":    "",
			"showAs":      "tentative",
			"organizer":   "Carol Lee",
		},
	}

	result := FormatEventsText(events)

	// Check numbered listing.
	if !strings.Contains(result, "1. Team Sync") {
		t.Error("expected '1. Team Sync' in output")
	}
	if !strings.Contains(result, "2. 1:1 with Bob") {
		t.Error("expected '2. 1:1 with Bob' in output")
	}
	if !strings.Contains(result, "3. All Hands") {
		t.Error("expected '3. All Hands' in output")
	}

	// Check time/location/status detail line.
	if !strings.Contains(result, "Wed Mar 19, 2:00 PM - 3:00 PM | Conference Room A | Busy") {
		t.Error("expected formatted detail line for Team Sync")
	}

	// Check organizer.
	if !strings.Contains(result, "Organizer: Alice Smith") {
		t.Error("expected 'Organizer: Alice Smith' in output")
	}

	// Check total.
	if !strings.Contains(result, "3 event(s) total.") {
		t.Error("expected '3 event(s) total.' in output")
	}

	// Event with empty location should not include extra pipe.
	if strings.Contains(result, "| |") {
		t.Error("empty location should not produce '| |'")
	}
}

// TestFormatEventsText_Empty verifies that an empty event list returns
// "No events found." (FR-18).
func TestFormatEventsText_Empty(t *testing.T) {
	result := FormatEventsText(nil)
	if result != "No events found." {
		t.Errorf("result = %q, want %q", result, "No events found.")
	}

	result = FormatEventsText([]map[string]any{})
	if result != "No events found." {
		t.Errorf("result = %q, want %q", result, "No events found.")
	}
}

// TestFormatFreeBusyText verifies that busy periods are formatted as a
// numbered list with time ranges, status, and a total count (FR-18).
func TestFormatFreeBusyText(t *testing.T) {
	data := FreeBusyResponse{
		TimeRange: FreeBusyTimeRange{
			Start: "2026-03-19T00:00:00",
			End:   "2026-03-19T23:59:59",
		},
		BusyPeriods: []BusyPeriod{
			{
				Start:   "2026-03-19T14:00:00",
				End:     "2026-03-19T15:00:00",
				Status:  "busy",
				Subject: "Team Sync",
			},
			{
				Start:   "2026-03-19T16:00:00",
				End:     "2026-03-19T16:30:00",
				Status:  "tentative",
				Subject: "1:1 with Bob",
			},
		},
	}

	result := FormatFreeBusyText(data)

	if !strings.Contains(result, "1. Team Sync") {
		t.Error("expected '1. Team Sync' in output")
	}
	if !strings.Contains(result, "2. 1:1 with Bob") {
		t.Error("expected '2. 1:1 with Bob' in output")
	}
	if !strings.Contains(result, "2 busy period(s) total.") {
		t.Error("expected '2 busy period(s) total.' in output")
	}
	if !strings.Contains(result, "Busy") {
		t.Error("expected title-cased 'Busy' in output")
	}
}

// TestFormatFreeBusyText_Empty verifies that empty busy periods returns the
// expected message.
func TestFormatFreeBusyText_Empty(t *testing.T) {
	data := FreeBusyResponse{
		TimeRange:   FreeBusyTimeRange{Start: "2026-03-19T00:00:00", End: "2026-03-19T23:59:59"},
		BusyPeriods: []BusyPeriod{},
	}

	result := FormatFreeBusyText(data)
	if result != "No busy periods found." {
		t.Errorf("result = %q, want %q", result, "No busy periods found.")
	}
}

// TestFormatCalendarsText verifies that calendars are formatted as a numbered
// list with name, default/read-only tags, owner, and a total count.
func TestFormatCalendarsText(t *testing.T) {
	calendars := []map[string]any{
		{
			"name":              "Calendar",
			"isDefaultCalendar": true,
			"canEdit":           true,
			"owner":             map[string]string{"name": "Alice Smith", "address": "alice@example.com"},
		},
		{
			"name":              "Shared Calendar",
			"isDefaultCalendar": false,
			"canEdit":           false,
			"owner":             map[string]string{"name": "Bob Jones", "address": "bob@example.com"},
		},
	}

	result := FormatCalendarsText(calendars)

	if !strings.Contains(result, "1. Calendar (default)") {
		t.Error("expected '1. Calendar (default)' in output")
	}
	if !strings.Contains(result, "2. Shared Calendar (read-only)") {
		t.Error("expected '2. Shared Calendar (read-only)' in output")
	}
	if !strings.Contains(result, "Owner: Alice Smith") {
		t.Error("expected 'Owner: Alice Smith' in output")
	}
	if !strings.Contains(result, "2 calendar(s) total.") {
		t.Error("expected '2 calendar(s) total.' in output")
	}
}

// TestFormatCalendarsText_Empty verifies that an empty calendar list returns
// the expected message.
func TestFormatCalendarsText_Empty(t *testing.T) {
	result := FormatCalendarsText(nil)
	if result != "No calendars found." {
		t.Errorf("result = %q, want %q", result, "No calendars found.")
	}
}

// TestFormatWriteConfirmation verifies that write tool confirmations include
// action, subject, event ID, time, and location.
func TestFormatWriteConfirmation(t *testing.T) {
	result := FormatWriteConfirmation(
		"created",
		"Weekly Sync",
		"AAMkAGQ3...",
		"Wed Mar 25, 2:00 PM - 3:00 PM",
		"Conference Room A",
	)

	if !strings.Contains(result, `Event created: "Weekly Sync"`) {
		t.Error("expected action and subject line")
	}
	if !strings.Contains(result, "ID: AAMkAGQ3...") {
		t.Error("expected ID line")
	}
	if !strings.Contains(result, "Time: Wed Mar 25, 2:00 PM - 3:00 PM") {
		t.Error("expected time line")
	}
	if !strings.Contains(result, "Location: Conference Room A") {
		t.Error("expected location line")
	}

	// Must not exceed 5 lines.
	lines := strings.Split(result, "\n")
	if len(lines) > 5 {
		t.Errorf("response has %d lines, want at most 5", len(lines))
	}
}

// TestFormatWriteConfirmation_NoLocation verifies that the Location line is
// omitted when location is empty.
func TestFormatWriteConfirmation_NoLocation(t *testing.T) {
	result := FormatWriteConfirmation(
		"updated",
		"Sprint Planning",
		"AAMkABC1...",
		"Thu Mar 26, 10:00 AM - 11:00 AM",
		"",
	)

	if !strings.Contains(result, `Event updated: "Sprint Planning"`) {
		t.Error("expected action and subject line")
	}
	if !strings.Contains(result, "ID: AAMkABC1...") {
		t.Error("expected ID line")
	}
	if !strings.Contains(result, "Time: Thu Mar 26, 10:00 AM - 11:00 AM") {
		t.Error("expected time line")
	}
	if strings.Contains(result, "Location:") {
		t.Error("location line should be omitted when location is empty")
	}

	// Must not exceed 5 lines.
	lines := strings.Split(result, "\n")
	if len(lines) > 5 {
		t.Errorf("response has %d lines, want at most 5", len(lines))
	}
}

// TestFormatEventDetailText verifies that a single event detail is formatted
// with subject, time, location, organizer, status, attendees, and body preview.
func TestFormatEventDetailText(t *testing.T) {
	event := map[string]any{
		"subject":     "Team Sync",
		"displayTime": "Wed Mar 19, 2:00 PM - 3:00 PM",
		"location":    "Conference Room A",
		"organizer":   "Alice Smith",
		"showAs":      "busy",
		"attendees": []map[string]string{
			{"name": "Bob Jones", "response": "accepted"},
			{"name": "Carol Lee", "response": "tentativelyAccepted"},
		},
		"bodyPreview": "Weekly sync to discuss project updates.",
	}

	result := FormatEventDetailText(event)

	if !strings.Contains(result, "Team Sync") {
		t.Error("expected 'Team Sync' in output")
	}
	if !strings.Contains(result, "Time: Wed Mar 19, 2:00 PM - 3:00 PM") {
		t.Error("expected formatted time line")
	}
	if !strings.Contains(result, "Location: Conference Room A") {
		t.Error("expected location line")
	}
	if !strings.Contains(result, "Organizer: Alice Smith") {
		t.Error("expected organizer line")
	}
	if !strings.Contains(result, "Status: Busy") {
		t.Error("expected status line")
	}
	if !strings.Contains(result, "- Bob Jones (accepted)") {
		t.Error("expected attendee Bob Jones with response")
	}
	if !strings.Contains(result, "- Carol Lee (tentativelyAccepted)") {
		t.Error("expected attendee Carol Lee with response")
	}
	if !strings.Contains(result, "Weekly sync to discuss project updates.") {
		t.Error("expected body preview")
	}
}

// TestFormatMessagesText verifies that multiple messages are formatted as a
// numbered list with sender, date, status flags, body preview, and total count.
func TestFormatMessagesText(t *testing.T) {
	messages := []map[string]any{
		{
			"subject":          "Weekly Design Review",
			"from":             map[string]string{"name": "Alice", "address": "alice@contoso.com"},
			"receivedDateTime": "2026-03-16T15:45:00Z",
			"isRead":           true,
			"hasAttachments":   false,
			"bodyPreview":      "Hi team, please review the attached mockups before...",
		},
		{
			"subject":          "Sprint Planning Notes",
			"from":             map[string]string{"name": "Bob", "address": "bob@contoso.com"},
			"receivedDateTime": "2026-03-16T14:12:00Z",
			"isRead":           false,
			"hasAttachments":   true,
			"bodyPreview":      "Here are the notes from today's sprint planning...",
		},
	}

	result := FormatMessagesText(messages)

	// Check numbered listing.
	if !strings.Contains(result, "1. Weekly Design Review") {
		t.Error("expected '1. Weekly Design Review' in output")
	}
	if !strings.Contains(result, "2. Sprint Planning Notes") {
		t.Error("expected '2. Sprint Planning Notes' in output")
	}

	// Check sender.
	if !strings.Contains(result, "From: alice@contoso.com") {
		t.Error("expected 'From: alice@contoso.com' in output")
	}
	if !strings.Contains(result, "From: bob@contoso.com") {
		t.Error("expected 'From: bob@contoso.com' in output")
	}

	// Check date formatting.
	if !strings.Contains(result, "Mon Mar 16, 2026") {
		t.Error("expected formatted date in output")
	}

	// Check flags: first message is read (no flag), second is unread with attachments.
	if strings.Contains(result, "[Unread]\n   Preview: Hi team") {
		t.Error("first message should not have [Unread] flag")
	}
	if !strings.Contains(result, "[Unread] [Has attachments]") {
		t.Error("expected '[Unread] [Has attachments]' for second message")
	}

	// Check preview.
	if !strings.Contains(result, "Preview: Hi team, please review") {
		t.Error("expected body preview for first message")
	}

	// Check total.
	if !strings.Contains(result, "2 message(s) total.") {
		t.Error("expected '2 message(s) total.' in output")
	}
}

// TestFormatMessagesText_Empty verifies that a nil message list returns
// "No messages found."
func TestFormatMessagesText_Empty(t *testing.T) {
	result := FormatMessagesText(nil)
	if result != "No messages found." {
		t.Errorf("result = %q, want %q", result, "No messages found.")
	}

	result = FormatMessagesText([]map[string]any{})
	if result != "No messages found." {
		t.Errorf("result = %q, want %q", result, "No messages found.")
	}
}

// TestFormatMessageDetailText verifies that a single message detail is formatted
// with subject, from, to, date, importance, attachment indicator, and body preview.
func TestFormatMessageDetailText(t *testing.T) {
	message := map[string]any{
		"subject":          "Weekly Design Review",
		"from":             map[string]string{"name": "Alice", "address": "alice@contoso.com"},
		"toRecipients":     []map[string]string{{"name": "Team", "address": "team@contoso.com"}},
		"receivedDateTime": "2026-03-16T15:45:00Z",
		"importance":       "high",
		"hasAttachments":   true,
		"bodyPreview":      "Hi team, please review the attached mockups before Wednesday...",
	}

	result := FormatMessageDetailText(message)

	if !strings.Contains(result, "Weekly Design Review\n") {
		t.Error("expected subject as first line")
	}
	if !strings.Contains(result, "From: alice@contoso.com") {
		t.Error("expected From line")
	}
	if !strings.Contains(result, "To: team@contoso.com") {
		t.Error("expected To line")
	}
	if !strings.Contains(result, "Date: Mon Mar 16, 2026 3:45 PM") {
		t.Error("expected formatted date line")
	}
	if !strings.Contains(result, "Importance: high") {
		t.Error("expected importance line for high importance")
	}
	if !strings.Contains(result, "[Has attachments]") {
		t.Error("expected attachment indicator")
	}
	if !strings.Contains(result, "Hi team, please review the attached mockups before Wednesday...") {
		t.Error("expected body preview")
	}
}

// TestFormatMessageDetailText_NormalImportance verifies that normal importance
// is not displayed in the detail view.
func TestFormatMessageDetailText_NormalImportance(t *testing.T) {
	message := map[string]any{
		"subject":          "Test",
		"from":             map[string]string{"name": "Alice", "address": "alice@contoso.com"},
		"receivedDateTime": "2026-03-16T15:45:00Z",
		"importance":       "normal",
		"hasAttachments":   false,
	}

	result := FormatMessageDetailText(message)

	if strings.Contains(result, "Importance:") {
		t.Error("normal importance should not be displayed")
	}
	if strings.Contains(result, "[Has attachments]") {
		t.Error("attachment indicator should not appear when hasAttachments is false")
	}
}

// TestFormatMailFoldersText verifies that folders are formatted as a numbered
// list with unread and total counts, plus a total count.
func TestFormatMailFoldersText(t *testing.T) {
	folders := []map[string]any{
		{"displayName": "Inbox", "unreadItemCount": int32(3), "totalItemCount": int32(142)},
		{"displayName": "Sent Items", "unreadItemCount": int32(0), "totalItemCount": int32(89)},
		{"displayName": "Drafts", "unreadItemCount": int32(0), "totalItemCount": int32(2)},
	}

	result := FormatMailFoldersText(folders)

	if !strings.Contains(result, "1. Inbox (3 unread, 142 total)") {
		t.Errorf("expected '1. Inbox (3 unread, 142 total)', got:\n%s", result)
	}
	if !strings.Contains(result, "2. Sent Items (0 unread, 89 total)") {
		t.Error("expected '2. Sent Items (0 unread, 89 total)'")
	}
	if !strings.Contains(result, "3. Drafts (0 unread, 2 total)") {
		t.Error("expected '3. Drafts (0 unread, 2 total)'")
	}
	if !strings.Contains(result, "3 folder(s) total.") {
		t.Error("expected '3 folder(s) total.' in output")
	}
}

// TestFormatMailFoldersText_Empty verifies that a nil folder list returns
// "No folders found."
func TestFormatMailFoldersText_Empty(t *testing.T) {
	result := FormatMailFoldersText(nil)
	if result != "No folders found." {
		t.Errorf("result = %q, want %q", result, "No folders found.")
	}

	result = FormatMailFoldersText([]map[string]any{})
	if result != "No folders found." {
		t.Errorf("result = %q, want %q", result, "No folders found.")
	}
}

// TestFormatAccountsText verifies that accounts with no UPN or auth_method
// fall back to the label-and-state rendering and the total count is correct.
func TestFormatAccountsText(t *testing.T) {
	accounts := []map[string]any{
		{"label": "work", "authenticated": true},
		{"label": "personal", "authenticated": false},
	}

	result := FormatAccountsText(accounts)

	if !strings.Contains(result, "1. work (authenticated)") {
		t.Errorf("expected '1. work (authenticated)' in output, got:\n%s", result)
	}
	if !strings.Contains(result, "2. personal (disconnected)") {
		t.Errorf("expected '2. personal (disconnected)' in output, got:\n%s", result)
	}
	if !strings.Contains(result, "2 account(s) total.") {
		t.Error("expected '2 account(s) total.' in output")
	}
}

// TestFormatAccountsText_WithUPNAndMethod verifies the CR-0056 format
// "N. label — upn (state, auth_method)" for both authenticated and
// disconnected accounts.
func TestFormatAccountsText_WithUPNAndMethod(t *testing.T) {
	accounts := []map[string]any{
		{"label": "default", "authenticated": true, "email": "alice@contoso.com", "auth_method": "browser"},
		{"label": "work", "authenticated": false, "email": "bob@contoso.com", "auth_method": "device_code"},
	}

	result := FormatAccountsText(accounts)

	if !strings.Contains(result, "1. default — alice@contoso.com (authenticated, browser)") {
		t.Errorf("expected CR-0056 format line for default, got:\n%s", result)
	}
	if !strings.Contains(result, "2. work — bob@contoso.com (disconnected, device_code)") {
		t.Errorf("expected CR-0056 format line for work, got:\n%s", result)
	}
}

// TestFormatAccountsText_Empty verifies that a nil account list returns
// "No accounts registered."
func TestFormatAccountsText_Empty(t *testing.T) {
	result := FormatAccountsText(nil)
	if result != "No accounts registered." {
		t.Errorf("result = %q, want %q", result, "No accounts registered.")
	}

	result = FormatAccountsText([]map[string]any{})
	if result != "No accounts registered." {
		t.Errorf("result = %q, want %q", result, "No accounts registered.")
	}
}

// TestFormatStatusText verifies that the status tool text output includes
// version, timezone, uptime, account list, and feature flags.
func TestFormatStatusText(t *testing.T) {
	status := statusResponse{
		Version:             "1.2.0",
		Timezone:            "Europe/Stockholm",
		ServerUptimeSeconds: 13320, // 3h 42m
		Accounts: []statusAccount{
			{Label: "work", Authenticated: true},
			{Label: "personal", Authenticated: false},
		},
		Config: statusConfig{
			Features: statusConfigFeatures{
				ReadOnly:      false,
				MailEnabled:   true,
				ProvenanceTag: "mcp_created",
			},
		},
	}

	result := FormatStatusText(status)

	if !strings.Contains(result, "Server: outlook-local-mcp v1.2.0") {
		t.Error("expected server version line")
	}
	if !strings.Contains(result, "Timezone: Europe/Stockholm") {
		t.Error("expected timezone line")
	}
	if !strings.Contains(result, "Uptime: 3h 42m") {
		t.Error("expected uptime line")
	}
	if !strings.Contains(result, "work: authenticated") {
		t.Error("expected work account with authenticated state")
	}
	if !strings.Contains(result, "personal: disconnected") {
		t.Error("expected personal account with disconnected state")
	}
	if !strings.Contains(result, "Features: read-only=off, mail=on, mail-manage=off, provenance=mcp_created") {
		t.Error("expected features line")
	}
}

// TestFormatAccountsText_WithEmail verifies that accounts with an email field
// display the UPN alongside the label per CR-0056. Accounts with an empty
// email fall back to the label-only format, and disconnected accounts use the
// "disconnected" state wording.
func TestFormatAccountsText_WithEmail(t *testing.T) {
	accounts := []map[string]any{
		{"label": "work", "authenticated": true, "email": "work@example.com"},
		{"label": "personal", "authenticated": false, "email": ""},
	}

	result := FormatAccountsText(accounts)

	if !strings.Contains(result, "1. work — work@example.com (authenticated)") {
		t.Errorf("expected formatted account line with email, got:\n%s", result)
	}
	// Personal has no email — should still render label and disconnected state.
	if !strings.Contains(result, "2. personal (disconnected)") {
		t.Errorf("expected disconnected account line without email, got:\n%s", result)
	}
}

// TestFormatStatusText_WithUPN verifies the status text output renders each
// account's persisted UPN and auth_method when available (CR-0056 FR-31).
func TestFormatStatusText_WithUPN(t *testing.T) {
	status := statusResponse{
		Version:             "1.2.0",
		Timezone:            "UTC",
		ServerUptimeSeconds: 60,
		Accounts: []statusAccount{
			{Label: "default", Authenticated: true, UPN: "alice@contoso.com", AuthMethod: "browser"},
			{Label: "work", Authenticated: false, UPN: "bob@contoso.com", AuthMethod: "device_code"},
		},
	}

	result := FormatStatusText(status)

	if !strings.Contains(result, "default: authenticated — alice@contoso.com (browser)") {
		t.Errorf("expected default line with UPN and auth_method, got:\n%s", result)
	}
	if !strings.Contains(result, "work: disconnected — bob@contoso.com (device_code)") {
		t.Errorf("expected work line with UPN and auth_method, got:\n%s", result)
	}
}

// TestFormatAccountLine_IncludesDisconnectedAdvisory verifies that a non-empty
// advisory is appended on a new line, surfacing the wider account landscape
// (CR-0056 FR-52).
func TestFormatAccountLine_IncludesDisconnectedAdvisory(t *testing.T) {
	advisory := "Note: 'work' is disconnected; run account_login to reconnect."
	result := FormatAccountLine("default", "alice@contoso.com", advisory)
	want := "Account: default (alice@contoso.com)\n" + advisory
	if result != want {
		t.Errorf("FormatAccountLine = %q, want %q", result, want)
	}
}

// TestFormatAccountLine_WithEmail verifies the full label+email format.
func TestFormatAccountLine_WithEmail(t *testing.T) {
	result := FormatAccountLine("default", "user@example.com")
	want := "Account: default (user@example.com)"
	if result != want {
		t.Errorf("FormatAccountLine = %q, want %q", result, want)
	}
}

// TestFormatAccountLine_EmailOmitted verifies that email is omitted when empty.
func TestFormatAccountLine_EmailOmitted(t *testing.T) {
	result := FormatAccountLine("default", "")
	want := "Account: default"
	if result != want {
		t.Errorf("FormatAccountLine = %q, want %q", result, want)
	}
}

// TestFormatAccountLine_EmptyLabel verifies that an empty label returns "".
func TestFormatAccountLine_EmptyLabel(t *testing.T) {
	result := FormatAccountLine("", "user@example.com")
	if result != "" {
		t.Errorf("FormatAccountLine with empty label = %q, want empty string", result)
	}
}
