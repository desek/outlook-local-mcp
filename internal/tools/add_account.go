// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the add_account MCP tool, which registers a new Microsoft
// account in the account registry. It creates a per-account credential with
// isolated token cache and auth record, performs inline authentication using
// MCP Elicitation (URL mode for browser, form mode for device code), creates a
// Graph client, and registers the account for use by other tools.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/config"
	"github.com/desek/outlook-local-mcp/internal/logging"
	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/pkg/browser"
)

// NewAddAccountTool creates the MCP tool definition for add_account. The tool
// accepts a required "label" parameter and optional "client_id", "tenant_id",
// and "auth_method" parameters for customizing the account's credential.
//
// Returns the configured mcp.Tool ready for registration with server.AddTool.
func NewAddAccountTool() mcp.Tool {
	return mcp.NewTool("account_add",
		mcp.WithTitleAnnotation("Add Account"),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithDescription(
			"Add and authenticate a new Microsoft account. "+
				"Call this tool directly when the user wants to connect an account — do not ask them to configure it manually. "+
				"Performs the full authentication flow inline and presents the result to the user. "+
				"After authentication completes, the User Principal Name (UPN) is resolved from Microsoft Graph /me and persisted to accounts.json so subsequent restarts surface the account's identity without an extra API call. "+
				"The label parameter is a short identifier for the account (e.g., a nickname); the UPN is the canonical identity that tools and surfaces display alongside the label. "+
				"Leave auth_method blank unless the user explicitly requests a specific method; the server will use its configured default.",
		),
		mcp.WithString("label",
			mcp.Required(),
			mcp.Description("Unique label for the account (1-64 chars, alphanumeric/underscore/hyphen)."),
		),
		mcp.WithString("client_id",
			mcp.Description("OAuth client ID. Defaults to the server's configured client ID."),
		),
		mcp.WithString("tenant_id",
			mcp.Description("Entra ID tenant ID. Defaults to the server's configured tenant ID."),
		),
		mcp.WithString("auth_method",
			mcp.Description("Authentication method: 'browser', 'device_code', or 'auth_code'. Defaults to the server's configured method."),
		),
	)
}

// graphClientFactory is the function used to create a Graph client from a
// credential. It is a package-level variable to allow test injection. In
// production, it is initialized by HandleAddAccount using the scopes from
// the server configuration.
var graphClientFactory auth.GraphClientFactory

// addAccountState holds injectable dependencies for the add_account handler.
// This allows tests to replace credential creation, elicitation, and
// authentication functions.
type addAccountState struct {
	// setupCredential creates per-account credentials (token credential,
	// authenticator, auth record path, cache name). Defaults to
	// auth.SetupCredentialForAccount in production; tests inject a mock.
	// The tokenStorage parameter is forwarded from cfg.TokenStorage so that
	// per-account credentials honour the same storage backend as the default.
	setupCredential func(label, clientID, tenantID, authMethod, cacheName, authRecordDir, tokenStorage string) (
		azcore.TokenCredential, auth.Authenticator, string, string, error)

	// authenticate performs authentication and persists the auth record.
	authenticate func(ctx context.Context, auth auth.Authenticator, authRecordPath string, scopes []string) (azidentity.AuthenticationRecord, error)

	// urlElicit requests URL mode elicitation from the MCP client.
	urlElicit func(ctx context.Context, elicitationID, url, message string) (*mcp.ElicitationResult, error)

	// elicit requests form mode elicitation from the MCP client.
	elicit func(ctx context.Context, request mcp.ElicitationRequest) (*mcp.ElicitationResult, error)

	// openBrowser opens a URL in the system browser. Defaults to
	// browser.OpenURL in production; tests inject a no-op.
	openBrowser func(url string) error

	// pendingMu protects the pending map from concurrent access.
	pendingMu sync.Mutex

	// pending stores in-progress authentication state keyed by account label.
	// Used to keep device_code auth goroutines alive between add_account calls
	// when elicitation is not supported by the MCP client.
	pending map[string]*pendingAccount

	// scopes holds the OAuth scopes to use for authentication and token
	// exchange during add_account. Built from auth.Scopes(cfg).
	scopes []string
}

