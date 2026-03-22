package graph

import (
	"fmt"
	"time"
)

// dateTimeFormat is the Go time layout for human-readable date and time
// in the style "Wed Mar 19, 2:00 PM".
const dateTimeFormat = "Mon Jan 2, 3:04 PM"

// dateOnlyFormat is the Go time layout for date-only display
// in the style "Wed Mar 19".
const dateOnlyFormat = "Mon Jan 2"

// timeOnlyFormat is the Go time layout for time-only display
// in the style "2:00 PM".
const timeOnlyFormat = "3:04 PM"

// FormatDisplayTime produces a human-readable time string from event start/end
// datetime strings, their timezones, and the all-day flag. It operates entirely
// on the provided data without making any API calls.
//
// Formatting rules:
//   - Same-day events: "Wed Mar 19, 2:00 PM - 3:00 PM"
//   - Multi-day events: "Wed Mar 19, 2:00 PM - Thu Mar 20, 10:00 AM"
//   - All-day single-day: "Wed Mar 19 (all day)"
//   - All-day multi-day: "Wed Mar 19 - Thu Mar 20 (all day)"
//
// Parameters:
//   - startDateTime: ISO 8601 datetime string without offset (e.g. "2026-03-19T14:00:00").
//   - endDateTime: ISO 8601 datetime string without offset (e.g. "2026-03-19T15:00:00").
//   - startTimeZone: IANA timezone name for the start time (e.g. "Europe/Stockholm").
//     Falls back to UTC when empty or invalid.
//   - endTimeZone: IANA timezone name for the end time. Falls back to startTimeZone
//     when empty, then to UTC.
//   - isAllDay: whether the event is an all-day event.
//
// Returns the formatted display time string. Returns an empty string when both
// startDateTime and endDateTime are empty.
//
// Side effects: none.
func FormatDisplayTime(startDateTime, endDateTime, startTimeZone, endTimeZone string, isAllDay bool) string {
	if startDateTime == "" && endDateTime == "" {
		return ""
	}

	startLoc := loadLocation(startTimeZone)
	endLoc := startLoc
	if endTimeZone != "" && endTimeZone != startTimeZone {
		endLoc = loadLocation(endTimeZone)
	}

	startTime := parseDateTime(startDateTime, startLoc)
	endTime := parseDateTime(endDateTime, endLoc)

	if startTime.IsZero() && endTime.IsZero() {
		return ""
	}

	if isAllDay {
		return formatAllDay(startTime, endTime)
	}

	return formatTimedEvent(startTime, endTime)
}

// loadLocation loads a *time.Location from an IANA timezone name, falling back
// to UTC if the name is empty or unrecognized.
func loadLocation(tz string) *time.Location {
	if tz == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

// parseDateTime parses an ISO 8601 datetime string (without offset) into a
// time.Time in the given location. It tries multiple common Graph API formats
// to handle fractional seconds. Returns the zero time on failure.
func parseDateTime(dt string, loc *time.Location) time.Time {
	if dt == "" {
		return time.Time{}
	}

	layouts := []string{
		"2006-01-02T15:04:05.0000000",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, dt, loc); err == nil {
			return t
		}
	}
	return time.Time{}
}

// sameDay reports whether two times fall on the same calendar date.
func sameDay(a, b time.Time) bool {
	y1, m1, d1 := a.Date()
	y2, m2, d2 := b.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

// formatAllDay formats an all-day event. Single-day events render as
// "Wed Mar 19 (all day)". Multi-day events render as
// "Wed Mar 19 - Thu Mar 20 (all day)".
//
// For all-day events, the Graph API sets the end date to the day after the
// last day of the event. For example, a single all-day event on Mar 19 has
// end = Mar 20T00:00:00. We subtract one day from the end to get the last
// visible day.
func formatAllDay(start, end time.Time) string {
	// Adjust end: Graph API uses exclusive end date for all-day events.
	lastDay := end.AddDate(0, 0, -1)
	if sameDay(start, lastDay) || end.IsZero() {
		return fmt.Sprintf("%s (all day)", start.Format(dateOnlyFormat))
	}
	return fmt.Sprintf("%s - %s (all day)", start.Format(dateOnlyFormat), lastDay.Format(dateOnlyFormat))
}

// formatTimedEvent formats a timed (non-all-day) event. Same-day events render
// as "Wed Mar 19, 2:00 PM - 3:00 PM". Multi-day events render as
// "Wed Mar 19, 2:00 PM - Thu Mar 20, 10:00 AM".
func formatTimedEvent(start, end time.Time) string {
	if end.IsZero() {
		return start.Format(dateTimeFormat)
	}
	if sameDay(start, end) {
		return fmt.Sprintf("%s - %s", start.Format(dateTimeFormat), end.Format(timeOnlyFormat))
	}
	return fmt.Sprintf("%s - %s", start.Format(dateTimeFormat), end.Format(dateTimeFormat))
}
