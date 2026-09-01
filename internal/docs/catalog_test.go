package docs_test

import (
	"strings"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/docs"
)

// TestCatalog_AllSlugsResolve verifies that every entry in the catalog maps to
// a non-empty file in the embedded bundle. A failure here indicates that the
// catalog metadata and the embed directive are out of sync.
func TestCatalog_AllSlugsResolve(t *testing.T) {
	t.Parallel()

	catalog, err := docs.Catalog()
	if err != nil {
		t.Fatalf("Catalog() error: %v", err)
	}
	if len(catalog) == 0 {
		t.Fatal("Catalog() returned empty slice")
	}

	for _, entry := range catalog {
		t.Run(entry.Slug, func(t *testing.T) {
			t.Parallel()

			data, err := docs.ReadSlug(entry.Slug)
			if err != nil {
				t.Fatalf("ReadSlug(%q) error: %v", entry.Slug, err)
			}
			if len(data) == 0 {
				t.Fatalf("ReadSlug(%q) returned empty content", entry.Slug)
			}
			if entry.Size <= 0 {
				t.Fatalf("catalog entry %q has non-positive Size: %d", entry.Slug, entry.Size)
			}
			if entry.Title == "" {
				t.Fatalf("catalog entry %q has empty Title", entry.Slug)
			}
			if entry.Summary == "" {
				t.Fatalf("catalog entry %q has empty Summary", entry.Slug)
			}
		})
	}
}

// TestMailGatingRowNamesMessageManagement asserts that the embedded concepts
// document's MAIL_MANAGE_ENABLED row stops reading as exhaustive over drafts
// (CR-0078 FR-24).
//
// The row previously read "including draft management", which an LLM reading the
// embedded bundle mid-session would take as the complete list of what the gate
// unlocks, so the four received-message write verbs would look unavailable even
// when registered. The assertion reads the embedded bundle rather than the file
// on disk, because the bundle is what a running server serves.
func TestMailGatingRowNamesMessageManagement(t *testing.T) {
	t.Parallel()

	data, err := docs.ReadSlug("concepts")
	if err != nil {
		t.Fatalf("ReadSlug(\"concepts\") error: %v", err)
	}

	var row string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, "`MAIL_MANAGE_ENABLED`") {
			row = line
			break
		}
	}
	if row == "" {
		t.Fatal("no MAIL_MANAGE_ENABLED row found in the embedded concepts document")
	}

	if !strings.Contains(row, "draft management") {
		t.Errorf("MAIL_MANAGE_ENABLED row no longer names draft management: %q", row)
	}
	if !strings.Contains(row, "received-message management") {
		t.Errorf("MAIL_MANAGE_ENABLED row names only draft management, so it reads as exhaustive; it must also name received-message management: %q", row)
	}
	for _, verb := range []string{"move_message", "set_flag", "set_categories", "mark_read"} {
		if !strings.Contains(row, verb) {
			t.Errorf("MAIL_MANAGE_ENABLED row does not name the gated verb %s: %q", verb, row)
		}
	}
}
