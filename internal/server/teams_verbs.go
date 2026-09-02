// Package server — this file builds the teams domain verb slice for the
// aggregate "teams" MCP tool.
//
// It lives in the server package rather than tools to avoid the import cycle
// that would arise from tools importing tools/help (which itself imports tools).
//
// The whole domain is gated: the caller builds and registers it only when
// cfg.TeamsEnabled is true, so a default server registers no teams tool and
// requests no Teams scope. Every verb reads, including compose_reply, which
// reads the message it quotes and returns prepared text without posting it, so
// the file declares no write chain and no ReadOnlyGuard layer: there is nothing
// for the guard to block.
//
// @agents-index: builds the thirteen read verbs of the opt-in teams domain tool.
package server

import (
	"time"

	"github.com/desek/outlook-local-mcp/internal/audit"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/observability"
	"github.com/desek/outlook-local-mcp/internal/tools"
	"github.com/desek/outlook-local-mcp/internal/tools/help"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel/trace"
)

// teamsVerbsConfig holds the dependencies required to build the teams domain
// verb slice. All fields are captured at server start.
//
// It carries no readOnly field, unlike the mail and calendar configurations,
// because the domain registers no write verb for ReadOnlyGuard to block.
type teamsVerbsConfig struct {
	// retryCfg is the Graph API retry configuration applied to all handlers.
	retryCfg graph.RetryConfig

	// timeout is the maximum duration for a single Graph API call.
	timeout time.Duration

	// m is the ToolMetrics instance for observability instrumentation.
	m *observability.ToolMetrics

	// tracer is the OTEL tracer for span creation.
	tracer trace.Tracer

	// authMW is the authentication middleware factory applied to every verb.
	authMW func(mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc

	// accountResolverMW is the account-resolver middleware applied to every
	// verb: teams verbs are Graph reads that need a resolved account.
	accountResolverMW func(mcpserver.ToolHandlerFunc) mcpserver.ToolHandlerFunc
}

// Descriptions shared by more than one verb, declared once so the published
// schema cannot describe the same parameter two different ways.
const (
	teamsOutputDescription    = "Output mode: 'text' (default), 'summary', or 'raw'."
	teamsAccountDescription   = "Account label or UPN to use. Omit to auto-select the default account."
	teamsChatIDDescription    = "The chat identifier, as carried by a list_chats entry or a search hit."
	teamsTeamIDDescription    = "The team identifier, as carried by a search hit for a channel message."
	teamsChannelIDDescription = "The channel identifier, as carried by a search hit for a channel message."
	teamsMeetingIDDescription = "The meeting-scoped onlineMeeting identifier returned by get_online_meeting."
)

// teamsReadClosing is the annotation sentence every verb's description ends
// with. The domain is uniformly read-only, so the sentence is uniform too.
const teamsReadClosing = "Read-only, non-destructive, idempotent, and open-world: it calls Microsoft Graph and writes nothing."

// teamsReadAnnotations returns the four hints every Graph-calling verb in this
// domain declares. The domain registers no write, so the matrix is uniform and
// stating it once keeps a new verb from being classified by hand and getting it
// wrong.
func teamsReadAnnotations() []mcp.ToolOption {
	return []mcp.ToolOption{
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(true),
	}
}

// teamsOutputParam returns the output tier parameter declared by each of the
// eleven resource reads. compose_reply and help do not declare it.
func teamsOutputParam() mcp.ToolOption {
	return mcp.WithString("output",
		mcp.Description(teamsOutputDescription),
		mcp.Enum("text", "summary", "raw"),
	)
}

// buildTeamsVerbs constructs the ordered []tools.Verb slice for the teams
// domain aggregate tool and returns a pointer to an initially empty
// VerbRegistry.
//
// The slice is exactly thirteen verbs: help, the eleven resource reads, and
// compose_reply, which reads the message it quotes and posts nothing. Each
// verb's Handler is pre-wrapped with authMW, accountResolverMW, observability,
// and audit middleware under the fully-qualified identity "teams.<verb>" with
// the audit operation "read", so the audit record and the OpenTelemetry
// attributes carry the same {domain}.{operation} identity every other verb
// carries.
//
// The returned registry pointer is empty at the time of return. The caller
// MUST call RegisterDomainTool with the returned verbs, then assign the
// returned VerbRegistry back through the pointer so the help verb can
// introspect all registered verbs at call time.
//
// Parameters:
//   - c: teamsVerbsConfig with all required dependencies.
//
// Returns:
//   - verbs: ordered Verb slice for use with RegisterDomainTool.
//   - registryPtr: pointer whose value is assigned after registration.
func buildTeamsVerbs(c teamsVerbsConfig) ([]tools.Verb, *tools.VerbRegistry) {
	empty := make(tools.VerbRegistry)
	registryPtr := &empty

	// wrap builds the read-verb chain:
	// authMW -> accountResolverMW -> WithObservability -> AuditWrap -> Handler.
	wrap := func(name, auditOp string, h mcpserver.ToolHandlerFunc) tools.Handler {
		return tools.Handler(c.authMW(c.accountResolverMW(observability.WithObservability(name, c.m, c.tracer, audit.AuditWrap(name, auditOp, h)))))
	}

	rc := c.retryCfg

	verbs := []tools.Verb{
		help.NewHelpVerb(registryPtr),
		buildTeamsSearchVerb(c, rc, wrap),
		buildListChatsVerb(c, rc, wrap),
		buildListChatMessagesVerb(c, rc, wrap),
		buildGetChatMessageVerb(c, rc, wrap),
		buildListChatMessageRepliesVerb(c, rc, wrap),
		buildListChannelMessagesVerb(c, rc, wrap),
		buildGetChannelMessageVerb(c, rc, wrap),
		buildListChannelMessageRepliesVerb(c, rc, wrap),
		buildComposeReplyVerb(c, rc, wrap),
		buildGetOnlineMeetingVerb(c, rc, wrap),
		buildListTranscriptsVerb(c, rc, wrap),
		buildGetTranscriptVerb(c, rc, wrap),
	}

	return verbs, registryPtr
}

// buildTeamsSearchVerb constructs the search Verb, the domain's entry point:
// the only verb that resolves identifiers without already holding one.
func buildTeamsSearchVerb(c teamsVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "search",
		Summary:     "free-text search over Teams chat and channel messages",
		Description: "Searches Microsoft Teams messages and returns ranked hits, each labelled with the chat or the team and channel it came from, so the identifiers every other verb in this domain requires come from here. Enumerating teams and channels is not offered; this is how a channel is resolved. Requires query; optional account and output ('text' by default, 'summary', or 'raw'). " + teamsReadClosing,
		Examples: []tools.Example{
			{Args: map[string]any{"query": "release checklist"}, Comment: "find the messages discussing a topic"},
			{Args: map[string]any{"query": "from:alex budget", "output": "summary"}, Comment: "narrow the hits and return identifiers only"},
		},
		SeeDocs:     []string{"concepts#output-tiers", "concepts#teams-gating", "troubleshooting#teams-disabled"},
		Handler:     wrap("teams.search", "read", tools.NewHandleTeamsSearch(rc, c.timeout)),
		Annotations: teamsReadAnnotations(),
		Schema: []mcp.ToolOption{
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Free-text search value. An empty or whitespace-only value is rejected before any request is issued."),
			),
			mcp.WithString("account", mcp.Description(teamsAccountDescription)),
			teamsOutputParam(),
		},
	}
}

