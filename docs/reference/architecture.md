# Architecture

Reference documentation for the server's component layout, middleware chain, MCP transport, tool registration, error handling, pagination, configuration, startup sequence, and Claude Desktop integration.

> **Status note:** The tool definitions in this document describe the original tool-per-operation surface that predates CR-0060. As of v0.6.0 the server exposes four aggregate domain tools (`calendar`, `mail`, `account`, `system`) dispatched by an `operation` verb. The verb-level semantics described below remain accurate; only the registration shape changed.

---

## Architecture and component overview

The server is a single Go binary that runs as a local stdio-based MCP server. It integrates three core packages:

- **mcp-go** (`github.com/mark3labs/mcp-go`) for MCP protocol server, tool registration, and stdio transport
- **azidentity** (`github.com/Azure/azure-sdk-for-go/sdk/azidentity`) for Device Code OAuth 2.0 flow with persistent token caching
- **msgraph-sdk-go** (`github.com/microsoftgraph/msgraph-sdk-go`) for the typed Go client targeting Microsoft Graph API v1.0

The runtime flow is: (1) initialize structured logging; (2) load cached authentication or prompt device code login; (3) construct a Graph client; (4) register MCP tools; (5) start the stdio server and process tool calls. The Graph client is initialized once at startup and shared across all tool handler invocations via a package-level variable. Each tool handler makes one or more Graph API calls, serializes the response to JSON, and returns it as an `mcp.CallToolResult` text content.

### Microsoft Graph API versioning

There is no Microsoft Graph API v2. The only two API versions are **v1.0** (General Availability, production-ready, strong stability guarantees) and **beta** (preview, zero stability guarantees). These correspond to the endpoints `https://graph.microsoft.com/v1.0/` and `https://graph.microsoft.com/beta/`. The common "v2" confusion stems from the separate **OAuth v2.0 endpoint** versioning (`https://login.microsoftonline.com/{tenant}/oauth2/v2.0/token`), which is unrelated to the Graph API version. This server targets Graph API **v1.0 exclusively**.

The Go SDK ecosystem reflects this split with two separate modules. There is no `/v2` suffix on any module path:

| Module | Targets | Purpose |
|---|---|---|
| `github.com/microsoftgraph/msgraph-sdk-go` | Graph API **v1.0** | This server uses this module |
| `github.com/microsoftgraph/msgraph-beta-sdk-go` | Graph API **beta** | Not used by this server |
| `github.com/microsoftgraph/msgraph-sdk-go-core` | Shared core | Pagination, middleware |

### Go module dependencies

```
github.com/mark3labs/mcp-go                          v0.45.0+
github.com/Azure/azure-sdk-for-go/sdk/azidentity     latest
github.com/Azure/azure-sdk-for-go/sdk/azidentity/cache  v0.4.x (pre-v1)
github.com/microsoftgraph/msgraph-sdk-go              v1.x
github.com/microsoftgraph/msgraph-sdk-go-core         v1.x
github.com/microsoft/kiota-authentication-azure-go    latest
github.com/microsoft/kiota-abstractions-go            latest
```

### Key import paths

```go
import (
    "log/slog"
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
    "github.com/Azure/azure-sdk-for-go/sdk/azidentity"
    "github.com/Azure/azure-sdk-for-go/sdk/azidentity/cache"
    msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
    "github.com/microsoftgraph/msgraph-sdk-go/models"
    "github.com/microsoftgraph/msgraph-sdk-go/models/odataerrors"
    graphusers "github.com/microsoftgraph/msgraph-sdk-go/users"
    msgraphcore "github.com/microsoftgraph/msgraph-sdk-go-core"
    abstractions "github.com/microsoft/kiota-abstractions-go"
    "github.com/microsoft/kiota-abstractions-go/serialization"
)
```

---

## Package layout

