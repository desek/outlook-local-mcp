// Package server contains the registry-derived read-only mode assertion.
//
// Every write verb so far has carried its own read-only test, and every domain
// that registers only reads asserts the declaration. Neither shape catches a
// write verb wired through the read chain by mistake: the per-verb test is not
// written for a verb nobody remembered, and the declaration test grades the
// hint, not the chain. This file derives the write set from the live registry
// under the maximal configuration and drives every member through the
// registered server with read-only mode on, so the binding between
// readOnlyHint false and ReadOnlyGuard is observed rather than assumed.
//
// @agents-index: Registry-derived test that every verb declaring readOnlyHint
// false is refused by the registered server in read-only mode.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/audit"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/observability"
	"github.com/desek/outlook-local-mcp/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// verbDeclaresReadOnly materialises the verb's annotation options and reports
// whether readOnlyHint is declared true. An undeclared hint reads as false, the
// same conservative reading the aggregate fold applies.
func verbDeclaresReadOnly(verb tools.Verb) bool {
	var t mcp.Tool
	for _, opt := range verb.Annotations {
		opt(&t)
	}
	return t.Annotations.ReadOnlyHint != nil && *t.Annotations.ReadOnlyHint
}

// readOnlyExemptDomains names the domains whose write-classified verbs are not
// governed by read-only mode. Account verbs change the local account registry
// and system.complete_auth completes a local sign-in; neither touches mailbox
// state, and the concepts document scopes read-only mode to the calendar and
// mail write verbs. Every other domain, present or future, is graded.
var readOnlyExemptDomains = map[string]bool{"account": true, "system": true}

// TestReadOnlyRefusesEveryWriteClassifiedVerb derives the set of verbs that do
// not declare readOnlyHint true from the maximal registry and asserts the
// registered server refuses each of them in read-only mode, naming the
// domain.verb identity in the refusal.
//
// The guard sits before argument validation in the write chain, so the call
// carries only the operation: a verb that reaches validation instead of the
// guard fails for the wrong reason and is reported as unguarded.
func TestReadOnlyRefusesEveryWriteClassifiedVerb(t *testing.T) {
	m, err := observability.InitMetrics(noop.NewMeterProvider().Meter("test"))
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}
	tracer := tracenoop.NewTracerProvider().Tracer("test")
	audit.InitAuditLog(false, "")
	cfg := maximalSurfaceConfig()

	sets := BuildDomainVerbSets(cfg, graph.RetryConfig{}, 30*time.Second, m, tracer, identityMW, testRegistry())

	s := mcpserver.NewMCPServer("test-server", "0.0.1",
		mcpserver.WithToolCapabilities(false),
		mcpserver.WithRecovery(),
	)
	RegisterTools(s, graph.RetryConfig{}, 30*time.Second, m, tracer, true, identityMW, testRegistry(), cfg, nil)

	var writes int
	for domain, verbs := range sets {
		if readOnlyExemptDomains[domain] {
			continue
		}
		for _, verb := range verbs {
			if verbDeclaresReadOnly(verb) {
				continue
			}
			writes++
			identity := domain + "." + verb.Name
			t.Run(identity, func(t *testing.T) {
				msg := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":%q,"arguments":{"operation":%q}}}`, domain, verb.Name)
				resp := s.HandleMessage(context.Background(), json.RawMessage(msg))
				rpcResp, ok := resp.(mcp.JSONRPCResponse)
				if !ok {
					t.Fatalf("%s: expected JSONRPCResponse, got %T", identity, resp)
				}
				result, ok := rpcResp.Result.(*mcp.CallToolResult)
				if !ok {
					t.Fatalf("%s: expected *CallToolResult, got %T", identity, rpcResp.Result)
				}
				if !result.IsError || len(result.Content) == 0 {
					t.Fatalf("%s: declares readOnlyHint false but was not refused in read-only mode", identity)
				}
				tc, ok := result.Content[0].(mcp.TextContent)
				if !ok {
					t.Fatalf("%s: expected TextContent, got %T", identity, result.Content[0])
				}
				if !strings.Contains(tc.Text, "read-only mode") {
					t.Errorf("%s: refusal %q is not the read-only guard; the verb reached validation, so it is wired through the read chain", identity, tc.Text)
				}
				if !strings.Contains(tc.Text, identity) {
					t.Errorf("%s: refusal %q does not carry the domain.verb identity", identity, tc.Text)
				}
			})
		}
	}

	// The maximal registry holds the calendar and mail write sets. A count of
	// zero means the derivation selected nothing, which would let the test
	// pass without grading a single verb.
	if writes == 0 {
		t.Fatal("derived write set is empty; the hint materialisation selected no verb")
	}
}