// pendingAccount holds the in-progress authentication state for a device_code
// add_account call where elicitation failed. The authentication goroutine
// continues running in the background. The next add_account call with the
// same label checks this state instead of creating a new credential.
type pendingAccount struct {
	// cred is the Azure token credential created for this account.
	cred azcore.TokenCredential

	// authenticator is the credential's Authenticator for completing auth.
	authenticator auth.Authenticator

	// authRecordPath is the filesystem path for persisting the auth record.
	authRecordPath string

	// cacheName is the OS keychain cache partition name for this account.
	cacheName string

	// clientID is the OAuth client ID used for this account.
	clientID string

	// tenantID is the Entra ID tenant ID used for this account.
	tenantID string

	// authMethod is the authentication method (always "device_code" for pending accounts).
	authMethod string

	// cancel cancels the authentication context, releasing resources after the
	// goroutine completes or on cleanup.
	cancel context.CancelFunc

	// done is closed when the authentication goroutine completes.
	done chan struct{}

	// err holds the authentication result, valid only after done is closed.
	err error
}

// defaultAddAccountState returns the production addAccountState with real
// implementations for authentication and elicitation.
//
// Parameters:
//   - scopes: OAuth scopes to use for authentication (from auth.Scopes(cfg)).
func defaultAddAccountState(scopes []string) *addAccountState {
	return &addAccountState{
		setupCredential: auth.SetupCredentialForAccount,
		authenticate:    auth.Authenticate,
		urlElicit:       defaultAddAccountURLElicit,
		elicit:          defaultAddAccountElicit,
		openBrowser:     browser.OpenURL,
		pending:         make(map[string]*pendingAccount),
		scopes:          scopes,
	}
}

// defaultAddAccountURLElicit retrieves the MCPServer and ClientSession from
// context and calls RequestURLElicitation.
func defaultAddAccountURLElicit(ctx context.Context, elicitationID, url, message string) (*mcp.ElicitationResult, error) {
	srv := mcpserver.ServerFromContext(ctx)
	if srv == nil {
		return nil, mcpserver.ErrElicitationNotSupported
	}
	session := mcpserver.ClientSessionFromContext(ctx)
	if session == nil {
		return nil, mcpserver.ErrElicitationNotSupported
	}
	return srv.RequestURLElicitation(ctx, session, elicitationID, url, message)
}

// defaultAddAccountElicit retrieves the MCPServer from context and calls
// RequestElicitation.
func defaultAddAccountElicit(ctx context.Context, request mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
	srv := mcpserver.ServerFromContext(ctx)
	if srv == nil {
		return nil, mcpserver.ErrElicitationNotSupported
	}
	return srv.RequestElicitation(ctx, request)
}

// HandleAddAccount creates a tool handler that adds a new account to the
// registry. The handler validates the label, creates a per-account credential,
// performs inline authentication using MCP Elicitation (URL mode for browser,
// form mode for device code), builds a Graph client, and registers the account.
//
// Parameters:
//   - registry: the account registry where the new account will be stored.
//   - cfg: the server configuration providing default values for client_id,
//     tenant_id, auth_method, cache_name, and auth_record_path.
//
// Returns a tool handler function compatible with the MCP server's AddTool
// method. The handler returns a JSON success message with the account label,
// or an error result if validation, credential creation, authentication, or
// registration fails.
//
// Side effects: creates a new credential with OS keychain cache partition,
// performs authentication (browser or device code), persists the auth record,
// registers the account in the registry.
func HandleAddAccount(registry *auth.AccountRegistry, cfg config.Config) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	scopes := auth.Scopes(cfg)
	// Initialize the package-level graph client factory with the configured
	// scopes so that add_account creates Graph clients with the correct scope
	// set (including Mail.Read when mail is enabled).
	graphClientFactory = auth.NewDefaultGraphClientFactory(scopes)
	state := defaultAddAccountState(scopes)
	return state.handleAddAccount(registry, cfg)
}

