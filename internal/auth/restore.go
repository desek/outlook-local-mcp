// Package auth provides account restoration for multi-account configurations.
//
// This file implements startup restoration of persisted accounts from the
// accounts JSON file. Each account is reconstructed using its persisted
// identity configuration (client_id, tenant_id, auth_method), and silent
// token acquisition is attempted to verify the cached credential. Accounts
// that fail silent auth are still registered in the registry for deferred
// re-authentication by the auth middleware.
package auth

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
)

// silentAuthTimeout is the maximum duration for a single account's silent
// token acquisition during startup restoration. This prevents a stalled
// token endpoint from blocking server startup.
const silentAuthTimeout = 5 * time.Second

// CredentialFactory creates a credential and authenticator for an account.
// It encapsulates the setup logic (cache partition, auth record path, SDK
// credential construction) behind a function type so that tests can inject
// mock credentials without triggering real Azure SDK authentication flows.
//
// The signature matches SetupCredentialForAccount exactly, allowing direct
// assignment: var factory CredentialFactory = SetupCredentialForAccount.
//
// Parameters:
//   - label: the unique account label.
//   - clientID: the OAuth client ID.
//   - tenantID: the Azure AD tenant ID.
//   - authMethod: the authentication method (browser, device_code, auth_code).
//   - cacheNameBase: base name for the OS keychain partition.
//   - authRecordDir: directory where per-account auth record files are stored.
//
// Returns the token credential, authenticator, auth record path, cache name,
// or an error if credential construction fails.
type CredentialFactory func(
	label, clientID, tenantID, authMethod, cacheNameBase, authRecordDir string,
) (azcore.TokenCredential, Authenticator, string, string, error)

// GraphClientFactory creates a Graph API client from a token credential.
// This function type allows test injection of a mock factory that returns
// a pre-built client without real Azure SDK interaction.
type GraphClientFactory func(cred azcore.TokenCredential) (*msgraphsdk.GraphServiceClient, error)

// NewDefaultGraphClientFactory returns a GraphClientFactory that creates a
// GraphServiceClient using the provided credential with the given OAuth scopes.
//
// Parameters:
//   - scopes: OAuth scopes to pass to the Graph SDK client (from Scopes(cfg)).
//
// Returns a factory function that creates Graph clients with the specified scopes.
func NewDefaultGraphClientFactory(scopes []string) GraphClientFactory {
	return func(cred azcore.TokenCredential) (*msgraphsdk.GraphServiceClient, error) {
		client, err := msgraphsdk.NewGraphServiceClientWithCredentials(cred, scopes)
		if err != nil {
			return nil, fmt.Errorf("create graph client: %w", err)
		}
		return client, nil
	}
}

// RestoreAccounts loads persisted account configurations and restores them
// into the account registry. For each account, it creates a credential via
// the provided CredentialFactory, attempts silent token acquisition with a
// bounded timeout, creates a Graph client, and registers the account in the
// registry.
//
// Accounts that fail silent auth are still registered with a nil Graph client.
// The auth middleware handles re-authentication on first tool call.
//
// If the accounts file does not exist, the function returns immediately with
// no error and zero restored accounts. Individual account failures are logged
// but do not prevent other accounts from being restored.
//
// Parameters:
//   - accountsPath: filesystem path to the accounts JSON file.
//   - cacheNameBase: base name for the OS keychain partition. Each account
//     appends its label as "{cacheNameBase}-{label}".
//   - authRecordDir: directory where per-account auth record files are stored.
//   - registry: the account registry to populate with restored accounts.
//   - credFactory: factory function to create credentials and authenticators.
//   - clientFactory: factory function to create Graph clients from credentials.
//   - scopes: OAuth scopes to use for silent token acquisition (from Scopes(cfg)).
//
// Returns the number of successfully restored accounts (with active tokens)
// and the total number of accounts loaded from the file.
func RestoreAccounts(
	accountsPath string,
	cacheNameBase string,
	authRecordDir string,
	registry *AccountRegistry,
	credFactory CredentialFactory,
	clientFactory GraphClientFactory,
	scopes []string,
) (restored int, total int) {
	accounts, err := LoadAccounts(accountsPath)
	if err != nil {
		slog.Warn("failed to load accounts file", "path", accountsPath, "error", err)
		return 0, 0
	}

	total = len(accounts)
	if total == 0 {
		return 0, 0
	}

	slog.Info("restoring accounts from accounts file", "path", accountsPath, "count", total)

	for _, acct := range accounts {
		if restoreOne(acct, cacheNameBase, authRecordDir, registry, credFactory, clientFactory, scopes) {
			restored++
		}
	}

	slog.Info("account restoration complete", "restored", restored, "total", total)
	return restored, total
}

