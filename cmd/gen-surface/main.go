//go:build ignore

// Command gen-surface writes the code-derived surface manifest to
// site/src/generated/surface.json, mirroring how cmd/gen-llms writes llms.txt
// (CR-0073 Phase 2).
//
// It builds the surface Record by inspecting the live verb builders and the
// configuration inventory, serializes it to canonical JSON, and writes it to the
// committed manifest path. It performs no network call and reads no credential,
// so it runs in continuous integration and offline. Regenerating against an
// unchanged tree produces a byte-identical file, which the drift check depends on.
//
// @agents-index: generator that writes site/src/generated/surface.json.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/desek/outlook-local-mcp/internal/surface"
)

// manifestPath is the committed manifest location, relative to the repository
// root, which is the working directory when the generator is invoked via the
// make target.
const manifestPath = "site/src/generated/surface.json"

func main() {
	data, err := surface.Serialize(surface.BuildRecord())
	if err != nil {
		fmt.Fprintf(os.Stderr, "gen-surface: serialize: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "gen-surface: mkdir: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "gen-surface: write %s: %v\n", manifestPath, err)
		os.Exit(1)
	}
}