// handleAddAccount is the internal implementation that uses the injectable
// addAccountState for elicitation and authentication.
func (s *addAccountState) handleAddAccount(registry *auth.AccountRegistry, cfg config.Config) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.Logger(ctx)

		label, err := request.RequireString("label")
		if err != nil {
			return mcp.NewToolResultError("missing required parameter: label"), nil
		}

		logger.Debug("tool called", "label", label)

		// Check if there is a pending authentication for this label.
		if p, result := s.checkPending(label, logger); result != nil {
			return result, nil
		} else if p != nil {
			// Pending auth completed successfully — register the account.
			client, err := graphClientFactory(p.cred)
			if err != nil {
				logger.Error("graph client creation failed", "label", label, "error", err.Error())
				return mcp.NewToolResultError(fmt.Sprintf("failed to create Graph client for account %q: %s", label, err.Error())), nil
			}
			entry := &auth.AccountEntry{
				Label: label, ClientID: p.clientID, TenantID: p.tenantID,
				AuthMethod: p.authMethod, Credential: p.cred, Authenticator: p.authenticator,
				Client: client, AuthRecordPath: p.authRecordPath, CacheName: p.cacheName,
				Authenticated: true,
			}
			if err := registry.Add(entry); err != nil {
				logger.Error("account registration failed", "label", label, "error", err.Error())
				return mcp.NewToolResultError(err.Error()), nil
			}
			if err := auth.AddAccountConfig(cfg.AccountsPath, auth.AccountConfig{
				Label: label, ClientID: p.clientID, TenantID: p.tenantID, AuthMethod: p.authMethod,
			}); err != nil {
				logger.Warn("failed to persist account config", "label", label, "error", err.Error())
			}
			// Resolve UPN from /me and backfill accounts.json so entry.Email is
			// populated immediately (CR-0056 FR-2/FR-3).
			auth.EnsureEmailAndPersistUPN(ctx, entry, cfg.AccountsPath)
			result := map[string]any{"added": true, "label": label, "upn": entry.Email, "message": fmt.Sprintf("Account %q added and authenticated successfully.", label)}
			data, err := json.Marshal(result)
			if err != nil {
				return mcp.NewToolResultError("failed to serialize response"), nil
			}
			logger.Info("account added from pending authentication", "label", label)
			return mcp.NewToolResultText(string(data)), nil
		}

		// Validate label uniqueness.
		if _, exists := registry.Get(label); exists {
			return mcp.NewToolResultError(fmt.Sprintf("account %q already exists", label)), nil
		}

		// Extract optional parameters with defaults from config.
		clientID := request.GetString("client_id", cfg.ClientID)
		tenantID := request.GetString("tenant_id", cfg.TenantID)
		authMethod := request.GetString("auth_method", cfg.AuthMethod)

		// Derive per-account auth record directory from the existing config path.
		authRecordDir := filepath.Dir(cfg.AuthRecordPath)

		// Create per-account credential via the injectable factory.
		cred, authenticator, authRecordPath, cacheName, err := s.setupCredential(
			label, clientID, tenantID, authMethod, cfg.CacheName, authRecordDir, cfg.TokenStorage,
		)
		if err != nil {
			logger.Error("credential setup failed", "label", label, "error", err.Error())
			return mcp.NewToolResultError(fmt.Sprintf("failed to set up credential for account %q: %s", label, err.Error())), nil
		}

		// Perform inline authentication using elicitation.
		authErr := s.authenticateInline(ctx, cred, authenticator, authRecordPath, authMethod, cacheName, clientID, tenantID, label, logger)
		if authErr != nil {
			// DeviceCodeFallbackError means elicitation failed but we captured the
			// device code. Return it as successful tool result text so the user
			// sees it in the chat.
			var dcErr *DeviceCodeFallbackError
			if errors.As(authErr, &dcErr) {
				return mcp.NewToolResultText(dcErr.Message), nil
			}
			logger.Error("authentication failed during add_account", "label", label, "error", authErr.Error())
			return mcp.NewToolResultError(fmt.Sprintf("failed to authenticate account %q: %s", label, authErr.Error())), nil
		}

		// Create Graph client for the new account.
		client, err := graphClientFactory(cred)
		if err != nil {
			logger.Error("graph client creation failed", "label", label, "error", err.Error())
			return mcp.NewToolResultError(fmt.Sprintf("failed to create Graph client for account %q: %s", label, err.Error())), nil
		}

		// Register the account with identity metadata for persistence.
		entry := &auth.AccountEntry{
			Label:          label,
			ClientID:       clientID,
			TenantID:       tenantID,
			AuthMethod:     authMethod,
			Credential:     cred,
			Authenticator:  authenticator,
			Client:         client,
			AuthRecordPath: authRecordPath,
			CacheName:      cacheName,
			Authenticated:  true,
		}

		if err := registry.Add(entry); err != nil {
			logger.Error("account registration failed", "label", label, "error", err.Error())
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Persist account identity configuration to accounts.json.
		if err := auth.AddAccountConfig(cfg.AccountsPath, auth.AccountConfig{
			Label:      label,
			ClientID:   clientID,
			TenantID:   tenantID,
			AuthMethod: authMethod,
		}); err != nil {
			logger.Warn("failed to persist account config", "label", label, "error", err.Error())
		}

		// Resolve UPN from /me and backfill accounts.json so entry.Email is
		// populated immediately (CR-0056 FR-2/FR-3).
		auth.EnsureEmailAndPersistUPN(ctx, entry, cfg.AccountsPath)

		result := map[string]any{
			"added":   true,
			"label":   label,
			"upn":     entry.Email,
			"message": fmt.Sprintf("Account %q added and authenticated successfully.", label),
		}

		data, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError("failed to serialize response"), nil
		}

		logger.Info("account added and authenticated", "label", label)
		return mcp.NewToolResultText(string(data)), nil
	}
}

