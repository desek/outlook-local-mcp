// Package server — this file grades the wiring of the calendar domain's
// scheduling reads: that they register unconditionally, that they are on the
// read chain rather than behind the read-only guard, and that the identity
// their middleware carries is the one an operator later greps for.
//
// The handlers themselves are graded in internal/tools; what is under test here
// is only the registration, which is where a read verb can be wired through the
// write wrapper and still pass every handler test in the repository.
package server

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/audit"
	"github.com/desek/outlook-local-mcp/internal/observability"
	"github.com/desek/outlook-local-mcp/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// schedulingReadVerbs names the calendar verbs this file grades.
func schedulingReadVerbs() []string {
	return []string{"find_meeting_times", "get_schedule"}
}

// buildSchedulingTestVerbs builds the calendar verb slice with the read-only
// mode caller-chosen, returning it keyed by verb name.
//
// Both middleware factories are the identity, so a handler runs with no Graph
// client bound in the context and fails at client resolution rather than
// issuing a network call. That failure is the point: it is distinguishable from
// the read-only refusal, which is what the guard test reads.
func buildSchedulingTestVerbs(t *testing.T, readOnly bool) map[string]tools.Verb {
	t.Helper()

	m, err := observability.InitMetrics(noop.NewMeterProvider().Meter("test"))
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}

	identity := func(h mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc { return h }
	verbs, _ := buildCalendarVerbs(calendarVerbsConfig{
		m:                 m,
		tracer:            tracenoop.NewTracerProvider().Tracer("test"),
		authMW:            identity,
		accountResolverMW: identity,
		readOnly:          readOnly,
	})

	byName := make(map[string]tools.Verb, len(verbs))
	for _, v := range verbs {
		byName[v.Name] = v
	}
	return byName
}

// schedulingArgs returns arguments sufficient to carry each verb past its own
// parameter validation, so what the invocation reaches is the middleware chain
// rather than a refusal the handler raised first.
func schedulingArgs(verb string) map[string]any {
	switch verb {
	case "find_meeting_times":
		return map[string]any{"attendees": `[{"email":"alice@contoso.com","type":"required"}]`}
	default:
		return map[string]any{"schedules": "alice@contoso.com", "date": "tomorrow"}
	}
}

// TestSchedulingReadsRegisteredAndReadOnly asserts that both scheduling reads
// are present in the calendar verb slice and that neither is blocked when the
// server runs in read-only mode.
//
// The slice is built with readOnly true precisely because that is the
// configuration in which a misrouted read announces itself: a read wired
// through wrapWrite would return the guard's refusal here while behaving
// identically in every other configuration.
func TestSchedulingReadsRegisteredAndReadOnly(t *testing.T) {
	audit.InitAuditLog(false, "")
	byName := buildSchedulingTestVerbs(t, true)

	for _, name := range schedulingReadVerbs() {
		v, ok := byName[name]
		if !ok {
			t.Errorf("verb %q is not registered in the calendar domain", name)
			continue
		}

		req := mcp.CallToolRequest{}
		req.Params.Arguments = schedulingArgs(name)
		result, err := v.Handler(context.Background(), req)
		if err != nil {
			t.Fatalf("%s handler returned a transport error: %v", name, err)
		}
		if result == nil || len(result.Content) == 0 {
			t.Fatalf("%s handler returned no content", name)
		}
		tc, ok := result.Content[0].(mcp.TextContent)
		if !ok {
			t.Fatalf("%s: expected TextContent, got %T", name, result.Content[0])
		}
		if strings.Contains(tc.Text, "read-only") {
			t.Errorf("verb %q was blocked in read-only mode: %q", name, tc.Text)
		}
	}
}

// TestSchedulingReadsCarryDotIdentity asserts that the audit record emitted for
// each scheduling read carries the calendar.<verb> identity passed to the
// middleware chain, classified as a read.
//
// The identity is the string that surfaces in the audit log and in the
// OpenTelemetry attributes, so a verb registered under a mismatched one would
// be invisible in exactly the record an operator consults after the fact. The
// handlers run with no Graph client bound and issue no network call; AuditWrap
// still emits the record, which is the surface under test.
func TestSchedulingReadsCarryDotIdentity(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "audit.log")
	audit.InitAuditLog(true, logPath)
	// Every other test in this package runs with auditing off; restore that.
	defer audit.InitAuditLog(false, "")

	byName := buildSchedulingTestVerbs(t, false)

	for _, name := range schedulingReadVerbs() {
		v, ok := byName[name]
		if !ok {
			t.Fatalf("verb %q is not registered in the calendar domain", name)
		}
		req := mcp.CallToolRequest{}
		req.Params.Arguments = schedulingArgs(name)
		if _, err := v.Handler(context.Background(), req); err != nil {
			t.Fatalf("%s handler returned a transport error: %v", name, err)
		}
	}

	f, err := os.Open(logPath) //nolint:gosec // path is the test's own TempDir
	if err != nil {
		t.Fatalf("audit log was not written: %v", err)
	}
	defer func() { _ = f.Close() }()

	seen := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry struct {
			ToolName      string `json:"tool_name"`
			OperationType string `json:"operation_type"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatalf("audit line is not JSON: %v", err)
		}
		seen[entry.ToolName] = entry.OperationType
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("reading the audit log: %v", err)
	}

	for _, name := range schedulingReadVerbs() {
		identity := "calendar." + name
		op, recorded := seen[identity]
		if !recorded {
			t.Errorf("no audit record carries the identity %q; the middleware chain was wrapped under a different string", identity)
			continue
		}
		if op != "read" {
			t.Errorf("audit record for %s carries operation_type %q, want \"read\"", identity, op)
		}
	}
}
