# CR-0056 Validation Report

Branch: `dev/cr-0056`
Validated commit: a2e9c34 (`checkpoint(CR-0056): CR finalized`)
Date: 2026-04-19

## Summary

Requirements: 53/53 | Acceptance Criteria: 19/19 | Tests: 38/38 | Gaps: 0 (4 previously identified gaps now FIXED)

Quality checks:

- `go build ./...` — PASS (no output, clean build)
- `make lint` — PASS (`golangci-lint run` → `0 issues.`)
- `make test` — PASS (all packages `ok`, no FAIL; cached; `ld` warnings are pre-existing macOS linker noise unrelated to this CR)

## Requirement Verification

| Req # | Description | Status | Evidence |
|-------|-------------|--------|----------|
| FR-1 | `AccountConfig.UPN` field with `json:"upn"` | PASS | `internal/auth/accounts.go:41` |
| FR-2 | `account_add` resolves UPN from `/me` and persists | PASS | `internal/tools/add_account.go:256,340` via `auth.EnsureEmailAndPersistUPN` |
| FR-3 | `account_add` sets `entry.Email` after authentication | PASS | `internal/tools/add_account.go` EnsureEmailAndPersistUPN sets Email (`internal/auth/email_resolver.go:69`) |
| FR-4 | `RestoreAccounts` populates Email from persisted `upn` without API call | PASS | `internal/auth/restore.go:183-189` |
| FR-5 | `EnsureEmail` persists newly resolved UPN when config UPN empty | PASS | `internal/auth/email_resolver.go:69-95` (EnsureEmailAndPersistUPN → UpdateAccountUPN) |
| FR-6 | `account` parameter resolves label then UPN | PASS | `internal/auth/account_resolver.go:241` (GetByUPN fallback) |
| FR-7 | `GetByUPN` case-insensitive UPN match | PASS | `internal/auth/registry.go:177-191` (`strings.EqualFold`) |
| FR-8 | `resolveAccount` calls `Get` first then `GetByUPN` | PASS | `internal/auth/account_resolver.go:~238-245` |
| FR-9 | `account_login` tool provided | PASS | `internal/tools/login_account.go:31` |
| FR-10 | required `label` parameter | PASS | `login_account.go:46-49` |
| FR-11 | errors when already authenticated | PASS | `login_account.go:103-105` |
| FR-12 | uses persisted auth_method/client_id/tenant_id | PASS | `login_account.go:107-118` |
| FR-13 | same inline auth flow as `account_add` | PASS | `login_account.go:122-138` (reuses `addAccountState.setupCredential` / `authenticateInline`) |
| FR-14 | sets Authenticated=true, creates Graph client, refreshes UPN | PASS | `login_account.go:140-166` |
| FR-15 | five MCP annotations | PASS | `login_account.go:33-37`; test `TestLoginAccount_Annotations` |
| FR-16 | `account_logout` tool provided | PASS | `internal/tools/logout_account.go:33` |
| FR-17 | required `label` parameter | PASS | `logout_account.go:50-53` |
| FR-18 | clears Authenticated, Client, Credential, Authenticator | PASS | `logout_account.go:97-102` |
| FR-19 | clears cached token from keychain | PASS | `logout_account.go:107` (`auth.ClearTokenCache`); test `TestLogoutAccount_ClearsTokenCache` |
| FR-20 | errors when already disconnected | PASS | `logout_account.go:91-93` |
| FR-21 | does not remove from registry/accounts.json | PASS | `logout_account.go` (only mutates fields; test `TestLogoutAccount_PreservesConfig`) |
| FR-22 | five MCP annotations | PASS | `logout_account.go:35-39`; `TestLogoutAccount_Annotations` |
| FR-23 | `account_refresh` tool provided | PASS | `internal/tools/refresh_account.go:35` |
| FR-24 | required `label` parameter | PASS | `refresh_account.go:52-55` |
| FR-25 | errors when disconnected | PASS | `refresh_account.go:95-97` |
| FR-26 | calls GetToken for force refresh | PASS | `refresh_account.go:102-105` (CAE-enabled `TokenRequestOptions`) |
| FR-27 | returns new expiry | PASS | `refresh_account.go:111-117`; test `TestRefreshAccount_ExpiryInResponse` |
| FR-28 | five MCP annotations | PASS | `refresh_account.go:37-41`; `TestRefreshAccount_Annotations` |
| FR-29 | `account_list` shows all accounts with UPN, label, auth_method, state | PASS | `internal/tools/list_accounts.go`; tests pass |
| FR-30 | text format `N. label — upn (state, auth_method)` | PASS | `internal/tools/text_format.go` `FormatAccountsText`; `TestFormatAccountsText_WithUPNAndMethod` |
| FR-31 | `FormatStatusText` includes UPN and auth method | PASS | `text_format.go`; `TestFormatStatusText_WithUPN` |
| FR-32 | `statusAccount` includes UPN and AuthMethod | PASS | `internal/tools/status.go` |
| FR-33 | `FormatAccountLine` includes UPN when available | PASS | `internal/tools/text_format.go` |
| FR-34 | elicitation enum shows `label (upn)` | PASS | `account_resolver.go` elicitAccountSelection; `TestElicitation_ShowsUPN` |
| FR-35 | elicitation includes disconnected accounts with state | PASS | `account_resolver.go`; `TestElicitation_ShowsUPN` |
| FR-36 | disconnected selection returns actionable error | PASS | `account_resolver.go`; `TestResolveAccount_DisconnectedExplicit` |
| FR-37 | zero-authenticated + disconnected lists UPNs with account_login | PASS | `account_resolver.go`; `TestResolveAccount_AllDisconnected` |
| FR-38 | explicit account on disconnected returns specific error | PASS | `TestResolveAccount_DisconnectedExplicit` |
| FR-39 | account_add description mentions UPN | PASS | `add_account.go` description text |
| FR-40 | account_list description mentions UPN/auth_method/disconnected | PASS | `list_accounts.go` |
| FR-41 | account_remove description mentions `account_logout` | PASS | `remove_account.go` |
| FR-42 | account param description notes label and UPN accepted | PASS | `client.go:35` (`AccountParamDescription`) |
| FR-43 | account_remove clears keychain token cache | PASS | `remove_account.go`; `TestRemoveAccount_ClearsTokenCache` |
| FR-44 | account_remove succeeds regardless of Authenticated state | PASS | `TestRemoveAccount_AllowsDisconnected` |
| FR-45 | last-account removal leaves clean state | PASS | `TestRemoveAccount_LastAccountCleanState` |
| FR-46 | zero-account resolver error mentions account_add, not account_login | PASS | `account_resolver.go`; `TestResolveAccount_ZeroAccounts_NoLoginHint` |
| FR-47 | account_list succeeds with zero accounts | PASS | `TestListAccounts_ZeroAccounts` |
| FR-48 | status succeeds with zero accounts | PASS | `TestStatus_ZeroAccounts` |
| FR-49 | tool descriptions forbid default-account assumption | PASS | `AccountParamDescription` in `client.go:35`; new tool descriptions include this guidance |
| FR-50 | account param description states silent vs advisory auto-select rules | PASS | `client.go:35` AccountParamDescription |
| FR-51 | account_list/status descriptions state they are authoritative source | PASS | `list_accounts.go`, `status.go` descriptions |
| FR-52 | auto-select advisory names disconnected siblings by UPN | PASS | `account_resolver.go`; `TestAutoSelect_AdvisoryForDisconnectedSiblings` |
| FR-53 | lifecycle tool descriptions direct LLM to proactively suggest | PASS | login/logout/refresh/remove descriptions each include "Proactively suggest" guidance |
| NFR-1 | each new tool in its own file | PASS | `login_account.go`, `logout_account.go`, `refresh_account.go` |
| NFR-2 | Go doc comments | PASS | Reviewed files |
| NFR-3 | existing tests pass | PASS | `make test` all `ok` |
| NFR-4 | manifest updated | PASS | `extension/manifest.json:140-148` |
| NFR-5 | CRUD test doc updated | PASS | `docs/prompts/mcp-tool-crud-test.md` (per checkpoint Phase 7) |
| NFR-6 | tool count updated in server.go | PASS | `internal/server/server.go` registers 3 new tools |

