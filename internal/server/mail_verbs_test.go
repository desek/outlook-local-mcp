// Package server — this file holds the registry-derived tests over the mail
// domain's published surface: the search_messages documentation check, the
// shared-parameter description check, the destination-versus-scope assertion,
// and the audit identity assertion for the received-message write verbs.
//
// These tests live in the server package, not tools, because the verbs they
// read are owned by the build*Verb constructors here, and server imports tools
// (not the reverse), so they can reach both the registry and the tools-package
// helpers while a tools-package test could not reach the registry.
//
// @agents-index: registry-derived tests over the mail domain's published verb
// surface, parameter descriptions, and audit identity.
package server

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/audit"
	"github.com/desek/outlook-local-mcp/internal/config"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/observability"
	"github.com/desek/outlook-local-mcp/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// forbiddenPropertyPhrase matches the double-quoted property phrase form
// (subject:"Design Review") that measured behaviour shows Graph rejects. Its
// presence anywhere in the verb description is the specific drift requirement 9
// prohibits, so the description prose is guarded against it directly rather than
// only through the extracted examples.
var forbiddenPropertyPhrase = regexp.MustCompile(`\w+:"`)

// TestDocumentedExamplesAreCanonical asserts that every example documented by
// the search_messages verb is already in the canonical form Microsoft Graph
// accepts, deriving its cases from the verb registry entry rather than from a
// hand-maintained list.
//
// Two assertions run against each documented example:
//
//  1. It normalises without error, so no documented example teaches a query the
//     verb would refuse.
//  2. NormaliseSearchQuery(example) equals example, so each example is already a
//     normalisation fixed point.
//
// The second assertion is what gives the check teeth against re-introduction. A
// re-added double-quoted example such as subject:"Design Review" normalises to
// the parenthesised form "subject:(Design Review)", so the equality fails and
// the build fails. Documentation is deliberately held to a stricter standard
// than caller input: normalisation still accepts and translates the
// double-quoted form at runtime for callers (requirement 2), but a documented
// example must already be canonical. A later reader must read that asymmetry as
// intentional, not as an inconsistency between this test and the runtime.
//
// The description prose is additionally guarded against the forbidden
// double-quoted property phrase, because that is the surface the broken syntax
// lived on and prose examples are not carried in the structured Examples slice.
func TestDocumentedExamplesAreCanonical(t *testing.T) {
	// The wrap stub is never invoked; the test reads the verb's documentation,
	// not its handler. A nil Handler is sufficient.
	wrap := func(_ string, _ string, _ mcpserver.ToolHandlerFunc) tools.Handler {
		return nil
	}
	verb := buildSearchMessagesVerb(mailVerbsConfig{}, graph.RetryConfig{}, wrap)

	if len(verb.Examples) == 0 {
		t.Fatal("search_messages verb documents no examples; expected at least one to govern")
	}

	for i, ex := range verb.Examples {
		raw, ok := ex.Args["query"]
		if !ok {
			t.Fatalf("example %d has no query argument to check", i)
		}
		example, ok := raw.(string)
		if !ok {
			t.Fatalf("example %d query argument is %T, want string", i, raw)
		}

		normalised, err := tools.NormaliseSearchQuery(example)
		if err != nil {
			t.Errorf("documented example %q does not normalise: %v", example, err)
			continue
		}
		if normalised != example {
			t.Errorf("documented example %q is not canonical: normalises to %q; write the canonical form so a documented example is a normalisation fixed point", example, normalised)
		}
	}

	if forbiddenPropertyPhrase.MatchString(verb.Description) {
		t.Errorf("verb description documents the double-quoted property phrase form Graph rejects; write a multi-word property value in parentheses, for example subject:(Design Review)")
	}
}

// maximalMailConfig returns the configuration under which every mail verb is
// registered, so a registry-derived check sees the whole domain rather than the
// default subset.
func maximalMailConfig() config.Config {
	c := testConfig()
	c.MailEnabled = true
	c.MailManageEnabled = true
	return c
}

// mailVerbParams returns, for one verb, its declared parameter names mapped to
// the description that verb declares. It reads the schema the way the aggregate
// builder does, by materialising the verb's Schema options into a throwaway tool
// and reading the resulting properties, so the test cannot drift from the
// publication path it is checking.
func mailVerbParams(v tools.Verb) map[string]string {
	out := make(map[string]string)
	if len(v.Schema) == 0 {
		return out
	}
	t := mcp.NewTool("_introspect", v.Schema...)
	for name, raw := range t.InputSchema.Properties {
		if name == "operation" {
			continue
		}
		schema, _ := raw.(map[string]any)
		desc, _ := schema["description"].(string)
		out[name] = desc
	}
	return out
}