```
internal/
  config/         Config struct, LoadConfig, ValidateConfig
  auth/           Browser/device code auth, token cache, auth record, account registry, account resolver
  logging/        InitLogger, SanitizingHandler, PII masking, MultiHandler, file logging
  audit/          Audit logging subsystem, AuditWrap middleware
  graph/          Graph API utilities: errors, retry, timeout, serialization, enums, recurrence
  validate/       Input validation helpers
  observability/  OpenTelemetry metrics and tracing, WithObservability middleware
  server/         RegisterTools, ReadOnlyGuard, AwaitShutdownSignal
  tools/          4 aggregate domain tools dispatching verb sets
  docs/           Catalog, search, llms.txt; consumes docs.Bundle from docs/embed.go
```

---

## Middleware chain

Tool invocations pass through a three-layer middleware chain before reaching the verb handler:

```
MCP client
  └── ReadOnlyGuard (internal/server)      — blocks write verbs when OUTLOOK_MCP_READ_ONLY=true
      └── AuditWrap (internal/audit)       — structured audit record per invocation
          └── WithObservability (internal/observability) — OTel span + metrics
              └── verb handler (internal/tools/{domain}/)
```

Each middleware is applied in `internal/server/server.go` via the `wrap` / `wrapWrite` helpers.

---

## MCP server setup and transport

### Transport choice: stdio

The server uses **stdio transport exclusively**. This is the correct choice for a local, single-user MCP server for three reasons:

1. **Claude Desktop's `claude_desktop_config.json` only supports stdio** for locally configured servers. There is no `"url"` field or `"type": "http"` option in the local config format.
2. **The MCP specification explicitly recommends stdio for local subprocess servers**: "Clients SHOULD support stdio whenever possible."
3. **Device code auth maps naturally to stderr.** The server prints the device code prompt to stderr, which Claude Desktop captures in its MCP server logs (`~/Library/Logs/Claude/mcp-server-*.log` on macOS).

mcp-go supports three other transports (SSE, Streamable HTTP, In-Process), all of which add unnecessary complexity for this use case. SSE is deprecated in the MCP 2025-03-26 spec. Streamable HTTP is designed for remote multi-client servers. Neither offers any benefit for a local subprocess.

### Server creation

```go
s := server.NewMCPServer(
    "outlook-local",
    "1.0.0",
    server.WithToolCapabilities(false),
    server.WithRecovery(),
)
```

- **Name**: `"outlook-local"`, display name reported to MCP clients
- **Version**: `"1.0.0"`, semver string
- **`WithToolCapabilities(false)`**: Enables tool support; `false` means server will not emit `tools/list_changed` notifications (tool list is static)
- **`WithRecovery()`**: Catches panics in tool handlers and converts them to errors

### Tool registration

Tools are registered via `s.AddTool(tool, handler)` where `tool` is an `mcp.Tool` (created with `mcp.NewTool`) and `handler` is a `server.ToolHandlerFunc` with signature:

```go
func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
```

### Stdio transport

```go
if err := server.ServeStdio(s); err != nil {
    slog.Error("stdio transport error", "error", err)
    os.Exit(1)
}
```

`server.ServeStdio` reads JSON-RPC messages from stdin and writes responses to stdout. All diagnostic output (authentication prompts, logs, errors) **must** go to stderr to avoid corrupting the MCP protocol stream. The `ServeStdio` function blocks until stdin closes (client disconnects) or the process receives SIGINT/SIGTERM.

---

## Error handling strategy

### Two-tier error model

The MCP protocol distinguishes two error levels, and the server must use both correctly:

1. **Tool-level errors** (user-visible, LLM can retry): Return `mcp.NewToolResultError(message)` with `nil` error. The `isError` field is set to `true` in the result. Use this for invalid parameters, Graph API errors (403, 404, etc.), and any expected failure.

2. **Protocol-level errors** (fatal): Return `nil` result with a non-nil `error`. Use this only for truly exceptional situations like serialization bugs or context cancellation. The MCP framework translates these to JSON-RPC error responses.

