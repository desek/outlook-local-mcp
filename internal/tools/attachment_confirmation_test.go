// Package tools tests.
//
// @agents-index: Tests for the draft and event attachment confirmation
// formatters, covering the rendered line order, the empty-subject placeholder,
// and that the identifiers reported are the ones passed in rather than any
// request-shaped substitute.
package tools

import (
	"strings"
	"testing"
)

// TestAttachmentConfirmationUsesResponseNotArguments asserts the confirmation
// reports the identifier it was given for the created attachment. The verb
// reads that identifier from the Graph response because the service mints it
// and the caller cannot observe it any other way, so a confirmation echoing an
// argument-derived value would name an attachment that may not exist.
func TestAttachmentConfirmationUsesResponseNotArguments(t *testing.T) {
	t.Parallel()
	const responseID = "AAMkAGI2-attachment-from-service"
	const requestName = "report.pdf"

	got := FormatAttachmentConfirmation(requestName, "Quarterly numbers", "AAMkAGI2-message", responseID, 2048, TransferDirect)

	if !strings.Contains(got, responseID) {
		t.Errorf("confirmation should report the service-assigned attachment ID %q, got:\n%s", responseID, got)
	}
	if !strings.Contains(got, requestName) {
		t.Errorf("confirmation should name the attachment %q, got:\n%s", requestName, got)
	}
	if !strings.Contains(got, "2048 bytes") {
		t.Errorf("confirmation should report the measured size in bytes, got:\n%s", got)
	}
	if !strings.Contains(got, TransferDirect) {
		t.Errorf("confirmation should name the transfer path, got:\n%s", got)
	}
}

// TestAttachmentConfirmationNoSubjectPlaceholder asserts a draft with no
// subject still renders a readable line, matching the placeholder the mail
// write confirmation already uses so the two do not disagree on how an absent
// subject reads.
func TestAttachmentConfirmationNoSubjectPlaceholder(t *testing.T) {
	t.Parallel()
	got := FormatAttachmentConfirmation("notes.txt", "", "AAMkAGI2-message", "AAMkAGI2-attachment", 12, TransferUploadSession)

	if !strings.Contains(got, "(No subject)") {
		t.Errorf("an empty subject should render as (No subject), got:\n%s", got)
	}
	if !strings.Contains(got, TransferUploadSession) {
		t.Errorf("confirmation should name the session transfer path, got:\n%s", got)
	}
}

// TestAttachmentConfirmationLineOrder pins the rendered line order. The order
// is a contract rather than a preference: it mirrors the mail write
// confirmation so a caller reading either sees the subject before the
// identifiers, and the assertions below fail on a reordering that would
// otherwise pass every substring check above.
func TestAttachmentConfirmationLineOrder(t *testing.T) {
	t.Parallel()
	lines := strings.Split(FormatAttachmentConfirmation("a.bin", "Draft subject", "msg-1", "att-1", 7, TransferDirect), "\n")

	want := []string{
		`Attachment added: "a.bin"`,
		`Message: "Draft subject"`,
		"Message ID: msg-1",
		"Attachment ID: att-1",
		"Size: 7 bytes",
		"Transfer: " + TransferDirect,
	}
	if len(lines) != len(want) {
		t.Fatalf("expected %d lines, got %d:\n%s", len(want), len(lines), strings.Join(lines, "\n"))
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("line %d: got %q, want %q", i+1, lines[i], w)
		}
	}
}

// TestFormatEventAttachmentConfirmation_NamesSubjectNameSizeIDAndPath pins the
// event confirmation's full contract on both transfer paths: the six lines, in
// order, naming the event rather than a message, and the placeholder an event
// with no subject renders as.
//
// The line order is asserted rather than only the substrings because a
// reordering passes every containment check while changing what a caller reads
// first, and the noun on lines two and three is what tells the caller which
// resource kind the identifier addresses.
func TestFormatEventAttachmentConfirmation_NamesSubjectNameSizeIDAndPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		subject  string
		transfer string
		want     []string
	}{
		{
			name:     "the direct path names the event and both identifiers",
			subject:  "Quarterly review",
			transfer: TransferDirect,
			want: []string{
				`Attachment added: "agenda.pdf"`,
				`Event: "Quarterly review"`,
				"Event ID: evt-1",
				"Attachment ID: att-from-service",
				"Size: 2048 bytes",
				"Transfer: " + TransferDirect,
			},
		},
		{
			name:     "an event with no subject renders the placeholder on the session path",
			subject:  "",
			transfer: TransferUploadSession,
			want: []string{
				`Attachment added: "agenda.pdf"`,
				`Event: "(No subject)"`,
				"Event ID: evt-1",
				"Attachment ID: att-from-service",
				"Size: 2048 bytes",
				"Transfer: " + TransferUploadSession,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := FormatEventAttachmentConfirmation(
				"agenda.pdf", tc.subject, "evt-1", "att-from-service", 2048, tc.transfer)
			lines := strings.Split(got, "\n")
			if len(lines) != len(tc.want) {
				t.Fatalf("expected %d lines, got %d:\n%s", len(tc.want), len(lines), got)
			}
			for i, w := range tc.want {
				if lines[i] != w {
					t.Errorf("line %d: got %q, want %q", i+1, lines[i], w)
				}
			}
			if strings.Contains(got, "Message") {
				t.Errorf("the event confirmation names a message rather than an event:\n%s", got)
			}
		})
	}
}
