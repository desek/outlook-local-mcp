package auth

import "sync/atomic"

// activeBackend holds the name of the token cache storage backend selected
// during the most recent InitCache call. Values are "keychain" or "file".
// The zero value is "file" so that callers always receive a valid string even
// before InitCache has run.
var activeBackend atomic.Value

func init() {
	activeBackend.Store("file")
}

// ActiveBackend returns the name of the currently-active token cache storage
// backend. The value is set by InitCache at server startup and does not change
// during the lifetime of the process.
//
// Possible return values:
//   - "keychain" – tokens are stored in the OS keychain (macOS Keychain
//     Services / Windows Credential Manager / Linux Secret Service).
//   - "file" – tokens are stored in an AES-256-GCM encrypted file under
//     ~/.outlook-local-mcp/.
//
// ActiveBackend is safe to call from any goroutine without synchronisation.
func ActiveBackend() string {
	v, _ := activeBackend.Load().(string)
	if v == "" {
		return "file"
	}
	return v
}

// setActiveBackend records the selected backend. Called by InitCache
// implementations once the backend choice is resolved.
func setActiveBackend(backend string) {
	activeBackend.Store(backend)
}