// authenticateInline performs authentication during add_account using MCP
// Elicitation. For browser auth, it uses URL mode elicitation to direct the
// user to the Microsoft login page. For device code auth, it uses form mode
// elicitation to display the verification URL and user code.
//
// Falls back to tool result text when elicitation is not supported, so the
// user sees actionable instructions in MCP clients like Claude Code.
//
// Parameters:
//   - ctx: the tool handler context containing the MCPServer.
//   - cred: the Azure token credential created for this account.
//   - authenticator: the credential's Authenticator for authentication.
//   - authRecordPath: filesystem path for persisting the auth record.
//   - authMethod: "browser", "device_code", or "auth_code".
//   - cacheName: the OS keychain cache partition name for this account.
//   - clientID: the OAuth client ID used for this account.
//   - tenantID: the Entra ID tenant ID used for this account.
//   - label: the account label, included in fallback messages.
//   - logger: structured logger for the tool.
//
// Returns nil on success, or an error if authentication fails. May return
// a *DeviceCodeFallbackError when elicitation fails for device_code auth.
func (s *addAccountState) authenticateInline(
	ctx context.Context,
	cred azcore.TokenCredential,
	authenticator auth.Authenticator,
	authRecordPath string,
	authMethod string,
	cacheName string,
	clientID string,
	tenantID string,
	label string,
	logger *slog.Logger,
) error {
	if authMethod == "browser" {
		return s.authenticateBrowser(ctx, authenticator, authRecordPath, logger)
	}
	if authMethod == "auth_code" {
		return s.authenticateAuthCode(ctx, authenticator, authRecordPath, label, logger)
	}
	return s.authenticateDeviceCode(ctx, cred, authenticator, authRecordPath, authMethod, cacheName, clientID, tenantID, label, logger)
}

