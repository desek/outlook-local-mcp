# Technical Specification: Outlook Local MCP Server in Go

> **Status note:** This document describes the original tool-per-operation surface that predates CR-0060. As of v0.6.0 the server exposes four aggregate domain tools (`calendar`, `mail`, `account`, `system`) dispatched by an `operation` verb. See CR-0060 and the README "Tool Invocation Shape" section for the current surface. The verb-level semantics described below remain accurate; only the registration shape changed.

**The server authenticates as the user via device code flow using a well-known Microsoft first-party client ID, manages calendar data through the Microsoft Graph API v1.0, and exposes up to twenty tools (five read-only calendar, six write, four read-only mail (opt-in), three account management, one diagnostic, plus conditional complete_auth) over MCP stdio transport.** No custom Entra ID app registration is required. This specification covers every component (authentication, token persistence, logging, tool schemas, error handling, and configuration) with exact Go types and API calls sufficient to write the complete implementation.

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

## Authentication: device code flow without app registration

### The critical client ID choice

The server uses the **Microsoft Office** first-party client ID: **`d3590ed6-52b3-4102-aeff-aad2292ab01c`**. This is the only well-known client ID confirmed to have `Calendars.Read` and `Calendars.ReadWrite` pre-authorized for the Microsoft Graph resource (`00000003-0000-0000-c000-000000000000`). The Azure CLI client ID (`04b07795-8ddb-461a-bbee-02f9e1bf7b46`) explicitly does **not** support calendar scopes and will fail with `AADSTS65002` ("consent between first party application and first party resource must be configured via preauthorization").

The Microsoft Office client ID is present in every Entra ID / Entra ID tenant by default. It supports device code flow and is pre-authorized for a broad set of Microsoft Graph delegated permissions including `Calendar.ReadWrite`, `Calendars.Read.Shared`, `Calendars.ReadWrite`, `Mail.ReadWrite`, `Files.Read`, `Contacts.ReadWrite`, `User.Read.All`, `People.Read`, and others.

### Tenant ID configuration

| Value | Supported accounts | Recommendation |
|---|---|---|
| `"common"` | Work/school + personal Microsoft accounts | **Use this as default**, broadest compatibility |
| `"organizations"` | Work/school accounts only | Use if personal accounts should be excluded |
| `"consumers"` | Personal Microsoft accounts only (Outlook.com) | Use for personal-only scenarios |
| `"<tenant-guid>"` | Single specific tenant | Use for enterprise lockdown |

The specification defaults to `"common"` but allows override via configuration.

### OAuth scopes

Request the delegated scope **`Calendars.ReadWrite`** by default. This is the least-privileged delegated permission that grants full read and write access to all calendar event properties including body, subject, location, attendees, and the ability to create, update, delete, and cancel events. `Calendars.Read` would be insufficient because it does not permit write operations. `Calendars.ReadBasic` is even more limited and omits body content entirely. The `offline_access` scope is automatically included by the Azure Identity library to obtain a refresh token.

When mail access is enabled via `OUTLOOK_MCP_MAIL_ENABLED=true`, the **`Mail.Read`** scope is additionally requested, granting read-only access to the user's mailbox. This scope is not requested when mail is disabled (the default). See CR-0043 for details on the opt-in mail feature.

The `Calendars.ReadWrite` scope is a delegated permission that **does not require admin consent**; users can self-consent. It covers all write operations including creating events with attendees (which automatically sends invitations), cancelling events (which sends cancellation notices), and enabling Teams online meetings via the `isOnlineMeeting` flag. No `Mail.Send` or `OnlineMeetings.ReadWrite` scope is needed.

When calling `msgraphsdk.NewGraphServiceClientWithCredentials`, pass scopes from `auth.Scopes(cfg)`, which returns `[]string{"Calendars.ReadWrite"}` when mail is disabled, or `[]string{"Calendars.ReadWrite", "Mail.Read"}` when mail is enabled. The SDK automatically prefixes the Graph resource URI.

### Device code flow sequence

1. The server calls `azidentity.NewDeviceCodeCredential(options)` at startup.
2. On first authentication (no cached token), the credential's `GetToken()` method posts to `https://login.microsoftonline.com/common/oauth2/v2.0/devicecode` with the client ID and scope.
3. Microsoft returns a `user_code` and `verification_uri` (`https://microsoft.com/devicelogin`).
4. The `UserPrompt` callback fires. The server prints the message to **stderr** (not stdout, which is reserved for MCP JSON-RPC). The message reads something like: *"To sign in, use a web browser to open the page https://microsoft.com/devicelogin and enter the code ABCD1234 to authenticate."*
5. The library polls `https://login.microsoftonline.com/common/oauth2/v2.0/token` with `grant_type=urn:ietf:params:oauth:grant-type:device_code` until the user completes sign-in or the code expires (~15 minutes).
6. On success, the credential receives `access_token`, `refresh_token`, `id_token`, and caches them.
7. Subsequent `GetToken()` calls return the cached access token or silently refresh using the refresh token, with no user interaction required.

### azidentity credential construction

```go
const (
    microsoftOfficeClientID = "d3590ed6-52b3-4102-aeff-aad2292ab01c"
    defaultTenantID         = "common"
)

cred, err := azidentity.NewDeviceCodeCredential(&azidentity.DeviceCodeCredentialOptions{
    ClientID:             microsoftOfficeClientID,
    TenantID:             defaultTenantID,
    Cache:                persistentCache,          // from azidentity/cache
    AuthenticationRecord: loadedAuthRecord,          // from disk, zero-value on first run
    UserPrompt: func(ctx context.Context, msg azidentity.DeviceCodeMessage) error {
        fmt.Fprintf(os.Stderr, "\n%s\n\n", msg.Message)
        slog.Info("device code message displayed to user")
        return nil
    },
})
```

**`DeviceCodeCredentialOptions` fields used:**

| Field | Type | Value | Purpose |
|---|---|---|---|
| `ClientID` | `string` | `"d3590ed6-52b3-4102-aeff-aad2292ab01c"` | Microsoft Office first-party app |
| `TenantID` | `string` | `"common"` (configurable) | Multi-tenant support |
| `Cache` | `azidentity.Cache` | From `cache.New()` | Persistent OS-level token cache |
| `AuthenticationRecord` | `azidentity.AuthenticationRecord` | Loaded from JSON file | Identifies cached account |
| `UserPrompt` | `func(context.Context, DeviceCodeMessage) error` | Print to stderr | Shows device code to user |

---

## Token caching and persistence

Token caching is critical so users authenticate only once. The implementation uses two complementary mechanisms:

### 1. Persistent token cache (`azidentity/cache`)

The `github.com/Azure/azure-sdk-for-go/sdk/azidentity/cache` package stores encrypted tokens in OS-native secure storage: macOS Keychain, Linux libsecret (GNOME Keyring), or Windows DPAPI.

```go
import "github.com/Azure/azure-sdk-for-go/sdk/azidentity/cache"

c, err := cache.New(&cache.Options{Name: "outlook-local-mcp"})
if err != nil {
    slog.Warn("persistent token cache unavailable, using in-memory cache", "error", err)
    c = nil  // nil Cache means in-memory only
} else {
    slog.Info("persistent token cache initialized", "cache_name", "outlook-local-mcp")
}
```

### 2. Authentication record file

`azidentity.AuthenticationRecord` is a non-secret JSON-serializable struct containing metadata (account ID, tenant, authority) that tells the credential which cached token to look up. It contains **no tokens** and is safe to store on disk.

**File path:** `~/.outlook-local-mcp/auth_record.json` (configurable). File permissions: `0600`.

**Load pattern:**
```go
func loadAuthRecord(path string) azidentity.AuthenticationRecord {
    var record azidentity.AuthenticationRecord
    data, err := os.ReadFile(path)
    if err != nil {
        slog.Info("no authentication record found, device code flow required", "path", path)
        return record // zero-value, triggers fresh auth
    }
    if err := json.Unmarshal(data, &record); err != nil {
        slog.Warn("corrupt authentication record, will re-authenticate", "path", path, "error", err)
        return azidentity.AuthenticationRecord{}
    }
    slog.Info("authentication record loaded", "path", path)
    return record
}
```

