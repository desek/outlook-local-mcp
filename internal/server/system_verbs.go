// Package server — this file builds the system domain verb slice for the
// aggregate "system" MCP tool (CR-0060 Phase 3a).
//
// It lives in the server package rather than tools to avoid the import cycle
// that would arise from tools importing tools/help (which itself imports tools).
package server

import (
	"time"

	"github.com/desek/outlook-local-mcp/internal/audit"
	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/config"
	"github.com/desek/outlook-local-mcp/internal/observability"
	"github.com/desek/outlook-local-mcp/internal/tools"
	"github.com/desek/outlook-local-mcp/internal/tools/help"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel/trace"
)

// systemVerbsConfig holds the dependencies required to build the system domain
// verb slice. All fields are captured at server start; Cred may be nil when
// auth_code is not the active authentication method.
type systemVerbsConfig struct {
	// cfg is the full server configuration, passed to HandleStatus and
	// HandleCompleteAuth.
	cfg config.Config

	// registry is the account registry, passed to HandleStatus and
	// HandleCompleteAuth.
	registry *auth.AccountRegistry

	// startTime is the server start time, used by HandleStatus to compute uptime.
	startTime time.Time

	// m is the ToolMetrics instance for observability instrumentation.
	m *observability.ToolMetrics

	// tracer is the OTEL tracer for span creation.
	tracer trace.Tracer

	// authMW is the authentication middleware factory, applied only to the
	// complete_auth verb.
	authMW func(mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc

	// cred is the default authenticator for the complete_auth verb. May be nil
	// when auth_code is not the active auth method.
	cred auth.Authenticator
}

// buildSystemVerbs constructs the ordered []tools.Verb slice for the system
// domain aggregate tool and returns a pointer to an initially empty VerbRegistry.
//
// The "help" verb is always first. The "status" verb is always included and is
// not wrapped with authMW because it reads only in-memory state. The
// "complete_auth" verb is included and wrapped with authMW only when
// cfg.AuthMethod == "auth_code".
//
// Each verb's Handler is pre-wrapped with observability and audit middleware
// using the fully-qualified identity "system.<verb>" per CR-0060 FR-13 and
// FR-14. The observability wrapper uses the composite name so that span names
// and metric labels carry the complete operation identity.
//
// The returned registry pointer is empty at the time of return. The caller
// MUST call RegisterDomainTool with the returned verbs, then assign the
// returned VerbRegistry back through the pointer so that the help verb can
// introspect all registered verbs at call time.
//
// Parameters:
//   - c: systemVerbsConfig with all required dependencies.
//
// Returns:
//   - verbs: ordered Verb slice for use with RegisterDomainTool.
//   - registryPtr: pointer whose value is assigned after registration.
func buildSystemVerbs(c systemVerbsConfig) ([]tools.Verb, *tools.VerbRegistry) {
	empty := make(tools.VerbRegistry)
	registryPtr := &empty

	// status verb: read-only, in-memory, no authMW.
	statusHandler := observability.WithObservability(
		"system.status", c.m, c.tracer,
		audit.AuditWrap("system.status", "read", tools.HandleStatus(c.cfg, c.registry, c.startTime)),
	)
	statusVerb := tools.Verb{
		Name:    "status",
		Summary: "return server health: version, accounts, uptime, config (no Graph call)",
		Handler: tools.Handler(statusHandler),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("output",
				mcp.Description("Output mode: 'text' (default), 'summary', or 'raw'."),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}

	verbs := []tools.Verb{
		help.NewHelpVerb(registryPtr),
		statusVerb,
	}

	// complete_auth verb: conditional on auth_code; requires authMW and network.
	if c.cfg.AuthMethod == "auth_code" {
		innerHandler := audit.AuditWrap(
			"system.complete_auth", "write",
			tools.HandleCompleteAuth(c.cred, c.cfg.AuthRecordPath, c.registry, auth.Scopes(c.cfg)),
		)
		obsHandler := observability.WithObservability("system.complete_auth", c.m, c.tracer, innerHandler)
		authedHandler := c.authMW(obsHandler)

		completeAuthVerb := tools.Verb{
			Name:    "complete_auth",
			Summary: "exchange browser redirect URL for tokens to finish auth_code flow",
			Handler: tools.Handler(authedHandler),
			Annotations: []mcp.ToolOption{
				mcp.WithReadOnlyHintAnnotation(false),
				mcp.WithDestructiveHintAnnotation(false),
				mcp.WithIdempotentHintAnnotation(false),
				mcp.WithOpenWorldHintAnnotation(true),
			},
			Schema: []mcp.ToolOption{
				mcp.WithString("redirect_url",
					mcp.Required(),
					mcp.Description("The full URL from the browser's address bar after signing in."),
				),
				mcp.WithString("account",
					mcp.Description("Account label or UPN that was provided to account_add when initiating auth_code authentication."),
				),
			},
		}
		verbs = append(verbs, completeAuthVerb)
	}

	return verbs, registryPtr
}

// systemToolAnnotations returns the conservative aggregate MCP annotations for
// the system domain tool per CR-0060 FR-9 and AC-9.
//
// readOnlyHint is false because the domain may host the write complete_auth
// verb (when auth_code is active). destructiveHint is false because no verb
// irreversibly deletes data. idempotentHint is false because complete_auth is
// non-idempotent. openWorldHint is true because complete_auth calls Microsoft
// Graph.
//
// These values represent the most conservative annotation across all verbs that
// may be registered for the domain. Even when complete_auth is absent (no
// auth_code), the manifest-level annotation is fixed at construction time and
// must remain consistent across deployment configurations.
func systemToolAnnotations() []mcp.ToolOption {
	return []mcp.ToolOption{
		mcp.WithTitleAnnotation("System"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	}
}