// authenticateBrowser performs browser authentication with URL mode elicitation.
// It presents the Microsoft login URL via elicitation, then runs authentication
// in the background (which opens the system browser). When elicitation fails,
// the browser still opens but the tool waits for auth to complete. If auth
// times out after an elicitation failure, a descriptive error message is
// returned explaining that a browser window was opened.
func (s *addAccountState) authenticateBrowser(
	ctx context.Context,
	authenticator auth.Authenticator,
	authRecordPath string,
	logger *slog.Logger,
) error {
	elicitationID := uuid.New().String()
	loginURL := "https://login.microsoftonline.com"
	message := "Authentication required. A browser window will open for Microsoft login."

	elicitationFailed := false
	_, elicitErr := s.urlElicit(ctx, elicitationID, loginURL, message)
	if elicitErr != nil {
		elicitationFailed = true
		logger.Warn("URL elicitation failed, browser will open silently", "error", elicitErr)
	}

	// Run authentication with a generous timeout for user interaction.
	authCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	_, err := s.authenticate(authCtx, authenticator, authRecordPath, s.scopes)
	if err != nil && elicitationFailed {
		return fmt.Errorf( //nolint:staticcheck // ST1005: user-facing message displayed as tool result text
			"A browser window was opened for Microsoft login but authentication was not completed in time. " +
				"Please try again -- when the browser opens, switch to it and complete the sign-in.")
	}
	return err
}

// authenticateAuthCode performs auth code authentication with form mode
// elicitation. It generates the authorization URL, opens the browser,
// and attempts to elicit the redirect URL from the user. When any
// elicitation error occurs, returns an error with the auth URL and
// complete_auth instructions so the user can complete authentication
// manually.
//
// Parameters:
//   - ctx: the tool handler context containing the MCPServer.
//   - authenticator: the credential's Authenticator, which must also
//     implement auth.AuthCodeFlow.
//   - authRecordPath: filesystem path for persisting the account.
//   - label: the account label, included in fallback error messages.
//   - logger: structured logger for the tool.
//
// Returns nil on success, or an error if the authenticator does not
// implement AuthCodeFlow, URL generation fails, elicitation fails,
// elicitation is declined/cancelled, or the code exchange fails.
func (s *addAccountState) authenticateAuthCode(
	ctx context.Context,
	authenticator auth.Authenticator,
	authRecordPath string,
	label string,
	logger *slog.Logger,
) error {
	acf, ok := authenticator.(auth.AuthCodeFlow)
	if !ok {
		return fmt.Errorf("credential does not support the auth_code flow")
	}

	authURL, err := acf.AuthCodeURL(ctx, s.scopes)
	if err != nil {
		return fmt.Errorf("generate authorization URL: %w", err)
	}

	// Open the authorization URL in the system browser.
	if browserErr := s.openBrowser(authURL); browserErr != nil {
		logger.Warn("failed to open browser for auth code flow", "error", browserErr)
	}

	// Attempt form mode elicitation with a redirect_url text field.
	elicitationRequest := mcp.ElicitationRequest{
		Params: mcp.ElicitationParams{
			Message: "Authentication required. A browser window has opened for Microsoft login. " +
				"After you sign in, the browser will show a blank page. " +
				"Copy the full URL from the browser's address bar and paste it below.",
			RequestedSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"redirect_url": map[string]any{
						"type":        "string",
						"description": "Paste the full URL from the browser's address bar after signing in.",
					},
				},
				"required": []string{"redirect_url"},
			},
		},
	}

	result, elicitErr := s.elicit(ctx, elicitationRequest)
	if elicitErr != nil {
		logger.Info("elicitation failed, returning auth URL for complete_auth tool", "error", elicitErr)
		return fmt.Errorf(
			"elicitation not supported by MCP client. A browser window has opened for Microsoft login. "+
				"After signing in, copy the full URL from the browser's address bar and use the "+
				"complete_auth tool with redirect_url parameter and account label %q to finish authentication",
			label)
	}

	// Handle elicitation response action.
	switch result.Action {
	case mcp.ElicitationResponseActionAccept:
		content, contentOK := result.Content.(map[string]any)
		if !contentOK {
			return fmt.Errorf("unexpected elicitation response content type")
		}
		redirectURL, urlOK := content["redirect_url"].(string)
		if !urlOK || redirectURL == "" {
			return fmt.Errorf("no redirect URL provided in elicitation response")
		}

		if exchangeErr := acf.ExchangeCode(ctx, redirectURL, s.scopes); exchangeErr != nil {
			return fmt.Errorf("exchange authorization code: %w", exchangeErr)
		}

		// Persist account metadata on success.
		if acc, isACC := authenticator.(*auth.AuthCodeCredential); isACC {
			if persistErr := acc.PersistAccount(authRecordPath); persistErr != nil {
				logger.Warn("failed to persist auth code account", "error", persistErr)
			}
		}

		return nil

	case mcp.ElicitationResponseActionDecline:
		return fmt.Errorf("authentication declined by user")

	case mcp.ElicitationResponseActionCancel:
		return fmt.Errorf("authentication cancelled by user")

	default:
		return fmt.Errorf("unexpected elicitation action: %s", result.Action)
	}
}

