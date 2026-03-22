# Outlook Local MCP Server

A Model Context Protocol (MCP) server that provides calendar management tools backed by the Microsoft Graph API v1.0. Built as a single Go binary, it communicates over stdio (JSON-RPC) and authenticates using the OAuth 2.0 device code flow by default -- no Azure AD app registration required.

## Features

- **Up to 20 MCP tools** -- 11 calendar tools (list, get, search, free/busy, create, update, delete, cancel, respond, reschedule), 4 mail tools (list folders, list messages, search messages, get message; opt-in via `OUTLOOK_MCP_MAIL_ENABLED`), 3 account management tools (add, list, remove), 1 diagnostic tool (`status`), plus `complete_auth` (registered when using `auth_code` method). Without mail: 16 tools (15 without `complete_auth`)
- **Multi-account support** -- manage multiple Microsoft accounts simultaneously with per-account token isolation; accounts persist across server restarts via `accounts.json` (see CR-0025, CR-0032)
- **Lazy authentication** -- authenticates on first tool call, not at startup; device code flow (default) displays a URL and code for simple sign-in (see CR-0034). Smart auth method defaulting infers the best method based on client ID (see CR-0034)
- **Persistent token cache** -- OS-native secure storage (macOS Keychain, Linux libsecret, Windows DPAPI) for desktop release builds (CGo-enabled); AES-256-GCM encrypted file cache (`~/.outlook-local-mcp/token_cache.bin`) as fallback or for container builds. Configurable via `OUTLOOK_MCP_TOKEN_STORAGE` (`auto`, `keychain`, `file`). See CR-0037, CR-0038
- **Read-only mode** -- toggle to disable all write operations
- **Structured logging** -- JSON or text format with configurable levels, PII sanitization, and optional file output
- **Audit logging** -- structured audit trail for every tool invocation
- **OpenTelemetry** -- optional metrics and tracing via OTLP gRPC
- **Event provenance tagging** -- every event created by the MCP server is tagged with a hidden extended property, enabling reliable identification and filtering of MCP-created events via `created_by_mcp` on `calendar_search_events`. Invisible in Outlook UI. Configurable via `OUTLOOK_MCP_PROVENANCE_TAG`; set to empty to disable (see CR-0040)
- **Event quality guardrails** -- tool descriptions guide the LLM to provide body and location when creating meetings with attendees; a response `_advisory` field prompts follow-up when these are missing (see CR-0039)
- **Mail read access** -- opt-in read-only email access via `OUTLOOK_MCP_MAIL_ENABLED=true`. Four mail tools (`mail_list_folders`, `mail_list_messages`, `mail_search_messages`, `mail_get_message`) enable finding emails related to calendar events using KQL full-text search, OData filtering by conversation ID/sender/date, and full message retrieval. Adds `Mail.Read` OAuth scope only when enabled. Default: disabled (see CR-0043)
- **MCP tool annotations** -- all tools include complete MCP annotations (`title`, `readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint`) for Anthropic Software Directory compliance. Clients can auto-approve read-only tools, prompt for confirmation on destructive operations, and display human-readable titles (see CR-0052)
- **Response filtering** -- text mode (default) returns token-efficient plain text for LLM consumption; summary mode returns compact JSON; raw mode preserves full Graph API data (see CR-0033, CR-0042, CR-0051)
- **Well-known client IDs** -- configure `OUTLOOK_MCP_CLIENT_ID` by friendly name (e.g., `outlook-desktop`, `teams-web`) instead of raw UUID (see CR-0033)
- **Automatic retry** -- exponential backoff for transient Graph API errors (429, 503, 504)
- **Configurable timeouts** -- per-request and graceful shutdown timeouts
- **Input validation** -- parameter validation and OData injection protection
- **Graceful shutdown** -- handles SIGINT/SIGTERM with configurable drain timeout

## Project Structure

```
outlook-mcp/
  cmd/
    outlook-local-mcp/
      main.go                  # Entry point
  internal/
    config/                    # Configuration
    auth/                      # Authentication
    logging/                   # Structured logging
    audit/                     # Audit logging
    graph/                     # Graph API utilities
    validate/                  # Input validation
    observability/             # OpenTelemetry
    server/                    # MCP server setup
    tools/                     # MCP tool handlers
  docs/
```

## Prerequisites

- **Go 1.24+**
- **Microsoft account** -- personal (Outlook.com) or work/school (Microsoft 365)
- A web browser for the one-time authentication (device code flow: visit a URL and enter a code)

## Installation

### Install with `go install`

```bash
go install github.com/desek/outlook-local-mcp/cmd/outlook-local-mcp@latest
```

### Build from source

```bash
git clone https://github.com/desek/outlook-local-mcp.git
cd outlook-local-mcp
go build ./cmd/outlook-local-mcp/
```

### Build with version injection

```bash
go build -ldflags="-X main.version=1.0.0" ./cmd/outlook-local-mcp/
```

### Docker image

Pre-built multi-architecture Docker images (`linux/amd64`, `linux/arm64`) are published to GitHub Container Registry on every release:

```bash
docker pull ghcr.io/desek/outlook-local-mcp:latest
```

The image is built from `scratch` (no OS, no shell) with `CGO_ENABLED=0` and contains only the statically linked binary and CA certificates. This minimizes image size (< 20 MB) and attack surface, but means `docker exec ... /bin/sh` is not available for debugging and the OS keychain is not available -- tokens are always stored in the file-based AES-256-GCM encrypted cache. The `OUTLOOK_MCP_AUTH_RECORD_PATH` environment variable defaults to `/data/auth/auth_record.json` inside the container; mount a volume at `/data/auth` to persist authentication state across restarts.

