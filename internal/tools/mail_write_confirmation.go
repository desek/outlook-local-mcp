// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the confirmation formatter shared by the mail write verbs
// that act on received messages (move_message, set_flag, set_categories,
// mark_read). It is separate from FormatDraftConfirmation (draft_helpers.go)
// because that formatter closes with the Drafts-folder sentence, which is false
// for a received message, and it follows that helper's precedent of living
// beside the verbs it serves rather than in text_format.go.
//
// @agents-index: Shared plain-text write confirmation for received-message mail
// verbs, naming the action, subject, ID, and resulting field value.
package tools

import (
	"fmt"
	"strings"
)

// FormatMailWriteConfirmation formats a concise plain-text confirmation for a
// write against a received message. The output names the action, the subject
// (or "(No subject)" when empty), the message identifier, and the changed field
// with the value the service reported after the write, so the caller reads the
// resulting state rather than the requested one.
//
// Parameters:
//   - action: the past-tense action verb (e.g., "flagged", "moved", "updated").
//   - subject: the message subject. May be empty.
//   - messageID: the Graph API message ID the write applied to.
//   - field: the human-readable name of the changed field (e.g., "Flag status").
//   - value: the resulting value of that field, read from the Graph response.
//   - consequence: an optional trailing sentence stating a consequence the
//     caller cannot otherwise see (e.g., that the original identifier no longer
//     resolves after a move). Omitted when empty.
//
// Returns a multi-line text confirmation of at most four lines.
//
// Side effects: none.
func FormatMailWriteConfirmation(action, subject, messageID, field, value, consequence string) string {
	display := subject
	if display == "" {
		display = "(No subject)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Message %s: %q\n", action, display)
	fmt.Fprintf(&b, "ID: %s\n", messageID)
	fmt.Fprintf(&b, "%s: %s", field, value)
	if consequence != "" {
		fmt.Fprintf(&b, "\n%s", consequence)
	}
	return b.String()
}
