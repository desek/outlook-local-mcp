# CR-0037 Validation Report

## Summary

Requirements: 21/21 | Acceptance Criteria: 17/17 | Tests: 28/28 | Gaps: 0 (5 fixed, post-validation fix for device_code cache persistence)

Build: PASS | Lint: PASS (0 issues) | Tests: PASS (all packages)

## Requirement Verification

| Req # | Description | Status | Evidence |
|-------|-------------|--------|----------|
| FR-1 | Account resolver falls back to "default" on any elicitation error | PASS | `internal/auth/account_resolver.go:185-196` -- `elicitAccountSelection` catches all errors from `s.elicit()` and falls back to `registry.Get("default")` |
| FR-2 | Account resolver considers only authenticated accounts | FIXED | `internal/auth/account_resolver.go:139` uses `s.registry.ListAuthenticated()`. `Authenticated: true` now set in `main.go:113`, `add_account.go:224,298`, and `restore.go:174`. |
| FR-3 | Auto-select sole authenticated account | FIXED | `internal/auth/account_resolver.go:145-147` auto-selects when `len(authenticated) == 1`. `Authenticated: true` now set in all production registration paths. |
| FR-4 | Actionable error when fallback to "default" fails | PASS | `internal/auth/account_resolver.go:190-194` -- error includes `strings.Join(labels, ", ")` and `"'account' parameter"` hint |
| FR-5 | Token cache persists to disk when CGo disabled | PASS | `internal/auth/cache_nocgo.go:53-90` -- `InitCache` returns a file-based `azidentity.Cache` backed by `encryptedFileAccessor` at `~/.outlook-local-mcp/{name}.bin` |
| FR-6 | File-based cache uses AES-256-GCM encryption | PASS | `internal/auth/filecache.go:211-228` -- `encryptAESGCM` uses `aes.NewCipher` (32-byte key) + `cipher.NewGCM` |
| FR-7 | Cache file created with 0600 permissions | PASS | `internal/auth/filecache.go:121` -- `os.WriteFile(a.path, ciphertext, 0600)` |
| FR-8 | Startup silent token probe with 5s timeout | PASS | `cmd/outlook-local-mcp/main.go:196,221` -- `startupProbeTimeout = 5 * time.Second`, `probeStartupToken` calls `cred.GetToken` with bounded context |
| FR-9 | Startup probe must not trigger interactive auth | PASS | `cmd/outlook-local-mcp/main.go:215-218` -- device_code is skipped entirely; browser/auth_code use silent `GetToken` only |
| FR-10 | Auth middleware detects fresh-credential and skips Graph API timeout | PASS | `internal/auth/middleware.go:198-201` -- checks `!state.preAuthenticated.Load() && !state.authenticated.Load()`, creates `freshErr` and calls `handleAuthError` directly |
| FR-10a | Device code probe checks auth record and marks pre-authenticated | PASS | `cmd/outlook-local-mcp/main.go:225-232` -- `os.Stat(authRecordPath)` succeeds → `markPreAuthenticated()` called, middleware uses normal Graph API path with cached tokens |
| FR-11 | Timezone detection reads OS-level sources before UTC fallback | PASS | `internal/config/timezone.go:26-61` -- checks TZ env, `/etc/localtime` (macOS), `/etc/timezone` (Linux), then UTC |
| FR-12 | AccountRegistry exposes ListAuthenticated() | PASS | `internal/auth/registry.go:191-207` -- `ListAuthenticated()` filters by `entry.Authenticated == true` and returns sorted slice |
| FR-13 | Auth error messages include LLM-actionable recovery, no raw SDK names | PASS | `internal/auth/errors.go:91-97` -- `FormatAuthError` appends recovery steps mentioning `list_accounts` and `add_account`; `stripSDKClassNames` at line 156-163 removes SDK names |
| FR-14 | FormatAuthError is sole path for auth error formatting | PASS | `internal/auth/middleware.go:182,335,391,499,506,586,593` -- all auth error paths use `FormatAuthError()` |
| FR-15 | Read-only status tool registered | PASS | `internal/tools/status.go:25-30` -- `NewStatusTool()` with `mcp.WithReadOnlyHintAnnotation(true)`. `internal/server/server.go:90` registers it. Returns version, timezone, accounts, uptime. |
| FR-16 | Account parameter description updated on all calendar tools | PASS | All 9 tools use `"Account label to use. If omitted, the default account is used. Use list_accounts to see available accounts."` -- confirmed via grep at `list_calendars.go:33`, `list_events.go:68`, `get_event.go:55`, `search_events.go:91`, `get_free_busy.go:57`, `create_event.go:96`, `update_event.go:87`, `delete_event.go:41`, `cancel_event.go:45` |
| FR-17 | RestoreAccounts skips GetToken for device_code | PASS | `internal/auth/restore.go:157-181` -- `if acct.AuthMethod != "device_code"` guards the GetToken call; device_code accounts get `entry.Client = nil` |
| FR-18 | list_events accepts optional date parameter | PASS | `internal/tools/list_events.go:47-49` -- `mcp.WithString("date", ...)` with description mentioning ISO 8601 and "today" |
| FR-19 | When date provided, start/end become optional | PASS | `internal/tools/list_events.go:114-128` -- `expandDateParam` fills in `startDatetime`/`endDatetime` when empty; only falls through to validation if both are still empty |
| FR-20 | When both date and start/end provided, start/end take precedence | PASS | `internal/tools/list_events.go:114` -- `if startDatetime == "" || endDatetime == ""` only enters date expansion when at least one is missing; existing values are never overwritten (lines 121-126) |

