// Package tools_test contains description-quality tests for the four aggregate
// MCP domain tools (calendar, mail, account, system). These enforce the
// structural and completeness requirements CR-0068 adds to the top-level tool
// descriptions so that an MCP client can select a verb and construct its
// arguments from tools/list alone:
//
//   - every verb inventory entry appears on its own line (OBS-4, FR-10, AC-6);
//   - the mail description names its gating keys (OBS-3, FR-11, AC-8);
//   - every verb states which parameters it requires (OBS-2, FR-9, AC-7);
//   - no description exceeds 4000 characters (NFR-3, AC-6);
//   - every parameter carries a non-empty description (FR-12, AC-12).
package tools_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/audit"
	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/config"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/observability"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	server "github.com/desek/outlook-local-mcp/internal/server"
)

// fullSurfaceConfig enables every gated verb (both mail flags and auth_code) so
// the description checks exercise the largest possible verb inventory.
func fullSurfaceConfig() config.Config {
	return config.Config{
		AuthRecordPath:    "/tmp/test",
		CacheName:         "test",
		AuthMethod:        "auth_code",
		MailEnabled:       true,
		MailManageEnabled: true,
	}
}

// domainVerbNames returns the verb names for the given configuration keyed by
// domain, built via server.BuildDomainVerbSets with inert dependencies.
func domainVerbNames(t *testing.T, cfg config.Config) map[string][]string {
	t.Helper()

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

	verbSets := server.BuildDomainVerbSets(cfg, graph.RetryConfig{}, 30*time.Second, m, tracer, identityMW, r)
	out := make(map[string][]string, len(verbSets))
	for domain, verbs := range verbSets {
		names := make([]string, 0, len(verbs))
		for _, v := range verbs {
			names = append(names, v.Name)
		}
		out[domain] = names
	}
	return out
}

// verbLine returns the inventory line for the named verb in a tool description,
// identified by the "- `name`" prefix the description composer emits, or "" if
// no such line exists.
func verbLine(description, verb string) string {
	prefix := "- `" + verb + "`"
	for _, line := range strings.Split(description, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			return line
		}
	}
	return ""
}

// TestDescriptionsListVerbsOnSeparateLines verifies that every registered verb
// occupies its own line in its domain tool description (OBS-4, FR-10, AC-6).
func TestDescriptionsListVerbsOnSeparateLines(t *testing.T) {
	cfg := fullSurfaceConfig()
	s := buildTestServer(t, cfg)
	names := domainVerbNames(t, cfg)

	for _, domain := range []string{"calendar", "mail", "account", "system"} {
		tool := getRegisteredTool(t, s, domain)
		for _, verb := range names[domain] {
			if verbLine(tool.Description, verb) == "" {
				t.Errorf("domain %q: verb %q has no dedicated inventory line in description", domain, verb)
			}
		}
	}
}

// TestMailDescriptionMentionsGatedVerbs verifies that the mail description names
// both gating configuration keys even in the default gated configuration, so a
// reader learns the write verbs exist behind them (OBS-3, FR-11, AC-8).
func TestMailDescriptionMentionsGatedVerbs(t *testing.T) {
	// Default gated configuration: neither mail flag set.
	s := buildTestServer(t, config.Config{
		AuthRecordPath: "/tmp/test",
		CacheName:      "test",
		AuthMethod:     "browser",
	})
	tool := getRegisteredTool(t, s, "mail")

	for _, key := range []string{"MailEnabled", "MailManageEnabled"} {
		if !strings.Contains(tool.Description, key) {
			t.Errorf("mail description does not mention gating key %q; got:\n%s", key, tool.Description)
		}
	}
}

// TestEveryVerbStatesRequiredParameters verifies that every verb's inventory
// line states its required parameters, either naming them after "Requires:" or
// declaring "No required parameters." (OBS-2, FR-9, AC-7).
func TestEveryVerbStatesRequiredParameters(t *testing.T) {
	cfg := fullSurfaceConfig()
	s := buildTestServer(t, cfg)
	names := domainVerbNames(t, cfg)

	for _, domain := range []string{"calendar", "mail", "account", "system"} {
		tool := getRegisteredTool(t, s, domain)
		for _, verb := range names[domain] {
			line := verbLine(tool.Description, verb)
			if line == "" {
				t.Errorf("domain %q verb %q: no inventory line", domain, verb)
				continue
			}
			if !strings.Contains(line, "Requires:") && !strings.Contains(line, "No required parameters.") {
				t.Errorf("domain %q verb %q: line does not state required parameters: %q", domain, verb, line)
			}
		}
	}
}

// TestDescriptionLengthBounded verifies that no aggregate tool description
// exceeds 4000 characters (NFR-3, AC-6).
func TestDescriptionLengthBounded(t *testing.T) {
	const maxLen = 4000
	s := buildTestServer(t, fullSurfaceConfig())

	for _, domain := range []string{"calendar", "mail", "account", "system"} {
		tool := getRegisteredTool(t, s, domain)
		if got := len(tool.Description); got >= maxLen {
			t.Errorf("domain %q description is %d chars, want < %d", domain, got, maxLen)
		}
	}
}

// TestEveryParameterHasDescription verifies that every parameter of every
// aggregate tool carries a non-empty description field (FR-12, AC-12).
func TestEveryParameterHasDescription(t *testing.T) {
	s := buildTestServer(t, fullSurfaceConfig())

	for _, domain := range []string{"calendar", "mail", "account", "system"} {
		tool := getRegisteredTool(t, s, domain)

		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatalf("domain %q: marshal input schema: %v", domain, err)
		}
		var schema struct {
			Properties map[string]struct {
				Description string `json:"description"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatalf("domain %q: parse input schema: %v", domain, err)
		}
		for name, prop := range schema.Properties {
			if strings.TrimSpace(prop.Description) == "" {
				t.Errorf("domain %q parameter %q: empty description", domain, name)
			}
		}
	}
}
