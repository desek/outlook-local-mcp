// Package docs_test provides drift-prevention tests for the embedded
// documentation bundle (CR-0065). These tests ensure that the repository-root
// README links to all embedded slugs, that QUICKSTART.md remains a pointer
// file, that no embedded heading enumerates a verb name, and that the bundle
// contains exactly the expected set of files.
package docs_test

import (
	"io/fs"
	"os"
	"strings"
	"testing"

	docspkg "github.com/desek/outlook-local-mcp/docs"
)

// embeddedSlugs is the canonical set of embedded documentation slugs.
var embeddedSlugs = []string{"readme", "quickstart", "concepts", "troubleshooting"}

// TestEmbeddedFilesArePresent asserts that docs.Bundle contains exactly the
// four canonical slug files and no others (CR-0065 AC-1, FR-2).
func TestEmbeddedFilesArePresent(t *testing.T) {
	found := make(map[string]bool)
	err := fs.WalkDir(docspkg.Bundle, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		found[path] = true
		return nil
	})
	if err != nil {
		t.Fatalf("walk docs.Bundle: %v", err)
	}

	expected := make(map[string]bool, len(embeddedSlugs))
	for _, s := range embeddedSlugs {
		expected[s+".md"] = true
	}

	for path := range found {
		if !expected[path] {
			t.Errorf("unexpected file in docs.Bundle: %q", path)
		}
	}
	for path := range expected {
		if !found[path] {
			t.Errorf("expected file missing from docs.Bundle: %q", path)
		}
	}
}

// TestRootReadmeLinksIntoDocs asserts that the repository-root README.md
// contains a hyperlink to each of the four embedded slug files (CR-0065 AC-2,
// FR-5).
func TestRootReadmeLinksIntoDocs(t *testing.T) {
	data, err := os.ReadFile("../README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	content := string(data)

	for _, slug := range embeddedSlugs {
		link := "docs/" + slug + ".md"
		if !strings.Contains(content, link) {
			t.Errorf("README.md does not contain a link to %q", link)
		}
	}
}

// TestRootQuickstartIsPointerOnly asserts that the repository-root QUICKSTART.md
// is a pointer file: at most 5 non-blank lines, and contains a hyperlink to
// docs/quickstart.md (CR-0065 AC-2, FR-6).
func TestRootQuickstartIsPointerOnly(t *testing.T) {
	data, err := os.ReadFile("../QUICKSTART.md")
	if err != nil {
		t.Fatalf("read QUICKSTART.md: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "docs/quickstart.md") {
		t.Error("QUICKSTART.md does not link to docs/quickstart.md")
	}

	var nonBlank int
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) != "" {
			nonBlank++
		}
	}
	const maxLines = 5
	if nonBlank > maxLines {
		t.Errorf("QUICKSTART.md has %d non-blank lines, want ≤%d (pointer file only)", nonBlank, maxLines)
	}
}

// knownVerbNames is the combined set of verb names across all four domain
// registries. This list is maintained manually and must be updated when verbs
// are added or removed. The TestNoVerbNamesInEmbeddedHeadings test uses this
// to ensure embedded markdown headings never enumerate verb names.
var knownVerbNames = []string{
	// calendar
	"help", "list_calendars", "list_events", "get_event", "search_events",
	"create_event", "update_event", "delete_event", "respond_event",
	"reschedule_event", "create_meeting", "update_meeting", "cancel_meeting",
	"reschedule_meeting", "get_free_busy",
	// mail
	"list_folders", "list_messages", "get_message", "search_messages",
	"get_conversation", "list_attachments", "get_attachment",
	"create_draft", "create_reply_draft", "create_forward_draft",
	"update_draft", "delete_draft",
	// account
	"add", "remove", "list", "login", "logout", "refresh",
	// system
	"status", "list_docs", "search_docs", "get_docs", "complete_auth",
}

// TestNoVerbNamesInEmbeddedHeadings asserts that no embedded markdown heading
// matches a known verb name. Per CR-0065, the registry is the per-verb
// reference; embedded markdown contains only narrative content (AC-7).
func TestNoVerbNamesInEmbeddedHeadings(t *testing.T) {
	verbSet := make(map[string]bool, len(knownVerbNames))
	for _, v := range knownVerbNames {
		verbSet[v] = true
	}

	for _, slug := range embeddedSlugs {
		data, err := docspkg.Bundle.ReadFile(slug + ".md")
		if err != nil {
			t.Fatalf("read embedded %s.md: %v", slug, err)
		}
		for lineNum, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(line, "## ") && !strings.HasPrefix(line, "# ") {
				continue
			}
			heading := strings.TrimLeft(line, "# ")
			// Headings are allowed to mention verbs only inside backtick code spans.
			// Strip backtick content before checking.
			stripped := removeBacktickSpans(heading)
			for verb := range verbSet {
				if strings.EqualFold(stripped, verb) {
					t.Errorf("%s.md line %d: heading %q matches verb name %q (registry is the per-verb reference)", slug, lineNum+1, heading, verb)
				}
			}
		}
	}
}

// removeBacktickSpans removes content between backticks from s.
func removeBacktickSpans(s string) string {
	var b strings.Builder
	inCode := false
	for _, c := range s {
		if c == '`' {
			inCode = !inCode
			continue
		}
		if !inCode {
			b.WriteRune(c)
		}
	}
	return b.String()
}