## Acceptance Criteria Verification

| AC # | Description | Status | Evidence |
|------|-------------|--------|----------|
| AC-1 | Fallback on any elicitation error | PASS | `account_resolver.go:185-196` catches all errors (not just `ErrElicitationNotSupported`). Test: `TestElicitAccountSelection_AnyError_FallsBackToDefault` at `account_resolver_test.go:456` |
| AC-2 | Filter by authenticated state | FIXED | `account_resolver.go:139` uses `ListAuthenticated()`. `Authenticated: true` now set in `main.go:113`, `add_account.go:224,298`, and `restore.go:174`. |
| AC-3 | Actionable error when no default + elicitation fails | PASS | `account_resolver.go:190-194` lists labels and hints `'account' parameter`. Test: `TestElicitAccountSelection_AnyError_NoDefault_ReturnsAccountList` at `account_resolver_test.go:490` |
| AC-4 | Token cache persists across restarts (non-CGo) | PASS | `cache_nocgo.go:53-90` creates file-based cache. Test: `TestFileCache_PersistAndReload` at `filecache_test.go:15` |
| AC-5 | Cache file encrypted with AES-256-GCM and 0600 permissions | PASS | `filecache.go:211-228` (AES-256-GCM), `filecache.go:121` (0600). Tests: `TestFileCache_Encryption` at `filecache_test.go:46`, `TestFileCache_Permissions` at `filecache_test.go:131` |
| AC-6 | Corrupt cache handled gracefully | PASS | `filecache.go:86-91` -- on decryption failure, logs warning, removes file, returns nil. Test: `TestFileCache_CorruptionRecovery` at `filecache_test.go:78` |
| AC-7 | No 30s delay for fresh credentials | PASS | `middleware.go:198-201` -- fresh-credential fast-path skips inner handler. Test: `TestMiddleware_FreshCredential_ImmediateAuthPrompt` at `middleware_test.go:1632` verifies < 5s |
| AC-8 | Startup probe completes within 5s, no interactive auth | PASS | `main.go:196,212-234` -- 5s timeout, device_code skipped. Test: `TestStartupTokenProbe_CompletesWithin5Seconds` at `main_test.go:48` |
| AC-8a | Device code probe uses auth record for pre-authentication | PASS | `main.go:225-232` -- `os.Stat(authRecordPath)` check, calls `markPreAuthenticated()` when file exists. Tests: `device_code_with_auth_record_marks_pre_authenticated` and `device_code_skipped_no_auth_record` at `main_test.go:109,89` |
| AC-9 | Timezone auto-detected from OS | PASS | `timezone.go:26-61` -- TZ env, /etc/localtime, /etc/timezone fallbacks. Test: `TestDetectTimezone_FallsBackToOSSources` at `timezone_test.go:181` |
| AC-10 | LLM-actionable error messages, no raw SDK class names | PASS | `errors.go:91-97,140-163` -- `FormatAuthError` strips SDK names, adds recovery steps. Tests: `TestFormatAuthError_NoRawSDKStrings` at `errors_test.go:165`, `TestFormatAuthError_IncludesRecoverySteps` at `errors_test.go:196` |
| AC-10a | FormatAuthError is sole formatting path | PASS | `middleware.go` -- all error paths use `FormatAuthError()`. Test: `TestMiddleware_AllAuthErrors_UseFormatAuthError` at `middleware_test.go:1779` |
| AC-11 | Status tool returns health summary | PASS | `status.go:74-103` -- returns JSON with version, timezone, accounts, uptime. Test: `TestStatus_ReturnsHealthSummary` at `status_test.go:16` |
| AC-12 | Tool descriptions reflect actual fallback behavior | PASS | All 9 calendar tools confirmed via grep: `"Account label to use. If omitted, the default account is used. Use list_accounts to see available accounts."` None mention elicitation prompt. |
| AC-13 | No startup crash-loop from device_code accounts | PASS | `restore.go:157-181` -- device_code skips GetToken, logs info instead. Test: `TestRestoreOne_DeviceCode_SkipsGetToken` at `restore_test.go:246` |
| AC-14 | Date convenience parameter on list_events | PASS | `list_events.go:47-49,114-128,324-346` -- `expandDateParam` handles "today" and ISO dates. Test: `TestListEvents_DateToday_ExpandsToStartEndOfDay` at `list_events_test.go:333` |
| AC-15 | Date parameter backward compatibility | PASS | `list_events.go:114` -- date expansion only runs when `startDatetime == ""` or `endDatetime == ""`. Test: `TestListEvents_DateAndExplicitRange_ExplicitWins` at `list_events_test.go:423` |
| AC-16 | No regression for elicitation-supporting clients | PASS | `account_resolver.go:199-213` -- successful elicitation returns selected account without fallback. Test: `TestResolveAccount_ElicitationClient_NoFallback` at `account_resolver_test.go:631` |

