package graph

import (
	"testing"

	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// @agents-index: Tests for the scheduling serializers, pinning the raw tier's
// full-shape guarantee and the summary tier's chosen field set.

// newTestSuggestion builds a fully populated meeting-time suggestion so the two
// tiers can be compared against the same source object.
func newTestSuggestion(reason string) models.MeetingTimeSuggestionable {
	start := models.NewDateTimeTimeZone()
	startDT := "2026-03-12T09:00:00"
	tz := "UTC"
	start.SetDateTime(&startDT)
	start.SetTimeZone(&tz)

	end := models.NewDateTimeTimeZone()
	endDT := "2026-03-12T09:30:00"
	end.SetDateTime(&endDT)
	end.SetTimeZone(&tz)

	slot := models.NewTimeSlot()
	slot.SetStart(start)
	slot.SetEnd(end)

	email := models.NewEmailAddress()
	addr := "attendee@example.com"
	name := "Attendee One"
	email.SetAddress(&addr)
	email.SetName(&name)

	attendee := models.NewAttendeeBase()
	attendee.SetEmailAddress(email)
	attType := models.REQUIRED_ATTENDEETYPE
	attendee.SetTypeEscaped(&attType)

	availability := models.NewAttendeeAvailability()
	availability.SetAttendee(attendee)
	free := models.FREE_FREEBUSYSTATUS
	availability.SetAvailability(&free)

	suggestion := models.NewMeetingTimeSuggestion()
	suggestion.SetMeetingTimeSlot(slot)
	confidence := 100.0
	suggestion.SetConfidence(&confidence)
	order := int32(1)
	suggestion.SetOrder(&order)
	organizerAvailability := models.FREE_FREEBUSYSTATUS
	suggestion.SetOrganizerAvailability(&organizerAvailability)
	suggestion.SetAttendeeAvailability([]models.AttendeeAvailabilityable{availability})
	if reason != "" {
		suggestion.SetSuggestionReason(&reason)
	}

	return suggestion
}

// TestSerializeMeetingTimeSuggestionRawKeepsEveryKey checks that the raw tier
// keeps its full shape even when the source object supplies nothing, which is
// what makes raw comparable across responses.
func TestSerializeMeetingTimeSuggestionRawKeepsEveryKey(t *testing.T) {
	raw := SerializeMeetingTimeSuggestion(models.NewMeetingTimeSuggestion())

	for _, key := range []string{
		"confidence", "order", "organizerAvailability", "suggestionReason",
		"meetingTimeSlot", "attendeeAvailability", "locations",
	} {
		if _, ok := raw[key]; !ok {
			t.Errorf("raw serialization is missing key %q on an empty suggestion", key)
		}
	}

	slot, ok := raw["meetingTimeSlot"].(map[string]any)
	if !ok {
		t.Fatalf("meetingTimeSlot = %T, want map[string]any", raw["meetingTimeSlot"])
	}
	if _, ok := slot["start"]; !ok {
		t.Error("meetingTimeSlot is missing the start key on an empty suggestion")
	}
}

// TestSuggestionTiersDifferInFieldSet checks that the summary tier is a chosen
// field set rather than the raw one with empty values dropped: raw carries keys
// summary omits, and summary introduces no key raw lacks.
func TestSuggestionTiersDifferInFieldSet(t *testing.T) {
	suggestion := newTestSuggestion("Suggested because everyone is free.")

	raw := SerializeMeetingTimeSuggestion(suggestion)
	summary := SerializeSummaryMeetingTimeSuggestion(suggestion)

	for key := range summary {
		if _, ok := raw[key]; !ok {
			t.Errorf("summary key %q is absent from raw", key)
		}
	}

	omitted := 0
	for key := range raw {
		if _, ok := summary[key]; !ok {
			omitted++
		}
	}
	if omitted == 0 {
		t.Error("summary omits no raw key, so it is not a distinct field set")
	}

	for _, key := range []string{"attendeeAvailability", "locations"} {
		if _, ok := summary[key]; ok {
			t.Errorf("summary carries %q, which belongs to the raw tier", key)
		}
	}
}

// TestSerializeSummaryMeetingTimeSuggestionFlattensSlot checks that the summary
// tier flattens the time slot to plain strings and adds the localised display
// time the text formatter renders.
func TestSerializeSummaryMeetingTimeSuggestionFlattensSlot(t *testing.T) {
	summary := SerializeSummaryMeetingTimeSuggestion(newTestSuggestion(""))

	slot, ok := summary["meetingTimeSlot"].(map[string]string)
	if !ok {
		t.Fatalf("meetingTimeSlot = %T, want map[string]string", summary["meetingTimeSlot"])
	}
	if slot["start"] != "2026-03-12T09:00:00" {
		t.Errorf("start = %q, want %q", slot["start"], "2026-03-12T09:00:00")
	}
	if slot["displayTime"] == "" {
		t.Error("displayTime is empty, so the text tier has nothing localised to render")
	}
}

// TestSerializeSummaryMeetingTimeSuggestionOmitsAbsentReason checks that a
// suggestion Graph gave no reason for omits the key entirely rather than
// publishing an empty field the reader must interpret.
func TestSerializeSummaryMeetingTimeSuggestionOmitsAbsentReason(t *testing.T) {
	withReason := SerializeSummaryMeetingTimeSuggestion(newTestSuggestion("Because."))
	if withReason["suggestionReason"] != "Because." {
		t.Errorf("suggestionReason = %v, want %q", withReason["suggestionReason"], "Because.")
	}

	withoutReason := SerializeSummaryMeetingTimeSuggestion(newTestSuggestion(""))
	if _, ok := withoutReason["suggestionReason"]; ok {
		t.Error("suggestionReason is present when Graph supplied no reason")
	}
}