**Save pattern (after first authentication):**
```go
func saveAuthRecord(path string, record azidentity.AuthenticationRecord) error {
    data, err := json.Marshal(record)
    if err != nil {
        return err
    }
    os.MkdirAll(filepath.Dir(path), 0700)
    if err := os.WriteFile(path, data, 0600); err != nil {
        return err
    }
    slog.Info("authentication record saved", "path", path)
    return nil
}
```

**First-run flow:** If no auth record exists, call `cred.Authenticate(ctx, nil)` explicitly to trigger device code flow and obtain the record, then save it. On subsequent runs, the record + persistent cache allow silent token acquisition with no user interaction.

```go
if record == (azidentity.AuthenticationRecord{}) {
    record, err = cred.Authenticate(context.Background(), nil)
    if err != nil {
        slog.Error("authentication failed", "error", err)
        os.Exit(1)
    }
    slog.Info("authentication successful", "tenant", record.TenantID)
    saveAuthRecord(authRecordPath, record)
}
```

---

## Graph client initialization

After authentication, construct the Graph client using the convenience constructor:

```go
graphClient, err := msgraphsdk.NewGraphServiceClientWithCredentials(
    cred,
    []string{"Calendars.ReadWrite"},
)
if err != nil {
    slog.Error("graph client initialization failed", "error", err)
    os.Exit(1)
}
slog.Info("graph client initialized", "scopes", []string{"Calendars.ReadWrite"})
```

This internally creates a `kiota-authentication-azure-go` auth provider and a `GraphRequestAdapter`. The `graphClient` is stored as a package-level `*msgraphsdk.GraphServiceClient` and shared across all tool handlers. Thread safety is guaranteed by the SDK.

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

## MCP tool definitions

The server exposes **up to twenty tools**: five read-only calendar, four read-only mail (opt-in via `OUTLOOK_MCP_MAIL_ENABLED`), six write, three account management, one diagnostic, plus conditional `complete_auth`. Each tool returns JSON-serialized data as text content. All tools include the full set of five MCP annotations (`title`, `readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint`) for Anthropic Software Directory compliance (see CR-0052). All domain tools use a `{domain}_{operation}[_{resource}]` naming convention (see CR-0050). The original nine tools (five read-only, four write) are documented below; see CR-0025 for account management tools, CR-0037 for the status diagnostic tool, CR-0030 for complete_auth, CR-0042 for `calendar_respond_event` and `calendar_reschedule_event`, and CR-0043 for the four mail tools (`mail_list_folders`, `mail_list_messages`, `mail_search_messages`, `mail_get_message`).

### Tool 1: `calendar_list`

**Purpose:** List all calendars accessible to the authenticated user.

**Parameters:** None.

**Registration:**
```go
mcp.NewTool("calendar_list",
    mcp.WithDescription("List all calendars for the authenticated user. Returns calendar ID, name, color, and whether it is the default calendar."),
    mcp.WithReadOnlyHintAnnotation(true),
)
```

**Graph API call:** `GET /me/calendars` via `graphClient.Me().Calendars().Get(ctx, nil)`

**Response schema (JSON array):**
```json
[
  {
    "id": "AAMkAG...",
    "name": "Calendar",
    "color": "auto",
    "hexColor": "",
    "isDefaultCalendar": true,
    "canEdit": true,
    "owner": {
      "name": "John Doe",
      "address": "john@example.com"
    }
  }
]
```

**Fields extracted from `models.Calendarable`:**

| Field | Getter | JSON key | Type |
|---|---|---|---|
| ID | `GetId()` | `id` | string |
| Name | `GetName()` | `name` | string |
| Color | `GetColor()` | `color` | string (enum) |
| Hex Color | `GetHexColor()` | `hexColor` | string |
| Is Default | `GetIsDefaultCalendar()` | `isDefaultCalendar` | bool |
| Can Edit | `GetCanEdit()` | `canEdit` | bool |
| Owner Name | `GetOwner().GetName()` | `owner.name` | string |
| Owner Address | `GetOwner().GetAddress()` | `owner.address` | string |

---

### Tool 2: `calendar_list_events`

**Purpose:** List calendar events within a time range. Uses the CalendarView endpoint, which expands recurring events into individual occurrences.

**Parameters:**

| Name | Type | Required | Description |
|---|---|---|---|
| `start_datetime` | string | Yes | Start of time range, ISO 8601 format (e.g., `"2026-03-12T00:00:00Z"` or `"2026-03-12T00:00:00-05:00"`) |
| `end_datetime` | string | Yes | End of time range, ISO 8601 format |
| `calendar_id` | string | No | Specific calendar ID. If omitted, uses default calendar. |
| `max_results` | number | No | Maximum events to return. Default 25, max 100. |
| `timezone` | string | No | IANA timezone name for response times (e.g., `"America/New_York"`). Default: UTC. |

**Registration:**
```go
mcp.NewTool("calendar_list_events",
    mcp.WithDescription("List calendar events within a time range. Recurring events are expanded into individual occurrences. Returns event ID, subject, start/end times, location, organizer, and status."),
    mcp.WithReadOnlyHintAnnotation(true),
    mcp.WithString("start_datetime",
        mcp.Required(),
        mcp.Description("Start of time range in ISO 8601 format, e.g. 2026-03-12T00:00:00Z"),
    ),
    mcp.WithString("end_datetime",
        mcp.Required(),
        mcp.Description("End of time range in ISO 8601 format, e.g. 2026-03-13T00:00:00Z"),
    ),
    mcp.WithString("calendar_id",
        mcp.Description("Calendar ID to query. Omit for default calendar."),
    ),
    mcp.WithNumber("max_results",
        mcp.Description("Maximum number of events to return (1-100, default 25)"),
        mcp.Min(1),
        mcp.Max(100),
    ),
    mcp.WithString("timezone",
        mcp.Description("IANA timezone for response times, e.g. America/New_York. Default: UTC"),
    ),
)
```

**Graph API call:** `GET /me/calendarView?startDateTime=...&endDateTime=...` (default calendar) or `GET /me/calendars/{id}/calendarView?startDateTime=...&endDateTime=...` (specific calendar).

**SDK call pattern:**
```go
params := &graphusers.ItemCalendarViewRequestBuilderGetQueryParameters{
    StartDateTime: &startDT,
    EndDateTime:   &endDT,
    Top:           &top,
    Select: []string{"id", "subject", "start", "end", "location", "organizer",
        "isAllDay", "showAs", "importance", "sensitivity", "isCancelled",
        "categories", "webLink", "isOnlineMeeting", "onlineMeeting"},
    Orderby: []string{"start/dateTime"},
}
headers := abstractions.NewRequestHeaders()
if timezone != "" {
    headers.Add("Prefer", fmt.Sprintf(`outlook.timezone="%s"`, timezone))
}
config := &graphusers.ItemCalendarViewRequestBuilderGetRequestConfiguration{
    QueryParameters: params,
    Headers:         headers,
}
result, err := graphClient.Me().CalendarView().Get(ctx, config)
```

For a specific calendar:
```go
result, err := graphClient.Me().Calendars().ByCalendarId(calendarID).CalendarView().Get(ctx, config)
```

**Response schema (JSON array):**
```json
[
  {
    "id": "AAMkAG...",
    "subject": "Team Standup",
    "start": {
      "dateTime": "2026-03-12T09:00:00.0000000",
      "timeZone": "America/New_York"
    },
    "end": {
      "dateTime": "2026-03-12T09:30:00.0000000",
      "timeZone": "America/New_York"
    },
    "location": "Conference Room A",
    "organizer": {
      "name": "Jane Smith",
      "email": "jane@example.com"
    },
    "isAllDay": false,
    "showAs": "busy",
    "importance": "normal",
    "sensitivity": "normal",
    "isCancelled": false,
    "categories": ["Blue Category"],
    "webLink": "https://outlook.office365.com/...",
    "isOnlineMeeting": true,
    "onlineMeetingUrl": "https://teams.microsoft.com/...",
    "createdByMcp": true
  }
]
```

