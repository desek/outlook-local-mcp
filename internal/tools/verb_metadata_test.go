// Package tools_test validates per-verb metadata requirements introduced by
// CR-0065: every verb MUST have a non-empty Description and a Summary that is
// non-empty and ≤80 characters. SeeDocs anchors, when present, MUST resolve to
// a slug in docs.Bundle and any anchor MUST match an H2 heading in that file.
package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	docspkg "github.com/desek/outlook-local-mcp/docs"
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

// buildMetadataTestServer builds a server with all four domain tools registered
// and all mail features enabled so that every verb is present.
func buildMetadataTestServer(t *testing.T) *mcpserver.MCPServer {
	t.Helper()

	s := mcpserver.NewMCPServer("test-metadata", "0.0.0",
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

	cfg := config.Config{
		AuthRecordPath:    "/tmp/test",
		CacheName:         "test",
		AuthMethod:        "browser",
		MailEnabled:       true,
		MailManageEnabled: true,
	}
	server.RegisterTools(s, graph.RetryConfig{}, 30*time.Second, m, tracer, false, identityMW, r, cfg, nil)
	return s
}

// verbsFromHelp calls the help verb for the given domain and parses the raw
// JSON output into a slice of verbRaw-like maps.
func verbsFromHelp(t *testing.T, s *mcpserver.MCPServer, domain string) []map[string]json.RawMessage {
	t.Helper()

	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + domain + `","arguments":{"operation":"help","output":"raw"}}}`
	resp := s.HandleMessage(context.Background(), json.RawMessage(msg))

	rpcResp, ok := resp.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("domain %q: expected JSONRPCResponse, got %T", domain, resp)
	}
	result, ok := rpcResp.Result.(*mcp.CallToolResult)
	if !ok {
		t.Fatalf("domain %q: expected *CallToolResult, got %T", domain, rpcResp.Result)
	}
	if result.IsError {
		t.Fatalf("domain %q help returned error: %v", domain, result.Content)
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("domain %q: expected TextContent, got %T", domain, result.Content[0])
	}

	var payload struct {
		Operations []map[string]json.RawMessage `json:"operations"`
	}
	if err := json.Unmarshal([]byte(tc.Text), &payload); err != nil {
		t.Fatalf("domain %q: parse help raw JSON: %v\n%s", domain, err, tc.Text)
	}
	return payload.Operations
}

// TestEveryVerbHasDescription asserts that every verb in every domain registry
// has a non-empty Description field (CR-0065 FR-9, AC-4).
func TestEveryVerbHasDescription(t *testing.T) {
	s := buildMetadataTestServer(t)
	domains := []string{"calendar", "mail", "account", "system"}

	for _, domain := range domains {
		verbs := verbsFromHelp(t, s, domain)
		for _, v := range verbs {
			nameRaw := v["name"]
			descRaw, hasDesc := v["description"]

			var name string
			_ = json.Unmarshal(nameRaw, &name)

			if !hasDesc {
				t.Errorf("domain %q verb %q: missing description field in raw help output", domain, name)
				continue
			}
			var desc string
			_ = json.Unmarshal(descRaw, &desc)
			if strings.TrimSpace(desc) == "" {
				t.Errorf("domain %q verb %q: description is empty", domain, name)
			}
		}
	}
}

// TestEveryVerbHasSummary asserts that every verb has a non-empty Summary of
// at most 80 characters (CR-0065 FR-9, original CR-0060 contract).
func TestEveryVerbHasSummary(t *testing.T) {
	s := buildMetadataTestServer(t)
	domains := []string{"calendar", "mail", "account", "system"}

	for _, domain := range domains {
		verbs := verbsFromHelp(t, s, domain)
		for _, v := range verbs {
			var name, summary string
			_ = json.Unmarshal(v["name"], &name)
			_ = json.Unmarshal(v["summary"], &summary)

			if strings.TrimSpace(summary) == "" {
				t.Errorf("domain %q verb %q: summary is empty", domain, name)
				continue
			}
			if utf8.RuneCountInString(summary) > 80 {
				t.Errorf("domain %q verb %q: summary is %d chars, want ≤80: %q",
					domain, name, utf8.RuneCountInString(summary), summary)
			}
		}
	}
}

