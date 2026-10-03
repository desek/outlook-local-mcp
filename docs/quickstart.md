# Quick Start

Get from zero to a working Outlook Local MCP server in minutes.

## Prerequisites

- **Go 1.25+** installed ([download](https://go.dev/dl/))
- A **Microsoft account** (personal, work, or school)

## 1. Build

```bash
git clone https://github.com/desek/outlook-local-mcp.git
cd outlook-local-mcp
go build ./cmd/outlook-local-mcp/
```

Or install directly:

```bash
go install github.com/desek/outlook-local-mcp/cmd/outlook-local-mcp@latest
```

## 2. Configure Claude Desktop

Add the server to your Claude Desktop configuration file:

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
**Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "outlook-local": {
      "command": "/absolute/path/to/outlook-local-mcp"
    }
  }
}
```

Replace `/absolute/path/to/outlook-local-mcp` with the actual path to the built binary.

To set environment variables:

```json
{
  "mcpServers": {
    "outlook-local": {
      "command": "/absolute/path/to/outlook-local-mcp",
      "env": {
        "OUTLOOK_MCP_DEFAULT_TIMEZONE": "America/New_York",
        "OUTLOOK_MCP_LOG_LEVEL": "info"
      }
    }
  }
}
```

## 2b. Configure Claude Code

Add an `.mcp.json` file to your project root:

```json
{
  "mcpServers": {
    "outlook-local": {
      "command": "/absolute/path/to/outlook-local-mcp"
    }
  }
}
```

Replace `/absolute/path/to/outlook-local-mcp` with the actual path to the built binary.

## 2c. Install as a Claude Code or Cowork plugin

Install the plugin from the Claude directory with `/plugin` in Claude Code. From a checkout of this repository, run `claude --plugin-dir ./plugin`.

The plugin launcher does these steps on first start of a version:

1. It downloads the raw release binary that matches the plugin version.
2. It verifies the SHA-256 digest of the binary.
3. It keeps the binary under `${CLAUDE_PLUGIN_DATA}`.
4. It starts the binary.

A later start uses the kept binary and makes no network call. Set `OUTLOOK_MCP_PLUGIN_BIN` to the path of a local binary to skip the download and the verification.

The plugin supports macOS arm64 and Linux amd64 only. On Windows, use the Claude Desktop extension (`.mcpb`) or the release zip.

At install, the plugin asks for the same options as the Claude Desktop extension. Each answer sets one variable: client ID (`OUTLOOK_MCP_CLIENT_ID`), tenant ID (`OUTLOOK_MCP_TENANT_ID`), authentication method (`OUTLOOK_MCP_AUTH_METHOD`), and timezone (`OUTLOOK_MCP_DEFAULT_TIMEZONE`).

## 3. Authenticate and Verify

Restart Claude Desktop (or reload MCP servers in Claude Code) and verify the server is reachable:

```
{tool: "system", args: {operation: "about"}}
```

This returns the build version, Go runtime, OS, and auth backend in use — no authentication required. Keep this output handy when reporting issues (see [Before you file an issue](troubleshooting#before-you-file-an-issue)).

Then ask:

> "List my calendars"

On first use, the server has no cached credentials. The default authentication method (`auth_code`) opens the system browser for Microsoft login. After signing in, the browser shows a blank page -- copy the full URL from the address bar and paste it when prompted (via MCP Elicitation) or use the `complete_auth` tool if your client does not support elicitation (e.g., Claude Code). After authentication completes, the tool call is retried automatically and your calendars are returned. Tokens are cached in your OS keychain -- subsequent requests authenticate silently.

## 4. Tool Examples

### Read

**List calendars** -- no parameters required:
> "Show me all my calendars"

**List events** in a time range:
> "What meetings do I have tomorrow?"

Parameters: `start_datetime` (required), `end_datetime` (required), `calendar_id`, `max_results`, `timezone`.

**Get event** details by ID:
> "Get the full details of event AAMkAD..."

Parameters: `event_id` (required), `timezone`.

### Search

**Search events** by subject, importance, sensitivity, and more:
> "Find all high-importance meetings in the next two weeks"

Parameters: `query`, `start_datetime`, `end_datetime`, `importance`, `sensitivity`, `is_all_day`, `show_as`, `is_cancelled`, `categories`, `max_results`, `timezone`. All optional; defaults to next 30 days.

**Free/busy** availability on your own calendar, with the subject of each busy period:
> "When am I free next Monday?"

Parameters: `start_datetime` (required), `end_datetime` (required), `timezone`.

**Schedule** -- free/busy blocks and working hours for one or more mailboxes you are permitted to view:
> "Is Alice free tomorrow afternoon, and when does she work?"

Parameters: `schedules` (required, comma-separated SMTP addresses, at most 20 per call), a resolvable window supplied either as `date` or as `start_datetime` and `end_datetime` (the explicit datetimes take precedence), `availability_view_interval`, `timezone`. A mailbox you may not view is reported as an error against that mailbox while the others still return their blocks -- see [A mailbox reports an error inside a schedule reply](troubleshooting#schedule-mailbox-error).

**Find meeting times** -- candidate slots for a set of attendees, ranked by confidence:
> "Find a 30-minute slot next week for alice@example.com and bob@example.com"

Parameters: `attendees` (required, JSON array), `meeting_duration` (ISO 8601, defaults to `PT30M`), `start_datetime` and `end_datetime` (supply both to bound the search, or omit both to let Graph choose the window), `max_candidates`, `minimum_attendee_percentage`, `is_organizer_optional`, `timezone`. This verb only suggests; booking a suggested slot is a separate `create_meeting` call.

### Write

**Create event**:
> "Schedule a team standup tomorrow at 9 AM Eastern for 30 minutes with alice@example.com"

Required: `subject`, `start_datetime`, `start_timezone`, `end_datetime`, `end_timezone`.
Optional: `body`, `location`, `attendees` (JSON array), `is_online_meeting`, `is_all_day`, `importance`, `sensitivity`, `show_as`, `categories`, `recurrence` (JSON object), `reminder_minutes`, `calendar_id`.

**Update event** -- only specified fields change (PATCH semantics):
> "Move my 2pm meeting to 3pm"

Required: `event_id`. All other fields are optional.

### Attachments

The files clipped to an event are reachable by the same `event_id` the reads above return. No opt-in flag governs them: an event attachment is calendar data.

**List event attachments** -- metadata only, no file content:
> "What is attached to the design review invite?"

Parameters: `event_id` (required), `account`, `output`.

**Get event attachment** -- the file's bytes, base64 encoded:
> "Read the agenda attached to that meeting"

Parameters: `event_id` (required), `attachment_id` (required, from the list above), `account`, `output`. The content is returned only within `OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES` (10 MB by default); a larger attachment is refused rather than truncated.

**Add event attachment** -- attach a file to an existing event:
> "Attach this PDF to tomorrow's team meeting"

Required: `event_id`, `name`, `content_bytes` (base64). Optional: `mime_type` (defaults to `application/octet-stream`), `account`. A small file goes in one request and a file above roughly 3 MB through a chunked upload session; the confirmation names the path used, the attachment identifier the service assigned, and the size measured after decoding. Attaching a file does not notify attendees. This is a write verb, so read-only mode refuses it -- see [Attachment upload did not complete](troubleshooting#attachment-upload-did-not-complete) if a large file fails to land.

### Delete

**Delete event**:
> "Delete the event AAMkAD..."

Parameters: `event_id` (required). Cancellation notices are sent to attendees automatically if you are the organizer.

**Cancel event** with a message to attendees:
> "Cancel tomorrow's team meeting and let everyone know it's rescheduled"

Parameters: `event_id` (required), `comment` (optional cancellation message). Only the organizer can cancel.

### Contacts (opt-in)

The verbs above take email addresses, never names. The `contacts` tool supplies the address, and it is one of the two tools that are absent unless asked for: set `OUTLOOK_MCP_CONTACTS_ENABLED=true` in the server's environment and restart the client.

```json
{
  "mcpServers": {
    "outlook-local": {
      "command": "/absolute/path/to/outlook-local-mcp",
      "env": {
        "OUTLOOK_MCP_CONTACTS_ENABLED": "true"
      }
    }
  }
}
```

Enabling it adds the `Contacts.Read` and `People.Read` scopes, so the next tool call re-runs the sign-in flow once for incremental consent -- see [Contacts consent prompt on first use](troubleshooting#contacts-consent). If the tool does not appear, see [Contacts tool not listed](troubleshooting#contacts-disabled).

**Search** across saved contacts and relevance-ranked people at once:
> "What is Alex's email address?"

Parameters: `query` (required), `account`, `output`. Each match is labelled with its source, a contact you saved or a person Graph inferred from your correspondence, so you can judge how much to trust it.

**Get contact** and **get person** fetch one full record by the identifier a search returned:
> "Show me the full contact record for that Alex"

Parameters: `contact_id` or `person_id` (required), `account`, `output`.

**List people** returns your correspondents in Graph's relevance order, most relevant first:
> "Who do I email most?"

Parameters: `account`, `output`.

Every contacts verb reads. Nothing in the domain creates, changes, or deletes a contact, and no contact write scope is ever requested. See [Contacts gating](concepts#contacts-gating).

### Teams (opt-in)

The `teams` tool reads Microsoft Teams conversations and meeting transcripts, and it is the other tool absent unless asked for: set `OUTLOOK_MCP_TEAMS_ENABLED=true` in the server's environment and restart the client.

```json
{
  "mcpServers": {
    "outlook-local": {
      "command": "/absolute/path/to/outlook-local-mcp",
      "env": {
        "OUTLOOK_MCP_TEAMS_ENABLED": "true"
      }
    }
  }
}
```

Enabling it adds the `Chat.Read`, `ChannelMessage.Read.All`, `OnlineMeetings.Read`, and `OnlineMeetingTranscript.Read.All` scopes, so the next tool call re-runs the sign-in flow once for incremental consent. If the tool does not appear, see [Teams tool not listed](troubleshooting#teams-disabled).

**Search** is where every Teams flow starts, because it is what turns a topic into the identifiers the other verbs need:
> "Find the Teams messages about the release checklist"

Parameters: `query` (required), `account`, `output`. Each hit names the chat, or the team and channel, it came from. Enumerating teams and channels is not offered; a channel is reached this way. See [Teams channel read is missing an identifier](troubleshooting#teams-channel-identifiers).

**Read a thread** with `list_chats`, `list_chat_messages`, and `list_channel_messages`, then escalate one message with `get_chat_message` or `get_channel_message`. Replies to a channel post are listed separately, by `list_channel_message_replies`. Each listing reads one page; pass `max_results` to bound it. A message body is returned as a preview by default; pass `output: "raw"` for the whole thing.

**Recap a meeting** by resolving the join URL a calendar event carries:
> "Summarise yesterday's project sync from its Teams transcript"

`get_online_meeting` takes exactly one of `meeting_id` or `join_web_url` and returns the meeting-scoped identifier `list_transcripts` and `get_transcript` are keyed by. The full WEBVTT text comes back only under `output: "raw"`. See [Teams meeting or transcript not resolved](troubleshooting#teams-meeting-unresolved).

**Compose a reply** without sending one:
> "Draft a reply to that message quoting what Sam asked"

Parameters: `body` (required, the reply text), `message_id` (required), plus `chat_id` for a chat message or both `team_id` and `channel_id` for a channel post, and optional `account`. It takes no `output` parameter, because it projects no Graph resource.

`compose_reply` reads the parent message, quotes it, and returns prepared text. It posts nothing: no Teams send scope is requested in any configuration, so pasting the reply into Microsoft Teams is a step the user takes. Every other Teams verb reads. See [Teams gating](concepts#teams-gating).

## 5. Configuration

All environment variables are prefixed with `OUTLOOK_MCP_`:

| Variable | Default | Description |
|---|---|---|
| `CLIENT_ID` | Microsoft Office client ID | OAuth 2.0 client ID |
| `TENANT_ID` | `common` | Entra ID tenant (`common`, `organizations`, `consumers`, or a GUID) |
| `DEFAULT_TIMEZONE` | `UTC` | IANA timezone for calendar operations |
| `LOG_LEVEL` | `warn` | Log level: `debug`, `info`, `warn`, `error` |
| `READ_ONLY` | `false` | Disable write tools (create, update, delete, cancel) |
| `CONTACTS_ENABLED` | `false` | Register the opt-in read-only `contacts` tool; requests `Contacts.Read` and `People.Read` |
| `TEAMS_ENABLED` | `false` | Register the opt-in read-only `teams` tool; requests `Chat.Read`, `ChannelMessage.Read.All`, `OnlineMeetings.Read`, and `OnlineMeetingTranscript.Read.All` |
| `LOG_FORMAT` | `json` | Log format: `json` or `text` |
| `LOG_SANITIZE` | `true` | Mask PII in log output |
| `LOG_FILE` | *(empty = disabled)* | Log file path for persistent file output |
| `ACCOUNTS_PATH` | `~/.outlook-local-mcp/accounts.json` | Path to the persistent accounts file for multi-account support (see CR-0032) |

## Getting help in-session

The server embeds its own documentation so the LLM can look up answers without leaving the conversation.

List available documents:

```
{tool: "system", args: {operation: "list_docs"}}
```

Search across all embedded docs:

```
{tool: "system", args: {operation: "search_docs", query: "token refresh"}}
```

Fetch a document or a specific section by heading anchor:

```
{tool: "system", args: {operation: "get_docs", slug: "troubleshooting"}}
{tool: "system", args: {operation: "get_docs", slug: "troubleshooting", section: "keychain-locked"}}
```

The embedded bundle contains `readme`, `quickstart`, `concepts`, and `troubleshooting`. Each document is also exposed as an MCP resource at `doc://outlook-local-mcp/{slug}` for clients that support `resources/list` and `resources/read`. Run `system.status` to discover the base URI and the troubleshooting slug.

