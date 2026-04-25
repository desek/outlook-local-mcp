package server

import (
	"log/slog"
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/config"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/observability"
	"github.com/desek/outlook-local-mcp/internal/tools"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel/trace"
)

// RegisterTools registers all MCP tool handlers on the given server. Each
// calendar tool handler retrieves the Graph client from the request context
// (injected by the AccountResolver middleware). Handlers are wrapped in the
// following middleware chain (outermost first):
//
//	authMW -> accountResolverMW -> WithObservability -> ReadOnlyGuard (write tools) -> AuditWrap -> Handler
//
// Account management tools (add_account, remove_account, list_accounts,
// login_account, logout_account, refresh_account) have their own chains.
// They do NOT go through AccountResolver (they manage the registry itself)
// or ReadOnlyGuard (they are not calendar operations).
//
// When cfg.AuthMethod is "auth_code", the complete_auth fallback tool is
// registered to support clients that do not have elicitation capability.
//
// Parameters:
//   - s: the MCP server instance on which tools are registered via s.AddTool.
//   - retryCfg: retry configuration for Graph API call retry logic.
//   - timeout: the maximum duration for a single Graph API request, applied via
//     context.WithTimeout before each call.
//   - m: the ToolMetrics instance for recording observability metrics.
//   - t: the OTEL tracer for creating spans per tool invocation.
//   - readOnly: when true, write/delete tool handlers are replaced with a
//     blocking guard that returns a tool error without invoking the handler.
//   - authMW: authentication middleware factory that wraps each tool handler as
//     the outermost layer to intercept auth errors and trigger re-authentication.
//   - registry: the account registry containing all authenticated accounts,
//     used by the AccountResolver middleware for per-request client resolution.
//   - cfg: the server configuration, passed to HandleAddAccount for defaults.
//   - cred: the default account's Authenticator, used by the complete_auth tool
//     to exchange authorization codes. May be nil when auth_code is not active.
//
// Side effects: registers tool handlers on the server and logs a completion
// message.
func RegisterTools(s *mcpserver.MCPServer, retryCfg graph.RetryConfig, timeout time.Duration, m *observability.ToolMetrics, t trace.Tracer, readOnly bool, authMW func(mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc, registry *auth.AccountRegistry, cfg config.Config, cred auth.Authenticator) {
	accountResolverMW := auth.AccountResolver(registry)

	// CR-0040: Build provenance property ID once at startup. Empty when
	// provenance tagging is disabled (cfg.ProvenanceTag == "").
	var provenancePropertyID string
	if cfg.ProvenanceTag != "" {
		provenancePropertyID = graph.BuildProvenancePropertyID(cfg.ProvenanceTag)
	}

	// CR-0060 Phase 3d: calendar domain aggregate tool. Replaces the 14
	// individual calendar_* tool registrations (calendar_list, calendar_list_events,
	// calendar_get_event, calendar_search_events, calendar_create_event,
	// calendar_update_event, calendar_delete_event, calendar_respond_event,
	// calendar_reschedule_event, calendar_create_meeting, calendar_update_meeting,
	// calendar_cancel_meeting, calendar_reschedule_meeting, calendar_get_free_busy)
	// with a single "calendar" tool dispatched by operation verb.
	//
	// The calRegistry pointer is captured by the help verb handler before
	// RegisterDomainTool populates it. After registration, *calRegistry is
	// updated with the populated map so that the help verb can introspect all
	// registered verbs at call time (not at construction time).
	calVerbs, calRegistry := buildCalendarVerbs(calendarVerbsConfig{
		retryCfg:             retryCfg,
		timeout:              timeout,
		defaultTimezone:      cfg.DefaultTimezone,
		provenancePropertyID: provenancePropertyID,
		m:                    m,
		tracer:               t,
		authMW:               authMW,
		accountResolverMW:    accountResolverMW,
		readOnly:             readOnly,
	})
	populatedCal := tools.RegisterDomainTool(s, tools.DomainToolConfig{
		Domain:          "calendar",
		Intro:           "Calendar operations for Microsoft Outlook via Microsoft Graph.",
		Verbs:           calVerbs,
		ToolAnnotations: calendarToolAnnotations(),
	})
	*calRegistry = populatedCal

	// CR-0060 Phase 3b: account domain aggregate tool. Replaces the individual
	// account_add, account_list, account_remove, account_login, account_logout,
	// and account_refresh registrations with a single "account" tool dispatched
	// by operation verb. Each verb's handler is pre-wrapped with authMW,
	// observability, and audit middleware inside buildAccountVerbs using the
	// fully-qualified identity "account.<verb>" per FR-13/FR-14.
	//
	// The accRegistry pointer is captured by the help verb handler before
	// RegisterDomainTool populates it. After registration, *accRegistry is
	// updated with the populated map so that the help verb can introspect all
	// registered verbs at call time (not at construction time).
	accVerbs, accRegistry := buildAccountVerbs(accountVerbsConfig{
		registry: registry,
		cfg:      cfg,
		m:        m,
		tracer:   t,
		authMW:   authMW,
	})
	populatedAcc := tools.RegisterDomainTool(s, tools.DomainToolConfig{
		Domain:          "account",
		Intro:           "Account management for Microsoft accounts connected to the Outlook MCP server.",
		Verbs:           accVerbs,
		ToolAnnotations: accountToolAnnotations(),
	})
	*accRegistry = populatedAcc

	// CR-0060 Phase 3a: system domain aggregate tool. Replaces the individual
	// status and complete_auth tool registrations with a single "system" tool
	// dispatched by operation verb. complete_auth is gated on auth_code within
	// NewSystemVerbs, preserving the pre-existing conditional behaviour.
	//
	// The sysRegistry pointer is captured by the help verb handler before
	// RegisterDomainTool populates it. After registration, *sysRegistry is
	// updated with the populated map so that the help verb can introspect all
	// registered verbs at call time (not at construction time).
	sysVerbs, sysRegistry := buildSystemVerbs(systemVerbsConfig{
		cfg:       cfg,
		registry:  registry,
		startTime: time.Now(),
		m:         m,
		tracer:    t,
		authMW:    authMW,
		cred:      cred,
	})
	populated := tools.RegisterDomainTool(s, tools.DomainToolConfig{
		Domain:          "system",
		Intro:           "System diagnostics and authentication utilities for the Outlook MCP server.",
		Verbs:           sysVerbs,
		ToolAnnotations: systemToolAnnotations(),
	})
	*sysRegistry = populated

	// CR-0060 Phase 3c: mail domain aggregate tool. Replaces the individual
	// mail_list_folders, mail_list_messages, mail_get_message, mail_search_messages,
	// mail_get_conversation, mail_get_attachment, mail_list_attachments,
	// mail_create_draft, mail_create_reply_draft, mail_create_forward_draft,
	// mail_update_draft, and mail_delete_draft registrations with a single "mail"
	// tool dispatched by operation verb. The tool is registered unconditionally
	// per FR-1; verbs are gated by feature flags inside buildMailVerbs per FR-2.
	//
	// The mailRegistry pointer is captured by the help verb handler before
	// RegisterDomainTool populates it. After registration, *mailRegistry is
	// updated with the populated map so that the help verb can introspect all
	// registered verbs at call time (not at construction time).
	mailVerbs, mailRegistry := buildMailVerbs(mailVerbsConfig{
		retryCfg:             retryCfg,
		timeout:              timeout,
		cfg:                  cfg,
		provenancePropertyID: provenancePropertyID,
		m:                    m,
		tracer:               t,
		authMW:               authMW,
		accountResolverMW:    accountResolverMW,
		readOnly:             readOnly,
	})
	populatedMail := tools.RegisterDomainTool(s, tools.DomainToolConfig{
		Domain:          "mail",
		Intro:           "Mail operations for Microsoft Outlook via Microsoft Graph.",
		Verbs:           mailVerbs,
		ToolAnnotations: mailToolAnnotations(),
	})
	*mailRegistry = populatedMail

	// Tool count: 4 aggregate domain tools (calendar, mail, account, system).
	// All verbs are dispatched within their domain tool.
	toolCount := 4

	slog.Info("tool registration complete", "tools", toolCount)
}