## Acceptance Criteria Verification

| AC # | Description | Status | Evidence |
|------|-------------|--------|----------|
| AC-1 | UPN persisted after account_add | PASS | `add_account.go:256,340` invokes `EnsureEmailAndPersistUPN`; `TestAddAccount_PersistsUPN` in `internal/tools/add_account_test.go` asserts `accounts.json` carries the resolved UPN after the persistence helper runs. |
| AC-2 | UPN available at startup without API call | PASS | `restore.go:183-189`; `TestRestoreAccounts_PopulatesEmailFromUPN` in `internal/auth/restore_test.go` asserts `entry.Email` equals the persisted UPN and that `GraphClientFactory` is never invoked during restore. |
| AC-3 | Dual lookup by UPN | PASS | `TestResolveAccount_ByUPN` |
| AC-4 | account_login re-authenticates disconnected | PASS | `TestLoginAccount_Success` |
| AC-5 | account_login rejects already connected | PASS | `TestLoginAccount_AlreadyConnected` |
| AC-6 | account_logout disconnects without removing | PASS | `TestLogoutAccount_Success`, `TestLogoutAccount_PreservesConfig` |
| AC-7 | account_refresh forces renewal | PASS | `TestRefreshAccount_Success`, `TestRefreshAccount_ExpiryInResponse` |
| AC-8 | Elicitation shows UPN and state | PASS | `TestElicitation_ShowsUPN` |
| AC-9 | Disconnected selection returns error with account_login hint | PASS | `TestResolveAccount_DisconnectedExplicit` |
| AC-10 | account_list shows full state for all accounts | PASS | `TestFormatAccountsText_WithUPNAndMethod` |
| AC-11 | status shows UPN and auth_method | PASS | `TestFormatStatusText_WithUPN` |
| AC-12 | Zero auth + disconnected correct error | PASS | `TestResolveAccount_AllDisconnected` |
| AC-13 | account_remove clears keychain + works on disconnected | PASS | `TestRemoveAccount_ClearsTokenCache`, `TestRemoveAccount_AllowsDisconnected` |
| AC-14 | Last-account removal clean zero-state | PASS | `TestRemoveAccount_LastAccountCleanState` |
| AC-15 | list + status succeed zero-account | PASS | `TestListAccounts_ZeroAccounts`, `TestStatus_ZeroAccounts` |
| AC-16 | Extension manifest includes new tools | PASS | `extension/manifest.json:140-148`; `TestManifest_NewTools` in `extension/manifest_test.go` parses the manifest and asserts `account_login`, `account_logout`, and `account_refresh` are present. |
| AC-17 | Auto-select advisory for disconnected siblings | PASS | `TestAutoSelect_AdvisoryForDisconnectedSiblings` |
| AC-18 | Descriptions forbid silent default assumption | PASS | `client.go:35`; `TestAccountParamDescription_ForbidsDefaultAssumption` and `TestAccountLifecycleTools_DescribeProactiveSuggestion` in `internal/tools/tool_descriptions_test.go` lock in the required language. |
| AC-19 | All quality checks pass | PASS | build/lint/test all green |

