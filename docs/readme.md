# Outlook Local MCP Server

A single-binary MCP server that connects Claude Desktop and Claude Code to Microsoft Outlook via the Microsoft Graph API. Manage your calendar, read and triage email, and compose drafts without leaving your AI assistant.

<p align="center">
  <img src="docs/assets/demo.gif" alt="outlook-local-mcp demo">
</p>

## Install

**Go binary** (recommended):

```bash
go install github.com/desek/outlook-local-mcp/cmd/outlook-local-mcp@latest
```

**Claude Desktop extension** (no terminal required):

Download the `.mcpb` file from the [latest release](https://github.com/desek/outlook-local-mcp/releases/latest) and open it in Claude Desktop via **Settings > Extensions > Install from file**.

For full setup instructions including Claude Desktop and Claude Code configuration, see [quickstart.md](quickstart.md).

## Features

- **Calendar management** -- list, search, create, update, delete, and respond to events; create and cancel meetings with attendee confirmation
- **Scheduling and availability** -- read your own busy periods with their subjects, read free/busy blocks and working hours for up to twenty mailboxes you are permitted to view, and ask Graph to propose candidate meeting slots for a set of attendees ranked by confidence. All three are reads; booking stays with the meeting verbs
- **Event attachments** -- list what is attached to an event, download one as base64, and attach a file to an existing event with `add_event_attachment`. No opt-in is required: an event attachment is calendar data, so no mail flag governs it. The transfer path is chosen for you, a single request for a small file and a chunked upload session above roughly 3 MB, and the same `MAX_ATTACHMENT_SIZE_BYTES` ceiling bounds both directions
- **Multi-account support** -- manage multiple Microsoft accounts simultaneously with per-account token isolation and lifecycle control (`add`, `remove`, `login`, `logout`, `refresh`)
- **Lazy authentication** -- authenticates on first tool call; device code, browser, and authorization code flows supported
- **Persistent token cache** -- OS-native secure storage (macOS Keychain, Linux libsecret, Windows DPAPI) with AES-256-GCM file fallback
- **Mail read access** (opt-in, `MAIL_ENABLED=true`) -- list folders, list and search messages, read conversations and attachments using KQL full-text search
- **Mail draft management** (opt-in, `MAIL_MANAGE_ENABLED=true`) -- compose new drafts, reply drafts, and forward drafts that land in Outlook Drafts for manual review and send; email is never sent automatically
- **Draft attachments** (same opt-in, `MAIL_MANAGE_ENABLED=true`) -- attach a file to a draft with `add_attachment`, supplying the bytes as base64. The transfer path is chosen for you: a single request for a small file, a chunked upload session above roughly 3 MB. The confirmation names the attachment identifier the service assigns. Only drafts accept an attachment, and the upper size bound is `MAX_ATTACHMENT_SIZE_BYTES`
- **Received-message management** (same opt-in, `MAIL_MANAGE_ENABLED=true`) -- act on a message already in the mailbox: move it to another folder, set or clear its follow-up flag, replace its categories, and mark it read or unread. A move returns the message's new identifier, because moving mints a new one and the original stops resolving
- **Contacts resolution** (opt-in, `CONTACTS_ENABLED=true`) -- turn a name into an email address before drafting mail or inviting an attendee. Registers a fifth, read-only `contacts` tool that searches saved personal contacts and relevance-ranked people together, labelling each match by its source, and fetches either kind by identifier. No contact is created, changed, or deleted, and no contact write scope is requested
- **Teams reading** (opt-in, `TEAMS_ENABLED=true`) -- search chat and channel messages together, read the threads and replies a search finds, and turn a meeting into a recap from its transcript. Registers a sixth, read-only `teams` tool. No verb sends a Teams message: nothing in the domain posts, edits, or deletes, `compose_reply` hands back prepared reply text for you to paste into Teams yourself, and no Teams send or write scope is requested in any configuration
- **Read-only mode** -- disable all writes via `READ_ONLY=true`
- **In-server documentation access** -- the LLM can look up docs and troubleshoot without leaving the session (see below)
- **MCP tool annotations** -- full annotation set for Anthropic Software Directory compliance
- **OpenTelemetry** -- optional metrics and tracing via OTLP gRPC
- **Structured and audit logging** -- JSON/text format with PII sanitization and per-invocation audit trail

## Tool invocation shape (v0.6.0+)

All operations use four aggregate domain tools dispatched by an `operation` verb, plus two opt-in ones registered only when their variable is set, `contacts` under `OUTLOOK_MCP_CONTACTS_ENABLED` and `teams` under `OUTLOOK_MCP_TEAMS_ENABLED`:

```
{tool: "calendar", args: {operation: "list_events", date: "today"}}
{tool: "mail",     args: {operation: "list_folders"}}
{tool: "account",  args: {operation: "list"}}
{tool: "system",   args: {operation: "status"}}
{tool: "contacts", args: {operation: "search", query: "alex"}}   // OUTLOOK_MCP_CONTACTS_ENABLED
{tool: "teams",    args: {operation: "search", query: "release checklist"}}   // OUTLOOK_MCP_TEAMS_ENABLED
```

Call any domain with `operation: "help"` to list its verbs and parameters:

```
{tool: "system", args: {operation: "help"}}
```

## In-server documentation access

The server embeds its own documentation as four user-facing files: `readme`, `quickstart`, `concepts`, and `troubleshooting` (see `docs/embed.go`). The LLM can search and retrieve them directly:

```
{tool: "system", args: {operation: "list_docs"}}
{tool: "system", args: {operation: "search_docs", query: "token refresh"}}
{tool: "system", args: {operation: "get_docs", slug: "troubleshooting", section: "keychain-locked"}}
```

Each document is also exposed as an MCP resource at `doc://outlook-local-mcp/{slug}` for clients that support `resources/list` and `resources/read`. The server entry point (`system.status`) includes a `docs` section with the base URI and troubleshooting slug.

## For LLM clients

If you are an LLM consuming this server, see [llms.txt](llms.txt) for a machine-readable index of all documentation, tools, and change requests with absolute GitHub links.

## Configuration

All settings use environment variables prefixed with `OUTLOOK_MCP_`. Key variables:

| Variable | Default | Description |
|---|---|---|
| `CLIENT_ID` | `outlook-desktop` | OAuth client ID (well-known name or UUID) |
| `TENANT_ID` | `common` | Entra ID tenant |
| `AUTH_METHOD` | *(inferred)* | `device_code`, `browser`, or `auth_code` |
| `DEFAULT_TIMEZONE` | `auto` | IANA timezone for calendar operations |
| `TOKEN_STORAGE` | `auto` | `auto`, `keychain`, or `file` |
| `READ_ONLY` | `false` | Disable write operations |
| `MAIL_ENABLED` | `false` | Enable read-only mail access |
| `MAIL_MANAGE_ENABLED` | `false` | Enable draft management, draft attachments, and received-message management (implies `MAIL_ENABLED`) |
| `CONTACTS_ENABLED` | `false` | Register the opt-in `contacts` tool for read-only name-to-address resolution; requests `Contacts.Read` and `People.Read` |
| `TEAMS_ENABLED` | `false` | Register the opt-in `teams` tool for read-only Teams message and transcript access; requests `Chat.Read`, `ChannelMessage.Read.All`, `OnlineMeetings.Read`, and `OnlineMeetingTranscript.Read.All`. No Teams send scope is ever requested |
| `MAX_ATTACHMENT_SIZE_BYTES` | `10485760` | Attachment size ceiling for every direction and both domains: `get_attachment`, `add_attachment`, `get_event_attachment`, and `add_event_attachment` |
| `LOG_LEVEL` | `warn` | `debug`, `info`, `warn`, `error` |
| `LOG_FILE` | *(disabled)* | File path for persistent log output |

Full configuration reference is in [docs/reference/architecture.md](docs/reference/architecture.md#configuration).

## Troubleshooting

Common issues including authentication failures, token refresh, Keychain errors, Graph throttling, and mail flag configuration are covered in [docs/troubleshooting.md](docs/troubleshooting.md). The LLM can retrieve this guide directly with `{tool: "system", args: {operation: "get_docs", slug: "troubleshooting"}}`.

## Contributing

Contributions follow the project's governance process. Architecture decisions and feature changes are proposed as Change Requests under `docs/cr/`. Run `make ci` before submitting a pull request.

## License

MIT License. See [LICENSE](LICENSE) for details.
