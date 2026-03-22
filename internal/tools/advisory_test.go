package tools

import (
	"strings"
	"testing"
)

// TestBuildAdvisory_NoAttendees verifies that no advisory is produced when the
// event has no attendees, regardless of other fields.
func TestBuildAdvisory_NoAttendees(t *testing.T) {
	result := buildAdvisory(false, false, false, false)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

// TestBuildAdvisory_AttendeesWithBodyAndLocation verifies that no advisory is
// produced when the event has attendees and all recommended fields are present.
func TestBuildAdvisory_AttendeesWithBodyAndLocation(t *testing.T) {
	result := buildAdvisory(true, true, true, false)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

// TestBuildAdvisory_AttendeesNoBody verifies that an advisory mentioning the
// missing description is produced when body is absent but location is present.
func TestBuildAdvisory_AttendeesNoBody(t *testing.T) {
	result := buildAdvisory(true, false, true, false)
	if result == "" {
		t.Fatal("expected non-empty advisory")
	}
	if !strings.Contains(result, "description") {
		t.Errorf("advisory should mention description, got %q", result)
	}
}

// TestBuildAdvisory_AttendeesNoLocation verifies that an advisory mentioning the
// missing location is produced when location is absent and is_online_meeting is
// not set.
func TestBuildAdvisory_AttendeesNoLocation(t *testing.T) {
	result := buildAdvisory(true, true, false, false)
	if result == "" {
		t.Fatal("expected non-empty advisory")
	}
	if !strings.Contains(result, "location") {
		t.Errorf("advisory should mention location, got %q", result)
	}
}

// TestBuildAdvisory_AttendeesNoLocationOnlineMeeting verifies that no advisory
// is produced for a missing location when is_online_meeting is true (Teams
// provides the meeting location).
func TestBuildAdvisory_AttendeesNoLocationOnlineMeeting(t *testing.T) {
	result := buildAdvisory(true, true, false, true)
	if result != "" {
		t.Errorf("expected empty string when online meeting covers location, got %q", result)
	}
}

// TestBuildAdvisory_AttendeesNoBothFields verifies that the advisory mentions
// both description and location when both are missing and is_online_meeting is
// not set.
func TestBuildAdvisory_AttendeesNoBothFields(t *testing.T) {
	result := buildAdvisory(true, false, false, false)
	if result == "" {
		t.Fatal("expected non-empty advisory")
	}
	if !strings.Contains(result, "description") {
		t.Errorf("advisory should mention description, got %q", result)
	}
	if !strings.Contains(result, "location") {
		t.Errorf("advisory should mention location, got %q", result)
	}
}