// buildListChatsVerb constructs the list_chats Verb.
func buildListChatsVerb(c teamsVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "list_chats",
		Summary:     "list the chats the signed-in user is a member of",
		Description: "Lists the signed-in user's Teams conversations with enough of each to choose between them, including the preview of its last message, which is often all that distinguishes one untitled one-to-one chat from another. Reading a single chat by identifier is not offered; this listing is how a chat identifier is resolved without a search. No required parameters; optional account and output ('text' by default, 'summary', or 'raw'). " + teamsReadClosing,
		Examples: []tools.Example{
			{Args: map[string]any{}, Comment: "list the user's chats"},
			{Args: map[string]any{"output": "summary"}, Comment: "return chat identifiers and topics only"},
		},
		SeeDocs:     []string{"concepts#output-tiers", "concepts#teams-gating", "troubleshooting#teams-disabled"},
		Handler:     wrap("teams.list_chats", "read", tools.NewHandleListChats(rc, c.timeout)),
		Annotations: teamsReadAnnotations(),
		Schema: []mcp.ToolOption{
			mcp.WithString("account", mcp.Description(teamsAccountDescription)),
			teamsOutputParam(),
		},
	}
}

// buildListChatMessagesVerb constructs the list_chat_messages Verb.
func buildListChatMessagesVerb(c teamsVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "list_chat_messages",
		Summary:     "list the messages in one chat, each with a body preview",
		Description: "Lists the messages in one Teams conversation, each with a body preview, so the caller can decide which single message is worth a full read rather than escalating every message in the thread. Use list_chats or search to obtain a chat identifier. Requires chat_id; optional account and output ('text' by default, 'summary', or 'raw'). " + teamsReadClosing,
		Examples: []tools.Example{
			{Args: map[string]any{"chat_id": "19:abc...@thread.v2"}, Comment: "read a conversation returned by list_chats"},
		},
		SeeDocs:     []string{"concepts#output-tiers", "concepts#teams-gating", "troubleshooting#teams-disabled"},
		Handler:     wrap("teams.list_chat_messages", "read", tools.NewHandleListChatMessages(rc, c.timeout)),
		Annotations: teamsReadAnnotations(),
		Schema: []mcp.ToolOption{
			mcp.WithString("chat_id", mcp.Required(), mcp.Description(teamsChatIDDescription)),
			mcp.WithString("account", mcp.Description(teamsAccountDescription)),
			teamsOutputParam(),
		},
	}
}