### Graph API error extraction

```go
func formatGraphError(err error) string {
    if odataErr, ok := err.(*odataerrors.ODataError); ok {
        if inner := odataErr.GetErrorEscaped(); inner != nil {
            code := safeStr(inner.GetCode())
            msg := safeStr(inner.GetMessage())
            return fmt.Sprintf("Graph API error [%s]: %s", code, msg)
        }
        return odataErr.Error()
    }
    return err.Error()
}
```

### Common Graph API error codes and handling

| HTTP Status | OData Code | Cause | Handler response |
|---|---|---|---|
| 400 | `BadRequest` | Invalid query params (bad date format, invalid filter) | Return tool error with descriptive message about the parameter issue |
| 400 | `ErrorOccurrenceCrossingBoundary` | Attempted to move a recurring occurrence before/after adjacent occurrences | Return tool error explaining the recurrence constraint |
| 400 | `ErrorPropertyValidationFailure` | Invalid recurrence pattern (unsupported property combination) | Return tool error with the specific validation failure |
| 401 | `Unauthorized` | Token expired and refresh failed | Return tool error: "Authentication expired. Please restart the server to re-authenticate." |
| 403 | `Forbidden` | Insufficient permissions | Return tool error: "Insufficient permissions. The Calendars.ReadWrite scope is required." |
| 403 | `ErrorAccessDenied` | Non-organizer attempted to cancel a meeting | Return tool error: "Only the meeting organizer can cancel this event." |
| 404 | `NotFound` / `ErrorItemNotFound` | Event/calendar ID doesn't exist | Return tool error: "Event not found: {id}" |
| 409 | `Conflict` | Concurrent modification conflict | Retry once, then return tool error |
| 429 | `TooManyRequests` | Rate limiting | Implement exponential backoff with `Retry-After` header; retry up to 3 times, then return tool error |
| 503 | `ServiceUnavailable` | Graph API temporarily down | Retry once after 5 seconds, then return tool error |

### Parameter validation

Every handler validates required parameters first using `request.RequireString(name)`, which returns an error if the parameter is missing or not a string. For optional numeric parameters, use `request.GetInt(name, defaultValue)` or `request.GetFloat(name, defaultValue)`. Validate ranges explicitly (e.g., `max_results` must be 1-100).

### Null safety

All `models.Eventable` and `models.Calendarable` getter methods return pointers. **Every pointer dereference must be nil-checked.** Use a helper:

```go
func safeStr(s *string) string {
    if s == nil { return "" }
    return *s
}

func safeBool(b *bool) bool {
    if b == nil { return false }
    return *b
}
```

---

## Pagination handling

For `calendar_list_events` and `calendar_search_events`, the Graph API returns paginated results. The handler should use `msgraphcore.NewPageIterator` to transparently follow `@odata.nextLink` pointers:

```go
result, err := graphClient.Me().CalendarView().Get(ctx, config)
if err != nil {
    slog.Error("graph api calendarView failed", "error", formatGraphError(err))
    return mcp.NewToolResultError(formatGraphError(err)), nil
}

var events []map[string]any
pageIterator, err := msgraphcore.NewPageIterator[models.Eventable](
    result,
    graphClient.GetAdapter(),
    models.CreateEventCollectionResponseFromDiscriminatorValue,
)
if err != nil {
    slog.Error("page iterator creation failed", "error", err)
    return mcp.NewToolResultError(err.Error()), nil
}

count := 0
err = pageIterator.Iterate(ctx, func(event models.Eventable) bool {
    events = append(events, serializeEvent(event))
    count++
    return count < maxResults  // stop after max_results
})
```

When `$top` is set in query parameters, the Graph API returns at most that many per page. If `max_results` exceeds one page, the iterator fetches additional pages automatically. The handler enforces `max_results` as a hard cap by returning `false` from the callback.

---

## Configuration