> **Note (CR-0040):** The `createdByMcp` field is only present (set to `true`) on events created by this MCP server when provenance tagging is enabled. It is omitted entirely for events not created by the MCP. The field is populated by checking for the provenance extended property via `$expand` on the CalendarView request.

---

### Tool 3: `calendar_get_event`

**Purpose:** Get full details of a specific calendar event by ID, including body content, attendees, and recurrence.

**Parameters:**

| Name | Type | Required | Description |
|---|---|---|---|
| `event_id` | string | Yes | The unique event ID |
| `timezone` | string | No | IANA timezone for response times |

**Registration:**
```go
mcp.NewTool("calendar_get_event",
    mcp.WithDescription("Get full details of a specific calendar event by ID, including body, attendees, recurrence pattern, and online meeting info."),
    mcp.WithReadOnlyHintAnnotation(true),
    mcp.WithString("event_id",
        mcp.Required(),
        mcp.Description("The unique ID of the event to retrieve"),
    ),
    mcp.WithString("timezone",
        mcp.Description("IANA timezone for response times, e.g. America/New_York"),
    ),
)
```

**Graph API call:** `GET /me/events/{id}` via `graphClient.Me().Events().ByEventId(eventID).Get(ctx, config)`

**SDK call pattern:**
```go
params := &graphusers.EventsItemRequestBuilderGetQueryParameters{
    Select: []string{"id", "subject", "body", "bodyPreview", "start", "end",
        "location", "locations", "organizer", "attendees", "isAllDay",
        "showAs", "importance", "sensitivity", "isCancelled", "recurrence",
        "categories", "webLink", "isOnlineMeeting", "onlineMeeting",
        "responseStatus", "seriesMasterId", "type", "hasAttachments",
        "createdDateTime", "lastModifiedDateTime"},
}
```

**Response schema (JSON object):**
```json
{
  "id": "AAMkAG...",
  "subject": "Quarterly Planning",
  "bodyPreview": "Let's review Q2 goals...",
  "body": {
    "contentType": "html",
    "content": "<html>...</html>"
  },
  "start": {"dateTime": "2026-03-15T14:00:00.0000000", "timeZone": "UTC"},
  "end": {"dateTime": "2026-03-15T15:00:00.0000000", "timeZone": "UTC"},
  "location": "Building 5, Room 301",
  "locations": ["Building 5, Room 301"],
  "organizer": {"name": "Jane Smith", "email": "jane@example.com"},
  "attendees": [
    {
      "name": "Bob Johnson",
      "email": "bob@example.com",
      "type": "required",
      "response": "accepted"
    }
  ],
  "isAllDay": false,
  "showAs": "busy",
  "importance": "high",
  "sensitivity": "normal",
  "isCancelled": false,
  "recurrence": null,
  "categories": [],
  "webLink": "https://outlook.office365.com/...",
  "isOnlineMeeting": true,
  "onlineMeetingUrl": "https://teams.microsoft.com/...",
  "responseStatus": {"response": "organizer", "time": "0001-01-01T00:00:00Z"},
  "seriesMasterId": null,
  "type": "singleInstance",
  "hasAttachments": false,
  "createdDateTime": "2026-03-01T10:00:00Z",
  "lastModifiedDateTime": "2026-03-10T14:30:00Z"
}
```

**Attendee extraction pattern:**
```go
for _, att := range event.GetAttendees() {
    attendee := map[string]string{}
    if ea := att.GetEmailAddress(); ea != nil {
        attendee["name"] = safeStr(ea.GetName())
        attendee["email"] = safeStr(ea.GetAddress())
    }
    if t := att.GetTypeEscaped(); t != nil {
        attendee["type"] = t.String()  // "required", "optional", "resource"
    }
    if s := att.GetStatus(); s != nil {
        if r := s.GetResponse(); r != nil {
            attendee["response"] = r.String()
        }
    }
}
```

---

### Tool 4: `calendar_search_events`

**Purpose:** Search for events with flexible filtering by subject text, date range, importance, sensitivity, and other properties. Uses CalendarView for date-range queries (which expands recurring events) with optional OData `$filter` and client-side fallback for unsupported filter operations.

**Parameters:**

| Name | Type | Required | Description |
|---|---|---|---|
| `query` | string | No | Text to search for in event subjects (case-insensitive). Uses `startsWith` server-side when possible, falls back to client-side `contains` for substring matching. |
| `start_datetime` | string | No | Start of search range, ISO 8601. Default: current time. |
| `end_datetime` | string | No | End of search range, ISO 8601. Default: 30 days from start. |
| `importance` | string | No | Filter by importance: `low`, `normal`, or `high`. |
| `sensitivity` | string | No | Filter by sensitivity: `normal`, `personal`, `private`, or `confidential`. |
| `is_all_day` | boolean | No | Filter to all-day events only (`true`) or timed events only (`false`). |
| `show_as` | string | No | Filter by free/busy status: `free`, `tentative`, `busy`, `oof`, or `workingElsewhere`. |
| `is_cancelled` | boolean | No | Filter to cancelled events only (`true`) or active events only (`false`). |
| `categories` | string | No | Comma-separated list of category names. Returns events matching any listed category. |
| `created_by_mcp` | boolean | No | When true, only return events created by this MCP server. Uses a server-side OData filter on the provenance extended property. Only available when provenance tagging is enabled (see CR-0040). |
| `max_results` | number | No | Maximum events to return (1-100, default 25). |
| `timezone` | string | No | IANA timezone for response times. Default: UTC. |

**Registration:**
```go
mcp.NewTool("calendar_search_events",
    mcp.WithDescription("Search calendar events with flexible filtering by subject text, date range, importance, sensitivity, categories, and other properties. Returns events within the specified time window matching all provided filters."),
    mcp.WithReadOnlyHintAnnotation(true),
    mcp.WithString("query",
        mcp.Description("Text to search for in event subjects (case-insensitive substring match)"),
    ),
    mcp.WithString("start_datetime",
        mcp.Description("Start of search range in ISO 8601. Default: current time."),
    ),
    mcp.WithString("end_datetime",
        mcp.Description("End of search range in ISO 8601. Default: 30 days from start."),
    ),
    mcp.WithString("importance",
        mcp.Description("Filter by importance: low, normal, or high"),
    ),
    mcp.WithString("sensitivity",
        mcp.Description("Filter by sensitivity: normal, personal, private, or confidential"),
    ),
    mcp.WithBoolean("is_all_day",
        mcp.Description("Filter to all-day events (true) or timed events (false)"),
    ),
    mcp.WithString("show_as",
        mcp.Description("Filter by free/busy status: free, tentative, busy, oof, or workingElsewhere"),
    ),
    mcp.WithBoolean("is_cancelled",
        mcp.Description("Filter to cancelled events (true) or active events (false)"),
    ),
    mcp.WithString("categories",
        mcp.Description("Comma-separated category names. Returns events matching any listed category."),
    ),
    mcp.WithNumber("max_results",
        mcp.Description("Maximum events to return (1-100, default 25)"),
        mcp.Min(1),
        mcp.Max(100),
    ),
    mcp.WithString("timezone",
        mcp.Description("IANA timezone for response times. Default: UTC"),
    ),
    // Only registered when provenance tagging is enabled (CR-0040)
    mcp.WithBoolean("created_by_mcp",
        mcp.Description("When true, only return events created by this MCP server (server-side filter)"),
    ),
)
```

**Graph API approach:** The handler builds a composite OData `$filter` string from the provided parameters, using CalendarView with the date range. Filters are combined with `and`:

