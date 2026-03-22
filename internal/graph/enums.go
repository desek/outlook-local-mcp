package graph

import (
	"strings"

	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// ParseAttendeeType converts a case-insensitive string to the corresponding
// models.AttendeeType enum constant. Valid values are "required", "optional",
// and "resource". Unknown values default to REQUIRED_ATTENDEETYPE.
//
// Parameters:
//   - s: the attendee type string from user input.
//
// Returns the matching models.AttendeeType constant.
func ParseAttendeeType(s string) models.AttendeeType {
	switch strings.ToLower(s) {
	case "required":
		return models.REQUIRED_ATTENDEETYPE
	case "optional":
		return models.OPTIONAL_ATTENDEETYPE
	case "resource":
		return models.RESOURCE_ATTENDEETYPE
	default:
		return models.REQUIRED_ATTENDEETYPE
	}
}

// ParseImportance converts a case-insensitive string to the corresponding
// models.Importance enum constant. Valid values are "low", "normal", and "high".
// Unknown values default to NORMAL_IMPORTANCE.
//
// Parameters:
//   - s: the importance string from user input.
//
// Returns the matching models.Importance constant.
func ParseImportance(s string) models.Importance {
	switch strings.ToLower(s) {
	case "low":
		return models.LOW_IMPORTANCE
	case "normal":
		return models.NORMAL_IMPORTANCE
	case "high":
		return models.HIGH_IMPORTANCE
	default:
		return models.NORMAL_IMPORTANCE
	}
}

// ParseSensitivity converts a case-insensitive string to the corresponding
// models.Sensitivity enum constant. Valid values are "normal", "personal",
// "private", and "confidential". Unknown values default to NORMAL_SENSITIVITY.
//
// Parameters:
//   - s: the sensitivity string from user input.
//
// Returns the matching models.Sensitivity constant.
func ParseSensitivity(s string) models.Sensitivity {
	switch strings.ToLower(s) {
	case "normal":
		return models.NORMAL_SENSITIVITY
	case "personal":
		return models.PERSONAL_SENSITIVITY
	case "private":
		return models.PRIVATE_SENSITIVITY
	case "confidential":
		return models.CONFIDENTIAL_SENSITIVITY
	default:
		return models.NORMAL_SENSITIVITY
	}
}

// ParseShowAs converts a case-insensitive string to the corresponding
// models.FreeBusyStatus enum constant. Valid values are "free", "tentative",
// "busy", "oof", and "workingelsewhere". Unknown values default to
// BUSY_FREEBUSYSTATUS.
//
// Parameters:
//   - s: the free/busy status string from user input.
//
// Returns the matching models.FreeBusyStatus constant.
func ParseShowAs(s string) models.FreeBusyStatus {
	switch strings.ToLower(s) {
	case "free":
		return models.FREE_FREEBUSYSTATUS
	case "tentative":
		return models.TENTATIVE_FREEBUSYSTATUS
	case "busy":
		return models.BUSY_FREEBUSYSTATUS
	case "oof":
		return models.OOF_FREEBUSYSTATUS
	case "workingelsewhere":
		return models.WORKINGELSEWHERE_FREEBUSYSTATUS
	default:
		return models.BUSY_FREEBUSYSTATUS
	}
}

// parseRecurrencePatternType converts a case-insensitive string to the
// corresponding models.RecurrencePatternType enum constant. Valid values are
// "daily", "weekly", "absolutemonthly", "relativemonthly", "absoluteyearly",
// and "relativeyearly". Unknown values default to DAILY_RECURRENCEPATTERNTYPE.
//
// Parameters:
//   - s: the recurrence pattern type string from user input.
//
// Returns the matching models.RecurrencePatternType constant.
func parseRecurrencePatternType(s string) models.RecurrencePatternType {
	switch strings.ToLower(s) {
	case "daily":
		return models.DAILY_RECURRENCEPATTERNTYPE
	case "weekly":
		return models.WEEKLY_RECURRENCEPATTERNTYPE
	case "absolutemonthly":
		return models.ABSOLUTEMONTHLY_RECURRENCEPATTERNTYPE
	case "relativemonthly":
		return models.RELATIVEMONTHLY_RECURRENCEPATTERNTYPE
	case "absoluteyearly":
		return models.ABSOLUTEYEARLY_RECURRENCEPATTERNTYPE
	case "relativeyearly":
		return models.RELATIVEYEARLY_RECURRENCEPATTERNTYPE
	default:
		return models.DAILY_RECURRENCEPATTERNTYPE
	}
}

// parseDayOfWeek converts a case-insensitive string to the corresponding
// models.DayOfWeek enum constant. Valid values are "sunday" through "saturday".
// Unknown values default to SUNDAY_DAYOFWEEK.
//
// Parameters:
//   - s: the day of week string from user input.
//
// Returns the matching models.DayOfWeek constant.
func parseDayOfWeek(s string) models.DayOfWeek {
	switch strings.ToLower(s) {
	case "sunday":
		return models.SUNDAY_DAYOFWEEK
	case "monday":
		return models.MONDAY_DAYOFWEEK
	case "tuesday":
		return models.TUESDAY_DAYOFWEEK
	case "wednesday":
		return models.WEDNESDAY_DAYOFWEEK
	case "thursday":
		return models.THURSDAY_DAYOFWEEK
	case "friday":
		return models.FRIDAY_DAYOFWEEK
	case "saturday":
		return models.SATURDAY_DAYOFWEEK
	default:
		return models.SUNDAY_DAYOFWEEK
	}
}

// parseWeekIndex converts a case-insensitive string to the corresponding
// models.WeekIndex enum constant. Valid values are "first", "second", "third",
// "fourth", and "last". Unknown values default to FIRST_WEEKINDEX.
//
// Parameters:
//   - s: the week index string from user input.
//
// Returns the matching models.WeekIndex constant.
func parseWeekIndex(s string) models.WeekIndex {
	switch strings.ToLower(s) {
	case "first":
		return models.FIRST_WEEKINDEX
	case "second":
		return models.SECOND_WEEKINDEX
	case "third":
		return models.THIRD_WEEKINDEX
	case "fourth":
		return models.FOURTH_WEEKINDEX
	case "last":
		return models.LAST_WEEKINDEX
	default:
		return models.FIRST_WEEKINDEX
	}
}