// restoreOne restores a single account from its persisted configuration.
// It creates the credential via the provided CredentialFactory, attempts
// silent token acquisition (except for device_code accounts), creates a
// Graph client on success, and registers the account in the registry.
//
// For device_code accounts, silent token acquisition is skipped entirely
// because DeviceCodeCredential.GetToken triggers the device code callback
// (printing to stderr) when no cached token exists, causing spurious output
// and startup delays. These accounts are registered with Client=nil and
// Authenticated=false, deferring re-authentication to the first tool call.
//
// Parameters:
//   - acct: the persisted account configuration.
//   - cacheNameBase: base name for the OS keychain partition.
//   - authRecordDir: directory where per-account auth record files are stored.
//   - registry: the account registry to populate.
//   - credFactory: factory function to create credentials and authenticators.
//   - clientFactory: factory function to create Graph clients.
//   - scopes: OAuth scopes to use for silent token acquisition (from Scopes(cfg)).
//
// Returns true if the account was restored with an active token and Graph
// client, false if the account was registered but needs re-authentication.
func restoreOne(
	acct AccountConfig,
	cacheNameBase string,
	authRecordDir string,
	registry *AccountRegistry,
	credFactory CredentialFactory,
	clientFactory GraphClientFactory,
	scopes []string,
) bool {
	logger := slog.With("account", acct.Label)

	cred, authenticator, authRecordPath, cacheName, err := credFactory(
		acct.Label, acct.ClientID, acct.TenantID, acct.AuthMethod,
		cacheNameBase, authRecordDir,
	)
	if err != nil {
		logger.Warn("failed to create credential for account, skipping",
			"error", err)
		return false
	}

	entry := &AccountEntry{
		Label:          acct.Label,
		ClientID:       acct.ClientID,
		TenantID:       acct.TenantID,
		AuthMethod:     acct.AuthMethod,
		Credential:     cred,
		Authenticator:  authenticator,
		AuthRecordPath: authRecordPath,
		CacheName:      cacheName,
	}

	// Skip silent token acquisition for device_code accounts. Without a
	// cached token, DeviceCodeCredential.GetToken triggers the device code
	// callback (printing to stderr) and then times out — wasting 5 seconds
	// per account and producing spurious output that confuses Claude Desktop.
	// These accounts are registered with Client=nil and Authenticated=false;
	// the auth middleware handles re-authentication on first tool call.
	if acct.AuthMethod != "device_code" {
		// Attempt silent token acquisition with a bounded timeout.
		ctx, cancel := context.WithTimeout(context.Background(), silentAuthTimeout)
		defer cancel()

		_, tokenErr := cred.GetToken(ctx, policy.TokenRequestOptions{
			Scopes: scopes,
		})

		if tokenErr == nil {
			// Silent auth succeeded — create Graph client.
			client, clientErr := clientFactory(cred)
			if clientErr != nil {
				logger.Warn("silent auth succeeded but graph client creation failed",
					"error", clientErr)
			} else {
				entry.Client = client
				entry.Authenticated = true
			}
		} else {
			logger.Info("silent auth failed, account will need re-authentication on first use",
				"error", tokenErr)
		}
	} else {
		logger.Info("device_code account skipped silent auth, re-authentication deferred to first use")
	}

	if regErr := registry.Add(entry); regErr != nil {
		logger.Warn("failed to register restored account", "error", regErr)
		return false
	}

	if entry.Client != nil {
		logger.Info("account restored with active token")
		return true
	}

	logger.Info("account restored, re-authentication required on first use")
	return false
}

// AuthRecordDir extracts the directory path from an auth record file path.
// This is used to derive the per-account auth record directory from the
// server config's AuthRecordPath.
//
// Parameters:
//   - authRecordPath: the full filesystem path to the auth record file.
//
// Returns the directory component of the path.
func AuthRecordDir(authRecordPath string) string {
	return filepath.Dir(authRecordPath)
}
