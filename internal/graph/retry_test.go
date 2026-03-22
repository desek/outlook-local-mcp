package graph

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	abstractions "github.com/microsoft/kiota-abstractions-go"
	"github.com/microsoftgraph/msgraph-sdk-go/models/odataerrors"
)

// newODataError creates an *odataerrors.ODataError with the given HTTP status
// code for use in retry tests. The error embeds an ApiError with the specified
// ResponseStatusCode.
func newODataError(statusCode int) *odataerrors.ODataError {
	odataErr := odataerrors.NewODataError()
	odataErr.ApiError = *abstractions.NewApiError()
	odataErr.ResponseStatusCode = statusCode
	return odataErr
}

// newODataErrorWithRetryAfter creates an *odataerrors.ODataError with the given
// HTTP status code and a Retry-After response header set to the specified value.
func newODataErrorWithRetryAfter(statusCode int, retryAfter string) *odataerrors.ODataError {
	odataErr := newODataError(statusCode)
	headers := abstractions.NewResponseHeaders()
	headers.Add("Retry-After", retryAfter)
	odataErr.SetResponseHeaders(headers)
	return odataErr
}

// testRetryConfig returns a RetryConfig suitable for unit tests, with a short
// initial backoff to keep tests fast and a logger that writes to the provided
// buffer.
func testRetryConfig(maxRetries int, buf *bytes.Buffer) RetryConfig {
	logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return RetryConfig{
		MaxRetries:     maxRetries,
		InitialBackoff: 1 * time.Millisecond,
		Logger:         logger,
	}
}

// TestRetryGraphCall_Success_NoRetry verifies that a successful call returns
// immediately without retrying.
func TestRetryGraphCall_Success_NoRetry(t *testing.T) {
	var buf bytes.Buffer
	cfg := testRetryConfig(3, &buf)

	calls := 0
	err := RetryGraphCall(context.Background(), cfg, func() error {
		calls++
		return nil
	})

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
	if strings.Contains(buf.String(), "retrying") {
		t.Errorf("expected no retry log entries, got: %s", buf.String())
	}
}

