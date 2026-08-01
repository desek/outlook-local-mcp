// Package tools — unit tests for the system.get_docs verb handler (CR-0061 Phase 2).
package tools

import (
	"regexp"
	"strings"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/docs"
)

// explicitAnchorRe matches a trailing "{#custom-id}" tag on a heading line and
// captures the identifier. This mirrors the GitHub-flavoured Markdown convention
// the embedded documents use to pin a section's anchor independently of its
// display text.
var explicitAnchorRe = regexp.MustCompile(`\s*\{#([^}]+)\}\s*$`)

// intendedAnchor computes the anchor an author expects a heading to resolve
// under, independently of the (possibly buggy) production headingToAnchor.
//
// When the heading carries a trailing "{#custom-id}" tag the identifier is used
// verbatim. Otherwise the anchor is derived from the heading text the same way a
// correct GitHub-flavoured Markdown renderer would: lower-cased, spaces to
// hyphens, all other non-alphanumeric characters dropped.
//
// This helper is deliberately independent of the code under test so the corpus
// reachability test can fail when get_docs cannot reach a heading the documents
// legitimately declare. Computing the anchor via the production helper would make
// the test agree with the implementation rather than with the corpus.
func intendedAnchor(heading string) string {
	heading = strings.TrimSpace(heading)
	if m := explicitAnchorRe.FindStringSubmatch(heading); m != nil {
		return strings.ToLower(strings.TrimSpace(m[1]))
	}
	heading = strings.ToLower(heading)
	var b strings.Builder
	for _, r := range heading {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	return b.String()
}

// TestGetDocs_EveryHeadingReachable walks every H2 heading in every embedded
// document, computes the anchor the author intends it to resolve under, and
// asserts that get_docs returns non-empty content for that anchor.
//
// This is the corpus reachability check the defect survived because the prior
// tests drew their cases from the implementation rather than from the documents.
// It fails for any heading the production parser cannot reach — including the
// five headings carrying explicit "{#...}" anchors — and names the document and
// heading so the failure is actionable. It requires no editing to catch a future
// unreachable heading.
func TestGetDocs_EveryHeadingReachable(t *testing.T) {
	h := HandleGetDocs()

	for _, entry := range docs.MustCatalog() {
		slug := entry.Slug
		data, err := docs.ReadSlug(slug)
		if err != nil {
			t.Fatalf("ReadSlug(%q): %v", slug, err)
		}

		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(line, "## ") {
				continue
			}
			heading := strings.TrimPrefix(line, "## ")
			anchor := intendedAnchor(heading)

			req := buildRequest("system", map[string]any{
				"operation": "get_docs",
				"slug":      slug,
				"section":   anchor,
			})

			result, hErr := h(t.Context(), req)
			if hErr != nil {
				t.Fatalf("get_docs(slug=%q, section=%q) unexpected Go error: %v", slug, anchor, hErr)
			}
			if result == nil {
				t.Fatalf("get_docs(slug=%q, section=%q) returned nil result", slug, anchor)
			}
			if result.IsError {
				t.Errorf("unreachable heading: document %q heading %q (anchor %q) is not reachable via get_docs", slug, strings.TrimSpace(heading), anchor)
				continue
			}
			if strings.TrimSpace(dispatchResultText(t, result)) == "" {
				t.Errorf("empty section: document %q heading %q (anchor %q) resolved but returned empty content", slug, strings.TrimSpace(heading), anchor)
			}
		}
	}
}

