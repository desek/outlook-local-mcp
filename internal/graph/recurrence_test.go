package graph

import (
	"testing"

	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// TestBuildRecurrenceWeekly validates weekly recurrence pattern construction
// with an endDate range. The pattern should have WEEKLY type, one day (monday),
// and the range should have parsed startDate and endDate.
func TestBuildRecurrenceWeekly(t *testing.T) {
	jsonStr := `{
		"pattern": {"type":"weekly","interval":1,"daysOfWeek":["monday"],"firstDayOfWeek":"sunday"},
		"range": {"type":"endDate","startDate":"2026-04-15","endDate":"2026-12-31"}
	}`

	rec, err := BuildRecurrence(jsonStr)
	if err != nil {
		t.Fatalf("BuildRecurrence() error = %v", err)
	}
	if rec == nil {
		t.Fatal("BuildRecurrence() returned nil")
	}

	pattern := rec.GetPattern()
	if pattern == nil {
		t.Fatal("pattern is nil")
	}
	if pt := pattern.GetTypeEscaped(); pt == nil || *pt != models.WEEKLY_RECURRENCEPATTERNTYPE {
		t.Errorf("pattern type = %v, want WEEKLY", pt)
	}
	days := pattern.GetDaysOfWeek()
	if len(days) != 1 || days[0] != models.MONDAY_DAYOFWEEK {
		t.Errorf("daysOfWeek = %v, want [MONDAY]", days)
	}
	if fdow := pattern.GetFirstDayOfWeek(); fdow == nil || *fdow != models.SUNDAY_DAYOFWEEK {
		t.Errorf("firstDayOfWeek = %v, want SUNDAY", fdow)
	}

	recRange := rec.GetRangeEscaped()
	if recRange == nil {
		t.Fatal("range is nil")
	}
	if rt := recRange.GetTypeEscaped(); rt == nil || *rt != models.ENDDATE_RECURRENCERANGETYPE {
		t.Errorf("range type = %v, want ENDDATE", rt)
	}
	if sd := recRange.GetStartDate(); sd == nil || sd.String() != "2026-04-15" {
		t.Errorf("startDate = %v, want 2026-04-15", sd)
	}
	if ed := recRange.GetEndDate(); ed == nil || ed.String() != "2026-12-31" {
		t.Errorf("endDate = %v, want 2026-12-31", ed)
	}
}

// TestBuildRecurrenceDaily validates daily recurrence pattern construction with
// a numbered range.
func TestBuildRecurrenceDaily(t *testing.T) {
	jsonStr := `{
		"pattern": {"type":"daily","interval":2},
		"range": {"type":"numbered","startDate":"2026-04-15","numberOfOccurrences":10}
	}`

	rec, err := BuildRecurrence(jsonStr)
	if err != nil {
		t.Fatalf("BuildRecurrence() error = %v", err)
	}

	pattern := rec.GetPattern()
	if pt := pattern.GetTypeEscaped(); pt == nil || *pt != models.DAILY_RECURRENCEPATTERNTYPE {
		t.Errorf("pattern type = %v, want DAILY", pt)
	}
	if iv := pattern.GetInterval(); iv == nil || *iv != 2 {
		t.Errorf("interval = %v, want 2", iv)
	}

	recRange := rec.GetRangeEscaped()
	if rt := recRange.GetTypeEscaped(); rt == nil || *rt != models.NUMBERED_RECURRENCERANGETYPE {
		t.Errorf("range type = %v, want NUMBERED", rt)
	}
	if n := recRange.GetNumberOfOccurrences(); n == nil || *n != 10 {
		t.Errorf("numberOfOccurrences = %v, want 10", n)
	}
}

// TestBuildRecurrenceRelativeMonthly validates relative monthly pattern with
// index and daysOfWeek.
func TestBuildRecurrenceRelativeMonthly(t *testing.T) {
	jsonStr := `{
		"pattern": {"type":"relativeMonthly","interval":1,"daysOfWeek":["friday"],"index":"last"},
		"range": {"type":"noEnd","startDate":"2026-04-25"}
	}`

	rec, err := BuildRecurrence(jsonStr)
	if err != nil {
		t.Fatalf("BuildRecurrence() error = %v", err)
	}

	pattern := rec.GetPattern()
	if pt := pattern.GetTypeEscaped(); pt == nil || *pt != models.RELATIVEMONTHLY_RECURRENCEPATTERNTYPE {
		t.Errorf("pattern type = %v, want RELATIVEMONTHLY", pt)
	}
	days := pattern.GetDaysOfWeek()
	if len(days) != 1 || days[0] != models.FRIDAY_DAYOFWEEK {
		t.Errorf("daysOfWeek = %v, want [FRIDAY]", days)
	}
	if idx := pattern.GetIndex(); idx == nil || *idx != models.LAST_WEEKINDEX {
		t.Errorf("index = %v, want LAST", idx)
	}

	recRange := rec.GetRangeEscaped()
	if rt := recRange.GetTypeEscaped(); rt == nil || *rt != models.NOEND_RECURRENCERANGETYPE {
		t.Errorf("range type = %v, want NOEND", rt)
	}
}

// TestBuildRecurrenceInvalidJSON validates that BuildRecurrence returns an error
// when given malformed JSON.
func TestBuildRecurrenceInvalidJSON(t *testing.T) {
	_, err := BuildRecurrence("not valid json")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// TestBuildRecurrenceMissingPattern validates that BuildRecurrence returns an
// error when the pattern type is missing.
func TestBuildRecurrenceMissingPattern(t *testing.T) {
	jsonStr := `{"range": {"type":"noEnd","startDate":"2026-04-15"}}`
	_, err := BuildRecurrence(jsonStr)
	if err == nil {
		t.Fatal("expected error for missing pattern type, got nil")
	}
}

// TestBuildRecurrenceWeeklyMissingDaysOfWeek validates that BuildRecurrence
// returns an error when a weekly pattern is missing daysOfWeek.
func TestBuildRecurrenceWeeklyMissingDaysOfWeek(t *testing.T) {
	jsonStr := `{
		"pattern": {"type":"weekly","interval":1,"firstDayOfWeek":"sunday"},
		"range": {"type":"noEnd","startDate":"2026-04-15"}
	}`
	_, err := BuildRecurrence(jsonStr)
	if err == nil {
		t.Fatal("expected error for missing daysOfWeek, got nil")
	}
}
