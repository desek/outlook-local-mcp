// Package server — this file exposes a zero-dependency entry point for building
// the four domain verb slices purely for surface-manifest inspection (CR-0073
// Phase 1).
//
// The surface generator needs the built verb sets under several configurations
// (all gates open, default configuration) to derive counts and per-verb gates.
// It must do so in continuous integration and offline, so this entry point
// passes no-op middleware and zero-value dependencies: no account registry, no
// metrics, no tracer, no credentials. Building a verb slice only constructs
// descriptors and wraps handlers; the handlers are never invoked, so no Graph
// call or other I/O occurs.
//
// @agents-index: zero-dependency verb-slice builder for surface inspection.
package server

import (
	"github.com/desek/outlook-local-mcp/internal/config"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/tools"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// BuildVerbsForInspection builds every domain's verb slice under cfg using
// no-op middleware and zero-value dependencies, returning them keyed by domain
// name ("calendar", "account", "system", "mail").
//
// It is a thin wrapper over BuildDomainVerbSets that supplies an identity
// middleware factory and nil registry, metrics, and tracer, and zero retry and
// timeout values. The verb gating still follows cfg (mail read/write verbs
// follow cfg.MailEnabled / cfg.MailManageEnabled, and system's complete_auth
// follows cfg.AuthMethod), so callers can build both the full and the default
// surface by varying cfg alone.
//
// Parameters:
//   - cfg: the server configuration driving verb gating.
//
// Side effects: none. No credentials are read and no network call is made,
// because the wrapped handlers are constructed but never invoked.
//
// Returns a map from domain name to that domain's ordered verb slice.
func BuildVerbsForInspection(cfg config.Config) map[string][]tools.Verb {
	noop := func(h mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc { return h }
	return BuildDomainVerbSets(cfg, graph.RetryConfig{}, 0, nil, nil, noop, nil)
}
