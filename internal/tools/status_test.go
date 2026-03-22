package tools_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/config"
	"github.com/desek/outlook-local-mcp/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
)

// testConfig returns a config.Config populated with known values suitable for
// status tool tests. Tests that need specific field values should override
// fields on the returned struct.
func testConfig() config.Config {
	return config.Config{
		Version:           "1.2.3",
		DefaultTimezone:   "Europe/Stockholm",
		ClientID:          "dd5fc5c5-eb9a-4f6f-97bd-1a9fecb277d3",
		TenantID:          "common",
		AuthMethod:        "device_code",
		AuthMethodSource:  "inferred",
		LogLevel:          "warn",
		LogFormat:         "json",
		LogFile:           "",
		LogSanitize:       true,
		AuditLogEnabled:   true,
		AuditLogPath:      "",
		TokenStorage:      "auto",
		TokenCacheBackend: "keychain",
		AuthRecordPath:    "/home/user/.outlook-local-mcp/auth_record.json",
		AccountsPath:      "/home/user/.outlook-local-mcp/accounts.json",
		CacheName:         "outlook-local-mcp",
		MaxRetries:        3,
		RetryBackoffMS:    1000,
		RequestTimeout:    30 * time.Second,
		ShutdownTimeout:   15 * time.Second,
		ReadOnly:          false,
		MailEnabled:       false,
		ProvenanceTag:     "com.github.desek.outlook-local-mcp.created",
		OTELEnabled:       false,
		OTELEndpoint:      "",
		OTELServiceName:   "outlook-local-mcp",
	}
}

// callStatus invokes the status handler with output=summary and returns the
// unmarshalled JSON response. The summary mode is used because the default
// output mode is now "text" (plain-text), which cannot be parsed as JSON.
func callStatus(t *testing.T, cfg config.Config, registry *auth.AccountRegistry, startTime time.Time) map[string]any {
	t.Helper()
	handler := tools.HandleStatus(cfg, registry, startTime)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"output": "summary"}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected success result, got error")
	}
	if len(result.Content) == 0 {
		t.Fatal("expected non-empty content")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	var resp map[string]any
	if err := json.Unmarshal([]byte(tc.Text), &resp); err != nil {
		t.Fatalf("failed to unmarshal status response: %v", err)
	}
	return resp
}

// TestStatus_ReturnsHealthSummary verifies that the status tool returns a JSON
// object containing version, timezone, accounts array, and uptime.
func TestStatus_ReturnsHealthSummary(t *testing.T) {
	registry := auth.NewAccountRegistry()
	_ = registry.Add(&auth.AccountEntry{Label: "default", Authenticated: true})
	_ = registry.Add(&auth.AccountEntry{Label: "work", Authenticated: false})

	cfg := testConfig()
	startTime := time.Now().Add(-1 * time.Hour)
	resp := callStatus(t, cfg, registry, startTime)

	if resp["version"] != "1.2.3" {
		t.Errorf("expected version %q, got %v", "1.2.3", resp["version"])
	}
	if resp["timezone"] != "Europe/Stockholm" {
		t.Errorf("expected timezone %q, got %v", "Europe/Stockholm", resp["timezone"])
	}
	accounts, ok := resp["accounts"].([]any)
	if !ok || len(accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %v", resp["accounts"])
	}

	// Uptime should be approximately 3600 seconds (1 hour).
	uptime, _ := resp["server_uptime_seconds"].(float64)
	if uptime < 3590 || uptime > 3610 {
		t.Errorf("expected uptime ~3600s, got %v", uptime)
	}
}

// TestStatus_NoGraphAPICalls verifies that the status tool completes without
// network access. The test uses a registry with nil Graph clients, which would
// panic if any Graph API call were attempted.
func TestStatus_NoGraphAPICalls(t *testing.T) {
	registry := auth.NewAccountRegistry()
	_ = registry.Add(&auth.AccountEntry{Label: "default", Authenticated: true})

	cfg := testConfig()
	handler := tools.HandleStatus(cfg, registry, time.Now())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result, err := handler(ctx, mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected success result, got error")
	}
	if ctx.Err() != nil {
		t.Errorf("context expired, tool took too long: %v", ctx.Err())
	}
}

