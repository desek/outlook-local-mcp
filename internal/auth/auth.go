package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/desek/outlook-local-mcp/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// calendarScope is the OAuth scope requested for Microsoft Graph calendar
// operations. The azidentity library automatically includes offline_access
// to obtain a refresh token.
const calendarScope = "Calendars.ReadWrite"

// mailScope is the OAuth scope requested for read-only Microsoft Graph mail
// operations. It is only included when MailEnabled is true in the config and
// MailManageEnabled is false.
const mailScope = "Mail.Read"

// mailReadWriteScope is the OAuth scope requested for draft-centric mail
// management. It is a superset of Mail.Read and permits creating, updating,
// and deleting messages (including drafts). It is requested in place of
// mailScope when MailManageEnabled is true. Mail.Send is intentionally never
// requested: sending remains a user-only action performed in Outlook.
const mailReadWriteScope = "Mail.ReadWrite"

// Scopes returns the OAuth scope slice based on the application configuration.
// The calendar scope is always included. Mail scopes are selected according to
// configuration:
//
//   - When cfg.MailManageEnabled is true, mailReadWriteScope ("Mail.ReadWrite")
//     is appended. This scope subsumes Mail.Read, so mailScope is not also
//     added.
//   - Otherwise, when cfg.MailEnabled is true, mailScope ("Mail.Read") is
//     appended.
//   - When both flags are false, no mail scope is requested.
//
// Scopes never includes "Mail.Send"; sending mail is deliberately outside the
// server's capability surface.
//
// Parameters:
//   - cfg: the application configuration providing MailEnabled and
//     MailManageEnabled.
//
// Returns the slice of OAuth scopes to request during authentication and
// Graph client initialization.
func Scopes(cfg config.Config) []string {
	scopes := []string{calendarScope}
	switch {
	case cfg.MailManageEnabled:
		scopes = append(scopes, mailReadWriteScope)
	case cfg.MailEnabled:
		scopes = append(scopes, mailScope)
	}
	return scopes
}

// Authenticator is the interface for performing explicit authentication and
// obtaining an AuthenticationRecord. Both *azidentity.DeviceCodeCredential
// and *azidentity.InteractiveBrowserCredential satisfy this interface
// implicitly via structural typing. The middleware uses this interface to
// trigger re-authentication when a cached token expires, without coupling
// to a specific credential type.
//
// This follows the Interface Segregation Principle: a single-method interface
// capturing only the Authenticate capability needed by the middleware, separate
// from the azcore.TokenCredential interface used by the Graph SDK.
type Authenticator interface {
	// Authenticate performs interactive authentication and returns an
	// AuthenticationRecord containing non-secret account metadata that
	// enables silent token acquisition on future runs.
	//
	// Parameters:
	//   - ctx: context for the authentication call. Should have a generous
	//     timeout since interactive authentication requires user action.
	//   - opts: token request options specifying scopes and CAE enablement.
	//
	// Returns the AuthenticationRecord on success, or an error if the
	// interactive authentication flow fails or times out.
	Authenticate(ctx context.Context, opts *policy.TokenRequestOptions) (azidentity.AuthenticationRecord, error)
}

// LoadAuthRecord reads and deserializes a JSON-encoded AuthenticationRecord
// from the file at path. The AuthenticationRecord contains only non-secret
// metadata (account ID, tenant, authority) and is safe to store as plain JSON.
//
// Parameters:
//   - path: absolute filesystem path to the authentication record JSON file.
//
// Returns a populated AuthenticationRecord when the file exists and contains
// valid JSON, or a zero-value AuthenticationRecord when the file is missing
// or contains invalid JSON. Errors are logged but not propagated, since a
// missing or corrupt record simply triggers re-authentication.
func LoadAuthRecord(path string) azidentity.AuthenticationRecord {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Info("no authentication record found, first-run authentication required",
				"path", path)
		} else {
			slog.Warn("failed to read authentication record",
				"path", path, "error", err)
		}
		return azidentity.AuthenticationRecord{}
	}

	// Verify file permissions are not overly permissive (best-effort on Unix).
	info, statErr := os.Stat(path)
	if statErr == nil && info.Mode().Perm()&0077 != 0 {
		slog.Warn("auth record file has overly permissive permissions",
			"path", path, "mode", fmt.Sprintf("%04o", info.Mode().Perm()))
	}

	var record azidentity.AuthenticationRecord
	if err := json.Unmarshal(data, &record); err != nil {
		slog.Warn("authentication record contains invalid JSON, treating as first run",
			"path", path, "error", err)
		return azidentity.AuthenticationRecord{}
	}

	slog.Info("authentication record loaded", "path", path)
	return record
}

