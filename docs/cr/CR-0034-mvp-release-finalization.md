---
id: "CR-0034"
status: "completed"
completed-date: 2026-03-16
date: 2026-03-16
requestor: Daniel Grenemark
stakeholders:
  - Daniel Grenemark
priority: "high"
target-version: "1.0.0"
source-branch: dev/cc-swarm
source-commit: 5539a17
---

# MVP Release Finalization: Default Auth, User Configuration, and Trust UX

## Change Summary

Finalize the MCPB extension for MVP release by making the Claude Desktop configuration interface user-friendly, secure-by-default, and self-documenting. This CR changes the default auth method to `device_code`, exposes auth method and timezone as configurable fields in the Claude Desktop UI via the MCPB manifest `user_config`, implements smart auth method defaulting based on client ID, and adds trust-building descriptions that highlight local execution and OS keychain usage.

## Motivation and Background

The current MCPB extension exposes only `client_id` and `tenant_id` in the Claude Desktop configuration UI. Users cannot configure the authentication method or timezone without manually editing environment variables. The default auth method is `auth_code`, which requires a multi-step manual flow (copy URL, paste redirect URL) that is confusing for first-time users. The `device_code` flow is simpler: the user visits a URL, enters a code, and authenticates — a flow familiar to anyone who has signed into a streaming service on a TV.

Additionally, the configuration descriptions don't communicate the security properties that matter to users: the MCP server runs entirely on their local machine, tokens are stored in the OS keychain (macOS Keychain / Windows Credential Manager), and no data leaves their device except direct Microsoft Graph API calls.

## Change Drivers

* **First-run friction**: `auth_code` requires users to manually copy/paste a redirect URL, while `device_code` is a familiar one-step flow.
* **Missing UI configuration**: Auth method and timezone cannot be configured from the Claude Desktop extensions UI — users must know about environment variables.
* **No trust signals**: The configuration UI doesn't communicate that the MCP runs locally, uses OS keychain, or keeps data on-device.
* **Smart defaults**: When a user provides their own app registration (custom client ID), `browser` auth is the better default since their app likely has `http://localhost` redirect URIs configured.
* **Timezone errors**: Invalid timezone values produce a cryptic validation error. Users need either a clear description of expected format or auto-detection.

## Current State

### Default Auth Method

`LoadConfig` in `internal/config/config.go` defaults `AuthMethod` to `"auth_code"` (line 221). This requires the user to:
1. Run the MCP server
2. Copy the authorization URL from the `complete_auth` tool prompt
3. Open it in a browser, authenticate, and consent
4. Copy the redirect URL from the browser address bar
5. Paste it back into the `complete_auth` tool

### MCPB Manifest user_config

The manifest `extension/manifest.json` exposes only two fields:
- `client_id` (string, optional) — "Leave empty to use the default Microsoft Office first-party client ID."
- `tenant_id` (string, optional) — Azure AD tenant identifier.

Auth method, timezone, and other operational settings are not exposed.

### Client ID Default