```go
var filters []string
if importance != "" {
    filters = append(filters, fmt.Sprintf("importance eq '%s'", importance))
}
if sensitivity != "" {
    filters = append(filters, fmt.Sprintf("sensitivity eq '%s'", sensitivity))
}
if isAllDay != nil {
    filters = append(filters, fmt.Sprintf("isAllDay eq %t", *isAllDay))
}
if showAs != "" {
    filters = append(filters, fmt.Sprintf("showAs eq '%s'", showAs))
}
if isCancelled != nil {
    filters = append(filters, fmt.Sprintf("isCancelled eq %t", *isCancelled))
}
// Provenance filter: server-side extended property filter (CR-0040)
if createdByMcp && provenancePropertyID != "" {
    filters = append(filters, fmt.Sprintf(
        "singleValueExtendedProperties/Any(ep: ep/id eq '%s' and ep/value eq 'true')",
        provenancePropertyID,
    ))
}
if query != "" {
    // startsWith is the only reliable server-side string function
    filters = append(filters, fmt.Sprintf("startsWith(subject,'%s')", escapeOData(query)))
}
filterStr := strings.Join(filters, " and ")
```

**Filtering limitations and fallbacks:**

The Graph API `$filter` on calendar events has important constraints. `startsWith(subject, 'text')` works server-side but has a known bug: if any event in the result set has a null or empty subject, the API returns a 500 error. The `contains()` function is partially functional for positive matches but `not(contains(...))` returns `ErrorInternalServerError`. `$search` is **not supported** on calendar events at all.

For subject text matching, the handler should attempt `startsWith` server-side first. If the API returns a 500 error, fall back to fetching all events in the range and filtering client-side with `strings.Contains(strings.ToLower(subject), strings.ToLower(query))`. Client-side filtering is also always used for substring matching that `startsWith` cannot cover.

**OData `$filter` syntax for DateTime fields:** When filtering on `start/dateTime` or `end/dateTime` outside of CalendarView (on the `/me/events` endpoint), values must be enclosed in single quotes because these are String-typed, not DateTimeOffset:

```
$filter=start/dateTime ge '2026-01-01T00:00:00' and end/dateTime le '2026-12-31T23:59:59'
```

**Response schema:** Same array format as `calendar_list_events`.

---

### Tool 5: `calendar_get_free_busy`

**Purpose:** Get free/busy status for the authenticated user over a time range, useful for scheduling.

**Parameters:**

| Name | Type | Required | Description |
|---|---|---|---|
| `start_datetime` | string | Yes | Start of time range, ISO 8601 |
| `end_datetime` | string | Yes | End of time range, ISO 8601 |
| `timezone` | string | No | IANA timezone. Default: UTC. |

**Registration:**
```go
mcp.NewTool("calendar_get_free_busy",
    mcp.WithDescription("Get free/busy status for the authenticated user over a time range. Returns a simplified view of busy periods."),
    mcp.WithReadOnlyHintAnnotation(true),
    mcp.WithString("start_datetime",
        mcp.Required(),
        mcp.Description("Start of time range in ISO 8601 format"),
    ),
    mcp.WithString("end_datetime",
        mcp.Required(),
        mcp.Description("End of time range in ISO 8601 format"),
    ),
    mcp.WithString("timezone",
        mcp.Description("IANA timezone for response times. Default: UTC"),
    ),
)
```

**Implementation approach:** Use `calendar_list_events` (CalendarView) internally, then extract only the `showAs`, `start`, `end`, and `subject` fields for each event where `showAs` is not `"free"`. Return a compact representation of busy periods.

**Response schema:**
```json
{
  "timeRange": {
    "start": "2026-03-12T00:00:00Z",
    "end": "2026-03-13T00:00:00Z"
  },
  "busyPeriods": [
    {
      "start": "2026-03-12T09:00:00.0000000",
      "end": "2026-03-12T09:30:00.0000000",
      "status": "busy",
      "subject": "Team Standup"
    }
  ]
}
```

---

### Tool 6: `calendar_create_event`

**Purpose:** Create a new calendar event, optionally with attendees, recurrence, online meeting (Teams), and other properties. When attendees are included, meeting invitations are sent automatically by the Graph API.

**Parameters:**

| Name | Type | Required | Description |
|---|---|---|---|
| `subject` | string | Yes | Event title |
| `start_datetime` | string | Yes | Start time, ISO 8601 without offset (e.g., `"2026-04-15T09:00:00"`). Timezone specified separately. |
| `start_timezone` | string | Yes | IANA timezone for start (e.g., `"America/New_York"`) |
| `end_datetime` | string | Yes | End time, same format as start |
| `end_timezone` | string | Yes | IANA timezone for end |
| `body` | string | No | Event body content. Interpreted as HTML if it contains tags, plain text otherwise. |
| `location` | string | No | Location display name (e.g., `"Conference Room A"`) |
| `attendees` | string | No | JSON array of attendee objects: `[{"email":"a@b.com","name":"Name","type":"required"}]`. Type is `required`, `optional`, or `resource`. |
| `is_online_meeting` | boolean | No | Set `true` to create a Teams meeting. Only works with work/school accounts. |
| `is_all_day` | boolean | No | If `true`, start and end must be midnight in the same timezone. |
| `importance` | string | No | `low`, `normal` (default), or `high` |
| `sensitivity` | string | No | `normal` (default), `personal`, `private`, or `confidential` |
| `show_as` | string | No | `free`, `tentative`, `busy` (default), `oof`, or `workingElsewhere` |
| `categories` | string | No | Comma-separated category display names (e.g., `"Blue Category,Red Category"`) |
| `recurrence` | string | No | JSON object defining recurrence. See Recurrence Patterns Reference below. |
| `reminder_minutes` | number | No | Minutes before start to trigger reminder |
| `calendar_id` | string | No | Target calendar ID. If omitted, creates in default calendar. |

**Registration:**

> **Note (CR-0039):** The tool description and `body`/`location` parameter descriptions have been enhanced with attendee quality guidance. The handler also injects an `_advisory` field into the response when attendees are present but body or location is missing. See CR-0039 for details.

```go
mcp.NewTool("calendar_create_event",
    mcp.WithDescription("Create a new calendar event. Supports attendees (sends invitations automatically), Teams online meetings, recurrence, and all standard event properties.\n\nIMPORTANT: When attendees are included, always provide a body (agenda or description) and location so recipients understand the purpose and place of the meeting. Ask the user for these details or suggest appropriate values before creating the event."),
    mcp.WithString("subject", mcp.Required(),
        mcp.Description("Event title"),
    ),
    mcp.WithString("start_datetime", mcp.Required(),
        mcp.Description("Start time in ISO 8601 without offset, e.g. 2026-04-15T09:00:00"),
    ),
    mcp.WithString("start_timezone", mcp.Required(),
        mcp.Description("IANA timezone for start time, e.g. America/New_York"),
    ),
    mcp.WithString("end_datetime", mcp.Required(),
        mcp.Description("End time in ISO 8601 without offset"),
    ),
    mcp.WithString("end_timezone", mcp.Required(),
        mcp.Description("IANA timezone for end time"),
    ),
    mcp.WithString("body",
        mcp.Description("Event body content (HTML or plain text). Strongly recommended when attendees are invited -- include the meeting agenda, purpose, or discussion topics. Attendees receive this in their invitation."),
    ),
    mcp.WithString("location",
        mcp.Description("Location display name (e.g. room name, office, or \"Microsoft Teams\"). Strongly recommended when attendees are invited. If an online meeting is enabled, you may use \"Microsoft Teams\" or omit this."),
    ),
    mcp.WithString("attendees",
        mcp.Description(`JSON array of attendees: [{"email":"a@b.com","name":"Name","type":"required|optional|resource"}]`),
    ),
    mcp.WithBoolean("is_online_meeting",
        mcp.Description("Set true to create a Teams online meeting (work/school accounts only)"),
    ),
    mcp.WithBoolean("is_all_day",
        mcp.Description("All-day event. Start/end must be midnight in the same timezone."),
    ),
    mcp.WithString("importance",
        mcp.Description("Event importance: low, normal, or high"),
    ),
    mcp.WithString("sensitivity",
        mcp.Description("Event sensitivity: normal, personal, private, or confidential"),
    ),
    mcp.WithString("show_as",
        mcp.Description("Free/busy status: free, tentative, busy, oof, or workingElsewhere"),
    ),
    mcp.WithString("categories",
        mcp.Description("Comma-separated category names"),
    ),
    mcp.WithString("recurrence",
        mcp.Description(`JSON recurrence object, e.g. {"pattern":{"type":"weekly","interval":1,"daysOfWeek":["monday"]},"range":{"type":"endDate","startDate":"2026-04-15","endDate":"2026-12-31"}}`),
    ),
    mcp.WithNumber("reminder_minutes",
        mcp.Description("Reminder minutes before start"),
    ),
    mcp.WithString("calendar_id",
        mcp.Description("Target calendar ID. Omit for default calendar."),
    ),
)
```