The server reads configuration from environment variables with sensible defaults. No configuration file is required for basic operation.

| Environment variable | Default | Description |
|---|---|---|
| `OUTLOOK_MCP_CLIENT_ID` | `d3590ed6-52b3-4102-aeff-aad2292ab01c` | OAuth client ID. Override only if using a custom app registration. |
| `OUTLOOK_MCP_TENANT_ID` | `common` | Entra ID tenant. Use `consumers` for personal-only, `organizations` for work-only. |
| `OUTLOOK_MCP_AUTH_RECORD_PATH` | `~/.outlook-local-mcp/auth_record.json` | Path to persisted authentication record. |
| `OUTLOOK_MCP_TOKEN_STORAGE` | `auto` | Token storage backend: `auto` (OS keychain with file fallback), `keychain` (OS keychain only), `file` (file-based only). See CR-0038. |
| `OUTLOOK_MCP_CACHE_NAME` | `outlook-local-mcp` | Name for the OS-level token cache partition. |
| `OUTLOOK_MCP_DEFAULT_TIMEZONE` | `auto` | Default timezone for event times when not specified in tool calls. `auto` detects from the system; falls back to `UTC` if detection returns `Local`. See CR-0034. |
| `OUTLOOK_MCP_LOG_LEVEL` | `warn` | Minimum log level: `debug`, `info`, `warn`, `error`. |
| `OUTLOOK_MCP_LOG_FORMAT` | `json` | Log output format: `json` for structured JSON lines, `text` for human-readable `key=value` format. Both include source file and line number. |
| `OUTLOOK_MCP_LOG_FILE` | *(empty)* | Optional file path for persistent log output. When set, log records are written to both stderr and the file via a `MultiHandler`. File is opened append-mode with `0600` permissions. See CR-0023. |
| `OUTLOOK_MCP_MAIL_ENABLED` | `false` | Enable read-only mail access. When `true`, adds `Mail.Read` OAuth scope and registers mail verbs. See CR-0043. |
| `OUTLOOK_MCP_PROVENANCE_TAG` | `com.github.desek.outlook-local-mcp.created` | Name for the provenance extended property stamped on MCP-created events. Combined with a dedicated GUID to form the full MAPI property ID. Set to empty string to disable provenance tagging entirely. See CR-0040. |

---

## Startup and lifecycle sequence

The server's `main()` function executes these steps in order:

1. **Initialize logger** from `OUTLOOK_MCP_LOG_LEVEL` and `OUTLOOK_MCP_LOG_FORMAT` environment variables. This MUST be the very first operation so that all subsequent steps can log properly.
2. **Load remaining configuration** from environment variables.
3. **Initialize persistent token cache** via `cache.New()`. If unavailable (e.g., missing libsecret on Linux), log a warning and continue with in-memory cache.
4. **Load authentication record** from disk. If the file doesn't exist, `record` is zero-value.
5. **Create `DeviceCodeCredential`** with the Microsoft Office client ID, tenant, cache, record, and stderr prompt.
6. **Check if first-run authentication is needed.** If `record` is zero-value, call `cred.Authenticate(ctx, nil)` which triggers device code flow. Save the returned `AuthenticationRecord` to disk.
7. **Create Graph client** via `msgraphsdk.NewGraphServiceClientWithCredentials(cred, []string{"Calendars.ReadWrite"})`.
8. **Create MCP server** via `server.NewMCPServer("outlook-local", "1.0.0", ...)`.
9. **Register all tools** with their handlers, logging each registration.
10. **Start stdio transport** via `server.ServeStdio(s)`. This blocks until the client disconnects or the process is killed.
11. On exit, log shutdown reason. No explicit cleanup is needed; the persistent cache and auth record are already on disk.

### Graceful shutdown

The stdio server exits naturally when stdin closes (MCP client disconnects). If the process receives SIGINT or SIGTERM, it should exit cleanly. Since `ServeStdio` blocks on stdin, a signal handler can cancel a context or simply call `os.Exit(0)`.

