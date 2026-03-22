package graph

import (
	"testing"
)

// TestFormatDisplayTime_SameDay validates that a same-day timed event formats
// as "Wed Mar 19, 2:00 PM - 3:00 PM" with date shown once and both times.
func TestFormatDisplayTime_SameDay(t *testing.T) {
	got := FormatDisplayTime(
		"2026-03-19T14:00:00", "2026-03-19T15:00:00",
		"Europe/Stockholm", "", false,
	)
	want := "Thu Mar 19, 2:00 PM - 3:00 PM"
	if got != want {
		t.Errorf("FormatDisplayTime same-day = %q, want %q", got, want)
	}
}

// TestFormatDisplayTime_MultiDay validates that a multi-day timed event
// includes both dates in the output.
func TestFormatDisplayTime_MultiDay(t *testing.T) {
	got := FormatDisplayTime(
		"2026-03-19T14:00:00", "2026-03-20T10:00:00",
		"Europe/Stockholm", "", false,
	)
	want := "Thu Mar 19, 2:00 PM - Fri Mar 20, 10:00 AM"
	if got != want {
		t.Errorf("FormatDisplayTime multi-day = %q, want %q", got, want)
	}
}

// TestFormatDisplayTime_AllDay validates that a single all-day event formats
// as "Wed Mar 19 (all day)" without clock times.
func TestFormatDisplayTime_AllDay(t *testing.T) {
	// Graph API: all-day event on Mar 19 has end = Mar 20T00:00:00 (exclusive).
	got := FormatDisplayTime(
		"2026-03-19T00:00:00", "2026-03-20T00:00:00",
		"Europe/Stockholm", "", true,
	)
	want := "Thu Mar 19 (all day)"
	if got != want {
		t.Errorf("FormatDisplayTime all-day = %q, want %q", got, want)
	}
}

// TestFormatDisplayTime_AllDayMultiDay validates that a multi-day all-day event
// formats as "Wed Mar 19 - Fri Mar 21 (all day)".
func TestFormatDisplayTime_AllDayMultiDay(t *testing.T) {
	// Graph API: 3-day event Mar 19-21 has end = Mar 22T00:00:00 (exclusive).
	got := FormatDisplayTime(
		"2026-03-19T00:00:00", "2026-03-22T00:00:00",
		"Europe/Stockholm", "", true,
	)
	want := "Thu Mar 19 - Sat Mar 21 (all day)"
	if got != want {
		t.Errorf("FormatDisplayTime all-day multi-day = %q, want %q", got, want)
	}
}

// TestFormatDisplayTime_EmptyInputs validates that empty start and end
// datetimes return an empty string.
func TestFormatDisplayTime_EmptyInputs(t *testing.T) {
	got := FormatDisplayTime("", "", "", "", false)
	if got != "" {
		t.Errorf("FormatDisplayTime empty = %q, want %q", got, "")
	}
}

// TestFormatDisplayTime_FallbackUTC validates that an invalid or empty
// timezone falls back to UTC without error.
func TestFormatDisplayTime_FallbackUTC(t *testing.T) {
	got := FormatDisplayTime(
		"2026-03-19T14:00:00", "2026-03-19T15:00:00",
		"", "", false,
	)
	want := "Thu Mar 19, 2:00 PM - 3:00 PM"
	if got != want {
		t.Errorf("FormatDisplayTime UTC fallback = %q, want %q", got, want)
	}
}

// TestFormatDisplayTime_FractionalSeconds validates that Graph API datetime
// strings with fractional seconds (e.g. "2026-03-19T14:00:00.0000000") are
// parsed correctly.
func TestFormatDisplayTime_FractionalSeconds(t *testing.T) {
	got := FormatDisplayTime(
		"2026-03-19T14:00:00.0000000", "2026-03-19T15:00:00.0000000",
		"Europe/Stockholm", "", false,
	)
	want := "Thu Mar 19, 2:00 PM - 3:00 PM"
	if got != want {
		t.Errorf("FormatDisplayTime fractional = %q, want %q", got, want)
	}
}
