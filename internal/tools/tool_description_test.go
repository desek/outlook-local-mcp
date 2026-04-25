// Package tools_test validates that each aggregate domain tool's top-level
// description lists every supported operation verb, satisfying CR-0060 AC-4
// and FR-3.
//
// After CR-0060, the four aggregate tools (calendar, mail, account, system)
// replace the former individual tools. Their top-level descriptions must
// enumerate every verb so LLM clients can discover operations without calling
// help.
package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/audit"
	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/config"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/observability"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	server "github.com/desek/outlook-local-mcp/internal/server"
)

// buildDescriptionTestServer registers all four domain tools with the given
// config and returns the server for description inspection.
func buildDescriptionTestServer(t *testing.T, cfg config.Config) *mcpserver.MCPServer {
	t.Helper()

	s := mcpserver.NewMCPServer("test-descriptions", "0.0.0",
		mcpserver.WithToolCapabilities(false),
	)

	meter := noop.NewMeterProvider().Meter("test")
	m, err := observability.InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics: %v", err)
	}
	tracer := tracenoop.NewTracerProvider().Tracer("test")
	identityMW := func(h mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc { return h }

	r := auth.NewAccountRegistry()
	_ = r.Add(&auth.AccountEntry{Label: "default", Authenticated: true})

	audit.InitAuditLog(false, "")

	server.RegisterTools(s, graph.RetryConfig{}, 30*time.Second, m, tracer, false, identityMW, r, cfg, nil)
	return s
}

// getToolDescription returns the Description field of the named tool.
func getToolDescription(t *testing.T, s *mcpserver.MCPServer, name string) string {
	t.Helper()
	st := s.ListTools()
	entry, ok := st[name]
	if !ok {
		t.Fatalf("tool %q not registered", name)
	}
	return entry.Tool.Description
}

// TestTopLevelDescription_Calendar verifies that the calendar aggregate tool
// description lists every calendar verb (AC-4 / FR-3).
func TestTopLevelDescription_Calendar(t *testing.T) {
	s := buildDescriptionTestServer(t, config.Config{
		AuthRecordPath: "/tmp/test",
		CacheName:      "test",
		AuthMethod:     "browser",
	})
	desc := getToolDescription(t, s, "calendar")

	// All 15 calendar verbs (including help) must appear in the description.
	verbs := []string{
		"help",
		"list_calendars",
		"list_events",
		"get_event",
		"search_events",
		"create_event",
		"update_event",
		"delete_event",
		"respond_event",
		"reschedule_event",
		"create_meeting",
		"update_meeting",
		"cancel_meeting",
		"reschedule_meeting",
		"get_free_busy",
	}
	for _, verb := range verbs {
		if !strings.Contains(desc, verb) {
			t.Errorf("calendar description missing verb %q\n  got: %s", verb, desc)
		}
	}
}

// TestTopLevelDescription_Account verifies that the account aggregate tool
// description lists every account verb (AC-4 / FR-3).
func TestTopLevelDescription_Account(t *testing.T) {
	s := buildDescriptionTestServer(t, config.Config{
		AuthRecordPath: "/tmp/test",
		CacheName:      "test",
		AuthMethod:     "browser",
	})
	desc := getToolDescription(t, s, "account")

	// All 7 account verbs (including help) must appear in the description.
	verbs := []string{
		"help",
		"add",
		"remove",
		"list",
		"login",
		"logout",
		"refresh",
	}
	for _, verb := range verbs {
		if !strings.Contains(desc, verb) {
			t.Errorf("account description missing verb %q\n  got: %s", verb, desc)
		}
	}
}

// TestTopLevelDescription_System verifies that the system aggregate tool
// description lists every system verb (AC-4 / FR-3).
func TestTopLevelDescription_System(t *testing.T) {
	s := buildDescriptionTestServer(t, config.Config{
		AuthRecordPath: "/tmp/test",
		CacheName:      "test",
		AuthMethod:     "browser",
	})
	desc := getToolDescription(t, s, "system")

	// help and status are always present.
	verbs := []string{"help", "status"}
	for _, verb := range verbs {
		if !strings.Contains(desc, verb) {
			t.Errorf("system description missing verb %q\n  got: %s", verb, desc)
		}
	}
}

// TestTopLevelDescription_System_CompleteAuth verifies that complete_auth
// appears in the system description when auth_code is active (FR-2).
func TestTopLevelDescription_System_CompleteAuth(t *testing.T) {
	s := buildDescriptionTestServer(t, config.Config{
		AuthRecordPath: "/tmp/test",
		CacheName:      "test",
		AuthMethod:     "auth_code",
	})
	desc := getToolDescription(t, s, "system")

	if !strings.Contains(desc, "complete_auth") {
		t.Errorf("system description missing verb %q when auth_code active\n  got: %s", "complete_auth", desc)
	}
}