See CR-0036 for details on the GoReleaser-based release pipeline and Docker image builds.

### Install as Claude Desktop extension

Download the `.mcpb` file from the [latest release](https://github.com/desek/outlook-local-mcp/releases/latest) and open it in Claude Desktop (**Settings > Extensions > Install from file**). Claude Desktop will prompt for your Microsoft Graph credentials (Client ID and Tenant ID are optional -- defaults work for most users). No manual binary placement or JSON configuration required.

To build the `.mcpb` bundle locally (requires the `mcpb` CLI):

```bash
make mcpb-pack
```

See CR-0029 for full details on the MCPB extension packaging.

## Authentication

The server uses the Microsoft Office first-party client ID (`outlook-desktop`) by default, which is pre-authorized for Graph Calendar scopes in all tenants -- no admin consent required. You can also use any [well-known client ID](#well-known-client-ids) or your own Azure AD app registration. Authentication is lazy -- it is deferred until the first tool call rather than blocking at startup (see CR-0022).

Three authentication methods are available, controlled by the `OUTLOOK_MCP_AUTH_METHOD` environment variable. When not set explicitly, the method is inferred from the client ID: well-known client IDs default to `device_code`; custom app registrations default to `browser` (see CR-0034).

| Method | Value | Description |
|--------|-------|-------------|
| **Device code** (default) | `device_code` | Displays a URL and code for the user to enter manually. Works everywhere with no app registration needed. May be blocked by Conditional Access policies. See CR-0024, CR-0034. |
| **Interactive browser** | `browser` | Opens the system browser and listens on a localhost port for the OAuth callback. Requires an app registration with `http://localhost` redirect URI. Default when using a custom client ID. See CR-0024, CR-0034. |
| **Authorization code** | `auth_code` | Opens the system browser for OAuth login. The user pastes the redirect URL back via MCP Elicitation or the `complete_auth` tool. Uses PKCE for security. For headless/remote environments. See CR-0030. |

### First tool call (unauthenticated)

On first use, the server has no cached token. When the MCP client invokes any tool, the `AuthMiddleware` detects the authentication error and initiates authentication automatically:

**Device code auth (default):**

1. The server captures the device code from Azure AD and attempts to display it via MCP Elicitation.
2. If Elicitation is supported, the user sees the device code in a prompt. If not, the device code and verification URL are returned as tool result text so the user can see them directly in the chat (see CR-0031). The tool returns immediately without blocking.
3. After the user completes sign-in in their browser, calling any tool (or `account_add` again) picks up the cached token automatically.

**Browser auth:**

1. The server attempts to notify the user via MCP Elicitation or `LoggingMessageNotification`.
2. The system browser opens to the Microsoft login page. The server listens on a localhost port for the OAuth callback.
3. After sign-in, the server persists the authentication record, caches tokens in OS-native secure storage, and automatically retries the original tool call.
4. If authentication times out (e.g., the user didn't notice the browser window), the server returns a descriptive error explaining that a browser window was opened and suggesting the user retry (see CR-0031).

**Authorization code auth:**

1. The server opens the system browser to the Microsoft login page with a PKCE challenge.
2. The user signs in and is redirected to the `nativeclient` redirect URI. The authorization code appears in the browser's address bar.
3. If the MCP client supports Elicitation, a prompt asks the user to paste the redirect URL. Otherwise, the server returns the auth URL and instructions to use the `complete_auth` tool as tool result text -- this works in all MCP clients including Claude Code (see CR-0031).
4. After the code is exchanged, the server persists the account, caches tokens in OS-native secure storage, and automatically retries the original tool call.

No manual pre-authentication step is needed. The MCP client guides the user through the entire process.

### Subsequent runs

The server acquires tokens silently using the cached refresh token. No browser interaction is needed unless the refresh token expires (typically 90 days of inactivity) or the token cache is cleared. Token storage is controlled by `OUTLOOK_MCP_TOKEN_STORAGE` (default `auto`): desktop release builds (CGo-enabled) use OS-native secure storage (macOS Keychain, Linux libsecret, Windows DPAPI) and automatically fall back to the file-based cache if the keychain is unavailable. Set `file` to always use the file-based cache, or `keychain` to require the OS keychain without fallback. Container/Docker builds (non-CGo) always use the file-based cache regardless of this setting. Tokens are persisted to an AES-256-GCM encrypted file at `~/.outlook-local-mcp/token_cache.bin` with `0600` permissions when file-based storage is active (see CR-0037, CR-0038). At startup, the server performs a silent token probe (5-second timeout) to pre-authenticate accounts without triggering interactive flows; `device_code` accounts skip the probe entirely to avoid crash loops (see CR-0037). When a token does expire mid-session, the `AuthMiddleware` detects the failure and re-initiates the configured authentication flow with client-visible prompts. Error messages now include LLM-actionable recovery instructions referencing specific MCP tool names (`account_list`, `account_add`) instead of raw SDK class names (see CR-0037).

### Headless environments

Device code auth (the default) works in headless environments. No additional configuration is needed:

```bash
./outlook-local-mcp
```

### Scopes

The server requests `Calendars.ReadWrite` by default. When mail access is enabled (`OUTLOOK_MCP_MAIL_ENABLED=true`), `Mail.Read` is additionally requested. The Azure identity library automatically includes `offline_access` for refresh tokens. Enabling mail for the first time triggers an incremental consent prompt for the `Mail.Read` scope (see CR-0043).

### Middleware chain

Authentication error interception is handled by `AuthMiddleware`, the outermost middleware in the tool handler chain. The `AccountResolver` middleware resolves which account's Graph client to use for each request:

```
AuthMiddleware -> AccountResolver -> WithObservability -> ReadOnlyGuard (write tools) -> AuditWrap -> Handler
```

When authentication is valid, the middleware adds negligible overhead (a simple conditional check). When an auth error occurs, only one re-authentication attempt runs at a time -- concurrent tool calls wait for the single authentication flow to complete before retrying.

Authentication prompts use the MCP Elicitation API when supported by the client. When the client does not support elicitation (e.g., Claude Code), all authentication feedback is delivered via tool result text -- the only channel guaranteed to be visible to the user. For `auth_code`, the server returns the auth URL and `complete_auth` instructions. For `device_code`, the device code is returned directly in the tool result. For `browser`, a descriptive timeout error is returned if the user doesn't complete login in time (see CR-0025, CR-0030, CR-0031).

## Multi-Account Support

The server supports managing multiple Microsoft accounts simultaneously (see CR-0025, CR-0032). A "default" account is automatically registered at startup using the server's configured credentials. Additional accounts can be added at runtime via the `account_add` tool and are automatically persisted to `accounts.json`. On subsequent startups, persisted accounts are restored with silent token acquisition from the per-account cache. Accounts with expired tokens are still registered -- re-authentication is handled automatically by the auth middleware on first tool call.

### Account management tools

- **`account_add`** -- Register and authenticate a new Microsoft account. Accepts a required `label` and optional `client_id`, `tenant_id`, and `auth_method` parameters. Each account gets an isolated token cache partition and auth record file.
- **`account_list`** -- List all registered accounts and their authentication status.
- **`account_remove`** -- Remove a registered account by label. The "default" account cannot be removed.

### Account selection

All 11 calendar tools and 4 mail tools (when enabled) accept an optional `account` parameter to target a specific account:

- **Explicit selection:** Pass `account: "work"` to use the account with that label.
- **Single account:** When only one account is registered, it is auto-selected (no `account` parameter needed).
- **Multiple accounts:** When multiple authenticated accounts are registered and no `account` parameter is provided, the server uses the MCP Elicitation API to prompt the user to select an account. Only authenticated accounts are considered for selection (see CR-0037).
- **Elicitation unsupported or fails:** When the MCP client does not support elicitation or any elicitation error occurs, the "default" account is used as a fallback. If no default account exists, the error message lists available accounts and suggests using the `account` parameter (see CR-0037).

### MCP Elicitation requirement

Multi-account features (account selection prompts, inline authentication during `account_add`) use the MCP Elicitation API. The server declares the `elicitation` capability at startup. MCP clients that support elicitation will receive interactive prompts; clients that do not will fall back to the default account for account selection and tool result text for authentication prompts (see CR-0031). For `device_code` auth without elicitation, `account_add` uses a two-call pattern: the first call returns the device code and keeps the authentication goroutine alive in the background; the second call with the same label picks up the completed authentication and registers the account (see CR-0035).

### Per-account isolation

Each account has:
- Its own token cache partition in OS-native secure storage (`{CACHE_NAME}-{label}`) or encrypted file (`~/.outlook-local-mcp/{CACHE_NAME}-{label}.bin`), depending on the `TOKEN_STORAGE` setting and build type (see CR-0038)
- Its own auth record file (`{AUTH_RECORD_DIR}/{label}_auth_record.json`)
- Its own identity configuration persisted in `accounts.json` (label, client_id, tenant_id, auth_method)
- Its own Graph client instance

The `accounts.json` file stores only non-secret identity metadata. Tokens and credentials are managed separately by the OS-native token cache. The file is written atomically (write to temp file, then rename) to prevent corruption. Accounts persist across server restarts and are restored automatically on startup (see CR-0032).

## Configuration

All configuration is via environment variables prefixed with `OUTLOOK_MCP_`.

| Variable | Default | Description |
|---|---|---|
| `CLIENT_ID` | `outlook-desktop` | OAuth client ID. Accepts a well-known friendly name or raw UUID. See [Well-Known Client IDs](#well-known-client-ids) |
| `TENANT_ID` | `common` | Azure AD tenant: `common`, `organizations`, `consumers`, or a tenant GUID |
| `AUTH_RECORD_PATH` | `~/.outlook-local-mcp/auth_record.json` | Path to the authentication record file |
| `AUTH_METHOD` | *(inferred)* | Authentication method: `device_code` (default for well-known client IDs), `browser` (default for custom client IDs), or `auth_code` (manual). When unset, inferred from client ID. See CR-0034 |
| `ACCOUNTS_PATH` | `~/.outlook-local-mcp/accounts.json` | Path to the persistent accounts configuration file for multi-account support. Defaults to the same directory as `AUTH_RECORD_PATH`. See CR-0032 |
| `TOKEN_STORAGE` | `auto` | Token storage backend: `auto` (OS keychain with file fallback), `keychain` (OS keychain only), `file` (file-based only). See CR-0038 |
| `CACHE_NAME` | `outlook-local-mcp` | Token cache partition name in OS secure storage |
| `DEFAULT_TIMEZONE` | `auto` | Default IANA timezone for calendar operations. `auto` detects from system using: `time.Now().Location()`, then `TZ` env var, then `/etc/localtime` (macOS) or `/etc/timezone` (Linux), then UTC fallback. See CR-0034, CR-0037 |
| `LOG_LEVEL` | `warn` | Log level: `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `json` | Log format: `json`, `text` |
| `LOG_SANITIZE` | `true` | Mask PII (emails, body content) in log output |
| `LOG_FILE` | *(empty = disabled)* | Log file path for persistent file output (see CR-0023) |
| `MAX_RETRIES` | `3` | Max retry attempts for transient errors (0-10) |
| `RETRY_BACKOFF_MS` | `1000` | Initial backoff in milliseconds (100-30000) |
| `REQUEST_TIMEOUT_SECONDS` | `30` | Per-request Graph API timeout (1-300) |
| `SHUTDOWN_TIMEOUT_SECONDS` | `15` | Graceful shutdown drain timeout (1-300) |
| `AUDIT_LOG_ENABLED` | `true` | Enable audit logging |
| `AUDIT_LOG_PATH` | *(empty = stderr)* | Audit log file path |
| `READ_ONLY` | `false` | Disable write/delete tools |
| `MAIL_ENABLED` | `false` | Enable read-only mail access. Adds `Mail.Read` OAuth scope and registers 4 mail tools (`mail_list_folders`, `mail_list_messages`, `mail_search_messages`, `mail_get_message`). See CR-0043 |
| `OTEL_ENABLED` | `false` | Enable OpenTelemetry metrics and tracing |
| `OTEL_ENDPOINT` | *(empty = localhost:4317)* | OTLP gRPC endpoint |
| `PROVENANCE_TAG` | `com.github.desek.outlook-local-mcp.created` | Name for the provenance extended property stamped on MCP-created events. Set to empty string to disable provenance tagging entirely. See CR-0040 |
| `OTEL_SERVICE_NAME` | `outlook-local-mcp` | OpenTelemetry service name |

## Usage

### Running directly

```bash
./outlook-local-mcp
```

The server communicates over stdin/stdout using JSON-RPC (MCP protocol). Logs are written to stderr. Authentication prompts are sent to the MCP client as `LoggingMessageNotification` messages; stderr is used as a fallback when no MCP client session is available.

### Claude Desktop configuration

Add to your Claude Desktop `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "outlook-local": {
      "command": "/path/to/outlook-local-mcp",
      "env": {
        "OUTLOOK_MCP_LOG_LEVEL": "warn",
        "OUTLOOK_MCP_DEFAULT_TIMEZONE": "America/New_York"
      }
    }
  }
}
```

### Claude Code configuration

Add to your `.mcp.json`:

```json
{
  "mcpServers": {
    "outlook-local": {
      "command": "/path/to/outlook-local-mcp",
      "env": {
        "OUTLOOK_MCP_LOG_LEVEL": "warn",
        "OUTLOOK_MCP_DEFAULT_TIMEZONE": "America/New_York"
      }
    }
  }
}
```

## Tools Reference

All calendar and mail tools accept an optional `account` parameter (string) to select which registered account to use. If omitted, the default account is used. Use `account_list` to see available accounts. When only one account is registered, it is auto-selected. When multiple authenticated accounts exist and the client supports MCP Elicitation, the server prompts for selection; otherwise, the default account is used as a fallback (see CR-0037).

### Account Management Tools

#### `account_add`

Register and authenticate a new Microsoft account. Creates a per-account credential with isolated token cache and auth record. After successful authentication and registry addition, the account's identity configuration (label, client_id, tenant_id, auth_method) is persisted to `accounts.json` for automatic restoration on server restart (see CR-0032). Authentication is performed inline using MCP Elicitation when supported. When the client does not support elicitation, the tool returns actionable instructions via tool result text: auth URL and `complete_auth` instructions for `auth_code`, the device code for `device_code`, or a descriptive timeout message for `browser` (see CR-0031). For `device_code` without elicitation, the authentication goroutine continues polling in the background; calling `account_add` again with the same label completes registration once the user has authenticated in their browser (see CR-0035).

| Parameter | Type | Required | Description |
|---|---|---|---|
| `label` | string | Yes | Unique label for the account (1-64 chars, alphanumeric/underscore/hyphen) |
| `client_id` | string | No | OAuth client ID. Defaults to the server's configured client ID |
| `tenant_id` | string | No | Azure AD tenant ID. Defaults to the server's configured tenant ID |
| `auth_method` | string | No | `auth_code`, `browser`, or `device_code`. Defaults to the server's configured auth method |

---

#### `account_list`

List all registered accounts and their authentication status.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `output` | string | No | Response format: `text` (default), `summary` for JSON, or `raw` for full response. See [Response Filtering](#response-filtering) |

---

#### `account_remove`

Remove a registered account from the server. The "default" account cannot be removed. Removes the account from both the in-memory registry and the persistent `accounts.json` file (see CR-0032). Does not delete the auth record file from disk.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `label` | string | Yes | The label of the account to remove |

---

#### `complete_auth`

Complete an in-progress authorization code authentication by providing the redirect URL from the browser. Only registered when the server is configured with `OUTLOOK_MCP_AUTH_METHOD=auth_code`. See CR-0030.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `redirect_url` | string | Yes | The full URL from the browser's address bar after Microsoft login (starts with `https://login.microsoftonline.com/common/oauth2/nativeclient?code=...`) |
| `account` | string | No | Account label. Defaults to the default account |

---

### Diagnostic Tools

#### `status`

Return server health summary including version, timezone, account authentication state, uptime, and feature flags. Full runtime configuration is available via `output=summary` or `output=raw`. Does not call the Graph API. See CR-0037, CR-0049, CR-0051.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `output` | string | No | Response format: `text` (default) shows health summary, `summary` for JSON with full config, or `raw` for complete response. See [Response Filtering](#response-filtering) |

**Response fields (via `output=summary` or `output=raw`):**

| Field | Type | Description |
|---|---|---|
| `version` | string | Server version (e.g., `1.0.0` or `dev`) |
| `timezone` | string | IANA timezone name configured for calendar operations |
| `accounts` | array | Each entry has `label` (string) and `authenticated` (bool) |
| `server_uptime_seconds` | number | Elapsed seconds since server started |
| `config` | object | Effective runtime configuration grouped into 6 categories (see below) |

**Config groups:**

| Group | Fields | Description |
|---|---|---|
| `identity` | `client_id`, `tenant_id`, `auth_method`, `auth_method_source` | OAuth identity configuration. `auth_method_source` reports `"explicit"` (env var set), `"inferred"` (well-known client ID), or `"default"` (fallback) |
| `logging` | `log_level`, `log_format`, `log_file`, `log_sanitize`, `audit_log_enabled`, `audit_log_path` | Log output configuration |
| `storage` | `token_storage`, `token_cache_backend`, `auth_record_path`, `accounts_path`, `cache_name` | Token and data persistence. `token_cache_backend` reports the actual resolved backend (`"keychain"` or `"file"`), not the configured preference |
| `graph_api` | `max_retries`, `retry_backoff_ms`, `request_timeout_seconds`, `shutdown_timeout_seconds` | Graph API client tuning |
| `features` | `read_only`, `mail_enabled`, `provenance_tag` | Feature flags and behavioral settings |
| `observability` | `otel_enabled`, `otel_endpoint`, `otel_service_name` | OpenTelemetry configuration |

---

### Read Tools

#### `calendar_list`

List all calendars accessible to the authenticated user.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `output` | string | No | Response format: `text` (default), `summary` for compact JSON, or `raw` for full Graph API response. See [Response Filtering](#response-filtering) |

---

#### `calendar_list_events`

List calendar events within a time range. Expands recurring events into individual occurrences.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `date` | string | No | Date shorthand: `today`, `tomorrow`, `this_week`, `next_week`, or ISO 8601 date (e.g., `2026-03-17`). Expands to start/end boundaries in the server's configured timezone. `this_week` and `next_week` resolve to Monday-Sunday (ISO week). When `start_datetime`/`end_datetime` are also provided, they take precedence. See CR-0037, CR-0042 |
| `start_datetime` | string | Conditional | Start of time range in ISO 8601 (e.g., `2026-03-12T00:00:00Z`). Required unless `date` is provided |
| `end_datetime` | string | Conditional | End of time range in ISO 8601 (e.g., `2026-03-13T00:00:00Z`). Required unless `date` is provided |
| `calendar_id` | string | No | Calendar ID. Omit for default calendar |
| `max_results` | number | No | Max events to return (default 25, max 100) |
| `timezone` | string | No | IANA timezone for returned times (e.g., `America/New_York`) |
| `output` | string | No | Response format: `text` (default), `summary` for compact JSON, or `raw` for full Graph API response. See [Response Filtering](#response-filtering) |

---

#### `calendar_get_event`

Get full details of a single calendar event by ID. Default output includes `bodyPreview` (plain-text snippet); full HTML body is only available via `output=raw`. In summary mode, returns subject, times, location, organizer, attendees (name and response only), and body preview. Use `output=raw` for full body, recurrence, and metadata.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `event_id` | string | Yes | The unique identifier of the event |
| `timezone` | string | No | IANA timezone for returned times |
| `output` | string | No | Response format: `text` (default), `summary` for compact JSON, or `raw` for full Graph API response. See [Response Filtering](#response-filtering) |

---

### Search Tools

#### `calendar_search_events`

Search events by subject text and properties within a time range. Defaults to the next 30 days. Subject matching uses case-insensitive substring matching (e.g., query `"budget"` matches `"Q2 Budget Review"`). See CR-0042.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `query` | string | No | Text to search for in event subjects (case-insensitive substring match) |
| `date` | string | No | Date shorthand: `today`, `tomorrow`, `this_week`, `next_week`, or ISO 8601 date (YYYY-MM-DD). Expands to start/end boundaries in the configured timezone. When `start_datetime`/`end_datetime` are also provided, they take precedence. See CR-0042 |
| `start_datetime` | string | No | Start of time range in ISO 8601. Defaults to now |
| `end_datetime` | string | No | End of time range in ISO 8601. Defaults to 30 days from start |
| `importance` | string | No | Filter: `low`, `normal`, `high` |
| `sensitivity` | string | No | Filter: `normal`, `personal`, `private`, `confidential` |
| `is_all_day` | boolean | No | Filter by all-day status |
| `show_as` | string | No | Filter: `free`, `tentative`, `busy`, `oof`, `workingElsewhere` |
| `is_cancelled` | boolean | No | Filter by cancellation status |
| `categories` | string | No | Comma-separated category names (matches any, client-side) |
| `created_by_mcp` | boolean | No | When true, only return events created by this MCP server (server-side filter on provenance extended property). Only available when provenance tagging is enabled. See CR-0040 |
| `max_results` | number | No | Max events to return (default 25, max 100) |
| `timezone` | string | No | IANA timezone for returned times |
| `output` | string | No | Response format: `text` (default), `summary` for compact JSON, or `raw` for full Graph API response. See [Response Filtering](#response-filtering) |

---

#### `calendar_get_free_busy`

Get free/busy availability for a time range. Returns busy periods where `showAs` is not `free`. Provide either `date` for day/week shorthand, or explicit `start_datetime`/`end_datetime`.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `date` | string | No | Date shorthand: `today`, `tomorrow`, `this_week`, `next_week`, or ISO 8601 date (YYYY-MM-DD). Expands to start/end boundaries in the configured timezone. When `start_datetime`/`end_datetime` are also provided, they take precedence. See CR-0042 |
| `start_datetime` | string | Conditional | Start of time range in ISO 8601. Required unless `date` is provided |
| `end_datetime` | string | Conditional | End of time range in ISO 8601. Required unless `date` is provided |
| `timezone` | string | No | IANA timezone for returned times |
| `output` | string | No | Response format: `text` (default), `summary` for compact JSON, or `raw` for full Graph API response. See [Response Filtering](#response-filtering) |

---

### Mail Tools (opt-in)

These tools are only registered when `OUTLOOK_MCP_MAIL_ENABLED=true`. All mail tools are read-only and support the `account` parameter for multi-account scenarios. See CR-0043.

#### `mail_list_folders`

List the user's mail folders (Inbox, Sent Items, Drafts, etc.) with display name, unread count, and total count.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `account` | string | No | Account label for multi-account scenarios |
| `max_results` | number | No | Maximum folders to return (default 25) |
| `output` | string | No | Response format: `text` (default), `summary` for compact JSON, or `raw` for full response. See [Response Filtering](#response-filtering) |

---

#### `mail_list_messages`

List messages in a specific mail folder or across all folders, with OData `$filter` support for date ranges, sender, and conversation threading. Use `conversation_id` to retrieve full email threads.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `folder_id` | string | No | Mail folder ID. Omit to search across all folders |
| `start_datetime` | string | No | Filter messages received on or after this ISO 8601 datetime |
| `end_datetime` | string | No | Filter messages received on or before this ISO 8601 datetime |
| `from` | string | No | Filter by sender email address |
| `conversation_id` | string | No | Filter by conversation ID to retrieve a full email thread |
| `max_results` | number | No | Maximum messages to return (default 25, max 100) |
| `timezone` | string | No | IANA timezone for returned times |
| `account` | string | No | Account label for multi-account scenarios |
| `output` | string | No | Response format: `text` (default), `summary` for compact JSON, or `raw` for full message fields. See [Response Filtering](#response-filtering) |

---

#### `mail_search_messages`

Full-text search across messages using Microsoft Graph's KQL `$search` syntax. Primary tool for finding emails related to calendar events by subject, participants, or content.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `query` | string | Yes | KQL search string. Examples: `subject:"Design Review"`, `from:alice@contoso.com`, `subject:"Sprint" AND from:alice@contoso.com`. Supports `subject:`, `from:`, `to:`, `participants:`, `received>=`, `hasAttachments:true`, and `AND`/`OR` operators |
| `folder_id` | string | No | Restrict search to a specific mail folder |
| `max_results` | number | No | Maximum messages to return (default 25, max 100) |
| `account` | string | No | Account label for multi-account scenarios |
| `output` | string | No | Response format: `text` (default), `summary` for compact JSON, or `raw` for full message fields. See [Response Filtering](#response-filtering) |

**Note:** `$search` and `$filter` cannot be combined on the messages endpoint. Results are ranked by relevance, not chronologically. For chronological listing with structured filters, use `mail_list_messages` instead.

---

#### `mail_get_message`

Retrieve details of a single message by ID. Default output includes `bodyPreview` (plain-text snippet); full HTML body and headers are only available via `output=raw`.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `message_id` | string | Yes | The unique identifier of the message |
| `account` | string | No | Account label for multi-account scenarios |
| `output` | string | No | Response format: `text` (default), `summary` for compact JSON, or `raw` for full message fields. See [Response Filtering](#response-filtering) |

---

### Write Tools

#### `calendar_create_event`

Create a new calendar event. Only `subject` and `start_datetime` are required -- timezones default to the server's configured timezone, and `end_datetime` defaults to start + 30 minutes (or + 24 hours for all-day events). Supports attendees (sends invitations automatically), Teams online meetings, recurrence, and all standard event properties. When attendees are included, always provide a body (agenda or description) and location so recipients understand the purpose and place of the meeting (see CR-0039, CR-0042).

| Parameter | Type | Required | Description |
|---|---|---|---|
| `subject` | string | Yes | Event title |
| `start_datetime` | string | Yes | Start time in ISO 8601 without offset (e.g., `2026-04-15T09:00:00`) |
| `start_timezone` | string | No | IANA timezone for start time (e.g., `America/New_York`). Defaults to the server's configured timezone when omitted. See CR-0042 |
| `end_datetime` | string | No | End time in ISO 8601 without offset. Defaults to `start_datetime` + 30 minutes (or + 24 hours when `is_all_day` is true). See CR-0042 |
| `end_timezone` | string | No | IANA timezone for end time. Defaults to the server's configured timezone when omitted. See CR-0042 |
| `body` | string | No | Event body (HTML or plain text, auto-detected). Strongly recommended when attendees are invited |
| `location` | string | No | Location display name. Strongly recommended when attendees are invited |
| `attendees` | string | No | JSON array: `[{"email":"a@b.com","name":"Name","type":"required"}]`. Type: `required`, `optional`, `resource` |
| `is_online_meeting` | boolean | No | Create a Teams online meeting (work/school accounts only) |
| `is_all_day` | boolean | No | All-day event. Start/end must be midnight in the same timezone |
| `importance` | string | No | `low`, `normal`, `high` |
| `sensitivity` | string | No | `normal`, `personal`, `private`, `confidential` |
| `show_as` | string | No | `free`, `tentative`, `busy`, `oof`, `workingElsewhere` |
| `categories` | string | No | Comma-separated category names |
| `recurrence` | string | No | JSON recurrence object (see example below) |
| `reminder_minutes` | number | No | Reminder minutes before start |
| `calendar_id` | string | No | Target calendar ID. Omit for default calendar |

**Recurrence example:**

```json
{
  "pattern": {
    "type": "weekly",
    "interval": 1,
    "daysOfWeek": ["monday"]
  },
  "range": {
    "type": "endDate",
    "startDate": "2026-04-15",
    "endDate": "2026-12-31"
  }
}
```

---

#### `calendar_update_event`

Update an existing calendar event. Only specified fields are changed (PATCH semantics). Sends update notifications to attendees if applicable. When attendees are included, always provide a body (agenda or description) and location so recipients understand the purpose and place of the meeting (see CR-0039).

| Parameter | Type | Required | Description |
|---|---|---|---|
| `event_id` | string | Yes | The unique ID of the event to update |
| `subject` | string | No | New event title |
| `start_datetime` | string | No | New start time. When provided without `start_timezone`, defaults to the server's configured timezone |
| `start_timezone` | string | No | IANA timezone for new start time. Defaults to the server's configured timezone when `start_datetime` is provided |
| `end_datetime` | string | No | New end time. When provided without `end_timezone`, defaults to the server's configured timezone |
| `end_timezone` | string | No | IANA timezone for new end time. Defaults to the server's configured timezone when `end_datetime` is provided |
| `body` | string | No | New body (HTML or plain text). Strongly recommended when attendees are invited |
| `location` | string | No | New location display name. Strongly recommended when attendees are invited |
| `attendees` | string | No | New attendees JSON array (replaces entire list) |
| `is_all_day` | boolean | No | Change all-day status |
| `importance` | string | No | `low`, `normal`, `high` |
| `sensitivity` | string | No | `normal`, `personal`, `private`, `confidential` |
| `show_as` | string | No | `free`, `tentative`, `busy`, `oof`, `workingElsewhere` |
| `categories` | string | No | Comma-separated categories (replaces all) |
| `recurrence` | string | No | New recurrence JSON, or `"null"` to remove. Series masters only |
| `reminder_minutes` | number | No | New reminder minutes before start |
| `is_reminder_on` | boolean | No | Enable or disable the reminder |

#### Attendee quality advisory

When `calendar_create_event` or `calendar_update_event` succeeds with attendees but the body or location is missing, the response JSON includes an `_advisory` field -- a plain-text hint for LLM clients suggesting they offer the user the option to add the missing information. The advisory is not present when there are no attendees, when all recommended fields are provided, or when `is_online_meeting` is set (which covers the location requirement). On `calendar_update_event`, the advisory only triggers when the `attendees` parameter is explicitly provided in the request. See CR-0039.

---

### Delete Tools

#### `calendar_delete_event`

Permanently delete a calendar event. When the organizer deletes a meeting, cancellation notices are sent automatically. Deleting a series master deletes all occurrences.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `event_id` | string | Yes | The unique identifier of the event to delete |

---

#### `calendar_cancel_event`

Cancel a meeting and send a cancellation message to all attendees. Only the organizer can cancel. For events without attendees, use `calendar_delete_event` instead.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `event_id` | string | Yes | The unique identifier of the meeting to cancel |
| `comment` | string | No | Custom cancellation message sent to attendees |

---

#### `calendar_respond_event`

Respond to a meeting invitation: accept, tentatively accept, or decline. Sends a response to the organizer. Only applicable to events where you are an attendee, not the organizer. See CR-0042.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `event_id` | string | Yes | The unique identifier of the event to respond to |
| `response` | string | Yes | Response type: `accept`, `tentative`, or `decline` |
| `comment` | string | No | Optional message to the organizer explaining your response |
| `send_response` | boolean | No | Whether to send the response to the organizer. Defaults to `true` |

---

#### `calendar_reschedule_event`

Move an event to a new time, preserving its original duration. Only the new start time is required; the end time is computed automatically from the original event's duration. Sends update notifications to attendees if applicable. Completes in at most 2 Graph API calls (GET + PATCH). See CR-0042.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `event_id` | string | Yes | The unique identifier of the event to reschedule |
| `new_start_datetime` | string | Yes | New start time in ISO 8601 without offset (e.g., `2026-04-17T14:00:00`) |
| `new_start_timezone` | string | No | IANA timezone for the new start time. Defaults to the server's configured timezone |

## Read-Only Mode

Set `OUTLOOK_MCP_READ_ONLY=true` to disable all write operations. In this mode, the write tools (`calendar_create_event`, `calendar_update_event`, `calendar_delete_event`, `calendar_cancel_event`, `calendar_respond_event`, `calendar_reschedule_event`) return an error when invoked. Read and search tools remain fully functional.

```bash
OUTLOOK_MCP_READ_ONLY=true ./outlook-local-mcp
```

## Response Filtering

By default, all read tools return token-efficient plain text optimized for LLM context consumption. Raw Graph API JSON can contain 12,000+ tokens per event (HTML bodies, Teams meeting boilerplate, inline CSS), while text mode returns roughly 150-800 tokens with the same useful information -- a 60-70% reduction. Write tools return concise text confirmations unconditionally. See CR-0033, CR-0051.

All read tools accept an `output` parameter with three modes:

- **`text`** (default) -- returns a pre-formatted plain-text string. Collections are rendered as numbered lists with human-readable fields and a total count. Detail views show labeled fields. The LLM can pass this through without additional formatting. See CR-0042, CR-0051.
- **`summary`** -- returns compact JSON with an intentionally curated field set per tool. Useful when the LLM needs structured data for programmatic reasoning. Nested objects are flattened: start/end become plain dateTime strings, organizer becomes a name string, location becomes a display name string. Summary mode includes a `displayTime` field with a pre-formatted human-readable time string (e.g., `"Wed Mar 19, 2:00 PM - 3:00 PM"` or `"Wed Mar 19 (all day)"`). See CR-0042.
- **`raw`** -- returns the full, unmodified Graph API serialization including empty values, identical to the pre-CR-0033 behavior. Use this when you need HTML body content, recurrence patterns, attendee email addresses, or other detailed fields.

Invalid values return an error: `output must be 'summary', 'raw', or 'text'`.

**Summary fields by tool:**

| Tool | Fields |
|------|--------|
| `calendar_list_events` | id, subject, start, end, displayTime, location, organizer, showAs, isOnlineMeeting |
| `calendar_get_event` | All `calendar_list_events` fields plus attendees (name + response), bodyPreview, hasAttachments, type |
| `calendar_search_events` | Same as `calendar_list_events` |
| `calendar_get_free_busy` | No difference (already compact) |
| `calendar_list` | No difference (already compact) |
| `mail_list_messages` | id, subject, bodyPreview, from, toRecipients, receivedDateTime, importance, isRead, hasAttachments, conversationId, webLink, categories, flag |
| `mail_search_messages` | Same as `mail_list_messages` |
| `mail_get_message` | All `mail_list_messages` fields plus body, ccRecipients, bccRecipients, sentDateTime, conversationIndex, internetMessageId, parentFolderId, replyTo, internetMessageHeaders |

When provenance tagging is enabled (the default), `calendar_list_events`, `calendar_get_event`, and `calendar_search_events` responses include `"createdByMcp": true` on events that were created by this MCP server. The field is omitted (not `false`) when the event was not created by the MCP. See CR-0040.

## Well-Known Client IDs

The `OUTLOOK_MCP_CLIENT_ID` environment variable accepts friendly names in addition to raw UUIDs. This simplifies configuration when using well-known Microsoft 365 application client IDs. Resolution is case-insensitive. See CR-0033.

| Friendly Name | Client ID | Application |
|---------------|-----------|-------------|
| `outlook-local-mcp` | `dd5fc5c5-eb9a-4f6f-97bd-1a9fecb277d3` | Outlook Local MCP (project app registration) |
| `teams-desktop` | `1fec8e78-bce4-4aaf-ab1b-5451cc387264` | Teams desktop & mobile |
| `teams-web` | `5e3ce6c0-2b1f-4285-8d4b-75ee78787346` | Teams web |
| `m365-web` | `4765445b-32c6-49b0-83e6-1d93765276ca` | Microsoft 365 web |
| `m365-desktop` | `0ec893e0-5785-4de6-99da-4ed124e5296c` | Microsoft 365 desktop |
| `m365-mobile` | `d3590ed6-52b3-4102-aeff-aad2292ab01c` | Microsoft 365 mobile |
| `outlook-desktop` | `d3590ed6-52b3-4102-aeff-aad2292ab01c` | Outlook desktop (default) |
| `outlook-web` | `bc59ab01-8403-45c6-8796-ac3ef710b3e3` | Outlook web |
| `outlook-mobile` | `27922004-5251-4030-b22d-91ecd9a37ea4` | Outlook mobile |

If the value is not a recognized friendly name and does not look like a UUID, a warning is logged listing the valid names and the value is used as-is.

## Observability

### Logging

Structured logs are written to stderr. Optionally, logs can also be written to a file for persistent, post-hoc analysis. Configure with:

- `OUTLOOK_MCP_LOG_LEVEL` -- minimum severity: `debug`, `info`, `warn`, `error`
- `OUTLOOK_MCP_LOG_FORMAT` -- output format: `json` (default) or `text`
- `OUTLOOK_MCP_LOG_SANITIZE` -- when `true` (default), PII such as email addresses and event body content is masked in log output
- `OUTLOOK_MCP_LOG_FILE` -- when set to a file path, log records are written to both stderr and the specified file. The file is opened in append mode with `0600` permissions. If the file cannot be opened, the server logs an error and continues with stderr-only logging. The file grows unbounded; use external log rotation (e.g., `logrotate`) for long-running deployments. Disabled by default

When `LOG_LEVEL` is set to `debug`, Graph API request URLs and response bodies are logged for each tool invocation. This is useful for troubleshooting Graph API interactions without external proxy tools. Debug-level Graph API logging respects the `LOG_SANITIZE` setting for PII masking. See CR-0033.

### Audit Logging

When `OUTLOOK_MCP_AUDIT_LOG_ENABLED=true` (default), every tool invocation emits a structured JSON audit entry containing the tool name, operation type, and outcome.

- `OUTLOOK_MCP_AUDIT_LOG_PATH` -- write audit entries to a file instead of stderr

### OpenTelemetry

Optional OTLP gRPC export for metrics and traces:

```bash
OUTLOOK_MCP_OTEL_ENABLED=true \
OUTLOOK_MCP_OTEL_ENDPOINT=localhost:4317 \
./outlook-local-mcp
```

Metrics include per-tool invocation counts and durations. Traces create a span per tool invocation with tool name, parameters, and outcome attributes.

## Development

This project uses a Makefile as the canonical command runner. See [CONTRIBUTING.md](CONTRIBUTING.md) for full development setup including pre-commit hooks.

### Build

```bash
make build
```

### Test

```bash
make test
```

### Lint

```bash
make lint
```

### Full quality check

```bash
make ci
```

### Validate GoReleaser configuration

```bash
make goreleaser-check
```

### Local release snapshot

Build cross-compiled binaries locally without publishing (replaces the former `make release-binaries` target). Outputs to `dist/`:

```bash
make snapshot
```

### Release artifacts

Tagged releases (via release-please) automatically produce: CGo-enabled desktop binaries for 4 platforms (darwin/arm64, linux/amd64, linux/arm64, windows/amd64) built on platform-native runners with OS keychain support, static container binaries (`CGO_ENABLED=0`) for Docker images, a `checksums.txt` (SHA-256), SBOMs in CycloneDX and SPDX formats, and multi-architecture Docker images pushed to `ghcr.io/desek/outlook-local-mcp`. See CR-0036, CR-0038.

## License

This project is licensed under the [MIT License](LICENSE).