// buildGetChatMessageVerb constructs the get_chat_message Verb.
func buildGetChatMessageVerb(c teamsVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "get_chat_message",
		Summary:     "get one chat message; the full body requires output=raw",
		Description: "Fetches one message in a Teams conversation. A Teams body is arbitrarily long and commonly HTML, so the default tier returns a body preview and the full body is returned only under output=raw; decide from the preview whether the full fetch is warranted. Requires chat_id and message_id; optional account and output ('text' by default, 'summary', or 'raw'). " + teamsReadClosing,
		Examples: []tools.Example{
			{Args: map[string]any{"chat_id": "19:abc...@thread.v2", "message_id": "1700000000000"}, Comment: "read the message behind a search hit"},
			{Args: map[string]any{"chat_id": "19:abc...@thread.v2", "message_id": "1700000000000", "output": "raw"}, Comment: "escalate to the full message body"},
		},
		SeeDocs:     []string{"concepts#output-tiers", "concepts#teams-gating", "troubleshooting#teams-disabled"},
		Handler:     wrap("teams.get_chat_message", "read", tools.NewHandleGetChatMessage(rc, c.timeout)),
		Annotations: teamsReadAnnotations(),
		Schema: []mcp.ToolOption{
			mcp.WithString("chat_id", mcp.Required(), mcp.Description(teamsChatIDDescription)),
			mcp.WithString("message_id", mcp.Required(), mcp.Description("The message identifier, as carried by a list_chat_messages entry or a search hit.")),
			mcp.WithString("account", mcp.Description(teamsAccountDescription)),
			teamsOutputParam(),
		},
	}
}

// buildListChatMessageRepliesVerb constructs the list_chat_message_replies Verb.
func buildListChatMessageRepliesVerb(c teamsVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "list_chat_message_replies",
		Summary:     "list the replies hanging under one chat message",
		Description: "Lists the replies under one chat message. A thread listing returns top-level messages only, so the answers to a question are reachable only here; each reply names the parent it answers. Requires chat_id and message_id; optional account and output ('text' by default, 'summary', or 'raw'). " + teamsReadClosing,
		Examples: []tools.Example{
			{Args: map[string]any{"chat_id": "19:abc...@thread.v2", "message_id": "1700000000000"}, Comment: "read the answers under a question"},
		},
		SeeDocs:     []string{"concepts#output-tiers", "concepts#teams-gating", "troubleshooting#teams-disabled"},
		Handler:     wrap("teams.list_chat_message_replies", "read", tools.NewHandleListChatMessageReplies(rc, c.timeout)),
		Annotations: teamsReadAnnotations(),
		Schema: []mcp.ToolOption{
			mcp.WithString("chat_id", mcp.Required(), mcp.Description(teamsChatIDDescription)),
			mcp.WithString("message_id", mcp.Required(), mcp.Description("The identifier of the parent message whose replies are read.")),
			mcp.WithString("account", mcp.Description(teamsAccountDescription)),
			teamsOutputParam(),
		},
	}
}

