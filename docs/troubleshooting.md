# Troubleshooting Guide

Common failure modes and remediation steps for `outlook-local-mcp`.

---

## Authentication failures

**Symptom:** A tool call returns an error like `authentication required` or `failed to acquire token`.

**Cause:** No token has been cached for the account, or the cached token has expired.

**Remediation:**

1. Call `{tool: "account", args: {operation: "list"}}` to see which accounts are registered and which are disconnected.
2. If the account shows `disconnected`, call `{tool: "account", args: {operation: "login", label: "<label>"}}` to re-authenticate using the account's persisted method.
3. If no account is registered at all, call `{tool: "account", args: {operation: "add", label: "default"}}` to register and authenticate the first account.
4. After successful login, retry the original tool call. The `AuthMiddleware` will automatically retry once authentication completes.

---

## Token refresh

**Symptom:** A tool call returns an error referencing `token expired`, `refresh token`, or `invalid_grant`.

**Cause:** The OAuth refresh token expired (typically after 90 days of inactivity or after an Entra ID policy change) or the token cache was corrupted.

**Remediation:**

1. Call `{tool: "account", args: {operation: "refresh", label: "<label>"}}` to force a silent token refresh. This succeeds when the refresh token is still valid.
2. If the refresh fails, call `{tool: "account", args: {operation: "login", label: "<label>"}}` to initiate a new interactive authentication flow.
3. After re-authenticating, the server persists a new token and automatically retries the pending tool call.

**Prevention:** The server performs a silent token probe at startup. `device_code` accounts skip the probe to avoid crash loops; other methods probe silently and pre-cache tokens before the first tool call.

---

## Device code flow

**Symptom:** Authentication returns a device code URL and code in the tool result text instead of completing automatically.

**Cause:** The MCP client does not support the Elicitation API (e.g., Claude Code). The server falls back to returning the device code directly in the tool result.

**Remediation:**

1. Copy the URL and code from the tool result.
2. Open the URL in a browser, enter the code, and complete the Microsoft sign-in.
3. After sign-in, call any tool again. The server picks up the cached token automatically.

---

## Browser auth flow

**Symptom:** A tool call returns an error like `browser authentication timed out` or the browser window did not appear.

**Cause:** The server opened the system browser for OAuth login but the user did not complete sign-in within the timeout, or the browser did not open (headless environment).

**Remediation:**

1. Retry the tool call to trigger a fresh browser auth attempt.
2. If the browser does not open automatically, use `auth_code` method instead: set `OUTLOOK_MCP_AUTH_METHOD=auth_code` and restart the server.
3. For headless environments (containers, SSH), use `device_code` authentication: set `OUTLOOK_MCP_AUTH_METHOD=device_code`.

---

## Auth code flow

**Symptom:** The server returns an auth URL and asks you to paste the redirect URL back.

**Cause:** The `auth_code` method was selected. After signing in, the browser redirects to the `nativeclient` URI and shows the full redirect URL in the address bar.

**Remediation:**

1. Copy the full redirect URL from the browser address bar after sign-in.
2. If the MCP client supports Elicitation, paste it into the prompt.
3. If not, call `{tool: "system", args: {operation: "complete_auth", redirect_url: "<url>"}}` to exchange the code for a token.

---

## Keychain locked

**Symptom:** Token storage fails with an error referencing `keychain`, `SecKeychain`, `libsecret`, or `DPAPI`.

**Cause:** The OS keychain is locked, unavailable, or the process lacks permission to access it (common in headless, container, or screen-locked environments).

**Remediation:**

1. Unlock the keychain (macOS: open **Keychain Access** and unlock the login keychain; Linux: ensure `gnome-keyring` or `kwallet` is running).
2. If the keychain cannot be unlocked, set `OUTLOOK_MCP_TOKEN_STORAGE=file` in the server configuration to switch to the file-based AES-256-GCM encrypted token cache at `~/.outlook-local-mcp/token_cache.bin`.
3. Restart the server after changing `OUTLOOK_MCP_TOKEN_STORAGE`.
4. Container and Docker builds (`CGO_ENABLED=0`) always use the file-based cache regardless of `TOKEN_STORAGE` setting; no action required.

**Note:** When `TOKEN_STORAGE=auto` (the default) and the keychain is unavailable, the server automatically falls back to the file cache. Only `TOKEN_STORAGE=keychain` (no fallback) will error in this scenario.

