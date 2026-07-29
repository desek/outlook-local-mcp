// Package server — this file exposes the built domain verb slices for metadata
// introspection by tests and tooling (CR-0068 FR-13 / AC-9).
//
// RegisterTools builds each domain's []tools.Verb and immediately hands it to
// RegisterDomainTool, which folds the per-verb classifications into a single
// aggregate annotation and discards the per-verb detail. The classification-
// presence guard needs that per-verb detail, so BuildDomainVerbSets reproduces
// the verb-building portion of RegisterTools without registering, returning the
// slices for inspection. It deliberately mirrors the four build-config literals
// in RegisterTools; keep the two in sync when a domain gains a dependency.
package server

import (
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/config"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/observability"
	"github.com/desek/outlook-local-mcp/internal/tools"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel/trace"
)

// BuildDomainVerbSets assembles every domain's verb slice under the given
// configuration WITHOUT registering the tools, so callers can inspect per-verb
// metadata (Name, Annotations) that RegisterTools does not otherwise expose.
//
// The returned map is keyed by domain name ("calendar", "account", "system",
// "mail"). The verb set for each domain reflects the same gating RegisterTools
// applies: mail read/write verbs follow cfg.MailEnabled / cfg.MailManageEnabled,
// and system's complete_auth follows cfg.AuthMethod.
//
// Parameters:
//   - cfg: the server configuration driving verb gating.
//   - retryCfg: Graph retry configuration threaded into read/write handlers.
//   - timeout: per-call Graph timeout threaded into handlers.
//   - m: the ToolMetrics instance used by the observability middleware.
//   - tracer: the OTEL tracer used by the observability middleware.
//   - authMW: the authentication middleware factory applied to verb handlers.
//   - registry: the account registry, used by account and account-resolver MW.
//
// Side effects: none. The handlers are wrapped but never invoked, so no Graph
// call or I/O occurs.
//
// Returns a map from domain name to that domain's ordered verb slice.
func BuildDomainVerbSets(
	cfg config.Config,
	retryCfg graph.RetryConfig,
	timeout time.Duration,
	m *observability.ToolMetrics,
	tracer trace.Tracer,
	authMW func(mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc,
	registry *auth.AccountRegistry,
) map[string][]tools.Verb {
	accountResolverMW := auth.AccountResolver(registry)

	var provenancePropertyID string
	if cfg.ProvenanceTag != "" {
		provenancePropertyID = graph.BuildProvenancePropertyID(cfg.ProvenanceTag)
	}

	calVerbs, _ := buildCalendarVerbs(calendarVerbsConfig{
		retryCfg:             retryCfg,
		timeout:              timeout,
		defaultTimezone:      cfg.DefaultTimezone,
		provenancePropertyID: provenancePropertyID,
		m:                    m,
		tracer:               tracer,
		authMW:               authMW,
		accountResolverMW:    accountResolverMW,
		readOnly:             false,
	})

	accVerbs, _ := buildAccountVerbs(accountVerbsConfig{
		registry: registry,
		cfg:      cfg,
		m:        m,
		tracer:   tracer,
		authMW:   authMW,
	})

	sysVerbs, _ := buildSystemVerbs(systemVerbsConfig{
		cfg:       cfg,
		registry:  registry,
		startTime: time.Now(),
		m:         m,
		tracer:    tracer,
		authMW:    authMW,
		cred:      nil,
		version:   cfg.Version,
		commit:    cfg.Commit,
		buildDate: cfg.BuildDate,
	})

	mailVerbs, _ := buildMailVerbs(mailVerbsConfig{
		retryCfg:             retryCfg,
		timeout:              timeout,
		cfg:                  cfg,
		provenancePropertyID: provenancePropertyID,
		m:                    m,
		tracer:               tracer,
		authMW:               authMW,
		accountResolverMW:    accountResolverMW,
		readOnly:             false,
	})

	return map[string][]tools.Verb{
		"calendar": calVerbs,
		"account":  accVerbs,
		"system":   sysVerbs,
		"mail":     mailVerbs,
	}
}
