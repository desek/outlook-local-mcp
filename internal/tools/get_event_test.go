// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the get_event tool, including tool registration
// validation, parameter handling, handler construction, and full event
// serialization with attendees and body content.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// TestGetEventTool_Registration validates that the NewGetEventTool is
// properly defined with the expected name and read-only annotation.
func TestGetEventTool_Registration(t *testing.T) {
	tool := NewGetEventTool()
	if tool.Name != "calendar_get_event" {
		t.Errorf("tool name = %q, want %q", tool.Name, "calendar_get_event")
	}

	annotations := tool.Annotations
	if annotations.ReadOnlyHint == nil || !*annotations.ReadOnlyHint {
		t.Error("expected ReadOnlyHint to be true")
	}
}

// TestGetEventTool_RequiredParameters validates that event_id is marked
// as required in the tool's input schema.
func TestGetEventTool_RequiredParameters(t *testing.T) {
	tool := NewGetEventTool()
	schema := tool.InputSchema

	required := make(map[string]bool)
	for _, r := range schema.Required {
		required[r] = true
	}

	if !required["event_id"] {
		t.Error("expected event_id to be required")
	}
}

// TestGetEventTool_HasFourParameters validates that the tool defines exactly
// four parameters: event_id, timezone, account, and output.
func TestGetEventTool_HasFourParameters(t *testing.T) {
	tool := NewGetEventTool()
	schema := tool.InputSchema

	expected := []string{"event_id", "timezone", "account", "output"}
	if len(schema.Properties) != len(expected) {
		t.Errorf("expected %d properties, got %d", len(expected), len(schema.Properties))
	}
	for _, name := range expected {
		if _, ok := schema.Properties[name]; !ok {
			t.Errorf("expected property %q to be defined", name)
		}
	}
}

// TestNewHandleGetEvent_ReturnsHandler validates that NewHandleGetEvent
// returns a non-nil handler function.
func TestNewHandleGetEvent_ReturnsHandler(t *testing.T) {
	handler := NewHandleGetEvent(graph.RetryConfig{}, 0, "", "")
	if handler == nil {
		t.Fatal("expected non-nil handler function")
	}
}

// TestGetEventToolCanBeAddedToServer validates that the NewGetEventTool
// and its handler can be registered on an MCP server without error or panic.
func TestGetEventToolCanBeAddedToServer(t *testing.T) {
	s := server.NewMCPServer("test-server", "0.0.1",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)
	// Must not panic.
	s.AddTool(NewGetEventTool(), NewHandleGetEvent(graph.RetryConfig{}, 0, "", ""))
}

// TestHandleGetEvent_NoClientInContext validates that the handler returns
// a tool error when no Graph client is present in the context.
func TestHandleGetEvent_NoClientInContext(t *testing.T) {
	handler := NewHandleGetEvent(graph.RetryConfig{}, 0, "", "")
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"event_id": "AAMkAGTest123",
	}

	result, err := handler(context.Background(), request)
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

// TestHandleGetEvent_MissingEventId validates that the handler returns
// a tool error with a recovery hint when event_id is not provided.
func TestHandleGetEvent_MissingEventId(t *testing.T) {
	handler := NewHandleGetEvent(graph.RetryConfig{}, 0, "", "")
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{}

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when event_id is missing")
	}
	text := result.Content[0].(mcp.TextContent).Text
	if !strings.Contains(text, "calendar_list_events or calendar_search_events") {
		t.Errorf("error missing recovery hint, got: %q", text)
	}
}