// TestStatus_ConfigPresent verifies that the response contains a "config"
// object with exactly 6 groups.
func TestStatus_ConfigPresent(t *testing.T) {
	registry := auth.NewAccountRegistry()
	_ = registry.Add(&auth.AccountEntry{Label: "default", Authenticated: true})

	resp := callStatus(t, testConfig(), registry, time.Now())

	cfgObj, ok := resp["config"].(map[string]any)
	if !ok {
		t.Fatal("expected config object in response")
	}

	expectedGroups := []string{"identity", "logging", "storage", "graph_api", "features", "observability"}
	for _, group := range expectedGroups {
		if _, exists := cfgObj[group]; !exists {
			t.Errorf("missing config group %q", group)
		}
	}
	if len(cfgObj) != len(expectedGroups) {
		t.Errorf("expected %d config groups, got %d", len(expectedGroups), len(cfgObj))
	}
}

// TestStatus_IdentityGroup verifies that the identity config group contains
// all required fields with correct values.
func TestStatus_IdentityGroup(t *testing.T) {
	registry := auth.NewAccountRegistry()
	_ = registry.Add(&auth.AccountEntry{Label: "default", Authenticated: true})

	cfg := testConfig()
	resp := callStatus(t, cfg, registry, time.Now())

	cfgObj := resp["config"].(map[string]any)
	identity := cfgObj["identity"].(map[string]any)

	if identity["client_id"] != cfg.ClientID {
		t.Errorf("client_id = %v, want %q", identity["client_id"], cfg.ClientID)
	}
	if identity["tenant_id"] != cfg.TenantID {
		t.Errorf("tenant_id = %v, want %q", identity["tenant_id"], cfg.TenantID)
	}
	if identity["auth_method"] != cfg.AuthMethod {
		t.Errorf("auth_method = %v, want %q", identity["auth_method"], cfg.AuthMethod)
	}
	if identity["auth_method_source"] != cfg.AuthMethodSource {
		t.Errorf("auth_method_source = %v, want %q", identity["auth_method_source"], cfg.AuthMethodSource)
	}
}

// TestStatus_LoggingGroup verifies that the logging config group contains
// all required fields with correct values.
func TestStatus_LoggingGroup(t *testing.T) {
	registry := auth.NewAccountRegistry()
	_ = registry.Add(&auth.AccountEntry{Label: "default", Authenticated: true})

	cfg := testConfig()
	cfg.LogLevel = "debug"
	cfg.LogFile = "/tmp/mcp.log"
	resp := callStatus(t, cfg, registry, time.Now())

	cfgObj := resp["config"].(map[string]any)
	logging := cfgObj["logging"].(map[string]any)

	if logging["log_level"] != "debug" {
		t.Errorf("log_level = %v, want %q", logging["log_level"], "debug")
	}
	if logging["log_format"] != "json" {
		t.Errorf("log_format = %v, want %q", logging["log_format"], "json")
	}
	if logging["log_file"] != "/tmp/mcp.log" {
		t.Errorf("log_file = %v, want %q", logging["log_file"], "/tmp/mcp.log")
	}
	if logging["log_sanitize"] != true {
		t.Errorf("log_sanitize = %v, want true", logging["log_sanitize"])
	}
	if logging["audit_log_enabled"] != true {
		t.Errorf("audit_log_enabled = %v, want true", logging["audit_log_enabled"])
	}
	if logging["audit_log_path"] != "" {
		t.Errorf("audit_log_path = %v, want empty", logging["audit_log_path"])
	}
}

