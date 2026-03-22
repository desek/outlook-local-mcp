// Package graph provides Graph API utilities for the Outlook Calendar MCP Server.
//
// This file tests the debug-level logging pattern used by tool handlers to log
// Graph API request URLs and response summaries. It verifies that slog.Debug
// calls with the expected attributes are captured by a debug-level handler.
package graph

import (
	"context"
	"log/slog"
	"sync"
	"testing"
)

// recordStore is a shared container for captured log records. It is used by
// capturingHandler instances created via WithAttrs/WithGroup so that all
// child handlers append to the same underlying slice.
type recordStore struct {
	mu      sync.Mutex
	records []slog.Record
}

// append adds a record to the store.
func (s *recordStore) append(r slog.Record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, r)
}

// snapshot returns a copy of all captured records.
func (s *recordStore) snapshot() []slog.Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]slog.Record, len(s.records))
	copy(cp, s.records)
	return cp
}

// capturingHandler is a slog.Handler that records all log records for
// test assertions. It only captures records at or above its configured level.
type capturingHandler struct {
	store *recordStore
	level slog.Level
}

// Enabled reports whether the handler is configured to log at the given level.
func (h *capturingHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle stores the log record for later inspection.
func (h *capturingHandler) Handle(_ context.Context, r slog.Record) error {
	h.store.append(r)
	return nil
}

// WithAttrs returns a new handler sharing the same record store.
func (h *capturingHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return &capturingHandler{store: h.store, level: h.level}
}

// WithGroup returns the handler unchanged (groups are not used in tests).
func (h *capturingHandler) WithGroup(_ string) slog.Handler {
	return h
}

// TestDebugLogRequestURL verifies that a debug-level logger captures Graph API
// request log entries with the expected "endpoint" attribute. This mirrors the
// pattern used in tool handlers before Graph API calls.
func TestDebugLogRequestURL(t *testing.T) {
	store := &recordStore{}
	handler := &capturingHandler{store: store, level: slog.LevelDebug}
	logger := slog.New(handler).With("tool", "list_events")

	// Simulate what tool handlers do before a Graph API call.
	logger.Debug("graph API request",
		"endpoint", "GET /me/calendarView",
		"start_datetime", "2026-03-12T00:00:00Z",
		"end_datetime", "2026-03-13T00:00:00Z",
		"top", int32(25))

	records := store.snapshot()
	if len(records) == 0 {
		t.Fatal("expected at least one debug log record")
	}

	r := records[0]
	if r.Level != slog.LevelDebug {
		t.Errorf("level = %v, want %v", r.Level, slog.LevelDebug)
	}
	if r.Message != "graph API request" {
		t.Errorf("message = %q, want %q", r.Message, "graph API request")
	}

	// Verify the endpoint attribute is present.
	found := false
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "endpoint" && a.Value.String() == "GET /me/calendarView" {
			found = true
			return false
		}
		return true
	})
	if !found {
		t.Error("expected 'endpoint' attribute with value 'GET /me/calendarView'")
	}
}

// TestDebugLogResponseBody verifies that a debug-level logger captures Graph API
// response log entries with a summary (not the full body). This mirrors the
// pattern used in tool handlers after successful Graph API calls.
func TestDebugLogResponseBody(t *testing.T) {
	store := &recordStore{}
	handler := &capturingHandler{store: store, level: slog.LevelDebug}
	logger := slog.New(handler).With("tool", "list_calendars")

	// Simulate what tool handlers do after a successful Graph API call.
	logger.Debug("graph API response",
		"endpoint", "GET /me/calendars",
		"count", 3)

	records := store.snapshot()
	if len(records) == 0 {
		t.Fatal("expected at least one debug log record")
	}

	r := records[0]
	if r.Level != slog.LevelDebug {
		t.Errorf("level = %v, want %v", r.Level, slog.LevelDebug)
	}
	if r.Message != "graph API response" {
		t.Errorf("message = %q, want %q", r.Message, "graph API response")
	}

	// Verify the endpoint and count attributes are present.
	foundEndpoint := false
	foundCount := false
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "endpoint" && a.Value.String() == "GET /me/calendars" {
			foundEndpoint = true
		}
		if a.Key == "count" {
			foundCount = true
		}
		return true
	})
	if !foundEndpoint {
		t.Error("expected 'endpoint' attribute with value 'GET /me/calendars'")
	}
	if !foundCount {
		t.Error("expected 'count' attribute in response log")
	}
}

// TestDebugLogNotCapturedAtInfoLevel verifies that debug-level Graph API
// logs are not emitted when the logger is configured at info level, ensuring
// these logs only appear when debug logging is explicitly enabled.
func TestDebugLogNotCapturedAtInfoLevel(t *testing.T) {
	store := &recordStore{}
	handler := &capturingHandler{store: store, level: slog.LevelInfo}
	logger := slog.New(handler).With("tool", "get_event")

	logger.Debug("graph API request",
		"endpoint", "GET /me/events/{id}",
		"event_id", "AAMk123")

	records := store.snapshot()
	if len(records) != 0 {
		t.Errorf("expected 0 records at info level, got %d", len(records))
	}
}