**Graph API call:** `POST /me/events` via `graphClient.Me().Events().Post(ctx, requestBody, nil)`, or `POST /me/calendars/{id}/events` via `graphClient.Me().Calendars().ByCalendarId(calId).Events().Post(ctx, requestBody, nil)`.

**SDK construction pattern:**
```go
requestBody := models.NewEvent()

subject := params["subject"].(string)
requestBody.SetSubject(&subject)

// Start time
start := models.NewDateTimeTimeZone()
start.SetDateTime(&startDT)
start.SetTimeZone(&startTZ)
requestBody.SetStart(start)

// End time
end := models.NewDateTimeTimeZone()
end.SetDateTime(&endDT)
end.SetTimeZone(&endTZ)
requestBody.SetEnd(end)

// Body (detect HTML vs text)
if bodyStr != "" {
    body := models.NewItemBody()
    if strings.Contains(bodyStr, "<") {
        contentType := models.HTML_BODYTYPE
        body.SetContentType(&contentType)
    } else {
        contentType := models.TEXT_BODYTYPE
        body.SetContentType(&contentType)
    }
    body.SetContent(&bodyStr)
    requestBody.SetBody(body)
}

// Location
if locationStr != "" {
    location := models.NewLocation()
    location.SetDisplayName(&locationStr)
    requestBody.SetLocation(location)
}

// Attendees (parsed from JSON string)
if attendeesJSON != "" {
    var attendeeList []struct {
        Email string `json:"email"`
        Name  string `json:"name"`
        Type  string `json:"type"`
    }
    json.Unmarshal([]byte(attendeesJSON), &attendeeList)

    var graphAttendees []models.Attendeeable
    for _, a := range attendeeList {
        att := models.NewAttendee()
        email := models.NewEmailAddress()
        email.SetAddress(&a.Email)
        email.SetName(&a.Name)
        att.SetEmailAddress(email)
        attType := parseAttendeeType(a.Type) // maps to models.REQUIRED_ATTENDEETYPE etc.
        att.SetTypeEscaped(&attType)
        graphAttendees = append(graphAttendees, att)
    }
    requestBody.SetAttendees(graphAttendees)
}

// Online meeting
if isOnline {
    requestBody.SetIsOnlineMeeting(&isOnline)
    provider := models.TEAMSFORBUSINESS_ONLINEMEETINGPROVIDERTYPE
    requestBody.SetOnlineMeetingProvider(&provider)
}

// Provenance tagging: stamp MCP-created events with a hidden extended property (CR-0040)
if provenancePropertyID != "" {
    requestBody.SetSingleValueExtendedProperties(
        []models.SingleValueLegacyExtendedPropertyable{
            graph.NewProvenanceProperty(provenancePropertyID),
        },
    )
}

createdEvent, err := graphClient.Me().Events().Post(ctx, requestBody, nil)
```

**Response:** Returns the full created event object (same schema as `calendar_get_event`), including server-generated fields like `id`, `iCalUId`, `webLink`, and `onlineMeeting.joinUrl` for Teams meetings. When provenance tagging is enabled (the default), the created event is stamped with a hidden single-value extended property identifying it as MCP-created (see CR-0040). When attendees are present but body or location is missing (and `is_online_meeting` is not set for location), the response includes an `_advisory` string field naming the missing fields and suggesting the LLM offer the user the option to add them (see CR-0039).

**Important behaviors:**

- When attendees are included, the Graph API **automatically sends meeting invitations**. No separate API call is needed.
- Teams online meeting properties (`onlineMeeting.joinUrl`, `onlineMeeting.conferenceId`) are **immutable after creation**. They cannot be changed via subsequent updates.
- The `dateTime` field must be ISO 8601 **without a UTC offset** (e.g., `"2026-04-15T09:00:00"`). The timezone is specified separately.
- Maximum attendees is 500 (Exchange Online limit).
- The `transactionId` property can be used for idempotency (preventing duplicate creation on retries), but is not exposed as a tool parameter; the handler may generate one internally.

---

### Tool 7: `calendar_update_event`

**Purpose:** Update properties of an existing calendar event. Uses PATCH semantics: only the specified fields are changed; omitted fields retain their current values. For meetings with attendees, the Graph API automatically sends update notifications.

**Parameters:**

| Name | Type | Required | Description |
|---|---|---|---|
| `event_id` | string | Yes | The unique event ID to update |
| `subject` | string | No | New event title |
| `start_datetime` | string | No | New start time (ISO 8601 without offset) |
| `start_timezone` | string | No | IANA timezone for new start time (required if `start_datetime` is provided) |
| `end_datetime` | string | No | New end time |
| `end_timezone` | string | No | IANA timezone for new end time (required if `end_datetime` is provided) |
| `body` | string | No | New event body (HTML or plain text) |
| `location` | string | No | New location display name |
| `attendees` | string | No | New attendees JSON array. **Replaces the entire attendee list.** |
| `is_all_day` | boolean | No | Change all-day status |
| `importance` | string | No | New importance: `low`, `normal`, or `high` |
| `sensitivity` | string | No | New sensitivity: `normal`, `personal`, `private`, or `confidential` |
| `show_as` | string | No | New free/busy status |
| `categories` | string | No | New comma-separated category names. Replaces all categories. |
| `recurrence` | string | No | New recurrence JSON, or `"null"` to remove recurrence. Only valid on series master events. |
| `reminder_minutes` | number | No | New reminder minutes |
| `is_reminder_on` | boolean | No | Enable or disable reminder |

**Registration:**

> **Note (CR-0039):** The tool description and `body`/`location` parameter descriptions have been enhanced with attendee quality guidance. The handler also injects an `_advisory` field into the response when the `attendees` parameter is provided but body or location is missing. See CR-0039 for details.

