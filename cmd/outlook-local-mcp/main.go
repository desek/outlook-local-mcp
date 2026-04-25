// Package main is the entry point for the Outlook Local MCP Server binary.
// It executes the startup lifecycle: load configuration, initialize subsystems
// (logging, audit, OpenTelemetry, authentication), create the Graph client and
// MCP server, register tools, and start the stdio transport.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/desek/outlook-local-mcp/internal/audit"
	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/config"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/logging"
	"github.com/desek/outlook-local-mcp/internal/observability"
	internalserver "github.com/desek/outlook-local-mcp/internal/server"
	"github.com/mark3labs/mcp-go/server"
	_ "github.com/microsoft/kiota-abstractions-go"
	_ "github.com/microsoft/kiota-authentication-azure-go"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	_ "github.com/microsoftgraph/msgraph-sdk-go-core"
	"go.opentelemetry.io/otel"
)

// version is the application version, injected at build time via
// -ldflags="-X main.version=<value>". Defaults to "dev" for local builds.
var version = "dev"

// main is the application entry point. It executes the full 11-step startup
// lifecycle in order: (1) init logger, (2) load config, (3-6) authentication
// (cache, auth record, credential, authenticate), (7) create Graph client,
// (8) create MCP server, (9) register tools, (10) start stdio transport,
// (11) log shutdown on exit.
//
// Side effects: reads environment variables, writes logs to stderr, may prompt
// the user for interactive authentication on first run (browser or device code
// depending on cfg.AuthMethod), writes auth record to disk on successful
// first-run authentication, creates a Graph client, starts the MCP stdio
// transport on stdin/stdout.
// Exits with code 1 if authentication setup, Graph client creation, or the
// stdio transport fails.
func main() {
	// Steps 1-2: Load config, validate, then init logger
	cfg := config.LoadConfig()
	cfg.Version = version
	if err := config.ValidateConfig(cfg); err != nil {
		slog.Error("configuration validation failed", "error", err)
		os.Exit(1)
	}
	logging.InitLogger(cfg.LogLevel, cfg.LogFormat, cfg.LogSanitize, cfg.LogFile)
	defer logging.CloseLogFile()
	audit.InitAuditLog(cfg.AuditLogEnabled, cfg.AuditLogPath)
	logFileField := "none"
	if cfg.LogFile != "" {
		logFileField = cfg.LogFile
	}
	slog.Info("server starting", "version", version, "transport", "stdio",
		"log_sanitize", cfg.LogSanitize, "log_file", logFileField,
		"audit_enabled", cfg.AuditLogEnabled,
		"read_only", cfg.ReadOnly, "auth_method", cfg.AuthMethod,
		"accounts", 1)
	slog.Info("request timeout configured", "timeout_seconds", int(cfg.RequestTimeout.Seconds()))

	// Initialize OpenTelemetry (noop or OTLP based on config).
	shutdownOTEL, err := observability.InitOTEL(cfg)
	if err != nil {
		slog.Error("otel initialization failed", "error", err)
		os.Exit(1) //nolint:gocritic // exitAfterDefer: intentional — defers are non-critical cleanup; startup failure must exit immediately
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownOTEL(ctx); err != nil {
			slog.Error("otel shutdown failed", "error", err)
		}
	}()

	// Steps 3-6: Authentication (cache, record, credential, authenticate)
	cred, authenticator, err := auth.SetupCredential(cfg)
	if err != nil {
		slog.Error("authentication setup failed", "error", err)
		os.Exit(1) //nolint:gocritic // exitAfterDefer: intentional — defers are non-critical cleanup; startup failure must exit immediately
	}

	// CR-0049: Record which token cache backend was actually resolved.
	cfg.TokenCacheBackend = auth.ResolveTokenCacheBackend(cfg.CacheName, cfg.TokenStorage)

	// Step 7: Create Graph client
	scopes := auth.Scopes(cfg)
	graphClient, err := msgraphsdk.NewGraphServiceClientWithCredentials(
		cred, scopes,
	)
	if err != nil {
		slog.Error("graph client initialization failed", "error", err)
		os.Exit(1)
	}
	slog.Info("graph client initialized", "scopes", scopes)

	// Step 7b: Create account registry. Conditionally register the startup
	// credential as the implicit "default" account entry. The implicit default
	// is skipped when accounts.json already covers the cfg identity (same
	// client_id + tenant_id) or already contains an explicit "default" label.
	// This prevents ghost entries and duplicate state in multi-account setups
	// (CR-0064).
	registry := auth.NewAccountRegistry()
	if shouldAddImplicitDefault(cfg.AccountsPath, cfg.ClientID, cfg.TenantID) {
		if err := registry.Add(&auth.AccountEntry{
			Label:          "default",
			ClientID:       cfg.ClientID,
			TenantID:       cfg.TenantID,
			AuthMethod:     cfg.AuthMethod,
			Credential:     cred,
			Authenticator:  authenticator,
			Client:         graphClient,
			AuthRecordPath: cfg.AuthRecordPath,
			CacheName:      cfg.CacheName,
			Authenticated:  true,
		}); err != nil {
			slog.Error("default account registration failed", "error", err)
			os.Exit(1)
		}
	}

	// Step 7c: Restore additional accounts from the persistent accounts file.
	// Accounts with valid cached tokens get a functional Graph client;
	// accounts with expired tokens are registered for deferred re-auth.
	authRecordDir := auth.AuthRecordDir(cfg.AuthRecordPath)
	restored, total := auth.RestoreAccounts(
		cfg.AccountsPath, cfg.CacheName, authRecordDir,
		registry, auth.SetupCredentialForAccount, auth.NewDefaultGraphClientFactory(scopes),
		scopes, cfg.TokenStorage,
	)
	if total > 0 {
		slog.Info("additional accounts loaded",
			"restored", restored, "total", total,
			"accounts", registry.Count())
	}

	// Step 7d: Create auth middleware (with fresh-credential detection).
	authMiddleware, markPreAuthenticated := auth.AuthMiddleware(authenticator, cfg.AuthRecordPath, cfg.AuthMethod, scopes)

	// Step 7e: Startup silent token probe. Attempt to silently acquire a token
	// for the default credential with a short timeout. This MUST NOT trigger
	// interactive authentication (no browser, no device code). If silent
	// renewal succeeds, the middleware is informed via markPreAuthenticated
	// so it can skip the fresh-credential fast-path and call the inner
	// handler normally. If it fails, the auth middleware will handle
	// re-authentication on the first tool call.
	//
	// For device_code credentials, the probe is skipped (would trigger
	// interactive auth). Instead, if a persistent auth record exists on disk,
	// we mark as pre-authenticated: the file-based token cache likely has
	// valid tokens from a previous session, so the middleware should try the
	// normal Graph API path first rather than the fresh-credential fast-path.
	probeStartupToken(cred, cfg.AuthMethod, cfg.AuthRecordPath, markPreAuthenticated, scopes)

	// Step 8: Create MCP server with elicitation capability for multi-account
	// account selection and interactive authentication prompts.
	s := server.NewMCPServer("outlook-local", version,
		server.WithToolCapabilities(false),
		server.WithResourceCapabilities(false, false),
		server.WithRecovery(),
		server.WithLogging(),
		server.WithElicitation(),
	)

	// Step 9: Initialize metrics, create tracer, register tools.
	meter := otel.Meter(cfg.OTELServiceName)
	tracer := otel.Tracer(cfg.OTELServiceName)
	metrics, err := observability.InitMetrics(meter)
	if err != nil {
		slog.Error("metrics initialization failed", "error", err)
		os.Exit(1)
	}
	retryCfg := graph.RetryConfig{
		MaxRetries:     cfg.MaxRetries,
		InitialBackoff: time.Duration(cfg.RetryBackoffMS) * time.Millisecond,
		Logger:         slog.Default(),
	}
	internalserver.RegisterTools(s, retryCfg, cfg.RequestTimeout, metrics, tracer, cfg.ReadOnly, authMiddleware, registry, cfg, authenticator)
	internalserver.RegisterResources(s)

	// Create root context for graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = ctx // Root context available for future use by tool handlers.

	// Create done channel to signal when ServeStdio returns.
	done := make(chan struct{})

	// Install context-aware shutdown handler with OTEL flush on exit.
	internalserver.AwaitShutdownSignal(cancel, cfg.ShutdownTimeout, done, shutdownOTEL)

	// Step 10: Start stdio transport (blocks)
	if err := server.ServeStdio(s); err != nil {
		slog.Error("stdio transport error", "error", err)
		cancel()
		close(done)
		os.Exit(1)
	}

	// Step 11: Shutdown (stdin closed path)
	slog.Info("server shutting down", "reason", "stdin closed")
	cancel()
	close(done)
}

