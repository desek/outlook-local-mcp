//go:build !cgo

package auth

import (
	"bytes"
	"log/slog"
	"testing"
	"unsafe"
)

// TestInitCache_NoCgo verifies that InitCache returns a non-zero-value
// azidentity.Cache backed by a file-based encrypted cache.
func TestInitCache_NoCgo(t *testing.T) {
	cache := InitCache("test-nocgo", "auto")

	// A zero-value azidentity.Cache has a nil impl pointer. Our file-based
	// cache should have a non-nil impl.
	shim := *(*cacheShim)(unsafe.Pointer(&cache))
	if shim.impl == nil {
		t.Fatal("InitCache returned zero-value cache; expected file-based cache")
	}

	if shim.impl.factory == nil {
		t.Fatal("cache factory is nil")
	}

	if shim.impl.mu == nil {
		t.Fatal("cache mutex is nil")
	}
}

// TestInitCache_FactoryProducesExportReplace verifies that the factory
// function embedded in the cache produces a non-nil ExportReplace.
func TestInitCache_FactoryProducesExportReplace(t *testing.T) {
	cache := InitCache("test-factory", "auto")
	shim := *(*cacheShim)(unsafe.Pointer(&cache))

	// Test non-CAE path.
	er, err := shim.impl.factory(false)
	if err != nil {
		t.Fatalf("factory(false) error: %v", err)
	}
	if er == nil {
		t.Error("factory(false) returned nil ExportReplace")
	}

	// Test CAE path.
	erCAE, err := shim.impl.factory(true)
	if err != nil {
		t.Fatalf("factory(true) error: %v", err)
	}
	if erCAE == nil {
		t.Error("factory(true) returned nil ExportReplace")
	}
}

// TestInitMSALCache_NoCgo verifies that InitMSALCache returns a non-nil
// ExportReplace backed by a file-based encrypted cache.
func TestInitMSALCache_NoCgo(t *testing.T) {
	er := InitMSALCache("test-msal-nocgo", "auto")
	if er == nil {
		t.Fatal("InitMSALCache returned nil; expected file-based ExportReplace")
	}
}

// TestInitCache_KeychainRequested_WarnsAndUsesFile verifies that when
// storage="keychain" is requested in a non-CGo build, a warning is logged
// and a file-based cache is returned (since keychain is not available
// without CGo).
func TestInitCache_KeychainRequested_WarnsAndUsesFile(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(slog.Default()) })

	cache := InitCache("test-keychain-nocgo", "keychain")

	// Verify we got a file-based cache (non-zero).
	shim := *(*cacheShim)(unsafe.Pointer(&cache))
	if shim.impl == nil {
		t.Fatal("InitCache returned zero-value cache; expected file-based cache")
	}

	// Verify the warning was logged.
	logOutput := buf.String()
	if !bytes.Contains([]byte(logOutput), []byte("CGo is disabled")) {
		t.Errorf("expected warning about CGo being disabled, got: %s", logOutput)
	}
}

// TestInitCache_Auto_UsesFile verifies that when storage="auto" is used in
// a non-CGo build, a file-based cache is returned (since keychain is not
// available without CGo).
func TestInitCache_Auto_UsesFile(t *testing.T) {
	cache := InitCache("test-auto-nocgo", "auto")

	// Verify we got a file-based cache (non-zero).
	shim := *(*cacheShim)(unsafe.Pointer(&cache))
	if shim.impl == nil {
		t.Fatal("InitCache returned zero-value cache; expected file-based cache")
	}

	if shim.impl.factory == nil {
		t.Fatal("cache factory is nil")
	}
}