## Test Strategy Verification

| Test File | Test Name | Specified | Exists | Matches Spec |
|-----------|-----------|-----------|--------|--------------|
| `internal/auth/account_resolver_test.go` | `TestElicitAccountSelection_AnyError_FallsBackToDefault` | Yes | Yes (line 456) | Yes -- elicit returns `fmt.Errorf("Method not found")`, default account returned |
| `internal/auth/account_resolver_test.go` | `TestElicitAccountSelection_AnyError_NoDefault_ReturnsAccountList` | Yes | Yes (line 490) | Yes -- error contains labels and `'account' parameter` hint |
| `internal/auth/account_resolver_test.go` | `TestResolveAccount_OnlyAuthenticatedConsidered` | Yes | Yes (line 520) | Yes -- 1 authenticated + 1 unauthenticated, authenticated auto-selected |
| `internal/auth/account_resolver_test.go` | `TestResolveAccount_ZeroAuthenticated_ReturnsError` | Yes | Yes (line 561) | Yes -- 2 unauthenticated, error contains "no authenticated accounts" |
| `internal/auth/account_resolver_test.go` | `TestResolveAccount_MultipleAuthenticated_ElicitsSelection` | Yes | Yes (line 596) | Yes -- 2 authenticated, elicitation called |
| `internal/auth/filecache_test.go` | `TestFileCache_PersistAndReload` | Yes | Yes (line 15) | Yes -- write, close, reopen with new accessor, read matches |
| `internal/auth/filecache_test.go` | `TestFileCache_Encryption` | Yes | Yes (line 46) | Yes -- raw bytes differ from plaintext |
| `internal/auth/filecache_test.go` | `TestFileCache_CorruptionRecovery` | Yes | Yes (line 78) | Yes -- corrupt file returns nil, file removed |
| `internal/auth/filecache_test.go` | `TestFileCache_Permissions` | Yes | Yes (line 131) | Yes -- verifies 0600 permissions |
| `internal/auth/registry_test.go` | `TestListAuthenticated_FiltersCorrectly` | Yes | Yes (line 278) | Yes -- mix of authenticated/unauthenticated, only authenticated returned |
| `internal/auth/errors_test.go` | `TestFormatAuthError_NoRawSDKStrings` | Yes | Yes (line 165) | Yes -- tests 5 SDK error variants, checks all 5 banned substrings |
| `internal/auth/errors_test.go` | `TestFormatAuthError_IncludesRecoverySteps` | Yes | Yes (line 196) | Yes -- checks `list_accounts` and `add_account` in output |
| `internal/auth/restore_test.go` | `TestRestoreOne_DeviceCode_SkipsGetToken` | Yes | Yes (line 246) | Yes -- device_code account registered with Client=nil, factory not called |
| `internal/auth/restore_test.go` | `TestRestoreOne_Browser_AttemptsGetToken` | Yes | Yes (line 303) | Yes -- browser account goes through silent auth path |
| `internal/tools/status_test.go` | `TestStatus_ReturnsHealthSummary` | Yes | Yes (line 16) | Yes -- verifies version, timezone, accounts array, uptime |
| `internal/tools/status_test.go` | `TestStatus_NoGraphAPICalls` | Yes | Yes (line 80) | Yes -- 100ms timeout context, no network calls |
| `internal/tools/list_events_test.go` | `TestListEvents_DateToday_ExpandsToStartEndOfDay` | Yes | Yes (line 333) | Yes -- date="today" expands to 00:00:00/23:59:59 in Europe/Stockholm |
| `internal/tools/list_events_test.go` | `TestListEvents_DateISO_ExpandsToStartEndOfDay` | Yes | Yes (line 381) | Yes -- "2026-03-17" expands to 00:00:00/23:59:59 |
| `internal/tools/list_events_test.go` | `TestListEvents_DateAndExplicitRange_ExplicitWins` | Yes | Yes (line 423) | Yes -- explicit 2026-06-01 values used despite date="today" |
| `internal/tools/list_events_test.go` | `TestListEvents_NoDateNoRange_ReturnsError` | Yes | Yes (line 467) | Yes -- empty args returns "start_datetime is required" error |
| `internal/auth/middleware_test.go` | `TestMiddleware_AllAuthErrors_UseFormatAuthError` | Yes | Yes (line 1779) | Yes -- tests browser, device_code, and fresh-credential error paths for banned SDK names and required recovery tools |
| `internal/auth/middleware_test.go` | `TestMiddleware_FreshCredential_ImmediateAuthPrompt` | Yes | Yes (line 1632) | Yes -- verifies handler called once (retry only), elapsed < 5s, auth called |
| `cmd/outlook-local-mcp/main_test.go` | `TestStartupTokenProbe_CompletesWithin5Seconds` | Yes | Yes (line 48) | Yes -- 5 subtests: success, failure, device_code skip (no record), device_code with auth record, slow credential timeout |
| `cmd/outlook-local-mcp/main_test.go` | `TestStartupTokenProbe_.../device_code_with_auth_record_marks_pre_authenticated` | Yes (AC-8a) | Yes (line 109) | Yes -- verifies markPreAuthenticated called when auth record exists |
| `cmd/outlook-local-mcp/main_test.go` | `TestStartupTokenProbe_.../device_code_skipped_no_auth_record` | Yes (AC-8a) | Yes (line 89) | Yes -- verifies markPreAuthenticated NOT called when no auth record |
| `internal/config/timezone_test.go` | `TestTimezoneDetection_FallsBackToOSSources` | Yes | Yes (line 181) | Yes -- unsets TZ, verifies non-"Local" result on macOS/Linux |
| `internal/tools/tool_description_test.go` | `TestCalendarTools_AccountParamDescription` | Yes | FIXED (line 16) | Yes -- verifies all 9 calendar tools have "the default account is used" and do not contain "you will be prompted to select" |
| `internal/auth/account_resolver_test.go` | `TestResolveAccount_ElicitationClient_NoFallback` | Yes | Yes (line 631) | Yes -- elicitation succeeds, "work" selected (not "default"), no fallback |