// DeviceCodeFallbackError is returned when elicitation fails during device code
// authentication. It carries the device code message for the caller to return
// as a successful tool result, so the user sees the device code in the chat.
type DeviceCodeFallbackError struct {
	Message string
}

// Error returns the device code fallback message.
func (e *DeviceCodeFallbackError) Error() string { return e.Message }

// authenticateDeviceCode performs device code authentication with form mode
// elicitation. It starts the device code flow in a background goroutine,
// captures the device code prompt, and presents it via elicitation. When
// elicitation fails, the goroutine is kept alive and the credential state is
// stored as a pending account so the next add_account call can pick it up.
// Returns a DeviceCodeFallbackError containing the device code message in that
// case. The caller converts this to a successful tool result text.
//
// Parameters:
//   - ctx: the tool handler context containing the MCPServer.
//   - cred: the Azure token credential created for this account.
//   - authenticator: the credential's Authenticator for authentication.
//   - authRecordPath: filesystem path for persisting the auth record.
//   - authMethod: the authentication method (always "device_code").
//   - cacheName: the OS keychain cache partition name for this account.
//   - clientID: the OAuth client ID used for this account.
//   - tenantID: the Entra ID tenant ID used for this account.
//   - label: the account label, included in fallback messages.
//   - logger: structured logger for the tool.
//
// Returns nil on success, a *DeviceCodeFallbackError when elicitation fails,
// or a regular error if authentication fails.
func (s *addAccountState) authenticateDeviceCode(
	ctx context.Context,
	cred azcore.TokenCredential,
	authenticator auth.Authenticator,
	authRecordPath string,
	authMethod string,
	cacheName string,
	clientID string,
	tenantID string,
	label string,
	logger *slog.Logger,
) error {
	sendNotification(ctx, mcp.LoggingLevelWarning,
		"Authentication required. Initiating device code login flow...")

	authCtx, cancel := context.WithTimeout(context.Background(), 300*time.Second)

	// Inject channel for capturing the device code message.
	deviceCodeCh := make(chan string, 1)
	authCtx = context.WithValue(authCtx, auth.DeviceCodeMsgKey, deviceCodeCh)

	// Create pending account struct upfront so goroutine can write to p.err.
	// The cancel function is stored so it can be called when the pending
	// account is cleaned up in checkPending, releasing context resources.
	p := &pendingAccount{
		cred:           cred,
		authenticator:  authenticator,
		authRecordPath: authRecordPath,
		cacheName:      cacheName,
		clientID:       clientID,
		tenantID:       tenantID,
		authMethod:     authMethod,
		cancel:         cancel,
		done:           make(chan struct{}),
	}
	go func() {
		defer close(p.done)
		_, p.err = s.authenticate(authCtx, authenticator, authRecordPath, s.scopes)
	}()

	// Wait for the device code prompt, then present it via elicitation.
	select {
	case msg := <-deviceCodeCh:
		logger.Info("device code prompt captured, presenting to client")
		if elicitErr := s.presentDeviceCodeElicitation(ctx, msg, label, logger); elicitErr != nil {
			// Elicitation failed. Keep goroutine alive and store pending state
			// so the next add_account call can pick up the completed auth.
			s.storePending(label, p)
			return elicitErr
		}
		// Elicitation succeeded — wait for auth to complete inline.
		<-p.done
		cancel()
		return p.err

	case <-p.done:
		// Auth completed before device code prompt (e.g., cached token).
		cancel()
		return p.err

	case <-time.After(15 * time.Second):
		cancel()
		return fmt.Errorf("device code prompt was not received within timeout")
	}
}

