// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the confirmation formatters for an attachment added to a
// draft or to a calendar event. The draft one is a third formatter rather than
// a call into either existing one:
// FormatDraftConfirmation (draft_helpers.go) closes with the Drafts-folder
// sentence and names a draft rather than an attachment, and
// FormatMailWriteConfirmation (mail_write_confirmation.go) carries exactly one
// field-and-value pair where this confirmation reports four. Four positional
// pairs is past where that shared signature stays readable, so the smaller
// change is a dedicated formatter following the same line order.
//
// @agents-index: Plain-text write confirmations for an attachment added to a
// draft or a calendar event, naming the attachment, its parent, both
// identifiers, the size, and the transfer path that carried it.
package tools

import (
	"fmt"
	"strings"
)

// Transfer path labels reported in an attachment confirmation. The caller
// cannot observe which path the verb chose, and the choice has a consequence it
// may need to reason about later (a chunked transfer is the one that can fail
// partway), so the confirmation names it rather than leaving it implicit.
const (
	// TransferDirect labels the single-POST path taken below the inline
	// threshold.
	TransferDirect = "direct upload"

	// TransferUploadSession labels the chunked upload-session path taken at or
	// above the inline threshold.
	TransferUploadSession = "chunked upload session"
)

// FormatAttachmentConfirmation formats a concise plain-text confirmation for an
// attachment added to a draft. Every value it reports about the created
// attachment is expected to come from the Graph response rather than from the
// request arguments, because the attachment identifier is minted by the service
// and is not otherwise observable to the caller.
//
// Parameters:
//   - name: the attachment file name.
//   - subject: the subject of the draft the attachment was added to. Rendered
//     as "(No subject)" when empty, matching FormatMailWriteConfirmation.
//   - messageID: the Graph API message ID of the draft.
//   - attachmentID: the service-assigned attachment ID.
//   - size: the attachment size in bytes, measured after base64 decoding.
//   - transfer: the transfer path label, one of TransferDirect or
//     TransferUploadSession.
//
// Returns a six-line text confirmation.
//
// Side effects: none.
func FormatAttachmentConfirmation(name, subject, messageID, attachmentID string, size int, transfer string) string {
	display := subject
	if display == "" {
		display = "(No subject)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Attachment added: %q\n", name)
	fmt.Fprintf(&b, "Message: %q\n", display)
	fmt.Fprintf(&b, "Message ID: %s\n", messageID)
	fmt.Fprintf(&b, "Attachment ID: %s\n", attachmentID)
	fmt.Fprintf(&b, "Size: %d bytes\n", size)
	fmt.Fprintf(&b, "Transfer: %s", transfer)
	return b.String()
}

// FormatEventAttachmentConfirmation formats the same confirmation for an
// attachment added to a calendar event. It is a sibling of
// FormatAttachmentConfirmation rather than a call into it because the two
// disagree on what the middle two lines name: a draft has a Message and a
// Message ID where an event has an Event and an Event ID, and a caller reading
// a confirmation that named the wrong resource kind would address the wrong
// thing next. The line order, the quoting, and the transfer labels are shared,
// so the two read identically apart from the noun.
//
// Parameters:
//   - name: the attachment file name.
//   - subject: the subject of the event the attachment was added to. Rendered
//     as "(No subject)" when empty, matching the draft confirmation.
//   - eventID: the Graph API event ID.
//   - attachmentID: the service-assigned attachment ID, read from the Graph
//     response because the service mints it and the caller cannot observe it
//     any other way.
//   - size: the attachment size in bytes, measured after base64 decoding.
//   - transfer: the transfer path label, one of TransferDirect or
//     TransferUploadSession.
//
// Returns a six-line text confirmation.
//
// Side effects: none.
func FormatEventAttachmentConfirmation(name, subject, eventID, attachmentID string, size int, transfer string) string {
	display := subject
	if display == "" {
		display = "(No subject)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Attachment added: %q\n", name)
	fmt.Fprintf(&b, "Event: %q\n", display)
	fmt.Fprintf(&b, "Event ID: %s\n", eventID)
	fmt.Fprintf(&b, "Attachment ID: %s\n", attachmentID)
	fmt.Fprintf(&b, "Size: %d bytes\n", size)
	fmt.Fprintf(&b, "Transfer: %s", transfer)
	return b.String()
}