// SaveAuthRecord serializes the AuthenticationRecord to JSON and writes it to
// the file at path. The parent directory is created with permissions 0700 if
// it does not exist. The file is written with permissions 0600 (owner
// read/write only).
//
// Parameters:
//   - path: absolute filesystem path where the record will be saved.
//   - record: the AuthenticationRecord obtained from a successful authentication.
//
// Returns an error if directory creation, JSON marshaling, or file writing fails.
//
// Side effects: creates directories and writes a file to disk.
func SaveAuthRecord(path string, record azidentity.AuthenticationRecord) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create auth record directory %s: %w", dir, err)
	}

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal authentication record: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write authentication record to %s: %w", path, err)
	}

	slog.Info("authentication record saved", "path", path)
	return nil
}

// SetupCredential constructs an Azure Identity credential configured with a
// persistent token cache, an authentication record loaded from disk, and the
// application's client and tenant IDs. The credential type is selected based
// on cfg.AuthMethod:
//
//   - "auth_code": constructs an AuthCodeCredential that implements the manual
//     authorization code flow with PKCE using the nativeclient redirect URI.
//     Uses MSAL Go's public.Client for token acquisition.
//   - "browser": constructs an InteractiveBrowserCredential that opens the
//     system browser for OAuth login via the authorization code flow with PKCE.
//     The RedirectURL is set to "http://localhost" as required by the app
//     registration.
//   - "device_code": constructs a DeviceCodeCredential with a context-aware
//     UserPrompt callback that sends the device code message as an MCP
//     notification or falls back to stderr.
//
// Authentication is deferred to the first tool call rather than blocking at
// startup.
//
// Parameters:
//   - cfg: the application configuration providing AuthMethod, ClientID,
//     TenantID, AuthRecordPath, and CacheName.
//
// Returns the credential as both azcore.TokenCredential (for the Graph SDK)
// and Authenticator (for the middleware), or an error if credential
// construction fails. The credential does not hold a token until GetToken
// is called (lazily by the Graph SDK on the first API call).
//
// Side effects: initializes the OS keychain token cache, reads the auth record
// from disk.
func SetupCredential(cfg config.Config) (azcore.TokenCredential, Authenticator, error) {
	tokenCache := InitCache(cfg.CacheName, cfg.TokenStorage)
	record := LoadAuthRecord(cfg.AuthRecordPath)

	switch cfg.AuthMethod {
	case "auth_code":
		return setupAuthCodeCredential(cfg)
	case "browser":
		return setupBrowserCredential(cfg, tokenCache, record)
	case "device_code":
		return setupDeviceCodeCredential(cfg, tokenCache, record)
	default:
		return nil, nil, fmt.Errorf("unsupported auth method: %s", cfg.AuthMethod)
	}
}

// setupAuthCodeCredential constructs an AuthCodeCredential configured with
// an MSAL persistent token cache and a persisted account loaded from disk.
// Unlike browser and device_code credentials, auth_code uses MSAL Go's
// public.Client directly rather than azidentity, so it has its own cache
// initialization via InitMSALCache.
//
// Parameters:
//   - cfg: application configuration providing ClientID, TenantID, CacheName,
//     and AuthRecordPath.
//
// Returns the credential as both TokenCredential and Authenticator, or an
// error if credential construction fails.
func setupAuthCodeCredential(cfg config.Config) (azcore.TokenCredential, Authenticator, error) {
	cacheAccessor := InitMSALCache(cfg.CacheName, cfg.TokenStorage)

	var opts []NewAuthCodeCredentialOption
	if cacheAccessor != nil {
		opts = append(opts, WithCacheAccessor(cacheAccessor))
	}

	cred, err := NewAuthCodeCredential(cfg.ClientID, cfg.TenantID, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("create auth code credential: %w", err)
	}

	if err := cred.LoadPersistedAccount(cfg.AuthRecordPath); err != nil {
		slog.Warn("failed to load persisted auth code account", "error", err)
	}

	slog.Info("credential constructed (auth_code), authentication deferred to first tool call")
	return cred, cred, nil
}