```go
mcp.NewTool("calendar_update_event",
    mcp.WithDescription("Update an existing calendar event. Only specified fields are changed (PATCH semantics). Automatically sends update notifications to attendees if applicable.\n\nIMPORTANT: When attendees are included, always provide a body (agenda or description) and location so recipients understand the purpose and place of the meeting. Ask the user for these details or suggest appropriate values before updating the event."),
    mcp.WithString("event_id", mcp.Required(),
        mcp.Description("The unique ID of the event to update"),
    ),
    mcp.WithString("subject",
        mcp.Description("New event title"),
    ),
    mcp.WithString("start_datetime",
        mcp.Description("New start time in ISO 8601 without offset"),
    ),
    mcp.WithString("start_timezone",
        mcp.Description("IANA timezone for new start time"),
    ),
    mcp.WithString("end_datetime",
        mcp.Description("New end time in ISO 8601 without offset"),
    ),
    mcp.WithString("end_timezone",
        mcp.Description("IANA timezone for new end time"),
    ),
    mcp.WithString("body",
        mcp.Description("New event body (HTML or plain text). Strongly recommended when attendees are invited -- include the meeting agenda, purpose, or discussion topics. Attendees receive this in their invitation."),
    ),
    mcp.WithString("location",
        mcp.Description("New location display name (e.g. room name, office, or \"Microsoft Teams\"). Strongly recommended when attendees are invited. If an online meeting is enabled, you may use \"Microsoft Teams\" or omit this."),
    ),
    mcp.WithString("attendees",
        mcp.Description(`New attendees JSON array (replaces entire list): [{"email":"a@b.com","name":"Name","type":"required"}]`),
    ),
    mcp.WithBoolean("is_all_day",
        mcp.Description("Change all-day status"),
    ),
    mcp.WithString("importance",
        mcp.Description("New importance: low, normal, or high"),
    ),
    mcp.WithString("sensitivity",
        mcp.Description("New sensitivity: normal, personal, private, or confidential"),
    ),
    mcp.WithString("show_as",
        mcp.Description("New free/busy status: free, tentative, busy, oof, or workingElsewhere"),
    ),
    mcp.WithString("categories",
        mcp.Description("New comma-separated category names (replaces all)"),
    ),
    mcp.WithString("recurrence",
        mcp.Description(`New recurrence JSON object, or "null" to remove. Only for series masters.`),
    ),
    mcp.WithNumber("reminder_minutes",
        mcp.Description("New reminder minutes before start"),
    ),
    mcp.WithBoolean("is_reminder_on",
        mcp.Description("Enable or disable the reminder"),
    ),
)
```

**Graph API call:** `PATCH /me/events/{id}` via `graphClient.Me().Events().ByEventId(eventID).Patch(ctx, requestBody, nil)`.

**SDK construction pattern:**
```go
requestBody := models.NewEvent()

// Only set fields that were provided in the request
if subject, ok := params["subject"].(string); ok {
    requestBody.SetSubject(&subject)
}
if startDT, ok := params["start_datetime"].(string); ok {
    start := models.NewDateTimeTimeZone()
    start.SetDateTime(&startDT)
    tz := params["start_timezone"].(string)
    start.SetTimeZone(&tz)
    requestBody.SetStart(start)
}
// ... repeat for each optional field ...

updatedEvent, err := graphClient.Me().Events().ByEventId(eventID).Patch(ctx, requestBody, nil)
```

**Response:** Returns the full updated event object (same schema as `calendar_get_event`). When the `attendees` parameter was provided in the request and is non-empty, but body or location is missing (and `is_online_meeting` is not set for location), the response includes an `_advisory` string field (see CR-0039).

**Series vs. occurrence updates:**

- To update an **entire recurring series**, PATCH the series master event (its `type` field is `"seriesMaster"`).
- To update a **single occurrence**, first retrieve occurrence IDs via `GET /me/events/{seriesMasterId}/instances?startDateTime=...&endDateTime=...`, then PATCH the specific occurrence by its ID.
- An occurrence **cannot be moved to or before the previous occurrence's day, or after the next occurrence's day**. The API returns `ErrorOccurrenceCrossingBoundary` (HTTP 400) if this constraint is violated.
- The API does **not** natively support "this and all following" updates. The workaround is to end the current series early and create a new series starting from the desired date.

**Caution with online meeting body content:** If updating the `body` of an event that has a Teams meeting, the body contains an HTML blob with the meeting join link. To preserve it, GET the event body first, apply changes while keeping the Teams blob intact, then PATCH. Removing the blob disables the online meeting.

---

### Tool 8: `calendar_delete_event`

**Purpose:** Delete a calendar event. When the authenticated user is the organizer of a meeting with attendees, the Graph API **automatically sends cancellation notices** to all attendees. The event is moved to the Deleted Items folder.

**Parameters:**

| Name | Type | Required | Description |
|---|---|---|---|
| `event_id` | string | Yes | The unique event ID to delete |

**Registration:**
```go
mcp.NewTool("calendar_delete_event",
    mcp.WithDescription("Delete a calendar event. If you are the organizer, cancellation notices are automatically sent to all attendees. Use calendar_cancel_event instead if you want to include a custom cancellation message."),
    mcp.WithString("event_id", mcp.Required(),
        mcp.Description("The unique ID of the event to delete"),
    ),
)
```

**Graph API call:** `DELETE /me/events/{id}` via `graphClient.Me().Events().ByEventId(eventID).Delete(ctx, nil)`.

**SDK pattern:**
```go
err := graphClient.Me().Events().ByEventId(eventID).Delete(ctx, nil)
if err != nil {
    slog.Error("delete event failed", "event_id", eventID, "error", formatGraphError(err))
    return mcp.NewToolResultError(formatGraphError(err)), nil
}
```

**Response:** The Graph API returns **204 No Content** with an empty body. The tool returns a confirmation message:

```json
{
  "deleted": true,
  "event_id": "AAMkAG...",
  "message": "Event deleted successfully. Cancellation notices were sent to attendees if applicable."
}
```

**Behavior notes:**

- Deleting a series master deletes **all occurrences** of the recurring event.
- Both organizers and attendees can call DELETE. When an organizer deletes, attendees receive auto-generated cancellation notices. When an attendee deletes, the event is removed only from their calendar.
- The event moves to Deleted Items and can potentially be recovered within the retention period.

---

### Tool 9: `calendar_cancel_event`

**Purpose:** Cancel a meeting and send a custom cancellation message to all attendees. This is the preferred method for cancelling meetings when you want to communicate a reason. Only the **organizer** can cancel a meeting.

**Parameters:**

| Name | Type | Required | Description |
|---|---|---|---|
| `event_id` | string | Yes | The unique event ID to cancel |
| `comment` | string | No | Custom cancellation message sent to all attendees. If omitted, a default system message is sent. |

**Registration:**
```go
mcp.NewTool("calendar_cancel_event",
    mcp.WithDescription("Cancel a meeting and send a cancellation message to all attendees. Only the organizer can cancel. Use calendar_delete_event for non-meeting events or when no custom message is needed."),
    mcp.WithString("event_id", mcp.Required(),
        mcp.Description("The unique ID of the event to cancel"),
    ),
    mcp.WithString("comment",
        mcp.Description("Custom cancellation message sent to attendees"),
    ),
)
```

**Graph API call:** `POST /me/events/{id}/cancel` via `graphClient.Me().Events().ByEventId(eventID).Cancel().Post(ctx, cancelBody, nil)`.

**SDK pattern:**
```go
cancelBody := graphusers.NewItemCancelPostRequestBody()
if comment != "" {
    cancelBody.SetComment(&comment)
}

err := graphClient.Me().Events().ByEventId(eventID).Cancel().Post(ctx, cancelBody, nil)
if err != nil {
    slog.Error("cancel event failed", "event_id", eventID, "error", formatGraphError(err))
    return mcp.NewToolResultError(formatGraphError(err)), nil
}
```

**Response:** The Graph API returns **202 Accepted**. The tool returns a confirmation message:

```json
{
  "cancelled": true,
  "event_id": "AAMkAG...",
  "message": "Meeting cancelled. Cancellation message sent to all attendees."
}
```

**Difference between delete and cancel:**

| Aspect | `calendar_delete_event` | `calendar_cancel_event` |
|--------|----------------|----------------|
| Who can call | Organizer or attendee | Organizer only |
| Custom message | No | Yes, via `comment` parameter |
| Attendee notification | Auto-generated system message | Custom message from organizer |
| Non-organizer behavior | Removes from own calendar only | Returns HTTP 400 error |
| Recurring events | Deleting master deletes all occurrences | Cancelling master cancels all future instances; cancelling an occurrence cancels just that one |

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

## Structured logging with `log/slog`

All logging MUST use Go's standard library `log/slog` package (introduced in Go 1.21). No third-party logging libraries (zerolog, zap, logrus, etc.) are permitted. Every log record MUST include the source file name and line number automatically via the handler's `AddSource` option.

### Logger initialization

