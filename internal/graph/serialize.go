package graph

import (
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// SerializeEvent extracts a standard set of fields from a models.Eventable into
// a map[string]any suitable for JSON serialization in MCP tool responses. All
// pointer fields are accessed through SafeStr and SafeBool helpers to prevent
// nil dereference panics. Nested objects (start, end, location, organizer,
// onlineMeetingUrl) are nil-checked before field extraction.
//
// When provenancePropertyID is non-empty and the event has a matching
// single-value extended property, the key "createdByMcp" is set to true.
// When absent, the key is omitted entirely (not set to false).
//
// Parameters:
//   - event: a models.Eventable representing a calendar event from the
//     Microsoft Graph API. All getter methods may return nil.
//   - provenancePropertyID: the full provenance property ID string. When empty,
//     provenance checking is skipped.
//
// Returns a map containing the standard event fields plus optionally
// "createdByMcp". Nested object fields are omitted from the map when the
// parent object is nil.
//
// Side effects: none.
func SerializeEvent(event models.Eventable, provenancePropertyID ...string) map[string]any {
	result := map[string]any{
		"id":              SafeStr(event.GetId()),
		"subject":         SafeStr(event.GetSubject()),
		"isAllDay":        SafeBool(event.GetIsAllDay()),
		"isCancelled":     SafeBool(event.GetIsCancelled()),
		"isOnlineMeeting": SafeBool(event.GetIsOnlineMeeting()),
		"webLink":         SafeStr(event.GetWebLink()),
	}

	// Start time: nested DateTimeTimeZoneable with dateTime and timeZone.
	if s := event.GetStart(); s != nil {
		result["start"] = map[string]string{
			"dateTime": SafeStr(s.GetDateTime()),
			"timeZone": SafeStr(s.GetTimeZone()),
		}
	}

	// End time: nested DateTimeTimeZoneable with dateTime and timeZone.
	if e := event.GetEnd(); e != nil {
		result["end"] = map[string]string{
			"dateTime": SafeStr(e.GetDateTime()),
			"timeZone": SafeStr(e.GetTimeZone()),
		}
	}

	// Location: nested Locationable with displayName.
	if loc := event.GetLocation(); loc != nil {
		result["location"] = SafeStr(loc.GetDisplayName())
	}

	// Organizer: nested Recipientable -> EmailAddressable with name and address.
	if org := event.GetOrganizer(); org != nil {
		if ea := org.GetEmailAddress(); ea != nil {
			result["organizer"] = map[string]string{
				"name":    SafeStr(ea.GetName()),
				"address": SafeStr(ea.GetAddress()),
			}
		}
	}

	// ShowAs: FreeBusyStatus enum with String() method.
	if sa := event.GetShowAs(); sa != nil {
		result["showAs"] = sa.String()
	}

	// Importance: Importance enum with String() method.
	if imp := event.GetImportance(); imp != nil {
		result["importance"] = imp.String()
	}

	// Sensitivity: Sensitivity enum with String() method.
	if sens := event.GetSensitivity(); sens != nil {
		result["sensitivity"] = sens.String()
	}

	// Categories: slice of strings, default to empty slice if nil.
	if cats := event.GetCategories(); cats != nil {
		result["categories"] = cats
	} else {
		result["categories"] = []string{}
	}

	// Online meeting URL: nested OnlineMeetingInfoable with joinUrl.
	if om := event.GetOnlineMeeting(); om != nil {
		result["onlineMeetingUrl"] = SafeStr(om.GetJoinUrl())
	}

	// Provenance: set createdByMcp to true when the provenance extended
	// property is present. Omit the key entirely when absent.
	if len(provenancePropertyID) > 0 && provenancePropertyID[0] != "" {
		if HasProvenanceTag(event, provenancePropertyID[0]) {
			result["createdByMcp"] = true
		}
	}

	return result
}

// summaryKeys lists the field names included in the list_events summary format.
var summaryKeys = []string{"id", "subject", "start", "end", "location", "organizer", "showAs", "isOnlineMeeting"}

// ToSummaryEventMap converts a raw serialized event map (from SerializeEvent)
// to summary format by extracting and flattening the summary fields. Nested
// start/end objects are flattened to dateTime strings, organizer is flattened
// to a name string, and location is kept as-is (already a string in raw).
// A displayTime field is computed from the start/end datetime and timezone
// values. If the raw map contains "createdByMcp", it is propagated to the
// result.
//
// Parameters:
//   - raw: a map produced by SerializeEvent with the full field set.
//
// Returns a new map containing the summary fields, displayTime, plus
// "createdByMcp" when present in the raw map.
//
// Side effects: none.
func ToSummaryEventMap(raw map[string]any) map[string]any {
	result := make(map[string]any, len(summaryKeys))

	result["id"], _ = raw["id"].(string)
	result["subject"], _ = raw["subject"].(string)

	// Extract start dateTime and timeZone from nested object.
	var startDT, startTZ, endDT, endTZ string
	if startObj, ok := raw["start"].(map[string]any); ok {
		startDT, _ = startObj["dateTime"].(string)
		startTZ, _ = startObj["timeZone"].(string)
	} else if startObj, ok := raw["start"].(map[string]string); ok {
		startDT = startObj["dateTime"]
		startTZ = startObj["timeZone"]
	}
	result["start"] = startDT

	if endObj, ok := raw["end"].(map[string]any); ok {
		endDT, _ = endObj["dateTime"].(string)
		endTZ, _ = endObj["timeZone"].(string)
	} else if endObj, ok := raw["end"].(map[string]string); ok {
		endDT = endObj["dateTime"]
		endTZ = endObj["timeZone"]
	}
	result["end"] = endDT

	// Compute displayTime from the extracted datetime and timezone values.
	isAllDay, _ := raw["isAllDay"].(bool)
	result["displayTime"] = FormatDisplayTime(startDT, endDT, startTZ, endTZ, isAllDay)

	// Location: raw SerializeEvent stores as string.
	result["location"], _ = raw["location"].(string)

	// Organizer: raw SerializeEvent stores as map with name/address.
	if orgObj, ok := raw["organizer"].(map[string]any); ok {
		result["organizer"], _ = orgObj["name"].(string)
	} else if orgObj, ok := raw["organizer"].(map[string]string); ok {
		result["organizer"] = orgObj["name"]
	} else {
		result["organizer"] = ""
	}

	result["showAs"], _ = raw["showAs"].(string)
	if v, ok := raw["isOnlineMeeting"].(bool); ok {
		result["isOnlineMeeting"] = v
	} else {
		result["isOnlineMeeting"] = false
	}

	// Propagate provenance tag from raw map when present.
	if v, ok := raw["createdByMcp"].(bool); ok && v {
		result["createdByMcp"] = true
	}

	return result
}

// SerializeSummaryEvent extracts a minimal set of fields from a
// models.Eventable into a map[string]any suitable for compact summary
// responses. Unlike SerializeEvent, nested objects are flattened: start and end
// are plain dateTime strings (no timeZone wrapper), location is a display name
// string, and organizer is a name string. This produces a concise
// representation similar to what a user sees in Outlook's calendar list view.
//
// When provenancePropertyID is non-empty and the event has a matching
// single-value extended property, the key "createdByMcp" is set to true.
// When absent, the key is omitted entirely.
//
// Parameters:
//   - event: a models.Eventable representing a calendar event from the
//     Microsoft Graph API. All getter methods may return nil.
//   - provenancePropertyID: optional provenance property ID string. When
//     provided and non-empty, provenance tag presence is checked.
//
// Returns a map containing the summary keys plus optionally "createdByMcp".
// All values have safe defaults when the source field is nil.
//
// Side effects: none.
func SerializeSummaryEvent(event models.Eventable, provenancePropertyID ...string) map[string]any {
	start := ""
	startTZ := ""
	if s := event.GetStart(); s != nil {
		start = SafeStr(s.GetDateTime())
		startTZ = SafeStr(s.GetTimeZone())
	}

	end := ""
	endTZ := ""
	if e := event.GetEnd(); e != nil {
		end = SafeStr(e.GetDateTime())
		endTZ = SafeStr(e.GetTimeZone())
	}

	location := ""
	if loc := event.GetLocation(); loc != nil {
		location = SafeStr(loc.GetDisplayName())
	}

	organizer := ""
	if org := event.GetOrganizer(); org != nil {
		if ea := org.GetEmailAddress(); ea != nil {
			organizer = SafeStr(ea.GetName())
		}
	}

	showAs := ""
	if sa := event.GetShowAs(); sa != nil {
		showAs = sa.String()
	}

	result := map[string]any{
		"id":              SafeStr(event.GetId()),
		"subject":         SafeStr(event.GetSubject()),
		"start":           start,
		"end":             end,
		"displayTime":     FormatDisplayTime(start, end, startTZ, endTZ, SafeBool(event.GetIsAllDay())),
		"location":        location,
		"organizer":       organizer,
		"showAs":          showAs,
		"isOnlineMeeting": SafeBool(event.GetIsOnlineMeeting()),
	}

	// Provenance: set createdByMcp to true when the provenance extended
	// property is present. Omit the key entirely when absent.
	if len(provenancePropertyID) > 0 && provenancePropertyID[0] != "" {
		if HasProvenanceTag(event, provenancePropertyID[0]) {
			result["createdByMcp"] = true
		}
	}

	return result
}

// SerializeSummaryGetEvent extracts a slightly larger set of fields from a
// models.Eventable for the get_event summary response. It starts from the same
// fields as SerializeSummaryEvent and adds attendees (name + response only,
// no email or type), bodyPreview, hasAttachments, and event type. This gives
// more detail than the list view while still excluding verbose data like HTML
// body content.
//
// When provenancePropertyID is non-empty, it is forwarded to
// SerializeSummaryEvent which checks for the provenance extended property.
//
// Parameters:
//   - event: a models.Eventable representing a calendar event from the
//     Microsoft Graph API. All getter methods may return nil.
//   - provenancePropertyID: optional provenance property ID string. When
//     provided and non-empty, provenance tag presence is checked.
//
// Returns a map containing the summary-get keys plus optionally "createdByMcp".
//
// Side effects: none.
func SerializeSummaryGetEvent(event models.Eventable, provenancePropertyID ...string) map[string]any {
	result := SerializeSummaryEvent(event, provenancePropertyID...)

	// Attendees: array of {name, response} maps only (no email, no type).
	if attendees := event.GetAttendees(); attendees != nil {
		attList := make([]map[string]string, 0, len(attendees))
		for _, att := range attendees {
			attMap := map[string]string{
				"name":     "",
				"response": "",
			}
			if ea := att.GetEmailAddress(); ea != nil {
				attMap["name"] = SafeStr(ea.GetName())
			}
			if status := att.GetStatus(); status != nil {
				if resp := status.GetResponse(); resp != nil {
					attMap["response"] = resp.String()
				}
			}
			attList = append(attList, attMap)
		}
		result["attendees"] = attList
	} else {
		result["attendees"] = []map[string]string{}
	}

	result["bodyPreview"] = SafeStr(event.GetBodyPreview())
	result["hasAttachments"] = SafeBool(event.GetHasAttachments())

	// Type: event type enum (singleInstance, occurrence, exception, seriesMaster).
	if et := event.GetTypeEscaped(); et != nil {
		result["type"] = et.String()
	} else {
		result["type"] = ""
	}

	return result
}
