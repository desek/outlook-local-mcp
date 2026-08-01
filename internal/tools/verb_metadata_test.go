// Package tools_test validates per-verb metadata requirements introduced by
// CR-0065: every verb MUST have a non-empty Description and a Summary that is
// non-empty and ≤80 characters. SeeDocs anchors, when present, MUST resolve to
// a slug in docs.Bundle and any anchor MUST match an H2 heading in that file.
package tools_test

import (
	"context"
	"encoding/json"
	"regexp"
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

// TestEveryVerbHasClassification asserts that every verb in every domain
// registry declares all four MCP annotation hints (readOnly, destructive,
// idempotent, openWorld), so a new verb cannot be registered without a
// classification (CR-0068 FR-13, AC-9). This is the guard that makes the
// conservative aggregate fold trustworthy: an undeclared hint would silently
// fold as its cautious value rather than the verb's true one.
//
// It builds the full verb surface (mail read and write verbs plus the auth_code
// complete_auth verb) via server.BuildDomainVerbSets and materialises each
// verb's Annotations the same way the aggregate fold does — by applying the
// opaque mcp.ToolOption closures to a zero-value mcp.Tool and reading the
// resulting hint pointers. A nil pointer means the hint was never declared.
func TestEveryVerbHasClassification(t *testing.T) {
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

	// auth_code plus both mail flags registers every verb the server can host.
	cfg := config.Config{
		AuthRecordPath:    "/tmp/test",
		CacheName:         "test",
		AuthMethod:        "auth_code",
		MailEnabled:       true,
		MailManageEnabled: true,
	}
	verbSets := server.BuildDomainVerbSets(cfg, graph.RetryConfig{}, 30*time.Second, m, tracer, identityMW, r)

	for _, domain := range []string{"calendar", "mail", "account", "system"} {
		verbs, ok := verbSets[domain]
		if !ok {
			t.Errorf("domain %q missing from verb sets", domain)
			continue
		}
		for _, v := range verbs {
			for _, hint := range missingClassificationHints(v.Annotations) {
				t.Errorf("domain %q verb %q: missing %s classification; every verb MUST declare all four hints (CR-0068 FR-13)", domain, v.Name, hint)
			}
		}
	}
}

// missingClassificationHints materialises the verb's annotation options onto a
// zero-value mcp.Tool and returns the names of any of the four required hints
// that were left undeclared (nil pointer). An empty result means the verb is
// fully classified.
func missingClassificationHints(opts []mcp.ToolOption) []string {
	var mt mcp.Tool
	for _, opt := range opts {
		opt(&mt)
	}
	a := mt.Annotations

	var missing []string
	if a.ReadOnlyHint == nil {
		missing = append(missing, "readOnlyHint")
	}
	if a.DestructiveHint == nil {
		missing = append(missing, "destructiveHint")
	}
	if a.IdempotentHint == nil {
		missing = append(missing, "idempotentHint")
	}
	if a.OpenWorldHint == nil {
		missing = append(missing, "openWorldHint")
	}
	return missing
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
			// Register exactly the single anchor the production get_docs parser
			// resolves this heading under. When the heading carries an explicit
			// "{#custom-id}" tag that is the id verbatim; otherwise it is the
			// text-derived form. Registering only that one anchor keeps this index
			// consistent with FR-3: the text-derived form of an explicit-anchor
			// heading is not a reachable anchor and must not be accepted here
			// (CR-0074).
			anchors[headingToAnchor(text)] = true
		}
		index[slug] = anchors
	}
	return index
}

// headingAnchorTagRe matches a trailing "{#custom-id}" anchor tag on a heading
// and captures the identifier. It mirrors the production regexp in
// internal/tools/get_docs.go; this file is package tools_test and cannot call
// the unexported production helper, so the derivation is kept behaviourally
// identical rather than literally shared (CR-0074).
var headingAnchorTagRe = regexp.MustCompile(`\s*\{#([^}]+)\}\s*$`)

// headingToAnchor converts a heading string to the anchor the production
// get_docs parser resolves it under. When the heading ends with an explicit
// "{#custom-id}" tag the identifier is used verbatim and the heading text is
// ignored (FR-1, FR-3); otherwise the anchor is derived from the text:
// lowercase, spaces to hyphens, other punctuation stripped (FR-2). Keeping this
// identical to production prevents the heading index from accepting anchors that
// get_docs cannot reach (CR-0074).
func headingToAnchor(heading string) string {
	heading = strings.TrimSpace(heading)
	if m := headingAnchorTagRe.FindStringSubmatch(heading); m != nil {
		return strings.ToLower(strings.TrimSpace(m[1]))
	}
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
