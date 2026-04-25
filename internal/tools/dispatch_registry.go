// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file defines the Verb registry type used by the Phase 1 dispatcher
// scaffolding (CR-0060). A Verb represents a single operation within a domain
// aggregate tool: it carries the operation name, a one-line summary, the
// handler function, the MCP tool option annotations, and the JSON Schema
// properties for operation-specific parameters.
//
// CR-0065 extends Verb with Description, Examples, and SeeDocs so that the
// registry is the single source of truth for per-verb reference content
// rendered by system.help (and per-domain help). Every verb MUST have a
// non-empty Description.
package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// Handler is the function signature for an MCP tool handler. It matches the
// ToolHandlerFunc type used by the mcp-go server, accepting a context and a
// CallToolRequest and returning a result or an error.
type Handler func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)

// Example represents a single illustrative invocation of a verb. It captures
// the argument map the caller would pass and an optional human-readable comment
// that explains what the example demonstrates.
//
// Args holds the key/value pairs that form the operation's arguments (excluding
// the "operation" key itself, which is implicit). Values may be any JSON-
// compatible type: string, bool, number, or a nested structure.
//
// Comment is a brief sentence describing what this invocation demonstrates.
// It is displayed in text and summary help output. An empty Comment is valid.
type Example struct {
	// Args is the argument map for this example invocation, excluding "operation".
	Args map[string]any

	// Comment describes what this example demonstrates.
	Comment string
}

// Verb describes a single dispatchable operation within a domain aggregate
// tool. Each Verb is one entry in the operation enum exposed to MCP clients.
//
// Name is the operation identifier (e.g., "list_events"). It MUST be unique
// within a domain's verb registry and MUST NOT include the domain prefix.
//
// Summary is a concise, ≤80-character description of the operation. It is
// appended to the domain tool's top-level description and returned verbatim
// when help output is produced.
//
// Description is a full prose explanation of the verb's purpose, behaviour,
// and important caveats. It MUST be non-empty for every registered verb
// (enforced by TestEveryVerbHasDescription). Unlike Summary, it may span
// multiple sentences and reference related verbs, output modes, or SeeDocs
// anchors.
//
// Examples is an ordered slice of illustrative invocations. Each entry
// contains an argument map and an optional comment. Examples are rendered in
// all three help output tiers (text, summary, raw). The slice may be empty.
//
// SeeDocs is an ordered slice of documentation references. Each entry is
// either a bare slug (e.g., "concepts") or a slug with an anchor
// (e.g., "concepts#output-tiers"). Every entry MUST resolve to a slug in
// docs.Bundle, and any anchor MUST match an H2 heading in that file
// (enforced by TestSeeDocsAnchorsResolve). The slice may be empty.
//
// Handler is the underlying MCP handler function that executes the operation.
// The dispatcher calls this function after routing the incoming request.
//
// Annotations is a slice of mcp.ToolOption values that carry the five
// required MCP annotations (title, readOnly, destructive, idempotent,
// openWorld) for this verb. These are recorded in the Verb registry so that
// the dispatcher can compute conservative aggregate annotations across all
// verbs in a domain.
//
// Schema is a slice of mcp.ToolOption values that define the JSON Schema
// properties for this verb's operation-specific parameters (e.g.,
// mcp.WithString, mcp.Required). The dispatcher validates unknown parameter
// names against this list before invoking the handler.
type Verb struct {
	// Name is the operation identifier without the domain prefix.
	Name string

	// Summary is a concise (≤80 char) human-readable description of the verb.
	Summary string

	// Description is the full prose explanation of the verb's purpose and
	// behaviour. MUST be non-empty for every registered verb (CR-0065 FR-9).
	Description string

	// Examples holds zero or more illustrative invocations for this verb.
	Examples []Example

	// SeeDocs holds zero or more documentation references in the form
	// "slug" or "slug#anchor" pointing into docs.Bundle (CR-0065 FR-11).
	SeeDocs []string

	// Handler is the MCP handler function invoked when this verb is dispatched.
	Handler Handler

	// Annotations holds the five MCP annotation ToolOption values for this
	// verb, used when computing conservative aggregate annotations.
	Annotations []mcp.ToolOption

	// Schema holds the parameter schema ToolOption values for this verb's
	// operation-specific inputs, used for unknown-parameter validation.
	Schema []mcp.ToolOption

	// middleware is the optional middleware chain applied to Handler. If nil,
	// the raw Handler is called directly. Set by RegisterDomainTool when a
	// middleware factory is provided.
	middleware func(mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc
}

// wrappedHandler returns the handler wrapped in the verb's middleware chain.
// If no middleware is configured, the raw Handler is returned unchanged.
func (v Verb) wrappedHandler() mcpserver.ToolHandlerFunc {
	h := mcpserver.ToolHandlerFunc(v.Handler)
	if v.middleware != nil {
		return v.middleware(h)
	}
	return h
}

// VerbRegistry maps operation names to their Verb descriptors for a single
// domain aggregate tool. It is populated by RegisterDomainTool and read by
// the dispatcher on every invocation.
type VerbRegistry map[string]Verb