---

## Claude Desktop integration

The server is designed to be invoked by Claude Desktop (or any MCP client) as a stdio subprocess. The Claude Desktop configuration in `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "outlook-local": {
      "command": "/path/to/outlook-local-mcp",
      "args": [],
      "env": {
        "OUTLOOK_MCP_TENANT_ID": "common",
        "OUTLOOK_MCP_LOG_LEVEL": "info"
      }
    }
  }
}
```

On first launch, the user sees the device code prompt in Claude Desktop's server logs (stderr). After authenticating once, subsequent launches are silent.

---

## Recurrence patterns reference

Recurrence is modeled as a JSON object with two sub-objects: `pattern` (what repeats) and `range` (how long). The `calendar_create_event` and `calendar_update_event` tools accept this as a JSON string in their `recurrence` parameter.

### Pattern types

Each pattern type requires specific properties. **Including unsupported properties for a given type causes errors.**

| Type | Example | Required properties |
|------|---------|---------------------|
| `daily` | Every 2 days | `type`, `interval` |
| `weekly` | Mon and Wed every week | `type`, `interval`, `daysOfWeek`, `firstDayOfWeek` |
| `absoluteMonthly` | 15th of every month | `type`, `interval`, `dayOfMonth` |
| `relativeMonthly` | 2nd Tuesday every 3 months | `type`, `interval`, `daysOfWeek`, `index` |
| `absoluteYearly` | March 15 every year | `type`, `interval`, `dayOfMonth`, `month` |
| `relativeYearly` | Last Friday of November | `type`, `interval`, `daysOfWeek`, `month`, `index` |

The `index` property accepts `first`, `second`, `third`, `fourth`, or `last`. The `daysOfWeek` values are lowercase: `sunday`, `monday`, `tuesday`, `wednesday`, `thursday`, `friday`, `saturday`.

### Range types

| Type | Required properties | Description |
|------|---------------------|-------------|
| `endDate` | `type`, `startDate`, `endDate` | Recur until a specific date |
| `noEnd` | `type`, `startDate` | Recur indefinitely |
| `numbered` | `type`, `startDate`, `numberOfOccurrences` | Recur a fixed number of times |

Dates in the range use **`YYYY-MM-DD` format** (date only, no time component). The `startDate` must match the event's start date.

### Examples

Weekly Monday meeting ending December 31:
```json
{
  "pattern": {"type": "weekly", "interval": 1, "daysOfWeek": ["monday"], "firstDayOfWeek": "sunday"},
  "range": {"type": "endDate", "startDate": "2026-04-15", "endDate": "2026-12-31"}
}
```

Every other day, 10 occurrences:
```json
{
  "pattern": {"type": "daily", "interval": 2},
  "range": {"type": "numbered", "startDate": "2026-04-15", "numberOfOccurrences": 10}
}
```

Last Friday of every month, no end:
```json
{
  "pattern": {"type": "relativeMonthly", "interval": 1, "daysOfWeek": ["friday"], "index": "last"},
  "range": {"type": "noEnd", "startDate": "2026-04-25"}
}
```

### Go SDK construction

```go
recurrence := models.NewPatternedRecurrence()

pattern := models.NewRecurrencePattern()
patternType := models.WEEKLY_RECURRENCEPATTERNTYPE
pattern.SetTypeEscaped(&patternType)
interval := int32(1)
pattern.SetInterval(&interval)
pattern.SetDaysOfWeek([]models.DayOfWeek{models.MONDAY_DAYOFWEEK})
firstDay := models.SUNDAY_DAYOFWEEK
pattern.SetFirstDayOfWeek(&firstDay)
recurrence.SetPattern(pattern)

recurrenceRange := models.NewRecurrenceRange()
rangeType := models.ENDDATE_RECURRENCERANGETYPE
recurrenceRange.SetTypeEscaped(&rangeType)
startDate := serialization.NewDateOnly(time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC))
recurrenceRange.SetStartDate(startDate)
endDate := serialization.NewDateOnly(time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC))
recurrenceRange.SetEndDate(endDate)
recurrence.SetRange(recurrenceRange)

requestBody.SetRecurrence(recurrence)
```

