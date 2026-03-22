package graph

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// WithTimeout derives a child context from the given parent with the specified
// timeout duration. The child context inherits all values and cancellation
// signals from the parent, ensuring that parent cancellation (e.g., MCP client
// disconnect) propagates to the Graph API call.
//
// When timeout is zero or negative, the parent context is returned unchanged
// with a no-op cancel function, effectively disabling the timeout. This allows
// tests and callers to opt out of timeout enforcement by passing 0.
//
// Parameters:
//   - ctx: the parent context from the MCP framework.
//   - timeout: the maximum duration for the Graph API call. Zero means no timeout.
//
// Returns the deadline-scoped child context and a cancel function that MUST be
// called (typically via defer) to release resources.
//
// Side effects: none beyond context creation.
func WithTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

// IsTimeoutError reports whether the given error is or wraps
// context.DeadlineExceeded, indicating that a Graph API call exceeded its
// configured timeout.
//
// Parameters:
//   - err: the error to inspect, may be nil.
//
// Returns true if err is or wraps context.DeadlineExceeded, false otherwise.
func IsTimeoutError(err error) bool {
	return errors.Is(err, context.DeadlineExceeded)
}

// TimeoutErrorMessage returns a user-facing error message for a timed-out
// request. The format is "request timed out after Xs" where X is the integer
// timeout in seconds.
//
// Parameters:
//   - timeoutSeconds: the configured timeout in whole seconds.
//
// Returns the formatted timeout error message string.
func TimeoutErrorMessage(timeoutSeconds int) string {
	return fmt.Sprintf("request timed out after %ds", timeoutSeconds)
}
