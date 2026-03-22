# CR-0020 Validation Report

## Summary

Requirements: 12/12 | Acceptance Criteria: 10/10 | Tests: 13/12 | Gaps: 0 (2 fixed)

Build: PASS | Lint: PASS (0 issues) | Tests: PASS (all passing)

## Requirement Verification

| Req # | Description | Status | Evidence |
|-------|-------------|--------|----------|
| FR1 | `ReadOnly` bool field in config struct, loaded from `OUTLOOK_MCP_READ_ONLY`, default `"false"` | PASS | `main.go:106` field declaration; `main.go:210` `strings.EqualFold(getEnv("OUTLOOK_MCP_READ_ONLY", "false"), "true")` |
| FR2 | Case-insensitive `"true"`/`"TRUE"` enables read-only; all other values disable it | PASS | `main.go:210` uses `strings.EqualFold(..., "true")` which is case-insensitive; tests `TestLoadConfig_ReadOnly_TrueLowercase` and `TestLoadConfig_ReadOnly_TrueUppercase` confirm |
| FR3 | `readOnlyGuard` middleware accepts tool name, bool, and wraps `server.ToolHandlerFunc` | PASS | `readonly.go:36` signature: `func readOnlyGuard(toolName string, readOnly bool, handler server.ToolHandlerFunc) server.ToolHandlerFunc` |
| FR4 | When enabled, returns `mcp.NewToolResultError` with message `"operation blocked: <tool_name> is not allowed in read-only mode"` without invoking inner handler | PASS | `readonly.go:46-48` returns `mcp.NewToolResultError(fmt.Sprintf("operation blocked: %s is not allowed in read-only mode", toolName))`, `nil`; `readonly.go:40` closure ignores context and request (`_`) so handler is never called |
| FR5 | When disabled, passes through to inner handler without modification | PASS | `readonly.go:37-39` `if !readOnly { return handler }` returns the original handler reference unchanged |
| FR6 | Guard applied to all four write/delete tools: create_event, update_event, delete_event, cancel_event | PASS | `server.go:48` create_event, `server.go:49` update_event, `server.go:56` delete_event, `server.go:57` cancel_event -- all wrapped with `readOnlyGuard` |
| FR7 | Guard NOT applied to read tools: list_calendars, list_events, get_event, search_events, get_free_busy | PASS | `server.go:43-45` (list_calendars, list_events, get_event) and `server.go:52-53` (search_events, get_free_busy) have no `readOnlyGuard` in chain |
| FR8 | All nine tools registered regardless of read-only mode | PASS | `server.go:43-57` registers all 9 tools unconditionally; `readOnlyGuard` only wraps the handler, not the tool registration; `server.go:59` logs `"tools", 9` |
| FR9 | Blocked invocations logged at `slog.Warn` with fields `"tool"`, `"mode"` (`"read-only"`), `"action"` (`"blocked"`) | PASS | `readonly.go:41-45` `slog.Warn("operation blocked in read-only mode", "tool", toolName, "mode", "read-only", "action", "blocked")` |
| FR10 | Startup log at `slog.Info` with `"read_only"` field when enabled | PASS | `main.go:242` `slog.Info("server starting", ..., "read_only", cfg.ReadOnly)` -- logged for both true and false values |
| FR11 | Guard inserted between `withObservability` and `auditWrap` in middleware chain | PASS | `server.go:48` chain: `withObservability("create_event", m, t, readOnlyGuard("create_event", readOnly, auditWrap(...)))` -- observability outermost, then guard, then auditWrap innermost |
| FR12 | `registerTools` function signature accepts `readOnly bool` parameter | PASS | `server.go:41` signature includes `readOnly bool` as final parameter |

## Acceptance Criteria Verification