// TestSerializeEventFull_WithAttendees validates the full event serialization
// path used by get_event, including body and attendees. This tests the
// additional serialization logic beyond the base SerializeEvent function.
func TestSerializeEventFull_WithAttendees(t *testing.T) {
	event := models.NewEvent()
	event.SetId(ptr("evt-456"))
	event.SetSubject(ptr("Team Standup"))

	// Body.
	body := models.NewItemBody()
	bodyType := models.HTML_BODYTYPE
	body.SetContentType(&bodyType)
	body.SetContent(ptr("<p>Agenda</p>"))
	event.SetBody(body)
	event.SetBodyPreview(ptr("Agenda"))

	// Attendees.
	att1Email := models.NewEmailAddress()
	att1Email.SetName(ptr("Bob"))
	att1Email.SetAddress(ptr("bob@example.com"))
	att1 := models.NewAttendee()
	att1.SetEmailAddress(att1Email)
	attType := models.REQUIRED_ATTENDEETYPE
	att1.SetTypeEscaped(&attType)
	respStatus := models.NewResponseStatus()
	respType := models.ACCEPTED_RESPONSETYPE
	respStatus.SetResponse(&respType)
	att1.SetStatus(respStatus)

	att2Email := models.NewEmailAddress()
	att2Email.SetName(ptr("Carol"))
	att2Email.SetAddress(ptr("carol@example.com"))
	att2 := models.NewAttendee()
	att2.SetEmailAddress(att2Email)
	optType := models.OPTIONAL_ATTENDEETYPE
	att2.SetTypeEscaped(&optType)

	event.SetAttendees([]models.Attendeeable{att1, att2})

	// HasAttachments.
	event.SetHasAttachments(boolPtr(true))

	// CreatedDateTime.
	created := time.Date(2026, 3, 10, 14, 0, 0, 0, time.UTC)
	event.SetCreatedDateTime(&created)

	// Run the base serialization first then add get_event-specific fields.
	result := graph.SerializeEvent(event)

	// Add body (matching get_event handler logic).
	if b := event.GetBody(); b != nil {
		bodyMap := map[string]string{
			"content": graph.SafeStr(b.GetContent()),
		}
		if ct := b.GetContentType(); ct != nil {
			bodyMap["contentType"] = ct.String()
		}
		result["body"] = bodyMap
	}
	result["bodyPreview"] = graph.SafeStr(event.GetBodyPreview())

	// Add attendees (matching get_event handler logic).
	if attendees := event.GetAttendees(); attendees != nil {
		attList := make([]map[string]string, 0, len(attendees))
		for _, att := range attendees {
			attMap := map[string]string{
				"name": "", "email": "", "type": "", "response": "",
			}
			if ea := att.GetEmailAddress(); ea != nil {
				attMap["name"] = graph.SafeStr(ea.GetName())
				attMap["email"] = graph.SafeStr(ea.GetAddress())
			}
			if at := att.GetTypeEscaped(); at != nil {
				attMap["type"] = at.String()
			}
			if status := att.GetStatus(); status != nil {
				if resp := status.GetResponse(); resp != nil {
					attMap["response"] = resp.String()
				}
			}
			attList = append(attList, attMap)
		}
		result["attendees"] = attList
	}

	result["hasAttachments"] = graph.SafeBool(event.GetHasAttachments())
	if cd := event.GetCreatedDateTime(); cd != nil {
		result["createdDateTime"] = cd.Format(time.RFC3339)
	}

	// Verify the serialized result.
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if parsed["id"] != "evt-456" {
		t.Errorf("id = %q, want %q", parsed["id"], "evt-456")
	}
	if parsed["bodyPreview"] != "Agenda" {
		t.Errorf("bodyPreview = %q, want %q", parsed["bodyPreview"], "Agenda")
	}
	if parsed["hasAttachments"] != true {
		t.Errorf("hasAttachments = %v, want true", parsed["hasAttachments"])
	}

	// Verify body.
	bodyParsed, ok := parsed["body"].(map[string]any)
	if !ok {
		t.Fatal("body is not a map")
	}
	if bodyParsed["contentType"] != "html" {
		t.Errorf("body.contentType = %q, want %q", bodyParsed["contentType"], "html")
	}
	if bodyParsed["content"] != "<p>Agenda</p>" {
		t.Errorf("body.content = %q, want %q", bodyParsed["content"], "<p>Agenda</p>")
	}

	// Verify attendees.
	attendeesParsed, ok := parsed["attendees"].([]any)
	if !ok {
		t.Fatal("attendees is not an array")
	}
	if len(attendeesParsed) != 2 {
		t.Fatalf("expected 2 attendees, got %d", len(attendeesParsed))
	}
	att1Parsed := attendeesParsed[0].(map[string]any)
	if att1Parsed["name"] != "Bob" {
		t.Errorf("attendee[0].name = %q, want %q", att1Parsed["name"], "Bob")
	}
	if att1Parsed["email"] != "bob@example.com" {
		t.Errorf("attendee[0].email = %q, want %q", att1Parsed["email"], "bob@example.com")
	}
	if att1Parsed["type"] != "required" {
		t.Errorf("attendee[0].type = %q, want %q", att1Parsed["type"], "required")
	}
	if att1Parsed["response"] != "accepted" {
		t.Errorf("attendee[0].response = %q, want %q", att1Parsed["response"], "accepted")
	}

	att2Parsed := attendeesParsed[1].(map[string]any)
	if att2Parsed["name"] != "Carol" {
		t.Errorf("attendee[1].name = %q, want %q", att2Parsed["name"], "Carol")
	}
	if att2Parsed["type"] != "optional" {
		t.Errorf("attendee[1].type = %q, want %q", att2Parsed["type"], "optional")
	}
}