---

## Multi-account resolution

**Symptom:** A tool call errors with `multiple authenticated accounts` or the wrong account is used.

**Cause:** Multiple accounts are authenticated and no `account` parameter was supplied.

**Remediation:**

1. Call `{tool: "account", args: {operation: "list"}}` to see all registered accounts with their labels and UPNs.
2. Pass the `account` parameter to any tool: `{tool: "calendar", args: {operation: "list_events", account: "work"}}`. Both label and UPN (e.g., `alice@contoso.com`) are accepted.
3. If the MCP client supports Elicitation, a prompt appears for account selection when no `account` param is supplied; select the desired account from the list.

**Account states:**

- `authenticated`: Token is cached and valid.
- `disconnected`: Account is registered but the token expired or was cleared. Call `account.login` to reconnect.

---

## Graph 429 throttling

**Symptom:** A tool call returns an error like `Graph API error [TooManyRequests]: 429` or `ApplicationThrottled`.

**Cause:** The Microsoft Graph API is rate-limiting requests from this application or tenant.

**Remediation:**

1. Wait a few seconds and retry the tool call. The server applies automatic exponential backoff for 429 responses (configured via `OUTLOOK_MCP_MAX_RETRIES` and `OUTLOOK_MCP_RETRY_BACKOFF_MS`).
2. If throttling persists, increase `OUTLOOK_MCP_RETRY_BACKOFF_MS` (default 1000ms) to give Graph more recovery time between retries.
3. Avoid tight loops of rapid tool calls, especially for calendar listing operations across large date ranges.

**Configuration:** `OUTLOOK_MCP_MAX_RETRIES` (default 3) and `OUTLOOK_MCP_RETRY_BACKOFF_MS` (default 1000) control the retry behaviour. Check the current values with `{tool: "system", args: {operation: "status", output: "summary"}}` under `config.graph_api`.

---

## Inefficient filter

**Symptom:** A tool call returns an error like `Graph API error [ErrorInvalidRequest]` or a message referencing `InefficientFilter` or missing `$orderby` constraints.

**Cause:** The Microsoft Graph `$filter` query contains a condition on a non-indexed field without the required `$orderby` clause, or uses a filter expression that Graph does not support server-side.

**Remediation:**

1. Avoid filtering on non-indexed properties. Graph indexes `subject`, `start/dateTime`, `end/dateTime`, `organizer/emailAddress/address`, and `isOrganizer` for calendar queries.
2. When using `$filter` on `start/dateTime` or `end/dateTime`, include `$orderby=start/dateTime` (or `end/dateTime`) in the same request.
3. For mail queries, use the `search` parameter (KQL syntax) instead of `$filter` for full-text search. `$filter` on mail supports `isRead`, `isDraft`, `hasAttachments`, `importance`, `flag/flagStatus`, and `receivedDateTime`.
4. If the error persists, use `output=raw` on the offending tool to see the full OData error detail, then adjust the filter expression.

---

## Mail disabled

**Symptom:** The `mail` tool returns `mail access is not enabled` or `unknown operation`.

**Cause:** The `OUTLOOK_MCP_MAIL_ENABLED` environment variable is not set (default is `false`). Mail tools are only registered when mail access is explicitly enabled.

**Remediation:**

1. Set `OUTLOOK_MCP_MAIL_ENABLED=true` in the server's environment configuration and restart the server.
2. Verify the setting with `{tool: "system", args: {operation: "status", output: "summary"}}` and check `config.features.mail_enabled`.
3. On first enable, a new OAuth consent for `Mail.Read` is required. The authentication flow triggers automatically on the next tool call.

---

## Mail management disabled

**Symptom:** Draft operations (`create_draft`, `create_reply_draft`, `create_forward_draft`, `update_draft`, `delete_draft`) return `mail management is not enabled` or `unknown operation`.

**Cause:** `OUTLOOK_MCP_MAIL_MANAGE_ENABLED` is not set. Draft management is a separate opt-in that implies `MAIL_ENABLED`.

**Remediation:**

1. Set `OUTLOOK_MCP_MAIL_MANAGE_ENABLED=true` (this automatically enables `MAIL_ENABLED` as well) and restart the server.
2. Verify with `{tool: "system", args: {operation: "status", output: "summary"}}` and check `config.features.mail_manage_enabled`.
3. On first enable, a new OAuth consent for `Mail.ReadWrite` is required (supersedes `Mail.Read`). The authentication flow triggers automatically on the next tool call.

