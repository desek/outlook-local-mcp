package graph

import (
	"encoding/json"
	"fmt"

	"github.com/microsoft/kiota-abstractions-go/serialization"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// RecurrenceInput represents the JSON structure accepted by the recurrence
// parameter. It mirrors the Microsoft Graph API recurrence object with two
// sub-objects: pattern and range.
type RecurrenceInput struct {
	Pattern PatternInput `json:"pattern"`
	Range   RangeInput   `json:"range"`
}

// PatternInput represents the pattern sub-object of a recurrence
// definition. Fields are optional; required fields depend on the pattern type.
type PatternInput struct {
	Type           string   `json:"type"`
	Interval       int32    `json:"interval"`
	DaysOfWeek     []string `json:"daysOfWeek"`
	FirstDayOfWeek string   `json:"firstDayOfWeek"`
	DayOfMonth     int32    `json:"dayOfMonth"`
	Month          int32    `json:"month"`
	Index          string   `json:"index"`
}

// RangeInput represents the range sub-object of a recurrence
// definition. The StartDate field is required for all range types.
type RangeInput struct {
	Type                string `json:"type"`
	StartDate           string `json:"startDate"`
	EndDate             string `json:"endDate"`
	NumberOfOccurrences int32  `json:"numberOfOccurrences"`
}

// BuildRecurrence parses a JSON string into a models.PatternedRecurrence SDK
// object. It validates required properties for the given pattern type and range
// type, returning a descriptive error if any required property is missing.
//
// Parameters:
//   - jsonStr: a JSON string representing the recurrence definition with
//     "pattern" and "range" sub-objects.
//
// Returns the constructed *models.PatternedRecurrence, or an error if the JSON
// is malformed, required properties are missing, or date parsing fails.
//
// Side effects: none.
func BuildRecurrence(jsonStr string) (*models.PatternedRecurrence, error) {
	var input RecurrenceInput
	if err := json.Unmarshal([]byte(jsonStr), &input); err != nil {
		return nil, fmt.Errorf("invalid recurrence JSON: %w", err)
	}

	if input.Pattern.Type == "" {
		return nil, fmt.Errorf("recurrence pattern.type is required")
	}
	if input.Range.Type == "" {
		return nil, fmt.Errorf("recurrence range.type is required")
	}

	// Validate pattern properties per type.
	if err := validatePatternProperties(&input.Pattern); err != nil {
		return nil, err
	}

	// Build SDK pattern.
	pattern, err := buildPattern(&input.Pattern)
	if err != nil {
		return nil, err
	}

	// Build SDK range.
	recRange, err := buildRange(&input.Range)
	if err != nil {
		return nil, err
	}

	result := models.NewPatternedRecurrence()
	result.SetPattern(pattern)
	result.SetRangeEscaped(recRange)
	return result, nil
}

// validatePatternProperties checks that required properties are present for the
// given pattern type. Returns an error describing the first missing property.
//
// Parameters:
//   - p: the parsed recurrence pattern input.
//
// Returns nil if all required properties are present, or a descriptive error.
func validatePatternProperties(p *PatternInput) error {
	switch parseRecurrencePatternType(p.Type) {
	case models.DAILY_RECURRENCEPATTERNTYPE:
		// Only interval required (always present as zero-value default).
	case models.WEEKLY_RECURRENCEPATTERNTYPE:
		if len(p.DaysOfWeek) == 0 {
			return fmt.Errorf("weekly pattern requires daysOfWeek")
		}
		if p.FirstDayOfWeek == "" {
			return fmt.Errorf("weekly pattern requires firstDayOfWeek")
		}
	case models.ABSOLUTEMONTHLY_RECURRENCEPATTERNTYPE:
		if p.DayOfMonth == 0 {
			return fmt.Errorf("absoluteMonthly pattern requires dayOfMonth")
		}
	case models.RELATIVEMONTHLY_RECURRENCEPATTERNTYPE:
		if len(p.DaysOfWeek) == 0 {
			return fmt.Errorf("relativeMonthly pattern requires daysOfWeek")
		}
		if p.Index == "" {
			return fmt.Errorf("relativeMonthly pattern requires index")
		}
	case models.ABSOLUTEYEARLY_RECURRENCEPATTERNTYPE:
		if p.DayOfMonth == 0 {
			return fmt.Errorf("absoluteYearly pattern requires dayOfMonth")
		}
		if p.Month == 0 {
			return fmt.Errorf("absoluteYearly pattern requires month")
		}
	case models.RELATIVEYEARLY_RECURRENCEPATTERNTYPE:
		if len(p.DaysOfWeek) == 0 {
			return fmt.Errorf("relativeYearly pattern requires daysOfWeek")
		}
		if p.Month == 0 {
			return fmt.Errorf("relativeYearly pattern requires month")
		}
		if p.Index == "" {
			return fmt.Errorf("relativeYearly pattern requires index")
		}
	}
	return nil
}

// buildPattern constructs a models.RecurrencePattern from the parsed input.
// It sets properties conditionally based on the pattern type.
//
// Parameters:
//   - p: the validated recurrence pattern input.
//
// Returns the constructed RecurrencePattern, or an error (currently none).
func buildPattern(p *PatternInput) (models.RecurrencePatternable, error) {
	pattern := models.NewRecurrencePattern()
	pt := parseRecurrencePatternType(p.Type)
	pattern.SetTypeEscaped(&pt)

	interval := p.Interval
	if interval == 0 {
		interval = 1
	}
	pattern.SetInterval(&interval)

	// Set daysOfWeek if provided.
	if len(p.DaysOfWeek) > 0 {
		days := make([]models.DayOfWeek, len(p.DaysOfWeek))
		for i, d := range p.DaysOfWeek {
			days[i] = parseDayOfWeek(d)
		}
		pattern.SetDaysOfWeek(days)
	}

	// Set firstDayOfWeek if provided.
	if p.FirstDayOfWeek != "" {
		fdow := parseDayOfWeek(p.FirstDayOfWeek)
		pattern.SetFirstDayOfWeek(&fdow)
	}

	// Set dayOfMonth if non-zero.
	if p.DayOfMonth != 0 {
		dom := p.DayOfMonth
		pattern.SetDayOfMonth(&dom)
	}

	// Set month if non-zero.
	if p.Month != 0 {
		m := p.Month
		pattern.SetMonth(&m)
	}

	// Set index if provided.
	if p.Index != "" {
		idx := parseWeekIndex(p.Index)
		pattern.SetIndex(&idx)
	}

	return pattern, nil
}

// buildRange constructs a models.RecurrenceRange from the parsed input.
// Date-only fields (startDate, endDate) are parsed via serialization.ParseDateOnly.
//
// Parameters:
//   - r: the validated recurrence range input.
//
// Returns the constructed RecurrenceRange, or an error if date parsing fails.
func buildRange(r *RangeInput) (models.RecurrenceRangeable, error) {
	recRange := models.NewRecurrenceRange()

	// Parse range type.
	switch r.Type {
	case "endDate":
		rt := models.ENDDATE_RECURRENCERANGETYPE
		recRange.SetTypeEscaped(&rt)
	case "noEnd":
		rt := models.NOEND_RECURRENCERANGETYPE
		recRange.SetTypeEscaped(&rt)
	case "numbered":
		rt := models.NUMBERED_RECURRENCERANGETYPE
		recRange.SetTypeEscaped(&rt)
	}

	// Parse and set startDate.
	if r.StartDate != "" {
		sd, err := serialization.ParseDateOnly(r.StartDate)
		if err != nil {
			return nil, fmt.Errorf("invalid startDate %q: %w", r.StartDate, err)
		}
		recRange.SetStartDate(sd)
	}

	// Parse and set endDate for endDate range type.
	if r.EndDate != "" {
		ed, err := serialization.ParseDateOnly(r.EndDate)
		if err != nil {
			return nil, fmt.Errorf("invalid endDate %q: %w", r.EndDate, err)
		}
		recRange.SetEndDate(ed)
	}

	// Set numberOfOccurrences for numbered range type.
	if r.NumberOfOccurrences > 0 {
		n := r.NumberOfOccurrences
		recRange.SetNumberOfOccurrences(&n)
	}

	return recRange, nil
}
