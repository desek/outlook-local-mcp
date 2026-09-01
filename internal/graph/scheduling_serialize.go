package graph

import (
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// This file holds the serializers for the Graph scheduling models, the shapes
// returned by the findMeetingTimes and getSchedule actions. They live apart
// from serialize.go (calendar events) and mail_serialize.go (mail) because the
// scheduling models are a distinct family and each file stays small.
//
// @agents-index: Raw and summary serializers for the Graph scheduling models
// backing the calendar availability reads.

// SerializeMeetingTimeSuggestion extracts the full Graph representation of a
// meeting-time suggestion into a map suitable for the "raw" output tier. Every
// key is present regardless of whether Graph supplied a value, so the raw tier
// keeps the "including empty values" property the output-tier contract states.
//
// Parameters:
//   - suggestion: a suggestion returned by findMeetingTimes. Every getter may
//     return nil.
//
// Returns a map carrying the suggestion's confidence, order, organizer
// availability, suggestion reason, meeting time slot, per-attendee
// availability, and suggested locations.
//
// Side effects: none.
func SerializeMeetingTimeSuggestion(suggestion models.MeetingTimeSuggestionable) map[string]any {
	confidence := float64(0)
	if c := suggestion.GetConfidence(); c != nil {
		confidence = *c
	}

	order := int32(0)
	if o := suggestion.GetOrder(); o != nil {
		order = *o
	}

	organizerAvailability := ""
	if oa := suggestion.GetOrganizerAvailability(); oa != nil {
		organizerAvailability = oa.String()
	}

	attendeeAvailability := make([]map[string]any, 0, len(suggestion.GetAttendeeAvailability()))
	for _, aa := range suggestion.GetAttendeeAvailability() {
		entry := map[string]any{
			"attendee":     map[string]any{"address": "", "name": "", "type": ""},
			"availability": "",
		}
		if av := aa.GetAvailability(); av != nil {
			entry["availability"] = av.String()
		}
		if att := aa.GetAttendee(); att != nil {
			attendee := map[string]any{"address": "", "name": "", "type": ""}
			if ea := att.GetEmailAddress(); ea != nil {
				attendee["address"] = SafeStr(ea.GetAddress())
				attendee["name"] = SafeStr(ea.GetName())
			}
			if t := att.GetTypeEscaped(); t != nil {
				attendee["type"] = t.String()
			}
			entry["attendee"] = attendee
		}
		attendeeAvailability = append(attendeeAvailability, entry)
	}

	locations := make([]string, 0, len(suggestion.GetLocations()))
	for _, loc := range suggestion.GetLocations() {
		locations = append(locations, SafeStr(loc.GetDisplayName()))
	}

	return map[string]any{
		"confidence":            confidence,
		"order":                 order,
		"organizerAvailability": organizerAvailability,
		"suggestionReason":      SafeStr(suggestion.GetSuggestionReason()),
		"meetingTimeSlot":       serializeTimeSlot(suggestion.GetMeetingTimeSlot()),
		"attendeeAvailability":  attendeeAvailability,
		"locations":             locations,
	}
}

// SerializeSummaryMeetingTimeSuggestion extracts the deliberately chosen field
// set for the "summary" and "text" output tiers: the time slot, the
// confidence, the rank order, the organizer's availability, and the suggestion
// reason when Graph supplied one.
//
// The time slot is flattened under the same "meetingTimeSlot" key the raw tier
// uses, so every key a summary carries also exists in raw and the two tiers can
// be compared key by key. Nested under it, start and end are plain ISO 8601
// strings plus a localised displayTime, matching how event summaries flatten
// their nested Graph objects.
//
// Parameters:
//   - suggestion: a suggestion returned by findMeetingTimes. Every getter may
//     return nil.
//
// Returns the summary map. The "suggestionReason" key is omitted entirely when
// Graph supplied no reason, so an absent reason is not rendered as an empty
// field the reader must interpret.
//
// Side effects: none.
func SerializeSummaryMeetingTimeSuggestion(suggestion models.MeetingTimeSuggestionable) map[string]any {
	confidence := float64(0)
	if c := suggestion.GetConfidence(); c != nil {
		confidence = *c
	}

	order := int32(0)
	if o := suggestion.GetOrder(); o != nil {
		order = *o
	}

	organizerAvailability := ""
	if oa := suggestion.GetOrganizerAvailability(); oa != nil {
		organizerAvailability = oa.String()
	}

	start, startTZ, end, endTZ := "", "", "", ""
	if slot := suggestion.GetMeetingTimeSlot(); slot != nil {
		if s := slot.GetStart(); s != nil {
			start = SafeStr(s.GetDateTime())
			startTZ = SafeStr(s.GetTimeZone())
		}
		if e := slot.GetEnd(); e != nil {
			end = SafeStr(e.GetDateTime())
			endTZ = SafeStr(e.GetTimeZone())
		}
	}

	result := map[string]any{
		"meetingTimeSlot": map[string]string{
			"start":       start,
			"end":         end,
			"displayTime": FormatDisplayTime(start, end, startTZ, endTZ, false),
		},
		"confidence":            confidence,
		"order":                 order,
		"organizerAvailability": organizerAvailability,
	}

	if reason := SafeStr(suggestion.GetSuggestionReason()); reason != "" {
		result["suggestionReason"] = reason
	}

	return result
}

// serializeTimeSlot renders a Graph TimeSlot as nested dateTime and timeZone
// pairs, keeping the shape Graph itself returns. A nil slot still yields the
// full key structure with empty values, so the raw tier never varies its shape.
func serializeTimeSlot(slot models.TimeSlotable) map[string]any {
	startDateTime, startTimeZone, endDateTime, endTimeZone := "", "", "", ""
	if slot != nil {
		if s := slot.GetStart(); s != nil {
			startDateTime = SafeStr(s.GetDateTime())
			startTimeZone = SafeStr(s.GetTimeZone())
		}
		if e := slot.GetEnd(); e != nil {
			endDateTime = SafeStr(e.GetDateTime())
			endTimeZone = SafeStr(e.GetTimeZone())
		}
	}
	return map[string]any{
		"start": map[string]string{"dateTime": startDateTime, "timeZone": startTimeZone},
		"end":   map[string]string{"dateTime": endDateTime, "timeZone": endTimeZone},
	}
}