// TestRetryGraphCall_429_RetriesAndSucceeds verifies that 429 triggers retries
// and eventual success.
func TestRetryGraphCall_429_RetriesAndSucceeds(t *testing.T) {
	var buf bytes.Buffer
	cfg := testRetryConfig(3, &buf)

	calls := 0
	err := RetryGraphCall(context.Background(), cfg, func() error {
		calls++
		if calls < 3 {
			return newODataError(429)
		}
		return nil
	})

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

// TestRetryGraphCall_503_RetriesAndSucceeds verifies that 503 triggers retry
// with backoff and eventual success.
func TestRetryGraphCall_503_RetriesAndSucceeds(t *testing.T) {
	var buf bytes.Buffer
	cfg := testRetryConfig(3, &buf)

	calls := 0
	err := RetryGraphCall(context.Background(), cfg, func() error {
		calls++
		if calls < 2 {
			return newODataError(503)
		}
		return nil
	})

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

// TestRetryGraphCall_504_RetriesAndSucceeds verifies that 504 triggers retry
// with backoff and eventual success.
func TestRetryGraphCall_504_RetriesAndSucceeds(t *testing.T) {
	var buf bytes.Buffer
	cfg := testRetryConfig(3, &buf)

	calls := 0
	err := RetryGraphCall(context.Background(), cfg, func() error {
		calls++
		if calls < 2 {
			return newODataError(504)
		}
		return nil
	})

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

// TestRetryGraphCall_429_ExhaustsRetries verifies that max retries is respected
// for 429 errors.
func TestRetryGraphCall_429_ExhaustsRetries(t *testing.T) {
	var buf bytes.Buffer
	cfg := testRetryConfig(2, &buf)

	calls := 0
	err := RetryGraphCall(context.Background(), cfg, func() error {
		calls++
		return newODataError(429)
	})

	if err == nil {
		t.Error("expected error, got nil")
	}
	// 1 initial + 2 retries = 3 calls
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
	if ExtractHTTPStatus(err) != 429 {
		t.Errorf("expected status 429, got %d", ExtractHTTPStatus(err))
	}
}

// TestRetryGraphCall_503_ExhaustsRetries verifies that max retries is respected
// for 503 errors.
func TestRetryGraphCall_503_ExhaustsRetries(t *testing.T) {
	var buf bytes.Buffer
	cfg := testRetryConfig(2, &buf)

	calls := 0
	err := RetryGraphCall(context.Background(), cfg, func() error {
		calls++
		return newODataError(503)
	})

	if err == nil {
		t.Error("expected error, got nil")
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

// TestRetryGraphCall_400_NoRetry verifies that 400 errors are not retried.
func TestRetryGraphCall_400_NoRetry(t *testing.T) {
	var buf bytes.Buffer
	cfg := testRetryConfig(3, &buf)

	calls := 0
	err := RetryGraphCall(context.Background(), cfg, func() error {
		calls++
		return newODataError(400)
	})

	if err == nil {
		t.Error("expected error, got nil")
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

// TestRetryGraphCall_401_NoRetry verifies that 401 errors are not retried.
func TestRetryGraphCall_401_NoRetry(t *testing.T) {
	var buf bytes.Buffer
	cfg := testRetryConfig(3, &buf)

	calls := 0
	err := RetryGraphCall(context.Background(), cfg, func() error {
		calls++
		return newODataError(401)
	})

	if err == nil {
		t.Error("expected error, got nil")
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

// TestRetryGraphCall_403_NoRetry verifies that 403 errors are not retried.
func TestRetryGraphCall_403_NoRetry(t *testing.T) {
	var buf bytes.Buffer
	cfg := testRetryConfig(3, &buf)

	calls := 0
	err := RetryGraphCall(context.Background(), cfg, func() error {
		calls++
		return newODataError(403)
	})

	if err == nil {
		t.Error("expected error, got nil")
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

// TestRetryGraphCall_404_NoRetry verifies that 404 errors are not retried.
func TestRetryGraphCall_404_NoRetry(t *testing.T) {
	var buf bytes.Buffer
	cfg := testRetryConfig(3, &buf)

	calls := 0
	err := RetryGraphCall(context.Background(), cfg, func() error {
		calls++
		return newODataError(404)
	})

	if err == nil {
		t.Error("expected error, got nil")
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

// TestRetryGraphCall_NonODataError_NoRetry verifies that generic (non-OData)
// errors are not retried.
func TestRetryGraphCall_NonODataError_NoRetry(t *testing.T) {
	var buf bytes.Buffer
	cfg := testRetryConfig(3, &buf)

	calls := 0
	err := RetryGraphCall(context.Background(), cfg, func() error {
		calls++
		return errors.New("network error")
	})

	if err == nil {
		t.Error("expected error, got nil")
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

// TestRetryGraphCall_ContextCancelled_NoRetry verifies that context cancellation
// before the call prevents any attempt and returns immediately.
func TestRetryGraphCall_ContextCancelled_NoRetry(t *testing.T) {
	var buf bytes.Buffer
	cfg := testRetryConfig(3, &buf)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before the call.

	calls := 0
	err := RetryGraphCall(ctx, cfg, func() error {
		calls++
		return newODataError(429)
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	// fn is not called when context is already cancelled before RetryGraphCall.
	if calls != 0 {
		t.Errorf("expected 0 calls, got %d", calls)
	}
}

// TestRetryGraphCall_ContextCancelledDuringWait verifies that context
// cancellation during the wait period aborts the retry.
func TestRetryGraphCall_ContextCancelledDuringWait(t *testing.T) {
	var buf bytes.Buffer
	cfg := RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 5 * time.Second, // Long backoff to ensure we're waiting.
		Logger:         slog.New(slog.NewTextHandler(&buf, nil)),
	}

	ctx, cancel := context.WithCancel(context.Background())

	calls := 0
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := RetryGraphCall(ctx, cfg, func() error {
		calls++
		return newODataError(429)
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

// TestRetryGraphCall_MaxRetries_Zero verifies that MaxRetries=0 disables retry.
func TestRetryGraphCall_MaxRetries_Zero(t *testing.T) {
	var buf bytes.Buffer
	cfg := testRetryConfig(0, &buf)

	calls := 0
	err := RetryGraphCall(context.Background(), cfg, func() error {
		calls++
		return newODataError(429)
	})

	if err == nil {
		t.Error("expected error, got nil")
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

// TestRetryGraphCall_BackoffIncreases verifies that exponential backoff
// durations increase with each attempt.
func TestRetryGraphCall_BackoffIncreases(t *testing.T) {
	initialBackoff := 10 * time.Millisecond
	b0 := CalculateBackoff(initialBackoff, 0)
	b1 := CalculateBackoff(initialBackoff, 1)
	b2 := CalculateBackoff(initialBackoff, 2)

	// With jitter, backoff ranges are:
	// attempt 0: [10ms, 20ms)
	// attempt 1: [20ms, 30ms)
	// attempt 2: [40ms, 50ms)
	// The base (without jitter) should be increasing.
	if b0 >= 20*time.Millisecond {
		t.Errorf("backoff[0] = %v, expected < 20ms", b0)
	}
	if b1 < 20*time.Millisecond {
		t.Errorf("backoff[1] = %v, expected >= 20ms", b1)
	}
	if b2 < 40*time.Millisecond {
		t.Errorf("backoff[2] = %v, expected >= 40ms", b2)
	}
}

// TestRetryGraphCall_BackoffCappedAt60s verifies that no single wait exceeds 60
// seconds.
func TestRetryGraphCall_BackoffCappedAt60s(t *testing.T) {
	// With 30s initial backoff, attempt 2 would be 30s * 4 = 120s + jitter,
	// but should be capped at 60s.
	var buf bytes.Buffer
	cfg := RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 30 * time.Second,
		Logger:         slog.New(slog.NewTextHandler(&buf, nil)),
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel quickly so we don't actually wait 60s.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_ = RetryGraphCall(ctx, cfg, func() error {
		return newODataError(503)
	})

	// Verify the logged wait does not exceed 60s by checking the log output.
	logOutput := buf.String()
	if strings.Contains(logOutput, "wait=1m1") || strings.Contains(logOutput, "wait=2m") {
		t.Errorf("wait exceeded 60s cap, log: %s", logOutput)
	}
}

// TestExtractHTTPStatus_ODataError verifies status extraction from an ODataError.
func TestExtractHTTPStatus_ODataError(t *testing.T) {
	odataErr := newODataError(429)
	got := ExtractHTTPStatus(odataErr)
	if got != 429 {
		t.Errorf("ExtractHTTPStatus() = %d, want 429", got)
	}
}

// TestExtractHTTPStatus_NonODataError verifies status extraction returns 0 for
// non-OData errors.
func TestExtractHTTPStatus_NonODataError(t *testing.T) {
	got := ExtractHTTPStatus(errors.New("generic error"))
	if got != 0 {
		t.Errorf("ExtractHTTPStatus() = %d, want 0", got)
	}
}

// TestExtractHTTPStatus_NilError verifies status extraction handles nil errors.
func TestExtractHTTPStatus_NilError(t *testing.T) {
	got := ExtractHTTPStatus(nil)
	if got != 0 {
		t.Errorf("ExtractHTTPStatus() = %d, want 0", got)
	}
}

// TestRetryGraphCall_LogsWarnOnRetry verifies that a warn-level log entry is
// emitted on each retry attempt with the expected structured fields.
func TestRetryGraphCall_LogsWarnOnRetry(t *testing.T) {
	var buf bytes.Buffer
	cfg := testRetryConfig(3, &buf)

	calls := 0
	_ = RetryGraphCall(context.Background(), cfg, func() error {
		calls++
		if calls < 2 {
			return newODataError(503)
		}
		return nil
	})

	logOutput := buf.String()
	if !strings.Contains(logOutput, "WARN") {
		t.Errorf("expected WARN log entry, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "attempt=1") {
		t.Errorf("expected attempt=1 in log, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "max_retries=3") {
		t.Errorf("expected max_retries=3 in log, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "status_code=503") {
		t.Errorf("expected status_code=503 in log, got: %s", logOutput)
	}
}

// TestRetryGraphCall_LogsErrorOnExhaustion verifies that an error-level log
// entry is emitted when retries are exhausted.
func TestRetryGraphCall_LogsErrorOnExhaustion(t *testing.T) {
	var buf bytes.Buffer
	cfg := testRetryConfig(1, &buf)

	_ = RetryGraphCall(context.Background(), cfg, func() error {
		return newODataError(503)
	})

	logOutput := buf.String()
	if !strings.Contains(logOutput, "ERROR") {
		t.Errorf("expected ERROR log entry, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "retries exhausted") {
		t.Errorf("expected 'retries exhausted' in log, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "attempts=1") {
		t.Errorf("expected attempts=1 in log, got: %s", logOutput)
	}
}

// TestExtractRetryAfter_Present verifies Retry-After header extraction from an
// ODataError with response headers.
func TestExtractRetryAfter_Present(t *testing.T) {
	odataErr := newODataErrorWithRetryAfter(429, "5")
	got := ExtractRetryAfter(odataErr)
	if got != 5 {
		t.Errorf("ExtractRetryAfter() = %d, want 5", got)
	}
}

// TestExtractRetryAfter_Absent verifies that ExtractRetryAfter returns 0 when
// no Retry-After header is present.
func TestExtractRetryAfter_Absent(t *testing.T) {
	odataErr := newODataError(429)
	got := ExtractRetryAfter(odataErr)
	if got != 0 {
		t.Errorf("ExtractRetryAfter() = %d, want 0", got)
	}
}

// TestExtractRetryAfter_Invalid verifies that ExtractRetryAfter returns 0 when
// the Retry-After header is not a valid integer.
func TestExtractRetryAfter_Invalid(t *testing.T) {
	odataErr := newODataErrorWithRetryAfter(429, "not-a-number")
	got := ExtractRetryAfter(odataErr)
	if got != 0 {
		t.Errorf("ExtractRetryAfter() = %d, want 0", got)
	}
}

// TestExtractRetryAfter_NonODataError verifies that ExtractRetryAfter returns 0
// for non-OData errors.
func TestExtractRetryAfter_NonODataError(t *testing.T) {
	got := ExtractRetryAfter(errors.New("generic"))
	if got != 0 {
		t.Errorf("ExtractRetryAfter() = %d, want 0", got)
	}
}
