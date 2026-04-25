// Package tools — this file builds the union of per-verb parameter schemas for
// a domain aggregate tool (CR-0060 follow-up: parameter-passthrough fix).
//
// MCP clients only forward call arguments declared in the tool's InputSchema.
// Without this union the aggregate tool exposes only `operation`, so per-verb
// parameters such as `label` or `message_id` are stripped before reaching the
// dispatcher. Per-verb required-ness is intentionally dropped here because each
// parameter is required only for some verbs; handlers continue to validate
// required arguments at call time.
package tools

import (
	"sort"

	"github.com/mark3labs/mcp-go/mcp"
)

// aggregateSchemaOptions returns the union of all per-verb parameter
// definitions as optional ToolOption values, suitable for appending to the
// aggregate domain tool's mcp.NewTool call. Duplicate parameter names across
// verbs are merged: the first occurrence wins for type, description, and enum.
//
// The returned options preserve property name, type, description, and enum
// (when present) but never set Required, since required-ness is verb-specific.
func aggregateSchemaOptions(verbs []Verb) []mcp.ToolOption {
	type prop struct {
		name        string
		typ         string
		description string
		enum        []string
	}

	seen := make(map[string]prop)
	for _, v := range verbs {
		if len(v.Schema) == 0 {
			continue
		}
		t := mcp.NewTool("_introspect", v.Schema...)
		for name, raw := range t.InputSchema.Properties {
			if name == "operation" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			schema, _ := raw.(map[string]any)
			p := prop{name: name}
			if schema != nil {
				if s, ok := schema["type"].(string); ok {
					p.typ = s
				}
				if s, ok := schema["description"].(string); ok {
					p.description = s
				}
				switch e := schema["enum"].(type) {
				case []string:
					p.enum = append(p.enum, e...)
				case []any:
					for _, item := range e {
						if s, ok := item.(string); ok {
							p.enum = append(p.enum, s)
						}
					}
				}
			}
			seen[name] = p
		}
	}

	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)

	out := make([]mcp.ToolOption, 0, len(names))
	for _, n := range names {
		p := seen[n]
		propOpts := make([]mcp.PropertyOption, 0, 2)
		if p.description != "" {
			propOpts = append(propOpts, mcp.Description(p.description))
		}
		if len(p.enum) > 0 {
			propOpts = append(propOpts, mcp.Enum(p.enum...))
		}
		out = append(out, buildProperty(p.typ, p.name, propOpts))
	}
	return out
}

// buildProperty returns the appropriate mcp.With* ToolOption for the given
// JSON Schema type, falling back to WithAny when the type is unknown or empty.
func buildProperty(typ, name string, opts []mcp.PropertyOption) mcp.ToolOption {
	switch typ {
	case "string":
		return mcp.WithString(name, opts...)
	case "number", "integer":
		return mcp.WithNumber(name, opts...)
	case "boolean":
		return mcp.WithBoolean(name, opts...)
	case "array":
		return mcp.WithArray(name, opts...)
	case "object":
		return mcp.WithObject(name, opts...)
	default:
		return mcp.WithAny(name, opts...)
	}
}
