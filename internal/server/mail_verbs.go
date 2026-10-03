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
//     create_forward_draft, update_draft, delete_draft, move_message, set_flag,
//     set_categories, mark_read, add_attachment.
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
//     create_forward_draft, update_draft, delete_draft, move_message,
//     set_flag, set_categories, mark_read, add_attachment (require
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
			buildMoveMessageVerb(c, rc, wrapWrite),
			buildSetFlagVerb(c, rc, wrapWrite),
			buildSetCategoriesVerb(c, rc, wrapWrite),
			buildMarkReadVerb(c, rc, wrapWrite),
			buildAddAttachmentVerb(c, rc, wrapWrite),
		)
	}

	return verbs, registryPtr
}

// buildListFoldersVerb constructs the list_folders Verb.
func buildListFoldersVerb(c mailVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "list_folders",
		Summary:     "list mail folders (Inbox, Sent, Drafts, etc.) with unread and total counts",
		Description: "Returns all mail folders with their display name, unread message count, and total message count. Use the returned folder IDs with list_messages to scope queries to a specific folder. Requires mail access (MAIL_ENABLED=true).",
		SeeDocs:     []string{"concepts#mail-gating"},
		Handler:     wrap("mail.list_folders", "read", tools.NewHandleListMailFolders(rc, c.timeout)),
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
		Name:        "list_messages",
		Summary:     "list messages in a folder or across all folders; filter by date, sender, thread",
		Description: "Lists messages in a mail folder or across all folders, with optional filters for date range, sender, conversation thread, read state, draft state, attachment presence, importance, and flag status. Results include a bodyPreview; use get_message with output=raw for the full HTML body. For full-text search, use search_messages instead.",
		Examples: []tools.Example{
			{Args: map[string]any{"folder_id": "Inbox", "is_read": false}, Comment: "list unread messages in inbox"},
			{Args: map[string]any{"from": "alice@contoso.com", "max_results": 10}, Comment: "list recent messages from a sender"},
		},
		SeeDocs: []string{"concepts#output-tiers", "concepts#mail-gating"},
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
				mcp.Description("Read/unread state. On list_messages it filters results (omit to include both); on mark_read it is the required state to write to the message."),
			),
			mcp.WithBoolean("is_draft",
				mcp.Description("Filter by draft state. Omit to include both."),
			),
			mcp.WithBoolean("has_attachments",
				mcp.Description("Filter by attachment presence. Omit to include both."),
			),
			mcp.WithString("importance",
				mcp.Description("Message importance. On list_messages it filters results; on create_draft and update_draft it is the importance to write to the draft."),
				mcp.Enum("low", "normal", "high"),
			),
			mcp.WithString("flag_status",
				mcp.Description("Follow-up flag status. On list_messages it filters results; on set_flag it is the required status to write to the message."),
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
		Name:        "get_message",
		Summary:     "get full message details by ID; bodyPreview by default, full body via output=raw",
		Description: "Fetches full metadata for a single mail message by its ID. Text and summary output include a bodyPreview (first 255 characters). To read the complete HTML body and all headers, use output=raw. Use list_messages or search_messages to obtain a message ID.",
		SeeDocs:     []string{"concepts#output-tiers"},
		Handler:     wrap("mail.get_message", "read", tools.NewHandleGetMessage(rc, c.timeout, c.provenancePropertyID)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("message_id",
				mcp.Required(),
				mcp.Description("The unique identifier of a message. On get_message it names the message to retrieve; on write verbs such as update_draft, mark_read, set_flag, set_categories, and move_message it names the message to modify."),
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
		Name:        "search_messages",
		Summary:     "full-text KQL search across messages; ranked by relevance, not chronologically",
		Description: "Searches mail messages using Keyword Query Language (KQL). Results are ranked by relevance, not chronological order. Restrict a search to a property with property:value, for example from:alice@contoso.com or hasAttachments:true. Write a multi-word property value in parentheses, for example subject:(Design Review); the parenthesised form matches all of its tokens in any order rather than as an adjacent phrase. Do not write a multi-word property value without parentheses: an unparenthesised value such as subject:Design Review binds only its first token to the property and turns the remaining words into free-text terms searched across the whole message. Use list_messages with date filters for chronological browsing.",
		Examples: []tools.Example{
			{Args: map[string]any{"query": "\"subject:(Design Review)\""}, Comment: "find messages whose subject holds both words, in any order"},
			{Args: map[string]any{"query": "\"from:alice@contoso.com hasAttachments:true\""}, Comment: "find messages with attachments from a sender"},
		},
		SeeDocs: []string{"concepts#output-tiers"},
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
				mcp.Description("KQL search string. Write a multi-word property value in parentheses, e.g. subject:(Design Review) from:alice@contoso.com; an unparenthesised multi-word value binds only its first token."),
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
		Name:        "get_conversation",
		Summary:     "retrieve all messages in an email thread in chronological order",
		Description: "Retrieves all messages that share a conversation thread in chronological order. Supply either a message_id (the server resolves the conversationId) or a conversation_id directly. Requires MAIL_ENABLED=true.",
		SeeDocs:     []string{"concepts#mail-gating"},
		Handler:     wrap("mail.get_conversation", "read", tools.NewHandleGetConversation(rc, c.timeout, c.provenancePropertyID)),
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
		Name:        "list_attachments",
		Summary:     "list attachment metadata (id, name, contentType, size) for a message",
		Description: "Lists the attachments of a mail message, returning metadata: attachment ID, name, content type, and size in bytes. Use get_attachment with the returned attachment_id to download the content. Requires MAIL_ENABLED=true.",
		SeeDocs:     []string{"concepts#mail-gating"},
		Handler:     wrap("mail.list_attachments", "read", tools.NewHandleListAttachments(rc, c.timeout)),
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
		Name:        "get_attachment",
		Summary:     "download an attachment; returns metadata and base64 content up to the size limit",
		Description: "Downloads a mail attachment by ID and returns its metadata plus base64-encoded content. Attachments larger than the server's MaxAttachmentSizeBytes limit are rejected with an informative error. Requires MAIL_ENABLED=true.",
		SeeDocs:     []string{"concepts#mail-gating"},
		Handler:     wrap("mail.get_attachment", "read", tools.NewHandleGetAttachment(rc, c.timeout, c.cfg.MaxAttachmentSizeBytes)),
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
		Name:        "create_draft",
		Summary:     "create a new email draft in the Drafts folder (not sent automatically)",
		Description: "Creates a new email draft and saves it to the Drafts folder. The draft is never sent automatically; the user opens Outlook and sends it manually. Supports To, Cc, Bcc recipients, subject, plain-text or HTML body, and importance. Requires MAIL_MANAGE_ENABLED=true.",
		Examples: []tools.Example{
			{Args: map[string]any{"to_recipients": "alice@contoso.com", "subject": "Follow-up", "body": "Hi Alice..."}, Comment: "create a simple plain-text draft"},
		},
		SeeDocs: []string{"concepts#mail-gating"},
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
		Name:        "create_reply_draft",
		Summary:     "create a reply draft to an existing message preserving threading headers",
		Description: "Creates a reply draft for an existing message, preserving all email threading headers (References, In-Reply-To). The original message is quoted automatically. Use reply_all=true to reply to all original recipients. Requires MAIL_MANAGE_ENABLED=true.",
		SeeDocs:     []string{"concepts#mail-gating"},
		Handler:     wrapWrite("mail.create_reply_draft", "write", tools.NewHandleCreateReplyDraft(rc, c.timeout, c.provenancePropertyID)),
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
		Name:        "create_forward_draft",
		Summary:     "create a forward draft of an existing message with new recipients",
		Description: "Creates a forward draft for an existing message with the original message quoted. Supply the new To recipients and an optional forward comment. Requires MAIL_MANAGE_ENABLED=true.",
		SeeDocs:     []string{"concepts#mail-gating"},
		Handler:     wrapWrite("mail.create_forward_draft", "write", tools.NewHandleCreateForwardDraft(rc, c.timeout, c.provenancePropertyID)),
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
		Name:        "update_draft",
		Summary:     "update draft fields (PATCH semantics; non-draft messages rejected)",
		Description: "Updates fields of an existing draft using PATCH semantics: only supplied fields are changed. Attempting to update a non-draft message returns an error. Supports recipients, subject, body, content type, and importance. Requires MAIL_MANAGE_ENABLED=true.",
		SeeDocs:     []string{"concepts#mail-gating"},
		Handler:     wrapWrite("mail.update_draft", "write", tools.NewHandleUpdateDraft(rc, c.timeout)),
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
		Name:        "delete_draft",
		Summary:     "permanently delete a draft message (irreversible; non-draft messages rejected)",
		Description: "Permanently deletes a draft message. This operation is irreversible. Attempting to delete a non-draft message returns an error as a safety guard. Requires MAIL_MANAGE_ENABLED=true.",
		SeeDocs:     []string{"concepts#mail-gating"},
		Handler:     wrapWrite("mail.delete_draft", "delete", tools.NewHandleDeleteDraft(rc, c.timeout)),
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

// buildMoveMessageVerb constructs the move_message Verb (MailManageEnabled-gated).
//
// It is the only one of the four received-message write verbs classified
// destructive and non-idempotent, because the move removes the message from its
// source folder and mints a new identifier, leaving the caller's original
// identifier unusable.
func buildMoveMessageVerb(c mailVerbsConfig, rc graph.RetryConfig, wrapWrite func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "move_message",
		Summary:     "move a message to another folder; the move mints a new message ID",
		Description: "Moves an existing message to a different mail folder. Requires message_id and destination_folder_id; obtain a destination identifier from list_folders. The move mints a NEW message identifier, so the confirmation names the new identifier alongside the original, which stops resolving once the move succeeds. Annotated destructive and non-idempotent for that reason: the message leaves its source folder and the identifier the caller was holding becomes unusable, and repeating the call with the original identifier fails. Moving to Deleted Items is the reversible alternative to deletion. Returns a text confirmation and takes no output parameter. Requires MAIL_MANAGE_ENABLED=true.",
		Examples: []tools.Example{
			{Args: map[string]any{"message_id": "AAMkAGI2...", "destination_folder_id": "AQMkADAwAT..."}, Comment: "file a message into a folder obtained from list_folders"},
		},
		SeeDocs: []string{"concepts#mail-gating", "concepts#tool-annotation-semantics"},
		Handler: wrapWrite("mail.move_message", "write", tools.NewHandleMoveMessage(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(false),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("message_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the message to move."),
			),
			mcp.WithString("destination_folder_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the destination mail folder, obtained from list_folders. This names where the message is moved to; folder_id scopes a read instead."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Omit to auto-select the default account."),
			),
		},
	}
}

// buildSetFlagVerb constructs the set_flag Verb (MailManageEnabled-gated).
func buildSetFlagVerb(c mailVerbsConfig, rc graph.RetryConfig, wrapWrite func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "set_flag",
		Summary:     "set a message's follow-up flag: notFlagged, flagged, or complete",
		Description: "Sets the follow-up flag on an existing message. Requires message_id and flag_status, which must be one of notFlagged, flagged, or complete. It applies to received messages as well as drafts and imposes no draft guard. Annotated non-destructive and idempotent: the flag value is replaced in place, so repeating the call leaves the same state and returns the same confirmation. The confirmation reports the status Graph stored, not the requested one. Returns a text confirmation and takes no output parameter. Requires MAIL_MANAGE_ENABLED=true.",
		Examples: []tools.Example{
			{Args: map[string]any{"message_id": "AAMkAGI2...", "flag_status": "flagged"}, Comment: "flag a message for follow-up"},
			{Args: map[string]any{"message_id": "AAMkAGI2...", "flag_status": "complete"}, Comment: "mark an existing follow-up flag complete"},
		},
		SeeDocs: []string{"concepts#mail-gating"},
		Handler: wrapWrite("mail.set_flag", "write", tools.NewHandleSetFlag(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("message_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the message to flag."),
			),
			mcp.WithString("flag_status",
				mcp.Required(),
				mcp.Description("Follow-up flag status to write: notFlagged, flagged, or complete."),
				mcp.Enum("notFlagged", "flagged", "complete"),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Omit to auto-select the default account."),
			),
		},
	}
}

// buildSetCategoriesVerb constructs the set_categories Verb (MailManageEnabled-gated).
func buildSetCategoriesVerb(c mailVerbsConfig, rc graph.RetryConfig, wrapWrite func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "set_categories",
		Summary:     "replace a message's categories; an empty value clears every category",
		Description: "Replaces the category list on an existing message. Requires message_id and categories, a comma-separated list that REPLACES the existing set rather than appending to it; surrounding whitespace is trimmed and empty entries are dropped. An empty or whitespace-only value clears every category, and the confirmation then states that the message carries none. Categories are written as supplied and no master category is created in the mailbox. Annotated non-destructive and idempotent: the list is replaced in place, so repeating the call leaves the same state. The confirmation lists the categories Graph stored. Returns a text confirmation and takes no output parameter. Requires MAIL_MANAGE_ENABLED=true.",
		Examples: []tools.Example{
			{Args: map[string]any{"message_id": "AAMkAGI2...", "categories": "Project Apollo, Urgent"}, Comment: "label a message with two categories"},
			{Args: map[string]any{"message_id": "AAMkAGI2...", "categories": ""}, Comment: "clear every category from a message"},
		},
		SeeDocs: []string{"concepts#mail-gating"},
		Handler: wrapWrite("mail.set_categories", "write", tools.NewHandleSetCategories(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("message_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the message to categorise."),
			),
			mcp.WithString("categories",
				mcp.Required(),
				mcp.Description("Comma-separated category list replacing the existing set. Supply an empty string to clear every category."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Omit to auto-select the default account."),
			),
		},
	}
}

// buildMarkReadVerb constructs the mark_read Verb (MailManageEnabled-gated).
func buildMarkReadVerb(c mailVerbsConfig, rc graph.RetryConfig, wrapWrite func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "mark_read",
		Summary:     "mark a message read or unread; is_read is a required boolean",
		Description: "Writes the read state of an existing message. Requires message_id and the boolean is_read: true marks the message read, false marks it unread. It writes in both directions, so is_read is required rather than assumed. Annotated non-destructive and idempotent: the state is set, not toggled, so repeating the call leaves the same state and returns the same confirmation. The confirmation reports the state Graph stored. Returns a text confirmation and takes no output parameter. Requires MAIL_MANAGE_ENABLED=true.",
		Examples: []tools.Example{
			{Args: map[string]any{"message_id": "AAMkAGI2...", "is_read": true}, Comment: "mark a message read"},
			{Args: map[string]any{"message_id": "AAMkAGI2...", "is_read": false}, Comment: "mark a message unread again"},
		},
		SeeDocs: []string{"concepts#mail-gating"},
		Handler: wrapWrite("mail.mark_read", "write", tools.NewHandleMarkRead(rc, c.timeout)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("message_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the message whose read state is written."),
			),
			mcp.WithBoolean("is_read",
				mcp.Required(),
				mcp.Description("True to mark the message read, false to mark it unread. Required: the verb writes in both directions."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Omit to auto-select the default account."),
			),
		},
	}
}

