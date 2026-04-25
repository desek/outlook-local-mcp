package docs

import "embed"

// Bundle is the embedded documentation filesystem.
//
// It contains exactly four user-facing Markdown files at the paths listed
// below. Engineering documentation (CRs, reference spec, research notes,
// CHANGELOG.md) is excluded by the explicit file list — not by a glob — so
// accidentally added paths fail the build rather than silently enlarging the
// binary.
//
// The canonical files live here at docs/{slug}.md so they are visible to
// humans browsing the repository on GitHub and to the Go embed directive.
// internal/docs consumes this Bundle rather than declaring its own embed.
//
//go:embed readme.md quickstart.md concepts.md troubleshooting.md
var Bundle embed.FS
