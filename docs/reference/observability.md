# Observability

Reference documentation for structured logging, audit logging, and OpenTelemetry instrumentation. For a user-facing overview, see the observability section in [docs/concepts.md](../concepts.md).

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

## Audit logging

The `internal/audit` package wraps every MCP tool handler via `AuditWrap` middleware. Every tool invocation produces a structured audit record containing:

- Tool name (domain + operation in `{domain}.{operation}` form)
- Caller identity (account UPN when available)
- Timestamp and duration
- Success or error outcome
- OpenTelemetry trace ID for correlation

Audit records are written to the same `slog` handler chain as application logs but always at `info` level regardless of the configured log level, ensuring they are never suppressed.

---

## OpenTelemetry attributes and metrics

The `internal/observability` package instruments every tool invocation with OpenTelemetry spans and metrics via the `WithObservability` middleware.

### Span attributes

Each tool invocation span carries the following attributes:

| Attribute | Type | Example |
|-----------|------|---------|
| `mcp.tool` | string | `"calendar"` |
| `mcp.operation` | string | `"list_events"` |
| `mcp.fqn` | string | `"calendar.list_events"` |
| `mcp.account` | string | `"user@example.com"` |
| `mcp.outcome` | string | `"ok"` / `"error"` |
| `mcp.error_code` | string | Graph OData error code, if any |

### Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `mcp.tool.invocations` | Counter | Total tool invocations, labelled by `mcp.fqn` and `mcp.outcome` |
| `mcp.tool.duration_ms` | Histogram | Tool invocation duration in milliseconds, labelled by `mcp.fqn` |

OpenTelemetry export is configured via the standard `OTEL_EXPORTER_*` environment variables. When no exporter is configured, the SDK uses a no-op exporter and adds no overhead.