// mailVerbIsReadOnly reports the verb's declared readOnlyHint, read back from
// its own Annotations rather than inferred from its name.
func mailVerbIsReadOnly(v tools.Verb) bool {
	t := mcp.NewTool("_introspect", v.Annotations...)
	return t.Annotations.ReadOnlyHint != nil && *t.Annotations.ReadOnlyHint
}

// TestSharedParametersNameTheirWriteVerbs asserts that every parameter
// name the mail domain declares on both a read-only verb and a write verb has a
// published description naming at least one of the declaring write verbs.
//
// The aggregate tool publishes the union of every verb's parameters and merges
// duplicate names first-occurrence-wins (aggregateSchemaOptions). Because the
// read verbs are registered first, a shared name is published with its read
// description, which would tell a caller that flag_status is a filter while
// set_flag requires it as the value to write.
//
// The cases are derived from the registry rather than listed, so a future shared
// mail parameter is covered without anyone remembering to add it here. That is
// deliberate: correcting named instances is the instance-level remedy this
// project has already recorded as failing to close a class.
//
// account is the single exemption. It is declared with identical text by every
// verb of every domain and carries no read or write sense, so naming its
// declaring write verbs would inflate the published schema for no gain.
func TestSharedParametersNameTheirWriteVerbs(t *testing.T) {
	verbs := BuildVerbsForInspection(maximalMailConfig())["mail"]
	if len(verbs) == 0 {
		t.Fatal("mail domain built no verbs under the maximal configuration")
	}

	// published mirrors the first-occurrence-wins merge the aggregate schema
	// performs, so the string under test is the one callers actually see.
	published := make(map[string]string)
	readDeclarers := make(map[string][]string)
	writeDeclarers := make(map[string][]string)

	for _, v := range verbs {
		readOnly := mailVerbIsReadOnly(v)
		for name, desc := range mailVerbParams(v) {
			if _, ok := published[name]; !ok {
				published[name] = desc
			}
			if readOnly {
				readDeclarers[name] = append(readDeclarers[name], v.Name)
			} else {
				writeDeclarers[name] = append(writeDeclarers[name], v.Name)
			}
		}
	}

	checked := 0
	for name, writers := range writeDeclarers {
		if name == "account" {
			continue
		}
		if len(readDeclarers[name]) == 0 {
			continue
		}
		checked++

		desc := published[name]
		named := false
		for _, w := range writers {
			if strings.Contains(desc, w) {
				named = true
				break
			}
		}
		if !named {
			t.Errorf("parameter %q is declared by read verb(s) %v and write verb(s) %v, but the published description %q names none of the write verbs; rewrite it so the flattened schema states both senses",
				name, readDeclarers[name], writers, desc)
		}
	}

	if checked == 0 {
		t.Error("the check selected no shared mail parameters; the derivation is broken, not the domain")
	}
}

// TestDestinationFolderIDIsNotFolderID asserts that the move
// destination is published under its own name with its own description, and is
// not merged into folder_id.
//
// folder_id scopes a read to a folder; destination_folder_id names where a
// message is moved to. Conflating them in the flattened schema is exactly the
// ambiguity the shared-description rule above exists to prevent, so the
// separation is asserted rather than left to the reader of the constructors.
func TestDestinationFolderIDIsNotFolderID(t *testing.T) {
	verbs := BuildVerbsForInspection(maximalMailConfig())["mail"]

	published := make(map[string]string)
	for _, v := range verbs {
		for name, desc := range mailVerbParams(v) {
			if _, ok := published[name]; !ok {
				published[name] = desc
			}
		}
	}

	scope, ok := published["folder_id"]
	if !ok {
		t.Fatal("the mail domain publishes no folder_id parameter")
	}
	destination, ok := published["destination_folder_id"]
	if !ok {
		t.Fatal("the mail domain publishes no destination_folder_id parameter; move_message must not reuse folder_id")
	}
	if scope == destination {
		t.Errorf("folder_id and destination_folder_id publish the same description %q; the scoping parameter and the move destination are different concepts and must read differently", scope)
	}
}

// mailPublishedSchema returns the flattened mail-domain parameter schema the
// aggregate tool publishes: every verb's declared properties merged
// first-occurrence-wins, which is the merge aggregateSchemaOptions performs.
func mailPublishedSchema(t *testing.T) map[string]map[string]any {
	t.Helper()

	published := make(map[string]map[string]any)
	for _, v := range BuildVerbsForInspection(maximalMailConfig())["mail"] {
		if len(v.Schema) == 0 {
			continue
		}
		tool := mcp.NewTool("_introspect", v.Schema...)
		for name, raw := range tool.InputSchema.Properties {
			if name == "operation" {
				continue
			}
			if _, seen := published[name]; seen {
				continue
			}
			schema, ok := raw.(map[string]any)
			if !ok {
				t.Fatalf("parameter %q publishes a %T schema, want an object", name, raw)
			}
			published[name] = schema
		}
	}
	return published
}