// TestSerializeEventFull_NilAttendeeFields validates that the attendee
// extraction pattern handles partially nil attendee data without panicking.
// Attendees may have nil EmailAddress, nil TypeEscaped, or nil Status.
func TestSerializeEventFull_NilAttendeeFields(t *testing.T) {
	event := models.NewEvent()
	event.SetId(ptr("evt-nil"))

	// Attendee with all nil optional fields.
	att := models.NewAttendee()
	event.SetAttendees([]models.Attendeeable{att})

	result := graph.SerializeEvent(event)

	// Simulate get_event attendee extraction.
	if attendees := event.GetAttendees(); attendees != nil {
		attList := make([]map[string]string, 0, len(attendees))
		for _, a := range attendees {
			attMap := map[string]string{
				"name": "", "email": "", "type": "", "response": "",
			}
			if ea := a.GetEmailAddress(); ea != nil {
				attMap["name"] = graph.SafeStr(ea.GetName())
				attMap["email"] = graph.SafeStr(ea.GetAddress())
			}
			if at := a.GetTypeEscaped(); at != nil {
				attMap["type"] = at.String()
			}
			if status := a.GetStatus(); status != nil {
				if resp := status.GetResponse(); resp != nil {
					attMap["response"] = resp.String()
				}
			}
			attList = append(attList, attMap)
		}
		result["attendees"] = attList
	}

	// Verify no panic occurred and defaults are applied.
	attList, ok := result["attendees"].([]map[string]string)
	if !ok {
		t.Fatal("attendees is not []map[string]string")
	}
	if len(attList) != 1 {
		t.Fatalf("expected 1 attendee, got %d", len(attList))
	}
	if attList[0]["name"] != "" {
		t.Errorf("name = %q, want %q", attList[0]["name"], "")
	}
	if attList[0]["email"] != "" {
		t.Errorf("email = %q, want %q", attList[0]["email"], "")
	}
	if attList[0]["type"] != "" {
		t.Errorf("type = %q, want %q", attList[0]["type"], "")
	}
	if attList[0]["response"] != "" {
		t.Errorf("response = %q, want %q", attList[0]["response"], "")
	}
}

// TestSerializeEventFull_NilBodyFields validates that the body extraction
// pattern handles a nil body without panicking.
func TestSerializeEventFull_NilBodyFields(t *testing.T) {
	event := models.NewEvent()
	event.SetId(ptr("evt-nobody"))

	result := graph.SerializeEvent(event)

	// Simulate get_event body extraction.
	if body := event.GetBody(); body != nil {
		t.Error("expected nil body")
	}

	// Body should not be present in result since it was nil.
	if _, ok := result["body"]; ok {
		t.Error("body should not be present when event body is nil")
	}

	// BodyPreview should be empty string via SafeStr.
	result["bodyPreview"] = graph.SafeStr(event.GetBodyPreview())
	if result["bodyPreview"] != "" {
		t.Errorf("bodyPreview = %q, want %q", result["bodyPreview"], "")
	}
}

