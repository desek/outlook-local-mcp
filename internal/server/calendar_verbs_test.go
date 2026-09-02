// Package server — this file grades the wiring of the calendar domain's
// scheduling reads and its event-attachment verbs: that they register
// unconditionally, that each is on the chain its classification calls for
// rather than the other one, and that the identity their middleware carries is
// the one an operator later greps for.
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

// eventAttachmentVerbs names the three event-attachment verbs this file grades,
// mapped to the audit operation each is expected to record.
func eventAttachmentVerbs() map[string]string {
	return map[string]string{
		"list_event_attachments": "read",
		"get_event_attachment":   "read",
		"add_event_attachment":   "write",
	}
}

// eventAttachmentArgs returns arguments sufficient to carry each verb past its
// own parameter validation, so an invocation reaches the middleware chain
// rather than a refusal the handler raised first.
func eventAttachmentArgs(verb string) map[string]any {
	switch verb {
	case "list_event_attachments":
		return map[string]any{"event_id": "evt-1"}
	case "get_event_attachment":
		return map[string]any{"event_id": "evt-1", "attachment_id": "att-1"}
	default:
		return map[string]any{"event_id": "evt-1", "name": "notes.txt", "content_bytes": "aGVsbG8gd29ybGQ="}
	}
}

// verbSchemaPropertyNames materialises a verb's schema options onto a throwaway
// tool and returns the property names it declares.
//
// The options are opaque closures, so the only way to read what a verb
// publishes is to apply them and inspect the result, which is also exactly what
// the server does at registration time.
func verbSchemaPropertyNames(opts []mcp.ToolOption) map[string]any {
	if len(opts) == 0 {
		return map[string]any{}
	}
	return mcp.NewTool("_introspect", opts...).InputSchema.Properties
}

// TestCalendarRegistersAttachmentVerbs asserts that all three event-attachment
// verbs are present in the calendar verb slice.
//
// They are calendar data and are gated by nothing: the slice is built here with
// no mail configuration at all, which is the configuration in which a verb
// wired behind a mail flag would go missing.
func TestCalendarRegistersAttachmentVerbs(t *testing.T) {
	audit.InitAuditLog(false, "")
	byName := buildSchedulingTestVerbs(t, false)

	for name := range eventAttachmentVerbs() {
		if _, ok := byName[name]; !ok {
			t.Errorf("verb %q is not registered in the calendar domain", name)
		}
	}
}

// TestEventAttachmentReadSchemasDeclareNoMutation asserts that neither read
// verb publishes a parameter through which a caller could change or remove an
// attachment.
//
// The handler side of this is graded in internal/tools, which observes that
// only GET is ever issued. The schema side has to be graded here, because a
// verb's published parameters are only assembled at registration: a read that
// grew a delete or update parameter would still issue GETs in every existing
// test while advertising a mutation the domain does not intend to offer.
func TestEventAttachmentReadSchemasDeclareNoMutation(t *testing.T) {
	audit.InitAuditLog(false, "")
	byName := buildSchedulingTestVerbs(t, false)

	allowed := map[string]bool{
		"event_id":      true,
		"attachment_id": true,
		"account":       true,
		"output":        true,
	}

	for _, name := range []string{"list_event_attachments", "get_event_attachment"} {
		v, ok := byName[name]
		if !ok {
			t.Errorf("verb %q is not registered in the calendar domain", name)
			continue
		}
		for param := range verbSchemaPropertyNames(v.Schema) {
			if !allowed[param] {
				t.Errorf("read verb %q declares the parameter %q; a read publishes only the identifiers, the account, and the output tier, so any other parameter is a mutation the verb must not offer", name, param)
			}
		}
	}
}

// TestReadOnlyBlocksAddEventAttachment asserts that read-only mode refuses the
// write and leaves the two reads reachable.
//
// This is the configuration in which a misrouted verb announces itself in both
// directions: a read wired through wrapWrite is refused here while behaving
// identically everywhere else, and a write wired through wrap silently accepts
// the call the operator turned the mode on to prevent.
func TestReadOnlyBlocksAddEventAttachment(t *testing.T) {
	audit.InitAuditLog(false, "")
	byName := buildSchedulingTestVerbs(t, true)

	for name, auditOp := range eventAttachmentVerbs() {
		v, ok := byName[name]
		if !ok {
			t.Errorf("verb %q is not registered in the calendar domain", name)
			continue
		}

		req := mcp.CallToolRequest{}
		req.Params.Arguments = eventAttachmentArgs(name)
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

		blocked := strings.Contains(tc.Text, "read-only")
		if auditOp == "write" {
			if !blocked {
				t.Errorf("verb %q was not blocked in read-only mode: %q", name, tc.Text)
				continue
			}
			if !strings.Contains(tc.Text, "calendar."+name) {
				t.Errorf("the read-only refusal for %q does not name the identity calendar.%s: %q", name, name, tc.Text)
			}
			continue
		}
		if blocked {
			t.Errorf("read verb %q was blocked in read-only mode: %q", name, tc.Text)
		}
	}
}

// TestAttachmentVerbsCarryDotIdentity asserts that the audit record emitted for
// each event-attachment verb carries the calendar.<verb> identity passed to the
// middleware chain, classified as the operation the verb performs.
//
// The identity is the string that surfaces in the audit log and in the
// OpenTelemetry attributes, so a verb registered under a mismatched one would
// be invisible in exactly the record an operator consults after the fact. The
// handlers run with no Graph client bound and issue no network call; AuditWrap
// still emits the record, which is the surface under test.
func TestAttachmentVerbsCarryDotIdentity(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "audit.log")
	audit.InitAuditLog(true, logPath)
	// Every other test in this package runs with auditing off; restore that.
	defer audit.InitAuditLog(false, "")

	byName := buildSchedulingTestVerbs(t, false)

	for name := range eventAttachmentVerbs() {
		v, ok := byName[name]
		if !ok {
			t.Fatalf("verb %q is not registered in the calendar domain", name)
		}
		req := mcp.CallToolRequest{}
		req.Params.Arguments = eventAttachmentArgs(name)
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

	for name, wantOp := range eventAttachmentVerbs() {
		identity := "calendar." + name
		op, recorded := seen[identity]
		if !recorded {
			t.Errorf("no audit record carries the identity %q; the middleware chain was wrapped under a different string", identity)
			continue
		}
		if op != wantOp {
			t.Errorf("audit record for %s carries operation_type %q, want %q", identity, op, wantOp)
		}
	}
}
