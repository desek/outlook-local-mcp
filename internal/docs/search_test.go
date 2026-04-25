package docs_test

import (
	"strings"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/docs"
)

// TestSearch_RanksExactMatchesFirst verifies that a substring match ranks above
// a token match. The query "InefficientFilter" should appear verbatim in
// troubleshooting.md, so it must be the first result.
func TestSearch_RanksExactMatchesFirst(t *testing.T) {
	t.Parallel()

	results, err := docs.Search("InefficientFilter")
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Search('InefficientFilter') returned no results")
	}
	if results[0].Slug != "troubleshooting" {
		t.Fatalf("expected first result slug 'troubleshooting', got %q", results[0].Slug)
	}
	if results[0].Score < 2 {
		t.Fatalf("expected exact-match score >=2, got %d", results[0].Score)
	}
}

// TestSearch_ReturnsSnippetWithLineNumbers verifies that each result includes a
// snippet string and valid 1-based line numbers with ±2 lines of context.
func TestSearch_ReturnsSnippetWithLineNumbers(t *testing.T) {
	t.Parallel()

	results, err := docs.Search("Keychain")
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Search('Keychain') returned no results")
	}

	r := results[0]
	if r.Snippet == "" {
		t.Fatal("result Snippet is empty")
	}
	if r.StartLine < 1 {
		t.Fatalf("StartLine must be >=1, got %d", r.StartLine)
	}
	if r.EndLine < r.StartLine {
		t.Fatalf("EndLine (%d) must be >= StartLine (%d)", r.EndLine, r.StartLine)
	}
	// Snippet must contain the query term (case-insensitive).
	if !strings.Contains(strings.ToLower(r.Snippet), "keychain") {
		t.Fatalf("snippet does not contain query term 'Keychain': %q", r.Snippet)
	}
}

// TestSearch_EmptyQueryReturnsNil verifies that an empty or whitespace-only
// query returns nil results without an error.
func TestSearch_EmptyQueryReturnsNil(t *testing.T) {
	t.Parallel()

	results, err := docs.Search("   ")
	if err != nil {
		t.Fatalf("Search('   ') unexpected error: %v", err)
	}
	if results != nil {
		t.Fatalf("expected nil results for empty query, got %v", results)
	}
}

// TestSearchDocs_AutoDefaultAnchor verifies that searching for "auto-default"
// returns a result from the troubleshooting document whose snippet contains the
// "Auto-default account" heading text, satisfying CR-0064 AC-6 and
// docs/troubleshooting.md anchor auto-default-account.
func TestSearchDocs_AutoDefaultAnchor(t *testing.T) {
	t.Parallel()

	results, err := docs.Search("auto-default")
	if err != nil {
		t.Fatalf("Search('auto-default') error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Search('auto-default') returned no results")
	}

	var found bool
	for _, r := range results {
		if r.Slug == "troubleshooting" && strings.Contains(r.Snippet, "Auto-default account") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a result from slug 'troubleshooting' with snippet containing 'Auto-default account', got: %+v", results)
	}
}
