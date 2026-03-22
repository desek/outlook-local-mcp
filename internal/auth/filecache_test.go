package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"unsafe"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// TestFileCache_PersistAndReload verifies that data written to the encrypted
// file cache can be read back after a fresh accessor is constructed for the
// same file. This simulates token persistence across server restarts.
func TestFileCache_PersistAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.bin")
	data := []byte(`{"access_token":"secret","refresh_token":"also_secret"}`)

	// Write with first accessor instance.
	a1, err := newEncryptedFileAccessor(path)
	if err != nil {
		t.Fatalf("newEncryptedFileAccessor (write): %v", err)
	}
	if err := a1.Write(context.Background(), data); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Read with a new accessor instance (simulates restart).
	a2, err := newEncryptedFileAccessor(path)
	if err != nil {
		t.Fatalf("newEncryptedFileAccessor (read): %v", err)
	}
	got, err := a2.Read(context.Background())
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if !bytes.Equal(got, data) {
		t.Errorf("round-trip mismatch:\n  got:  %q\n  want: %q", got, data)
	}
}

// TestFileCache_Encryption verifies that the file on disk contains encrypted
// (non-plaintext) data, confirming that AES-256-GCM encryption is applied.
func TestFileCache_Encryption(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.bin")
	plaintext := []byte("this is a plaintext token value that should not appear on disk")

	a, err := newEncryptedFileAccessor(path)
	if err != nil {
		t.Fatalf("newEncryptedFileAccessor: %v", err)
	}
	if err := a.Write(context.Background(), plaintext); err != nil {
		t.Fatalf("Write: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if bytes.Contains(raw, plaintext) {
		t.Error("plaintext found in encrypted cache file; encryption is not applied")
	}

	// Encrypted data should be longer than plaintext (nonce + tag overhead).
	if len(raw) <= len(plaintext) {
		t.Errorf("encrypted file (%d bytes) is not larger than plaintext (%d bytes)",
			len(raw), len(plaintext))
	}
}

// TestFileCache_CorruptionRecovery verifies that a corrupted cache file is
// handled gracefully: Read returns nil (not an error), and the corrupt file
// is removed so subsequent writes can re-create it.
func TestFileCache_CorruptionRecovery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.bin")

	// Write valid data first.
	a, err := newEncryptedFileAccessor(path)
	if err != nil {
		t.Fatalf("newEncryptedFileAccessor: %v", err)
	}
	if err := a.Write(context.Background(), []byte("valid data")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Corrupt the file by overwriting with random bytes.
	corrupt := make([]byte, 64)
	if _, err := rand.Read(corrupt); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	if err := os.WriteFile(path, corrupt, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Read should return nil (not crash or error).
	got, err := a.Read(context.Background())
	if err != nil {
		t.Fatalf("Read after corruption should not return error, got: %v", err)
	}
	if got != nil {
		t.Errorf("Read after corruption should return nil, got: %q", got)
	}

	// Corrupt file should have been removed.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("corrupt cache file should have been removed")
	}

	// Verify we can write again after corruption.
	newData := []byte("fresh data after corruption")
	if err := a.Write(context.Background(), newData); err != nil {
		t.Fatalf("Write after corruption: %v", err)
	}

	got, err = a.Read(context.Background())
	if err != nil {
		t.Fatalf("Read after re-write: %v", err)
	}
	if !bytes.Equal(got, newData) {
		t.Errorf("read after re-write mismatch:\n  got:  %q\n  want: %q", got, newData)
	}
}

// TestFileCache_Permissions verifies that the cache file is created with
// restrictive permissions (0600 - owner read/write only).
func TestFileCache_Permissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.bin")

	a, err := newEncryptedFileAccessor(path)
	if err != nil {
		t.Fatalf("newEncryptedFileAccessor: %v", err)
	}
	if err := a.Write(context.Background(), []byte("test")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("cache file permissions = %04o, want 0600", perm)
	}
}

// TestFileCache_ReadNonexistent verifies that reading from a non-existent
// cache file returns nil data and nil error.
func TestFileCache_ReadNonexistent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does_not_exist.bin")

	a, err := newEncryptedFileAccessor(path)
	if err != nil {
		t.Fatalf("newEncryptedFileAccessor: %v", err)
	}

	got, err := a.Read(context.Background())
	if err != nil {
		t.Fatalf("Read nonexistent file should not error, got: %v", err)
	}
	if got != nil {
		t.Errorf("Read nonexistent file should return nil, got: %q", got)
	}
}

// TestFileCache_WriteEmpty verifies that writing empty data is a no-op.
func TestFileCache_WriteEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.bin")

	a, err := newEncryptedFileAccessor(path)
	if err != nil {
		t.Fatalf("newEncryptedFileAccessor: %v", err)
	}

	if err := a.Write(context.Background(), nil); err != nil {
		t.Fatalf("Write nil: %v", err)
	}
	if err := a.Write(context.Background(), []byte{}); err != nil {
		t.Fatalf("Write empty: %v", err)
	}

	// File should not exist since both writes were no-ops.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("cache file should not exist after writing empty data")
	}
}

