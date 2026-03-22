# Quick Start

Get from zero to a working Outlook Local MCP server in minutes.

## Prerequisites

- **Go 1.24+** installed ([download](https://go.dev/dl/))
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

## 3. Authenticate and Verify

Restart Claude Desktop (or reload MCP servers in Claude Code) and ask:

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

**Free/busy** availability:
> "When am I free next Monday?"

Parameters: `start_datetime` (required), `end_datetime` (required), `timezone`.

### Write

**Create event**:
> "Schedule a team standup tomorrow at 9 AM Eastern for 30 minutes with alice@example.com"

Required: `subject`, `start_datetime`, `start_timezone`, `end_datetime`, `end_timezone`.
Optional: `body`, `location`, `attendees` (JSON array), `is_online_meeting`, `is_all_day`, `importance`, `sensitivity`, `show_as`, `categories`, `recurrence` (JSON object), `reminder_minutes`, `calendar_id`.

**Update event** -- only specified fields change (PATCH semantics):
> "Move my 2pm meeting to 3pm"

Required: `event_id`. All other fields are optional.

### Delete

**Delete event**:
> "Delete the event AAMkAD..."

Parameters: `event_id` (required). Cancellation notices are sent to attendees automatically if you are the organizer.

**Cancel event** with a message to attendees:
> "Cancel tomorrow's team meeting and let everyone know it's rescheduled"

Parameters: `event_id` (required), `comment` (optional cancellation message). Only the organizer can cancel.

## 5. Configuration

All environment variables are prefixed with `OUTLOOK_MCP_`:

| Variable | Default | Description |
|---|---|---|
| `CLIENT_ID` | Microsoft Office client ID | OAuth 2.0 client ID |
| `TENANT_ID` | `common` | Azure AD tenant (`common`, `organizations`, `consumers`, or a GUID) |
| `DEFAULT_TIMEZONE` | `UTC` | IANA timezone for calendar operations |
| `LOG_LEVEL` | `warn` | Log level: `debug`, `info`, `warn`, `error` |
| `READ_ONLY` | `false` | Disable write tools (create, update, delete, cancel) |
| `LOG_FORMAT` | `json` | Log format: `json` or `text` |
| `LOG_SANITIZE` | `true` | Mask PII in log output |
| `LOG_FILE` | *(empty = disabled)* | Log file path for persistent file output |
| `ACCOUNTS_PATH` | `~/.outlook-local-mcp/accounts.json` | Path to the persistent accounts file for multi-account support (see CR-0032) |

## Further Reading

See [README.md](README.md) for the full reference documentation.
