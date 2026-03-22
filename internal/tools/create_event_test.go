package tools

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// TestBodyContentTypeDetection validates that body content containing a "<"
// character is detected as HTML, while plain text is detected as TEXT.
func TestBodyContentTypeDetection(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantHTML bool
	}{
		{"HTML body", "<p>Hello</p>", true},
		{"text body", "Hello world", false},
		{"body with angle bracket", "x < y", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isHTML := strings.Contains(tt.body, "<")
			if isHTML != tt.wantHTML {
				t.Errorf("Contains(<) = %v, want %v for %q", isHTML, tt.wantHTML, tt.body)
			}
		})
	}
}

// TestAttendeesJSONParsing validates that a well-formed attendees JSON array
// is correctly parsed into a slice of Attendeeable objects with the expected
// email, name, and type fields.
func TestAttendeesJSONParsing(t *testing.T) {
	jsonStr := `[{"email":"a@b.com","name":"A","type":"required"}]`
	attendees, err := parseAttendees(jsonStr)
	if err != nil {
		t.Fatalf("parseAttendees() error = %v", err)
	}
	if len(attendees) != 1 {
		t.Fatalf("len(attendees) = %d, want 1", len(attendees))
	}

	att := attendees[0]
	ea := att.GetEmailAddress()
	if ea == nil {
		t.Fatal("email address is nil")
	}
	if addr := graph.SafeStr(ea.GetAddress()); addr != "a@b.com" {
		t.Errorf("email = %q, want %q", addr, "a@b.com")
	}
	if name := graph.SafeStr(ea.GetName()); name != "A" {
		t.Errorf("name = %q, want %q", name, "A")
	}
	if at := att.GetTypeEscaped(); at == nil || *at != models.REQUIRED_ATTENDEETYPE {
		t.Errorf("type = %v, want REQUIRED", at)
	}
}

// TestAttendeesJSONInvalid validates that parseAttendees returns an error when
// given malformed JSON.
func TestAttendeesJSONInvalid(t *testing.T) {
	_, err := parseAttendees("not json")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// TestAttendeesExceedLimit validates that parseAttendees returns an error when
// the attendee count exceeds the 500 limit.
func TestAttendeesExceedLimit(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("[")
	for i := 0; i < 501; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`{"email":"a@b.com","name":"A","type":"required"}`)
	}
	sb.WriteString("]")

	_, err := parseAttendees(sb.String())
	if err == nil {
		t.Fatal("expected error for exceeding attendee limit, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %q, want message containing '500'", err.Error())
	}
}

// TestAttendeesMultipleTypes validates parsing attendees with different types.
func TestAttendeesMultipleTypes(t *testing.T) {
	jsonStr := `[
		{"email":"a@b.com","name":"A","type":"required"},
		{"email":"b@c.com","name":"B","type":"optional"},
		{"email":"room@c.com","name":"Room","type":"resource"}
	]`
	attendees, err := parseAttendees(jsonStr)
	if err != nil {
		t.Fatalf("parseAttendees() error = %v", err)
	}
	if len(attendees) != 3 {
		t.Fatalf("len(attendees) = %d, want 3", len(attendees))
	}

	expectedTypes := []models.AttendeeType{
		models.REQUIRED_ATTENDEETYPE,
		models.OPTIONAL_ATTENDEETYPE,
		models.RESOURCE_ATTENDEETYPE,
	}
	for i, att := range attendees {
		at := att.GetTypeEscaped()
		if at == nil || *at != expectedTypes[i] {
			t.Errorf("attendee[%d] type = %v, want %v", i, at, expectedTypes[i])
		}
	}
}