// buildListChannelMessagesVerb constructs the list_channel_messages Verb.
func buildListChannelMessagesVerb(c teamsVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "list_channel_messages",
		Summary:     "list the top-level posts in one channel",
		Description: "Lists the thread openers in one Teams channel; the answers under each are reached through list_channel_message_replies, so a caller scanning a channel is not handed every reply in the team at once. A channel is addressed by both identifiers together, obtained from a search hit. Requires team_id and channel_id; optional account and output ('text' by default, 'summary', or 'raw'). " + teamsReadClosing,
		Examples: []tools.Example{
			{Args: map[string]any{"team_id": "b1c2...", "channel_id": "19:def...@thread.tacv2"}, Comment: "scan a channel's threads"},
		},
		SeeDocs:     []string{"concepts#output-tiers", "concepts#teams-gating", "troubleshooting#teams-channel-identifiers", "troubleshooting#teams-disabled"},
		Handler:     wrap("teams.list_channel_messages", "read", tools.NewHandleListChannelMessages(rc, c.timeout)),
		Annotations: teamsReadAnnotations(),
		Schema: []mcp.ToolOption{
			mcp.WithString("team_id", mcp.Required(), mcp.Description(teamsTeamIDDescription)),
			mcp.WithString("channel_id", mcp.Required(), mcp.Description(teamsChannelIDDescription)),
			mcp.WithString("account", mcp.Description(teamsAccountDescription)),
			teamsOutputParam(),
		},
	}
}

// buildGetChannelMessageVerb constructs the get_channel_message Verb.
func buildGetChannelMessageVerb(c teamsVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "get_channel_message",
		Summary:     "get one channel post; the full body requires output=raw",
		Description: "Fetches one post in a Teams channel. A channel post is arbitrarily long and commonly HTML, so the default tier returns a body preview and the full body is returned only under output=raw; decide from the preview whether the full fetch is warranted. Requires team_id, channel_id, and message_id; optional account and output ('text' by default, 'summary', or 'raw'). " + teamsReadClosing,
		Examples: []tools.Example{
			{Args: map[string]any{"team_id": "b1c2...", "channel_id": "19:def...@thread.tacv2", "message_id": "1700000000000"}, Comment: "read the post behind a search hit"},
			{Args: map[string]any{"team_id": "b1c2...", "channel_id": "19:def...@thread.tacv2", "message_id": "1700000000000", "output": "raw"}, Comment: "escalate to the full post body"},
		},
		SeeDocs:     []string{"concepts#output-tiers", "concepts#teams-gating", "troubleshooting#teams-channel-identifiers", "troubleshooting#teams-disabled"},
		Handler:     wrap("teams.get_channel_message", "read", tools.NewHandleGetChannelMessage(rc, c.timeout)),
		Annotations: teamsReadAnnotations(),
		Schema: []mcp.ToolOption{
			mcp.WithString("team_id", mcp.Required(), mcp.Description(teamsTeamIDDescription)),
			mcp.WithString("channel_id", mcp.Required(), mcp.Description(teamsChannelIDDescription)),
			mcp.WithString("message_id", mcp.Required(), mcp.Description("The message identifier, as carried by a list_channel_messages entry or a search hit.")),
			mcp.WithString("account", mcp.Description(teamsAccountDescription)),
			teamsOutputParam(),
		},
	}
}

// buildListChannelMessageRepliesVerb constructs the
// list_channel_message_replies Verb.
func buildListChannelMessageRepliesVerb(c teamsVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "list_channel_message_replies",
		Summary:     "list the replies hanging under one channel post",
		Description: "Lists the replies under one channel post. A channel listing returns thread openers only, so the discussion under a post is reachable only here; each reply names the parent it answers. Requires team_id, channel_id, and message_id; optional account and output ('text' by default, 'summary', or 'raw'). " + teamsReadClosing,
		Examples: []tools.Example{
			{Args: map[string]any{"team_id": "b1c2...", "channel_id": "19:def...@thread.tacv2", "message_id": "1700000000000"}, Comment: "read the discussion under a post"},
		},
		SeeDocs:     []string{"concepts#output-tiers", "concepts#teams-gating", "troubleshooting#teams-channel-identifiers", "troubleshooting#teams-disabled"},
		Handler:     wrap("teams.list_channel_message_replies", "read", tools.NewHandleListChannelMessageReplies(rc, c.timeout)),
		Annotations: teamsReadAnnotations(),
		Schema: []mcp.ToolOption{
			mcp.WithString("team_id", mcp.Required(), mcp.Description(teamsTeamIDDescription)),
			mcp.WithString("channel_id", mcp.Required(), mcp.Description(teamsChannelIDDescription)),
			mcp.WithString("message_id", mcp.Required(), mcp.Description("The identifier of the parent post whose replies are read.")),
			mcp.WithString("account", mcp.Description(teamsAccountDescription)),
			teamsOutputParam(),
		},
	}
}