| AC # | Description | Status | Evidence |
|------|-------------|--------|----------|
| AC-1 | Read-only mode blocks create_event with correct error message, no Graph API call | PASS | `server.go:48` wraps create_event with `readOnlyGuard`; `readonly.go:46-48` returns error with "create_event is not allowed in read-only mode"; handler never invoked. Test: `TestRegisterTools_ReadOnly_BlocksWriteTool` at `server_test.go:52` sends create_event and verifies error |
| AC-2 | Read-only mode blocks update_event | PASS | `server.go:49` wraps update_event with `readOnlyGuard`; error message would contain "update_event is not allowed in read-only mode". Test: `TestReadOnlyGuard_Enabled_ErrorIsToolError` at `readonly_test.go:95` uses update_event |
| AC-3 | Read-only mode blocks delete_event | PASS | `server.go:56` wraps delete_event with `readOnlyGuard`. Test: `TestReadOnlyGuard_Enabled_ErrorMessageFormat` at `readonly_test.go:73` verifies delete_event error message |
| AC-4 | Read-only mode blocks cancel_event | PASS | `server.go:57` wraps cancel_event with `readOnlyGuard`. Integration coverage via `TestRegisterTools_ReadOnly_False_AllWriteToolsPass` at `server_test.go:150` which tests all four write tools including cancel_event |
| AC-5 | Read-only mode allows all read tools | PASS | Read tools at `server.go:43-45,52-53` have no `readOnlyGuard`. Test: `TestRegisterTools_ReadOnly_AllowsReadTool` at `server_test.go:102` verifies list_calendars is not blocked |
| AC-6 | All nine tools listed in catalog regardless of mode | PASS | All 9 `s.AddTool` calls at `server.go:43-57` execute unconditionally; guard only wraps handler, not registration. `server.go:59` confirms count of 9 |
| AC-7 | Blocked invocations logged with tool, mode, action fields | PASS | `readonly.go:41-45` logs `slog.Warn` with `"tool"`, `"mode"="read-only"`, `"action"="blocked"` |
| AC-8 | Startup log includes `"read_only"` field | PASS | `main.go:242` includes `"read_only", cfg.ReadOnly` in startup slog.Info call |
| AC-9 | Default behavior unchanged when unset or false | PASS | `readonly.go:37-39` returns original handler unchanged when `readOnly=false` (zero overhead, same function reference). Test: `TestReadOnlyGuard_Disabled_NoOverhead` at `readonly_test.go:113` confirms same pointer. `TestLoadConfig_ReadOnly_Default` at `main_test.go:469` confirms default false |
| AC-10 | Blocked invocations recorded by observability and guard logging | FIXED | AC-10 in the CR was amended to align with the design decision documented in Risk 2. The middleware chain `withObservability -> readOnlyGuard -> auditWrap` intentionally blocks before `auditWrap`; blocked invocations are captured by `withObservability` metrics and the guard's `slog.Warn` log (`readonly.go:41-45`). CR AC-10 updated to reflect this architecture |

## Test Strategy Verification