// shouldAddImplicitDefault reports whether the startup lifecycle should register
// an implicit "default" AccountEntry from the env cfg. It returns false when:
//   - clientID is empty (no env cfg identity to register), OR
//   - accounts.json contains an entry whose (client_id, tenant_id) matches
//     the supplied clientID and tenantID (identity already covered), OR
//   - accounts.json contains any entry with the literal label "default"
//     (explicit default already configured).
//
// When accounts.json is missing or empty, this function returns true so that
// the single-account env-only configuration path continues to work.
//
// Parameters:
//   - accountsPath: path to the accounts JSON file.
//   - clientID: the OAuth 2.0 client ID from the env cfg.
//   - tenantID: the Entra ID tenant identifier from the env cfg.
//
// Side effects: reads accounts.json from disk. Logs a warning if the file
// cannot be read (other than not-found), and treats it as an empty file.
func shouldAddImplicitDefault(accountsPath, clientID, tenantID string) bool {
	if clientID == "" {
		return false
	}

	accounts, err := auth.LoadAccounts(accountsPath)
	if err != nil {
		slog.Warn("failed to load accounts for implicit-default check, assuming empty",
			"error", err, "path", accountsPath)
		accounts = nil
	}

	if _, found := auth.FindByIdentity(accounts, clientID, tenantID); found {
		slog.Info("implicit default registration skipped: accounts.json covers cfg identity",
			"client_id", clientID, "tenant_id", tenantID)
		return false
	}

	for _, a := range accounts {
		if a.Label == "default" {
			slog.Info("implicit default registration skipped: accounts.json has explicit default label")
			return false
		}
	}

	return true
}