The logger is created once at startup and set as the process-wide default via `slog.SetDefault()`. All subsequent code uses the package-level functions (`slog.Info(...)`, `slog.Debug(...)`, etc.) or retrieves the logger via `slog.Default()`. The `initLogger()` call MUST be the very first operation in `main()`, before any other code that might produce log output.

```go
import (
    "log/slog"
    "os"
    "strings"
)

func initLogger(levelStr string, format string) {
    var level slog.Level
    switch strings.ToLower(levelStr) {
    case "debug":
        level = slog.LevelDebug
    case "info":
        level = slog.LevelInfo
    case "warn":
        level = slog.LevelWarn
    case "error":
        level = slog.LevelError
    default:
        level = slog.LevelWarn
    }

    opts := &slog.HandlerOptions{
        AddSource: true,
        Level:     level,
    }

    var handler slog.Handler
    switch strings.ToLower(format) {
    case "text":
        handler = slog.NewTextHandler(os.Stderr, opts)
    default:
        handler = slog.NewJSONHandler(os.Stderr, opts)
    }

    slog.SetDefault(slog.New(handler))
}
```

### Critical constraint: all log output MUST go to stderr

The stdout stream is exclusively reserved for MCP JSON-RPC protocol messages over the stdio transport. Writing any log output to stdout will corrupt the protocol and break the MCP client connection. The `slog.NewJSONHandler` (or `slog.NewTextHandler`) constructor takes an `io.Writer` as its first argument; this MUST always be `os.Stderr`.

> **Note (CR-0023):** In addition to stderr, log output can optionally be written to a file via the `OUTLOOK_MCP_LOG_FILE` environment variable. When set, a `MultiHandler` fans out each log record to both stderr and the specified file. Stderr remains the primary output; file logging is a secondary, persistent destination. See CR-0023 for full details.

### Mandatory source location via `AddSource: true`

Setting `AddSource: true` in `slog.HandlerOptions` causes every log record to include a `source` field containing the file name, line number, and function name of the call site. This is populated automatically by the `slog` runtime using `runtime.Callers`. No manual injection of file/line information is needed. The only requirement is `AddSource: true`.

In JSON format, the source appears as:

```json
{
  "time": "2026-03-12T10:15:30.123Z",
  "level": "INFO",
  "source": {
    "function": "main.handleListEvents",
    "file": "/home/user/outlook-local-mcp/handlers.go",
    "line": 87
  },
  "msg": "listing events",
  "calendar_id": "AAMkAG...",
  "start": "2026-03-12T00:00:00Z",
  "end": "2026-03-13T00:00:00Z"
}
```

In text format, the same record renders as:

```
time=2026-03-12T10:15:30.123Z level=INFO source=/home/user/outlook-local-mcp/handlers.go:87 msg="listing events" calendar_id=AAMkAG... start=2026-03-12T00:00:00Z end=2026-03-13T00:00:00Z
```

### Log format

