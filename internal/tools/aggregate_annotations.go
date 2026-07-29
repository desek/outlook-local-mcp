// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file implements the shared conservative-fold helper that derives an
// aggregate domain tool's five MCP annotations from the set of verbs actually
// registered for that domain (CR-0068). The four domain tools (calendar, mail,
// account, system) publish a single tool-granularity annotation each, yet host
// many verbs whose individual read-only, destructive, idempotent and open-world
// classifications differ. Publishing a static annotation is wrong in at least
// one supported gated configuration, so the aggregate is computed here as a pure
// function of the registered verb set.
//
// The per-verb classification lives in each Verb's Annotations field, which is a
// slice of opaque mcp.ToolOption closures. The accessors below materialise those
// closures by applying them to a throwaway mcp.Tool and reading the resulting
// ToolAnnotation struct, rather than attempting to inspect the closures directly.
package tools

import "github.com/mark3labs/mcp-go/mcp"

// materializeAnnotations applies a verb's Annotations options to a zero-value
// mcp.Tool and returns the resulting ToolAnnotation struct.
//
// A zero-value mcp.Tool is used deliberately (rather than mcp.NewTool, which
// pre-seeds the hint pointers with cautious defaults) so that only hints the
// verb explicitly declared are non-nil. A nil hint pointer therefore means the
// verb did not declare that hint, and the accessors treat an undeclared hint as
// its safe (non-asserting) value.
//
// verb is the registry entry whose classification is being read. The function
// has no side effects on the registry and performs no I/O.
func materializeAnnotations(verb Verb) mcp.ToolAnnotation {
	var t mcp.Tool
	for _, opt := range verb.Annotations {
		opt(&t)
	}
	return t.Annotations
}

// verbIsReadOnly reports whether the verb declares readOnlyHint: true.
//
// An undeclared hint yields false, meaning the verb is not treated as read-only.
func verbIsReadOnly(verb Verb) bool {
	a := materializeAnnotations(verb)
	return a.ReadOnlyHint != nil && *a.ReadOnlyHint
}

// verbIsDestructive reports whether the verb declares destructiveHint: true.
//
// An undeclared hint yields false, meaning the verb is not treated as destructive.
func verbIsDestructive(verb Verb) bool {
	a := materializeAnnotations(verb)
	return a.DestructiveHint != nil && *a.DestructiveHint
}

// verbIsIdempotent reports whether the verb declares idempotentHint: true.
//
// An undeclared hint yields false, meaning the verb is treated as
// non-idempotent, the conservative value.
func verbIsIdempotent(verb Verb) bool {
	a := materializeAnnotations(verb)
	return a.IdempotentHint != nil && *a.IdempotentHint
}

// verbCallsGraph reports whether the verb declares openWorldHint: true, i.e.
// whether it interacts with Microsoft Graph.
//
// An undeclared hint yields false, meaning the verb is treated as local.
func verbCallsGraph(verb Verb) bool {
	a := materializeAnnotations(verb)
	return a.OpenWorldHint != nil && *a.OpenWorldHint
}

// AggregateAnnotations computes the conservative aggregate MCP annotations for a
// domain tool from the set of verbs registered for that domain, returning them
// as the mcp.ToolOption slice mcp.NewTool expects.
//
// The fold is conservative as defined in AGENTS.md and CR-0068 (FR-3..FR-6):
//   - readOnlyHint is true only when EVERY registered verb is read-only.
//   - destructiveHint is true when AT LEAST ONE registered verb is destructive.
//   - idempotentHint is false when AT LEAST ONE registered verb is non-idempotent.
//   - openWorldHint is true when AT LEAST ONE registered verb calls Graph.
//
// title is the human-readable tool title carried through unchanged. verbs is the
// registered verb set; an empty slice yields the deterministic, maximally
// cautious default (readOnly false, destructive true, idempotent false,
// openWorld true) and never panics, so a degenerate registry cannot produce a
// misleadingly permissive annotation.
//
// The function is pure: it performs no I/O and does not mutate its inputs, so
// tool registration remains synchronous and side-effect free (NFR-2).
func AggregateAnnotations(title string, verbs []Verb) []mcp.ToolOption {
	// Empty registry: emit the maximally cautious default rather than folding
	// over nothing, which would otherwise yield the least cautious values.
	if len(verbs) == 0 {
		return []mcp.ToolOption{
			mcp.WithTitleAnnotation(title),
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(false),
			mcp.WithOpenWorldHintAnnotation(true),
		}
	}

	readOnly := true     // true only if all verbs are read-only
	destructive := false // true if any verb is destructive
	idempotent := true   // false if any verb is non-idempotent
	openWorld := false   // true if any verb calls Graph

	for _, v := range verbs {
		if !verbIsReadOnly(v) {
			readOnly = false
		}
		if verbIsDestructive(v) {
			destructive = true
		}
		if !verbIsIdempotent(v) {
			idempotent = false
		}
		if verbCallsGraph(v) {
			openWorld = true
		}
	}

	return []mcp.ToolOption{
		mcp.WithTitleAnnotation(title),
		mcp.WithReadOnlyHintAnnotation(readOnly),
		mcp.WithDestructiveHintAnnotation(destructive),
		mcp.WithIdempotentHintAnnotation(idempotent),
		mcp.WithOpenWorldHintAnnotation(openWorld),
	}
}