The `serialization.DateOnly` type (from `github.com/microsoft/kiota-abstractions-go/serialization`) wraps date-only values. Create with `serialization.NewDateOnly(time.Time)` or parse with `serialization.ParseDateOnly("2026-04-15")`.

---

## Go SDK enum reference

The msgraph-sdk-go uses Kiota-generated enum constants following the `UPPERCASENAME_TYPENAME` convention. All setter methods take pointers. This reference lists the constants used across the write tools:

```go
// BodyType
models.TEXT_BODYTYPE
models.HTML_BODYTYPE

// AttendeeType
models.REQUIRED_ATTENDEETYPE
models.OPTIONAL_ATTENDEETYPE
models.RESOURCE_ATTENDEETYPE

// Importance
models.LOW_IMPORTANCE
models.NORMAL_IMPORTANCE
models.HIGH_IMPORTANCE

// Sensitivity
models.NORMAL_SENSITIVITY
models.PERSONAL_SENSITIVITY
models.PRIVATE_SENSITIVITY
models.CONFIDENTIAL_SENSITIVITY

// FreeBusyStatus (showAs)
models.FREE_FREEBUSYSTATUS
models.TENTATIVE_FREEBUSYSTATUS
models.BUSY_FREEBUSYSTATUS
models.OOF_FREEBUSYSTATUS
models.WORKINGELSEWHERE_FREEBUSYSTATUS
models.UNKNOWN_FREEBUSYSTATUS

// RecurrencePatternType
models.DAILY_RECURRENCEPATTERNTYPE
models.WEEKLY_RECURRENCEPATTERNTYPE
models.ABSOLUTEMONTHLY_RECURRENCEPATTERNTYPE
models.RELATIVEMONTHLY_RECURRENCEPATTERNTYPE
models.ABSOLUTEYEARLY_RECURRENCEPATTERNTYPE
models.RELATIVEYEARLY_RECURRENCEPATTERNTYPE

// RecurrenceRangeType
models.ENDDATE_RECURRENCERANGETYPE
models.NOEND_RECURRENCERANGETYPE
models.NUMBERED_RECURRENCERANGETYPE

// DayOfWeek
models.SUNDAY_DAYOFWEEK
models.MONDAY_DAYOFWEEK
models.TUESDAY_DAYOFWEEK
models.WEDNESDAY_DAYOFWEEK
models.THURSDAY_DAYOFWEEK
models.FRIDAY_DAYOFWEEK
models.SATURDAY_DAYOFWEEK

// WeekIndex (for relative patterns)
models.FIRST_WEEKINDEX
models.SECOND_WEEKINDEX
models.THIRD_WEEKINDEX
models.FOURTH_WEEKINDEX
models.LAST_WEEKINDEX

// OnlineMeetingProviderType
models.TEAMSFORBUSINESS_ONLINEMEETINGPROVIDERTYPE
models.SKYPEFORBUSINESS_ONLINEMEETINGPROVIDERTYPE
models.SKYPEFORCONSUMER_ONLINEMEETINGPROVIDERTYPE
```

---

## In-server documentation surface (CR-0061)

The server embeds a curated documentation bundle at compile time using Go `embed.FS` and exposes it through two complementary surfaces.

### MCP resources

Each embedded document is registered as an MCP resource:

| URI | MIME type | Description |
|-----|-----------|-------------|
| `doc://outlook-local-mcp/readme` | `text/markdown` | README |
| `doc://outlook-local-mcp/quickstart` | `text/markdown` | Quick Start guide |
| `doc://outlook-local-mcp/concepts` | `text/markdown` | Concepts and narrative content |
| `doc://outlook-local-mcp/troubleshooting` | `text/markdown` | Troubleshooting guide |

