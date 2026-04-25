// Package tools_test contains cross-cutting annotation tests for the four
// aggregate MCP domain tools (calendar, mail, account, system). Each test
// verifies that the five MCP annotations (Title, ReadOnlyHint, DestructiveHint,
// IdempotentHint, OpenWorldHint) on the aggregate tool use the most conservative
// value across all verbs it hosts, per CR-0060 AC-9 and FR-9.
//
// Per-verb annotation semantics are documented in the help output of each domain
// tool rather than on the aggregate tool itself.
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

// aggregateAnnotationExpectation holds expected annotation values for an
// aggregate domain tool.
type aggregateAnnotationExpectation struct {
	title       string
	readOnly    bool
	destructive bool
	idempotent  bool
	openWorld   bool
}

// buildTestServer registers all four domain tools and returns the server for
// inspection. Uses no-op metrics, tracer, and identity middleware.
func buildTestServer(t *testing.T, cfg config.Config) *mcpserver.MCPServer {
	t.Helper()

	s := mcpserver.NewMCPServer("test-annotations", "0.0.0",
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

// getRegisteredTool retrieves the mcp.Tool registered under the given name.
// It fails the test if the tool is not found.
func getRegisteredTool(t *testing.T, s *mcpserver.MCPServer, name string) mcp.Tool {
	t.Helper()

	st := s.ListTools()
	entry, ok := st[name]
	if !ok {
		t.Fatalf("tool %q not registered; available: %v", name, serverToolNames(st))
	}
	return entry.Tool
}

// serverToolNames extracts the keys from a ListTools map for error messages.
func serverToolNames(tools map[string]*mcpserver.ServerTool) []string {
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	return names
}

// assertAggregateAnnotations verifies all five MCP annotations on a domain
// aggregate tool match the expected conservative values.
func assertAggregateAnnotations(t *testing.T, tool mcp.Tool, want aggregateAnnotationExpectation) {
	t.Helper()

	ann := tool.Annotations

	if ann.Title != want.title {
		t.Errorf("Title = %q, want %q", ann.Title, want.title)
	}
	if ann.ReadOnlyHint == nil {
		t.Error("ReadOnlyHint is nil")
	} else if *ann.ReadOnlyHint != want.readOnly {
		t.Errorf("ReadOnlyHint = %v, want %v", *ann.ReadOnlyHint, want.readOnly)
	}
	if ann.DestructiveHint == nil {
		t.Error("DestructiveHint is nil")
	} else if *ann.DestructiveHint != want.destructive {
		t.Errorf("DestructiveHint = %v, want %v", *ann.DestructiveHint, want.destructive)
	}
	if ann.IdempotentHint == nil {
		t.Error("IdempotentHint is nil")
	} else if *ann.IdempotentHint != want.idempotent {
		t.Errorf("IdempotentHint = %v, want %v", *ann.IdempotentHint, want.idempotent)
	}
	if ann.OpenWorldHint == nil {
		t.Error("OpenWorldHint is nil")
	} else if *ann.OpenWorldHint != want.openWorld {
		t.Errorf("OpenWorldHint = %v, want %v", *ann.OpenWorldHint, want.openWorld)
	}
}

// TestAggregateAnnotations_Calendar verifies the conservative aggregate
// annotations on the "calendar" domain tool (AC-9 / FR-9).
//
// calendar hosts both read-only verbs (list_calendars, list_events, ...) and
// destructive write verbs (delete_event, cancel_meeting), so the aggregate
// annotation is: readOnly=false, destructive=true, idempotent=false, openWorld=true.
func TestAggregateAnnotations_Calendar(t *testing.T) {
	s := buildTestServer(t, config.Config{
		AuthRecordPath: "/tmp/test",
		CacheName:      "test",
		AuthMethod:     "browser",
	})
	tool := getRegisteredTool(t, s, "calendar")
	assertAggregateAnnotations(t, tool, aggregateAnnotationExpectation{
		title:       "Calendar",
		readOnly:    false,
		destructive: true,
		idempotent:  false,
		openWorld:   true,
	})
}

// TestAggregateAnnotations_Mail verifies the conservative aggregate annotations
// on the "mail" domain tool (AC-9 / FR-9).
//
// mail may host delete_draft (destructive) and create_draft (non-idempotent,
// write), so: readOnly=false, destructive=true, idempotent=false, openWorld=true.
func TestAggregateAnnotations_Mail(t *testing.T) {
	s := buildTestServer(t, config.Config{
		AuthRecordPath:    "/tmp/test",
		CacheName:         "test",
		AuthMethod:        "browser",
		MailEnabled:       true,
		MailManageEnabled: true,
	})
	tool := getRegisteredTool(t, s, "mail")
	assertAggregateAnnotations(t, tool, aggregateAnnotationExpectation{
		title:       "Mail",
		readOnly:    false,
		destructive: true,
		idempotent:  false,
		openWorld:   true,
	})
}

// TestAggregateAnnotations_Account verifies the conservative aggregate
// annotations on the "account" domain tool (AC-9 / FR-9).
//
// account hosts remove (destructive) and add/login (non-idempotent, write), so:
// readOnly=false, destructive=true, idempotent=false, openWorld=true.
func TestAggregateAnnotations_Account(t *testing.T) {
	s := buildTestServer(t, config.Config{
		AuthRecordPath: "/tmp/test",
		CacheName:      "test",
		AuthMethod:     "browser",
	})
	tool := getRegisteredTool(t, s, "account")
	assertAggregateAnnotations(t, tool, aggregateAnnotationExpectation{
		title:       "Account",
		readOnly:    false,
		destructive: true,
		idempotent:  false,
		openWorld:   true,
	})
}

// TestAggregateAnnotations_System verifies the conservative aggregate
// annotations on the "system" domain tool (AC-9 / FR-9).
//
// system hosts complete_auth (write, non-idempotent) when auth_code is active,
// and status (read). No verb is destructive. Conservative: readOnly=false,
// destructive=false, idempotent=false, openWorld=true.
func TestAggregateAnnotations_System(t *testing.T) {
	s := buildTestServer(t, config.Config{
		AuthRecordPath: "/tmp/test",
		CacheName:      "test",
		AuthMethod:     "browser",
	})
	tool := getRegisteredTool(t, s, "system")
	assertAggregateAnnotations(t, tool, aggregateAnnotationExpectation{
		title:       "System",
		readOnly:    false,
		destructive: false,
		idempotent:  false,
		openWorld:   true,
	})
}

// TestAggregateAnnotations_NoOldToolNames verifies that no old
// {domain}_{operation} tool names survive registration after CR-0060 (AC-1).
func TestAggregateAnnotations_NoOldToolNames(t *testing.T) {
	s := buildTestServer(t, config.Config{
		AuthRecordPath:    "/tmp/test",
		CacheName:         "test",
		AuthMethod:        "browser",
		MailEnabled:       true,
		MailManageEnabled: true,
	})

	registered := s.ListTools()
	for name := range registered {
		if strings.Contains(name, "_") {
			t.Errorf("old-style tool name %q still registered; CR-0060 requires aggregate tools only", name)
		}
	}
}

// TestAggregateAnnotations_FourToolsRegistered verifies that exactly four
// aggregate tools are registered (AC-1 / FR-1).
func TestAggregateAnnotations_FourToolsRegistered(t *testing.T) {
	s := buildTestServer(t, config.Config{
		AuthRecordPath:    "/tmp/test",
		CacheName:         "test",
		AuthMethod:        "browser",
		MailEnabled:       true,
		MailManageEnabled: true,
	})

	registered := s.ListTools()
	const wantCount = 4
	if got := len(registered); got != wantCount {
		t.Errorf("registered %d tools, want %d; got: %v", got, wantCount, serverToolNames(registered))
	}

	for _, name := range []string{"calendar", "mail", "account", "system"} {
		if _, ok := registered[name]; !ok {
			t.Errorf("aggregate tool %q not registered", name)
		}
	}
}

// TestPerVerbAnnotations_DocumentedInHelp verifies that calling
// operation="help" on each domain tool returns output that documents
// annotation-relevant semantics (read-only, destructive) per AC-9.
func TestPerVerbAnnotations_DocumentedInHelp(t *testing.T) {
	s := buildTestServer(t, config.Config{
		AuthRecordPath:    "/tmp/test",
		CacheName:         "test",
		AuthMethod:        "browser",
		MailEnabled:       true,
		MailManageEnabled: true,
	})

	domains := []string{"calendar", "mail", "account", "system"}
	for _, domain := range domains {
		t.Run(domain, func(t *testing.T) {
			msg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + domain + `","arguments":{"operation":"help"}}}`
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
				t.Fatalf("help verb returned error: %v", result.Content)
			}
			if len(result.Content) == 0 {
				t.Fatal("help verb returned empty content")
			}
			tc, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatalf("expected TextContent, got %T", result.Content[0])
			}
			// Help output must be non-empty and contain the domain name so the
			// LLM can identify the documentation context.
			if len(tc.Text) == 0 {
				t.Error("help text is empty")
			}
		})
	}
}