// TestSystemGetDocs_Section verifies that get_docs with slug=troubleshooting and
// a valid section anchor returns only the body of that section.
func TestSystemGetDocs_Section(t *testing.T) {
	h := HandleGetDocs()
	// The troubleshooting doc has "## Authentication Failures" which anchors as
	// "authentication-failures".
	req := buildRequest("system", map[string]any{
		"operation": "get_docs",
		"slug":      "troubleshooting",
		"section":   "authentication-failures",
	})

	result, err := h(t.Context(), req)
	if err != nil {
		t.Fatalf("HandleGetDocs(section) unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("HandleGetDocs() returned nil result")
	}
	if result.IsError {
		t.Fatalf("HandleGetDocs(section) returned IsError=true: %v", result.Content)
	}

	text := dispatchResultText(t, result)

	// The section heading must be present.
	if !strings.Contains(strings.ToLower(text), "authentication") {
		t.Errorf("get_docs section output should contain 'Authentication'; got:\n%s", text)
	}
}

// TestSystemGetDocs_UnknownSlug verifies that get_docs returns a tool error for
// an unknown slug, not a Go error.
func TestSystemGetDocs_UnknownSlug(t *testing.T) {
	h := HandleGetDocs()
	req := buildRequest("system", map[string]any{
		"operation": "get_docs",
		"slug":      "does-not-exist",
	})

	result, err := h(t.Context(), req)
	if err != nil {
		t.Fatalf("HandleGetDocs(unknown slug) unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Error("HandleGetDocs(unknown slug) should return IsError=true")
	}
}

// TestSystemGetDocs_UnknownSection verifies that get_docs returns a tool error
// when the section anchor is not found.
func TestSystemGetDocs_UnknownSection(t *testing.T) {
	h := HandleGetDocs()
	req := buildRequest("system", map[string]any{
		"operation": "get_docs",
		"slug":      "troubleshooting",
		"section":   "this-section-does-not-exist",
	})

	result, err := h(t.Context(), req)
	if err != nil {
		t.Fatalf("HandleGetDocs(unknown section) unexpected Go error: %v", err)
	}
	if !result.IsError {
		t.Error("HandleGetDocs(unknown section) should return IsError=true")
	}
}

// TestSystemGetDocs_Raw verifies that output=raw returns unmodified markdown.
func TestSystemGetDocs_Raw(t *testing.T) {
	h := HandleGetDocs()
	req := buildRequest("system", map[string]any{
		"operation": "get_docs",
		"slug":      "readme",
		"output":    "raw",
	})

	result, err := h(t.Context(), req)
	if err != nil {
		t.Fatalf("HandleGetDocs(raw) unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("HandleGetDocs(raw) returned IsError=true: %v", result.Content)
	}

	text := dispatchResultText(t, result)
	// Raw output must be non-empty markdown.
	if len(text) < 100 {
		t.Errorf("get_docs raw output unexpectedly short (%d bytes)", len(text))
	}
}

// TestHeadingToAnchor verifies the anchor conversion helper.
func TestHeadingToAnchor(t *testing.T) {
	cases := []struct {
		heading string
		want    string
	}{
		// Text-derived cases (FR-2): no explicit anchor tag.
		{"Token Refresh", "token-refresh"},
		{"Keychain Locked / Unavailable", "keychain-locked--unavailable"},
		{"Graph 429 Throttling", "graph-429-throttling"},
		{"Authentication Failures", "authentication-failures"},
		// Explicit-anchor cases (FR-1): the trailing "{#custom-id}" tag wins and
		// the heading text is ignored.
		{"Container deployment {#container-deployment}", "container-deployment"},
		{"Auto-default account {#auto-default-account}", "auto-default-account"},
		// FR-8: the explicit anchor differs materially from the text-derived form.
		{"Container has no keychain access {#container-no-keychain}", "container-no-keychain"},
	}
	for _, c := range cases {
		got := headingToAnchor(c.heading)
		if got != c.want {
			t.Errorf("headingToAnchor(%q) = %q, want %q", c.heading, got, c.want)
		}
	}

	// FR-3: a heading carrying an explicit anchor MUST NOT also resolve under its
	// text-derived form. The explicit anchor is the only one that addresses the
	// section; accepting the derived form too would give one section two names and
	// silently override the author's explicit choice.
	const explicitHeading = "Container has no keychain access {#container-no-keychain}"
	const derivedForm = "container-has-no-keychain-access"
	if got := headingToAnchor(explicitHeading); got == derivedForm {
		t.Errorf("headingToAnchor(%q) = %q; the text-derived form must not resolve for an explicit-anchor heading", explicitHeading, got)
	}
}
