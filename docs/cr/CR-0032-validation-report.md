# CR-0032 Validation Report

## Summary

Requirements: 10/10 | Acceptance Criteria: 8/8 | Tests: 13/13 | Gaps: 0

Build: PASS | Lint: PASS (0 issues) | Tests: 13/13 packages pass

## Requirement Verification

| Req # | Description | Status | Evidence |
|-------|-------------|--------|----------|
| FR-1  | AccountEntry struct MUST include ClientID, TenantID, and AuthMethod fields | PASS | `internal/auth/registry.go:34-45` -- fields declared with doc comments |
| FR-2  | add_account tool MUST populate identity fields in AccountEntry before registry addition | PASS | `internal/tools/add_account.go:207-217` -- entry constructed with clientID, tenantID, authMethod before registry.Add |
| FR-3  | add_account tool MUST persist account identity config to accounts.json after successful auth and registry addition | PASS | `internal/tools/add_account.go:225-232` -- AddAccountConfig called after registry.Add |
| FR-4  | remove_account tool MUST remove account identity config from accounts.json | PASS | `internal/tools/remove_account.go:65-67` -- RemoveAccountConfig called after registry.Remove |
| FR-5  | On startup, server MUST read accounts.json and restore all previously added accounts | PASS | `cmd/outlook-local-mcp/main.go:118-127` -- RestoreAccounts called with cfg.AccountsPath, registry, DefaultGraphClientFactory |
| FR-6  | Restored accounts MUST attempt silent token acquisition; failures MUST still be registered | PASS | `internal/auth/restore.go:133-164` -- GetToken called with silentAuthTimeout context; entry registered regardless of tokenErr; client set to nil on failure |
| FR-7  | SetupCredentialForAccount MUST accept default values as parameters instead of hardcoded fallbacks | PASS | `internal/auth/auth.go:365-383` -- function takes clientID, tenantID, authMethod as parameters and passes them directly to SetupCredential via config struct; no hardcoded fallbacks; empty authMethod causes "unsupported auth method" error |
| FR-8  | OUTLOOK_MCP_ACCOUNTS_PATH env var MUST override default accounts file location | PASS | `internal/config/config.go:223-226` -- GetEnv reads OUTLOOK_MCP_ACCOUNTS_PATH; empty falls back to AuthRecordPath directory |
| FR-9  | Server MUST start normally when accounts.json does not exist | PASS | `internal/auth/accounts.go:52-56` -- LoadAccounts returns empty slice on os.IsNotExist; `internal/auth/restore.go:84-87` -- returns 0,0 when no accounts |
| FR-10 | accounts.json file MUST use JSON format consistent with existing auth record files | PASS | `internal/auth/accounts.go:60-66` -- standard json.Unmarshal/MarshalIndent used; AccountsFile wraps AccountConfig slice with json tags |

## Acceptance Criteria Verification

| AC # | Description | Status | Evidence |
|------|-------------|--------|----------|
| AC-1 | Per-account identity stored in registry | PASS | `internal/auth/registry.go:34-45` -- fields in struct; `internal/tools/add_account.go:207-217` -- populated on add; `internal/auth/registry_test.go:246-274` -- TestAccountEntry_IdentityFields verifies Get returns correct values |
| AC-2 | Account config persisted on add_account | PASS | `internal/tools/add_account.go:225-232` -- AddAccountConfig call; `internal/tools/add_account_test.go:856-905` -- TestHandleAddAccount_PersistsConfig verifies accounts.json content |
| AC-3 | Account config removed on remove_account | PASS | `internal/tools/remove_account.go:65-67` -- RemoveAccountConfig call; `internal/tools/remove_account_test.go:137-178` -- TestHandleRemoveAccount_CleansUpConfig verifies contoso removed, redeploy retained |
| AC-4 | Accounts restored on startup | PASS | `cmd/outlook-local-mcp/main.go:118-127` -- RestoreAccounts called; `internal/auth/restore.go:71-99` -- iterates accounts, calls restoreOne; `internal/auth/restore_test.go:26-79` -- TestRestoreAccounts_Success verifies both accounts registered |
| AC-5 | Startup tolerates missing accounts file | PASS | `internal/auth/accounts.go:54-56` -- returns empty slice on not-exist (no error); `internal/auth/restore.go:84-87` -- returns 0,0 immediately; `internal/auth/restore_test.go:130-143` -- TestRestoreAccounts_FileNotExist |
| AC-6 | Startup tolerates failed silent auth | PASS | `internal/auth/restore.go:152-164` -- account registered with nil client on tokenErr; `internal/auth/restore_test.go:83-126` -- TestRestoreAccounts_SilentAuthFailure verifies entry in registry with nil Client |
| AC-7 | Defaults chain uses server config | PASS | `internal/tools/add_account.go:169-171` -- request.GetString defaults to cfg.ClientID/TenantID/AuthMethod; `internal/auth/auth.go:365-383` -- no hardcoded fallbacks in SetupCredentialForAccount; `internal/auth/auth_test.go:522-530` -- TestSetupCredentialForAccount_NoHardcodedDefaults |
| AC-8 | Env var overrides accounts file path | PASS | `internal/config/config.go:223-226` -- reads OUTLOOK_MCP_ACCOUNTS_PATH; `internal/config/config_test.go:663-672` -- TestLoadConfig_AccountsPathEnvVar sets to /tmp/custom-accounts.json and verifies |

