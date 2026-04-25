// Package docs provides the embedded documentation bundle for outlook-local-mcp.
//
// The bundle embeds a curated set of user-facing Markdown files (README.md,
// QUICKSTART.md, and docs/troubleshooting.md) into the compiled binary via
// Go's embed.FS so no filesystem access is required at runtime.
//
// The package exposes three public surfaces:
//
//   - [Bundle] — the raw embed.FS containing the embedded files.
//   - [Catalog] — a pre-built slice of [Entry] descriptors (slug, title, summary, tags, size).
//   - [Search] — a case-insensitive substring and token search function that returns
//     ranked [Result] values with ±2 lines of snippet context and 1-based line numbers.
//
// Engineering documentation (Change Requests under docs/cr/, the reference spec
// docs/reference/outlook-local-mcp-spec.md, research notes under docs/research/,
// and CHANGELOG.md) is intentionally excluded from the bundle and from every
// public function in this package. A build-time test enforces this exclusion.
//
// This package is used by Phase 2 (MCP resources + system verb handlers) and
// Phase 3 (error see-hints, status integration). See CR-0061 for the full design.
package docs