// startupProbeTimeout is the maximum duration for the startup silent token
// probe. The probe MUST complete within this bound and MUST NOT trigger
// interactive authentication.
const startupProbeTimeout = 5 * time.Second

// probeStartupToken attempts a silent token acquisition for the default
// credential with a short timeout. If the cached token is valid or can be
// silently renewed, markPreAuthenticated is called to inform the auth
// middleware that the credential is ready, preventing the fresh-credential
// fast-path from triggering.
//
// Device code credentials are skipped because GetToken would trigger the
// device code callback (interactive auth). Instead, the function checks
// whether a persistent auth record exists on disk — if so, the user
// authenticated in a previous session and the file-based token cache likely
// contains valid tokens. In that case, markPreAuthenticated is called so the
// middleware tries the normal Graph API path (which uses the cached token)
// rather than the fresh-credential fast-path that skips straight to re-auth.
//
// Parameters:
//   - cred: the default credential implementing azcore.TokenCredential.
//   - authMethod: the configured auth method for the default account.
//   - authRecordPath: path to the persistent auth record file on disk.
//   - markPreAuthenticated: callback to signal the middleware that the
//     credential has a valid cached token.
//   - scopes: OAuth scopes to request for the token probe (from auth.Scopes(cfg)).
//
// Side effects: logs the probe result. Does not block startup on failure.
func probeStartupToken(cred azcore.TokenCredential, authMethod, authRecordPath string, markPreAuthenticated func(), scopes []string) {
	// Device code credentials trigger interactive auth on GetToken when no
	// cached token exists. Skip the probe to avoid spurious output.
	if authMethod == "device_code" {
		// Check if a persistent auth record exists from a previous session.
		// If so, the file-based token cache should have valid tokens and the
		// middleware should try the normal path rather than fresh-credential.
		if _, err := os.Stat(authRecordPath); err == nil {
			markPreAuthenticated()
			slog.Info("startup token probe skipped for device_code, auth record exists",
				"auth_record", authRecordPath,
				"reason", "file-based cache likely has valid tokens from previous session")
			return
		}
		slog.Info("startup token probe skipped", "auth_method", "device_code",
			"reason", "would trigger interactive device code flow, no prior auth record")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), startupProbeTimeout)
	defer cancel()

	_, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: scopes,
	})
	if err != nil {
		slog.Info("startup token probe failed, re-authentication deferred to first tool call",
			"error", err)
		return
	}

	markPreAuthenticated()
	slog.Info("startup token probe succeeded, credential has valid cached token")
}