## Test Strategy Verification

### Tests to Add

| Test File | Test Name | Specified | Exists | Matches Spec |
|-----------|-----------|-----------|--------|--------------|
| `internal/auth/accounts_test.go` | `TestSaveAndLoadAccounts` | Yes | Yes | Yes -- round-trips 2 accounts with different identity configs |
| `internal/auth/accounts_test.go` | `TestLoadAccounts_FileNotExist` | Yes | Yes | Yes -- non-existent path returns empty slice, no error |
| `internal/auth/accounts_test.go` | `TestAddAccountConfig` | Yes | Yes | Yes -- starts with 1 account, appends, verifies 2 accounts |
| `internal/auth/accounts_test.go` | `TestRemoveAccountConfig` | Yes | Yes | Yes -- starts with 2, removes 1, verifies 1 remaining |
| `internal/auth/accounts_test.go` | `TestRemoveAccountConfig_NotFound` | Yes | Yes | Yes -- non-existent label returns no error, file unchanged |
| `internal/auth/registry_test.go` | `TestAccountEntry_IdentityFields` | Yes | Yes | Yes -- adds entry with ClientID/TenantID/AuthMethod, verifies via Get |
| `internal/tools/add_account_test.go` | `TestHandleAddAccount_PersistsConfig` | Yes | Yes | Yes -- calls add_account, verifies accounts.json contains entry with label/clientID/tenantID/authMethod |
| `internal/tools/remove_account_test.go` | `TestHandleRemoveAccount_CleansUpConfig` | Yes | Yes | Yes -- pre-populates 2 accounts, removes 1, verifies 1 remains |
| `internal/auth/restore_test.go` | `TestRestoreAccounts_Success` | Yes | Yes | Yes -- 2 entries in accounts.json, both registered in registry (note: silent auth fails since no real cached tokens, but accounts are registered) |
| `internal/auth/restore_test.go` | `TestRestoreAccounts_SilentAuthFailure` | Yes | Yes | Yes -- 1 entry, no cached tokens, account registered with nil Client |
| `internal/config/config_test.go` | `TestLoadConfig_AccountsPathEnvVar` | Yes | Yes | Yes -- sets OUTLOOK_MCP_ACCOUNTS_PATH=/tmp/custom-accounts.json, verifies Config.AccountsPath |

### Tests to Modify

| Test File | Test Name | Specified | Exists | Matches Spec |
|-----------|-----------|-----------|--------|--------------|
| `internal/tools/add_account_test.go` | `TestHandleAddAccount_Success` | Yes | Yes | Yes -- lines 147-155 now assert ClientID, TenantID, AuthMethod match config |
| `internal/auth/auth_test.go` | `TestSetupCredentialForAccount_NoHardcodedDefaults` (was `_Defaults`) | Yes | Yes | Yes -- verifies error when empty strings passed (no hardcoded fallbacks); name changed to `_NoHardcodedDefaults` to reflect new behavior |

## Non-Functional Requirements Verification

| NFR # | Description | Status | Evidence |
|-------|-------------|--------|----------|
| NFR-1 | Atomic file writes | PASS | `internal/auth/accounts.go:93-113` -- writes to temp file, renames to target; cleanup on failure |
| NFR-2 | No secrets in accounts.json | PASS | `internal/auth/accounts.go:19-34` -- AccountConfig only stores label, client_id, tenant_id, auth_method |
| NFR-3 | Silent auth bounded timeout | PASS | `internal/auth/restore.go:26-27` -- silentAuthTimeout = 5 * time.Second; line 134 -- context.WithTimeout |

## Gaps

None.
