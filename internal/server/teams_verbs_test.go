// Package server — this file holds the registry-derived tests over the teams
// domain's published verb surface: its exact verb set, the capabilities that
// are deliberately absent from it, the audit identity every verb is wrapped
// under, and the one verb that must not offer an output tier.
//
// @agents-index: registry-derived tests over the teams domain's verb set,
// absent capabilities, audit identity, and compose_reply's tier-less schema.
package server

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/audit"
	"github.com/desek/outlook-local-mcp/internal/observability"
	"github.com/desek/outlook-local-mcp/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// buildTestTeamsVerbs builds the teams verb slice with pass-through middleware
// and noop telemetry, so a test reads the same slice the server registers
// without any credential or network dependency.
func buildTestTeamsVerbs(t *testing.T) []tools.Verb {
	t.Helper()

	m, err := observability.InitMetrics(noop.NewMeterProvider().Meter("test"))
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}

	verbs, _ := buildTeamsVerbs(teamsVerbsConfig{
		m:                 m,
		tracer:            tracenoop.NewTracerProvider().Tracer("test"),
		authMW:            identityMW,
		accountResolverMW: identityMW,
	})
	return verbs
}

// teamsVerbByName returns the named verb from the built slice, failing the test
// when it is absent, so a later assertion reads a verb that exists rather than
// a zero value that silently passes.
func teamsVerbByName(t *testing.T, name string) tools.Verb {
	t.Helper()

	for _, v := range buildTestTeamsVerbs(t) {
		if v.Name == name {
			return v
		}
	}
	t.Fatalf("the teams domain registers no verb named %q", name)
	return tools.Verb{}
}

// teamsSchemaProperties materialises a verb's own Schema options against a
// throwaway tool and returns the declared property names. It reads the schema
// the way the aggregate builder reads it, so the properties inspected are the
// ones the verb declares rather than the union the aggregate publishes.
func teamsSchemaProperties(opts []mcp.ToolOption) map[string]any {
	if len(opts) == 0 {
		return map[string]any{}
	}
	return mcp.NewTool("_introspect", opts...).InputSchema.Properties
}

// TestTeamsVerbsRegisterThirteen asserts the domain registers exactly the
// thirteen verbs the inventory states. The set is the scope boundary: a
// fourteenth verb appearing here is a scope expansion, not a detail.
func TestTeamsVerbsRegisterThirteen(t *testing.T) {
	verbs := buildTestTeamsVerbs(t)

	got := make([]string, 0, len(verbs))
	for _, v := range verbs {
		got = append(got, v.Name)
	}
	sort.Strings(got)

	want := []string{
		"compose_reply",
		"get_channel_message",
		"get_chat_message",
		"get_online_meeting",
		"get_transcript",
		"help",
		"list_channel_message_replies",
		"list_channel_messages",
		"list_chat_message_replies",
		"list_chat_messages",
		"list_chats",
		"list_transcripts",
		"search",
	}
	if len(got) != len(want) {
		t.Fatalf("teams registers %d verbs (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("teams verb set = %v, want %v", got, want)
			break
		}
	}
}

// TestTeamsRegistryExposesNoOutOfScopeVerb asserts that no capability outside
// the domain's read scope has been added to the registry.
//
// The verb set alone would catch a rename, but not the reason a capability is
// excluded: chat creation, sending, presence, recording, and attendance each
// need a permission this server never requests, and a verb for one of them
// would fail at consent rather than at review. The name prefixes are checked
// directly so an addition is refused here, where the exclusion is documented,
// rather than in a scope test that only counts.
func TestTeamsRegistryExposesNoOutOfScopeVerb(t *testing.T) {
	excludedNames := map[string]bool{
		"get_chat":           true,
		"create_chat":        true,
		"update_chat":        true,
		"list_joined_teams":  true,
		"list_channels":      true,
		"get_presence":       true,
		"list_recordings":    true,
		"get_recording":      true,
		"get_attendance":     true,
		"list_attendance":    true,
		"send_chat_message":  true,
		"reply_to_message":   true,
		"create_online_meet": true,
	}
	excludedPrefixes := []string{"send_", "create_", "update_", "delete_", "post_"}

	for _, v := range buildTestTeamsVerbs(t) {
		if excludedNames[v.Name] {
			t.Errorf("verb %q is registered; it is a capability this domain deliberately does not expose", v.Name)
		}
		for _, p := range excludedPrefixes {
			if strings.HasPrefix(v.Name, p) {
				t.Errorf("verb %q is registered; a verb named %s* writes, and this domain registers reads only", v.Name, p)
			}
		}
	}
}

