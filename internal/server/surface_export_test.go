// Package server — test for the zero-dependency surface inspection entry point
// (CR-0073 Phase 1).
//
// @agents-index: asserts BuildVerbsForInspection needs no credentials.
package server

import (
	"testing"

	"github.com/desek/outlook-local-mcp/internal/config"
)

// TestBuildVerbsRequiresNoCredentials asserts that the surface inspection entry
// point builds all four domain verb slices from a zero-value configuration with
// nil metrics, tracer, and registry, without panicking. Building constructs and
// wraps handlers but never invokes them, so no credential is read and no Graph
// call is made (NFR-2, FR-1). The four slices must each be non-empty.
func TestBuildVerbsRequiresNoCredentials(t *testing.T) {
	var cfg config.Config // zero value: no client, no accounts, no gates enabled

	sets := BuildVerbsForInspection(cfg)

	for _, domain := range []string{"calendar", "account", "system", "mail"} {
		verbs, ok := sets[domain]
		if !ok {
			t.Errorf("missing domain %q in inspection result", domain)
			continue
		}
		if len(verbs) == 0 {
			t.Errorf("domain %q built an empty verb slice", domain)
		}
	}
}
