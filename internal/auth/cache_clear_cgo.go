//go:build cgo

package auth

import (
	"context"
	"log/slog"

	"github.com/AzureAD/microsoft-authentication-extensions-for-go/cache/accessor"
)

// clearKeychainTokenCache attempts to delete the OS keychain token cache entry
// for the given partition name. Failures are logged at debug level and
// otherwise ignored: the cache may legitimately not exist (never written, or
// already cleared), and the logout flow continues regardless.
//
// Parameters:
//   - name: the cache partition name (AccountEntry.CacheName).
func clearKeychainTokenCache(name string) {
	store, err := accessor.New(name)
	if err != nil {
		slog.Debug("keychain accessor unavailable during logout", "name", name, "error", err)
		return
	}
	if err := store.Delete(context.Background()); err != nil {
		slog.Debug("keychain token cache delete failed", "name", name, "error", err)
	}
}