Clients that support `resources/list` and `resources/read` can fetch any of these URIs directly. The bundle is intentionally limited to user-facing documents. Engineering documentation (Change Requests under `docs/cr/`, the reference files in `docs/reference/`, research notes under `docs/research/`, and `CHANGELOG.md`) is not embedded and is not exposed through this surface.

### System domain verbs

Three verbs on the `system` aggregate tool provide a deterministic lookup path for LLM clients that do not surface MCP resources natively:

| Verb | Description |
|------|-------------|
| `system.list_docs` | Returns the catalog of available documents (slug, title, one-line summary, tags, size). |
| `system.search_docs` | Case-insensitive keyword search across the bundle. Returns ranked results with slug, matched snippet (±2 lines), and 1-based line numbers. |
| `system.get_docs` | Fetches a document or a single section by heading anchor. Parameters: `slug` (required), `section` (optional heading anchor), `output` (`text` default, `raw` markdown). |

All three verbs are read-only, idempotent, local (no Graph API calls), and honour the three-tier output model (`text` default, `raw` on request).

### Bundle constraints

The embedded bundle is enforced by build-time tests:

* Total uncompressed size must be under 2 MiB (`TestBundleSizeUnder2MiB`).
* The allowlist is explicit: exactly four slugs (`readme`, `quickstart`, `concepts`, `troubleshooting`).
* Secret patterns (`eyJ`, `sk-`, `client_secret`, `refresh_token`) cause a build failure.

The bundle is verified by `make docs-bundle`, which is wired into `make ci`.

---

## Known limitations and caveats

**Using a first-party client ID is unsupported by Microsoft.** The Microsoft Office client ID's pre-authorized scopes could change without notice. For production or enterprise use, registering a custom Entra ID application is strongly recommended. The `OUTLOOK_MCP_CLIENT_ID` environment variable allows seamless migration to a custom registration when needed.

**Conditional Access policies** in some organizations may block device code flow entirely or restrict it to compliant devices. If authentication fails with policy-related errors, the user must consult their IT administrator.

**The `azidentity/cache` package is pre-v1** (v0.4.x). Its API may change before reaching stable. Pin the exact version in `go.mod`.

**CalendarView vs Events endpoint:** The `calendar_list_events` tool uses CalendarView rather than the Events endpoint because CalendarView expands recurring events into individual occurrences, which is what users expect when querying "what meetings do I have this week." The Events endpoint returns series masters without expansion, which is less useful for time-range queries.

**Personal Microsoft accounts** (Outlook.com, Hotmail) use `TenantID: "consumers"` or `"common"`. The Graph API endpoints work identically for personal and work accounts. However, some features (calendar sharing, group calendars, Teams online meetings) are only available with work/school accounts.

**Teams online meeting properties are immutable after creation.** The `onlineMeeting.joinUrl`, `onlineMeeting.conferenceId`, and `onlineMeeting.tollNumber` fields cannot be changed via the update event endpoint. If a different meeting link is needed, a new event must be created.

**`startsWith()` filter on subject has a null-subject bug.** If any event in the CalendarView result set has a null or empty subject, a `startsWith(subject, 'text')` filter causes a 500 error from the Graph API. The `calendar_search_events` handler must catch this and fall back to client-side filtering.

**Recurrence pattern properties must exactly match the pattern type.** Including extra properties (e.g., specifying `dayOfMonth` on a `weekly` pattern) causes a 400 error. The handler should validate the recurrence JSON before sending it to the API.

**No "this and all following" update for recurring events.** The Graph API does not support updating a recurring occurrence and all subsequent ones in a single call. The workaround (ending the series early and creating a new one) is complex and is left to the AI client to orchestrate via sequential tool calls.

**Maximum 500 attendees** per event (Exchange Online limit). Attempting to add more causes a 400 error.
