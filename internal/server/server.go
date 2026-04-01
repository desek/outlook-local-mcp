package server

import (
	"log/slog"
	"time"

	"github.com/desek/outlook-local-mcp/internal/audit"
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
// Account management tools (add_account, remove_account, list_accounts) have
// their own chains. They do NOT go through AccountResolver (they manage the
// registry itself) or ReadOnlyGuard (they are not calendar operations).
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

	// wrap builds the standard middleware chain for calendar tools:
	// authMW -> accountResolverMW -> WithObservability -> AuditWrap -> Handler
	wrap := func(name, auditOp string, handler mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc {
		return authMW(accountResolverMW(observability.WithObservability(name, m, t, audit.AuditWrap(name, auditOp, handler))))
	}

	// wrapWrite adds ReadOnlyGuard between observability and audit for write tools.
	wrapWrite := func(name, auditOp string, handler mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc {
		return authMW(accountResolverMW(observability.WithObservability(name, m, t, ReadOnlyGuard(name, readOnly, audit.AuditWrap(name, auditOp, handler)))))
	}

	// CR-0040: Build provenance property ID once at startup. Empty when
	// provenance tagging is disabled (cfg.ProvenanceTag == "").
	var provenancePropertyID string
	if cfg.ProvenanceTag != "" {
		provenancePropertyID = graph.BuildProvenancePropertyID(cfg.ProvenanceTag)
	}

	// CR-0006: Read-only calendar tools.
	s.AddTool(tools.NewListCalendarsTool(), wrap("calendar_list", "read", tools.NewHandleListCalendars(retryCfg, timeout)))
	s.AddTool(tools.NewListEventsTool(), wrap("calendar_list_events", "read", tools.NewHandleListEvents(retryCfg, timeout, cfg.DefaultTimezone, provenancePropertyID)))
	s.AddTool(tools.NewGetEventTool(), wrap("calendar_get_event", "read", tools.NewHandleGetEvent(retryCfg, timeout, provenancePropertyID)))

	// CR-0008: Create and update event tools.
	s.AddTool(tools.NewCreateEventTool(), wrapWrite("calendar_create_event", "write", tools.HandleCreateEvent(retryCfg, timeout, cfg.DefaultTimezone, provenancePropertyID)))
	s.AddTool(tools.NewUpdateEventTool(), wrapWrite("calendar_update_event", "write", tools.HandleUpdateEvent(retryCfg, timeout, cfg.DefaultTimezone)))

	// CR-0007: Search and free/busy tools.
	s.AddTool(tools.NewSearchEventsTool(provenancePropertyID != ""), wrap("calendar_search_events", "read", tools.NewHandleSearchEvents(retryCfg, timeout, cfg.DefaultTimezone, provenancePropertyID)))
	s.AddTool(tools.NewGetFreeBusyTool(), wrap("calendar_get_free_busy", "read", tools.NewHandleGetFreeBusy(retryCfg, timeout, cfg.DefaultTimezone)))

	// CR-0009: Delete and cancel event tools.
	s.AddTool(tools.NewDeleteEventTool(), wrapWrite("calendar_delete_event", "delete", tools.HandleDeleteEvent(retryCfg, timeout)))
	s.AddTool(tools.NewCancelMeetingTool(), wrapWrite("calendar_cancel_meeting", "delete", tools.HandleCancelEvent(retryCfg, timeout)))

	// CR-0042: Respond to meeting invitations (accept/tentative/decline).
	s.AddTool(tools.NewRespondEventTool(), wrapWrite("calendar_respond_event", "write", tools.HandleRespondEvent(retryCfg, timeout)))

	// CR-0042: Reschedule event (preserves duration, computes new end time).
	s.AddTool(tools.NewRescheduleEventTool(), wrapWrite("calendar_reschedule_event", "write", tools.HandleRescheduleEvent(retryCfg, timeout, cfg.DefaultTimezone)))

	// CR-0054: Meeting tools (attendee-focused variants reusing event handlers).
	s.AddTool(tools.NewCreateMeetingTool(), wrapWrite("calendar_create_meeting", "write", tools.HandleCreateEvent(retryCfg, timeout, cfg.DefaultTimezone, provenancePropertyID)))
	s.AddTool(tools.NewUpdateMeetingTool(), wrapWrite("calendar_update_meeting", "write", tools.HandleUpdateEvent(retryCfg, timeout, cfg.DefaultTimezone)))
	s.AddTool(tools.NewRescheduleMeetingTool(), wrapWrite("calendar_reschedule_meeting", "write", tools.HandleRescheduleEvent(retryCfg, timeout, cfg.DefaultTimezone)))

	// CR-0025: Account management tools. These do NOT go through
	// AccountResolver (they manage the registry) or ReadOnlyGuard (they are
	// not calendar operations). list_accounts is inherently read-only.
	s.AddTool(tools.NewAddAccountTool(), authMW(observability.WithObservability("account_add", m, t, audit.AuditWrap("account_add", "write", tools.HandleAddAccount(registry, cfg)))))
	s.AddTool(tools.NewListAccountsTool(), authMW(observability.WithObservability("account_list", m, t, audit.AuditWrap("account_list", "read", tools.HandleListAccounts(registry)))))
	s.AddTool(tools.NewRemoveAccountTool(), authMW(observability.WithObservability("account_remove", m, t, audit.AuditWrap("account_remove", "write", tools.HandleRemoveAccount(registry, cfg.AccountsPath)))))

	// CR-0037: Status diagnostic tool. No auth middleware, no account
	// resolver — purely reads in-memory state with no Graph API calls.
	s.AddTool(tools.NewStatusTool(), observability.WithObservability("status", m, t, audit.AuditWrap("status", "read", tools.HandleStatus(cfg, registry, time.Now()))))

	// CR-0043: Mail tools, registered only when mail access is enabled.
	// All mail tools are read-only and use the standard middleware chain.
	if cfg.MailEnabled {
		s.AddTool(tools.NewListMailFoldersTool(), wrap("mail_list_folders", "read", tools.NewHandleListMailFolders(retryCfg, timeout)))
		s.AddTool(tools.NewListMessagesTool(), wrap("mail_list_messages", "read", tools.NewHandleListMessages(retryCfg, timeout)))
		s.AddTool(tools.NewSearchMessagesTool(), wrap("mail_search_messages", "read", tools.NewHandleSearchMessages(retryCfg, timeout)))
		s.AddTool(tools.NewGetMessageTool(), wrap("mail_get_message", "read", tools.NewHandleGetMessage(retryCfg, timeout)))
	}

	// CR-0030: complete_auth fallback tool for auth_code method. Only
	// registered when auth_code is active, since the tool is meaningless
	// for browser or device_code flows.
	toolCount := 18
	if cfg.MailEnabled {
		toolCount += 4
	}
	if cfg.AuthMethod == "auth_code" {
		s.AddTool(tools.NewCompleteAuthTool(), authMW(observability.WithObservability("complete_auth", m, t, audit.AuditWrap("complete_auth", "write", tools.HandleCompleteAuth(cred, cfg.AuthRecordPath, registry, auth.Scopes(cfg))))))
		toolCount++
	}

	slog.Info("tool registration complete", "tools", toolCount)
}