## Container deployment {#container-deployment}

The server is available as an OCI image at `ghcr.io/desek/outlook-local-mcp`. No Go toolchain is required.

### Recommended invocation

```bash
docker run -i --rm \
  -v outlook-mcp-auth:/data/auth \
  -e OUTLOOK_MCP_TENANT_ID=<tenant> \
  -e OUTLOOK_MCP_CLIENT_ID=<client> \
  ghcr.io/desek/outlook-local-mcp:latest
```

The named volume `outlook-mcp-auth` persists the token cache across container restarts so the device-code or browser auth flow does not repeat on every session.

### Claude Desktop / generic MCP client config

```json
{
  "mcpServers": {
    "outlook-local": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-v", "outlook-mcp-auth:/data/auth",
        "-e", "OUTLOOK_MCP_TENANT_ID",
        "-e", "OUTLOOK_MCP_CLIENT_ID",
        "ghcr.io/desek/outlook-local-mcp:latest"
      ],
      "env": {
        "OUTLOOK_MCP_TENANT_ID": "your-tenant-id",
        "OUTLOOK_MCP_CLIENT_ID": "your-client-id"
      }
    }
  }
}
```

### Non-root variant (distroless)

For deployment targets that enforce non-root containers (Kubernetes PSA `restricted`, OpenShift, hardened CI), use the `:distroless` tag:

```json
{
  "mcpServers": {
    "outlook-local": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "--user", "65532:65532",
        "-v", "outlook-mcp-auth:/data/auth",
        "-e", "OUTLOOK_MCP_TENANT_ID",
        "-e", "OUTLOOK_MCP_CLIENT_ID",
        "ghcr.io/desek/outlook-local-mcp:distroless"
      ],
      "env": {
        "OUTLOOK_MCP_TENANT_ID": "your-tenant-id",
        "OUTLOOK_MCP_CLIENT_ID": "your-client-id"
      }
    }
  }
}
```

For more on image variants and the keychain trade-off, see [Container runtime](concepts#container-runtime).

## Further Reading

See [README.md](README.md) for the full reference documentation.
