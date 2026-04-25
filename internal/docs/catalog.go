package docs

import (
	"fmt"
	"io/fs"

	extdocs "github.com/desek/outlook-local-mcp/docs"
)

// Entry describes a single document in the embedded bundle.
//
// Slug is the stable, URL-safe identifier used in doc:// URIs and tool parameters.
// Title is the human-readable display name shown in list output.
// Summary is a one-sentence description of the document's purpose.
// Tags is a slice of searchable labels that describe the document's content.
// Size is the uncompressed byte count of the embedded file.
type Entry struct {
	// Slug is the URL-safe document identifier (e.g., "readme", "troubleshooting").
	Slug string

	// Title is the human-readable display name for UI and list output.
	Title string

	// Summary is a one-sentence description of the document.
	Summary string

	// Tags are searchable labels describing the document content.
	Tags []string

	// Size is the uncompressed byte count of the embedded file.
	Size int64
}

// staticEntries defines the catalog metadata for every embedded document.
// The slug must match the filename stem in docs.Bundle ({slug}.md).
// This list is the single source of truth for the bundle allowlist.
var staticEntries = []struct {
	slug    string
	title   string
	summary string
	tags    []string
}{
	{
		slug:    "readme",
		title:   "README",
		summary: "Project overview, install pointer, supported features, and links to further documentation.",
		tags:    []string{"overview", "install", "features"},
	},
	{
		slug:    "quickstart",
		title:   "Quick Start Guide",
		summary: "Step-by-step setup: prerequisites, installation, authentication, and first tool call.",
		tags:    []string{"setup", "install", "auth", "quickstart"},
	},
	{
		slug:    "troubleshooting",
		title:   "Troubleshooting Guide",
		summary: "Common failure modes and remediation steps: auth errors, Graph throttling, mail flags, Keychain, and account lifecycle.",
		tags:    []string{"auth", "error", "keychain", "throttling", "mail", "account", "troubleshooting"},
	},
}

// Catalog returns the full catalog of embedded documents with resolved file sizes.
//
// It reads each file from [extdocs.Bundle] to determine its uncompressed size,
// then assembles [Entry] values from the static metadata defined in staticEntries.
// An error is returned only when an embedded file is unreadable — which
// indicates a broken build rather than a runtime condition.
func Catalog() ([]Entry, error) {
	entries := make([]Entry, 0, len(staticEntries))
	for _, s := range staticEntries {
		path := s.slug + ".md"
		info, err := fs.Stat(extdocs.Bundle, path)
		if err != nil {
			return nil, fmt.Errorf("docs: catalog: slug %q not found in bundle: %w", s.slug, err)
		}
		entries = append(entries, Entry{
			Slug:    s.slug,
			Title:   s.title,
			Summary: s.summary,
			Tags:    s.tags,
			Size:    info.Size(),
		})
	}
	return entries, nil
}

// MustCatalog returns the catalog or panics on error.
//
// It is intended for use at package init time or in tests where a broken
// bundle is a programming error, not a recoverable condition.
func MustCatalog() []Entry {
	c, err := Catalog()
	if err != nil {
		panic(err)
	}
	return c
}

// ReadSlug returns the raw Markdown bytes for the document identified by slug.
//
// It returns an error when the slug is not in the bundle — either because the
// slug is unknown or because the embedded file is missing (broken build).
func ReadSlug(slug string) ([]byte, error) {
	path := slug + ".md"
	data, err := extdocs.Bundle.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("docs: slug %q not found in bundle: %w", slug, err)
	}
	return data, nil
}