// TestGetEvent_OutputSummary validates that the default output mode returns
// summary format with 12 keys, flat fields, and no HTML body content.
func TestGetEvent_OutputSummary(t *testing.T) {
	event := models.NewEvent()
	event.SetId(ptr("evt-summary"))
	event.SetSubject(ptr("Team Standup"))
	event.SetBodyPreview(ptr("Agenda items"))

	body := models.NewItemBody()
	bodyType := models.HTML_BODYTYPE
	body.SetContentType(&bodyType)
	body.SetContent(ptr("<p>Agenda</p>"))
	event.SetBody(body)

	event.SetHasAttachments(boolPtr(true))

	eventType := models.SINGLEINSTANCE_EVENTTYPE
	event.SetTypeEscaped(&eventType)

	att1Email := models.NewEmailAddress()
	att1Email.SetName(ptr("Bob"))
	att1Email.SetAddress(ptr("bob@example.com"))
	att1 := models.NewAttendee()
	att1.SetEmailAddress(att1Email)
	attType := models.REQUIRED_ATTENDEETYPE
	att1.SetTypeEscaped(&attType)
	respStatus := models.NewResponseStatus()
	respType := models.ACCEPTED_RESPONSETYPE
	respStatus.SetResponse(&respType)
	att1.SetStatus(respStatus)
	event.SetAttendees([]models.Attendeeable{att1})

	result := graph.SerializeSummaryGetEvent(event)

	// Verify 13 keys (12 base + displayTime).
	if len(result) != 13 {
		t.Errorf("expected 13 keys in summary, got %d", len(result))
	}

	// Verify no HTML body.
	if _, ok := result["body"]; ok {
		t.Error("summary should not contain body")
	}

	// Verify bodyPreview is present.
	if result["bodyPreview"] != "Agenda items" {
		t.Errorf("bodyPreview = %v, want %q", result["bodyPreview"], "Agenda items")
	}

	// Verify attendees have only name + response (no email, no type).
	attendees, ok := result["attendees"].([]map[string]string)
	if !ok {
		t.Fatal("attendees should be []map[string]string")
	}
	if len(attendees) != 1 {
		t.Fatalf("expected 1 attendee, got %d", len(attendees))
	}
	if attendees[0]["name"] != "Bob" {
		t.Errorf("attendee name = %q, want %q", attendees[0]["name"], "Bob")
	}
	if attendees[0]["response"] != "accepted" {
		t.Errorf("attendee response = %q, want %q", attendees[0]["response"], "accepted")
	}
	if _, ok := attendees[0]["email"]; ok {
		t.Error("summary attendees should not contain email")
	}
	if _, ok := attendees[0]["type"]; ok {
		t.Error("summary attendees should not contain type")
	}
}

// TestGetEvent_OutputRaw validates that output=raw returns the full field set
// including body, attendees with email/type, and metadata fields.
func TestGetEvent_OutputRaw(t *testing.T) {
	event := models.NewEvent()
	event.SetId(ptr("evt-raw"))
	event.SetSubject(ptr("Team Standup"))
	event.SetBodyPreview(ptr("Agenda items"))

	body := models.NewItemBody()
	bodyType := models.HTML_BODYTYPE
	body.SetContentType(&bodyType)
	body.SetContent(ptr("<p>Agenda</p>"))
	event.SetBody(body)

	event.SetHasAttachments(boolPtr(true))

	att1Email := models.NewEmailAddress()
	att1Email.SetName(ptr("Bob"))
	att1Email.SetAddress(ptr("bob@example.com"))
	att1 := models.NewAttendee()
	att1.SetEmailAddress(att1Email)
	attType := models.REQUIRED_ATTENDEETYPE
	att1.SetTypeEscaped(&attType)
	respStatus := models.NewResponseStatus()
	respType := models.ACCEPTED_RESPONSETYPE
	respStatus.SetResponse(&respType)
	att1.SetStatus(respStatus)
	event.SetAttendees([]models.Attendeeable{att1})

	created := time.Date(2026, 3, 10, 14, 0, 0, 0, time.UTC)
	event.SetCreatedDateTime(&created)

	// Simulate the raw serialization path from the get_event handler.
	result := graph.SerializeEvent(event)

	if b := event.GetBody(); b != nil {
		bodyMap := map[string]string{
			"content": graph.SafeStr(b.GetContent()),
		}
		if ct := b.GetContentType(); ct != nil {
			bodyMap["contentType"] = ct.String()
		}
		result["body"] = bodyMap
	}
	result["bodyPreview"] = graph.SafeStr(event.GetBodyPreview())

	if attendees := event.GetAttendees(); attendees != nil {
		attList := make([]map[string]string, 0, len(attendees))
		for _, att := range attendees {
			attMap := map[string]string{
				"name": "", "email": "", "type": "", "response": "",
			}
			if ea := att.GetEmailAddress(); ea != nil {
				attMap["name"] = graph.SafeStr(ea.GetName())
				attMap["email"] = graph.SafeStr(ea.GetAddress())
			}
			if at := att.GetTypeEscaped(); at != nil {
				attMap["type"] = at.String()
			}
			if status := att.GetStatus(); status != nil {
				if resp := status.GetResponse(); resp != nil {
					attMap["response"] = resp.String()
				}
			}
			attList = append(attList, attMap)
		}
		result["attendees"] = attList
	}
	result["hasAttachments"] = graph.SafeBool(event.GetHasAttachments())

	// Round-trip via JSON to match handler behavior.
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// Verify body is present with HTML content.
	bodyParsed, ok := parsed["body"].(map[string]any)
	if !ok {
		t.Fatal("raw format should contain body as map")
	}
	if bodyParsed["content"] != "<p>Agenda</p>" {
		t.Errorf("body.content = %v, want %q", bodyParsed["content"], "<p>Agenda</p>")
	}

	// Verify attendees include email and type.
	attendeesParsed, ok := parsed["attendees"].([]any)
	if !ok {
		t.Fatal("attendees is not an array")
	}
	att := attendeesParsed[0].(map[string]any)
	if att["email"] != "bob@example.com" {
		t.Errorf("attendee email = %v, want %q", att["email"], "bob@example.com")
	}
	if att["type"] != "required" {
		t.Errorf("attendee type = %v, want %q", att["type"], "required")
	}
}

