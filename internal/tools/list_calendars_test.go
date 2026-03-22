// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the list_calendars tool, including the
// SerializeCalendar function, tool registration, and handler construction.
package tools

import (
	"context"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// TestListCalendarsTool_Registration validates that the NewListCalendarsTool is
// properly defined with the expected name and read-only annotation.
func TestListCalendarsTool_Registration(t *testing.T) {
	tool := NewListCalendarsTool()
	if tool.Name != "calendar_list" {
		t.Errorf("tool name = %q, want %q", tool.Name, "calendar_list")
	}

	annotations := tool.Annotations
	if annotations.ReadOnlyHint == nil || !*annotations.ReadOnlyHint {
		t.Error("expected ReadOnlyHint to be true")
	}
}

// TestSerializeCalendar_AllFields validates that SerializeCalendar correctly
// extracts all fields from a fully populated Calendarable, including the
// nested owner object.
func TestSerializeCalendar_AllFields(t *testing.T) {
	cal := models.NewCalendar()
	cal.SetId(ptr("cal-123"))
	cal.SetName(ptr("Work Calendar"))
	cal.SetHexColor(ptr("#0078D4"))
	cal.SetIsDefaultCalendar(boolPtr(true))
	cal.SetCanEdit(boolPtr(true))

	color := models.LIGHTBLUE_CALENDARCOLOR
	cal.SetColor(&color)

	owner := models.NewEmailAddress()
	owner.SetName(ptr("Alice"))
	owner.SetAddress(ptr("alice@example.com"))
	cal.SetOwner(owner)

	result := SerializeCalendar(cal)

	if got := result["id"]; got != "cal-123" {
		t.Errorf("id = %q, want %q", got, "cal-123")
	}
	if got := result["name"]; got != "Work Calendar" {
		t.Errorf("name = %q, want %q", got, "Work Calendar")
	}
	if got := result["hexColor"]; got != "#0078D4" {
		t.Errorf("hexColor = %q, want %q", got, "#0078D4")
	}
	if got := result["isDefaultCalendar"]; got != true {
		t.Errorf("isDefaultCalendar = %v, want true", got)
	}
	if got := result["canEdit"]; got != true {
		t.Errorf("canEdit = %v, want true", got)
	}
	if got := result["color"]; got != "lightBlue" {
		t.Errorf("color = %q, want %q", got, "lightBlue")
	}

	ownerMap, ok := result["owner"].(map[string]string)
	if !ok {
		t.Fatal("owner is not map[string]string")
	}
	if ownerMap["name"] != "Alice" {
		t.Errorf("owner.name = %q, want %q", ownerMap["name"], "Alice")
	}
	if ownerMap["address"] != "alice@example.com" {
		t.Errorf("owner.address = %q, want %q", ownerMap["address"], "alice@example.com")
	}
}

// TestSerializeCalendar_NilFields validates that SerializeCalendar does not
// panic when all optional fields on the Calendarable return nil. A freshly
// constructed Calendar with no setters called has nil for all optional fields.
func TestSerializeCalendar_NilFields(t *testing.T) {
	cal := models.NewCalendar()

	result := SerializeCalendar(cal)

	if got := result["id"]; got != "" {
		t.Errorf("id = %q, want %q", got, "")
	}
	if got := result["name"]; got != "" {
		t.Errorf("name = %q, want %q", got, "")
	}
	if got := result["hexColor"]; got != "" {
		t.Errorf("hexColor = %q, want %q", got, "")
	}
	if got := result["isDefaultCalendar"]; got != false {
		t.Errorf("isDefaultCalendar = %v, want false", got)
	}
	if got := result["canEdit"]; got != false {
		t.Errorf("canEdit = %v, want false", got)
	}
	if got := result["color"]; got != "" {
		t.Errorf("color = %q, want %q", got, "")
	}

	ownerMap, ok := result["owner"].(map[string]string)
	if !ok {
		t.Fatal("owner is not map[string]string")
	}
	if ownerMap["name"] != "" {
		t.Errorf("owner.name = %q, want %q", ownerMap["name"], "")
	}
	if ownerMap["address"] != "" {
		t.Errorf("owner.address = %q, want %q", ownerMap["address"], "")
	}
}

// TestListCalendarsTool_HasParameters validates that the NewListCalendarsTool
// defines account and output parameters, both optional.
func TestListCalendarsTool_HasParameters(t *testing.T) {
	tool := NewListCalendarsTool()
	schema := tool.InputSchema

	if len(schema.Required) != 0 {
		t.Errorf("expected 0 required properties, got %d", len(schema.Required))
	}

	if _, ok := schema.Properties["account"]; !ok {
		t.Error("expected account property to be defined")
	}
	if _, ok := schema.Properties["output"]; !ok {
		t.Error("expected output property to be defined")
	}
}

// TestNewHandleListCalendars_ReturnsHandler validates that NewHandleListCalendars
// returns a non-nil handler function.
func TestNewHandleListCalendars_ReturnsHandler(t *testing.T) {
	handler := NewHandleListCalendars(graph.RetryConfig{}, 0)
	if handler == nil {
		t.Fatal("expected non-nil handler function")
	}
}

// TestListCalendarsToolCanBeAddedToServer validates that the
// NewListCalendarsTool and its handler can be registered on an MCP server
// without error or panic.
func TestListCalendarsToolCanBeAddedToServer(t *testing.T) {
	s := server.NewMCPServer("test-server", "0.0.1",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)
	// Must not panic.
	s.AddTool(NewListCalendarsTool(), NewHandleListCalendars(graph.RetryConfig{}, 0))
}

// TestHandleListCalendars_NoClientInContext validates that the handler returns
// a tool error when no Graph client is present in the context.
func TestHandleListCalendars_NoClientInContext(t *testing.T) {
	handler := NewHandleListCalendars(graph.RetryConfig{}, 0)
	request := mcp.CallToolRequest{}

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

// TestHandleListCalendars_WithClientInContext validates that the handler
// proceeds past the client lookup when a Graph client is in the context.
// The handler will fail at the Graph API call, but should not return the
// "no account selected" error.
func TestHandleListCalendars_WithClientInContext(t *testing.T) {
	handler := NewHandleListCalendars(graph.RetryConfig{}, 0)
	request := mcp.CallToolRequest{}

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The call may fail (no mock response), but it should not be "no account selected".
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		if text == "no account selected" {
			t.Error("expected error other than 'no account selected' when client is in context")
		}
	}
}
