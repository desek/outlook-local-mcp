package graph

import (
	"testing"

	"github.com/microsoft/kiota-abstractions-go/serialization"
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

// newTestScheduleInformation builds one mailbox's schedule with a busy block and
// working hours, so the two tiers can be compared against the same source
// object. The failure argument sets the per-mailbox error Graph returns for a
// mailbox it could not read.
func newTestScheduleInformation(scheduleID string, failure string) models.ScheduleInformationable {
	start := models.NewDateTimeTimeZone()
	startDT := "2026-03-12T09:00:00"
	tz := "UTC"
	start.SetDateTime(&startDT)
	start.SetTimeZone(&tz)

	end := models.NewDateTimeTimeZone()
	endDT := "2026-03-12T10:00:00"
	end.SetDateTime(&endDT)
	end.SetTimeZone(&tz)

	item := models.NewScheduleItem()
	item.SetStart(start)
	item.SetEnd(end)
	status := models.BUSY_FREEBUSYSTATUS
	item.SetStatus(&status)
	subject := "Budget review"
	item.SetSubject(&subject)
	location := "Room 1"
	item.SetLocation(&location)

	zone := models.NewTimeZoneBase()
	zoneName := "UTC"
	zone.SetName(&zoneName)

	hoursStart, _ := serialization.ParseTimeOnly("08:00:00")
	hoursEnd, _ := serialization.ParseTimeOnly("17:00:00")
	hours := models.NewWorkingHours()
	hours.SetDaysOfWeek([]models.DayOfWeek{models.MONDAY_DAYOFWEEK, models.TUESDAY_DAYOFWEEK})
	hours.SetStartTime(hoursStart)
	hours.SetEndTime(hoursEnd)
	hours.SetTimeZone(zone)

	info := models.NewScheduleInformation()
	info.SetScheduleId(&scheduleID)
	availability := "0022"
	info.SetAvailabilityView(&availability)
	info.SetScheduleItems([]models.ScheduleItemable{item})
	info.SetWorkingHours(hours)

	if failure != "" {
		freeBusyErr := models.NewFreeBusyError()
		code := "ErrorAccessDenied"
		freeBusyErr.SetMessage(&failure)
		freeBusyErr.SetResponseCode(&code)
		info.SetError(freeBusyErr)
	}

	return info
}

// TestSerializeScheduleInformationRawKeepsEveryKey checks that the raw tier keeps
// its full shape even when the source object supplies nothing, which is what
// makes raw comparable across mailboxes within one response.
func TestSerializeScheduleInformationRawKeepsEveryKey(t *testing.T) {
	raw := SerializeScheduleInformation(models.NewScheduleInformation())

	for _, key := range []string{"scheduleId", "availabilityView", "scheduleItems", "workingHours", "error"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("raw serialization is missing key %q on an empty schedule", key)
		}
	}

	hours, ok := raw["workingHours"].(map[string]any)
	if !ok {
		t.Fatalf("workingHours = %T, want map[string]any", raw["workingHours"])
	}
	for _, key := range []string{"daysOfWeek", "startTime", "endTime", "timeZone"} {
		if _, ok := hours[key]; !ok {
			t.Errorf("raw working hours are missing key %q on an empty schedule", key)
		}
	}
}

// TestSerializeScheduleTiersDifferInFieldSet checks that the summary tier is a
// chosen field set rather than the raw one with empty values dropped: raw
// carries keys summary omits, and summary introduces no key raw lacks.
func TestSerializeScheduleTiersDifferInFieldSet(t *testing.T) {
	info := newTestScheduleInformation("a@example.com", "")

	raw := SerializeScheduleInformation(info)
	summary := SerializeSummaryScheduleInformation(info)

	for key := range summary {
		if _, ok := raw[key]; !ok {
			t.Errorf("summary key %q is absent from raw", key)
		}
	}

	if _, ok := summary["error"]; ok {
		t.Error("summary carries an error key for a mailbox Graph read successfully")
	}
	if _, ok := raw["error"]; !ok {
		t.Error("raw omits the error key, so its shape varies between mailboxes")
	}

	rawItems, ok := raw["scheduleItems"].([]map[string]any)
	if !ok || len(rawItems) != 1 {
		t.Fatalf("raw scheduleItems = %v, want one item", raw["scheduleItems"])
	}
	if rawItems[0]["subject"] != "Budget review" {
		t.Errorf("raw item subject = %v, want the Graph subject", rawItems[0]["subject"])
	}

	summaryItems, ok := summary["scheduleItems"].([]map[string]any)
	if !ok || len(summaryItems) != 1 {
		t.Fatalf("summary scheduleItems = %v, want one item", summary["scheduleItems"])
	}
	for _, key := range []string{"subject", "location", "isPrivate"} {
		if _, ok := summaryItems[0][key]; ok {
			t.Errorf("summary item carries %q, which belongs to the raw tier", key)
		}
	}
}

// TestSerializeSummaryScheduleInformationSurfacesError checks that a mailbox
// Graph could not read states its error and its response code, so the caller can
// tell it from a mailbox with no meetings.
func TestSerializeSummaryScheduleInformationSurfacesError(t *testing.T) {
	summary := SerializeSummaryScheduleInformation(newTestScheduleInformation("b@example.com", "Access is denied."))

	failure, ok := summary["error"].(map[string]any)
	if !ok {
		t.Fatalf("error = %T, want map[string]any", summary["error"])
	}
	if failure["message"] != "Access is denied." {
		t.Errorf("error message = %v, want the Graph message", failure["message"])
	}
	if failure["responseCode"] != "ErrorAccessDenied" {
		t.Errorf("error responseCode = %v, want the Graph response code", failure["responseCode"])
	}
}

// TestSerializeSummaryScheduleInformationFlattensItems checks that the summary
// tier flattens each block's bounds to plain strings and adds the localised
// display time the text formatter renders.
func TestSerializeSummaryScheduleInformationFlattensItems(t *testing.T) {
	summary := SerializeSummaryScheduleInformation(newTestScheduleInformation("a@example.com", ""))

	items, ok := summary["scheduleItems"].([]map[string]any)
	if !ok || len(items) != 1 {
		t.Fatalf("scheduleItems = %v, want one item", summary["scheduleItems"])
	}
	if items[0]["start"] != "2026-03-12T09:00:00" {
		t.Errorf("start = %v, want %q", items[0]["start"], "2026-03-12T09:00:00")
	}
	if display, _ := items[0]["displayTime"].(string); display == "" {
		t.Error("displayTime is empty, so the text tier has nothing localised to render")
	}

	hours, ok := summary["workingHours"].(map[string]any)
	if !ok {
		t.Fatalf("workingHours = %T, want map[string]any", summary["workingHours"])
	}
	if hours["startTime"] != "08:00:00" {
		t.Errorf("working hours startTime = %v, want %q", hours["startTime"], "08:00:00")
	}
	days, _ := hours["daysOfWeek"].([]string)
	if len(days) != 2 {
		t.Errorf("working hours daysOfWeek = %v, want two days", hours["daysOfWeek"])
	}
}

// TestSerializeSummaryScheduleInformationOmitsAbsentWorkingHours checks that a
// mailbox Graph published no working hours for omits the key entirely rather
// than publishing an empty section the reader must interpret.
func TestSerializeSummaryScheduleInformationOmitsAbsentWorkingHours(t *testing.T) {
	summary := SerializeSummaryScheduleInformation(models.NewScheduleInformation())

	if _, ok := summary["workingHours"]; ok {
		t.Error("workingHours is present when Graph supplied none")
	}
}