## Gaps (all fixed)

### Gap 1: `Authenticated` field never set to `true` in production code -- FIXED

**Affected Requirements**: FR-2, FR-3, AC-2

**Fix**: Added `Authenticated: true` to AccountEntry literals in all production registration paths:
- `cmd/outlook-local-mcp/main.go:113` -- default account
- `internal/tools/add_account.go:224` -- pending auth path
- `internal/tools/add_account.go:298` -- inline auth path
- `internal/auth/restore.go:174` -- after successful silent auth

### Gap 2: Missing test file `internal/tools/tool_description_test.go` -- FIXED

**Affected Requirements**: AC-12

**Fix**: Created `internal/tools/tool_description_test.go` with `TestCalendarTools_AccountParamDescription` (line 16). Verifies all 9 calendar tools have "the default account is used" and do not contain "you will be prompted to select".

### Gap 3: `add_account` entries never set `Authenticated: true` -- FIXED

**Affected Requirements**: FR-2, FR-3

**Fix**: See Gap 1 -- `add_account.go:224` (pending path) and `add_account.go:298` (inline path).

### Gap 4: Restored accounts with active tokens never set `Authenticated: true` -- FIXED

**Affected Requirements**: FR-2, FR-3

**Fix**: See Gap 1 -- `restore.go:174` sets `entry.Authenticated = true` after successful silent auth.

### Gap 5: `Authenticated` field in status tool output always `false` -- FIXED

**Affected Requirements**: AC-11

**Fix**: Resolved by Gaps 1, 3, 4. Production accounts now have correct `Authenticated` state, so the status tool reports accurate values.
