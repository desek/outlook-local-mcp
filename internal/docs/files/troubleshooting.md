# Troubleshooting

This guide covers common failure modes for `outlook-local-mcp` and how to resolve them.

> **Note:** This file is a placeholder authored for Phase 1 of CR-0061. Full content will be written in Phase 3.

## Authentication Failures

### Login

Run `account login` (via the `account` tool with `operation="login"`) to start a new sign-in flow. The server supports browser-based, device-code, and auth-code fallback methods (see CR-0022, CR-0024, CR-0030, CR-0031).

### Token Refresh

If the server reports an expired or invalid token, call `account login` for the affected account to obtain a fresh token. The token cache is stored securely in the system Keychain.

### Keychain Locked or Unavailable

On macOS, the token cache relies on the system Keychain. If the Keychain is locked or unavailable (e.g., in a headless CI environment), the server falls back to a file-based cache. Unlock the Keychain or set `OUTLOOK_MCP_KEYCHAIN_FALLBACK=file` in the server environment.

## Multi-Account Resolution

When multiple accounts are registered, tool calls that omit `account` resolve to the default account. To target a specific account, pass the UPN (e.g., `user@example.com`) as the `account` parameter. Use `account list` to see registered accounts (see CR-0056).

## Graph API Throttling (429)

Microsoft Graph enforces per-tenant rate limits. When the server receives a `429 Too Many Requests` response it backs off automatically using the `Retry-After` header. If throttling persists, reduce call frequency or contact your tenant admin (see CR-0010).

## InefficientFilter Errors

Graph rejects certain filter expressions that lack a supported `$orderby` constraint. The server rewrites common patterns automatically. If you encounter an `InefficientFilter` error from a custom query, add an `$orderby=receivedDateTime desc` clause or simplify the filter predicate.

## Mail Tool Enablement Flags

- `MailEnabled=false` — the `mail` aggregate tool is hidden entirely. Set `OUTLOOK_MCP_MAIL_ENABLED=true` to re-enable.
- `MailManageEnabled=false` — write verbs (send, delete, move) are hidden; read verbs remain available. Set `OUTLOOK_MCP_MAIL_MANAGE_ENABLED=true` to expose manage verbs (see CR-0058).

## Read-Only Mode

When `ReadOnly=true` is configured, all write verbs across every domain tool are disabled. The `system.status` verb reports the current read-only state (see CR-0020).

## Log File Location

The server writes structured logs to `outlook-local-mcp.log` in the working directory. PII is redacted in log output. To change the log path, set `OUTLOOK_MCP_LOG_FILE` (see CR-0002, CR-0023).

## Audit Log

Audit events are written alongside the main log. Each event includes the tool name, verb, account UPN, and outcome. Use these entries to diagnose which operations were attempted and whether they succeeded (see CR-0015).

## Account Lifecycle

Use the `account` domain tool verbs to manage accounts:

- `account login` — authenticate a new or existing account.
- `account list` — list all registered accounts.
- `account remove` — deregister an account and purge its cached tokens.
- `account refresh` — force a token refresh for a specific account.

See CR-0056 for the full account lifecycle specification.
