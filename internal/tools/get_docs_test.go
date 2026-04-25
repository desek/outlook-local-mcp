// Package tools — unit tests for the system.get_docs verb handler (CR-0061 Phase 2).
package tools

import (
	"strings"
	"testing"
)

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
		{"Token Refresh", "token-refresh"},
		{"Keychain Locked / Unavailable", "keychain-locked--unavailable"},
		{"Graph 429 Throttling", "graph-429-throttling"},
		{"Authentication Failures", "authentication-failures"},
	}
	for _, c := range cases {
		got := headingToAnchor(c.heading)
		if got != c.want {
			t.Errorf("headingToAnchor(%q) = %q, want %q", c.heading, got, c.want)
		}
	}
}
