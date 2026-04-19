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

// EnsureEmail lazily fetches the authenticated user's email address from the
// Microsoft Graph API and caches it on entry.Email. If the email is already
// set or the entry has no Graph client, the function returns immediately.
// Failures are logged and silently ignored — callers should tolerate an empty
// email and degrade gracefully.
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
		slog.WarnContext(ctx, "failed to fetch account email", "label", entry.Label, "error", err)
		return
	}

	if m := user.GetMail(); m != nil && *m != "" {
		entry.Email = *m
		return
	}
	if upn := user.GetUserPrincipalName(); upn != nil && *upn != "" {
		entry.Email = *upn
	}
}

// EnsureEmailAndPersistUPN behaves like EnsureEmail but additionally backfills
// the resolved value to the persistent accounts file when the corresponding
// entry's UPN is empty. This is the CR-0056 migration path: accounts that
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
