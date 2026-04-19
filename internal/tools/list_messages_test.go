// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains tests for the list_messages tool, including the
// buildMessageFilter function, tool registration, handler construction,
// and parameter validation.
package tools

import (
	"context"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// TestListMessagesTool_Registration validates that NewListMessagesTool is
// properly defined with the expected name and read-only annotation.
func TestListMessagesTool_Registration(t *testing.T) {
	tool := NewListMessagesTool()
	if tool.Name != "mail_list_messages" {
		t.Errorf("tool name = %q, want %q", tool.Name, "mail_list_messages")
	}

	annotations := tool.Annotations
	if annotations.ReadOnlyHint == nil || !*annotations.ReadOnlyHint {
		t.Error("expected ReadOnlyHint to be true")
	}
}

// TestListMessagesTool_HasParameters validates that NewListMessagesTool defines
// all expected parameters, with none required.
func TestListMessagesTool_HasParameters(t *testing.T) {
	tool := NewListMessagesTool()
	schema := tool.InputSchema

	if len(schema.Required) != 0 {
		t.Errorf("expected 0 required properties, got %d", len(schema.Required))
	}

	expectedParams := []string{
		"folder_id", "start_datetime", "end_datetime", "from",
		"conversation_id", "is_read", "is_draft", "has_attachments",
		"importance", "flag_status",
		"max_results", "timezone", "account", "output",
	}
	for _, param := range expectedParams {
		if _, ok := schema.Properties[param]; !ok {
			t.Errorf("expected %q property to be defined", param)
		}
	}
}

// TestNewHandleListMessages_ReturnsHandler validates that NewHandleListMessages
// returns a non-nil handler function.
func TestNewHandleListMessages_ReturnsHandler(t *testing.T) {
	handler := NewHandleListMessages(graph.RetryConfig{}, 0, "")
	if handler == nil {
		t.Fatal("expected non-nil handler function")
	}
}

// TestListMessagesToolCanBeAddedToServer validates that NewListMessagesTool and
// its handler can be registered on an MCP server without error or panic.
func TestListMessagesToolCanBeAddedToServer(t *testing.T) {
	s := server.NewMCPServer("test-server", "0.0.1",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)
	s.AddTool(NewListMessagesTool(), NewHandleListMessages(graph.RetryConfig{}, 0, ""))
}

// TestListMessages_DefaultFolder validates that the handler proceeds past the
// client lookup when a Graph client is in the context and no folder_id is
// specified. The handler will fail at the Graph API call (no mock response),
// but should not return the "no account selected" error.
func TestListMessages_DefaultFolder(t *testing.T) {
	handler := NewHandleListMessages(graph.RetryConfig{}, 0, "")
	request := mcp.CallToolRequest{}

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		if text == "no account selected" {
			t.Error("expected error other than 'no account selected' when client is in context")
		}
	}
}

// TestListMessages_SpecificFolder validates that the handler proceeds past the
// client lookup when a folder_id is specified. The handler will fail at the
// Graph API call (no mock response), but should not return the "no account
// selected" error.
func TestListMessages_SpecificFolder(t *testing.T) {
	handler := NewHandleListMessages(graph.RetryConfig{}, 0, "")
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"folder_id": "AAMkAGI2TGULAAA=",
	}

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		if text == "no account selected" {
			t.Error("expected error other than 'no account selected' when client is in context")
		}
	}
}