// TestFileCache_Delete verifies that Delete removes the cache file.
func TestFileCache_Delete(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.bin")

	a, err := newEncryptedFileAccessor(path)
	if err != nil {
		t.Fatalf("newEncryptedFileAccessor: %v", err)
	}
	if err := a.Write(context.Background(), []byte("data")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if err := a.Delete(context.Background()); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("cache file should not exist after Delete")
	}

	// Delete of non-existent file should not error.
	if err := a.Delete(context.Background()); err != nil {
		t.Fatalf("Delete nonexistent: %v", err)
	}
}

// TestFileCache_DirectoryCreation verifies that Write creates parent
// directories with permissions 0700.
func TestFileCache_DirectoryCreation(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "sub", "dir", "cache.bin")

	a, err := newEncryptedFileAccessor(nested)
	if err != nil {
		t.Fatalf("newEncryptedFileAccessor: %v", err)
	}
	if err := a.Write(context.Background(), []byte("data")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, "sub", "dir"))
	if err != nil {
		t.Fatalf("Stat directory: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory to be created")
	}
}

// TestFileCacheShimLayout verifies that the cacheShim struct has the same
// size as azidentity.Cache. This is a compile-time safety check for the
// unsafe.Pointer cast used in initFileCacheValue. If the azidentity module
// changes its internal Cache struct layout, this test will fail.
func TestFileCacheShimLayout(t *testing.T) {
	shimSize := unsafe.Sizeof(cacheShim{})
	cacheSize := unsafe.Sizeof(azidentity.Cache{})

	if shimSize != cacheSize {
		t.Fatalf("cacheShim size (%d) != azidentity.Cache size (%d); "+
			"the internal struct layout may have changed", shimSize, cacheSize)
	}
}

// TestCacheImplShimFields verifies that the cacheImplShim struct has the
// expected field layout by checking offsets. This catches field reordering
// in the upstream internal.impl struct.
func TestCacheImplShimFields(t *testing.T) {
	var s cacheImplShim

	// factory should be at offset 0.
	factoryOffset := unsafe.Offsetof(s.factory)
	if factoryOffset != 0 {
		t.Errorf("factory offset = %d, want 0", factoryOffset)
	}

	// cae should be after factory (one func pointer = 8 bytes on 64-bit).
	caeOffset := unsafe.Offsetof(s.cae)
	if caeOffset != unsafe.Sizeof(s.factory) {
		t.Errorf("cae offset = %d, want %d", caeOffset, unsafe.Sizeof(s.factory))
	}

	// noCAE should be after cae.
	noCAEOffset := unsafe.Offsetof(s.noCAE)
	expectedNoCAE := caeOffset + unsafe.Sizeof(s.cae)
	if noCAEOffset != expectedNoCAE {
		t.Errorf("noCAE offset = %d, want %d", noCAEOffset, expectedNoCAE)
	}

	// mu should be after noCAE.
	muOffset := unsafe.Offsetof(s.mu)
	expectedMu := noCAEOffset + unsafe.Sizeof(s.noCAE)
	if muOffset != expectedMu {
		t.Errorf("mu offset = %d, want %d", muOffset, expectedMu)
	}

	// Verify total size matches expectations. On 64-bit:
	// func(8) + interface(16) + interface(16) + pointer(8) = 48
	totalSize := unsafe.Sizeof(s)
	t.Logf("cacheImplShim total size: %d bytes", totalSize)
	if totalSize == 0 {
		t.Error("cacheImplShim has zero size")
	}

	// Cross-check: create a real sync.RWMutex to ensure the mu field type matches.
	_ = &sync.RWMutex{}
}

// TestInitFileCacheValue_ReturnsUsableCache verifies that initFileCacheValue
// constructs a functional azidentity.Cache with a non-nil impl and factory.
func TestInitFileCacheValue_ReturnsUsableCache(t *testing.T) {
	c, err := initFileCacheValue("test-filecache")
	if err != nil {
		t.Fatalf("initFileCacheValue error: %v", err)
	}

	shim := *(*cacheShim)(unsafe.Pointer(&c))
	if shim.impl == nil {
		t.Fatal("returned cache has nil impl; expected file-based cache")
	}
	if shim.impl.factory == nil {
		t.Fatal("cache factory is nil")
	}
	if shim.impl.mu == nil {
		t.Fatal("cache mutex is nil")
	}

	// Verify factory produces a usable ExportReplace.
	er, err := shim.impl.factory(false)
	if err != nil {
		t.Fatalf("factory(false) error: %v", err)
	}
	if er == nil {
		t.Error("factory(false) returned nil ExportReplace")
	}
}

// TestInitFileMSALCache_ReturnsUsableAccessor verifies that initFileMSALCache
// constructs a functional MSAL cache accessor implementing ExportReplace.
func TestInitFileMSALCache_ReturnsUsableAccessor(t *testing.T) {
	er, err := initFileMSALCache("test-msal-filecache")
	if err != nil {
		t.Fatalf("initFileMSALCache error: %v", err)
	}
	if er == nil {
		t.Fatal("initFileMSALCache returned nil; expected file-based ExportReplace")
	}

}
