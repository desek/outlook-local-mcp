package graph

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"strconv"
	"time"

	"github.com/microsoftgraph/msgraph-sdk-go/models/odataerrors"
)

// MaxRetryWait is the maximum duration for any single retry wait. This cap
// prevents unbounded sleep times when the exponential backoff calculation
// produces very large values.
const MaxRetryWait = 60 * time.Second

// RetryConfig holds configuration for the Graph API retry middleware.
// It is constructed once at startup from the loaded config and passed to
// registerTools for use by all tool handlers.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts after the initial call.
	// A value of 0 disables retrying.
	MaxRetries int

	// InitialBackoff is the base duration for exponential backoff calculations.
	// The actual wait for attempt N is min(InitialBackoff * 2^N + jitter, 60s).
	InitialBackoff time.Duration

	// Logger is the structured logger used for retry-related log entries.
	Logger *slog.Logger
}

// ExtractHTTPStatus inspects an error for an embedded *odataerrors.ODataError
// and returns the HTTP response status code. If the error is not an ODataError,
// 0 is returned.
//
// Parameters:
//   - err: the error to inspect.
//
// Returns the HTTP status code from the ODataError, or 0 if not applicable.
func ExtractHTTPStatus(err error) int {
	if err == nil {
		return 0
	}
	var odataErr *odataerrors.ODataError
	if errors.As(err, &odataErr) {
		return odataErr.ResponseStatusCode
	}
	return 0
}

// ExtractRetryAfter attempts to read a Retry-After value (in seconds) from an
// ODataError's response headers. If the header is absent, unparseable, or the
// error is not an ODataError, 0 is returned.
//
// Parameters:
//   - err: the error to inspect for response headers.
//
// Returns the Retry-After value in seconds, or 0 if unavailable.
func ExtractRetryAfter(err error) int {
	var odataErr *odataerrors.ODataError
	if !errors.As(err, &odataErr) {
		return 0
	}
	headers := odataErr.GetResponseHeaders()
	if headers == nil {
		return 0
	}
	values := headers.Get("Retry-After")
	if len(values) == 0 {
		return 0
	}
	secs, parseErr := strconv.Atoi(values[0])
	if parseErr != nil || secs <= 0 {
		return 0
	}
	return secs
}

// RetryGraphCall executes fn and retries on transient Graph API errors (HTTP
// 429, 503, 504) up to cfg.MaxRetries times. Non-retryable errors and context
// cancellations are returned immediately.
//
// For HTTP 429 responses, the Retry-After header is respected when present.
// For HTTP 503 and 504, exponential backoff with jitter is used:
//
//	wait = min(cfg.InitialBackoff * 2^attempt + jitter, 60s)
//
// where jitter is a random duration in [0, cfg.InitialBackoff).
//
// Parameters:
//   - ctx: the request context. If cancelled during a wait, the function
//     returns ctx.Err() immediately.
//   - cfg: retry configuration (max retries, initial backoff, logger).
//   - fn: the callable to execute. It is re-invoked on each retry attempt.
//
// Returns nil on success, or the last error after retries are exhausted.
//
// Side effects: sleeps between retries; logs at warn level on each retry
// attempt and at error level when retries are exhausted.
func RetryGraphCall(ctx context.Context, cfg RetryConfig, fn func() error) error {
	// Return immediately if the context is already cancelled before any attempt.
	if ctx.Err() != nil {
		return ctx.Err()
	}

	var lastErr error

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		// Do not retry if context was cancelled during the call.
		if ctx.Err() != nil {
			return ctx.Err()
		}

		status := ExtractHTTPStatus(lastErr)
		if status != 429 && status != 503 && status != 504 {
			return lastErr
		}

		// No more retries remaining.
		if attempt >= cfg.MaxRetries {
			break
		}

		// Calculate wait duration.
		wait := CalculateBackoff(cfg.InitialBackoff, attempt)
		if status == 429 {
			if retryAfter := ExtractRetryAfter(lastErr); retryAfter > 0 {
				wait = time.Duration(retryAfter) * time.Second
			}
		}
		if wait > MaxRetryWait {
			wait = MaxRetryWait
		}

		cfg.Logger.Warn("retrying graph API call",
			"attempt", attempt+1,
			"max_retries", cfg.MaxRetries,
			"status_code", status,
			"wait", wait,
		)

		// Wait with context cancellation support.
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	status := ExtractHTTPStatus(lastErr)
	cfg.Logger.Error("graph API retries exhausted",
		"attempts", cfg.MaxRetries,
		"status_code", status,
		"error", lastErr,
	)
	return lastErr
}

// CalculateBackoff computes the exponential backoff duration for the given
// attempt number: initialBackoff * 2^attempt + jitter, where jitter is a
// random duration in [0, initialBackoff).
//
// Parameters:
//   - initialBackoff: the base backoff duration.
//   - attempt: the zero-indexed retry attempt number.
//
// Returns the calculated backoff duration (not capped; caller applies the cap).
func CalculateBackoff(initialBackoff time.Duration, attempt int) time.Duration {
	backoff := initialBackoff * (1 << uint(attempt))
	jitter := time.Duration(rand.Int64N(int64(initialBackoff)))
	return backoff + jitter
}