// presentDeviceCodeElicitation attempts to present the device code prompt
// via form mode elicitation. When elicitation fails, returns a
// *DeviceCodeFallbackError containing the device code and instructions
// for the user to complete the flow and call add_account again.
//
// Parameters:
//   - ctx: the tool handler context containing the MCPServer.
//   - msg: the device code message from the credential callback.
//   - label: the account label, included in fallback instructions.
//   - logger: structured logger for the tool.
//
// Returns nil if elicitation succeeds, or a *DeviceCodeFallbackError if
// elicitation fails.
func (s *addAccountState) presentDeviceCodeElicitation(
	ctx context.Context,
	msg string,
	label string,
	logger *slog.Logger,
) error {
	elicitationRequest := mcp.ElicitationRequest{
		Params: mcp.ElicitationParams{
			Message: msg,
			RequestedSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"acknowledged": map[string]any{
						"type":        "boolean",
						"description": "Check this after completing the device code login in your browser.",
					},
				},
			},
		},
	}

	_, err := s.elicit(ctx, elicitationRequest)
	if err != nil {
		logger.Info("device code elicitation failed, returning device code as tool result", "error", err)
		return &DeviceCodeFallbackError{Message: fmt.Sprintf(
			"Authentication requires a device code. %s\n\n"+
				"After completing the device code flow, call account_add again with label %q "+
				"to finish registration.",
			msg, label)}
	}
	return nil
}

// checkPending checks whether a pending authentication exists for the given
// label. Returns:
//   - (entry, nil): auth completed successfully, entry is ready to register
//   - (nil, result): auth still in progress or failed, result is the tool response
//   - (nil, nil): no pending auth for this label, proceed normally
//
// Parameters:
//   - label: the account label to look up in the pending map.
//   - logger: structured logger for warning on failure.
//
// Side effects: removes the pending entry on completion (success or failure)
// and cancels the authentication context to release resources.
func (s *addAccountState) checkPending(label string, logger *slog.Logger) (*pendingAccount, *mcp.CallToolResult) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()

	p, exists := s.pending[label]
	if !exists {
		return nil, nil
	}

	select {
	case <-p.done:
		// Auth goroutine completed — release context resources.
		delete(s.pending, label)
		if p.cancel != nil {
			p.cancel()
		}
		if p.err != nil {
			logger.Warn("pending authentication failed", "label", label, "error", p.err)
			return nil, mcp.NewToolResultError(fmt.Sprintf(
				"Previous authentication for account %q failed: %s. "+
					"Please try account_add again.", label, p.err))
		}
		return p, nil

	default:
		// Auth still in progress.
		return nil, mcp.NewToolResultText(fmt.Sprintf(
			"Authentication for account %q is still in progress. "+
				"Please complete the device code login in your browser, "+
				"then call account_add again with label %q.", label, label))
	}
}

// storePending stores a pending authentication entry for the given label.
// The caller must ensure the authentication goroutine is running and will
// close p.done when complete.
//
// Parameters:
//   - label: the account label to key the pending entry.
//   - p: the pending account state to store.
//
// Side effects: acquires pendingMu and inserts the entry into the pending map.
func (s *addAccountState) storePending(label string, p *pendingAccount) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	s.pending[label] = p
}

// sendNotification sends a LoggingMessageNotification to the MCP client.
// Falls back to stderr if no MCP server is in context.
func sendNotification(ctx context.Context, level mcp.LoggingLevel, message string) {
	srv := mcpserver.ServerFromContext(ctx)
	if srv != nil {
		notification := mcp.NewLoggingMessageNotification(level, "auth", message)
		if err := srv.SendLogMessageToClient(ctx, notification); err != nil {
			slog.Warn("failed to send notification to client", "error", err)
		}
		return
	}
	fmt.Fprintln(os.Stderr, message) //nolint:errcheck // best-effort stderr fallback
}
