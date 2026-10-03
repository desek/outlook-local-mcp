// Package server — this file holds the registry-derived tests over the contacts
// domain's published verb surface: its exact verb set, the absence of any write
// verb, and the audit identity every verb is wrapped under.
//
// @agents-index: registry-derived tests over the contacts domain's verb set,
// read-only property, and audit identity.
package server

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/audit"
	"github.com/desek/outlook-local-mcp/internal/observability"
	"github.com/desek/outlook-local-mcp/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// buildTestContactsVerbs builds the contacts verb slice with pass-through
// middleware and noop telemetry, so a test reads the same slice the server
// registers without any credential or network dependency.
func buildTestContactsVerbs(t *testing.T) []tools.Verb {
	t.Helper()

	m, err := observability.InitMetrics(noop.NewMeterProvider().Meter("test"))
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}

	verbs, _ := buildContactsVerbs(contactsVerbsConfig{
		m:                 m,
		tracer:            tracenoop.NewTracerProvider().Tracer("test"),
		authMW:            identityMW,
		accountResolverMW: identityMW,
	})
	return verbs
}

// verbIsReadOnlyHint materialises a verb's annotation options against a
// throwaway tool and reports the resulting readOnlyHint, treating an undeclared
// hint as false. It reads the annotations the way the aggregate builder does,
// so the assertion cannot drift from the publication path.
func verbIsReadOnlyHint(v tools.Verb) bool {
	var tool mcp.Tool
	for _, opt := range v.Annotations {
		opt(&tool)
	}
	return tool.Annotations.ReadOnlyHint != nil && *tool.Annotations.ReadOnlyHint
}

// TestContactsVerbsRegisterFive asserts the domain registers exactly the five
// verbs it is scoped to and nothing else. The set is the scope boundary: a
// sixth verb appearing here is a scope expansion, not a detail.
func TestContactsVerbsRegisterFive(t *testing.T) {
	verbs := buildTestContactsVerbs(t)

	got := make([]string, 0, len(verbs))
	for _, v := range verbs {
		got = append(got, v.Name)
	}
	sort.Strings(got)

	want := []string{"get_contact", "get_person", "help", "list_people", "search"}
	if len(got) != len(want) {
		t.Fatalf("contacts registers %d verbs (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("contacts verb set = %v, want %v", got, want)
			break
		}
	}
}

// TestContactsExposesNoWriteVerb asserts that every registered verb declares
// readOnlyHint: true and destructiveHint: false, which is the domain's
// load-bearing safety property: a read-only domain cannot mutate the mailbox,
// so no client confirmation gate is ever tripped by a contacts call.
func TestContactsExposesNoWriteVerb(t *testing.T) {
	for _, v := range buildTestContactsVerbs(t) {
		if !verbIsReadOnlyHint(v) {
			t.Errorf("verb %q does not declare readOnlyHint: true; the contacts domain registers reads only", v.Name)
		}

		var tool mcp.Tool
		for _, opt := range v.Annotations {
			opt(&tool)
		}
		if tool.Annotations.DestructiveHint != nil && *tool.Annotations.DestructiveHint {
			t.Errorf("verb %q declares destructiveHint: true; the contacts domain removes nothing", v.Name)
		}
	}
}

// TestContactsVerbsWrappedUnderDomainIdentity asserts that the audit record
// emitted for every contacts verb carries the contacts.<verb> identity and the
// read operation type.
//
// The identity is what surfaces in the audit log and in the OpenTelemetry
// attributes, so a verb wrapped under a mismatched string would be invisible in
// exactly the record an operator consults after the fact. The verbs are derived
// from the built slice rather than listed, so a verb added later without an
// audit identity fails the build instead of shipping unasserted.
//
// The handlers are invoked with no Graph client bound, so each fails at account
// resolution and issues no network call; AuditWrap still emits the record,
// which is the surface under test.
//
// The help verb is excluded: it is the shared registry-rendering verb every
// domain embeds unwrapped, so it emits no audit record in any domain, and
// asserting one here would grade a property the shared verb does not have.
func TestContactsVerbsWrappedUnderDomainIdentity(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "audit.log")
	audit.InitAuditLog(true, logPath)
	// Every other test in this package runs with auditing off; restore that.
	defer audit.InitAuditLog(false, "")

	verbs := buildTestContactsVerbs(t)

	for _, v := range verbs {
		if v.Name == "help" {
			continue
		}
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{
			"query":      "alex",
			"contact_id": "contact-1",
			"person_id":  "person-1",
		}
		if _, err := v.Handler(context.Background(), req); err != nil {
			t.Fatalf("%s handler returned a transport error: %v", v.Name, err)
		}
	}

	f, err := os.Open(logPath) //nolint:gosec // path is the test's own TempDir
	if err != nil {
		t.Fatalf("audit log was not written: %v", err)
	}
	defer func() { _ = f.Close() }()

	seen := make(map[string]bool)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry struct {
			ToolName      string `json:"tool_name"`
			OperationType string `json:"operation_type"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatalf("audit line is not JSON: %v", err)
		}
		seen[entry.ToolName] = true
		if entry.OperationType != "read" {
			t.Errorf("audit record for %s carries operation_type %q, want read", entry.ToolName, entry.OperationType)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("reading the audit log: %v", err)
	}

	for _, v := range verbs {
		if v.Name == "help" {
			continue
		}
		if !seen["contacts."+v.Name] {
			t.Errorf("no audit record carries the identity %q; the middleware chain was wrapped under a different string", "contacts."+v.Name)
		}
	}
}

// TestContactsPagingParametersDeclared asserts that search and list_people
// declare the paging parameters their handlers send as $top and $skip.
func TestContactsPagingParametersDeclared(t *testing.T) {
	want := map[string][]string{
		"search":      {"limit"},
		"list_people": {"limit", "skip"},
	}
	for _, v := range buildTestContactsVerbs(t) {
		if len(want[v.Name]) == 0 {
			continue
		}
		props := mcp.NewTool("_introspect", v.Schema...).InputSchema.Properties
		for _, p := range want[v.Name] {
			if _, ok := props[p]; !ok {
				t.Errorf("contacts.%s does not declare %q", v.Name, p)
			}
		}
	}
}
