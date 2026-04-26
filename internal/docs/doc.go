// Package docs provides the embedded documentation bundle for outlook-local-mcp.
//
// The bundle is declared in the top-level docs package (github.com/desek/outlook-local-mcp/docs)
// and consumed here. The canonical user-facing Markdown files live at docs/{slug}.md
// so they are visible to humans browsing the repository on GitHub. This package
// is a pure consumer of that bundle.
//
// The package exposes three public surfaces:
//
//   - [Catalog] — a pre-built slice of [Entry] descriptors (slug, title, summary, tags, size).
//   - [ReadSlug] — returns raw Markdown bytes for a slug from the bundle.
//   - [Search] — a case-insensitive substring and token search function that returns
//     ranked [Result] values with ±2 lines of snippet context and 1-based line numbers.
//
// Engineering documentation (Change Requests under docs/cr/, the reference files
// under docs/reference/, research notes under docs/research/, and CHANGELOG.md)
// is intentionally excluded from the bundle and from every public function in this
// package. A build-time test enforces this exclusion.
//
// This package is used by Phase 2 (MCP resources + system verb handlers) and
// Phase 3 (error see-hints, status integration). See CR-0061 for the full design.
package docs
