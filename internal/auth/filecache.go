package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"sync"
	"unsafe"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	extcache "github.com/AzureAD/microsoft-authentication-extensions-for-go/cache"
	msalcache "github.com/AzureAD/microsoft-authentication-library-for-go/apps/cache"
)

// cacheImplShim mirrors the internal.impl struct layout from
// github.com/Azure/azure-sdk-for-go/sdk/azidentity/internal. This allows
// constructing an azidentity.Cache without importing the internal package,
// which is restricted to the azidentity module.
//
// IMPORTANT: This struct must exactly match the layout of internal.impl at
// azidentity v1.13.1. If the azidentity module changes the internal Cache
// struct layout, this shim must be updated. The struct layout is verified
// by TestFileCacheShimLayout.
type cacheImplShim struct {
	factory    func(bool) (msalcache.ExportReplace, error)
	cae, noCAE msalcache.ExportReplace
	mu         *sync.RWMutex
}

// cacheShim mirrors the internal.Cache struct layout.
type cacheShim struct {
	impl *cacheImplShim
}

// defaultCacheDir is the directory used for the file-based token cache. It
// matches the directory used for auth records (~/.outlook-local-mcp/).
const defaultCacheDir = ".outlook-local-mcp"

// initFileCacheValue constructs an azidentity.Cache backed by encrypted files.
// The cache encrypts token data at rest using AES-256-GCM with a machine-derived
// key and stores it at ~/.outlook-local-mcp/{name}.bin (and {name}.cae.bin for
// CAE tokens).
//
// This function mirrors the internal.NewCache factory pattern from the Azure SDK
// using unsafe.Pointer to construct the opaque azidentity.Cache struct.
//
// Parameters:
//   - name: the partition name for the cache (used in the filename to isolate
//     different application instances).
//
// Returns the constructed cache, or an error if the home directory cannot be
// determined.
func initFileCacheValue(name string) (azidentity.Cache, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return azidentity.Cache{}, fmt.Errorf("cannot determine home directory: %w", err)
	}

	cacheDir := filepath.Join(home, defaultCacheDir)

	factory := func(cae bool) (msalcache.ExportReplace, error) {
		suffix := ""
		if cae {
			suffix = ".cae"
		}
		cachePath := filepath.Join(cacheDir, name+suffix+".bin")
		lockPath := filepath.Join(cacheDir, name+suffix)

		accessor, accessorErr := newEncryptedFileAccessor(cachePath)
		if accessorErr != nil {
			return nil, accessorErr
		}

		return extcache.New(accessor, lockPath)
	}

	shim := cacheShim{
		impl: &cacheImplShim{
			factory: factory,
			mu:      &sync.RWMutex{},
		},
	}
	cache := *(*azidentity.Cache)(unsafe.Pointer(&shim))
	return cache, nil
}

// initFileMSALCache constructs an MSAL-compatible persistent token cache
// accessor backed by an encrypted file. The cache file is stored at
// ~/.outlook-local-mcp/{name}_msal.bin, encrypted with AES-256-GCM using
// a machine-derived key.
//
// Parameters:
//   - name: the partition name that isolates this application's cached tokens
//     from other applications.
//
// Returns the MSAL-compatible cache accessor implementing ExportReplace, or
// an error if the home directory cannot be determined or the accessor cannot
// be initialized.
func initFileMSALCache(name string) (msalcache.ExportReplace, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	cacheDir := filepath.Join(home, defaultCacheDir)
	cachePath := filepath.Join(cacheDir, name+"_msal.bin")
	lockPath := filepath.Join(cacheDir, name+"_msal")

	accessor, err := newEncryptedFileAccessor(cachePath)
	if err != nil {
		return nil, fmt.Errorf("create encrypted file accessor: %w", err)
	}

	c, err := extcache.New(accessor, lockPath)
	if err != nil {
		return nil, fmt.Errorf("create MSAL cache: %w", err)
	}

	return c, nil
}

// initFileCacheOrWarn calls initFileCacheValue and returns the resulting cache.
// On error, it logs a warning and returns a zero-value azidentity.Cache
// (in-memory only).
//
// Parameters:
//   - name: the partition name for the cache.
//
// Returns a file-backed azidentity.Cache, or a zero-value cache on failure.
func initFileCacheOrWarn(name string) azidentity.Cache {
	c, err := initFileCacheValue(name)
	if err != nil {
		slog.Warn("file-based token cache unavailable, falling back to in-memory cache",
			"error", err)
		return azidentity.Cache{}
	}
	return c
}

// encryptedFileAccessor implements the accessor.Accessor interface using
// AES-256-GCM encryption over a local file. It provides defense-in-depth
// token protection for non-CGo builds where the OS keychain is unavailable.
//
// The encryption key is derived from machine-specific entropy (hostname,
// username, and a stable machine identifier) using SHA-256. This prevents
// casual token theft if the cache file is copied to another machine, but
// is not a security boundary -- the primary protection is file permissions
// (0600).
//
// Thread safety: encryptedFileAccessor is safe for concurrent use via an
// internal sync.RWMutex.
type encryptedFileAccessor struct {
	// path is the absolute filesystem path to the encrypted cache file.
	path string

	// key is the 32-byte AES-256 encryption key derived from machine entropy.
	key [32]byte

	// mu synchronizes concurrent reads and writes.
	mu sync.RWMutex
}

