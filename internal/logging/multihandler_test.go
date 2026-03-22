package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

// TestMultiHandler_BothReceiveRecords verifies that both the primary and
// secondary handlers receive the same log record when Handle is called.
func TestMultiHandler_BothReceiveRecords(t *testing.T) {
	var primaryBuf, secondaryBuf bytes.Buffer
	primary := slog.NewJSONHandler(&primaryBuf, &slog.HandlerOptions{Level: slog.LevelInfo})
	secondary := slog.NewJSONHandler(&secondaryBuf, &slog.HandlerOptions{Level: slog.LevelInfo})

	mh := NewMultiHandler(primary, secondary)
	logger := slog.New(mh)
	logger.Info("both test", "key", "value")

	if !strings.Contains(primaryBuf.String(), "both test") {
		t.Errorf("primary missing message, got: %s", primaryBuf.String())
	}
	if !strings.Contains(secondaryBuf.String(), "both test") {
		t.Errorf("secondary missing message, got: %s", secondaryBuf.String())
	}
}

// TestMultiHandler_Enabled_EitherTrue verifies that Enabled returns true if
// either inner handler is enabled for the given level.
func TestMultiHandler_Enabled_EitherTrue(t *testing.T) {
	var buf bytes.Buffer
	primary := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	secondary := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})

	mh := NewMultiHandler(primary, secondary)

	if !mh.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("expected Enabled(Info) to be true when primary is at Info level")
	}
}

// TestMultiHandler_Enabled_BothFalse verifies that Enabled returns false when
// both inner handlers are disabled for the given level.
func TestMultiHandler_Enabled_BothFalse(t *testing.T) {
	var buf bytes.Buffer
	primary := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError})
	secondary := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError})

	mh := NewMultiHandler(primary, secondary)

	if mh.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("expected Enabled(Debug) to be false when both are at Error level")
	}
}

// errorHandler is a test double that always returns an error from Handle.
type errorHandler struct {
	slog.Handler
}

// Handle always returns an error to simulate a failing secondary handler.
func (h *errorHandler) Handle(_ context.Context, _ slog.Record) error {
	return errors.New("simulated write failure")
}

// TestMultiHandler_SecondaryError_PrimarySucceeds verifies that the primary
// handler receives the record even when the secondary handler returns an error.
func TestMultiHandler_SecondaryError_PrimarySucceeds(t *testing.T) {
	var primaryBuf bytes.Buffer
	primary := slog.NewJSONHandler(&primaryBuf, &slog.HandlerOptions{Level: slog.LevelInfo})
	secondary := &errorHandler{Handler: slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelInfo})}

	mh := NewMultiHandler(primary, secondary)
	logger := slog.New(mh)
	logger.Info("primary survives")

	output := primaryBuf.String()
	if !strings.Contains(output, "primary survives") {
		t.Errorf("primary should have received the record, got: %s", output)
	}
	if !strings.Contains(output, "secondary log handler failed") {
		t.Errorf("primary should have received a warning about secondary failure, got: %s", output)
	}
}

// TestMultiHandler_WithAttrs_PropagatesBoth verifies that WithAttrs applies
// the attributes to both inner handlers.
func TestMultiHandler_WithAttrs_PropagatesBoth(t *testing.T) {
	var primaryBuf, secondaryBuf bytes.Buffer
	primary := slog.NewJSONHandler(&primaryBuf, &slog.HandlerOptions{Level: slog.LevelInfo})
	secondary := slog.NewJSONHandler(&secondaryBuf, &slog.HandlerOptions{Level: slog.LevelInfo})

	mh := NewMultiHandler(primary, secondary)
	attributed := mh.WithAttrs([]slog.Attr{slog.String("component", "test")})
	logger := slog.New(attributed)
	logger.Info("attr test")

	for name, buf := range map[string]string{"primary": primaryBuf.String(), "secondary": secondaryBuf.String()} {
		if !strings.Contains(buf, "component") {
			t.Errorf("%s missing WithAttrs attribute 'component', got: %s", name, buf)
		}
	}
}

// TestMultiHandler_WithGroup_PropagatesBoth verifies that WithGroup applies
// the group to both inner handlers.
func TestMultiHandler_WithGroup_PropagatesBoth(t *testing.T) {
	var primaryBuf, secondaryBuf bytes.Buffer
	primary := slog.NewJSONHandler(&primaryBuf, &slog.HandlerOptions{Level: slog.LevelInfo})
	secondary := slog.NewJSONHandler(&secondaryBuf, &slog.HandlerOptions{Level: slog.LevelInfo})

	mh := NewMultiHandler(primary, secondary)
	grouped := mh.WithGroup("mygroup")
	logger := slog.New(grouped)
	logger.Info("group test", "field", "val")

	for name, buf := range map[string]string{"primary": primaryBuf.String(), "secondary": secondaryBuf.String()} {
		var record map[string]any
		if err := json.Unmarshal([]byte(buf), &record); err != nil {
			t.Fatalf("%s output is not valid JSON: %v", name, err)
		}
		if _, ok := record["mygroup"]; !ok {
			t.Errorf("%s missing group 'mygroup' in output: %s", name, buf)
		}
	}
}

// TestMultiHandler_ConcurrentSafety verifies that concurrent Handle calls do
// not cause data races. This test should be run with -race to detect races.
func TestMultiHandler_ConcurrentSafety(t *testing.T) {
	var primaryBuf, secondaryBuf bytes.Buffer
	primary := slog.NewJSONHandler(&primaryBuf, &slog.HandlerOptions{Level: slog.LevelInfo})
	secondary := slog.NewJSONHandler(&secondaryBuf, &slog.HandlerOptions{Level: slog.LevelInfo})

	mh := NewMultiHandler(primary, secondary)
	logger := slog.New(mh)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			logger.Info("concurrent", "goroutine", n)
		}(i)
	}
	wg.Wait()
}
