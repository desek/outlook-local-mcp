// Package help — this file extracts parameter metadata from a Verb's Schema
// (CR-0060 follow-up: Option 1 of the parameter-name-discoverability fix).
//
// The Verb struct stores its parameter definitions as []mcp.ToolOption — opaque
// function values that mutate a *mcp.Tool when applied. To surface parameter
// names, types, and required-ness through the help verb, we instantiate a
// throwaway mcp.Tool, apply the verb's Schema options to it, then read the
// resulting InputSchema. This relies only on the public API of mcp-go.
package help

import (
	"sort"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/desek/outlook-local-mcp/internal/tools"
)

// paramSpec is the structured description of a single verb parameter rendered
// by the help verb across all three output tiers.
type paramSpec struct {
	// Name is the parameter key as accepted by the verb handler.
	Name string `json:"name"`

	// Type is the JSON schema type ("string", "number", "boolean", etc.).
	Type string `json:"type,omitempty"`

	// Required is true if the parameter is in the verb's required list.
	Required bool `json:"required"`

	// Description is the human-readable explanation registered with the property.
	Description string `json:"description,omitempty"`

	// Enum is the allowed value set when the property defines one.
	Enum []string `json:"enum,omitempty"`
}

// verbParameters extracts the parameter list for a verb by introspecting the
// mcp.Tool produced by applying its Schema options. Returns parameters sorted
// with required first, then alphabetical, so the most relevant fields surface
// at the top of help output.
//
// Returns an empty slice when the verb registers no parameters (e.g., help).
func verbParameters(v tools.Verb) []paramSpec {
	if len(v.Schema) == 0 {
		return nil
	}

	tool := mcp.NewTool("_introspect", v.Schema...)
	required := make(map[string]bool, len(tool.InputSchema.Required))
	for _, name := range tool.InputSchema.Required {
		required[name] = true
	}

	specs := make([]paramSpec, 0, len(tool.InputSchema.Properties))
	for name, raw := range tool.InputSchema.Properties {
		schema, _ := raw.(map[string]any)
		specs = append(specs, paramSpec{
			Name:        name,
			Type:        stringField(schema, "type"),
			Required:    required[name],
			Description: stringField(schema, "description"),
			Enum:        stringSliceField(schema, "enum"),
		})
	}

	sort.Slice(specs, func(i, j int) bool {
		if specs[i].Required != specs[j].Required {
			return specs[i].Required
		}
		return specs[i].Name < specs[j].Name
	})
	return specs
}

// stringField returns the string value at key, or "" when missing or not a string.
func stringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

// stringSliceField returns the []string value at key, accepting either []string
// or []any of strings (mcp-go uses []any for enum values). Returns nil when
// missing or empty.
func stringSliceField(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}
	switch v := m[key].(type) {
	case []string:
		if len(v) == 0 {
			return nil
		}
		out := make([]string, len(v))
		copy(out, v)
		return out
	case []any:
		if len(v) == 0 {
			return nil
		}
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
	return nil
}