// buildComposeReplyVerb constructs the compose_reply Verb, the domain's only
// non-listing verb and its only one whose name invites the reading that it
// communicates. It reads the parent message for context and returns prepared
// text; posting it is a manual action the user takes in Microsoft Teams, and no
// send scope is requested in any configuration.
//
// It declares no output parameter: it projects no Graph resource, so there is
// nothing to tier, and its prepared text is returned unconditionally.
func buildComposeReplyVerb(c teamsVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "compose_reply",
		Summary:     "prepare reply text quoting a message; nothing is sent",
		Description: "Prepares a reply to a Teams message: it reads the message being answered so the draft quotes it, then returns the prepared text. It does not send, post, or draft anything in Teams, and no Teams send scope is requested in any configuration; posting the text is a manual action the user performs in Microsoft Teams. Requires body and message_id, plus either chat_id for a chat message or both team_id and channel_id for a channel post. Optional account. Returns the prepared text unconditionally and takes no output parameter. " + teamsReadClosing,
		Examples: []tools.Example{
			{Args: map[string]any{"chat_id": "19:abc...@thread.v2", "message_id": "1700000000000", "body": "Confirmed, I will send the numbers today."}, Comment: "prepare a reply to a chat message"},
			{Args: map[string]any{"team_id": "b1c2...", "channel_id": "19:def...@thread.tacv2", "message_id": "1700000000000", "body": "Adding the release date below."}, Comment: "prepare a reply to a channel post"},
		},
		SeeDocs:     []string{"concepts#teams-gating", "troubleshooting#teams-channel-identifiers", "troubleshooting#teams-disabled"},
		Handler:     wrap("teams.compose_reply", "read", tools.NewHandleComposeReply(rc, c.timeout)),
		Annotations: teamsReadAnnotations(),
		Schema: []mcp.ToolOption{
			mcp.WithString("body", mcp.Required(), mcp.Description("The reply text to prepare. It is returned to the caller and posted nowhere.")),
			mcp.WithString("message_id", mcp.Required(), mcp.Description("The identifier of the message being answered.")),
			mcp.WithString("chat_id", mcp.Description("The chat holding the parent message. Supply this or both team_id and channel_id, never both shapes.")),
			mcp.WithString("team_id", mcp.Description("The team holding the parent post. Supply with channel_id, in place of chat_id.")),
			mcp.WithString("channel_id", mcp.Description("The channel holding the parent post. Supply with team_id, in place of chat_id.")),
			mcp.WithString("account", mcp.Description(teamsAccountDescription)),
		},
	}
}

