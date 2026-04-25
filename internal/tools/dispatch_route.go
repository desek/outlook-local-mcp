// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file implements RegisterDomainTool, the single entry point for
// registering a domain aggregate tool with the MCP server (CR-0060 Phase 1).
// It also contains the dispatch handler that routes incoming requests to the
// appropriate Verb handler based on the required `operation` parameter.
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// DomainToolConfig holds configuration for a single domain aggregate tool
// registration. It is the input to RegisterDomainTool.
//
// Domain is the domain name, used as both the MCP tool name and the prefix
// in the fully qualified identity "{domain}.{operation}" logged by audit.
//
// Intro is a one-sentence domain description prepended to the verb list in
// the top-level tool description (see AC-4 / FR-3).
//
// Verbs is the ordered list of Verb descriptors for this domain. The order
// determines the operation enum order exposed to MCP clients. "help" MUST
// be the first entry when it is supported.
//
// ToolAnnotations is the slice of mcp.ToolOption values providing the five
// required MCP annotations (title, readOnly, destructive, idempotent,
// openWorld) for the aggregate tool as a whole, computed conservatively
// across all verbs per AC-9 / FR-9.
//
// Middleware is an optional middleware factory applied to every verb handler.
// When non-nil, each verb's Handler is wrapped via middleware before being
// called. The factory receives the raw handler and returns the wrapped handler.
// Pass nil to call verb handlers directly (useful in tests).
type DomainToolConfig struct {
	// Domain is the MCP tool name (e.g., "calendar", "mail").
	Domain string

	// Intro is the one-sentence domain description used in the top-level
	// tool description.
	Intro string

	// Verbs is the ordered list of operations for this domain, including "help".
	Verbs []Verb

	// ToolAnnotations provides the five required MCP annotation ToolOptions for
	// the aggregate tool, computed conservatively across all verbs.
	ToolAnnotations []mcp.ToolOption

	// Middleware is an optional middleware factory applied to every verb handler.
	Middleware func(mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc
}

// RegisterDomainTool registers a single domain aggregate MCP tool on the given
// server. It performs three actions:
//
//  1. Builds the `operation` enum from the registered verb names.
//  2. Composes the top-level description listing every verb with its summary.
//  3. Registers one mcp.Tool on the server whose handler routes calls to the
//     correct Verb.Handler based on the `operation` argument.
//
// The returned VerbRegistry may be used by callers (e.g., help renderers,
// tests) to inspect the registered verbs after registration.
//
// Per FR-11, an unknown `operation` value returns a structured error listing
// valid verbs and suggesting `operation="help"`. Per FR-12, unknown parameters
// for a known verb are rejected with an error naming the parameter and verb.
//
// Parameters:
//   - s: the MCP server instance on which the tool is registered.
//   - cfg: configuration for the domain tool, including domain name, intro,
//     verbs, aggregate annotations, and an optional middleware factory.
//
// Returns the populated VerbRegistry for the domain.
//
// Side effects: registers one MCP tool on s and applies cfg.Middleware (if
// non-nil) to each verb handler.
func RegisterDomainTool(s *mcpserver.MCPServer, cfg DomainToolConfig) VerbRegistry {
	registry := make(VerbRegistry, len(cfg.Verbs))

	enumNames := buildOperationEnum(cfg.Verbs)
	description := buildTopLevelDescription(cfg.Intro, cfg.Verbs)

	for _, v := range cfg.Verbs {
		if cfg.Middleware != nil {
			v.middleware = cfg.Middleware
		}
		registry[v.Name] = v
	}

	// Compose tool options: description, all aggregate annotations, and the
	// required `operation` string enum.
	toolOpts := make([]mcp.ToolOption, 0, 2+len(cfg.ToolAnnotations))
	toolOpts = append(toolOpts, mcp.WithDescription(description))
	toolOpts = append(toolOpts, cfg.ToolAnnotations...)
	toolOpts = append(toolOpts,
		mcp.WithString("operation",
			mcp.Description("The operation to perform. Call with operation=\"help\" for full documentation."),
			mcp.Required(),
			mcp.Enum(enumNames...),
		),
	)

	tool := mcp.NewTool(cfg.Domain, toolOpts...)

	handler := buildDispatchHandler(cfg.Domain, registry)
	s.AddTool(tool, handler)

	return registry
}

// buildDispatchHandler returns a ToolHandlerFunc that routes an incoming MCP
// call to the appropriate Verb handler based on the `operation` argument.
//
// Per FR-11, an unrecognised operation value causes a structured error
// listing valid verbs and pointing to operation="help".
//
// Parameters:
//   - domain: the domain name, used in error messages.
//   - registry: the verb registry for this domain.
//
// Returns a ToolHandlerFunc suitable for s.AddTool.
func buildDispatchHandler(domain string, registry VerbRegistry) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		op := req.GetString("operation", "")
		if op == "" {
			return mcp.NewToolResultError(
				fmt.Sprintf("missing required parameter 'operation' for tool '%s'. "+
					"Call with operation=\"help\" to see available operations.", domain),
			), nil
		}

		verb, ok := registry[op]
		if !ok {
			validOps := collectValidOps(registry)
			return mcp.NewToolResultError(
				fmt.Sprintf("unknown operation %q for tool '%s'. "+
					"Valid operations: %s. "+
					"Call with operation=\"help\" for full documentation.",
					op, domain, strings.Join(validOps, ", ")),
			), nil
		}

		return verb.wrappedHandler()(ctx, req)
	}
}

// collectValidOps returns a sorted list of operation names from the registry.
// The list always begins with "help" (if present) followed by the remaining
// names in insertion-stable order (map iteration is not ordered, so we derive
// order from a deterministic sort).
func collectValidOps(registry VerbRegistry) []string {
	names := make([]string, 0, len(registry))
	// Emit "help" first if registered, for consistency with FR-11 error UX.
	if _, ok := registry["help"]; ok {
		names = append(names, "help")
	}
	for name := range registry {
		if name != "help" {
			names = append(names, name)
		}
	}
	return names
}