**Note:** The server never requests `Mail.Send`. Drafts are created in the Outlook Drafts folder; the user sends them manually from Outlook.

---

## Read-only mode

**Symptom:** Write tool calls (create, update, delete, cancel, draft operations) return `server is in read-only mode`.

**Cause:** `OUTLOOK_MCP_READ_ONLY=true` is set. In read-only mode all write operations are blocked at the `ReadOnlyGuard` middleware before reaching the handler.

**Remediation:**

1. If read-only mode is intentional (e.g., a shared or supervised environment), do not change this setting. Only read verbs (`list_*`, `get_*`, `search_*`, `status`) are available.
2. To re-enable writes, remove or set `OUTLOOK_MCP_READ_ONLY=false` and restart the server.
3. Verify the current mode with `{tool: "system", args: {operation: "status"}}` and look for `read-only=on` in the Features line.

---

## Log file location

Logs are written to the path configured in `OUTLOOK_MCP_LOG_FILE`. By default, no file logging is active and logs are written only to stderr (which the MCP client typically captures but does not surface in the chat).

To enable file logging:

1. Set `OUTLOOK_MCP_LOG_FILE=/path/to/outlook-local-mcp.log` in the server environment.
2. Set `OUTLOOK_MCP_LOG_LEVEL=debug` for verbose output during troubleshooting.
3. Restart the server.

The log file path and current level can be read with `{tool: "system", args: {operation: "status", output: "summary"}}` under `config.logging.log_file` and `config.logging.log_level`.

**PII sanitization:** When `OUTLOOK_MCP_LOG_SANITIZE=true` (the default), email addresses and other identifiers are replaced with redacted placeholders in all log output. Set to `false` only in controlled debugging sessions.

**Audit log:** When `OUTLOOK_MCP_AUDIT_LOG_ENABLED=true`, a structured audit trail is written to `OUTLOOK_MCP_AUDIT_LOG_PATH` (defaults to a file alongside the main log). The audit log records every tool invocation with timestamp, operation name, account label, and outcome.

---

## Account lifecycle

The `account` domain tool manages the full lifecycle of Microsoft accounts registered with the server.

### Add an account

```
{tool: "account", args: {operation: "add", label: "work"}}
```

Registers and authenticates a new account. Optional parameters: `client_id`, `tenant_id`, `auth_method`. The UPN (`alice@contoso.com`) is resolved from Graph `/me` and persisted to `accounts.json` after successful authentication.

### List accounts

```
{tool: "account", args: {operation: "list"}}
```

Returns all registered accounts with label, UPN, authentication state, and auth method. Disconnected accounts (expired tokens) are shown as first-class entries, not hidden.

### Log in a disconnected account

```
{tool: "account", args: {operation: "login", label: "work"}}
```

Re-authenticates a disconnected account using its persisted `auth_method`, `client_id`, and `tenant_id`. Errors if the account is already connected.

### Log out an account

```
{tool: "account", args: {operation: "logout", label: "work"}}
```

Disconnects an account without removing its configuration from `accounts.json`. The account remains visible as `disconnected` and can be reconnected via `account.login`.

### Force token refresh

```
{tool: "account", args: {operation: "refresh", label: "work"}}
```

Forces a silent token refresh (`ForceRefresh=true`). Returns the new token expiry. Useful after an Entra ID permission change or when token staleness is suspected.

### Remove an account

```
{tool: "account", args: {operation: "remove", label: "work"}}
```

Permanently removes the account from the registry and clears its keychain cache. Use `account.logout` instead when you want to disconnect without deleting the configuration. The `default` account cannot be removed.

---

## In-server documentation access

The server embeds this guide and other user-facing documentation. The LLM can access it directly without leaving the session:

```
{tool: "system", args: {operation: "list_docs"}}
{tool: "system", args: {operation: "search_docs", query: "token refresh"}}
{tool: "system", args: {operation: "get_docs", slug: "troubleshooting", section: "token-refresh"}}
```

Each embedded document is also available as an MCP resource at `doc://outlook-local-mcp/{slug}` (e.g., `doc://outlook-local-mcp/troubleshooting`). LLM clients that support `resources/list` and `resources/read` can fetch documents natively.
