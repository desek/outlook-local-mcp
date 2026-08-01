// Package tools_test contains the golden inventory assertion over the full
// registered verb surface (CR-0071 Stage 3). A framework bump (mcp-go) can alter
// tool registration, drop a verb, or change an annotation hint without any file
// under internal/tools/ changing, so NFR-1 ("no observable MCP surface change")
// needs a check that reads the registry itself. This is that check.
package tools_test

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/config"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/observability"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	server "github.com/desek/outlook-local-mcp/internal/server"
)

// verbInventoryGolden is the committed golden set of every registered
// {domain}.{operation} identity paired with its four MCP annotation hints, in
// the canonical form "domain.operation ro=%t de=%t id=%t ow=%t", sorted.
//
// It is generated under the maximal configuration (AuthMethod "auth_code" plus
// both mail flags) so every gateable verb is present. This is deliberately a
// golden list rather than a count: a count would pass if one verb were dropped
// and another added, which is exactly the shape a framework migration failure
// takes. Per the project's standard on golden diffs, a failure here is a
// question, not a verdict — it fails identically for an accidental regression
// and for an intentional future change, so the diff is read before deciding
// which. When a change to the verb surface is intentional, regenerate this list
// from the test's failure output and review the delta.
var verbInventoryGolden = []string{
	"account.add ro=false de=false id=false ow=true",
	"account.help ro=true de=false id=true ow=false",
	"account.list ro=true de=false id=true ow=false",
	"account.login ro=false de=false id=false ow=true",
	"account.logout ro=false de=false id=true ow=false",
	"account.refresh ro=false de=false id=true ow=true",
	"account.remove ro=false de=true id=true ow=false",
	"calendar.cancel_meeting ro=false de=true id=true ow=true",
	"calendar.create_event ro=false de=false id=false ow=true",
	"calendar.create_meeting ro=false de=false id=false ow=true",
	"calendar.delete_event ro=false de=true id=true ow=true",
	"calendar.get_event ro=true de=false id=true ow=true",
	"calendar.get_free_busy ro=true de=false id=true ow=true",
	"calendar.help ro=true de=false id=true ow=false",
	"calendar.list_calendars ro=true de=false id=true ow=true",
	"calendar.list_events ro=true de=false id=true ow=true",
	"calendar.reschedule_event ro=false de=false id=true ow=true",
	"calendar.reschedule_meeting ro=false de=false id=true ow=true",
	"calendar.respond_event ro=false de=false id=true ow=true",
	"calendar.search_events ro=true de=false id=true ow=true",
	"calendar.update_event ro=false de=false id=true ow=true",
	"calendar.update_meeting ro=false de=false id=true ow=true",
	"mail.create_draft ro=false de=false id=false ow=true",
	"mail.create_forward_draft ro=false de=false id=false ow=true",
	"mail.create_reply_draft ro=false de=false id=false ow=true",
	"mail.delete_draft ro=false de=true id=true ow=true",
	"mail.get_attachment ro=true de=false id=true ow=true",
	"mail.get_conversation ro=true de=false id=true ow=true",
	"mail.get_message ro=true de=false id=true ow=true",
	"mail.help ro=true de=false id=true ow=false",
	"mail.list_attachments ro=true de=false id=true ow=true",
	"mail.list_folders ro=true de=false id=true ow=true",
	"mail.list_messages ro=true de=false id=true ow=true",
	"mail.search_messages ro=true de=false id=true ow=true",
	"mail.update_draft ro=false de=false id=true ow=true",
	"system.about ro=true de=false id=true ow=false",
	"system.complete_auth ro=false de=false id=false ow=true",
	"system.get_docs ro=true de=false id=true ow=false",
	"system.help ro=true de=false id=true ow=false",
	"system.list_docs ro=true de=false id=true ow=false",
	"system.search_docs ro=true de=false id=true ow=false",
	"system.status ro=true de=false id=true ow=false",
}

// buildFullVerbInventory returns the canonical, sorted inventory lines for every
// verb the server can host, materialising each verb's four annotation hints the
// same way the aggregate fold does (applying the opaque mcp.ToolOption closures
// to a zero-value mcp.Tool and reading the resulting hint pointers). A nil hint
// pointer is rendered as false, its conservative value.
func buildFullVerbInventory(t *testing.T) []string {
	t.Helper()

	meter := noop.NewMeterProvider().Meter("test")
	m, err := observability.InitMetrics(meter)
	if err != nil {
		t.Fatalf("InitMetrics: %v", err)
	}
	tracer := tracenoop.NewTracerProvider().Tracer("test")
	identityMW := func(h mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc { return h }

	r := auth.NewAccountRegistry()
	_ = r.Add(&auth.AccountEntry{Label: "default", Authenticated: true})

	// auth_code plus both mail flags registers every verb the server can host.
	cfg := config.Config{
		AuthRecordPath:    "/tmp/test",
		CacheName:         "test",
		AuthMethod:        "auth_code",
		MailEnabled:       true,
		MailManageEnabled: true,
	}
	verbSets := server.BuildDomainVerbSets(cfg, graph.RetryConfig{}, 30*time.Second, m, tracer, identityMW, r)

	var lines []string
	for domain, verbs := range verbSets {
		for _, v := range verbs {
			var mt mcp.Tool
			for _, opt := range v.Annotations {
				opt(&mt)
			}
			a := mt.Annotations
			lines = append(lines, fmt.Sprintf("%s.%s ro=%t de=%t id=%t ow=%t",
				domain, v.Name,
				hint(a.ReadOnlyHint), hint(a.DestructiveHint),
				hint(a.IdempotentHint), hint(a.OpenWorldHint)))
		}
	}
	sort.Strings(lines)
	return lines
}

// hint dereferences an annotation hint pointer, treating nil (undeclared) as
// false, the conservative value the aggregate fold assigns it.
func hint(p *bool) bool {
	return p != nil && *p
}

// TestVerbInventoryUnchangedAfterUpgrade asserts the registered
// {domain}.{operation} set and each verb's four annotation hints match the
// committed golden list (CR-0071 AC-6, NFR-1). It is the check that makes a
// framework bump's effect on the MCP surface non-theoretical: a dropped,
// renamed, or re-classified verb fails here even when no file under
// internal/tools/ changed.
func TestVerbInventoryUnchangedAfterUpgrade(t *testing.T) {
	got := buildFullVerbInventory(t)

	// Index both sides so the diff names exactly what changed rather than
	// reporting a positional mismatch.
	wantSet := make(map[string]bool, len(verbInventoryGolden))
	for _, l := range verbInventoryGolden {
		wantSet[l] = true
	}
	gotSet := make(map[string]bool, len(got))
	for _, l := range got {
		gotSet[l] = true
	}

	for _, l := range got {
		if !wantSet[l] {
			t.Errorf("verb inventory line present but not in golden: %q", l)
		}
	}
	for _, l := range verbInventoryGolden {
		if !gotSet[l] {
			t.Errorf("golden verb inventory line missing from registry: %q", l)
		}
	}

	if t.Failed() {
		t.Logf("regenerate the golden from the current registry:\nvar verbInventoryGolden = []string{")
		for _, l := range got {
			t.Logf("\t%q,", l)
		}
		t.Logf("}")
	}
}
