// Package tools tests.
//
// @agents-index: Tests for FormatAttachmentConfirmation, covering the rendered
// line order, the empty-subject placeholder, and that the identifiers reported
// are the ones passed in rather than any request-shaped substitute.
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
