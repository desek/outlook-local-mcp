// Package server — this file tests that the inspection builder gates the same
// domains the registration path gates.
//
// The two are separate code paths over one domain set: a domain registered by
// RegisterTools alone is invisible to the surface generator and to the
// manifest-sync check, both of which read only BuildDomainVerbSets. This test
// is what makes that divergence fail the build.
//
// @agents-index: tests that BuildDomainVerbSets gates contacts on the same flag
// RegisterTools does.
package server

import (
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/observability"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// TestBuildDomainVerbSets_ContactsFollowsFlag asserts that the inspection
// builder yields a contacts key with the domain's five verbs when the flag is
// set, and no contacts key at all when it is not.
func TestBuildDomainVerbSets_ContactsFollowsFlag(t *testing.T) {
	m, err := observability.InitMetrics(noop.NewMeterProvider().Meter("test"))
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}
	tracer := tracenoop.NewTracerProvider().Tracer("test")

	enabled := testConfig()
	enabled.ContactsEnabled = true

	sets := BuildDomainVerbSets(enabled, graph.RetryConfig{}, 30*time.Second, m, tracer, identityMW, testRegistry())
	verbs, ok := sets["contacts"]
	if !ok {
		t.Fatal("BuildDomainVerbSets omits the contacts domain although the flag is set; the surface manifest would not see it")
	}
	if got := len(verbs); got != 5 {
		t.Errorf("contacts domain built with %d verbs, want 5", got)
	}

	sets = BuildDomainVerbSets(testConfig(), graph.RetryConfig{}, 30*time.Second, m, tracer, identityMW, testRegistry())
	if _, ok := sets["contacts"]; ok {
		t.Error("BuildDomainVerbSets returns a contacts key although the flag is unset")
	}
}