// TestTopLevelDescription_Mail_AlwaysOn verifies that the always-on mail verbs
// appear in the mail aggregate tool description (AC-4 / FR-3).
func TestTopLevelDescription_Mail_AlwaysOn(t *testing.T) {
	s := buildDescriptionTestServer(t, config.Config{
		AuthRecordPath: "/tmp/test",
		CacheName:      "test",
		AuthMethod:     "browser",
		MailEnabled:    false,
	})
	desc := getToolDescription(t, s, "mail")

	alwaysOn := []string{
		"help",
		"list_folders",
		"list_messages",
		"get_message",
		"search_messages",
	}
	for _, verb := range alwaysOn {
		if !strings.Contains(desc, verb) {
			t.Errorf("mail description (MailEnabled=false) missing always-on verb %q\n  got: %s", verb, desc)
		}
	}
}

// TestTopLevelDescription_Mail_MailEnabled verifies that MailEnabled-gated
// verbs appear when MailEnabled=true (AC-4 / FR-2).
func TestTopLevelDescription_Mail_MailEnabled(t *testing.T) {
	s := buildDescriptionTestServer(t, config.Config{
		AuthRecordPath: "/tmp/test",
		CacheName:      "test",
		AuthMethod:     "browser",
		MailEnabled:    true,
	})
	desc := getToolDescription(t, s, "mail")

	gatedVerbs := []string{"get_conversation", "list_attachments", "get_attachment"}
	for _, verb := range gatedVerbs {
		if !strings.Contains(desc, verb) {
			t.Errorf("mail description (MailEnabled=true) missing gated verb %q\n  got: %s", verb, desc)
		}
	}
}

// TestTopLevelDescription_Mail_ManageEnabled verifies that MailManageEnabled-
// gated draft verbs appear when MailManageEnabled=true (AC-4 / FR-2).
func TestTopLevelDescription_Mail_ManageEnabled(t *testing.T) {
	s := buildDescriptionTestServer(t, config.Config{
		AuthRecordPath:    "/tmp/test",
		CacheName:         "test",
		AuthMethod:        "browser",
		MailEnabled:       true,
		MailManageEnabled: true,
	})
	desc := getToolDescription(t, s, "mail")

	draftVerbs := []string{
		"create_draft",
		"create_reply_draft",
		"create_forward_draft",
		"update_draft",
		"delete_draft",
	}
	for _, verb := range draftVerbs {
		if !strings.Contains(desc, verb) {
			t.Errorf("mail description (MailManageEnabled=true) missing draft verb %q\n  got: %s", verb, desc)
		}
	}
}

// TestTopLevelDescription_HelpVerbPresent verifies that every domain tool
// description includes "help" as the first or prominent operation, guiding
// LLM clients to call help for detailed documentation (AC-4).
func TestTopLevelDescription_HelpVerbPresent(t *testing.T) {
	s := buildDescriptionTestServer(t, config.Config{
		AuthRecordPath:    "/tmp/test",
		CacheName:         "test",
		AuthMethod:        "browser",
		MailEnabled:       true,
		MailManageEnabled: true,
	})

	for _, domain := range []string{"calendar", "mail", "account", "system"} {
		desc := getToolDescription(t, s, domain)
		if !strings.Contains(desc, "help") {
			t.Errorf("domain %q description missing 'help' verb\n  got: %s", domain, desc)
		}
	}
}

// TestTopLevelDescription_DescriptionNonEmpty verifies that every domain tool
// has a non-empty description string after registration.
func TestTopLevelDescription_DescriptionNonEmpty(t *testing.T) {
	s := buildDescriptionTestServer(t, config.Config{
		AuthRecordPath:    "/tmp/test",
		CacheName:         "test",
		AuthMethod:        "browser",
		MailEnabled:       true,
		MailManageEnabled: true,
	})

	for _, domain := range []string{"calendar", "mail", "account", "system"} {
		desc := getToolDescription(t, s, domain)
		if desc == "" {
			t.Errorf("domain %q has empty description", domain)
		}
	}
}

// TestHelpVerb_ReturnsDocForEveryVerb verifies that calling operation="help"
// returns documentation containing every registered verb name (AC-2).
func TestHelpVerb_ReturnsDocForEveryVerb(t *testing.T) {
	s := buildDescriptionTestServer(t, config.Config{
		AuthRecordPath: "/tmp/test",
		CacheName:      "test",
		AuthMethod:     "browser",
	})

	calVerbs := []string{
		"list_calendars", "list_events", "get_event", "search_events",
		"create_event", "update_event", "delete_event", "respond_event",
		"reschedule_event", "create_meeting", "update_meeting",
		"cancel_meeting", "reschedule_meeting", "get_free_busy",
	}

	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"calendar","arguments":{"operation":"help"}}}`
	resp := s.HandleMessage(context.Background(), json.RawMessage(msg))

	rpcResp, ok := resp.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", resp)
	}
	result, ok := rpcResp.Result.(*mcp.CallToolResult)
	if !ok {
		t.Fatalf("expected *CallToolResult, got %T", rpcResp.Result)
	}
	if result.IsError {
		t.Fatalf("help returned error: %v", result.Content)
	}
	if len(result.Content) == 0 {
		t.Fatal("help returned empty content")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	for _, verb := range calVerbs {
		if !strings.Contains(tc.Text, verb) {
			t.Errorf("calendar help output missing verb %q", verb)
		}
	}
}
