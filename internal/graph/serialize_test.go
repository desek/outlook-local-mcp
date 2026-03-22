package graph

import (
	"testing"

	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// ptr returns a pointer to the given string value. It is a test helper for
// constructing *string values inline.
func ptr(s string) *string {
	return &s
}

// boolPtr returns a pointer to the given bool value. It is a test helper for
// constructing *bool values inline.
func boolPtr(b bool) *bool {
	return &b
}

// TestSerializeEvent_NilFields validates that SerializeEvent does not panic
// when all optional getter methods on the Eventable return nil. A freshly
// constructed Event with no setters called has nil for all optional fields.
func TestSerializeEvent_NilFields(t *testing.T) {
	event := models.NewEvent()

	result := SerializeEvent(event)

	// Verify no panic occurred and basic fields have safe defaults.
	if got := result["id"]; got != "" {
		t.Errorf("id = %q, want %q", got, "")
	}
	if got := result["subject"]; got != "" {
		t.Errorf("subject = %q, want %q", got, "")
	}
	if got := result["isAllDay"]; got != false {
		t.Errorf("isAllDay = %v, want false", got)
	}
	if got := result["isCancelled"]; got != false {
		t.Errorf("isCancelled = %v, want false", got)
	}
	if got := result["isOnlineMeeting"]; got != false {
		t.Errorf("isOnlineMeeting = %v, want false", got)
	}
	if got := result["webLink"]; got != "" {
		t.Errorf("webLink = %q, want %q", got, "")
	}

	// Categories should default to empty slice.
	cats, ok := result["categories"].([]string)
	if !ok {
		t.Fatal("categories is not []string")
	}
	if len(cats) != 0 {
		t.Errorf("categories length = %d, want 0", len(cats))
	}

	// Nested objects should be absent when their parent is nil.
	for _, key := range []string{"start", "end", "location", "organizer", "showAs", "importance", "sensitivity", "onlineMeetingUrl"} {
		if _, exists := result[key]; exists {
			t.Errorf("key %q should not be present when parent object is nil", key)
		}
	}
}

// TestSerializeSummaryEvent validates that SerializeSummaryEvent returns exactly
// 8 keys with flattened values for start, end, location, and organizer when all
// fields are populated.
func TestSerializeSummaryEvent(t *testing.T) {
	event := models.NewEvent()
	event.SetId(ptr("event-456"))
	event.SetSubject(ptr("Weekly Sync"))
	event.SetIsOnlineMeeting(boolPtr(true))

	startTime := models.NewDateTimeTimeZone()
	startTime.SetDateTime(ptr("2026-03-16T09:30:00"))
	startTime.SetTimeZone(ptr("Europe/Stockholm"))
	event.SetStart(startTime)

	endTime := models.NewDateTimeTimeZone()
	endTime.SetDateTime(ptr("2026-03-16T10:00:00"))
	endTime.SetTimeZone(ptr("Europe/Stockholm"))
	event.SetEnd(endTime)

	loc := models.NewLocation()
	loc.SetDisplayName(ptr("Conference Room A"))
	event.SetLocation(loc)

	email := models.NewEmailAddress()
	email.SetName(ptr("Jane Smith"))
	email.SetAddress(ptr("jane@example.com"))
	org := models.NewRecipient()
	org.SetEmailAddress(email)
	event.SetOrganizer(org)

	showAs := models.BUSY_FREEBUSYSTATUS
	event.SetShowAs(&showAs)

	result := SerializeSummaryEvent(event)

	// Verify exactly 9 keys (8 base + displayTime).
	if len(result) != 9 {
		t.Fatalf("result has %d keys, want 9", len(result))
	}

	// Verify flattened string values (not nested objects).
	checks := map[string]any{
		"id":              "event-456",
		"subject":         "Weekly Sync",
		"start":           "2026-03-16T09:30:00",
		"end":             "2026-03-16T10:00:00",
		"displayTime":     "Mon Mar 16, 9:30 AM - 10:00 AM",
		"location":        "Conference Room A",
		"organizer":       "Jane Smith",
		"showAs":          "busy",
		"isOnlineMeeting": true,
	}
	for key, want := range checks {
		got := result[key]
		if got != want {
			t.Errorf("%s = %v (%T), want %v (%T)", key, got, got, want, want)
		}
	}
}

// TestSerializeSummaryEvent_NilFields validates that SerializeSummaryEvent does
// not panic when all optional getter methods return nil and returns safe
// defaults for every key.
func TestSerializeSummaryEvent_NilFields(t *testing.T) {
	event := models.NewEvent()

	result := SerializeSummaryEvent(event)

	if len(result) != 9 {
		t.Fatalf("result has %d keys, want 9", len(result))
	}

	strChecks := []string{"id", "subject", "start", "end", "displayTime", "location", "organizer", "showAs"}
	for _, key := range strChecks {
		got, ok := result[key].(string)
		if !ok {
			t.Errorf("%s is %T, want string", key, result[key])
		} else if got != "" {
			t.Errorf("%s = %q, want %q", key, got, "")
		}
	}

	if got, ok := result["isOnlineMeeting"].(bool); !ok || got != false {
		t.Errorf("isOnlineMeeting = %v, want false", result["isOnlineMeeting"])
	}
}

// TestSerializeSummaryGetEvent validates that SerializeSummaryGetEvent returns
// exactly 12 keys for a fully populated event, including attendees with only
// name and response fields.
func TestSerializeSummaryGetEvent(t *testing.T) {
	event := models.NewEvent()
	event.SetId(ptr("event-789"))
	event.SetSubject(ptr("Design Review"))
	event.SetIsOnlineMeeting(boolPtr(false))
	event.SetBodyPreview(ptr("Review the new design mockups."))
	event.SetHasAttachments(boolPtr(true))

	startTime := models.NewDateTimeTimeZone()
	startTime.SetDateTime(ptr("2026-03-17T14:00:00"))
	event.SetStart(startTime)

	endTime := models.NewDateTimeTimeZone()
	endTime.SetDateTime(ptr("2026-03-17T15:00:00"))
	event.SetEnd(endTime)

	loc := models.NewLocation()
	loc.SetDisplayName(ptr("Room B"))
	event.SetLocation(loc)

	email := models.NewEmailAddress()
	email.SetName(ptr("Bob"))
	org := models.NewRecipient()
	org.SetEmailAddress(email)
	event.SetOrganizer(org)

	showAs := models.TENTATIVE_FREEBUSYSTATUS
	event.SetShowAs(&showAs)

	eventType := models.OCCURRENCE_EVENTTYPE
	event.SetTypeEscaped(&eventType)

	// Add one attendee.
	attEmail := models.NewEmailAddress()
	attEmail.SetName(ptr("Alice Johnson"))
	attEmail.SetAddress(ptr("alice@example.com"))
	attStatus := models.NewResponseStatus()
	resp := models.ACCEPTED_RESPONSETYPE
	attStatus.SetResponse(&resp)
	att := models.NewAttendee()
	att.SetEmailAddress(attEmail)
	att.SetStatus(attStatus)
	attType := models.REQUIRED_ATTENDEETYPE
	att.SetTypeEscaped(&attType)
	event.SetAttendees([]models.Attendeeable{att})

	result := SerializeSummaryGetEvent(event)

	// Verify exactly 13 keys (12 base + displayTime).
	if len(result) != 13 {
		t.Fatalf("result has %d keys, want 13; keys: %v", len(result), keysOf(result))
	}

	// Verify the 4 additional fields.
	if got := result["bodyPreview"]; got != "Review the new design mockups." {
		t.Errorf("bodyPreview = %q, want %q", got, "Review the new design mockups.")
	}
	if got := result["hasAttachments"]; got != true {
		t.Errorf("hasAttachments = %v, want true", got)
	}
	if got := result["type"]; got != "occurrence" {
		t.Errorf("type = %q, want %q", got, "occurrence")
	}

	// Verify attendees.
	attList, ok := result["attendees"].([]map[string]string)
	if !ok {
		t.Fatalf("attendees is %T, want []map[string]string", result["attendees"])
	}
	if len(attList) != 1 {
		t.Fatalf("attendees length = %d, want 1", len(attList))
	}
	if attList[0]["name"] != "Alice Johnson" {
		t.Errorf("attendee name = %q, want %q", attList[0]["name"], "Alice Johnson")
	}
	if attList[0]["response"] != "accepted" {
		t.Errorf("attendee response = %q, want %q", attList[0]["response"], "accepted")
	}
}

// TestSerializeSummaryGetEvent_AttendeeSummary validates that attendees in the
// get-event summary contain only name and response — email and type are
// stripped even when populated on the source attendee.
func TestSerializeSummaryGetEvent_AttendeeSummary(t *testing.T) {
	event := models.NewEvent()

	// Build two attendees with all fields populated.
	att1Email := models.NewEmailAddress()
	att1Email.SetName(ptr("Carol"))
	att1Email.SetAddress(ptr("carol@example.com"))
	att1Status := models.NewResponseStatus()
	resp1 := models.TENTATIVELYACCEPTED_RESPONSETYPE
	att1Status.SetResponse(&resp1)
	att1 := models.NewAttendee()
	att1.SetEmailAddress(att1Email)
	att1.SetStatus(att1Status)
	att1Type := models.REQUIRED_ATTENDEETYPE
	att1.SetTypeEscaped(&att1Type)

	att2Email := models.NewEmailAddress()
	att2Email.SetName(ptr("Dave"))
	att2Email.SetAddress(ptr("dave@example.com"))
	att2Status := models.NewResponseStatus()
	resp2 := models.DECLINED_RESPONSETYPE
	att2Status.SetResponse(&resp2)
	att2 := models.NewAttendee()
	att2.SetEmailAddress(att2Email)
	att2.SetStatus(att2Status)
	att2Type := models.OPTIONAL_ATTENDEETYPE
	att2.SetTypeEscaped(&att2Type)

	event.SetAttendees([]models.Attendeeable{att1, att2})

	result := SerializeSummaryGetEvent(event)
	attList, ok := result["attendees"].([]map[string]string)
	if !ok {
		t.Fatalf("attendees is %T, want []map[string]string", result["attendees"])
	}
	if len(attList) != 2 {
		t.Fatalf("attendees length = %d, want 2", len(attList))
	}

	// Verify each attendee has exactly 2 keys (name, response) — no email, no type.
	for i, att := range attList {
		if len(att) != 2 {
			t.Errorf("attendee[%d] has %d keys, want 2; keys: %v", i, len(att), att)
		}
		if _, exists := att["email"]; exists {
			t.Errorf("attendee[%d] contains 'email' key, should be stripped", i)
		}
		if _, exists := att["type"]; exists {
			t.Errorf("attendee[%d] contains 'type' key, should be stripped", i)
		}
	}

	// Verify values.
	if attList[0]["name"] != "Carol" || attList[0]["response"] != "tentativelyAccepted" {
		t.Errorf("attendee[0] = %v, want name=Carol response=tentativelyAccepted", attList[0])
	}
	if attList[1]["name"] != "Dave" || attList[1]["response"] != "declined" {
		t.Errorf("attendee[1] = %v, want name=Dave response=declined", attList[1])
	}
}

// keysOf returns the keys of a map[string]any for diagnostic output.
func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// TestSerializeEvent_AllFields validates that SerializeEvent correctly extracts
// all 15 fields from a fully populated Eventable, including nested objects for
// start/end times, location, organizer, and online meeting URL.
func TestSerializeEvent_AllFields(t *testing.T) {
	event := models.NewEvent()

	// Simple fields.
	event.SetId(ptr("event-123"))
	event.SetSubject(ptr("Team Standup"))
	event.SetIsAllDay(boolPtr(false))
	event.SetIsCancelled(boolPtr(false))
	event.SetIsOnlineMeeting(boolPtr(true))
	event.SetWebLink(ptr("https://outlook.office.com/calendar/item/123"))

	// Start time.
	startTime := models.NewDateTimeTimeZone()
	startTime.SetDateTime(ptr("2026-03-12T09:00:00"))
	startTime.SetTimeZone(ptr("America/New_York"))
	event.SetStart(startTime)

	// End time.
	endTime := models.NewDateTimeTimeZone()
	endTime.SetDateTime(ptr("2026-03-12T09:30:00"))
	endTime.SetTimeZone(ptr("America/New_York"))
	event.SetEnd(endTime)

	// Location.
	loc := models.NewLocation()
	loc.SetDisplayName(ptr("Conference Room A"))
	event.SetLocation(loc)

	// Organizer.
	email := models.NewEmailAddress()
	email.SetName(ptr("Alice"))
	email.SetAddress(ptr("alice@example.com"))
	org := models.NewRecipient()
	org.SetEmailAddress(email)
	event.SetOrganizer(org)

	// Enums.
	showAs := models.BUSY_FREEBUSYSTATUS
	event.SetShowAs(&showAs)
	importance := models.NORMAL_IMPORTANCE
	event.SetImportance(&importance)
	sensitivity := models.PRIVATE_SENSITIVITY
	event.SetSensitivity(&sensitivity)

	// Categories.
	event.SetCategories([]string{"Work", "Meeting"})

	// Online meeting.
	om := models.NewOnlineMeetingInfo()
	om.SetJoinUrl(ptr("https://teams.microsoft.com/meet/123"))
	event.SetOnlineMeeting(om)

	result := SerializeEvent(event)

	// Verify simple fields.
	checks := map[string]any{
		"id":              "event-123",
		"subject":         "Team Standup",
		"isAllDay":        false,
		"isCancelled":     false,
		"isOnlineMeeting": true,
		"webLink":         "https://outlook.office.com/calendar/item/123",
	}
	for key, want := range checks {
		if got := result[key]; got != want {
			t.Errorf("%s = %v, want %v", key, got, want)
		}
	}

	// Verify start time.
	start, ok := result["start"].(map[string]string)
	if !ok {
		t.Fatal("start is not map[string]string")
	}
	if start["dateTime"] != "2026-03-12T09:00:00" {
		t.Errorf("start.dateTime = %q, want %q", start["dateTime"], "2026-03-12T09:00:00")
	}
	if start["timeZone"] != "America/New_York" {
		t.Errorf("start.timeZone = %q, want %q", start["timeZone"], "America/New_York")
	}

	// Verify end time.
	end, ok := result["end"].(map[string]string)
	if !ok {
		t.Fatal("end is not map[string]string")
	}
	if end["dateTime"] != "2026-03-12T09:30:00" {
		t.Errorf("end.dateTime = %q, want %q", end["dateTime"], "2026-03-12T09:30:00")
	}
	if end["timeZone"] != "America/New_York" {
		t.Errorf("end.timeZone = %q, want %q", end["timeZone"], "America/New_York")
	}

	// Verify location.
	if got := result["location"]; got != "Conference Room A" {
		t.Errorf("location = %q, want %q", got, "Conference Room A")
	}

	// Verify organizer.
	orgMap, ok := result["organizer"].(map[string]string)
	if !ok {
		t.Fatal("organizer is not map[string]string")
	}
	if orgMap["name"] != "Alice" {
		t.Errorf("organizer.name = %q, want %q", orgMap["name"], "Alice")
	}
	if orgMap["address"] != "alice@example.com" {
		t.Errorf("organizer.address = %q, want %q", orgMap["address"], "alice@example.com")
	}

	// Verify enums.
	if got := result["showAs"]; got != "busy" {
		t.Errorf("showAs = %q, want %q", got, "busy")
	}
	if got := result["importance"]; got != "normal" {
		t.Errorf("importance = %q, want %q", got, "normal")
	}
	if got := result["sensitivity"]; got != "private" {
		t.Errorf("sensitivity = %q, want %q", got, "private")
	}

	// Verify categories.
	cats, ok := result["categories"].([]string)
	if !ok {
		t.Fatal("categories is not []string")
	}
	if len(cats) != 2 || cats[0] != "Work" || cats[1] != "Meeting" {
		t.Errorf("categories = %v, want [Work Meeting]", cats)
	}

	// Verify online meeting URL.
	if got := result["onlineMeetingUrl"]; got != "https://teams.microsoft.com/meet/123" {
		t.Errorf("onlineMeetingUrl = %q, want %q", got, "https://teams.microsoft.com/meet/123")
	}
}

// TestSerializeEvent_CreatedByMcp validates that SerializeEvent includes
// "createdByMcp": true when the event has a matching provenance extended
// property and a non-empty provenancePropertyID is provided.
func TestSerializeEvent_CreatedByMcp(t *testing.T) {
	propID := BuildProvenancePropertyID("com.github.desek.outlook-local-mcp.created")

	event := models.NewEvent()
	event.SetId(ptr("event-prov"))
	event.SetSubject(ptr("Tagged Event"))

	// Set provenance extended property on the event.
	prop := models.NewSingleValueLegacyExtendedProperty()
	prop.SetId(&propID)
	val := "true"
	prop.SetValue(&val)
	event.SetSingleValueExtendedProperties([]models.SingleValueLegacyExtendedPropertyable{prop})

	result := SerializeEvent(event, propID)

	got, exists := result["createdByMcp"]
	if !exists {
		t.Fatal("createdByMcp key not present, want true")
	}
	if got != true {
		t.Errorf("createdByMcp = %v, want true", got)
	}
}

// TestSerializeEvent_NoMcpTag validates that SerializeEvent omits
// "createdByMcp" when the event has no provenance extended property, even
// when a provenancePropertyID is provided.
func TestSerializeEvent_NoMcpTag(t *testing.T) {
	propID := BuildProvenancePropertyID("com.github.desek.outlook-local-mcp.created")

	event := models.NewEvent()
	event.SetId(ptr("event-noprov"))
	event.SetSubject(ptr("Regular Event"))

	result := SerializeEvent(event, propID)

	if _, exists := result["createdByMcp"]; exists {
		t.Error("createdByMcp key should not be present when event has no provenance tag")
	}
}

// TestSerializeSummaryEvent_CreatedByMcp validates that SerializeSummaryEvent
// includes "createdByMcp": true when the provenance tag is present.
func TestSerializeSummaryEvent_CreatedByMcp(t *testing.T) {
	propID := BuildProvenancePropertyID("com.github.desek.outlook-local-mcp.created")

	event := models.NewEvent()
	event.SetId(ptr("event-summ-prov"))

	prop := models.NewSingleValueLegacyExtendedProperty()
	prop.SetId(&propID)
	val := "true"
	prop.SetValue(&val)
	event.SetSingleValueExtendedProperties([]models.SingleValueLegacyExtendedPropertyable{prop})

	result := SerializeSummaryEvent(event, propID)

	got, exists := result["createdByMcp"]
	if !exists {
		t.Fatal("createdByMcp key not present, want true")
	}
	if got != true {
		t.Errorf("createdByMcp = %v, want true", got)
	}
}

// TestSerializeSummaryGetEvent_CreatedByMcp validates that
// SerializeSummaryGetEvent includes "createdByMcp": true when the provenance
// tag is present.
func TestSerializeSummaryGetEvent_CreatedByMcp(t *testing.T) {
	propID := BuildProvenancePropertyID("com.github.desek.outlook-local-mcp.created")

	event := models.NewEvent()
	event.SetId(ptr("event-get-prov"))

	prop := models.NewSingleValueLegacyExtendedProperty()
	prop.SetId(&propID)
	val := "true"
	prop.SetValue(&val)
	event.SetSingleValueExtendedProperties([]models.SingleValueLegacyExtendedPropertyable{prop})

	result := SerializeSummaryGetEvent(event, propID)

	got, exists := result["createdByMcp"]
	if !exists {
		t.Fatal("createdByMcp key not present, want true")
	}
	if got != true {
		t.Errorf("createdByMcp = %v, want true", got)
	}
}

// TestSerializeEvent_NoProvenanceID validates that SerializeEvent omits
// "createdByMcp" entirely when no provenancePropertyID is provided (provenance
// tagging disabled).
func TestSerializeEvent_NoProvenanceID(t *testing.T) {
	event := models.NewEvent()
	event.SetId(ptr("event-disabled"))

	// Call without provenancePropertyID (variadic omitted).
	result := SerializeEvent(event)

	if _, exists := result["createdByMcp"]; exists {
		t.Error("createdByMcp key should not be present when provenancePropertyID is omitted")
	}
}

// TestSerializeSummaryEvent_DisplayTime validates that SerializeSummaryEvent
// includes a correctly formatted displayTime field for a timed event with
// timezone information.
func TestSerializeSummaryEvent_DisplayTime(t *testing.T) {
	event := models.NewEvent()
	event.SetId(ptr("event-dt"))
	event.SetSubject(ptr("Team Sync"))
	event.SetIsAllDay(boolPtr(false))

	startTime := models.NewDateTimeTimeZone()
	startTime.SetDateTime(ptr("2026-03-19T14:00:00"))
	startTime.SetTimeZone(ptr("Europe/Stockholm"))
	event.SetStart(startTime)

	endTime := models.NewDateTimeTimeZone()
	endTime.SetDateTime(ptr("2026-03-19T15:00:00"))
	endTime.SetTimeZone(ptr("Europe/Stockholm"))
	event.SetEnd(endTime)

	result := SerializeSummaryEvent(event)

	got, ok := result["displayTime"].(string)
	if !ok {
		t.Fatalf("displayTime is %T, want string", result["displayTime"])
	}
	want := "Thu Mar 19, 2:00 PM - 3:00 PM"
	if got != want {
		t.Errorf("displayTime = %q, want %q", got, want)
	}
}

// TestSerializeSummaryEvent_DisplayTime_AllDay validates that an all-day event
// renders displayTime with "(all day)" instead of clock times.
func TestSerializeSummaryEvent_DisplayTime_AllDay(t *testing.T) {
	event := models.NewEvent()
	event.SetId(ptr("event-allday"))
	event.SetSubject(ptr("Company Holiday"))
	event.SetIsAllDay(boolPtr(true))

	startTime := models.NewDateTimeTimeZone()
	startTime.SetDateTime(ptr("2026-03-19T00:00:00"))
	startTime.SetTimeZone(ptr("Europe/Stockholm"))
	event.SetStart(startTime)

	endTime := models.NewDateTimeTimeZone()
	endTime.SetDateTime(ptr("2026-03-20T00:00:00"))
	endTime.SetTimeZone(ptr("Europe/Stockholm"))
	event.SetEnd(endTime)

	result := SerializeSummaryEvent(event)

	got, ok := result["displayTime"].(string)
	if !ok {
		t.Fatalf("displayTime is %T, want string", result["displayTime"])
	}
	want := "Thu Mar 19 (all day)"
	if got != want {
		t.Errorf("displayTime = %q, want %q", got, want)
	}
}
