// Package tools — unit tests for the system.search_docs verb handler (CR-0061 Phase 2).
package tools

import (
	"strings"
	"testing"
)

// TestSystemSearchDocs_NoResults verifies that a query matching nothing returns a
// structured zero-results response, not an error.
func TestSystemSearchDocs_NoResults(t *testing.T) {
	h := HandleSearchDocs()
	req := buildRequest("system", map[string]any{
		"operation": "search_docs",
		"query":     "zzzxyz-definitely-not-in-docs",
	})

	result, err := h(t.Context(), req)
	if err != nil {
		t.Fatalf("HandleSearchDocs(no results) unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("HandleSearchDocs() returned nil result")
	}
	if result.IsError {
		t.Errorf("HandleSearchDocs(no results) should NOT return IsError=true")
	}

	text := dispatchResultText(t, result)
	if !strings.Contains(strings.ToLower(text), "no results") {
		t.Errorf("zero-results response should say 'No results'; got:\n%s", text)
	}
}

// TestSystemSearchDocs_EmptyQuery verifies that an empty query returns a
// zero-results message, not an error.
func TestSystemSearchDocs_EmptyQuery(t *testing.T) {
	h := HandleSearchDocs()
	req := buildRequest("system", map[string]any{
		"operation": "search_docs",
		"query":     "",
	})

	result, err := h(t.Context(), req)
	if err != nil {
		t.Fatalf("HandleSearchDocs(empty) unexpected error: %v", err)
	}
	if result.IsError {
		t.Error("HandleSearchDocs(empty query) should not return IsError=true")
	}
}

// TestSystemSearchDocs_Hits verifies that a known term returns results with
// slug and line numbers.
func TestSystemSearchDocs_Hits(t *testing.T) {
	h := HandleSearchDocs()
	req := buildRequest("system", map[string]any{
		"operation": "search_docs",
		"query":     "authentication",
	})

	result, err := h(t.Context(), req)
	if err != nil {
		t.Fatalf("HandleSearchDocs(hits) unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("HandleSearchDocs(hits) returned IsError=true: %v", result.Content)
	}

	text := dispatchResultText(t, result)
	if !strings.Contains(text, "lines") {
		t.Errorf("search_docs hit output should contain 'lines' range; got:\n%s", text)
	}
}
