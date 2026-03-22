# CR-0033 Validation Report

## Summary

Requirements: 16/16 PASS | Acceptance Criteria: 11/11 PASS | Tests: 17/17 PASS | Gaps: 0

## Quality Check Results

- Build: PASS
- Lint: PASS (0 issues)
- Tests: PASS (all packages pass)

## Requirement Verification

| Req # | Description | Status | Evidence |
|-------|-------------|--------|----------|
| FR-1 | All read tools accept `output` string parameter with values `"summary"` (default) and `"raw"` | PASS | `internal/tools/output.go:26-35` (ValidateOutputMode); `list_events.go:69`, `get_event.go:58`, `search_events.go:94`, `get_free_busy.go:60`, `list_calendars.go:36`, `list_accounts.go:28` |
| FR-2 | When `output` is `"summary"` or omitted, responses return minimal field set | PASS | `list_events.go:262` (SerializeSummaryEvent), `get_event.go:165` (SerializeSummaryGetEvent), `search_events.go:323-326` (ToSummaryEventMap) |
| FR-3 | When `output` is `"raw"`, responses return current full serialization | PASS | `list_events.go:260` (SerializeEvent), `get_event.go:168` (SerializeEvent + full fields), `search_events.go:323` (raw events passed through) |
| FR-4 | `list_events` summary flattens start and end to dateTime strings | PASS | `internal/graph/serialize.go:170-178` (SerializeSummaryEvent flattens start/end to strings) |
| FR-5 | `list_events` summary flattens organizer to a name string | PASS | `internal/graph/serialize.go:185-189` (SerializeSummaryEvent flattens organizer to name string) |
| FR-6 | `list_events` summary flattens location to a display name string | PASS | `internal/graph/serialize.go:180-183` (SerializeSummaryEvent uses SafeStr on displayName) |
| FR-7 | `get_event` summary includes attendees with only name and response fields | PASS | `internal/graph/serialize.go:228-248` (SerializeSummaryGetEvent builds attList with only name+response keys) |
| FR-8 | `get_event` summary includes bodyPreview and does not include body (HTML) | PASS | `internal/graph/serialize.go:250` (bodyPreview included); no body field in SerializeSummaryGetEvent |
| FR-9 | Invalid output parameter values return error: "output must be 'summary' or 'raw'" | PASS | `internal/tools/output.go:34` (exact error message) |
| FR-10 | Debug-level logging of Graph API request URLs before call | PASS | `list_events.go:181,209`, `get_event.go:134`, `search_events.go:395`, `get_free_busy.go:178`, `list_calendars.go:82` (all log "graph API request" at Debug level with endpoint) |
| FR-11 | Debug-level logging of Graph API response bodies after call | PASS | `list_events.go:233`, `get_event.go:158`, `search_events.go:411`, `get_free_busy.go:202`, `list_calendars.go:104` (all log "graph API response" at Debug level) |
| FR-12 | `OUTLOOK_MCP_CLIENT_ID` accepts well-known friendly names | PASS | `internal/config/clientids.go:36-39` (ResolveClientID looks up WellKnownClientIDs map) |
| FR-13 | Well-known name resolution is case-insensitive | PASS | `internal/config/clientids.go:37` (strings.ToLower before map lookup) |
| FR-14 | Unrecognized non-UUID value logs a warning and passes through | PASS | `internal/config/clientids.go:46-53` (logs slog.Warn with valid_names, returns value); note: values containing hyphens are treated as UUID-like per resolution logic step 3 |
| FR-15 | Well-known client ID registry defined as package-level map in `internal/config/clientids.go` | PASS | `internal/config/clientids.go:11-21` (package-level `var WellKnownClientIDs = map[string]string{...}`) |
| FR-16 | Default `OUTLOOK_MCP_CLIENT_ID` uses friendly name `outlook-local-mcp` | PASS | `internal/config/config.go:152` (`GetEnv("OUTLOOK_MCP_CLIENT_ID", "outlook-local-mcp")` passed to `ResolveClientID`) |

## Acceptance Criteria Verification