// TestSplitCategories validates that comma-separated strings are correctly
// split and trimmed.
func TestSplitCategories(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"Blue Category,Red Category", []string{"Blue Category", "Red Category"}},
		{"  Work , Meeting  ", []string{"Work", "Meeting"}},
		{"Single", []string{"Single"}},
		{",,,", nil},
	}
	for _, tt := range tests {
		got := splitCategories(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitCategories(%q) = %v (len %d), want %v (len %d)",
				tt.input, got, len(got), tt.want, len(tt.want))
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitCategories(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

// TestCreateEventMinimalParams validates that the NewCreateEventTool returns a
// tool with the expected name and account parameter, and that the handler can
// be constructed without panicking.
func TestCreateEventMinimalParams(t *testing.T) {
	tool := NewCreateEventTool()
	if tool.Name != "calendar_create_event" {
		t.Errorf("tool name = %q, want %q", tool.Name, "calendar_create_event")
	}

	if _, ok := tool.InputSchema.Properties["account"]; !ok {
		t.Error("expected account property to be defined")
	}

	handler := HandleCreateEvent(graph.RetryConfig{}, 0, "America/New_York", "")
	if handler == nil {
		t.Fatal("handler is nil")
	}
}

// TestHandleCreateEvent_NoClientInContext validates that the handler returns
// a tool error when no Graph client is present in the context.
func TestHandleCreateEvent_NoClientInContext(t *testing.T) {
	handler := HandleCreateEvent(graph.RetryConfig{}, 0, "America/New_York", "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"subject":        "Test",
		"start_datetime": "2026-04-15T09:00:00",
		"start_timezone": "America/New_York",
		"end_datetime":   "2026-04-15T10:00:00",
		"end_timezone":   "America/New_York",
	}

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when no client in context")
	}
	text := result.Content[0].(mcp.TextContent).Text
	if text != "no account selected" {
		t.Errorf("error text = %q, want %q", text, "no account selected")
	}
}

// mockEventJSON is a minimal Graph API event response used by advisory integration tests.
const mockEventJSON = `{"id":"AAMkTest","subject":"Test","start":{"dateTime":"2026-04-15T09:00:00","timeZone":"America/New_York"},"end":{"dateTime":"2026-04-15T10:00:00","timeZone":"America/New_York"},"isAllDay":false,"isCancelled":false,"isOnlineMeeting":false,"webLink":"","categories":[]}`

// createEventMockHandler returns an http.Handler that responds to POST /me/events
// with a valid Graph API event JSON response.
func createEventMockHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockEventJSON)) //nolint:errcheck // test helper
	})
}

// createEventBaseArgs returns the minimum required arguments for a create_event call.
func createEventBaseArgs() map[string]any {
	return map[string]any{
		"subject":        "Test",
		"start_datetime": "2026-04-15T09:00:00",
		"start_timezone": "America/New_York",
		"end_datetime":   "2026-04-15T10:00:00",
		"end_timezone":   "America/New_York",
	}
}

// createEventCapturingHandler returns an http.Handler that captures the POST
// request body for inspection and responds with a valid Graph API event JSON.
func createEventCapturingHandler(capturedBody *[]byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body) //nolint:errcheck // test helper
		*capturedBody = body
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockEventJSON)) //nolint:errcheck // test helper
	})
}

// TestCreateEvent_SetsProvenanceProperty validates that the POST request body
// includes the singleValueExtendedProperties array with the provenance property
// ID and value "true" when provenance tagging is enabled.
func TestCreateEvent_SetsProvenanceProperty(t *testing.T) {
	var captured []byte
	client, srv := newTestGraphClient(t, createEventCapturingHandler(&captured))
	defer srv.Close()

	propID := graph.BuildProvenancePropertyID("com.github.desek.outlook-local-mcp.created")
	handler := HandleCreateEvent(graph.RetryConfig{}, 30*time.Second, "America/New_York", propID)
	args := createEventBaseArgs()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %v", result.Content[0].(mcp.TextContent).Text)
	}

	body := string(captured)
	if !strings.Contains(body, "singleValueExtendedProperties") {
		t.Fatal("POST body missing singleValueExtendedProperties")
	}
	if !strings.Contains(body, graph.ProvenanceGUID) {
		t.Errorf("POST body missing provenance GUID %q", graph.ProvenanceGUID)
	}
}

// TestCreateEvent_ProvenanceDisabled validates that no singleValueExtendedProperties
// are included in the POST request body when provenance tagging is disabled
// (empty property ID).
func TestCreateEvent_ProvenanceDisabled(t *testing.T) {
	var captured []byte
	client, srv := newTestGraphClient(t, createEventCapturingHandler(&captured))
	defer srv.Close()

	handler := HandleCreateEvent(graph.RetryConfig{}, 30*time.Second, "America/New_York", "")
	args := createEventBaseArgs()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %v", result.Content[0].(mcp.TextContent).Text)
	}

	body := string(captured)
	if strings.Contains(body, "singleValueExtendedProperties") {
		t.Error("POST body should NOT contain singleValueExtendedProperties when provenance disabled")
	}
}

