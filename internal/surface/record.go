// Package surface — this file defines the record types that make up the surface
// manifest. Every field carries an explicit JSON tag so the serialized shape is
// stable and owned here rather than inherited from another package's struct
// (CR-0073 Phase 1).
//
// @agents-index: surface manifest record types (domains, verbs, config, counts).
package surface

// VerbEntry describes a single dispatchable verb within a domain, carrying only
// the facts the manifest publishes: the verb name, its one-line summary,
// whether it is read-only, and the configuration key that gates it.
type VerbEntry struct {
	// Name is the operation identifier without the domain prefix.
	Name string `json:"name"`

	// Summary is the verb's one-line human-readable description.
	Summary string `json:"summary"`

	// ReadOnly reports whether the verb declares the read-only annotation hint.
	ReadOnly bool `json:"readOnly"`

	// Gate is the full environment variable name that gates this verb, or nil
	// (serialized as JSON null) when the verb is always registered.
	Gate *string `json:"gate"`
}

// Domain describes one aggregate domain tool: its name, the ordered full verb
// list (every gate open), and the full and default-configuration verb counts.
type Domain struct {
	// Name is the domain tool name (calendar, mail, account, system).
	Name string `json:"name"`

	// Verbs is the ordered list of every verb the domain registers with all
	// gates open.
	Verbs []VerbEntry `json:"verbs"`

	// FullCount is the number of verbs registered with every gate open.
	FullCount int `json:"fullCount"`

	// DefaultCount is the number of verbs registered under the default
	// configuration (no optional gate enabled).
	DefaultCount int `json:"defaultCount"`
}

// Counts holds a full-surface and default-configuration count pair.
type Counts struct {
	// FullCount is the count with every gate open.
	FullCount int `json:"fullCount"`

	// DefaultCount is the count under the default configuration.
	DefaultCount int `json:"defaultCount"`
}

// ConfigVar describes one configuration variable in the manifest: its full
// environment variable name, its default value where one applies, and its
// one-line description.
type ConfigVar struct {
	// Name is the full OUTLOOK_MCP_ environment variable name.
	Name string `json:"name"`

	// Default is the documented default value, empty when unset or derived.
	Default string `json:"default"`

	// Description is the single-line explanation of the variable's effect.
	Description string `json:"description"`
}

// Record is the complete surface manifest: the per-domain verb inventory with
// its counts, the aggregate totals, and the configuration variable inventory.
type Record struct {
	// Domains is the ordered list of domain records.
	Domains []Domain `json:"domains"`

	// Totals is the full and default verb counts summed across all domains.
	Totals Counts `json:"totals"`

	// Config is the ordered configuration variable inventory.
	Config []ConfigVar `json:"config"`
}