// setupBrowserCredential constructs an InteractiveBrowserCredential configured
// with the provided token cache, authentication record, and application
// identity from cfg. The RedirectURL is set to "http://localhost" as required
// by the OAuth authorization code flow with PKCE.
//
// Parameters:
//   - cfg: application configuration providing ClientID and TenantID.
//   - tokenCache: persistent OS-native token cache.
//   - record: authentication record from a previous session (may be zero-value).
//
// Returns the credential as both TokenCredential and Authenticator, or an
// error if credential construction fails.
func setupBrowserCredential(cfg config.Config, tokenCache azidentity.Cache, record azidentity.AuthenticationRecord) (azcore.TokenCredential, Authenticator, error) {
	opts := &azidentity.InteractiveBrowserCredentialOptions{
		ClientID:             cfg.ClientID,
		TenantID:             cfg.TenantID,
		Cache:                tokenCache,
		AuthenticationRecord: record,
		RedirectURL:          "http://localhost",
	}

	cred, err := azidentity.NewInteractiveBrowserCredential(opts)
	if err != nil {
		return nil, nil, fmt.Errorf("create interactive browser credential: %w", err)
	}

	slog.Info("credential constructed (browser), authentication deferred to first tool call")
	return cred, cred, nil
}

// setupDeviceCodeCredential constructs a DeviceCodeCredential configured with
// the provided token cache, authentication record, application identity from
// cfg, and the deviceCodeUserPrompt callback for forwarding the device code
// message to the user.
//
// Parameters:
//   - cfg: application configuration providing ClientID and TenantID.
//   - tokenCache: persistent OS-native token cache.
//   - record: authentication record from a previous session (may be zero-value).
//
// Returns the credential as both TokenCredential and Authenticator, or an
// error if credential construction fails.
func setupDeviceCodeCredential(cfg config.Config, tokenCache azidentity.Cache, record azidentity.AuthenticationRecord) (azcore.TokenCredential, Authenticator, error) {
	opts := &azidentity.DeviceCodeCredentialOptions{
		ClientID:             cfg.ClientID,
		TenantID:             cfg.TenantID,
		Cache:                tokenCache,
		AuthenticationRecord: record,
		UserPrompt:           deviceCodeUserPrompt,
	}

	cred, err := azidentity.NewDeviceCodeCredential(opts)
	if err != nil {
		return nil, nil, fmt.Errorf("create device code credential: %w", err)
	}

	slog.Info("credential constructed (device_code), authentication deferred to first tool call")
	return cred, cred, nil
}

// deviceCodeMsgKeyType is the context key type for passing a device code
// message channel from callers to deviceCodeUserPrompt.
type deviceCodeMsgKeyType struct{}

// DeviceCodeMsgKey is the context key for injecting a chan string into the
// authentication context. The middleware and add_account handler write this
// channel so that deviceCodeUserPrompt can forward the device code message
// back, allowing callers to present it to the user.
var DeviceCodeMsgKey = deviceCodeMsgKeyType{}

// deviceCodeUserPrompt is the UserPrompt callback used by DeviceCodeCredential.
// It attempts to send the device code message as a LoggingMessageNotification
// via the MCPServer found in ctx (populated by AuthMiddleware's mergedContext
// during re-authentication). When no MCPServer is available, the message is
// written to stderr as a fallback.
//
// Additionally, if the context contains a device code message channel (injected
// by the auth middleware via DeviceCodeMsgKey), the message is forwarded through
// it so the middleware can return it as a tool result visible to the user.
//
// Parameters:
//   - ctx: the context provided by the credential during Authenticate. When
//     called from AuthMiddleware, this context contains the MCPServer.
//   - msg: the device code message containing the URL and code for the user.
//
// Returns nil on success. Returns an error only if stderr writing fails (which
// is unlikely and non-fatal).
func deviceCodeUserPrompt(ctx context.Context, msg azidentity.DeviceCodeMessage) error {
	// Forward to middleware channel if available, so the device code
	// message can be returned as a tool result visible in the chat.
	if ch, ok := ctx.Value(DeviceCodeMsgKey).(chan string); ok {
		select {
		case ch <- msg.Message:
		default:
		}
	}

	srv := mcpserver.ServerFromContext(ctx)
	if srv != nil {
		notification := mcp.NewLoggingMessageNotification(
			mcp.LoggingLevelWarning, "auth", msg.Message,
		)
		if err := srv.SendLogMessageToClient(ctx, notification); err != nil {
			slog.Warn("failed to send device code prompt to client, falling back to stderr",
				"error", err)
		} else {
			return nil
		}
	}
	// Fallback: write to stderr when MCPServer is unavailable.
	_, err := fmt.Fprintln(os.Stderr, msg.Message)
	return err
}