// TestCreateEvent_AdvisoryPresent validates that the response contains an
// advisory when attendees are provided but body is missing.
func TestCreateEvent_AdvisoryPresent(t *testing.T) {
	client, srv := newTestGraphClient(t, createEventMockHandler())
	defer srv.Close()

	handler := HandleCreateEvent(graph.RetryConfig{}, 30*time.Second, "America/New_York", "")
	args := createEventBaseArgs()
	args["attendees"] = `[{"email":"a@b.com","name":"Alice","type":"required"}]`

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %v", result.Content[0].(mcp.TextContent).Text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "Event created:") {
		t.Errorf("expected text confirmation, got: %q", text)
	}
	if !strings.Contains(text, "description") {
		t.Errorf("expected advisory mentioning description, got: %q", text)
	}
}

// TestCreateEvent_NoAdvisoryWithoutAttendees validates that the response does
// not contain an advisory when no attendees are provided.
func TestCreateEvent_NoAdvisoryWithoutAttendees(t *testing.T) {
	client, srv := newTestGraphClient(t, createEventMockHandler())
	defer srv.Close()

	handler := HandleCreateEvent(graph.RetryConfig{}, 30*time.Second, "America/New_York", "")
	args := createEventBaseArgs()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %v", result.Content[0].(mcp.TextContent).Text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "Event created:") {
		t.Errorf("expected text confirmation, got: %q", text)
	}
	if strings.Contains(text, "description") || strings.Contains(text, "location") {
		t.Error("expected no advisory when no attendees")
	}
}

// TestCreateEvent_NoAdvisoryOnlineMeetingCoversLocation validates that no
// advisory is present when attendees, body, and is_online_meeting are provided
// but location is omitted.
func TestCreateEvent_NoAdvisoryOnlineMeetingCoversLocation(t *testing.T) {
	client, srv := newTestGraphClient(t, createEventMockHandler())
	defer srv.Close()

	handler := HandleCreateEvent(graph.RetryConfig{}, 30*time.Second, "America/New_York", "")
	args := createEventBaseArgs()
	args["attendees"] = `[{"email":"a@b.com","name":"Alice","type":"required"}]`
	args["body"] = "Weekly sync agenda"
	args["is_online_meeting"] = true

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %v", result.Content[0].(mcp.TextContent).Text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "Event created:") {
		t.Errorf("expected text confirmation, got: %q", text)
	}
	if strings.Contains(text, "description") || strings.Contains(text, "Advisory") {
		t.Error("expected no advisory when is_online_meeting covers location")
	}
}

// TestCreateEvent_NoAdvisoryAllFieldsPresent validates that no advisory is
// present when attendees, body, and location are all provided.
func TestCreateEvent_NoAdvisoryAllFieldsPresent(t *testing.T) {
	client, srv := newTestGraphClient(t, createEventMockHandler())
	defer srv.Close()

	handler := HandleCreateEvent(graph.RetryConfig{}, 30*time.Second, "America/New_York", "")
	args := createEventBaseArgs()
	args["attendees"] = `[{"email":"a@b.com","name":"Alice","type":"required"}]`
	args["body"] = "Weekly sync agenda"
	args["location"] = "Conference Room A"

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %v", result.Content[0].(mcp.TextContent).Text)
	}

	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "Event created:") {
		t.Errorf("expected text confirmation, got: %q", text)
	}
	if strings.Contains(text, "description") || strings.Contains(text, "Advisory") {
		t.Error("expected no advisory when all fields present")
	}
}

// TestCreateEvent_TimezoneDefaults validates that when start_timezone and
// end_timezone are omitted, the handler uses the configured default timezone.
func TestCreateEvent_TimezoneDefaults(t *testing.T) {
	var captured []byte
	client, srv := newTestGraphClient(t, createEventCapturingHandler(&captured))
	defer srv.Close()

	handler := HandleCreateEvent(graph.RetryConfig{}, 30*time.Second, "Europe/Stockholm", "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"subject":        "Test",
		"start_datetime": "2026-04-15T09:00:00",
		"end_datetime":   "2026-04-15T10:00:00",
	}
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %v", result.Content[0].(mcp.TextContent).Text)
	}

	body := string(captured)
	if !strings.Contains(body, "Europe/Stockholm") {
		t.Errorf("POST body should contain default timezone Europe/Stockholm, got: %s", body)
	}
}