## Test Strategy Verification

| Test File | Test Name | Specified | Exists | Matches Spec |
|-----------|-----------|-----------|--------|--------------|
| registry_test.go | TestGetByUPN_Found | Yes | Yes | Yes |
| registry_test.go | TestGetByUPN_NotFound | Yes | Yes | Yes |
| registry_test.go | TestUpdate_ModifiesEntry | Yes | Yes | Yes |
| registry_test.go | TestUpdate_NotFound | Yes | Yes | Yes |
| accounts_test.go | TestUpdateAccountUPN_Success | Yes | Yes | Yes |
| accounts_test.go | TestUpdateAccountUPN_NotFound | Yes | Yes | Yes |
| account_resolver_test.go | TestResolveAccount_ByUPN | Yes | Yes | Yes |
| account_resolver_test.go | TestResolveAccount_DisconnectedExplicit | Yes | Yes | Yes |
| account_resolver_test.go | TestResolveAccount_AllDisconnected | Yes | Yes | Yes |
| account_resolver_test.go | TestElicitation_ShowsUPN | Yes | Yes | Yes |
| account_resolver_test.go | TestAutoSelect_AdvisoryForDisconnectedSiblings | Yes | Yes | Yes |
| account_resolver_test.go | TestResolveAccount_ZeroAccounts_NoLoginHint | Yes | Yes | Yes |
| login_account_test.go | TestLoginAccount_Success | Yes | Yes | Yes |
| login_account_test.go | TestLoginAccount_AlreadyConnected | Yes | Yes | Yes |
| login_account_test.go | TestLoginAccount_NotFound | Yes | Yes | Yes |
| logout_account_test.go | TestLogoutAccount_Success | Yes | Yes | Yes |
| logout_account_test.go | TestLogoutAccount_AlreadyDisconnected | Yes | Yes | Yes |
| logout_account_test.go | TestLogoutAccount_PreservesConfig | Yes | Yes | Yes |
| logout_account_test.go | TestLogoutAccount_ClearsKeychain | Yes | Yes (named `TestLogoutAccount_ClearsTokenCache`) | Equivalent — slight rename |
| refresh_account_test.go | TestRefreshAccount_Success | Yes | Yes | Yes |
| refresh_account_test.go | TestRefreshAccount_Disconnected | Yes | Yes | Yes |
| refresh_account_test.go | TestRefreshAccount_ReturnsExpiry | Yes | Yes (named `TestRefreshAccount_ExpiryInResponse`) | Equivalent — slight rename |
| tool_annotations_test.go | TestLoginAccount_Annotations | Yes | Yes | Yes |
| tool_annotations_test.go | TestLogoutAccount_Annotations | Yes | Yes | Yes |
| tool_annotations_test.go | TestRefreshAccount_Annotations | Yes | Yes | Yes |
| text_format_test.go | TestFormatAccountsText_WithUPNAndMethod | Yes | Yes | Yes |
| text_format_test.go | TestFormatStatusText_WithUPN | Yes | Yes | Yes |
| add_account_test.go | TestAddAccount_PersistsUPN | Yes | Yes | Yes — asserts UPN persisted via `auth.EnsureEmailAndPersistUPN` |
| restore_test.go | TestRestoreAccounts_PopulatesEmailFromUPN | Yes | Yes | Yes — asserts Email populated from persisted UPN with zero Graph calls |
| remove_account_test.go | TestRemoveAccount_ClearsKeychain | Yes | Yes (named `TestRemoveAccount_ClearsTokenCache`) | Equivalent — slight rename |
| remove_account_test.go | TestRemoveAccount_Disconnected | Yes | Yes (named `TestRemoveAccount_AllowsDisconnected`) | Equivalent — slight rename |
| remove_account_test.go | TestRemoveAccount_LastAccountCleanState | Yes | Yes | Yes |
| list_accounts_test.go | TestListAccounts_ZeroAccounts | Yes | Yes | Yes |
| status_test.go | TestStatus_ZeroAccounts | Yes | Yes | Yes |
| manifest_test.go | TestManifest_NewTools | Yes | Yes | Yes — `extension/manifest_test.go` added |
| tool_descriptions_test.go | TestAccountParamDescription_ForbidsDefaultAssumption | Yes | Yes | Yes — `internal/tools/tool_descriptions_test.go` added |
| tool_descriptions_test.go | TestAccountLifecycleTools_DescribeProactiveSuggestion | Yes | Yes | Yes — `internal/tools/tool_descriptions_test.go` added |
| text_format_test.go (modify) | TestFormatAccountsText | Yes | Yes | Yes — updated to new format |
| text_format_test.go (modify) | TestFormatStatusText | Yes | Yes | Yes — updated to new format |
| account_resolver_test.go (modify) | TestElicitAccountSelection | Yes | Yes | Yes — enum uses label+UPN |
| list_accounts_test.go (modify) | existing tests | Yes | Yes | Yes — include auth_method |