// TestGetEvent_CreatedByMcpInResponse validates that when get_event returns an
// event with the provenance extended property, the JSON response includes
// "createdByMcp": true.
func TestGetEvent_CreatedByMcpInResponse(t *testing.T) {
	propID := graph.BuildProvenancePropertyID("com.github.desek.outlook-local-mcp.created")

	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper
		fmt.Fprintf(w, `{"id":"evt-prov","subject":"Tagged","singleValueExtendedProperties":[{"id":"%s","value":"true"}]}`, propID)
	}))
	defer srv.Close()

	handler := NewHandleGetEvent(graph.RetryConfig{}, 30*time.Second, "", propID)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"event_id": "evt-prov",
		"output":   "summary",
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %v", result.Content[0].(mcp.TextContent).Text)
	}

	var parsed map[string]any
	text := result.Content[0].(mcp.TextContent).Text
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	got, exists := parsed["createdByMcp"]
	if !exists {
		t.Fatal("createdByMcp key not present in response")
	}
	if got != true {
		t.Errorf("createdByMcp = %v, want true", got)
	}
}

// TestHandleGetEvent_DefaultsToServerTimezone verifies that when the caller
// omits the timezone parameter, the handler falls back to the server-configured
// default timezone and sends it as the Graph API Prefer: outlook.timezone
// header so event times render in the operator's local timezone rather than
// UTC. Regression for TEST-REPORT.md F3.
func TestHandleGetEvent_DefaultsToServerTimezone(t *testing.T) {
	var capturedPrefer string
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPrefer = r.Header.Get("Prefer")
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper
		fmt.Fprint(w, `{"id":"evt-tz","subject":"TZ Test","start":{"dateTime":"2026-04-19T12:00:00.0000000","timeZone":"Europe/Stockholm"},"end":{"dateTime":"2026-04-19T13:00:00.0000000","timeZone":"Europe/Stockholm"}}`)
	}))
	defer srv.Close()

	handler := NewHandleGetEvent(graph.RetryConfig{}, 30*time.Second, "Europe/Stockholm", "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"event_id": "evt-tz"}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got: %v", result.Content[0].(mcp.TextContent).Text)
	}
	if !strings.Contains(capturedPrefer, `outlook.timezone="Europe/Stockholm"`) {
		t.Errorf("Prefer header = %q, want to contain outlook.timezone=\"Europe/Stockholm\"", capturedPrefer)
	}
}

// TestHandleGetEvent_ExplicitTimezoneOverridesDefault verifies that when the
// caller provides a timezone, the handler forwards the caller's value rather
// than the server default.
func TestHandleGetEvent_ExplicitTimezoneOverridesDefault(t *testing.T) {
	var capturedPrefer string
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPrefer = r.Header.Get("Prefer")
		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck // test helper
		fmt.Fprint(w, `{"id":"evt-tz","subject":"TZ Test"}`)
	}))
	defer srv.Close()

	handler := NewHandleGetEvent(graph.RetryConfig{}, 30*time.Second, "Europe/Stockholm", "")
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"event_id": "evt-tz",
		"timezone": "America/New_York",
	}

	ctx := auth.WithGraphClient(context.Background(), client)
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got: %v", result.Content[0].(mcp.TextContent).Text)
	}
	if !strings.Contains(capturedPrefer, `outlook.timezone="America/New_York"`) {
		t.Errorf("Prefer header = %q, want caller-provided timezone", capturedPrefer)
	}
}
