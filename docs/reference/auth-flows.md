# Authentication Flows

Reference documentation for device code flow, OAuth scopes, token caching, and the Graph client initialization sequence. For a user-facing overview of authentication concepts, see [docs/concepts.md](../concepts.md).

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