## Gaps

All four previously identified gaps have been FIXED.

1. **FIXED — `TestAddAccount_PersistsUPN`** (AC-1). Added to `internal/tools/add_account_test.go`. The test seeds `accounts.json` with a freshly added entry (empty UPN), simulates the post-authentication state by pre-populating `entry.Email`, invokes the same `auth.EnsureEmailAndPersistUPN` helper that `add_account.go:256,340` calls on success, and asserts the persisted record now carries `upn == "alice@contoso.com"`. **Scope note:** `HandleAddAccount` cannot be driven end-to-end without an interactive auth flow and a live Graph client, so the test targets the closest deterministic surface — the persistence helper that owns the UPN-write contract.

2. **FIXED — `TestRestoreAccounts_PopulatesEmailFromUPN`** (AC-2). Added to `internal/auth/restore_test.go`. Seeds `accounts.json` with `upn: "alice@contoso.com"`, calls `RestoreAccounts` using the existing `fakeCredentialFactory` + `countingGraphClientFactory`, and asserts `entry.Email == "alice@contoso.com"` plus `GraphClientFactory` call-count == 0 (the strongest in-process signal that no Graph API call was made).

3. **FIXED — `TestManifest_NewTools`** (AC-16). Added `extension/manifest_test.go` (with a one-line `extension/doc.go` so the directory is a valid Go package). The test parses `manifest.json` and asserts `tools[]` contains `account_login`, `account_logout`, and `account_refresh`.

4. **FIXED — `TestAccountParamDescription_ForbidsDefaultAssumption` + `TestAccountLifecycleTools_DescribeProactiveSuggestion`** (AC-18, FR-49/50/53). Added `internal/tools/tool_descriptions_test.go`. Asserts `AccountParamDescription` contains the phrases "Never assume a default account" and "disconnected", and that the descriptions of `account_login`, `account_logout`, `account_refresh`, and `account_remove` each contain "proactively suggest".

All four tests pass locally under `make lint` and `make test` on `dev/cr-0056`.
