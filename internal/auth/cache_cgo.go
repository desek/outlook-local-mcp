//go:build cgo

package auth

import (
	"log/slog"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity/cache"
	extcache "github.com/AzureAD/microsoft-authentication-extensions-for-go/cache"
	"github.com/AzureAD/microsoft-authentication-extensions-for-go/cache/accessor"
	msalcache "github.com/AzureAD/microsoft-authentication-library-for-go/apps/cache"
)

// InitCache initializes a persistent token cache identified by name, using
// the storage backend specified by the storage parameter.
//
// Storage behavior:
//   - "file": uses file-based AES-256-GCM encrypted cache directly, without
//     attempting the OS keychain.
//   - "auto": attempts OS keychain first; falls back to file-based cache if
//     the keychain is unavailable at runtime.
//   - "keychain": attempts OS keychain only. If unavailable, returns a
//     zero-value cache (in-memory) and logs an error — no file-based fallback.
//
// Parameters:
//   - name: the partition name that isolates this application's cached tokens
//     from other applications.
//   - storage: the token storage backend preference ("auto", "keychain", or
//     "file").
//
// Returns an azidentity.Cache backed by the selected storage, or a zero-value
// azidentity.Cache when storage fails (in-memory only).
func InitCache(name, storage string) azidentity.Cache {
	if storage == "file" {
		slog.Info("token storage explicitly set to file-based", "name", name)
		return initFileCacheOrWarn(name)
	}

	c, err := cache.New(&cache.Options{Name: name})
	if err != nil {
		if storage == "keychain" {
			slog.Error("OS keychain unavailable and token_storage=keychain",
				"error", err)
			return azidentity.Cache{}
		}
		// storage == "auto": fall back to file-based.
		slog.Warn("OS keychain unavailable, falling back to file-based cache",
			"error", err)
		return initFileCacheOrWarn(name)
	}

	slog.Info("persistent token cache initialized (OS keychain)", "name", name)
	return c
}

// InitMSALCache creates an MSAL-compatible persistent token cache accessor
// using the storage backend specified by the storage parameter.
//
// Storage behavior:
//   - "file": uses file-based AES-256-GCM encrypted cache directly, without
//     attempting the OS keychain.
//   - "auto": attempts OS keychain first; falls back to file-based cache if
//     the keychain is unavailable at runtime.
//   - "keychain": attempts OS keychain only. If unavailable, returns nil and
//     logs an error — no file-based fallback.
//
// Parameters:
//   - name: the partition name that isolates this application's cached tokens
//     from other applications. On macOS and Linux this is a service/label name;
//     on Windows it is the path to a storage file.
//   - storage: the token storage backend preference ("auto", "keychain", or
//     "file").
//
// Returns the MSAL-compatible cache accessor, or nil when the selected storage
// is unavailable. A nil return causes the credential to fall back to MSAL's
// built-in in-memory cache.
func InitMSALCache(name, storage string) msalcache.ExportReplace {
	if storage == "file" {
		slog.Info("MSAL token storage explicitly set to file-based", "name", name)
		c, err := initFileMSALCache(name)
		if err != nil {
			slog.Warn("MSAL file-based cache unavailable, falling back to in-memory cache",
				"error", err)
			return nil
		}
		return c
	}

	store, err := accessor.New(name)
	if err != nil {
		if storage == "keychain" {
			slog.Error("MSAL OS keychain unavailable and token_storage=keychain",
				"error", err)
			return nil
		}
		// storage == "auto": fall back to file-based.
		slog.Warn("MSAL OS keychain unavailable, falling back to file-based cache",
			"error", err)
		c, fileErr := initFileMSALCache(name)
		if fileErr != nil {
			slog.Warn("MSAL file-based cache also unavailable, falling back to in-memory cache",
				"error", fileErr)
			return nil
		}
		return c
	}

	c, err := extcache.New(store, name)
	if err != nil {
		if storage == "keychain" {
			slog.Error("MSAL OS keychain cache creation failed and token_storage=keychain",
				"error", err)
			return nil
		}
		// storage == "auto": fall back to file-based.
		slog.Warn("MSAL OS keychain cache creation failed, falling back to file-based cache",
			"error", err)
		fileC, fileErr := initFileMSALCache(name)
		if fileErr != nil {
			slog.Warn("MSAL file-based cache also unavailable, falling back to in-memory cache",
				"error", fileErr)
			return nil
		}
		return fileC
	}

	slog.Info("MSAL persistent cache initialized (OS keychain)", "name", name)
	return c
}
