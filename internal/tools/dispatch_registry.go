// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file defines the Verb registry type used by the Phase 1 dispatcher
// scaffolding (CR-0060). A Verb represents a single operation within a domain
// aggregate tool: it carries the operation name, a one-line summary, the
// handler function, the MCP tool option annotations, and the JSON Schema
// properties for operation-specific parameters.
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