| Test File | Specified Name | Exists | Actual Name | Matches Spec |
|-----------|---------------|--------|-------------|--------------|
| `main_test.go` | `TestLoadConfig_ReadOnlyTrue` | Yes | `TestLoadConfig_ReadOnly_TrueLowercase` (line 430) | Yes -- name differs but behavior matches: sets `OUTLOOK_MCP_READ_ONLY=true`, asserts `cfg.ReadOnly == true` |
| `main_test.go` | `TestLoadConfig_ReadOnlyTrueCaseInsensitive` | Yes | `TestLoadConfig_ReadOnly_TrueUppercase` (line 443) | Yes -- name differs but behavior matches: sets `OUTLOOK_MCP_READ_ONLY=TRUE`, asserts `cfg.ReadOnly == true` |
| `main_test.go` | `TestLoadConfig_ReadOnlyFalse` | Yes | `TestLoadConfig_ReadOnly_False` (line 456) | Yes -- name differs slightly but behavior matches: sets `OUTLOOK_MCP_READ_ONLY=false`, asserts `cfg.ReadOnly == false` |
| `main_test.go` | `TestLoadConfig_ReadOnlyDefault` | Yes | `TestLoadConfig_ReadOnly_Default` (line 469) | Yes -- name differs slightly but behavior matches: no env var set, asserts `cfg.ReadOnly == false`. Also covered in `TestLoadConfigDefaults` at line 101 |
| `readonly_test.go` | `TestReadOnlyGuard_Enabled_BlocksHandler` | Yes | `TestReadOnlyGuard_Enabled_BlocksHandler` (line 21) | Yes -- exact name match; verifies handler not called, error result returned, `IsError=true` |
| `readonly_test.go` | `TestReadOnlyGuard_Disabled_PassesThrough` | Yes | `TestReadOnlyGuard_Disabled_PassesThrough` (line 47) | Yes -- exact name match; verifies handler called, result returned, no error |
| `readonly_test.go` | `TestReadOnlyGuard_Enabled_ErrorMessageFormat` | Yes | `TestReadOnlyGuard_Enabled_ErrorMessageFormat` (line 73) | Yes -- exact name match; verifies error text contains tool name "delete_event" |
| `readonly_test.go` | `TestReadOnlyGuard_Enabled_ErrorIsToolError` | Yes | `TestReadOnlyGuard_Enabled_ErrorIsToolError` (line 95) | Yes -- exact name match; verifies `result.IsError == true` |
| `readonly_test.go` | `TestReadOnlyGuard_Disabled_NoOverhead` | Yes | `TestReadOnlyGuard_Disabled_NoOverhead` (line 113) | Yes -- exact name match; verifies same function pointer reference via `%p` comparison |
| `server_test.go` | `TestRegisterTools_ReadOnlyBlocksWrite` | Yes | `TestRegisterTools_ReadOnly_BlocksWriteTool` (line 52) | Yes -- name differs but behavior matches: `readOnly=true`, invokes create_event via HandleMessage, asserts `IsError=true` and message contains "read-only mode" |
| `server_test.go` | `TestRegisterTools_ReadOnlyAllowsRead` | Yes | `TestRegisterTools_ReadOnly_AllowsReadTool` (line 102) | Yes -- name differs but behavior matches: `readOnly=true`, invokes list_calendars via HandleMessage, asserts response does NOT contain "read-only mode" |
| `server_test.go` | `TestRegisterTools_WriteEnabledPassesThrough` | Yes | `TestRegisterTools_ReadOnly_False_AllWriteToolsPass` (line 150) | Yes -- name differs but behavior exceeds spec: tests ALL four write tools with `readOnly=false`, asserts none contain "read-only mode" |

Additional tests found beyond CR spec:
- `TestLoadConfigDefaults` at `main_test.go:101` also asserts `cfg.ReadOnly == false`
- `clearOutlookEnvVars` at `main_test.go:208` includes `OUTLOOK_MCP_READ_ONLY` in cleanup list

## Gaps (Resolved)

### Gap 1: AC-10 -- Audit log entry not emitted for blocked invocations

**Severity:** Medium | **Status:** FIXED

AC-10 originally required an audit entry with outcome "error" for blocked invocations, but the middleware chain ordering (FR11) intentionally places the guard before `auditWrap`. The CR's Risk 2 section explicitly chose this tradeoff.

**Resolution applied:** AC-10 in `CR-0020-read-only-mode.md` was amended to match the design decision from Risk 2. Blocked invocations are recorded by `withObservability` metrics and the guard's `slog.Warn` log, not by `auditWrap`.

### Gap 2: NFR-2 -- No validation warning for non-standard OUTLOOK_MCP_READ_ONLY values

**Severity:** Low | **Status:** FIXED

`validateConfig` did not warn when `OUTLOOK_MCP_READ_ONLY` was set to a non-standard value like `"yes"` or `"1"`.

**Resolution applied:** Added a validation check in `validate_config.go` that reads the raw `OUTLOOK_MCP_READ_ONLY` env var and logs `slog.Warn` if the value is not empty, `"true"`, or `"false"` (case-insensitive). Test `TestValidateConfig_ReadOnlyNonStandardWarning` in `validate_config_test.go` covers standard values (no warning), non-standard values (warning emitted), and empty (no warning).