Use `slog.NewJSONHandler` as the default. JSON is preferred because MCP client log viewers (such as Claude Desktop's developer tools and the `~/Library/Logs/Claude/mcp-server-*.log` files on macOS) parse structured output more effectively. The server accepts a `OUTLOOK_MCP_LOG_FORMAT` environment variable set to `text` if a human-readable `key=value` format is desired for local debugging. Both formats include source file and line number.

### Log levels

The server supports four log levels, configured via the `OUTLOOK_MCP_LOG_LEVEL` environment variable (default: `warn`). Setting the level acts as a minimum threshold consistent with `slog`'s standard behavior.

| Level | slog constant | Use for |
|-------|---------------|---------|
| `debug` | `slog.LevelDebug` | Tool call parameters, Graph API request URLs/query params, token cache hits/misses, raw response field counts, pagination details |
| `info` | `slog.LevelInfo` | Server startup, successful authentication, tool registration, tool call completion with duration |
| `warn` | `slog.LevelWarn` | Persistent cache unavailable (falling back to in-memory), Graph API retries due to 429/503, deprecated parameter usage, corrupt auth record |
| `error` | `slog.LevelError` | Authentication failure, Graph API errors returned to the user, unrecoverable token refresh failure, stdio transport error |

### Required log points

The following events MUST be logged at the specified levels. Every log call MUST use structured key-value attributes (never string interpolation with `fmt.Sprintf` inside the message).

**Startup sequence (info):**
```go
slog.Info("server starting", "version", "1.0.0", "transport", "stdio")
slog.Info("persistent token cache initialized", "cache_name", cacheName)
// OR
slog.Warn("persistent token cache unavailable, using in-memory cache", "error", err)
slog.Info("authentication record loaded", "path", authRecordPath)
// OR
slog.Info("no authentication record found, device code flow required", "path", authRecordPath)
slog.Info("authentication successful", "tenant", record.TenantID)
slog.Info("authentication record saved", "path", authRecordPath)
slog.Info("graph client initialized", "scopes", []string{"Calendars.ReadWrite"})
slog.Info("tool registered", "tool", "calendar_list")
slog.Info("tool registered", "tool", "calendar_list_events")
slog.Info("tool registered", "tool", "calendar_get_event")
slog.Info("tool registered", "tool", "calendar_search_events")
slog.Info("tool registered", "tool", "calendar_get_free_busy")
slog.Info("tool registered", "tool", "calendar_create_event")
slog.Info("tool registered", "tool", "calendar_update_event")
slog.Info("tool registered", "tool", "calendar_delete_event")
slog.Info("tool registered", "tool", "calendar_cancel_event")
slog.Info("stdio transport started, waiting for requests")
```

**Tool call lifecycle:**
```go
// Entry (debug), log full parameters for diagnostics
slog.Debug("tool called",
    "tool", request.Params.Name,
    "params", request.Params.Arguments,
)

// Successful completion (info), log duration and result size
slog.Info("tool completed",
    "tool", request.Params.Name,
    "duration_ms", elapsed.Milliseconds(),
    "result_count", len(events),
)

// Tool-level error returned to MCP client (error)
slog.Error("tool failed",
    "tool", request.Params.Name,
    "error", formatGraphError(err),
    "duration_ms", elapsed.Milliseconds(),
)

// Parameter validation failure (warn)
slog.Warn("invalid tool parameters",
    "tool", request.Params.Name,
    "error", "start_datetime is required",
)
```

**Graph API interactions (debug):**
```go
slog.Debug("graph api request",
    "method", "GET",
    "path", "/me/calendarView",
    "query_params", map[string]any{
        "startDateTime": startDT,
        "endDateTime":   endDT,
        "$top":          top,
        "$select":       selectFields,
    },
)

slog.Debug("graph api response",
    "path", "/me/calendarView",
    "event_count", len(events),
    "has_next_page", nextLink != "",
)
```

**Authentication events:**
```go
slog.Debug("token cache hit, using cached credential")
slog.Warn("token refresh required, attempting silent refresh")
slog.Error("token refresh failed, re-authentication required", "error", err)
slog.Info("device code message displayed to user")
```

**Retry logic (warn):**
```go
slog.Warn("graph api rate limited, retrying",
    "attempt", attempt,
    "max_attempts", 3,
    "retry_after_seconds", retryAfter,
    "path", requestPath,
)

slog.Warn("graph api service unavailable, retrying",
    "attempt", attempt,
    "wait_seconds", 5,
    "path", requestPath,
)

slog.Error("graph api retries exhausted",
    "attempts", 3,
    "last_status", statusCode,
    "path", requestPath,
)
```

**Shutdown (info):**
```go
slog.Info("server shutting down", "reason", "stdin closed")
// OR
slog.Info("server shutting down", "reason", "signal received", "signal", sig.String())
```

### Per-handler context with `slog.With()`

For grouped log context (adding a persistent "tool" key to all messages within a handler), use `slog.With()`:

```go
func handleListEvents(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    log := slog.With("tool", "calendar_list_events")
    log.Debug("tool called", "params", request.Params.Arguments)
    // ... all subsequent log.Info/log.Error calls include tool=calendar_list_events automatically
}
```

Since `slog.SetDefault()` is called once at startup, all code outside handlers uses the package-level `slog.Info(...)`, `slog.Debug(...)`, etc. directly. There is no need to pass a logger instance through function parameters or struct fields.

### Testing

For unit tests, the default logger can be replaced by calling `slog.SetDefault(slog.New(testHandler))` in test setup, where `testHandler` writes to a `*bytes.Buffer` for assertion on log content.

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

## In-server documentation surface (CR-0061)

The server embeds a curated documentation bundle at compile time using Go `embed.FS` and exposes it through two complementary surfaces.

### MCP resources

Each embedded document is registered as an MCP resource:

| URI | MIME type | Description |
|-----|-----------|-------------|
| `doc://outlook-local-mcp/readme` | `text/markdown` | README |
| `doc://outlook-local-mcp/quickstart` | `text/markdown` | Quick Start guide |
| `doc://outlook-local-mcp/troubleshooting` | `text/markdown` | Troubleshooting guide |

Clients that support `resources/list` and `resources/read` can fetch any of these URIs directly. The bundle is intentionally limited to user-facing documents. Engineering documentation (Change Requests under `docs/cr/`, the reference spec, research notes, and `CHANGELOG.md`) is not embedded and is not exposed through this surface.

### System domain verbs

Three verbs on the `system` aggregate tool provide a deterministic lookup path for LLM clients that do not surface MCP resources natively:

| Verb | Description |
|------|-------------|
| `system.list_docs` | Returns the catalog of available documents (slug, title, one-line summary, tags, size). |
| `system.search_docs` | Case-insensitive keyword search across the bundle. Returns ranked results with slug, matched snippet (±2 lines), and 1-based line numbers. |
| `system.get_docs` | Fetches a document or a single section by heading anchor. Parameters: `slug` (required), `section` (optional heading anchor), `output` (`text` default, `raw` markdown). |

All three verbs are read-only, idempotent, local (no Graph API calls), and honour the three-tier output model from CR-0051 (`text` default, `raw` on request).

### Error `see` hints

When a tool handler wraps a known Graph API error class that has a corresponding section in `docs/troubleshooting.md`, the error payload includes a `see` field:

```
"see": "doc://outlook-local-mcp/troubleshooting#inefficient-filter"
```

The URI scheme matches the MCP resource URIs above. A build-time test (`TestErrorSeeTable_AnchorsCoverEmbeddedHeadings`) verifies that every anchor in the mapping table resolves to an actual `##` heading in the embedded troubleshooting document.

### Status entry point

`system.status` includes a `docs` section so the LLM can discover the surface from a single known entry point:

```json
"docs": {
  "base_uri": "doc://outlook-local-mcp/",
  "troubleshooting_slug": "troubleshooting",
  "version": "<build version>"
}
```

### Bundle constraints

The embedded bundle is enforced by build-time tests:

* Total uncompressed size must be under 2 MiB (`TestBundleSizeUnder2MiB`).
* The allowlist is explicit, not glob-based (`TestBundle_OnlyAllowedSlugsPresent`, `TestBundle_AllAllowedSlugsPresent`).
* Secret patterns (`eyJ`, `sk-`, `client_secret`, `refresh_token`) cause a build failure (`TestBundleContainsNoSecrets`).

The bundle is regenerated by `make docs-bundle`, which is wired into `make ci`.

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
| `OUTLOOK_MCP_MAIL_ENABLED` | `false` | Enable read-only mail access. When `true`, adds `Mail.Read` OAuth scope and registers 4 mail tools (`mail_list_folders`, `mail_list_messages`, `mail_search_messages`, `mail_get_message`). See CR-0043. |
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

```go
func main() {
    // Step 1: Logger FIRST
    logLevel := getEnv("OUTLOOK_MCP_LOG_LEVEL", "warn")
    logFormat := getEnv("OUTLOOK_MCP_LOG_FORMAT", "json")
    initLogger(logLevel, logFormat)

    slog.Info("server starting", "version", "1.0.0", "transport", "stdio")

    // Step 2: Load config
    cfg := loadConfig()

    // Step 3: Init cache
    persistentCache, err := cache.New(&cache.Options{Name: cfg.CacheName})
    if err != nil {
        slog.Warn("persistent token cache unavailable, using in-memory cache", "error", err)
    } else {
        slog.Info("persistent token cache initialized", "cache_name", cfg.CacheName)
    }

    // Step 4: Load auth record
    record := loadAuthRecord(cfg.AuthRecordPath)

    // Step 5: Create credential
    cred, err := azidentity.NewDeviceCodeCredential(&azidentity.DeviceCodeCredentialOptions{
        ClientID:             cfg.ClientID,
        TenantID:             cfg.TenantID,
        Cache:                persistentCache,
        AuthenticationRecord: record,
        UserPrompt: func(ctx context.Context, msg azidentity.DeviceCodeMessage) error {
            fmt.Fprintf(os.Stderr, "\n%s\n\n", msg.Message)
            slog.Info("device code message displayed to user")
            return nil
        },
    })
    if err != nil {
        slog.Error("credential creation failed", "error", err)
        os.Exit(1)
    }

    // Step 6: Authenticate if needed
    if record == (azidentity.AuthenticationRecord{}) {
        record, err = cred.Authenticate(context.Background(), nil)
        if err != nil {
            slog.Error("authentication failed", "error", err)
            os.Exit(1)
        }
        slog.Info("authentication successful", "tenant", record.TenantID)
        if err := saveAuthRecord(cfg.AuthRecordPath, record); err != nil {
            slog.Warn("failed to save authentication record", "path", cfg.AuthRecordPath, "error", err)
        }
    }

    // Step 7: Create Graph client
    graphClient, err := msgraphsdk.NewGraphServiceClientWithCredentials(cred, []string{"Calendars.ReadWrite"})
    if err != nil {
        slog.Error("graph client initialization failed", "error", err)
        os.Exit(1)
    }
    slog.Info("graph client initialized", "scopes", []string{"Calendars.ReadWrite"})

    // Step 8: Create MCP server
    s := server.NewMCPServer("outlook-local", "1.0.0",
        server.WithToolCapabilities(false),
        server.WithRecovery(),
    )

    // Step 9: Register tools
    registerTools(s, graphClient)

    // Step 10: Start stdio transport
    slog.Info("stdio transport started, waiting for requests")
    if err := server.ServeStdio(s); err != nil {
        slog.Error("stdio transport error", "error", err)
        os.Exit(1)
    }

    slog.Info("server shutting down", "reason", "stdin closed")
}
```

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

---

## Conclusion

This specification defines a self-contained Go binary that bridges Microsoft Outlook calendar and mail management to any MCP-compatible AI client. The design prioritizes zero-friction setup: the user runs the binary, enters a device code once, and immediately has calendar tools available to their AI assistant, covering the full lifecycle of calendar events (list, search, create, update, delete, cancel). Optionally, four read-only mail tools can be enabled for email access and event-email correlation (see CR-0043). The Microsoft Office client ID (`d3590ed6-52b3-4102-aeff-aad2292ab01c`) is the only viable well-known client ID for `Calendars.ReadWrite` without app registration. The persistent token cache via `azidentity/cache` plus `AuthenticationRecord` serialization ensures the device code prompt appears only on first use. Structured logging via `log/slog` with mandatory `AddSource: true` provides full observability (including source file and line number on every log record) while keeping all output on stderr to protect the MCP protocol stream on stdout. Read-only tool handlers follow a consistent pattern: validate parameters, call Graph API v1.0 with explicit `$select` for minimal data transfer, nil-check all pointer fields, serialize to JSON, and return as `mcp.NewToolResultText`. Write tool handlers construct Kiota-generated model objects using the pointer-heavy setter pattern, with only specified fields set to leverage PATCH partial-update semantics. The entire server lifecycle, from OAuth handshake to MCP tool dispatch, runs within a single process with no external dependencies beyond the Go binary itself.
