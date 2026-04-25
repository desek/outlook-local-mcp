// Package tools — unit tests for the system.list_docs verb handler (CR-0061 Phase 2).
package tools

import (
	"strings"
	"testing"
)

// TestSystemListDocs_Text verifies that HandleListDocs returns a numbered
// plain-text list of documents when output is unset (text default).
func TestSystemListDocs_Text(t *testing.T) {
	h := HandleListDocs()
	req := buildRequest("system", map[string]any{"operation": "list_docs"})

	result, err := h(t.Context(), req)
	if err != nil {
		t.Fatalf("HandleListDocs() unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("HandleListDocs() returned nil result")
	}
	if result.IsError {
		t.Fatalf("HandleListDocs() returned IsError=true: %v", result.Content)
	}

	text := dispatchResultText(t, result)

	// Expect numbered list with all three slugs.
	for _, slug := range []string{"readme", "quickstart", "troubleshooting"} {
		if !strings.Contains(text, slug) {
			t.Errorf("list_docs text output missing slug %q; got:\n%s", slug, text)
		}
	}

	// Expect doc:// URIs.
	if !strings.Contains(text, "doc://outlook-local-mcp/") {
		t.Errorf("list_docs text output missing doc:// URIs; got:\n%s", text)
	}

	// Expect total count line.
	if !strings.Contains(text, "Total:") {
		t.Errorf("list_docs text output missing 'Total:' line; got:\n%s", text)
	}
}

// TestSystemListDocs_Raw verifies that output=raw returns a JSON array.
func TestSystemListDocs_Raw(t *testing.T) {
	h := HandleListDocs()
	req := buildRequest("system", map[string]any{"operation": "list_docs", "output": "raw"})

	result, err := h(t.Context(), req)
	if err != nil {
		t.Fatalf("HandleListDocs(raw) unexpected error: %v", err)
	}
	text := dispatchResultText(t, result)

	if !strings.HasPrefix(text, "[") || !strings.HasSuffix(strings.TrimSpace(text), "]") {
		t.Errorf("list_docs raw output should be a JSON array; got:\n%s", text)
	}
	if !strings.Contains(text, `"slug"`) {
		t.Errorf("list_docs raw output missing 'slug' field; got:\n%s", text)
	}
}