// TestCreateEvent_EndTimeDefaults30Min validates that when end_datetime is
// omitted, the handler defaults to start_datetime + 30 minutes.
func TestCreateEvent_EndTimeDefaults30Min(t *testing.T) {
	var captured []byte
	client, srv := newTestGraphClient(t, createEventCapturingHandler(&captured))
	defer srv.Close()

	handler := HandleCreateEvent(graph.RetryConfig{}, 30*time.Second, "America/New_York", "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"subject":        "Quick Sync",
		"start_datetime": "2026-04-15T09:00:00",
		"start_timezone": "America/New_York",
		"end_timezone":   "America/New_York",
	}
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %v", result.Content[0].(mcp.TextContent).Text)
	}

	body := string(captured)
	if !strings.Contains(body, "2026-04-15T09:30:00") {
		t.Errorf("POST body should contain computed end time 2026-04-15T09:30:00, got: %s", body)
	}
}

// TestCreateEvent_EndTimeDefaultsAllDay validates that when end_datetime is
// omitted and is_all_day is true, the handler defaults to start_datetime + 24h.
func TestCreateEvent_EndTimeDefaultsAllDay(t *testing.T) {
	var captured []byte
	client, srv := newTestGraphClient(t, createEventCapturingHandler(&captured))
	defer srv.Close()

	handler := HandleCreateEvent(graph.RetryConfig{}, 30*time.Second, "America/New_York", "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"subject":        "All Day Workshop",
		"start_datetime": "2026-04-15T00:00:00",
		"start_timezone": "America/New_York",
		"end_timezone":   "America/New_York",
		"is_all_day":     true,
	}
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %v", result.Content[0].(mcp.TextContent).Text)
	}

	body := string(captured)
	if !strings.Contains(body, "2026-04-16T00:00:00") {
		t.Errorf("POST body should contain computed end time 2026-04-16T00:00:00, got: %s", body)
	}
}

// TestCreateEvent_ExplicitOverridesDefaults validates that explicit timezone
// and end_datetime values take precedence over defaults.
func TestCreateEvent_ExplicitOverridesDefaults(t *testing.T) {
	var captured []byte
	client, srv := newTestGraphClient(t, createEventCapturingHandler(&captured))
	defer srv.Close()

	handler := HandleCreateEvent(graph.RetryConfig{}, 30*time.Second, "Europe/Stockholm", "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"subject":        "Explicit Test",
		"start_datetime": "2026-04-15T09:00:00",
		"start_timezone": "America/New_York",
		"end_datetime":   "2026-04-15T11:00:00",
		"end_timezone":   "America/Chicago",
	}
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %v", result.Content[0].(mcp.TextContent).Text)
	}

	body := string(captured)
	if !strings.Contains(body, "America/New_York") {
		t.Errorf("POST body should contain explicit start timezone America/New_York, got: %s", body)
	}
	if !strings.Contains(body, "America/Chicago") {
		t.Errorf("POST body should contain explicit end timezone America/Chicago, got: %s", body)
	}
	if !strings.Contains(body, "2026-04-15T11:00:00") {
		t.Errorf("POST body should contain explicit end time 2026-04-15T11:00:00, got: %s", body)
	}
	if strings.Contains(body, "Europe/Stockholm") {
		t.Error("POST body should NOT contain the default timezone when explicit values provided")
	}
}

// TestCreateEvent_MissingSubject_HintMessage validates that when subject is
// omitted, the error message includes a hint to ask the user for the event name.
func TestCreateEvent_MissingSubject_HintMessage(t *testing.T) {
	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()

	handler := HandleCreateEvent(graph.RetryConfig{}, 30*time.Second, "UTC", "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"start_datetime": "2026-04-15T09:00:00",
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when subject is missing")
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "Ask the user") {
		t.Errorf("error missing recovery hint, got: %q", text)
	}
}

// TestComputeDefaultEndTime validates the computeDefaultEndTime helper for
// both regular and all-day events.
func TestComputeDefaultEndTime(t *testing.T) {
	tests := []struct {
		name     string
		startDT  string
		isAllDay bool
		want     string
		wantErr  bool
	}{
		{"regular 30min", "2026-04-15T09:00:00", false, "2026-04-15T09:30:00", false},
		{"all-day 24h", "2026-04-15T00:00:00", true, "2026-04-16T00:00:00", false},
		{"invalid start", "not-a-date", false, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := computeDefaultEndTime(tt.startDT, tt.isAllDay)
			if (err != nil) != tt.wantErr {
				t.Fatalf("computeDefaultEndTime() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("computeDefaultEndTime() = %q, want %q", got, tt.want)
			}
		})
	}
}
