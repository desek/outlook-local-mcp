// Package server — this file builds the mail domain verb slice for the
// aggregate "mail" MCP tool (CR-0060 Phase 3c).
//
// It lives in the server package rather than tools to avoid the import cycle
// that would arise from tools importing tools/help (which itself imports tools).
//
// Verb registration is feature-flag gated:
//   - Always-on (when mail is enabled at all): help, list_folders, list_messages,
//     get_message, search_messages.
//   - Gated by MailEnabled: get_conversation, list_attachments, get_attachment.
//   - Gated by MailManageEnabled: create_draft, create_reply_draft,
//     create_forward_draft, update_draft, delete_draft.
//
// The aggregate "mail" tool is registered unconditionally (FR-1). The operation
// enum only includes verbs whose feature flag is enabled at server start (FR-2).
package server

import (
	"time"

	"github.com/desek/outlook-local-mcp/internal/audit"
	"github.com/desek/outlook-local-mcp/internal/config"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/observability"
	"github.com/desek/outlook-local-mcp/internal/tools"
	"github.com/desek/outlook-local-mcp/internal/tools/help"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel/trace"
)

// mailVerbsConfig holds the dependencies required to build the mail domain verb
// slice. All fields are captured at server start.
type mailVerbsConfig struct {
	// retryCfg is the Graph API retry configuration applied to all mail handlers.
	retryCfg graph.RetryConfig

	// timeout is the maximum duration for a single Graph API call.
	timeout time.Duration

	// cfg is the full server configuration, used for feature-flag gating and
	// derived values such as MaxAttachmentSizeBytes and ProvenanceTag.
	cfg config.Config

	// provenancePropertyID is the fully-qualified MAPI extended property ID for
	// provenance tagging, built once at startup. Empty string disables tagging.
	provenancePropertyID string

	// m is the ToolMetrics instance for observability instrumentation.
	m *observability.ToolMetrics

	// tracer is the OTEL tracer for span creation.
	tracer trace.Tracer

	// authMW is the authentication middleware factory applied to every mail verb.
	authMW func(mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc

	// accountResolverMW is the account-resolver middleware applied to every mail
	// verb (mail tools resolve the Graph client via AccountResolver).
	accountResolverMW func(mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc

	// readOnly controls whether write verbs are blocked by ReadOnlyGuard.
	readOnly bool
}

// buildMailVerbs constructs the ordered []tools.Verb slice for the mail domain
// aggregate tool and returns a pointer to an initially empty VerbRegistry.
//
// Verbs are partitioned into three tiers based on feature flags:
//   - Always-on: list_folders, list_messages, get_message, search_messages
//     (registered whenever mail access is active at all; the caller is
//     responsible for calling this function only when mail is needed).
//   - MailEnabled-gated: get_conversation, list_attachments, get_attachment
//     (require Mail.Read scope provided by MailEnabled).
//   - MailManageEnabled-gated: create_draft, create_reply_draft,
//     create_forward_draft, update_draft, delete_draft (require
//     Mail.ReadWrite scope provided by MailManageEnabled).
//
// Each verb's Handler is pre-wrapped with authMW, accountResolverMW,
// observability, and audit middleware using the fully-qualified identity
// "mail.<verb>" per CR-0060 FR-13 and FR-14. Write verbs additionally include
// ReadOnlyGuard between observability and audit.
//
// The returned registry pointer is empty at the time of return. The caller
// MUST call RegisterDomainTool with the returned verbs, then assign the returned
// VerbRegistry back through the pointer so that the help verb can introspect
// all registered verbs at call time.
//
// Parameters:
//   - c: mailVerbsConfig with all required dependencies.
//
// Returns:
//   - verbs: ordered Verb slice for use with RegisterDomainTool.
//   - registryPtr: pointer whose value is assigned after registration.
func buildMailVerbs(c mailVerbsConfig) ([]tools.Verb, *tools.VerbRegistry) {
	empty := make(tools.VerbRegistry)
	registryPtr := &empty

	// wrap builds the read-verb chain: authMW -> accountResolverMW -> WithObservability -> AuditWrap -> Handler.
	wrap := func(name, auditOp string, h mcpserver.ToolHandlerFunc) tools.Handler {
		return tools.Handler(c.authMW(c.accountResolverMW(observability.WithObservability(name, c.m, c.tracer, audit.AuditWrap(name, auditOp, h)))))
	}

	// wrapWrite adds ReadOnlyGuard between observability and audit for write verbs.
	wrapWrite := func(name, auditOp string, h mcpserver.ToolHandlerFunc) tools.Handler {
		return tools.Handler(c.authMW(c.accountResolverMW(observability.WithObservability(name, c.m, c.tracer, ReadOnlyGuard(name, c.readOnly, audit.AuditWrap(name, auditOp, h))))))
	}

	rc := c.retryCfg

	verbs := []tools.Verb{
		help.NewHelpVerb(registryPtr),
		// Always-on read verbs.
		buildListFoldersVerb(c, rc, wrap),
		buildListMessagesVerb(c, rc, wrap),
		buildGetMessageVerb(c, rc, wrap),
		buildSearchMessagesVerb(c, rc, wrap),
	}

	// MailEnabled-gated read verbs.
	if c.cfg.MailEnabled {
		verbs = append(verbs,
			buildGetConversationVerb(c, rc, wrap),
			buildListAttachmentsVerb(c, rc, wrap),
			buildGetAttachmentVerb(c, rc, wrap),
		)
	}

	// MailManageEnabled-gated write verbs.
	if c.cfg.MailManageEnabled {
		verbs = append(verbs,
			buildCreateDraftVerb(c, rc, wrapWrite),
			buildCreateReplyDraftVerb(c, rc, wrapWrite),
			buildCreateForwardDraftVerb(c, rc, wrapWrite),
			buildUpdateDraftVerb(c, rc, wrapWrite),
			buildDeleteDraftVerb(c, rc, wrapWrite),
		)
	}

	return verbs, registryPtr
}

// buildListFoldersVerb constructs the list_folders Verb.
func buildListFoldersVerb(c mailVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:    "list_folders",
		Summary: "list mail folders (Inbox, Sent, Drafts, etc.) with unread and total counts",
		Handler: wrap("mail.list_folders", "read", tools.NewHandleListMailFolders(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Omit to auto-select the default account."),
			),
			mcp.WithNumber("max_results",
				mcp.Description("Maximum number of folders to return (default 25)."),
				mcp.Min(1),
			),
			mcp.WithString("output",
				mcp.Description("Output mode: 'text' (default), 'summary', or 'raw'."),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// buildListMessagesVerb constructs the list_messages Verb.
func buildListMessagesVerb(c mailVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:    "list_messages",
		Summary: "list messages in a folder or across all folders; filter by date, sender, thread",
		Handler: wrap("mail.list_messages", "read", tools.NewHandleListMessages(rc, c.timeout, c.provenancePropertyID)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("folder_id",
				mcp.Description("Mail folder ID to list messages from. Omit to list from all folders."),
			),
			mcp.WithString("start_datetime",
				mcp.Description("Start of date range (ISO 8601, e.g. 2026-03-12T00:00:00Z). Filters by receivedDateTime >=."),
			),
			mcp.WithString("end_datetime",
				mcp.Description("End of date range (ISO 8601). Filters by receivedDateTime <=."),
			),
			mcp.WithString("from",
				mcp.Description("Sender email address to filter by (e.g. alice@contoso.com)."),
			),
			mcp.WithString("conversation_id",
				mcp.Description("Conversation ID to retrieve all messages in a thread."),
			),
			mcp.WithBoolean("is_read",
				mcp.Description("Filter by read/unread state. Omit to include both."),
			),
			mcp.WithBoolean("is_draft",
				mcp.Description("Filter by draft state. Omit to include both."),
			),
			mcp.WithBoolean("has_attachments",
				mcp.Description("Filter by attachment presence. Omit to include both."),
			),
			mcp.WithString("importance",
				mcp.Description("Filter by message importance."),
				mcp.Enum("low", "normal", "high"),
			),
			mcp.WithString("flag_status",
				mcp.Description("Filter by follow-up flag status."),
				mcp.Enum("notFlagged", "flagged", "complete"),
			),
			mcp.WithBoolean("provenance",
				mcp.Description("Filter to messages created by this MCP server (requires provenance tagging)."),
			),
			mcp.WithNumber("max_results",
				mcp.Description("Maximum number of messages to return (default 25, max 100)."),
				mcp.Min(1),
				mcp.Max(100),
			),
			mcp.WithString("timezone",
				mcp.Description("IANA timezone name for the Prefer: outlook.timezone header."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Omit to auto-select the default account."),
			),
			mcp.WithString("output",
				mcp.Description("Output mode: 'text' (default), 'summary', or 'raw'."),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// buildGetMessageVerb constructs the get_message Verb.
func buildGetMessageVerb(c mailVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:    "get_message",
		Summary: "get full details of a single message by ID (bodyPreview by default; full body via output=raw)",
		Handler: wrap("mail.get_message", "read", tools.NewHandleGetMessage(rc, c.timeout, c.provenancePropertyID)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("message_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the message to retrieve."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Omit to auto-select the default account."),
			),
			mcp.WithString("output",
				mcp.Description("Output mode: 'text' (default), 'summary', or 'raw' (includes full HTML body and headers)."),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// buildSearchMessagesVerb constructs the search_messages Verb.
func buildSearchMessagesVerb(c mailVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:    "search_messages",
		Summary: "full-text KQL search across messages; ranked by relevance, not chronologically",
		Handler: wrap("mail.search_messages", "read", tools.NewHandleSearchMessages(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("KQL search string (e.g. subject:\"Design Review\" from:alice@contoso.com)."),
			),
			mcp.WithString("folder_id",
				mcp.Description("Mail folder ID to restrict search to. Omit to search all folders."),
			),
			mcp.WithNumber("max_results",
				mcp.Description("Maximum number of messages to return (default 25, max 100)."),
				mcp.Min(1),
				mcp.Max(100),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Omit to auto-select the default account."),
			),
			mcp.WithString("output",
				mcp.Description("Output mode: 'text' (default), 'summary', or 'raw'."),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// buildGetConversationVerb constructs the get_conversation Verb (MailEnabled-gated).
func buildGetConversationVerb(c mailVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:    "get_conversation",
		Summary: "retrieve all messages in an email thread in chronological order",
		Handler: wrap("mail.get_conversation", "read", tools.NewHandleGetConversation(rc, c.timeout, c.provenancePropertyID)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("message_id",
				mcp.Description("A message ID in the conversation; conversationId is resolved from it."),
			),
			mcp.WithString("conversation_id",
				mcp.Description("Conversation ID to retrieve directly (skips the initial message fetch)."),
			),
			mcp.WithNumber("max_results",
				mcp.Description("Maximum number of messages to return (default 50, max 100)."),
				mcp.Min(1),
				mcp.Max(100),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Omit to auto-select the default account."),
			),
			mcp.WithString("output",
				mcp.Description("Output mode: 'text' (default), 'summary', or 'raw'."),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// buildListAttachmentsVerb constructs the list_attachments Verb (MailEnabled-gated).
func buildListAttachmentsVerb(c mailVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:    "list_attachments",
		Summary: "list attachment metadata (id, name, contentType, size) for a message",
		Handler: wrap("mail.list_attachments", "read", tools.NewHandleListAttachments(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("message_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the parent message."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Omit to auto-select the default account."),
			),
			mcp.WithString("output",
				mcp.Description("Output mode: 'text' (default), 'summary', or 'raw'."),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// buildGetAttachmentVerb constructs the get_attachment Verb (MailEnabled-gated).
func buildGetAttachmentVerb(c mailVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:    "get_attachment",
		Summary: "download an attachment; returns metadata and base64 content up to the size limit",
		Handler: wrap("mail.get_attachment", "read", tools.NewHandleGetAttachment(rc, c.timeout, c.cfg.MaxAttachmentSizeBytes)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("message_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the parent message."),
			),
			mcp.WithString("attachment_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the attachment."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Omit to auto-select the default account."),
			),
			mcp.WithString("output",
				mcp.Description("Output mode: 'text' (default), 'summary', or 'raw'."),
				mcp.Enum("text", "summary", "raw"),
			),
		},
	}
}

// buildCreateDraftVerb constructs the create_draft Verb (MailManageEnabled-gated).
func buildCreateDraftVerb(c mailVerbsConfig, rc graph.RetryConfig, wrapWrite func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:    "create_draft",
		Summary: "create a new email draft in the Drafts folder (not sent automatically)",
		Handler: wrapWrite("mail.create_draft", "write", tools.NewHandleCreateDraft(rc, c.timeout, c.provenancePropertyID)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(false),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("to_recipients",
				mcp.Description("Comma-separated list of To recipient email addresses."),
			),
			mcp.WithString("cc_recipients",
				mcp.Description("Comma-separated list of Cc recipient email addresses."),
			),
			mcp.WithString("bcc_recipients",
				mcp.Description("Comma-separated list of Bcc recipient email addresses."),
			),
			mcp.WithString("subject",
				mcp.Description("Draft subject line."),
			),
			mcp.WithString("body",
				mcp.Description("Draft body content. Plain text unless content_type is 'html'."),
			),
			mcp.WithString("content_type",
				mcp.Description("Body content type: 'text' (default) or 'html'."),
				mcp.Enum("text", "html"),
			),
			mcp.WithString("importance",
				mcp.Description("Draft importance: low, normal, or high."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Omit to auto-select the default account."),
			),
		},
	}
}

// buildCreateReplyDraftVerb constructs the create_reply_draft Verb (MailManageEnabled-gated).
func buildCreateReplyDraftVerb(c mailVerbsConfig, rc graph.RetryConfig, wrapWrite func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:    "create_reply_draft",
		Summary: "create a reply draft to an existing message preserving threading headers",
		Handler: wrapWrite("mail.create_reply_draft", "write", tools.NewHandleCreateReplyDraft(rc, c.timeout, c.provenancePropertyID)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(false),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("message_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the source message to reply to."),
			),
			mcp.WithString("comment",
				mcp.Description("Optional reply body text prepended to the quoted original."),
			),
			mcp.WithBoolean("reply_all",
				mcp.Description("When true, reply to all original recipients (To + Cc). Default false."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Omit to auto-select the default account."),
			),
		},
	}
}

// buildCreateForwardDraftVerb constructs the create_forward_draft Verb (MailManageEnabled-gated).
func buildCreateForwardDraftVerb(c mailVerbsConfig, rc graph.RetryConfig, wrapWrite func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:    "create_forward_draft",
		Summary: "create a forward draft of an existing message with new recipients",
		Handler: wrapWrite("mail.create_forward_draft", "write", tools.NewHandleCreateForwardDraft(rc, c.timeout, c.provenancePropertyID)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(false),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("message_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the source message to forward."),
			),
			mcp.WithString("to_recipients",
				mcp.Description("Comma-separated list of To recipient email addresses."),
			),
			mcp.WithString("comment",
				mcp.Description("Optional forward body text prepended to the quoted original."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Omit to auto-select the default account."),
			),
		},
	}
}

// buildUpdateDraftVerb constructs the update_draft Verb (MailManageEnabled-gated).
func buildUpdateDraftVerb(c mailVerbsConfig, rc graph.RetryConfig, wrapWrite func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:    "update_draft",
		Summary: "update fields of an existing draft (PATCH semantics; non-draft messages rejected)",
		Handler: wrapWrite("mail.update_draft", "write", tools.NewHandleUpdateDraft(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("message_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the draft message to update."),
			),
			mcp.WithString("to_recipients",
				mcp.Description("Comma-separated list of To recipient email addresses (replaces existing)."),
			),
			mcp.WithString("cc_recipients",
				mcp.Description("Comma-separated list of Cc recipient email addresses (replaces existing)."),
			),
			mcp.WithString("bcc_recipients",
				mcp.Description("Comma-separated list of Bcc recipient email addresses (replaces existing)."),
			),
			mcp.WithString("subject",
				mcp.Description("New draft subject line."),
			),
			mcp.WithString("body",
				mcp.Description("New draft body content."),
			),
			mcp.WithString("content_type",
				mcp.Description("Body content type: 'text' or 'html'."),
				mcp.Enum("text", "html"),
			),
			mcp.WithString("importance",
				mcp.Description("New draft importance: low, normal, or high."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Omit to auto-select the default account."),
			),
		},
	}
}

// buildDeleteDraftVerb constructs the delete_draft Verb (MailManageEnabled-gated).
func buildDeleteDraftVerb(c mailVerbsConfig, rc graph.RetryConfig, wrapWrite func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:    "delete_draft",
		Summary: "permanently delete a draft message (irreversible; non-draft messages rejected)",
		Handler: wrapWrite("mail.delete_draft", "delete", tools.NewHandleDeleteDraft(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("message_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the draft message to delete."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Omit to auto-select the default account."),
			),
		},
	}
}

// mailToolAnnotations returns the conservative aggregate MCP annotations for
// the mail domain tool per CR-0060 FR-9 and AC-9.
//
// readOnlyHint is false because write verbs (create_draft, create_reply_draft,
// create_forward_draft, update_draft, delete_draft) may be present when
// MailManageEnabled is true. destructiveHint is true because delete_draft
// permanently removes a message. idempotentHint is false because create_draft,
// create_reply_draft, and create_forward_draft are non-idempotent.
// openWorldHint is true because all verbs call Microsoft Graph.
//
// Per FR-9 these values represent the most conservative annotation across all
// verbs that may be registered for the domain. They remain fixed at
// construction time and must be consistent across deployment configurations.
func mailToolAnnotations() []mcp.ToolOption {
	return []mcp.ToolOption{
		mcp.WithTitleAnnotation("Mail"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	}
}