// schemaEnumValues normalises a property schema's enum field to its string
// values. The tool builder holds it as []string before marshalling and as
// []any once it has round-tripped through JSON, so both forms are read rather
// than assuming the one the in-process builder happens to produce.
func schemaEnumValues(raw any) []string {
	switch values := raw.(type) {
	case []string:
		return values
	case []any:
		out := make([]string, 0, len(values))
		for _, v := range values {
			s, _ := v.(string)
			out = append(out, s)
		}
		return out
	default:
		return nil
	}
}

// TestMimeTypeIsNotBodyContentType asserts that the attachment MIME type is
// published under its own name, and that reusing content_type for it was not
// what happened.
//
// The two concepts are genuinely different: content_type is a draft *body*
// content type, restricted by an enum to text and html, while an attachment's
// content type is a MIME type such as application/pdf. Because the flattened
// aggregate schema merges duplicate parameter names first-occurrence-wins and
// the draft verbs are registered first, a merged name would publish the
// text-or-html enum over the MIME type and tell a caller that an attachment
// must be one of two body formats. The separation is asserted here rather than
// left to a reader of the constructors.
func TestMimeTypeIsNotBodyContentType(t *testing.T) {
	published := mailPublishedSchema(t)

	contentType, ok := published["content_type"]
	if !ok {
		t.Fatal("the mail domain publishes no content_type parameter")
	}
	bodyEnum := schemaEnumValues(contentType["enum"])
	if len(bodyEnum) == 0 {
		t.Fatalf("content_type publishes no enum; the body content type must stay restricted, got %#v", contentType["enum"])
	}
	want := map[string]bool{"text": true, "html": true}
	if len(bodyEnum) != len(want) {
		t.Errorf("content_type enum is %v, want exactly the body formats text and html", bodyEnum)
	}
	for _, value := range bodyEnum {
		if !want[value] {
			t.Errorf("content_type enum admits %q; it is the draft body content type and must stay text or html", value)
		}
	}

	mimeType, ok := published["mime_type"]
	if !ok {
		t.Fatal("the mail domain publishes no mime_type parameter; add_attachment must not reuse content_type for the attachment MIME type")
	}
	if _, restricted := mimeType["enum"]; restricted {
		t.Errorf("mime_type publishes an enum %#v; an attachment MIME type is a free string and must not be restricted to the body formats", mimeType["enum"])
	}
	if kind, _ := mimeType["type"].(string); kind != "string" {
		t.Errorf("mime_type publishes type %q, want \"string\"", kind)
	}
}

// TestMailManagementVerbsCarryDotIdentity asserts that the audit
// record emitted for each received-message write verb carries the same
// mail.<verb> identity that is passed to the middleware chain.
//
// The identity is what surfaces in the audit log, in the OpenTelemetry
// attributes, and in the read-only refusal, so a verb registered under a
// mismatched string would be invisible in exactly the record an operator
// consults after the fact. The handlers are invoked with no Graph client bound,
// so each fails at account resolution and issues no network call; AuditWrap
// still emits the record, which is the surface under test.
func TestMailManagementVerbsCarryDotIdentity(t *testing.T) {
	m, err := observability.InitMetrics(noop.NewMeterProvider().Meter("test"))
	if err != nil {
		t.Fatalf("InitMetrics() error: %v", err)
	}

	logPath := filepath.Join(t.TempDir(), "audit.log")
	audit.InitAuditLog(true, logPath)
	// Every other test in this package runs with auditing off; restore that.
	defer audit.InitAuditLog(false, "")

	identity := func(h mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc { return h }
	verbs, _ := buildMailVerbs(mailVerbsConfig{
		cfg:               maximalMailConfig(),
		m:                 m,
		tracer:            tracenoop.NewTracerProvider().Tracer("test"),
		authMW:            identity,
		accountResolverMW: identity,
	})

	byName := make(map[string]tools.Verb, len(verbs))
	for _, v := range verbs {
		byName[v.Name] = v
	}

	want := []string{"move_message", "set_flag", "set_categories", "mark_read"}
	for _, name := range want {
		v, ok := byName[name]
		if !ok {
			t.Fatalf("verb %q is not registered under the maximal configuration", name)
		}
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{"message_id": "msg-1"}
		if _, err := v.Handler(context.Background(), req); err != nil {
			t.Fatalf("%s handler returned a transport error: %v", name, err)
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
		if strings.HasPrefix(entry.ToolName, "mail.") && entry.OperationType != "write" {
			t.Errorf("audit record for %s carries operation_type %q, want \"write\"", entry.ToolName, entry.OperationType)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("reading the audit log: %v", err)
	}

	for _, name := range want {
		if !seen["mail."+name] {
			t.Errorf("no audit record carries the identity %q; the middleware chain was wrapped under a different string", "mail."+name)
		}
	}
}