// buildAddAttachmentVerb constructs the add_attachment Verb
// (MailManageEnabled-gated).
//
// The MIME parameter is named mime_type rather than content_type deliberately.
// The aggregate tool publishes the union of every verb's parameters and merges
// duplicate names first-occurrence-wins, and the mail domain already declares
// content_type as a body content type restricted by an enum to text and html.
// Reusing that name would publish a schema telling a caller an attachment's
// content type must be text or html, so the distinct concept takes a distinct
// name.
func buildAddAttachmentVerb(c mailVerbsConfig, rc graph.RetryConfig, wrapWrite func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "add_attachment",
		Summary:     "attach a file to an existing draft; bytes supplied base64 in content_bytes",
		Description: "Attaches a file to an existing draft message. Requires message_id, name, and content_bytes; content_bytes carries the file content as a standard base64-encoded string, and the optional mime_type sets the attachment content type (defaults to application/octet-stream). Only drafts accept attachments: a non-draft message is refused. The transfer mechanism is chosen from the decoded size, small files in a single request and larger ones through a chunked upload session, so the caller never selects it. Attachments above the server's MaxAttachmentSizeBytes limit are rejected before any upload begins. Uploads above 150 MB (the Graph upload-session limit) are refused whatever the configured limit says, and the tenant message-size limit (35 MB by default) can refuse smaller files. Annotated non-destructive (it only adds, leaving the draft and its existing attachments untouched) and non-idempotent (a repeated call adds a second attachment with a new ID). The confirmation names the draft, the attachment, its size, and the attachment ID the service assigned. Returns a text confirmation and takes no output parameter. Requires MAIL_MANAGE_ENABLED=true.",
		Examples: []tools.Example{
			{Args: map[string]any{"message_id": "AAMkAGI2...", "name": "report.pdf", "content_bytes": "JVBERi0xLjQK...", "mime_type": "application/pdf"}, Comment: "attach a PDF to a draft"},
			{Args: map[string]any{"message_id": "AAMkAGI2...", "name": "notes.txt", "content_bytes": "aGVsbG8gd29ybGQ="}, Comment: "attach a small file without naming its MIME type"},
		},
		SeeDocs: []string{"concepts#mail-gating"},
		Handler: wrapWrite("mail.add_attachment", "write", tools.NewHandleAddAttachment(rc, c.timeout, c.cfg.MaxAttachmentSizeBytes)),
		Annotations: []mcp.ToolOption{
			mcp.WithReadOnlyHintAnnotation(false),
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithIdempotentHintAnnotation(false),
			mcp.WithOpenWorldHintAnnotation(true),
		},
		Schema: []mcp.ToolOption{
			mcp.WithString("message_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the draft message the attachment is added to."),
			),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("File name the attachment carries in the draft, for example report.pdf."),
			),
			mcp.WithString("content_bytes",
				mcp.Required(),
				mcp.Description("File content as a standard base64-encoded string. The decoded size selects the transfer path and is checked against the server's attachment size limit."),
			),
			mcp.WithString("mime_type",
				mcp.Description("MIME type of the attachment, for example application/pdf. Omit to default to application/octet-stream. This is the attachment content type, not the draft body content_type."),
			),
			mcp.WithString("account",
				mcp.Description("Account label or UPN to use. Omit to auto-select the default account."),
			),
		},
	}
}
