//go:build !cgo

package auth

import (
	"log/slog"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	msalcache "github.com/AzureAD/microsoft-authentication-library-for-go/apps/cache"
)

// InitCache returns a persistent file-based azidentity.Cache when CGo is
// disabled. The cache encrypts token data at rest using AES-256-GCM with a
// machine-derived key and stores it at ~/.outlook-local-mcp/{name}.bin.
//
// Storage behavior:
//   - "keychain": logs a warning that keychain requires CGo, then uses
//     file-based storage as fallback.
//   - "auto" or "file": uses file-based storage directly.
//
// If the encrypted file accessor cannot be initialized, it falls back to a
// zero-value azidentity.Cache (in-memory only) and logs a warning.
//
// Parameters:
//   - name: the partition name for the cache (used in the filename to
//     isolate different application instances).
//   - storage: the token storage backend preference ("auto", "keychain", or
//     "file").
func InitCache(name, storage string) azidentity.Cache {
	if storage == "keychain" {
		slog.Warn("token_storage=keychain requested but CGo is disabled; "+
			"falling back to file-based cache", "name", name)
	}

	c, err := initFileCacheValue(name)
	if err != nil {
		slog.Warn("file-based token cache unavailable, falling back to in-memory cache",
			"error", err)
		return azidentity.Cache{}
	}

	slog.Info("file-based persistent token cache initialized (CGo disabled)",
		"name", name)
	return c
}

// InitMSALCache creates an MSAL-compatible persistent token cache accessor
// backed by an encrypted file when CGo is disabled. The cache file is stored
// at ~/.outlook-local-mcp/{name}_msal.bin, encrypted with AES-256-GCM using
// a machine-derived key.
//
// Storage behavior:
//   - "keychain": logs a warning that keychain requires CGo, then uses
//     file-based storage as fallback.
//   - "auto" or "file": uses file-based storage directly.
//
// Parameters:
//   - name: the partition name that isolates this application's cached tokens
//     from other applications.
//   - storage: the token storage backend preference ("auto", "keychain", or
//     "file").
//
// Returns the MSAL-compatible cache accessor implementing ExportReplace, or
// nil when the encrypted file accessor cannot be initialized. A nil return
// causes the credential to fall back to MSAL's built-in in-memory cache.
func InitMSALCache(name, storage string) msalcache.ExportReplace {
	if storage == "keychain" {
		slog.Warn("MSAL token_storage=keychain requested but CGo is disabled; "+
			"falling back to file-based cache", "name", name)
	}

	c, err := initFileMSALCache(name)
	if err != nil {
		slog.Warn("MSAL file-based cache unavailable, falling back to in-memory cache",
			"error", err)
		return nil
	}

	slog.Info("MSAL file-based persistent cache initialized (CGo disabled)",
		"name", name)
	return c
}
