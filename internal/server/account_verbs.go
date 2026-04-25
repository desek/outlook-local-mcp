// Package server — this file builds the account domain verb slice for the
// aggregate "account" MCP tool (CR-0060 Phase 3b).
//
// It lives in the server package rather than tools to avoid the import cycle
// that would arise from tools importing tools/help (which itself imports tools).
package server

import (
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

// accountVerbsConfig holds the dependencies required to build the account domain
// verb slice. All fields are captured at server start.
type accountVerbsConfig struct {
	// registry is the account registry, used by all account handlers.
	registry *auth.AccountRegistry

	// cfg is the full server configuration, passed to handlers that need it.
	cfg config.Config

	// m is the ToolMetrics instance for observability instrumentation.
	m *observability.ToolMetrics

	// tracer is the OTEL tracer for span creation.
	tracer trace.Tracer

	// authMW is the authentication middleware factory, applied to every account
	// verb as the outermost wrapper.
	authMW func(mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc
}

// buildAccountVerbs constructs the ordered []tools.Verb slice for the account
// domain aggregate tool and returns a pointer to an initially empty VerbRegistry.
//
// The "help" verb is always first, followed by add, remove, list, login, logout,
// and refresh. Each verb's Handler is pre-wrapped with authMW, observability, and
// audit middleware using the fully-qualified identity "account.<verb>" per
// CR-0060 FR-13 and FR-14.
//
// The returned registry pointer is empty at the time of return. The caller
// MUST call RegisterDomainTool with the returned verbs, then assign the returned
// VerbRegistry back through the pointer so that the help verb can introspect all
// registered verbs at call time.
//
// Parameters:
//   - c: accountVerbsConfig with all required dependencies.
//
// Returns:
//   - verbs: ordered Verb slice for use with RegisterDomainTool.
//   - registryPtr: pointer whose value is assigned after registration.
func buildAccountVerbs(c accountVerbsConfig) ([]tools.Verb, *tools.VerbRegistry) {
	empty := make(tools.VerbRegistry)
	registryPtr := &empty

	// wrap builds: authMW -> WithObservability -> AuditWrap -> Handler
	wrap := func(name, auditOp string, h mcpserver.ToolHandlerFunc) tools.Handler {
		return tools.Handler(c.authMW(observability.WithObservability(name, c.m, c.tracer, audit.AuditWrap(name, auditOp, h))))
	}

	addVerb := tools.Verb{
		Name:    "add",
		Summary: "add and authenticate a new Microsoft account (browser/device_code/auth_code)",
		Handler: wrap("account.add", "write", tools.HandleAddAccount(c.registry, c.cfg)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(false),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("label",
				mcp.Required(),
				mcp.Description("Unique label for the account (1-64 chars, alphanumeric/underscore/hyphen)."),
			),
			mcp.WithString("client_id",
				mcp.Description("OAuth client ID. Defaults to the server's configured client ID."),
			),
			mcp.WithString("tenant_id",
				mcp.Description("Entra ID tenant ID. Defaults to the server's configured tenant ID."),
			),
			mcp.WithString("auth_method",
				mcp.Description("Authentication method: 'browser', 'device_code', or 'auth_code'. Defaults to the server's configured method."),
			),
		},
	}

	removeVerb := tools.Verb{
		Name:    "remove",
		Summary: "remove an account from the registry and clear its tokens (irreversible)",
		Handler: wrap("account.remove", "write", tools.HandleRemoveAccount(c.registry, c.cfg.AccountsPath)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("label",
				mcp.Required(),
				mcp.Description("The label of the account to remove."),
			),
		},
	}

	listVerb := tools.Verb{
		Name:    "list",
		Summary: "list all registered accounts with label, UPN, state, and auth_method",
		Handler: wrap("account.list", "read", tools.HandleListAccounts(c.registry)),
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

	loginVerb := tools.Verb{
		Name:    "login",
		Summary: "re-authenticate a disconnected account without removing it",
		Handler: wrap("account.login", "write", tools.HandleLoginAccount(c.registry, c.cfg)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(false),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("label",
				mcp.Required(),
				mcp.Description("Label of the disconnected account to re-authenticate."),
			),
		},
	}

	logoutVerb := tools.Verb{
		Name:    "logout",
		Summary: "disconnect an account without removing it; preserves config for login",
		Handler: wrap("account.logout", "write", tools.HandleLogoutAccount(c.registry)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(false),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("label",
				mcp.Required(),
				mcp.Description("Label of the authenticated account to disconnect."),
			),
		},
	}

	refreshVerb := tools.Verb{
		Name:    "refresh",
		Summary: "force a token refresh for an authenticated account; returns new expiry",
		Handler: wrap("account.refresh", "write", tools.HandleRefreshAccount(c.registry, c.cfg)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("label",
				mcp.Required(),
				mcp.Description("Label of the authenticated account whose token should be refreshed."),
			),
		},
	}

	verbs := []tools.Verb{
		help.NewHelpVerb(registryPtr),
		addVerb,
		removeVerb,
		listVerb,
		loginVerb,
		logoutVerb,
		refreshVerb,
	}

	return verbs, registryPtr
}

// accountToolAnnotations returns the conservative aggregate MCP annotations for
// the account domain tool per CR-0060 FR-9 and AC-9.
//
// readOnlyHint is false because write verbs (add, remove, login, logout, refresh)
// are present. destructiveHint is true because remove irreversibly deletes an
// account and its token cache. idempotentHint is false because add/login/logout
// are non-idempotent. openWorldHint is true because add/login/refresh call
// Microsoft identity and Graph APIs.
//
// These values represent the most conservative annotation across all verbs in
// the account domain per FR-9.
func accountToolAnnotations() []mcp.ToolOption {
	return []mcp.ToolOption{
		mcp.WithTitleAnnotation("Account"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	}
}