`LoadConfig` defaults `OUTLOOK_MCP_CLIENT_ID` to `"outlook-local-mcp"` which resolves to `dd5fc5c5-eb9a-4f6f-97bd-1a9fecb277d3` (the project's own multi-tenant app registration). This requires admin consent in most tenants. The `outlook-desktop` app (`d3590ed6-52b3-4102-aeff-aad2292ab01c`) is pre-authorized and works out of the box.

### Auth Method Inference

`internal/auth/account_resolver.go` contains `inferAuthMethod` which type-asserts the credential to determine the auth method at runtime. There is no logic to infer the auth method based on the client ID at configuration time.

### Timezone Configuration

`LoadConfig` defaults `DefaultTimezone` to `"UTC"`. Invalid values are caught by `ValidateConfig` which returns a generic error: "DefaultTimezone must be a valid IANA timezone".

## Proposed Change

### 1. Change Default Auth Method to `device_code`

Change the `LoadConfig` default for `AuthMethod` from `"auth_code"` to `"device_code"`.

The `device_code` flow is the simplest for users:
1. The MCP server displays a URL and a code
2. The user visits the URL in any browser and enters the code
3. Authentication completes automatically

### 2. Smart Auth Method Defaulting

When a user provides a custom `client_id` (not a well-known name or the default) and leaves `auth_method` blank:
- Default to `"browser"` — custom app registrations typically have `http://localhost` redirect URIs, making browser auth the natural choice.

When using the default client ID or a well-known name:
- Default to `"device_code"` — the well-known apps and the project default support device code flow without additional redirect URI configuration.

Implementation: add `InferAuthMethod(clientID, explicitAuthMethod string) string` to `internal/config/config.go` that applies this logic after `LoadConfig` reads both values.

### 3. Expose Auth Method in MCPB Manifest

Add `auth_method` to `user_config` in the manifest:

```json
"auth_method": {
  "type": "string",
  "title": "Authentication Method",
  "description": "How you sign in to Microsoft. 'device_code' (default): visit a URL and enter a code — works everywhere, no app registration needed. 'browser': opens your browser automatically — requires your own app registration with http://localhost redirect URI. 'auth_code': manual flow for headless/remote environments.",
  "required": false,
  "default": "device_code"
}
```

Note: MCPB `user_config` does not support `enum`/dropdown field types (only `string`, `number`, `boolean`, `directory`, `file`). The description **MUST** clearly list all valid values and when to use each one.

### 4. Expose Timezone in MCPB Manifest

Add `timezone` to `user_config` in the manifest:

```json
"timezone": {
  "type": "string",
  "title": "Timezone",
  "description": "Your timezone for calendar operations (IANA format). Use 'auto' to detect from your system, or specify explicitly, e.g. 'America/New_York', 'Europe/London', 'Asia/Tokyo'. Defaults to 'auto'.",
  "required": false,
  "default": "auto"
}
```

Change the `LoadConfig` default for `DefaultTimezone` from `"UTC"` to `"auto"`.

Implement timezone auto-detection: when the value is `"auto"`, resolve it to the system's local timezone using Go's `time.Now().Location().String()`. If the resolved value is `"Local"` (Go's fallback when TZ is unset), fall back to `"UTC"` and log a warning.

Improve the validation error for invalid timezone values to include an example: `"DefaultTimezone 'X' is not a valid IANA timezone. Examples: America/New_York, Europe/London, Asia/Tokyo, UTC"`.

### 5. Update Client ID Description and Default Name

Change the default `OUTLOOK_MCP_CLIENT_ID` from `"outlook-local-mcp"` to `"outlook-desktop"` in both `LoadConfig` and the MCPB manifest. The `outlook-desktop` app (`d3590ed6-52b3-4102-aeff-aad2292ab01c`) is the Microsoft Office first-party client ID, which is pre-authorized for Graph Calendar scopes in all tenants — no admin consent required.

Update the `client_id` user_config:

```json
"client_id": {
  "type": "string",
  "title": "Microsoft Application (Client) ID",
  "description": "The app used to access Microsoft Graph. Default: 'outlook-desktop' (Microsoft Office, works out of the box with device_code auth). Use 'outlook-local-mcp' for the project's own app registration. To use browser auth, register your own app in Azure AD with http://localhost redirect URI and paste the client ID here.",
  "required": false,
  "default": "outlook-desktop"
}
```

### 6. Trust Signal Descriptions

Update the manifest `description` and individual field descriptions to communicate security properties:

**Extension description:**
```
"description": "Manage Microsoft Outlook calendars and events directly from Claude. Runs entirely on your local machine — calendar data stays on your device. Tokens are stored securely in your OS keychain (macOS Keychain / Windows Credential Manager). Only connects to Microsoft Graph API with your explicit consent."
```

**Tenant ID description** — add trust context:
```json
"tenant_id": {
  "type": "string",
  "title": "Azure AD Tenant ID",
  "description": "Your Azure AD tenant. Use 'common' (default) for any Microsoft account, 'organizations' for work/school only, or 'consumers' for personal accounts only. Advanced: paste a specific tenant GUID to restrict access to a single organization.",
  "required": false,
  "default": "common"
}
```

### 7. Wire MCPB user_config to Environment Variables

The MCPB manifest `user_config` fields are injected as environment variables by Claude Desktop using the pattern `OUTLOOK_MCP_<FIELD_NAME>` (uppercased). The `server.mcp_config.env` section **MUST** map each `user_config` key to its corresponding environment variable:

```json
"env": {
  "OUTLOOK_MCP_CLIENT_ID": "${user_config.client_id}",
  "OUTLOOK_MCP_TENANT_ID": "${user_config.tenant_id}",
  "OUTLOOK_MCP_AUTH_METHOD": "${user_config.auth_method}",
  "OUTLOOK_MCP_DEFAULT_TIMEZONE": "${user_config.timezone}"
}
```

## Requirements

### Functional Requirements

1. `LoadConfig` **MUST** default `AuthMethod` to `"device_code"` when `OUTLOOK_MCP_AUTH_METHOD` is unset.
2. When `OUTLOOK_MCP_CLIENT_ID` is set to a non-default, non-well-known value AND `OUTLOOK_MCP_AUTH_METHOD` is unset, `AuthMethod` **MUST** default to `"browser"`.
3. When `OUTLOOK_MCP_CLIENT_ID` is unset, set to a well-known name, or set to the default, AND `OUTLOOK_MCP_AUTH_METHOD` is unset, `AuthMethod` **MUST** default to `"device_code"`.
4. When `OUTLOOK_MCP_AUTH_METHOD` is explicitly set, it **MUST** be used regardless of the client ID value.
5. The MCPB manifest `user_config` **MUST** include `auth_method` (string, optional, default `"device_code"`).
6. The MCPB manifest `user_config` **MUST** include `timezone` (string, optional, default `"auto"`).
7. When `DefaultTimezone` is `"auto"`, `LoadConfig` **MUST** resolve it to the system's local IANA timezone.
8. When `DefaultTimezone` is `"auto"` and the system timezone resolves to `"Local"`, it **MUST** fall back to `"UTC"` with a warning.
9. Timezone validation errors **MUST** include example valid values in the error message.
10. The MCPB manifest `description` **MUST** mention local execution, OS keychain token storage, and explicit consent.
11. The `client_id` user_config description **MUST** mention `outlook-desktop` and `outlook-local-mcp` by name and explain when to use a custom app registration.
12. The `auth_method` user_config description **MUST** list all three valid values with a brief explanation of each.
13. The MCPB manifest `server.mcp_config.env` **MUST** map all `user_config` keys to their corresponding `OUTLOOK_MCP_*` environment variables.
14. The default `client_id` in both `LoadConfig` and the MCPB manifest **MUST** be `"outlook-desktop"`.
15. The `LoadConfig` default for `OUTLOOK_MCP_CLIENT_ID` **MUST** change from `"outlook-local-mcp"` to `"outlook-desktop"`.

### Non-Functional Requirements

1. Timezone auto-detection **MUST NOT** make network calls — it **MUST** use only local system APIs.
2. The `InferAuthMethod` function **MUST** be a pure function with no side effects beyond logging.
3. All user-facing text in the manifest **MUST** be written in plain language without technical jargon where possible.

## Affected Components

* `internal/config/config.go` — `LoadConfig` default changes, `InferAuthMethod` function, timezone auto-detection
* `internal/config/validate.go` — improved timezone error message
* `internal/config/config_test.go` — updated defaults and new inference tests
* `internal/config/validate_test.go` — updated timezone error message tests
* `extension/manifest.json` — new user_config fields, updated descriptions, env mapping

## Scope Boundaries

### In Scope

* Default client ID change from `outlook-local-mcp` to `outlook-desktop`
* Default auth method change from `auth_code` to `device_code`
* Smart auth method inference based on client ID
* New `user_config` fields: `auth_method`, `timezone`
* Timezone auto-detection with `"auto"` value
* Improved timezone validation error messages
* Trust-building descriptions in the manifest
* Environment variable mapping in `server.mcp_config.env`
* Updated `client_id` and `tenant_id` descriptions

### Out of Scope ("Here, But Not Further")

* MCPB manifest dropdown/enum support — not available in the MCPB v0.2 schema
* Changes to the actual auth flows (device_code, browser, auth_code implementations)
* Adding new well-known client IDs
* Changes to the `complete_auth` tool registration logic
* Multi-account configuration via the MCPB UI
* Linux platform support in MCPB manifest

## Implementation Approach

### Phase 1: Config Changes

1. Change `OUTLOOK_MCP_CLIENT_ID` default from `"outlook-local-mcp"` to `"outlook-desktop"` in `LoadConfig`.
2. Change `AuthMethod` default from `"auth_code"` to `"device_code"` in `LoadConfig`.
3. Add `InferAuthMethod(clientID, explicitAuthMethod string) string` to `internal/config/config.go`.
4. Call `InferAuthMethod` in `LoadConfig` after reading both `ClientID` and `AuthMethod`.
5. Add timezone auto-detection: when `DefaultTimezone` is `"auto"`, resolve via `time.Now().Location().String()`.
6. Update timezone validation error message to include examples.
7. Update tests.

### Phase 2: Manifest Updates

1. Add `auth_method` and `timezone` to `user_config` in `extension/manifest.json`.
2. Update `client_id` and `tenant_id` descriptions with trust signals and guidance.
3. Update the extension `description` with trust signals.
4. Add environment variable mapping in `server.mcp_config.env`.
5. Validate manifest with `mcpb validate`.

## Test Strategy

### Tests to Add

| Test File | Test Name | Description | Inputs | Expected Output |
|-----------|-----------|-------------|--------|-----------------|
| `internal/config/config_test.go` | `TestInferAuthMethod_DefaultDeviceCode` | Default client ID with no explicit auth method | clientID=default, authMethod="" | `"device_code"` |
| `internal/config/config_test.go` | `TestInferAuthMethod_CustomClientBrowser` | Custom UUID client ID with no explicit auth method | clientID=custom-uuid, authMethod="" | `"browser"` |
| `internal/config/config_test.go` | `TestInferAuthMethod_WellKnownDeviceCode` | Well-known name client ID with no explicit auth method | clientID=outlook-desktop (resolved), authMethod="" | `"device_code"` |
| `internal/config/config_test.go` | `TestInferAuthMethod_ExplicitOverride` | Explicit auth method overrides inference | clientID=custom-uuid, authMethod="device_code" | `"device_code"` |
| `internal/config/config_test.go` | `TestLoadConfig_DefaultClientIDOutlookDesktop` | LoadConfig defaults client ID to outlook-desktop | No env vars set | cfg.ClientID == outlook-desktop UUID |
| `internal/config/config_test.go` | `TestLoadConfig_DefaultAuthMethodDeviceCode` | LoadConfig defaults to device_code | No env vars set | cfg.AuthMethod == "device_code" |
| `internal/config/config_test.go` | `TestLoadConfig_TimezoneAuto` | Auto timezone resolves to system timezone | OUTLOOK_MCP_DEFAULT_TIMEZONE=auto | cfg.DefaultTimezone is valid IANA name |
| `internal/config/config_test.go` | `TestLoadConfig_TimezoneExplicit` | Explicit timezone passes through | OUTLOOK_MCP_DEFAULT_TIMEZONE=Europe/London | cfg.DefaultTimezone == "Europe/London" |
| `internal/config/config_test.go` | `TestLoadConfig_TimezoneAutoFallback` | Auto timezone falls back to UTC when system returns "Local" | OUTLOOK_MCP_DEFAULT_TIMEZONE=auto, TZ="" (unset) | cfg.DefaultTimezone is valid IANA (UTC or system) |
| `internal/config/validate_test.go` | `TestValidateConfig_BadTimezoneMessage` | Invalid timezone error includes examples | DefaultTimezone="NotATimezone" | Error contains "Examples:" |

### Tests to Modify

| Test File | Test Name | Current Behavior | New Behavior | Reason for Change |
|-----------|-----------|------------------|--------------|-------------------|
| `internal/config/config_test.go` | `TestLoadConfigDefaults` | Asserts ClientID default resolves from `"outlook-local-mcp"` | Asserts ClientID default resolves from `"outlook-desktop"` | Default changed |
| `internal/config/config_test.go` | `TestLoadConfigDefaults` | Asserts AuthMethod default is `"auth_code"` | Asserts AuthMethod default is `"device_code"` | Default changed |
| `internal/config/config_test.go` | `TestLoadConfigDefaults` | Asserts DefaultTimezone is `"UTC"` | Asserts DefaultTimezone is resolved from `"auto"` | Default changed |
| `internal/config/clientids_test.go` | `TestResolveClientID_Default` | Asserts `"outlook-local-mcp"` resolves | Asserts `"outlook-desktop"` resolves (test still valid, default env value changes) | Default changed |

## Acceptance Criteria

### AC-1: Default auth method is device_code

```gherkin
Given OUTLOOK_MCP_AUTH_METHOD is not set
  And OUTLOOK_MCP_CLIENT_ID is not set
When LoadConfig returns
Then cfg.AuthMethod is "device_code"
```

### AC-2: Custom client ID defaults to browser auth

```gherkin
Given OUTLOOK_MCP_CLIENT_ID is set to a UUID not in the well-known registry
  And OUTLOOK_MCP_AUTH_METHOD is not set
When LoadConfig returns
Then cfg.AuthMethod is "browser"
```

### AC-3: Explicit auth method overrides inference

```gherkin
Given OUTLOOK_MCP_CLIENT_ID is set to a custom UUID
  And OUTLOOK_MCP_AUTH_METHOD is set to "device_code"
When LoadConfig returns
Then cfg.AuthMethod is "device_code"
```

### AC-4: Timezone auto-detection

```gherkin
Given OUTLOOK_MCP_DEFAULT_TIMEZONE is set to "auto"
When LoadConfig returns
Then cfg.DefaultTimezone is a valid IANA timezone name
  And it matches the system's local timezone
```

### AC-5: Timezone auto fallback

```gherkin
Given OUTLOOK_MCP_DEFAULT_TIMEZONE is set to "auto"
  And the system timezone resolves to "Local"
When LoadConfig returns
Then cfg.DefaultTimezone is "UTC"
  And a warning is logged
```

### AC-6: Improved timezone error

```gherkin
Given OUTLOOK_MCP_DEFAULT_TIMEZONE is set to "NotATimezone"
When ValidateConfig runs
Then the error message contains "Examples: America/New_York, Europe/London, Asia/Tokyo, UTC"
```

### AC-7: MCPB manifest validates

```gherkin
Given the updated extension/manifest.json
When mcpb validate extension/manifest.json is run
Then validation passes
```

### AC-8: Trust signals in description

```gherkin
Given the updated extension/manifest.json
Then the description mentions "local machine", "keychain", and "consent"
```

### AC-9: Environment variable mapping

```gherkin
Given the updated extension/manifest.json
Then server.mcp_config.env maps client_id, tenant_id, auth_method, timezone to OUTLOOK_MCP_* variables
```

### AC-10: Well-known client ID defaults to device_code

```gherkin
Given OUTLOOK_MCP_CLIENT_ID is set to "outlook-desktop"
  And OUTLOOK_MCP_AUTH_METHOD is not set
When LoadConfig returns
Then cfg.AuthMethod is "device_code"
```

### AC-11: Default client ID is outlook-desktop

```gherkin
Given OUTLOOK_MCP_CLIENT_ID is not set
When LoadConfig returns
Then cfg.ClientID is "d3590ed6-52b3-4102-aeff-aad2292ab01c"
```

## Quality Standards Compliance

### Build & Compilation

- [x] Code compiles/builds without errors
- [x] No new compiler warnings introduced

### Linting & Code Style

- [x] All linter checks pass with zero warnings/errors
- [x] Code follows project coding conventions and style guides
- [ ] Any linter exceptions are documented with justification

### Test Execution

- [x] All existing tests pass after implementation
- [x] All new tests pass
- [x] Test coverage meets project requirements for changed code

### Documentation

- [x] Inline code documentation updated where applicable
- [ ] API documentation updated for any API changes
- [ ] User-facing documentation updated if behavior changes

### Code Review

- [ ] Changes submitted via pull request
- [ ] PR title follows Conventional Commits format
- [ ] Code review completed and approved
- [ ] Changes squash-merged to maintain linear history

### Verification Commands

```bash
# Build verification
go build ./cmd/outlook-local-mcp/

# Lint verification
golangci-lint run

# Test execution
go test ./...

# Full CI check
make ci

# Manifest validation
mcpb validate extension/manifest.json
```

## Risks and Mitigation

### Risk 1: Timezone auto-detection returns unexpected values on CI/containers

**Likelihood:** medium
**Impact:** low
**Mitigation:** Tests that verify auto-detection should accept any valid IANA timezone, not a specific value. The `"Local"` → `"UTC"` fallback handles the common CI case where TZ is unset.

### Risk 2: Breaking existing users who rely on auth_code default

**Likelihood:** low
**Impact:** medium
**Mitigation:** This is a pre-release change (no users have the released MCPB yet). Users who have configured `OUTLOOK_MCP_AUTH_METHOD=auth_code` explicitly will be unaffected since explicit values override inference.

### Risk 3: MCPB user_config env mapping format changes

**Likelihood:** low
**Impact:** high
**Mitigation:** The `${user_config.key}` variable substitution syntax is documented in the MCPB v0.2 spec. Validate with `mcpb validate` and test with a local Claude Desktop import.

## Dependencies

* MCPB manifest v0.2 spec (`@anthropic-ai/mcpb` 2.1)
* No external library additions required — timezone detection uses Go stdlib.

## Decision Outcome

Chosen approach: change defaults to device_code + auto timezone, add smart inference, and enrich the MCPB manifest with user-facing configuration and trust signals. This creates a zero-configuration first-run experience (install extension → authenticate via device code → calendars work in local timezone) while supporting advanced scenarios (custom app registration with browser auth) through self-documenting configuration fields.

## Related Items

* CR-0030: Manual Auth Code Flow — introduced `auth_code` as the default
* CR-0033: Well-Known Client IDs — introduced friendly name resolution

<!-- CR Review Summary (2026-03-16)

Findings: 4
Fixes applied: 4
Unresolvable items: 0

1. COVERAGE GAP (AC→Test): AC-5 (timezone auto fallback to UTC when "Local") had no
   corresponding test entry. Added TestLoadConfig_TimezoneAutoFallback to the Tests to Add table.

2. COVERAGE GAP (FR→AC): FR-3 (well-known name defaults to device_code) was only
   partially covered by AC-1 (which tests unset client ID, not explicit well-known names).
   FR-14/FR-15 (default client_id is outlook-desktop) had no dedicated AC.
   Added AC-10 (well-known client ID defaults to device_code) and AC-11 (default client
   ID is outlook-desktop) to close both gaps.

3. INCONSISTENCY (Risk 3): Risk 3 mitigation referenced "${user_config:key}" (colon syntax)
   but the Implementation Approach (Section 7) uses "${user_config.key}" (dot syntax).
   Fixed Risk 3 mitigation to use dot syntax, consistent with the env mapping JSON.

4. CURRENT STATE VERIFIED: All line references and descriptions in the Current State
   section match the source code (config.go:152,156,221; validate.go:78,81;
   manifest.json:82-94; account_resolver.go:226-231).

-->