// TestSeeDocsAnchorsResolve verifies that every SeeDocs entry for every verb
// resolves to a slug in docs.Bundle and that any anchor matches an H2 heading
// in that file (CR-0065 FR-11, AC-6).
func TestSeeDocsAnchorsResolve(t *testing.T) {
	s := buildMetadataTestServer(t)
	domains := []string{"calendar", "mail", "account", "system"}

	// Build heading index: slug -> set of anchor strings derived from "## Heading".
	headingIndex := buildHeadingIndex(t)

	for _, domain := range domains {
		verbs := verbsFromHelp(t, s, domain)
		for _, v := range verbs {
			var name string
			_ = json.Unmarshal(v["name"], &name)

			seeDocsRaw, ok := v["see_docs"]
			if !ok {
				continue // empty SeeDocs is allowed
			}
			var seeDocs []string
			if err := json.Unmarshal(seeDocsRaw, &seeDocs); err != nil {
				t.Errorf("domain %q verb %q: parse see_docs: %v", domain, name, err)
				continue
			}
			for _, ref := range seeDocs {
				slug, anchor, hasAnchor := strings.Cut(ref, "#")
				// Verify slug exists in docs.Bundle.
				if _, err := docspkg.Bundle.Open(slug + ".md"); err != nil {
					t.Errorf("domain %q verb %q: SeeDocs %q: slug %q not found in docs.Bundle", domain, name, ref, slug)
					continue
				}
				if !hasAnchor {
					continue
				}
				// Verify anchor matches a heading.
				anchors, ok := headingIndex[slug]
				if !ok {
					t.Errorf("domain %q verb %q: SeeDocs %q: no headings found for slug %q", domain, name, ref, slug)
					continue
				}
				if !anchors[anchor] {
					t.Errorf("domain %q verb %q: SeeDocs %q: anchor %q not found in %s.md headings", domain, name, ref, anchor, slug)
				}
			}
		}
	}
}

// buildHeadingIndex reads each embedded markdown file and returns a map from
// slug to the set of anchor strings derived from "## Heading" lines.
// The derivation follows GitHub-compatible anchor rules: lowercase, spaces
// replaced by hyphens, punctuation stripped.
func buildHeadingIndex(t *testing.T) map[string]map[string]bool {
	t.Helper()

	slugs := []string{"readme", "quickstart", "concepts", "troubleshooting"}
	index := make(map[string]map[string]bool, len(slugs))

	for _, slug := range slugs {
		data, err := docspkg.Bundle.ReadFile(slug + ".md")
		if err != nil {
			t.Fatalf("buildHeadingIndex: read %s.md: %v", slug, err)
		}

		anchors := make(map[string]bool)
		for _, line := range strings.Split(string(data), "\n") {
			// Match lines that start with "## " (H2 headings only, per CR-0065 scope).
			text, ok := strings.CutPrefix(line, "## ")
			if !ok {
				continue
			}
			// Strip inline `{#anchor}` if present (handled but not recommended per CR-0065 risk #6).
			if i := strings.Index(text, " {#"); i != -1 {
				// Also register the explicit anchor id.
				explicit := strings.TrimSuffix(strings.TrimPrefix(text[i+3:], ""), "}")
				explicit = strings.TrimSuffix(explicit, "}")
				anchors[strings.TrimSpace(explicit)] = true
				text = strings.TrimSpace(text[:i])
			}
			anchors[headingToAnchor(text)] = true
		}
		index[slug] = anchors
	}
	return index
}

// headingToAnchor converts a heading string to its GitHub-compatible anchor.
// Lowercase, replace spaces with hyphens, strip most punctuation.
func headingToAnchor(heading string) string {
	// Strip inline code markers and common punctuation, keep alphanumerics, spaces, hyphens.
	var b strings.Builder
	for _, r := range strings.ToLower(heading) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
			// skip other characters (punctuation, brackets, etc.)
		}
	}
	return b.String()
}