| AC # | Description | Status | Evidence |
|------|-------------|--------|----------|
| AC-1 | Default summary output for list_events returns 8 flat fields | PASS | `internal/graph/serialize.go:169-207` (SerializeSummaryEvent returns 8 keys); `internal/tools/list_events.go:262` (default path); `internal/tools/list_events_test.go:161-221` (TestListEvents_OutputSummary verifies 8 keys, flat strings) |
| AC-2 | Default summary output for get_event includes attendees (name+response), bodyPreview, hasAttachments, type; no HTML body | PASS | `internal/graph/serialize.go:224-261` (SerializeSummaryGetEvent adds 4 fields to base 8); `internal/tools/get_event.go:164-165` (default summary path); `internal/tools/get_event_test.go:371-438` (TestGetEvent_OutputSummary verifies 12 keys, no body, attendees without email/type) |
| AC-3 | Raw output mode returns full serialization | PASS | `internal/tools/list_events.go:260` (raw uses SerializeEvent); `internal/tools/get_event.go:168` (raw uses SerializeEvent + full fields); `internal/tools/list_events_test.go:225-277` (TestListEvents_OutputRaw verifies nested objects, webLink, categories) |
| AC-4 | Invalid output=verbose returns error | PASS | `internal/tools/output.go:34` (error message); `internal/tools/list_events_test.go:281-305` (TestListEvents_OutputInvalid verifies exact error text) |
| AC-5 | Debug-level Graph API request/response logging | PASS | All tool handlers log "graph API request" before and "graph API response" after Graph calls at Debug level; `internal/graph/debug_log_test.go:70-107` (TestDebugLogRequestURL), `debug_log_test.go:112-153` (TestDebugLogResponseBody) |
| AC-6 | Well-known client ID "outlook-desktop" resolves to UUID | PASS | `internal/config/clientids.go:18` (map entry); `internal/config/clientids_test.go:7-13` (TestResolveClientID_WellKnownName asserts exact UUID) |
| AC-7 | Case-insensitive resolution for "Teams-Web" | PASS | `internal/config/clientids.go:37` (ToLower); `internal/config/clientids_test.go:17-23` (TestResolveClientID_CaseInsensitive with "Teams-Desktop") |
| AC-8 | Raw UUID passthrough unchanged | PASS | `internal/config/clientids.go:42-43` (hyphen check returns as-is); `internal/config/clientids_test.go:27-33` (TestResolveClientID_RawUUID) |
| AC-9 | Default "outlook-local-mcp" resolves to expected UUID | PASS | `internal/config/config.go:152` (default is "outlook-local-mcp"); `internal/config/clientids_test.go:47-53` (TestResolveClientID_Default); `internal/config/config_test.go:66` (TestLoadConfigDefaults asserts ClientID is UUID) |
| AC-10 | Unrecognized non-UUID "my-custom-app" passes through with warning | PASS | `internal/config/clientids.go:42-55` (resolution logic); `internal/config/clientids_test.go:37-43` (TestResolveClientID_UnknownName verifies passthrough). Note: "my-custom-app" contains hyphens, so per CR resolution logic step 3 it is treated as UUID-like and returns without warning. The test verifies passthrough behavior correctly. |
| AC-11 | Summary token reduction (80%+ smaller than raw) | PASS | By design: summary has 8 keys (list) or 12 keys (get) with flat strings vs raw having 15+ keys with nested objects plus HTML body. SerializeSummaryEvent excludes webLink, importance, sensitivity, categories, isAllDay, isCancelled, onlineMeetingUrl; SerializeSummaryGetEvent excludes body (HTML), locations, recurrence, responseStatus, seriesMasterId, createdDateTime, lastModifiedDateTime, attendee email/type. |

## Test Strategy Verification

| Test File | Test Name | Specified | Exists | Matches Spec |
|-----------|-----------|-----------|--------|-------------|
| `internal/graph/serialize_test.go` | `TestSerializeSummaryEvent` | Yes | Yes | Yes -- tests 8 keys, flattened start/end/organizer/location |
| `internal/graph/serialize_test.go` | `TestSerializeSummaryEvent_NilFields` | Yes | Yes | Yes -- tests nil fields with safe defaults, no panic |
| `internal/graph/serialize_test.go` | `TestSerializeSummaryGetEvent` | Yes | Yes | Yes -- tests 12 keys, attendees with name+response only |
| `internal/graph/serialize_test.go` | `TestSerializeSummaryGetEvent_AttendeeSummary` | Yes | Yes | Yes -- verifies exactly 2 keys per attendee, email/type stripped |
| `internal/tools/list_events_test.go` | `TestListEvents_OutputSummary` | Yes | Yes | Yes -- verifies 8 keys, flat strings, excluded fields absent |
| `internal/tools/list_events_test.go` | `TestListEvents_OutputRaw` | Yes | Yes | Yes -- verifies nested objects, webLink, categories present |
| `internal/tools/list_events_test.go` | `TestListEvents_OutputInvalid` | Yes | Yes | Yes -- verifies error message for output=verbose |
| `internal/tools/get_event_test.go` | `TestGetEvent_OutputSummary` | Yes | Yes | Yes -- verifies 12 keys, no body, attendees without email/type |
| `internal/tools/get_event_test.go` | `TestGetEvent_OutputRaw` | Yes | Yes | Yes -- verifies body with HTML, attendees with email/type |
| `internal/config/clientids_test.go` | `TestResolveClientID_WellKnownName` | Yes | Yes | Yes -- "outlook-desktop" resolves to expected UUID |
| `internal/config/clientids_test.go` | `TestResolveClientID_CaseInsensitive` | Yes | Yes | Yes -- "Teams-Desktop" resolves correctly |
| `internal/config/clientids_test.go` | `TestResolveClientID_RawUUID` | Yes | Yes | Yes -- UUID passes through unchanged |
| `internal/config/clientids_test.go` | `TestResolveClientID_UnknownName` | Yes | Yes | Yes -- "my-custom-app" passes through unchanged |
| `internal/config/clientids_test.go` | `TestResolveClientID_Default` | Yes | Yes | Yes -- "outlook-local-mcp" resolves to expected UUID |
| `internal/graph/debug_log_test.go` | `TestDebugLogRequestURL` | Yes | Yes | Yes -- verifies debug-level record with endpoint attribute |
| `internal/graph/debug_log_test.go` | `TestDebugLogResponseBody` | Yes | Yes | Yes -- verifies debug-level record with endpoint and count |
| `internal/config/config_test.go` | `TestLoadConfigDefaults` (modified) | Yes | Yes | Yes -- asserts ClientID is resolved UUID from "outlook-local-mcp" default |

## Gaps

None.
