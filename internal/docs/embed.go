package docs

import "embed"

// Bundle is the embedded documentation filesystem.
//
// It contains exactly three user-facing Markdown files at the paths listed
// below. Engineering documentation (CRs, reference spec, research notes,
// CHANGELOG.md) is excluded by the explicit file list — not by a glob — so
// accidentally added paths fail the build rather than silently enlarging the
// binary.
//
//go:embed files/readme.md files/quickstart.md files/troubleshooting.md
var Bundle embed.FS