// buildGetOnlineMeetingVerb constructs the get_online_meeting Verb, the entry
// point of the transcript chain.
func buildGetOnlineMeetingVerb(c teamsVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "get_online_meeting",
		Summary:     "resolve a meeting id or join URL to its online meeting",
		Description: "Reads one online meeting and returns the meeting-scoped identifier the transcript verbs are keyed by. A calendar event carries a join URL rather than that identifier, so a recap flow starts here with the join_web_url from calendar get_event. Requires exactly one of meeting_id or join_web_url: naming neither or both is refused before any request, because there is no unfiltered listing to fall back to. Optional account and output ('text' by default, 'summary', or 'raw'). " + teamsReadClosing,
		Examples: []tools.Example{
			{Args: map[string]any{"join_web_url": "https://teams.microsoft.com/l/meetup-join/..."}, Comment: "resolve the join URL of a calendar event"},
			{Args: map[string]any{"meeting_id": "MSpkYzE3Njc0Yy04MWQ5..."}, Comment: "read a meeting already resolved"},
		},
		SeeDocs:     []string{"concepts#output-tiers", "concepts#teams-gating", "troubleshooting#teams-meeting-unresolved", "troubleshooting#teams-disabled"},
		Handler:     wrap("teams.get_online_meeting", "read", tools.NewHandleGetOnlineMeeting(rc, c.timeout)),
		Annotations: teamsReadAnnotations(),
		Schema: []mcp.ToolOption{
			mcp.WithString("meeting_id", mcp.Description("The onlineMeeting identifier. Supply this or join_web_url, never both.")),
			mcp.WithString("join_web_url", mcp.Description("The meeting's join URL, as returned by calendar get_event. Supply this or meeting_id, never both.")),
			mcp.WithString("account", mcp.Description(teamsAccountDescription)),
			teamsOutputParam(),
		},
	}
}

// buildListTranscriptsVerb constructs the list_transcripts Verb.
func buildListTranscriptsVerb(c teamsVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "list_transcripts",
		Summary:     "list the transcript metadata one online meeting holds",
		Description: "Lists the transcripts a meeting holds, as metadata only: a meeting can hold several, each of them the full text of a call, so this verb exists to choose which one to fetch rather than to deliver any of them. Use get_online_meeting to obtain the meeting identifier. Requires meeting_id; optional account and output ('text' by default, 'summary', or 'raw'). " + teamsReadClosing,
		Examples: []tools.Example{
			{Args: map[string]any{"meeting_id": "MSpkYzE3Njc0Yy04MWQ5..."}, Comment: "see which transcripts a meeting holds"},
		},
		SeeDocs:     []string{"concepts#output-tiers", "concepts#teams-gating", "troubleshooting#teams-meeting-unresolved", "troubleshooting#teams-disabled"},
		Handler:     wrap("teams.list_transcripts", "read", tools.NewHandleListTranscripts(rc, c.timeout)),
		Annotations: teamsReadAnnotations(),
		Schema: []mcp.ToolOption{
			mcp.WithString("meeting_id", mcp.Required(), mcp.Description(teamsMeetingIDDescription)),
			mcp.WithString("account", mcp.Description(teamsAccountDescription)),
			teamsOutputParam(),
		},
	}
}

// buildGetTranscriptVerb constructs the get_transcript Verb.
func buildGetTranscriptVerb(c teamsVerbsConfig, rc graph.RetryConfig, wrap func(string, string, mcpserver.ToolHandlerFunc) tools.Handler) tools.Verb {
	return tools.Verb{
		Name:        "get_transcript",
		Summary:     "get one transcript; the full WEBVTT text requires output=raw",
		Description: "Reads one meeting transcript: its metadata and a short preview of its WEBVTT text. The full text, which can be very large, is returned only under output=raw, so decide from the preview whether the full fetch is warranted. Requires meeting_id and transcript_id; optional account and output ('text' by default, 'summary', or 'raw'). " + teamsReadClosing,
		Examples: []tools.Example{
			{Args: map[string]any{"meeting_id": "MSpkYzE3Njc0Yy04MWQ5...", "transcript_id": "VjIjIzE0..."}, Comment: "preview a transcript listed by list_transcripts"},
			{Args: map[string]any{"meeting_id": "MSpkYzE3Njc0Yy04MWQ5...", "transcript_id": "VjIjIzE0...", "output": "raw"}, Comment: "escalate to the full WEBVTT text"},
		},
		SeeDocs:     []string{"concepts#output-tiers", "concepts#teams-gating", "troubleshooting#teams-meeting-unresolved", "troubleshooting#teams-disabled"},
		Handler:     wrap("teams.get_transcript", "read", tools.NewHandleGetTranscript(rc, c.timeout)),
		Annotations: teamsReadAnnotations(),
		Schema: []mcp.ToolOption{
			mcp.WithString("meeting_id", mcp.Required(), mcp.Description(teamsMeetingIDDescription)),
			mcp.WithString("transcript_id", mcp.Required(), mcp.Description("The transcript identifier, as carried by a list_transcripts entry.")),
			mcp.WithString("account", mcp.Description(teamsAccountDescription)),
			teamsOutputParam(),
		},
	}
}
