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

// SerializeScheduleInformation extracts the full Graph representation of one
// mailbox's schedule into a map suitable for the "raw" output tier. Every key is
// present regardless of whether Graph supplied a value, including the per-mailbox
// error, so the raw tier keeps the "including empty values" property the
// output-tier contract states and its shape never varies between mailboxes.
//
// Parameters:
//   - info: one entry of the getSchedule response. Every getter may return nil.
//
// Returns a map carrying the schedule id, the availability view string, the
// schedule items with their subject, location and privacy flag, the working
// hours, and the per-mailbox error.
//
// Side effects: none.
func SerializeScheduleInformation(info models.ScheduleInformationable) map[string]any {
	items := make([]map[string]any, 0, len(info.GetScheduleItems()))
	for _, item := range info.GetScheduleItems() {
		status := ""
		if s := item.GetStatus(); s != nil {
			status = s.String()
		}
		isPrivate := false
		if p := item.GetIsPrivate(); p != nil {
			isPrivate = *p
		}
		items = append(items, map[string]any{
			"start":     serializeDateTimeTimeZone(item.GetStart()),
			"end":       serializeDateTimeTimeZone(item.GetEnd()),
			"status":    status,
			"subject":   SafeStr(item.GetSubject()),
			"location":  SafeStr(item.GetLocation()),
			"isPrivate": isPrivate,
		})
	}

	return map[string]any{
		"scheduleId":       SafeStr(info.GetScheduleId()),
		"availabilityView": SafeStr(info.GetAvailabilityView()),
		"scheduleItems":    items,
		"workingHours":     serializeWorkingHours(info.GetWorkingHours()),
		"error":            serializeFreeBusyError(info.GetError()),
	}
}

// SerializeSummaryScheduleInformation extracts the deliberately chosen field set
// for the "summary" and "text" output tiers: the schedule id, the availability
// view string, each schedule item's start, end and status, and the working
// hours' days, start, end and timezone.
//
// Two keys are conditional rather than always present, which is what makes the
// summary a chosen field set rather than raw with its empty values dropped.
// "error" appears only when Graph reported one for this mailbox, so a caller
// reading a mailbox it may not view sees a stated error rather than an absent
// one (and a mailbox that succeeded carries no empty error field to interpret).
// "workingHours" appears only when Graph supplied them.
//
// Parameters:
//   - info: one entry of the getSchedule response. Every getter may return nil.
//
// Returns the summary map. Every key it carries also exists in the raw map, so
// the two tiers can be compared key by key.
//
// Side effects: none.
func SerializeSummaryScheduleInformation(info models.ScheduleInformationable) map[string]any {
	items := make([]map[string]any, 0, len(info.GetScheduleItems()))
	for _, item := range info.GetScheduleItems() {
		start, startTZ, end, endTZ := "", "", "", ""
		if s := item.GetStart(); s != nil {
			start = SafeStr(s.GetDateTime())
			startTZ = SafeStr(s.GetTimeZone())
		}
		if e := item.GetEnd(); e != nil {
			end = SafeStr(e.GetDateTime())
			endTZ = SafeStr(e.GetTimeZone())
		}
		status := ""
		if s := item.GetStatus(); s != nil {
			status = s.String()
		}
		items = append(items, map[string]any{
			"start":       start,
			"end":         end,
			"displayTime": FormatDisplayTime(start, end, startTZ, endTZ, false),
			"status":      status,
		})
	}

	result := map[string]any{
		"scheduleId":       SafeStr(info.GetScheduleId()),
		"availabilityView": SafeStr(info.GetAvailabilityView()),
		"scheduleItems":    items,
	}

	if wh := info.GetWorkingHours(); wh != nil {
		result["workingHours"] = serializeWorkingHours(wh)
	}
	if e := info.GetError(); e != nil {
		result["error"] = serializeFreeBusyError(e)
	}

	return result
}

// serializeWorkingHours renders a Graph WorkingHours as the days it covers and
// its daily bounds. The start and end are Kiota TimeOnly values rather than
// strings, and the timezone is a TimeZoneBase whose readable content is its
// name. A nil value still yields the full key structure with empty values, so
// the raw tier never varies its shape.
func serializeWorkingHours(hours models.WorkingHoursable) map[string]any {
	days := []string{}
	startTime, endTime, timeZone := "", "", ""
	if hours != nil {
		for _, day := range hours.GetDaysOfWeek() {
			days = append(days, day.String())
		}
		if s := hours.GetStartTime(); s != nil {
			startTime = s.String()
		}
		if e := hours.GetEndTime(); e != nil {
			endTime = e.String()
		}
		if tz := hours.GetTimeZone(); tz != nil {
			timeZone = SafeStr(tz.GetName())
		}
	}
	return map[string]any{
		"daysOfWeek": days,
		"startTime":  startTime,
		"endTime":    endTime,
		"timeZone":   timeZone,
	}
}

// serializeFreeBusyError renders the error Graph returns for a single mailbox
// it could not read. A nil error still yields the full key structure with empty
// values, so the raw tier never varies its shape.
func serializeFreeBusyError(err models.FreeBusyErrorable) map[string]any {
	message, responseCode := "", ""
	if err != nil {
		message = SafeStr(err.GetMessage())
		responseCode = SafeStr(err.GetResponseCode())
	}
	return map[string]any{
		"message":      message,
		"responseCode": responseCode,
	}
}

// serializeDateTimeTimeZone renders a Graph DateTimeTimeZone as the nested
// dateTime and timeZone pair Graph itself returns, with the full key structure
// preserved when the value is nil.
func serializeDateTimeTimeZone(value models.DateTimeTimeZoneable) map[string]string {
	dateTime, timeZone := "", ""
	if value != nil {
		dateTime = SafeStr(value.GetDateTime())
		timeZone = SafeStr(value.GetTimeZone())
	}
	return map[string]string{"dateTime": dateTime, "timeZone": timeZone}
}

// serializeTimeSlot renders a Graph TimeSlot as nested dateTime and timeZone
// pairs, keeping the shape Graph itself returns. A nil slot still yields the
// full key structure with empty values, so the raw tier never varies its shape.
func serializeTimeSlot(slot models.TimeSlotable) map[string]any {
	var start, end models.DateTimeTimeZoneable
	if slot != nil {
		start = slot.GetStart()
		end = slot.GetEnd()
	}
	return map[string]any{
		"start": serializeDateTimeTimeZone(start),
		"end":   serializeDateTimeTimeZone(end),
	}
}
