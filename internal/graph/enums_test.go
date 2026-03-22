package graph

import (
	"testing"

	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// TestParseAttendeeType validates attendee type parsing for all valid values
// and unknown input. Unknown values default to REQUIRED_ATTENDEETYPE.
func TestParseAttendeeType(t *testing.T) {
	tests := []struct {
		input string
		want  models.AttendeeType
	}{
		{"required", models.REQUIRED_ATTENDEETYPE},
		{"optional", models.OPTIONAL_ATTENDEETYPE},
		{"resource", models.RESOURCE_ATTENDEETYPE},
		{"Required", models.REQUIRED_ATTENDEETYPE},
		{"OPTIONAL", models.OPTIONAL_ATTENDEETYPE},
		{"unknown", models.REQUIRED_ATTENDEETYPE},
		{"", models.REQUIRED_ATTENDEETYPE},
	}
	for _, tt := range tests {
		got := ParseAttendeeType(tt.input)
		if got != tt.want {
			t.Errorf("ParseAttendeeType(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// TestParseImportance validates importance parsing for all valid values and
// empty/unknown input. Unknown values default to NORMAL_IMPORTANCE.
func TestParseImportance(t *testing.T) {
	tests := []struct {
		input string
		want  models.Importance
	}{
		{"low", models.LOW_IMPORTANCE},
		{"normal", models.NORMAL_IMPORTANCE},
		{"high", models.HIGH_IMPORTANCE},
		{"LOW", models.LOW_IMPORTANCE},
		{"High", models.HIGH_IMPORTANCE},
		{"", models.NORMAL_IMPORTANCE},
		{"unknown", models.NORMAL_IMPORTANCE},
	}
	for _, tt := range tests {
		got := ParseImportance(tt.input)
		if got != tt.want {
			t.Errorf("ParseImportance(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// TestParseSensitivity validates sensitivity parsing for all valid values.
// Unknown values default to NORMAL_SENSITIVITY.
func TestParseSensitivity(t *testing.T) {
	tests := []struct {
		input string
		want  models.Sensitivity
	}{
		{"normal", models.NORMAL_SENSITIVITY},
		{"personal", models.PERSONAL_SENSITIVITY},
		{"private", models.PRIVATE_SENSITIVITY},
		{"confidential", models.CONFIDENTIAL_SENSITIVITY},
		{"PRIVATE", models.PRIVATE_SENSITIVITY},
		{"unknown", models.NORMAL_SENSITIVITY},
	}
	for _, tt := range tests {
		got := ParseSensitivity(tt.input)
		if got != tt.want {
			t.Errorf("ParseSensitivity(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// TestParseShowAs validates free/busy status parsing for all valid values.
// Unknown values default to BUSY_FREEBUSYSTATUS.
func TestParseShowAs(t *testing.T) {
	tests := []struct {
		input string
		want  models.FreeBusyStatus
	}{
		{"free", models.FREE_FREEBUSYSTATUS},
		{"tentative", models.TENTATIVE_FREEBUSYSTATUS},
		{"busy", models.BUSY_FREEBUSYSTATUS},
		{"oof", models.OOF_FREEBUSYSTATUS},
		{"workingElsewhere", models.WORKINGELSEWHERE_FREEBUSYSTATUS},
		{"WORKINGELSEWHERE", models.WORKINGELSEWHERE_FREEBUSYSTATUS},
		{"unknown", models.BUSY_FREEBUSYSTATUS},
	}
	for _, tt := range tests {
		got := ParseShowAs(tt.input)
		if got != tt.want {
			t.Errorf("ParseShowAs(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// TestParseRecurrencePatternType validates recurrence pattern type parsing for
// all valid values. Unknown values default to DAILY_RECURRENCEPATTERNTYPE.
func TestParseRecurrencePatternType(t *testing.T) {
	tests := []struct {
		input string
		want  models.RecurrencePatternType
	}{
		{"daily", models.DAILY_RECURRENCEPATTERNTYPE},
		{"weekly", models.WEEKLY_RECURRENCEPATTERNTYPE},
		{"absoluteMonthly", models.ABSOLUTEMONTHLY_RECURRENCEPATTERNTYPE},
		{"relativeMonthly", models.RELATIVEMONTHLY_RECURRENCEPATTERNTYPE},
		{"absoluteYearly", models.ABSOLUTEYEARLY_RECURRENCEPATTERNTYPE},
		{"relativeYearly", models.RELATIVEYEARLY_RECURRENCEPATTERNTYPE},
		{"DAILY", models.DAILY_RECURRENCEPATTERNTYPE},
		{"unknown", models.DAILY_RECURRENCEPATTERNTYPE},
	}
	for _, tt := range tests {
		got := parseRecurrencePatternType(tt.input)
		if got != tt.want {
			t.Errorf("parseRecurrencePatternType(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// TestParseDayOfWeek validates day of week parsing for all seven days.
// Unknown values default to SUNDAY_DAYOFWEEK.
func TestParseDayOfWeek(t *testing.T) {
	tests := []struct {
		input string
		want  models.DayOfWeek
	}{
		{"sunday", models.SUNDAY_DAYOFWEEK},
		{"monday", models.MONDAY_DAYOFWEEK},
		{"tuesday", models.TUESDAY_DAYOFWEEK},
		{"wednesday", models.WEDNESDAY_DAYOFWEEK},
		{"thursday", models.THURSDAY_DAYOFWEEK},
		{"friday", models.FRIDAY_DAYOFWEEK},
		{"saturday", models.SATURDAY_DAYOFWEEK},
		{"MONDAY", models.MONDAY_DAYOFWEEK},
		{"unknown", models.SUNDAY_DAYOFWEEK},
	}
	for _, tt := range tests {
		got := parseDayOfWeek(tt.input)
		if got != tt.want {
			t.Errorf("parseDayOfWeek(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// TestParseWeekIndex validates week index parsing for all valid values.
// Unknown values default to FIRST_WEEKINDEX.
func TestParseWeekIndex(t *testing.T) {
	tests := []struct {
		input string
		want  models.WeekIndex
	}{
		{"first", models.FIRST_WEEKINDEX},
		{"second", models.SECOND_WEEKINDEX},
		{"third", models.THIRD_WEEKINDEX},
		{"fourth", models.FOURTH_WEEKINDEX},
		{"last", models.LAST_WEEKINDEX},
		{"LAST", models.LAST_WEEKINDEX},
		{"unknown", models.FIRST_WEEKINDEX},
	}
	for _, tt := range tests {
		got := parseWeekIndex(tt.input)
		if got != tt.want {
			t.Errorf("parseWeekIndex(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