// TestListMessages_NoClient validates that the handler returns a tool error
// when no Graph client is present in the context.
func TestListMessages_NoClient(t *testing.T) {
	handler := NewHandleListMessages(graph.RetryConfig{}, 0, "")
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

// TestListMessages_DateRangeFilter validates that buildMessageFilter constructs
// the correct OData $filter for date range parameters.
func TestListMessages_DateRangeFilter(t *testing.T) {
	filter := buildMessageFilter(messageFilterOptions{
		startDatetime: "2026-03-01T00:00:00Z",
		endDatetime:   "2026-03-15T23:59:59Z",
	})
	expected := "receivedDateTime ge 2026-03-01T00:00:00Z and receivedDateTime le 2026-03-15T23:59:59Z"
	if filter != expected {
		t.Errorf("filter = %q, want %q", filter, expected)
	}
}

// TestListMessages_ConversationIdFilter validates that buildMessageFilter
// constructs the correct OData $filter for conversation ID.
func TestListMessages_ConversationIdFilter(t *testing.T) {
	filter := buildMessageFilter(messageFilterOptions{conversationID: "AAQkAGI2TGuLAAA="})
	expected := "conversationId eq 'AAQkAGI2TGuLAAA='"
	if filter != expected {
		t.Errorf("filter = %q, want %q", filter, expected)
	}
}

// TestListMessages_FromFilter validates that buildMessageFilter constructs the
// correct OData $filter for sender email address.
func TestListMessages_FromFilter(t *testing.T) {
	filter := buildMessageFilter(messageFilterOptions{fromEmail: "alice@contoso.com"})
	expected := "from/emailAddress/address eq 'alice@contoso.com'"
	if filter != expected {
		t.Errorf("filter = %q, want %q", filter, expected)
	}
}

// TestListMessages_IsReadFilter validates that buildMessageFilter emits an
// isRead eq clause only when a non-nil pointer is supplied.
func TestListMessages_IsReadFilter(t *testing.T) {
	filter := buildMessageFilter(messageFilterOptions{isRead: boolPtr(false)})
	expected := "isRead eq false"
	if filter != expected {
		t.Errorf("filter = %q, want %q", filter, expected)
	}
	filter = buildMessageFilter(messageFilterOptions{isRead: boolPtr(true)})
	expected = "isRead eq true"
	if filter != expected {
		t.Errorf("filter = %q, want %q", filter, expected)
	}
	if buildMessageFilter(messageFilterOptions{}) != "" {
		t.Error("expected nil is_read pointer to produce no filter clause")
	}
}

// TestListMessages_IsDraftFilter validates that buildMessageFilter emits the
// expected isDraft clause when the tri-state pointer is set.
func TestListMessages_IsDraftFilter(t *testing.T) {
	filter := buildMessageFilter(messageFilterOptions{isDraft: boolPtr(true)})
	expected := "isDraft eq true"
	if filter != expected {
		t.Errorf("filter = %q, want %q", filter, expected)
	}
}

// TestListMessages_HasAttachmentsFilter validates the hasAttachments clause.
func TestListMessages_HasAttachmentsFilter(t *testing.T) {
	filter := buildMessageFilter(messageFilterOptions{hasAttachments: boolPtr(true)})
	expected := "hasAttachments eq true"
	if filter != expected {
		t.Errorf("filter = %q, want %q", filter, expected)
	}
}

// TestListMessages_ImportanceFilter validates the importance eq clause.
func TestListMessages_ImportanceFilter(t *testing.T) {
	filter := buildMessageFilter(messageFilterOptions{importance: "high"})
	expected := "importance eq 'high'"
	if filter != expected {
		t.Errorf("filter = %q, want %q", filter, expected)
	}
}

// TestListMessages_FlagStatusFilter validates the flag/flagStatus clause.
func TestListMessages_FlagStatusFilter(t *testing.T) {
	filter := buildMessageFilter(messageFilterOptions{flagStatus: "flagged"})
	expected := "flag/flagStatus eq 'flagged'"
	if filter != expected {
		t.Errorf("filter = %q, want %q", filter, expected)
	}
}

// TestListMessages_CombinedFilters validates that buildMessageFilter ANDs
// multiple filter conditions together, including the Phase 3 additions.
func TestListMessages_CombinedFilters(t *testing.T) {
	filter := buildMessageFilter(messageFilterOptions{
		startDatetime:  "2026-03-01T00:00:00Z",
		endDatetime:    "2026-03-15T23:59:59Z",
		fromEmail:      "alice@contoso.com",
		conversationID: "AAQkAGI2TGuLAAA=",
		isRead:         boolPtr(false),
		isDraft:        boolPtr(false),
		hasAttachments: boolPtr(true),
		importance:     "high",
		flagStatus:     "flagged",
	})
	expected := "receivedDateTime ge 2026-03-01T00:00:00Z and receivedDateTime le 2026-03-15T23:59:59Z and from/emailAddress/address eq 'alice@contoso.com' and conversationId eq 'AAQkAGI2TGuLAAA=' and isRead eq false and isDraft eq false and hasAttachments eq true and importance eq 'high' and flag/flagStatus eq 'flagged'"
	if filter != expected {
		t.Errorf("filter = %q, want %q", filter, expected)
	}
}

// TestListMessages_NoFilters validates that buildMessageFilter returns an empty
// string when no filter parameters are provided.
func TestListMessages_NoFilters(t *testing.T) {
	filter := buildMessageFilter(messageFilterOptions{})
	if filter != "" {
		t.Errorf("filter = %q, want empty string", filter)
	}
}

// TestListMessages_ProvenanceFilter validates that buildMessageFilter appends
// the singleValueExtendedProperties/any clause when a provenance property ID
// is supplied on the filter options, and that the clause is ANDed with other
// filters.
func TestListMessages_ProvenanceFilter(t *testing.T) {
	propID := "String {73830cef-ea4a-4459-b555-80a4619f667d} Name test.tag"
	filter := buildMessageFilter(messageFilterOptions{provenancePropertyID: propID})
	expected := "singleValueExtendedProperties/any(ep: ep/id eq '" + propID + "' and ep/value eq 'true')"
	if filter != expected {
		t.Errorf("filter = %q, want %q", filter, expected)
	}

	filter = buildMessageFilter(messageFilterOptions{
		fromEmail:            "alice@contoso.com",
		provenancePropertyID: propID,
	})
	expected = "from/emailAddress/address eq 'alice@contoso.com' and " +
		"singleValueExtendedProperties/any(ep: ep/id eq '" + propID + "' and ep/value eq 'true')"
	if filter != expected {
		t.Errorf("combined filter = %q, want %q", filter, expected)
	}
}

// TestListMessages_ProvenanceNoTag validates that the handler returns an error
// when the caller requests provenance=true but the server was not configured
// with a provenance tag (propertyID is empty).
func TestListMessages_ProvenanceNoTag(t *testing.T) {
	handler := NewHandleListMessages(graph.RetryConfig{}, 0, "")
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"provenance": true,
	}

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error when provenance=true but tag unset")
	}
	text := result.Content[0].(mcp.TextContent).Text
	expected := "provenance filter requested but provenance tagging is not configured on the server"
	if text != expected {
		t.Errorf("error text = %q, want %q", text, expected)
	}
}

// TestListMessages_MaxResultsClamped validates that max_results values exceeding
// 100 are clamped to 100. This is tested indirectly by verifying the handler
// does not error for a value over 100 and still proceeds to the Graph API call.
func TestListMessages_MaxResultsClamped(t *testing.T) {
	handler := NewHandleListMessages(graph.RetryConfig{}, 0, "")
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"max_results": float64(200),
	}

	client, srv := newTestGraphClient(t, nil)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	result, err := handler(ctx, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The call should proceed past validation (may fail at Graph API call).
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		if text == "no account selected" {
			t.Error("expected error other than 'no account selected'")
		}
	}
}
