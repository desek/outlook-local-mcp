// Package auth account email resolver for multi-account support.
//
// This file provides EnsureEmail, a best-effort helper that lazily fetches
// the authenticated user's email address from the Microsoft Graph /me endpoint
// and caches it on the AccountEntry. Once set, the email is never re-fetched.
package auth

import (
	"context"
	"log/slog"
)

// emailResolveFailedFix is the fix instruction logged when GET /me fails, so an
// operator learns why the account email is empty and how to restore it rather
// than seeing only a degraded account list.
const emailResolveFailedFix = "GET /me failed, so the account email stays empty. Fix: grant the delegated User.Read permission to the client id (the server requests it at sign-in) and sign in to the account again, or use a client id that is pre-authorized for User.Read. Verify: the account domain's list operation shows the email for this account"

// emailMissingFix is the fix instruction logged when GET /me succeeds but
// returns neither mail nor userPrincipalName.
const emailMissingFix = "GET /me returned neither mail nor userPrincipalName, so the account email stays empty. Fix: confirm the signed-in user has a mailbox or a user principal name in the directory, then sign in again. Verify: the account domain's list operation shows the email for this account"

// EnsureEmail lazily fetches the authenticated user's email address from the
// Microsoft Graph API and caches it on entry.Email. If the email is already
// set or the entry has no Graph client, the function returns immediately.
// Failures leave entry.Email empty, so callers still degrade gracefully, but
// each failure is logged at error level with a fix instruction because an
// empty email otherwise hides a missing User.Read grant.
//
// Parameters:
//   - ctx: the context for the Graph API call.
//   - entry: the account entry to populate. Email is set on success.
//
// Side effects: calls GET /me on the Microsoft Graph API on first invocation
// per entry. Uses entry.emailMu to prevent concurrent fetches.
func EnsureEmail(ctx context.Context, entry *AccountEntry) {
	if entry.Client == nil {
		return
	}

	entry.emailMu.Lock()
	defer entry.emailMu.Unlock()

	if entry.Email != "" {
		return
	}

	user, err := entry.Client.Me().Get(ctx, nil)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch account email",
			"label", entry.Label, "error", err, "fix", emailResolveFailedFix)
		return
	}

	if m := user.GetMail(); m != nil && *m != "" {
		entry.Email = *m
		return
	}
	if upn := user.GetUserPrincipalName(); upn != nil && *upn != "" {
		entry.Email = *upn
		return
	}
	slog.ErrorContext(ctx, "account email missing from GET /me",
		"label", entry.Label, "fix", emailMissingFix)
}

// EnsureEmailAndPersistUPN behaves like EnsureEmail but additionally backfills
// the resolved value to the persistent accounts file when the corresponding
// entry's UPN is empty. This is the migration path: accounts that
// existed before the UPN field was introduced have their UPN persisted the
// first time the email is resolved, so subsequent restarts can populate
// AccountEntry.Email without a Graph /me call.
//
// Parameters:
//   - ctx: context for the Graph API call.
//   - entry: the account entry to populate.
//   - accountsPath: filesystem path to accounts.json; when empty, no
//     persistence is attempted (behaves exactly like EnsureEmail).
//
// Side effects: may issue one GET /me call on first invocation per entry,
// and one atomic rewrite of accounts.json when the persisted UPN is empty.
// Persistence failures are logged and otherwise ignored so tool flows remain
// resilient to a read-only or misconfigured accounts file.
func EnsureEmailAndPersistUPN(ctx context.Context, entry *AccountEntry, accountsPath string) {
	EnsureEmail(ctx, entry)

	if accountsPath == "" || entry.Email == "" {
		return
	}

	accounts, err := LoadAccounts(accountsPath)
	if err != nil {
		slog.WarnContext(ctx, "failed to load accounts for UPN backfill",
			"label", entry.Label, "error", err)
		return
	}

	for _, a := range accounts {
		if a.Label == entry.Label {
			if a.UPN != "" {
				return
			}
			if err := UpdateAccountUPN(accountsPath, entry.Label, entry.Email); err != nil {
				slog.WarnContext(ctx, "failed to persist UPN backfill",
					"label", entry.Label, "error", err)
			}
			return
		}
	}
}