// newEncryptedFileAccessor constructs an encryptedFileAccessor for the given
// file path. The encryption key is derived from machine-specific entropy at
// construction time.
//
// Parameters:
//   - path: absolute filesystem path for the encrypted cache file.
//
// Returns the accessor, or an error if the machine-derived key cannot be
// generated.
func newEncryptedFileAccessor(path string) (*encryptedFileAccessor, error) {
	key, err := deriveMachineKey()
	if err != nil {
		return nil, fmt.Errorf("derive machine key: %w", err)
	}
	return &encryptedFileAccessor{path: path, key: key}, nil
}

// Read returns the decrypted contents of the cache file, or nil if the file
// does not exist. If the file exists but cannot be decrypted (corrupt data,
// key mismatch), the file is removed and nil is returned to allow the cache
// to be rebuilt.
//
// Parameters:
//   - ctx: context for the operation (unused but required by interface).
//
// Returns the decrypted data, or nil with a nil error when the file is absent
// or corrupt.
func (a *encryptedFileAccessor) Read(_ context.Context) ([]byte, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	data, err := os.ReadFile(a.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read cache file: %w", err)
	}

	if len(data) == 0 {
		return nil, nil
	}

	plaintext, err := decryptAESGCM(a.key[:], data)
	if err != nil {
		slog.Warn("token cache corrupted or key mismatch, clearing cache",
			"path", a.path, "error", err)
		// Best-effort removal; ignore errors.
		_ = os.Remove(a.path)
		return nil, nil
	}

	return plaintext, nil
}

// Write encrypts data with AES-256-GCM and writes it to the cache file. The
// parent directory is created with permissions 0700 if it does not exist. The
// file is written with permissions 0600 (owner read/write only).
//
// Parameters:
//   - ctx: context for the operation (unused but required by interface).
//   - data: plaintext data to encrypt and persist.
//
// Returns an error if encryption or file I/O fails.
//
// Side effects: creates directories and writes an encrypted file to disk.
func (a *encryptedFileAccessor) Write(_ context.Context, data []byte) error {
	if len(data) == 0 {
		return nil
	}

	ciphertext, err := encryptAESGCM(a.key[:], data)
	if err != nil {
		return fmt.Errorf("encrypt cache data: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	err = os.WriteFile(a.path, ciphertext, 0600)
	if errors.Is(err, os.ErrNotExist) {
		if mkErr := os.MkdirAll(filepath.Dir(a.path), 0700); mkErr != nil {
			return fmt.Errorf("create cache directory: %w", mkErr)
		}
		err = os.WriteFile(a.path, ciphertext, 0600)
	}
	return err
}

// Delete removes the cache file, if it exists.
//
// Parameters:
//   - ctx: context for the operation (unused but required by interface).
//
// Returns an error if file removal fails for a reason other than the file
// not existing.
func (a *encryptedFileAccessor) Delete(_ context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	err := os.Remove(a.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// deriveMachineKey builds a 32-byte AES-256 key from machine-specific entropy:
// hostname, current username, and a stable machine identifier. The inputs are
// concatenated with null separators and hashed with SHA-256.
//
// This key is deterministic for a given machine and user, meaning the same
// user on the same machine will always derive the same key. If the file is
// copied to a different machine, decryption will fail (defense-in-depth).
//
// Returns the 32-byte key, or an error if system information cannot be
// retrieved.
func deriveMachineKey() ([32]byte, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return [32]byte{}, fmt.Errorf("get hostname: %w", err)
	}

	u, err := user.Current()
	if err != nil {
		return [32]byte{}, fmt.Errorf("get current user: %w", err)
	}

	machineID := stableMachineID()

	// Concatenate with null separator to prevent collisions.
	material := hostname + "\x00" + u.Username + "\x00" + machineID
	return sha256.Sum256([]byte(material)), nil
}

// stableMachineID returns a stable, platform-specific machine identifier.
// On macOS, it reads the IOPlatformUUID via ioreg. On Linux, it reads
// /etc/machine-id. On other platforms or on failure, it falls back to the
// home directory path (which is still user+machine-specific).
func stableMachineID() string {
	// Try /etc/machine-id (Linux standard).
	if data, err := os.ReadFile("/etc/machine-id"); err == nil {
		id := string(data)
		if len(id) > 0 {
			return id
		}
	}

	// Try macOS IOPlatformUUID via the hardware UUID file.
	if data, err := os.ReadFile("/var/db/SystemKey"); err == nil {
		return string(data)
	}

	// Fallback: use home directory path as machine-specific entropy.
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}

	return "fallback-machine-id"
}

// encryptAESGCM encrypts plaintext using AES-256-GCM with a random nonce.
// The returned ciphertext is formatted as: nonce || encrypted_data || tag.
//
// Parameters:
//   - key: 32-byte AES-256 key.
//   - plaintext: data to encrypt.
//
// Returns the nonce-prefixed ciphertext, or an error if encryption fails.
func encryptAESGCM(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// decryptAESGCM decrypts ciphertext produced by encryptAESGCM. It expects
// the format: nonce || encrypted_data || tag.
//
// Parameters:
//   - key: 32-byte AES-256 key (must match the key used for encryption).
//   - ciphertext: the nonce-prefixed ciphertext to decrypt.
//
// Returns the decrypted plaintext, or an error if the data is too short,
// the nonce is invalid, or authentication fails (wrong key or corrupt data).
func decryptAESGCM(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short: %d bytes, need at least %d", len(ciphertext), nonceSize)
	}

	nonce, encrypted := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, encrypted, nil)
}
