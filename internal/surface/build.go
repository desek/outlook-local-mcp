// Package surface — this file builds the surface Record from the live verb
// builders and the configuration inventory (CR-0073 Phase 1).
//
// Counts and gates are derived, never stated as literals: the full and default
// verb sets are built by varying only the configuration passed to the server's
// inspection entry point, and each gated verb's gate is attributed by observing
// which single configuration toggle first introduces it. The domain order and
// every collection order are fixed so serialization is deterministic.
//
// @agents-index: constructs the surface Record by inspecting built verb sets.
package surface

import (
	"github.com/desek/outlook-local-mcp/internal/config"
	"github.com/desek/outlook-local-mcp/internal/server"
	"github.com/desek/outlook-local-mcp/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
)

// domainOrder is the fixed order domains appear in the manifest. It matches the
// tool-naming convention list and is independent of map iteration order.
var domainOrder = []string{"calendar", "mail", "account", "system"}

// fullConfig returns the configuration in which every optional gate is open, so
// that BuildVerbsForInspection yields the complete verb surface.
func fullConfig() config.Config {
	return config.Config{
		MailEnabled:       true,
		MailManageEnabled: true,
		AuthMethod:        "auth_code",
	}
}

// defaultConfig returns the out-of-box configuration: no optional mail gate
// enabled and the default device_code auth method, so complete_auth is absent.
func defaultConfig() config.Config {
	return config.Config{
		MailEnabled:       false,
		MailManageEnabled: false,
		AuthMethod:        "device_code",
	}
}

// gateProbe pairs a configuration key with the configuration that enables only
// that gate on top of the default, so the verbs it introduces can be attributed
// to it.
type gateProbe struct {
	// key is the environment variable name recorded as the verb's gate.
	key string

	// cfg is the default configuration with exactly this gate enabled.
	cfg config.Config
}

// gateProbes returns the ordered set of single-gate probes. Each probe enables
// exactly one optional gate on top of the default configuration, so the verbs
// that appear beyond the default set are attributed to that probe's key.
func gateProbes() []gateProbe {
	mailRead := defaultConfig()
	mailRead.MailEnabled = true

	mailManage := defaultConfig()
	mailManage.MailManageEnabled = true

	authCode := defaultConfig()
	authCode.AuthMethod = "auth_code"

	return []gateProbe{
		{config.EnvMailEnabled, mailRead},
		{config.EnvMailManageEnabled, mailManage},
		{config.EnvAuthMethod, authCode},
	}
}

// verbIsReadOnly reports whether the verb declares readOnlyHint: true. It
// materializes the verb's opaque annotation options against a zero-value
// mcp.Tool and reads the resulting hint, treating an undeclared hint as false.
//
// This mirrors the accessor in the tools package rather than importing an
// unexported helper; the small duplication keeps the surface package decoupled.
func verbIsReadOnly(v tools.Verb) bool {
	var t mcp.Tool
	for _, opt := range v.Annotations {
		opt(&t)
	}
	return t.Annotations.ReadOnlyHint != nil && *t.Annotations.ReadOnlyHint
}

// nameSet returns the set of verb names in the slice.
func nameSet(verbs []tools.Verb) map[string]bool {
	s := make(map[string]bool, len(verbs))
	for _, v := range verbs {
		s[v.Name] = true
	}
	return s
}

// BuildRecord constructs the complete surface Record by inspecting the built
// verb sets under the full and default configurations and attributing each
// gated verb to its gate via single-gate probes. It performs no network call
// and reads no credential.
//
// Returns the populated Record. The Record is deterministic: domains and verbs
// follow fixed orders and no value is read from the environment.
func BuildRecord() Record {
	full := server.BuildVerbsForInspection(fullConfig())
	def := server.BuildVerbsForInspection(defaultConfig())

	// Attribute each gated verb (per domain) to the probe that introduces it.
	gateOf := make(map[string]map[string]string) // domain -> verb -> gate key
	for _, d := range domainOrder {
		gateOf[d] = make(map[string]string)
	}
	for _, probe := range gateProbes() {
		probeSets := server.BuildVerbsForInspection(probe.cfg)
		for _, d := range domainOrder {
			defNames := nameSet(def[d])
			for _, v := range probeSets[d] {
				if !defNames[v.Name] {
					gateOf[d][v.Name] = probe.key
				}
			}
		}
	}

	rec := Record{
		Config: buildConfigInventory(),
	}

	for _, d := range domainOrder {
		fullVerbs := full[d]
		defNames := nameSet(def[d])

		entries := make([]VerbEntry, 0, len(fullVerbs))
		for _, v := range fullVerbs {
			var gate *string
			if !defNames[v.Name] {
				if key, ok := gateOf[d][v.Name]; ok {
					k := key
					gate = &k
				}
			}
			entries = append(entries, VerbEntry{
				Name:     v.Name,
				Summary:  v.Summary,
				ReadOnly: verbIsReadOnly(v),
				Gate:     gate,
			})
		}

		domain := Domain{
			Name:         d,
			Verbs:        entries,
			FullCount:    len(fullVerbs),
			DefaultCount: len(def[d]),
		}
		rec.Domains = append(rec.Domains, domain)
		rec.Totals.FullCount += domain.FullCount
		rec.Totals.DefaultCount += domain.DefaultCount
	}

	return rec
}

// buildConfigInventory maps the config package inventory into the manifest's
// ConfigVar shape, preserving the inventory's fixed order.
func buildConfigInventory() []ConfigVar {
	src := config.Inventory()
	out := make([]ConfigVar, 0, len(src))
	for _, v := range src {
		out = append(out, ConfigVar{
			Name:        v.Name,
			Default:     v.Default,
			Description: v.Description,
		})
	}
	return out
}