// TestTeamsExposesNoWriteVerb asserts that every registered verb declares
// readOnlyHint: true and destructiveHint: false, including compose_reply, whose
// name invites the opposite reading. The classification is the load-bearing
// statement that the domain does not communicate: it prepares text and posts
// nothing.
func TestTeamsExposesNoWriteVerb(t *testing.T) {
	for _, v := range buildTestTeamsVerbs(t) {
		if !verbIsReadOnlyHint(v) {
			t.Errorf("verb %q does not declare readOnlyHint: true; the teams domain registers reads only", v.Name)
		}

		var tool mcp.Tool
		for _, opt := range v.Annotations {
			opt(&tool)
		}
		if tool.Annotations.DestructiveHint != nil && *tool.Annotations.DestructiveHint {
			t.Errorf("verb %q declares destructiveHint: true; the teams domain removes nothing", v.Name)
		}
	}
}

// TestComposeReply_DeclaresNoOutputParameter asserts that compose_reply
// publishes no output parameter.
//
// It returns prepared text rather than a projection of a Graph resource, so
// there is nothing to tier; a declared tier would advertise a summary and a raw
// form of an answer that has only one form. The check reads the registered
// verb, because the schema is what a client sees and a handler that ignores the
// parameter would still leave it published.
func TestComposeReply_DeclaresNoOutputParameter(t *testing.T) {
	v := teamsVerbByName(t, "compose_reply")

	if _, declared := teamsSchemaProperties(v.Schema)["output"]; declared {
		t.Error("compose_reply declares an output parameter; it returns prepared text unconditionally and has no resource to tier")
	}
}

// TestTeamsReadVerbsDeclareOutput asserts that every verb which does project a
// Graph resource declares the output parameter, so the tier rule is graded in
// both directions: compose_reply and help have none, and the eleven reads all
// do. The cases are derived from the built slice, so a verb added later is
// covered without anyone extending a list here.
func TestTeamsReadVerbsDeclareOutput(t *testing.T) {
	tierless := map[string]bool{"help": true, "compose_reply": true}

	checked := 0
	for _, v := range buildTestTeamsVerbs(t) {
		if tierless[v.Name] {
			continue
		}
		checked++
		if _, declared := teamsSchemaProperties(v.Schema)["output"]; !declared {
			t.Errorf("verb %q declares no output parameter; a resource read implements all three tiers", v.Name)
		}
	}
	if checked != 11 {
		t.Errorf("the check selected %d resource reads, want 11; the derivation is broken, not the registry", checked)
	}
}

// TestTeamsVerbsCarryDomainQualifiedIdentity asserts that the audit record
// emitted for every teams verb carries the teams.<verb> identity and the read
// operation type.
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
// domain embeds unwrapped, so it emits no audit record in any domain.
func TestTeamsVerbsCarryDomainQualifiedIdentity(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "audit.log")
	audit.InitAuditLog(true, logPath)
	// Every other test in this package runs with auditing off; restore that.
	defer audit.InitAuditLog(false, "")

	verbs := buildTestTeamsVerbs(t)

	for _, v := range verbs {
		if v.Name == "help" {
			continue
		}
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{
			"query":         "release",
			"chat_id":       "19:chat@thread.v2",
			"team_id":       "team-1",
			"channel_id":    "19:channel@thread.tacv2",
			"message_id":    "1700000000000",
			"meeting_id":    "meeting-1",
			"transcript_id": "transcript-1",
			"body":          "prepared reply",
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
		if !seen["teams."+v.Name] {
			t.Errorf("no audit record carries the identity %q; the middleware chain was wrapped under a different string", "teams."+v.Name)
		}
	}
}
