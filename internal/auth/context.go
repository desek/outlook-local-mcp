// Package auth context helpers for multi-account support.
//
// This file defines context key types and helper functions for storing and
// retrieving per-request Graph clients and account authentication details.
// These are used by the AccountResolver middleware to inject the resolved
// account's client into the request context, and by tool handlers and the
// AuthMiddleware to retrieve them.
package auth

import (
	"context"

	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
)

// graphClientKeyType is the unexported context key type for storing a
// *msgraphsdk.GraphServiceClient in context. Using an unexported struct
// type prevents collisions with other packages' context keys.
type graphClientKeyType struct{}

// graphClientKey is the package-level context key for Graph client storage.
var graphClientKey = graphClientKeyType{}

// WithGraphClient returns a new context with the given Graph client stored
// under graphClientKey. The AccountResolver middleware calls this to inject
// the resolved account's client into the request context.
//
// Parameters:
//   - ctx: the parent context.
//   - client: the Graph client for the resolved account.
//
// Returns a derived context containing the client.
func WithGraphClient(ctx context.Context, client *msgraphsdk.GraphServiceClient) context.Context {
	return context.WithValue(ctx, graphClientKey, client)
}

// GraphClientFromContext retrieves the Graph client previously stored via
// WithGraphClient. Tool handlers call this to obtain the per-request client.
//
// Parameters:
//   - ctx: the context to retrieve the client from.
//
// Returns the client and true if present, or nil and false if not.
func GraphClientFromContext(ctx context.Context) (*msgraphsdk.GraphServiceClient, bool) {
	if ctx == nil {
		return nil, false
	}
	client, ok := ctx.Value(graphClientKey).(*msgraphsdk.GraphServiceClient)
	return client, ok
}

// accountAuthKeyType is the unexported context key type for storing an
// AccountAuth value in context.
type accountAuthKeyType struct{}

// accountAuthKey is the package-level context key for account auth storage.
var accountAuthKey = accountAuthKeyType{}

// AccountAuth holds the authentication details for a resolved account. The
// AuthMiddleware uses these fields to perform per-account re-authentication
// when a tool call encounters an auth error.
type AccountAuth struct {
	// Authenticator is the credential's Authenticator interface for triggering
	// interactive re-authentication.
	Authenticator Authenticator

	// AuthRecordPath is the filesystem path for persisting this account's
	// AuthenticationRecord.
	AuthRecordPath string

	// AuthMethod is the authentication method ("browser" or "device_code")
	// that controls the re-authentication flow.
	AuthMethod string
}

// WithAccountAuth returns a new context with the given AccountAuth stored
// under accountAuthKey. The AccountResolver middleware calls this to inject
// the resolved account's authentication details into the request context.
//
// Parameters:
//   - ctx: the parent context.
//   - auth: the account authentication details.
//
// Returns a derived context containing the AccountAuth.
func WithAccountAuth(ctx context.Context, auth AccountAuth) context.Context {
	return context.WithValue(ctx, accountAuthKey, auth)
}

// AccountAuthFromContext retrieves the AccountAuth previously stored via
// WithAccountAuth. The AuthMiddleware calls this to obtain per-account
// credentials for re-authentication.
//
// Parameters:
//   - ctx: the context to retrieve the AccountAuth from.
//
// Returns the AccountAuth and true if present, or a zero-value AccountAuth
// and false if not.
func AccountAuthFromContext(ctx context.Context) (AccountAuth, bool) {
	if ctx == nil {
		return AccountAuth{}, false
	}
	auth, ok := ctx.Value(accountAuthKey).(AccountAuth)
	return auth, ok
}

// AccountInfo holds the label and email of the account resolved for a request.
// Email may be empty if the /me fetch has not yet completed for this account.
type AccountInfo struct {
	// Label is the unique account identifier (e.g., "default", "work").
	Label string

	// Email is the authenticated user's email address fetched from /me.
	// Empty when EnsureEmail has not yet run successfully for this account.
	Email string
}

// accountInfoKeyType is the unexported context key type for AccountInfo storage.
type accountInfoKeyType struct{}

// accountInfoKey is the package-level context key for AccountInfo storage.
var accountInfoKey = accountInfoKeyType{}

// WithAccountInfo returns a new context with the given AccountInfo stored
// under accountInfoKey. The AccountResolver middleware calls this after
// resolving the account to make label and email available to tool handlers.
//
// Parameters:
//   - ctx: the parent context.
//   - info: the resolved account label and email.
//
// Returns a derived context containing the AccountInfo.
func WithAccountInfo(ctx context.Context, info AccountInfo) context.Context {
	return context.WithValue(ctx, accountInfoKey, info)
}

// AccountInfoFromContext retrieves the AccountInfo previously stored via
// WithAccountInfo. Tool handlers call this to obtain the account label and
// email for inclusion in response text.
//
// Parameters:
//   - ctx: the context to retrieve the AccountInfo from.
//
// Returns the AccountInfo and true if present, or a zero-value AccountInfo
// and false if not.
func AccountInfoFromContext(ctx context.Context) (AccountInfo, bool) {
	if ctx == nil {
		return AccountInfo{}, false
	}
	info, ok := ctx.Value(accountInfoKey).(AccountInfo)
	return info, ok
}
