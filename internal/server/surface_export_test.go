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
// point builds every domain verb slice from a credential-free configuration
// with nil metrics, tracer, and registry, without panicking. Building constructs
// and wraps handlers but never invokes them, so no credential is read and no
// Graph call is made (NFR-2, FR-1). Each slice must be non-empty.
//
// Only the contacts gate is opened, because that domain is not registered at all
// when it is closed and would otherwise be absent from the assertion. Opening a
// gate reads no credential, so the property under test is unaffected.
func TestBuildVerbsRequiresNoCredentials(t *testing.T) {
	// No client, no accounts, no credentials; only the contacts gate is open.
	cfg := config.Config{ContactsEnabled: true}

	sets := BuildVerbsForInspection(cfg)

	for _, domain := range []string{"calendar", "account", "system", "mail", "contacts"} {
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
