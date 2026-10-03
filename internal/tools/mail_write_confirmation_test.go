// Package tools tests.
//
// @agents-index: Tests for FormatMailWriteConfirmation, covering every rendered
// field, the no-subject fallback, and the optional consequence line.
package tools

import (
	"strings"
	"testing"
)

// TestFormatMailWriteConfirmationFields asserts the shared formatter renders
// every field a write confirmation must carry: the action, the subject or its
// "(No subject)" fallback, the message identifier, and the changed field with
// its resulting value, plus the optional consequence line when supplied.
func TestFormatMailWriteConfirmationFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		action      string
		subject     string
		messageID   string
		field       string
		value       string
		consequence string
		wantContain []string
		wantAbsent  []string
	}{
		{
			name:        "all fields present",
			action:      "flagged",
			subject:     "Quarterly report",
			messageID:   "AAMkAGI2",
			field:       "Flag status",
			value:       "flagged",
			wantContain: []string{"flagged", "Quarterly report", "ID: AAMkAGI2", "Flag status: flagged"},
			wantAbsent:  []string{"(No subject)"},
		},
		{
			name:        "empty subject falls back",
			action:      "updated",
			subject:     "",
			messageID:   "AAMkAGI3",
			field:       "Categories",
			value:       "Project, Urgent",
			wantContain: []string{"(No subject)", "ID: AAMkAGI3", "Categories: Project, Urgent"},
		},
		{
			name:        "consequence line appended",
			action:      "moved",
			subject:     "Invoice",
			messageID:   "AAMkAGI5",
			field:       "Destination folder",
			value:       "AQMkAD00",
			consequence: "The original message ID no longer resolves.",
			wantContain: []string{"moved", "Invoice", "ID: AAMkAGI5", "Destination folder: AQMkAD00", "The original message ID no longer resolves."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatMailWriteConfirmation(tt.action, tt.subject, tt.messageID, tt.field, tt.value, tt.consequence)
			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("confirmation should contain %q, got:\n%s", want, got)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("confirmation should not contain %q, got:\n%s", absent, got)
				}
			}
		})
	}
}

// TestFormatMailWriteConfirmationOmitsEmptyConsequence asserts the optional
// trailing line is dropped rather than rendered blank when no consequence
// applies, keeping a property-write confirmation to three lines.
func TestFormatMailWriteConfirmationOmitsEmptyConsequence(t *testing.T) {
	t.Parallel()

	got := FormatMailWriteConfirmation("marked", "Standup notes", "AAMkAGI4", "Read state", "read", "")
	if lines := strings.Split(got, "\n"); len(lines) != 3 {
		t.Errorf("expected 3 lines without a consequence, got %d:\n%s", len(lines), got)
	}
	if strings.HasSuffix(got, "\n") {
		t.Errorf("confirmation should not end with a newline, got:\n%q", got)
	}
}