// TestStatus_StorageGroup verifies that the storage config group contains
// all required fields with correct values.
func TestStatus_StorageGroup(t *testing.T) {
	registry := auth.NewAccountRegistry()
	_ = registry.Add(&auth.AccountEntry{Label: "default", Authenticated: true})

	cfg := testConfig()
	resp := callStatus(t, cfg, registry, time.Now())

	cfgObj := resp["config"].(map[string]any)
	storage := cfgObj["storage"].(map[string]any)

	if storage["token_storage"] != "auto" {
		t.Errorf("token_storage = %v, want %q", storage["token_storage"], "auto")
	}
	if storage["token_cache_backend"] != "keychain" {
		t.Errorf("token_cache_backend = %v, want %q", storage["token_cache_backend"], "keychain")
	}
	if storage["auth_record_path"] != cfg.AuthRecordPath {
		t.Errorf("auth_record_path = %v, want %q", storage["auth_record_path"], cfg.AuthRecordPath)
	}
	if storage["accounts_path"] != cfg.AccountsPath {
		t.Errorf("accounts_path = %v, want %q", storage["accounts_path"], cfg.AccountsPath)
	}
	if storage["cache_name"] != "outlook-local-mcp" {
		t.Errorf("cache_name = %v, want %q", storage["cache_name"], "outlook-local-mcp")
	}
}

// TestStatus_GraphAPIGroup verifies that the graph_api config group contains
// all required fields with correct values.
func TestStatus_GraphAPIGroup(t *testing.T) {
	registry := auth.NewAccountRegistry()
	_ = registry.Add(&auth.AccountEntry{Label: "default", Authenticated: true})

	cfg := testConfig()
	resp := callStatus(t, cfg, registry, time.Now())

	cfgObj := resp["config"].(map[string]any)
	graphAPI := cfgObj["graph_api"].(map[string]any)

	if graphAPI["max_retries"] != float64(3) {
		t.Errorf("max_retries = %v, want 3", graphAPI["max_retries"])
	}
	if graphAPI["retry_backoff_ms"] != float64(1000) {
		t.Errorf("retry_backoff_ms = %v, want 1000", graphAPI["retry_backoff_ms"])
	}
	if graphAPI["request_timeout_seconds"] != float64(30) {
		t.Errorf("request_timeout_seconds = %v, want 30", graphAPI["request_timeout_seconds"])
	}
	if graphAPI["shutdown_timeout_seconds"] != float64(15) {
		t.Errorf("shutdown_timeout_seconds = %v, want 15", graphAPI["shutdown_timeout_seconds"])
	}
}

// TestStatus_FeaturesGroup verifies that the features config group contains
// all required fields with correct values.
func TestStatus_FeaturesGroup(t *testing.T) {
	registry := auth.NewAccountRegistry()
	_ = registry.Add(&auth.AccountEntry{Label: "default", Authenticated: true})

	cfg := testConfig()
	resp := callStatus(t, cfg, registry, time.Now())

	cfgObj := resp["config"].(map[string]any)
	features := cfgObj["features"].(map[string]any)

	if features["read_only"] != false {
		t.Errorf("read_only = %v, want false", features["read_only"])
	}
	if features["mail_enabled"] != false {
		t.Errorf("mail_enabled = %v, want false", features["mail_enabled"])
	}
	if features["provenance_tag"] != "com.github.desek.outlook-local-mcp.created" {
		t.Errorf("provenance_tag = %v, want %q", features["provenance_tag"], "com.github.desek.outlook-local-mcp.created")
	}
}

// TestStatus_ObservabilityGroup verifies that the observability config group
// contains all required fields with correct values.
func TestStatus_ObservabilityGroup(t *testing.T) {
	registry := auth.NewAccountRegistry()
	_ = registry.Add(&auth.AccountEntry{Label: "default", Authenticated: true})

	cfg := testConfig()
	resp := callStatus(t, cfg, registry, time.Now())

	cfgObj := resp["config"].(map[string]any)
	obs := cfgObj["observability"].(map[string]any)

	if obs["otel_enabled"] != false {
		t.Errorf("otel_enabled = %v, want false", obs["otel_enabled"])
	}
	if obs["otel_endpoint"] != "" {
		t.Errorf("otel_endpoint = %v, want empty", obs["otel_endpoint"])
	}
	if obs["otel_service_name"] != "outlook-local-mcp" {
		t.Errorf("otel_service_name = %v, want %q", obs["otel_service_name"], "outlook-local-mcp")
	}
}