// SetupCredentialForAccount constructs an Azure Identity credential for a
// specific account, using per-account token cache partitioning and a dedicated
// auth record path. This is the multi-account counterpart to SetupCredential,
// used by the add_account tool to create credentials dynamically at runtime.
//
// The caller is responsible for providing non-empty clientID, tenantID, and
// authMethod values. The add_account handler resolves defaults from the server
// config before calling this function.
//
// Parameters:
//   - label: the unique account label, used to derive the cache partition and
//     auth record filename.
//   - clientID: the OAuth 2.0 client (application) ID. Must be non-empty.
//   - tenantID: the Entra ID tenant identifier. Must be non-empty.
//   - authMethod: the authentication method ("auth_code", "browser", or
//     "device_code"). Must be non-empty.
//   - cacheNameBase: the base name for the OS keychain partition. The account
//     label is appended as "{cacheNameBase}-{label}".
//   - authRecordDir: the directory where auth record files are stored. The
//     per-account file is named "{label}_auth_record.json" within this directory.
//   - tokenStorage: the token storage backend ("auto", "keychain", or "file"),
//     forwarded from the top-level server configuration so that per-account
//     credentials honour the same storage preference as the default credential.
//     An empty string falls through to the "auto" behaviour in InitCache.
//
// Returns the credential as azcore.TokenCredential, the Authenticator interface,
// the full auth record path, and the cache name. Returns an error if the auth
// method is unsupported or credential construction fails.
//
// Side effects: initializes the token cache (OS keychain or file-based
// depending on tokenStorage), reads the auth record from disk if present.
func SetupCredentialForAccount(label, clientID, tenantID, authMethod, cacheNameBase, authRecordDir, tokenStorage string) (azcore.TokenCredential, Authenticator, string, string, error) {
	cacheName := cacheNameBase + "-" + label
	authRecordPath := filepath.Join(authRecordDir, label+"_auth_record.json")

	cfg := config.Config{
		ClientID:       clientID,
		TenantID:       tenantID,
		AuthRecordPath: authRecordPath,
		CacheName:      cacheName,
		AuthMethod:     authMethod,
		TokenStorage:   tokenStorage,
	}

	cred, auth, err := SetupCredential(cfg)
	if err != nil {
		return nil, nil, "", "", err
	}

	return cred, auth, authRecordPath, cacheName, nil
}

// Authenticate performs interactive authentication using the provided
// Authenticator and persists the resulting AuthenticationRecord to disk. This
// function is intended to be called by the AuthMiddleware when a tool call
// encounters an authentication error, rather than during server startup.
//
// The Authenticator may be either an InteractiveBrowserCredential (which opens
// the system browser) or a DeviceCodeCredential (which invokes a UserPrompt
// callback), depending on the configured auth method.
//
// Parameters:
//   - ctx: context for the authentication call. Should not be bound to a
//     short-lived tool call deadline, since interactive authentication requires
//     user action (up to ~15 minutes for device code flow).
//   - auth: the Authenticator to use for authentication.
//   - authRecordPath: absolute filesystem path where the AuthenticationRecord
//     will be persisted on success.
//   - scopes: OAuth scopes to request (e.g., from Scopes(cfg)).
//
// Returns the AuthenticationRecord on success, or an error if the
// authentication flow fails or the record cannot be saved.
//
// Side effects: triggers interactive authentication (browser launch or device
// code prompt), writes the AuthenticationRecord to disk on success.
func Authenticate(ctx context.Context, auth Authenticator, authRecordPath string, scopes []string) (azidentity.AuthenticationRecord, error) {
	record, err := auth.Authenticate(ctx, &policy.TokenRequestOptions{
		Scopes:    scopes,
		EnableCAE: true,
	})
	if err != nil {
		return azidentity.AuthenticationRecord{}, fmt.Errorf("authentication failed: %w", err)
	}

	if saveErr := SaveAuthRecord(authRecordPath, record); saveErr != nil {
		slog.Warn("failed to save authentication record, next run will re-authenticate",
			"error", saveErr)
	}

	return record, nil
}
