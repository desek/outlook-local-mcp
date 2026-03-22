package graph

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// TestWithTimeout_DerivesChildContext verifies that WithTimeout returns a
// context with the specified deadline approximately in the future.
func TestWithTimeout_DerivesChildContext(t *testing.T) {
	ctx := context.Background()
	before := time.Now()

	childCtx, cancel := WithTimeout(ctx, 5*time.Second)
	defer cancel()

	deadline, ok := childCtx.Deadline()
	if !ok {
		t.Fatal("expected child context to have a deadline")
	}

	expected := before.Add(5 * time.Second)
	if deadline.Before(before) || deadline.After(expected.Add(100*time.Millisecond)) {
		t.Errorf("deadline = %v, want approximately %v", deadline, expected)
	}
}

// TestWithTimeout_ParentCancellation verifies that cancelling the parent
// context also cancels the child context derived by WithTimeout.
func TestWithTimeout_ParentCancellation(t *testing.T) {
	parentCtx, parentCancel := context.WithCancel(context.Background())

	childCtx, cancel := WithTimeout(parentCtx, 1*time.Hour)
	defer cancel()

	parentCancel()

	select {
	case <-childCtx.Done():
		// Expected: child context is cancelled when parent is cancelled.
	case <-time.After(1 * time.Second):
		t.Fatal("child context was not cancelled when parent was cancelled")
	}
}

// TestWithTimeout_DeadlineExpires verifies that the context expires after the
// specified duration.
func TestWithTimeout_DeadlineExpires(t *testing.T) {
	ctx := context.Background()

	childCtx, cancel := WithTimeout(ctx, 1*time.Millisecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond)

	if childCtx.Err() != context.DeadlineExceeded {
		t.Errorf("Err() = %v, want context.DeadlineExceeded", childCtx.Err())
	}
}

// TestWithTimeout_CancelFreesResources verifies that calling cancel before the
// deadline releases resources and marks the context as cancelled.
func TestWithTimeout_CancelFreesResources(t *testing.T) {
	ctx := context.Background()

	childCtx, cancel := WithTimeout(ctx, 1*time.Hour)
	cancel()

	if childCtx.Err() != context.Canceled {
		t.Errorf("Err() = %v, want context.Canceled", childCtx.Err())
	}
}

// TestIsTimeoutError_DeadlineExceeded verifies that IsTimeoutError returns true
// for context.DeadlineExceeded.
func TestIsTimeoutError_DeadlineExceeded(t *testing.T) {
	if !IsTimeoutError(context.DeadlineExceeded) {
		t.Error("IsTimeoutError(context.DeadlineExceeded) = false, want true")
	}
}

// TestIsTimeoutError_WrappedDeadlineExceeded verifies that IsTimeoutError
// returns true for a wrapped context.DeadlineExceeded error.
func TestIsTimeoutError_WrappedDeadlineExceeded(t *testing.T) {
	wrapped := fmt.Errorf("graph: %w", context.DeadlineExceeded)
	if !IsTimeoutError(wrapped) {
		t.Error("IsTimeoutError(wrapped DeadlineExceeded) = false, want true")
	}
}

// TestIsTimeoutError_OtherError verifies that IsTimeoutError returns false for
// a non-timeout error.
func TestIsTimeoutError_OtherError(t *testing.T) {
	if IsTimeoutError(errors.New("network error")) {
		t.Error("IsTimeoutError(network error) = true, want false")
	}
}

// TestIsTimeoutError_ContextCanceled verifies that IsTimeoutError returns false
// for context.Canceled (not a timeout).
func TestIsTimeoutError_ContextCanceled(t *testing.T) {
	if IsTimeoutError(context.Canceled) {
		t.Error("IsTimeoutError(context.Canceled) = true, want false")
	}
}

// TestIsTimeoutError_Nil verifies that IsTimeoutError returns false for nil.
func TestIsTimeoutError_Nil(t *testing.T) {
	if IsTimeoutError(nil) {
		t.Error("IsTimeoutError(nil) = true, want false")
	}
}

// TestTimeoutErrorMessage verifies the default 30-second message format.
func TestTimeoutErrorMessage(t *testing.T) {
	msg := TimeoutErrorMessage(30)
	want := "request timed out after 30s"
	if msg != want {
		t.Errorf("TimeoutErrorMessage(30) = %q, want %q", msg, want)
	}
}

// TestTimeoutErrorMessage_CustomValue verifies the message format with a custom
// timeout value.
func TestTimeoutErrorMessage_CustomValue(t *testing.T) {
	msg := TimeoutErrorMessage(60)
	want := "request timed out after 60s"
	if msg != want {
		t.Errorf("TimeoutErrorMessage(60) = %q, want %q", msg, want)
	}
}
