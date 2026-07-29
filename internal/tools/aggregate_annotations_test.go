// Package tools unit tests for the conservative annotation fold (CR-0068).
//
// These tests exercise AggregateAnnotations in isolation over synthetic verb
// sets, verifying each fold rule (FR-3..FR-6) and the empty-set default. They
// materialise the returned mcp.ToolOption slice back into a ToolAnnotation
// struct via the same apply-to-a-Tool technique the helper uses internally.
package tools

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// classifiedVerb constructs a Verb whose Annotations declare the four hint
// classifications, so the accessors can materialise them. Only the classification
// matters for these tests, so Name and Handler are left minimal.
func classifiedVerb(name string, readOnly, destructive, idempotent, openWorld bool) Verb {
	return Verb{
		Name: name,
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(readOnly),
			mcp.WithDestructiveHintAnnotation(destructive),
			mcp.WithIdempotentHintAnnotation(idempotent),
			mcp.WithOpenWorldHintAnnotation(openWorld),
		},
	}
}

// foldResult applies the ToolOption slice returned by AggregateAnnotations to a
// throwaway Tool and returns the resulting ToolAnnotation struct for assertion.
func foldResult(t *testing.T, opts []mcp.ToolOption) mcp.ToolAnnotation {
	t.Helper()
	var tool mcp.Tool
	for _, opt := range opts {
		opt(&tool)
	}
	return tool.Annotations
}

// requireBool fails the test unless the pointer hint is non-nil and equals want.
func requireBool(t *testing.T, field string, got *bool, want bool) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s: expected %v, got nil pointer", field, want)
	}
	if *got != want {
		t.Fatalf("%s: expected %v, got %v", field, want, *got)
	}
}

// TestAggregateAnnotationsAllReadOnly verifies that a fold over read-only,
// idempotent, non-destructive verbs yields readOnlyHint true and
// destructiveHint false (FR-3, FR-4).
func TestAggregateAnnotationsAllReadOnly(t *testing.T) {
	verbs := []Verb{
		classifiedVerb("a", true, false, true, true),
		classifiedVerb("b", true, false, true, true),
		classifiedVerb("c", true, false, true, true),
	}
	got := foldResult(t, AggregateAnnotations("Test", verbs))

	requireBool(t, "readOnlyHint", got.ReadOnlyHint, true)
	requireBool(t, "destructiveHint", got.DestructiveHint, false)
	if got.Title != "Test" {
		t.Fatalf("title: expected %q, got %q", "Test", got.Title)
	}
}

// TestAggregateAnnotationsOneDestructive verifies that a single destructive verb
// dominates the aggregate: destructiveHint becomes true and readOnlyHint false
// (FR-3, FR-4, AC-5).
func TestAggregateAnnotationsOneDestructive(t *testing.T) {
	verbs := []Verb{
		classifiedVerb("a", true, false, true, true),
		classifiedVerb("b", true, false, true, true),
		classifiedVerb("delete", false, true, true, true),
	}
	got := foldResult(t, AggregateAnnotations("Test", verbs))

	requireBool(t, "destructiveHint", got.DestructiveHint, true)
	requireBool(t, "readOnlyHint", got.ReadOnlyHint, false)
}

// TestAggregateAnnotationsAllLocal verifies that when no verb calls Graph the
// aggregate reports a closed world (FR-6).
func TestAggregateAnnotationsAllLocal(t *testing.T) {
	verbs := []Verb{
		classifiedVerb("a", true, false, true, false),
		classifiedVerb("b", true, false, true, false),
		classifiedVerb("c", true, false, true, false),
	}
	got := foldResult(t, AggregateAnnotations("Test", verbs))

	requireBool(t, "openWorldHint", got.OpenWorldHint, false)
}

// TestAggregateAnnotationsOneNonIdempotent verifies that a single non-idempotent
// verb dominates the aggregate, setting idempotentHint false (FR-5, AC-11).
func TestAggregateAnnotationsOneNonIdempotent(t *testing.T) {
	verbs := []Verb{
		classifiedVerb("a", true, false, true, true),
		classifiedVerb("b", true, false, true, true),
		classifiedVerb("create", false, false, false, true),
	}
	got := foldResult(t, AggregateAnnotations("Test", verbs))

	requireBool(t, "idempotentHint", got.IdempotentHint, false)
}

// TestAggregateAnnotationsEmptyVerbSet verifies that a degenerate empty registry
// yields the deterministic, maximally cautious default without panicking.
func TestAggregateAnnotationsEmptyVerbSet(t *testing.T) {
	got := foldResult(t, AggregateAnnotations("Test", nil))

	requireBool(t, "readOnlyHint", got.ReadOnlyHint, false)
	requireBool(t, "destructiveHint", got.DestructiveHint, true)
	requireBool(t, "idempotentHint", got.IdempotentHint, false)
	requireBool(t, "openWorldHint", got.OpenWorldHint, true)
	if got.Title != "Test" {
		t.Fatalf("title: expected %q, got %q", "Test", got.Title)
	}
}