// TestStatus_AuthMethodSourceExplicit verifies that auth_method_source reports
// "explicit" when the auth method was explicitly set.
func TestStatus_AuthMethodSourceExplicit(t *testing.T) {
	registry := auth.NewAccountRegistry()
	_ = registry.Add(&auth.AccountEntry{Label: "default", Authenticated: true})

	cfg := testConfig()
	cfg.AuthMethodSource = "explicit"
	resp := callStatus(t, cfg, registry, time.Now())

	cfgObj := resp["config"].(map[string]any)
	identity := cfgObj["identity"].(map[string]any)

	if identity["auth_method_source"] != "explicit" {
		t.Errorf("auth_method_source = %v, want %q", identity["auth_method_source"], "explicit")
	}
}

// TestStatus_AuthMethodSourceInferred verifies that auth_method_source reports
// "inferred" when the auth method was determined from a well-known client ID.
func TestStatus_AuthMethodSourceInferred(t *testing.T) {
	registry := auth.NewAccountRegistry()
	_ = registry.Add(&auth.AccountEntry{Label: "default", Authenticated: true})

	cfg := testConfig()
	cfg.AuthMethodSource = "inferred"
	resp := callStatus(t, cfg, registry, time.Now())

	cfgObj := resp["config"].(map[string]any)
	identity := cfgObj["identity"].(map[string]any)

	if identity["auth_method_source"] != "inferred" {
		t.Errorf("auth_method_source = %v, want %q", identity["auth_method_source"], "inferred")
	}
}

// TestStatus_TokenCacheBackendKeychain verifies that token_cache_backend
// reports "keychain" when the OS keychain was successfully initialized.
func TestStatus_TokenCacheBackendKeychain(t *testing.T) {
	registry := auth.NewAccountRegistry()
	_ = registry.Add(&auth.AccountEntry{Label: "default", Authenticated: true})

	cfg := testConfig()
	cfg.TokenCacheBackend = "keychain"
	resp := callStatus(t, cfg, registry, time.Now())

	cfgObj := resp["config"].(map[string]any)
	storage := cfgObj["storage"].(map[string]any)

	if storage["token_cache_backend"] != "keychain" {
		t.Errorf("token_cache_backend = %v, want %q", storage["token_cache_backend"], "keychain")
	}
}

// TestStatus_TokenCacheBackendFile verifies that token_cache_backend reports
// "file" when the file-based backend is in use.
func TestStatus_TokenCacheBackendFile(t *testing.T) {
	registry := auth.NewAccountRegistry()
	_ = registry.Add(&auth.AccountEntry{Label: "default", Authenticated: true})

	cfg := testConfig()
	cfg.TokenCacheBackend = "file"
	resp := callStatus(t, cfg, registry, time.Now())

	cfgObj := resp["config"].(map[string]any)
	storage := cfgObj["storage"].(map[string]any)

	if storage["token_cache_backend"] != "file" {
		t.Errorf("token_cache_backend = %v, want %q", storage["token_cache_backend"], "file")
	}
}

// TestStatus_BackwardCompatTopLevel verifies that the top-level fields
// (version, timezone, accounts, server_uptime_seconds) remain present for
// backward compatibility.
func TestStatus_BackwardCompatTopLevel(t *testing.T) {
	registry := auth.NewAccountRegistry()
	_ = registry.Add(&auth.AccountEntry{Label: "default", Authenticated: true})

	resp := callStatus(t, testConfig(), registry, time.Now())

	requiredFields := []string{"version", "timezone", "accounts", "server_uptime_seconds"}
	for _, field := range requiredFields {
		if _, exists := resp[field]; !exists {
			t.Errorf("missing top-level field %q", field)
		}
	}
}
