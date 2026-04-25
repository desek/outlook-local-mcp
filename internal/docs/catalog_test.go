package docs_test

import (
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
