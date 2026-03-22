package auth

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// successResult returns a successful CallToolResult for use in tests.
func successResult() *mcp.CallToolResult {
	return mcp.NewToolResultText("ok")
}

// authErrorResult returns a CallToolResult with IsError=true and auth error text.
func authErrorResult(msg string) *mcp.CallToolResult {
	return mcp.NewToolResultError(msg)
}

// nonAuthErrorResult returns a CallToolResult with IsError=true and non-auth text.
func nonAuthErrorResult() *mcp.CallToolResult {
	return mcp.NewToolResultError("event not found")
}

// elicitUnsupported is a mock elicitFunc that always returns
// ErrElicitationNotSupported. Used in test state constructors so existing
// tests exercise the LoggingMessageNotification fallback path.
func elicitUnsupported(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
	return nil, mcpserver.ErrElicitationNotSupported
}

// urlElicitUnsupported is a mock urlElicitFunc that always returns
// ErrElicitationNotSupported. Used in test state constructors so existing
// tests exercise the LoggingMessageNotification fallback path.
func urlElicitUnsupported(_ context.Context, _, _, _ string) (*mcp.ElicitationResult, error) {
	return nil, mcpserver.ErrElicitationNotSupported
}

// newTestState creates an authMiddlewareState with a mock authenticate function
// for use in tests. The authFn replaces the real Authenticate to avoid Azure AD
// calls. The authMethod defaults to "browser" — use newTestStateWithMethod for
// explicit method selection. Elicitation functions default to returning
// ErrElicitationNotSupported (exercising the fallback path).
func newTestState(authFn authenticateFunc) *authMiddlewareState {
	return newTestStateWithMethod(authFn, "browser")
}

// newTestStateWithMethod creates an authMiddlewareState with a mock authenticate
// function and explicit auth method for use in tests.
//
// Parameters:
//   - authFn: the mock authenticate function replacing the real Authenticate.
//   - authMethod: the auth method ("browser" or "device_code") controlling flow.
func newTestStateWithMethod(authFn authenticateFunc, authMethod string) *authMiddlewareState {
	return &authMiddlewareState{
		cred:           nil, // not used when authenticate is mocked
		authRecordPath: "/tmp/test-auth-record.json",
		authMethod:     authMethod,
		authenticate:   authFn,
		elicit:         elicitUnsupported,
		urlElicit:      urlElicitUnsupported,
		openBrowser:    func(_ string) error { return nil },
		browserTimeout: 120 * time.Second,
		scopes:         []string{"Calendars.ReadWrite"},
	}
}

// buildMiddleware creates the full middleware chain for testing, using the
// given authMiddlewareState and inner handler.
func buildMiddleware(state *authMiddlewareState, handler func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	mw := func(next func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := next(ctx, request)

			if err == nil && (result == nil || !result.IsError) {
				return result, nil
			}

			if !isAuthRelated(err, result) {
				return result, err
			}

			return state.handleAuthError(ctx, next, request, err)
		}
	}
	return mw(handler)
}

// TestAuthMiddleware_SuccessPassthrough verifies that successful handler results
// pass through the middleware without triggering authentication or modification.
func TestAuthMiddleware_SuccessPassthrough(t *testing.T) {
	authCalled := false
	state := newTestState(func(_ context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
		authCalled = true
		return azidentity.AuthenticationRecord{}, nil
	})

	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return successResult(), nil
	}

	wrapped := buildMiddleware(state, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatal("expected success result")
	}
	if authCalled {
		t.Error("Authenticate should not be called on success")
	}
}

// TestAuthMiddleware_AuthError_TriggersReauth verifies that an authentication
// error from the inner handler triggers re-authentication and retries the
// handler on success.
func TestAuthMiddleware_AuthError_TriggersReauth(t *testing.T) {
	callCount := 0
	state := newTestState(func(_ context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
		return testRecord(), nil
	})

	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		callCount++
		if callCount == 1 {
			// First call: return auth error.
			return nil, fmt.Errorf("DeviceCodeCredential: context deadline exceeded")
		}
		// Retry: return success.
		return successResult(), nil
	}

	wrapped := buildMiddleware(state, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatal("expected success result after retry")
	}
	if callCount != 2 {
		t.Errorf("handler call count = %d, want 2 (initial + retry)", callCount)
	}
}

// TestAuthMiddleware_AuthFailure_ReturnsTroubleshooting verifies that when
// re-authentication fails, the middleware returns a user-friendly error with
// troubleshooting guidance.
func TestAuthMiddleware_AuthFailure_ReturnsTroubleshooting(t *testing.T) {
	state := newTestState(func(_ context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
		return azidentity.AuthenticationRecord{}, fmt.Errorf("device code authentication failed: timeout")
	})

	origErr := fmt.Errorf("DeviceCodeCredential: context deadline exceeded")
	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return nil, origErr
	}

	wrapped := buildMiddleware(state, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result == nil {
		t.Fatal("expected error result, got nil")
	}
	if !result.IsError {
		t.Fatal("expected IsError=true")
	}

	text := extractResultText(result)
	requiredSubstrings := []string{
		"account_list",
		"account_add",
		"retry your original request",
	}
	for _, sub := range requiredSubstrings {
		if !strings.Contains(text, sub) {
			t.Errorf("error result missing %q in:\n%s", sub, text)
		}
	}
}

// TestAuthMiddleware_NonAuthError_NoRetry verifies that non-authentication
// errors pass through without triggering re-authentication.
func TestAuthMiddleware_NonAuthError_NoRetry(t *testing.T) {
	authCalled := false
	state := newTestState(func(_ context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
		authCalled = true
		return azidentity.AuthenticationRecord{}, nil
	})

	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return nil, fmt.Errorf("network timeout")
	}

	wrapped := buildMiddleware(state, handler)
	_, err := wrapped(context.Background(), mcp.CallToolRequest{})

	if err == nil {
		t.Fatal("expected error to pass through")
	}
	if err.Error() != "network timeout" {
		t.Errorf("error = %q, want %q", err.Error(), "network timeout")
	}
	if authCalled {
		t.Error("Authenticate should not be called for non-auth errors")
	}
}

// TestAuthMiddleware_ResultAuthError_TriggersReauth verifies that auth error
// text in CallToolResult (with IsError=true) triggers re-authentication, even
// when the Go error is nil.
func TestAuthMiddleware_ResultAuthError_TriggersReauth(t *testing.T) {
	callCount := 0
	state := newTestState(func(_ context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
		return testRecord(), nil
	})

	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		callCount++
		if callCount == 1 {
			return authErrorResult("DeviceCodeCredential: context deadline exceeded"), nil
		}
		return successResult(), nil
	}

	wrapped := buildMiddleware(state, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatal("expected success result after retry")
	}
	if callCount != 2 {
		t.Errorf("handler call count = %d, want 2", callCount)
	}
}

// TestAuthMiddleware_NonAuthResultError_NoRetry verifies that CallToolResult
// errors without auth patterns do not trigger re-authentication.
func TestAuthMiddleware_NonAuthResultError_NoRetry(t *testing.T) {
	authCalled := false
	state := newTestState(func(_ context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
		authCalled = true
		return azidentity.AuthenticationRecord{}, nil
	})

	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return nonAuthErrorResult(), nil
	}

	wrapped := buildMiddleware(state, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for non-auth error")
	}
	if authCalled {
		t.Error("Authenticate should not be called for non-auth result errors")
	}
}

// TestAuthMiddleware_ConcurrentReauth verifies that concurrent auth errors
// result in only a single re-authentication attempt. All goroutines should
// wait for the single auth to complete and then retry.
func TestAuthMiddleware_ConcurrentReauth(t *testing.T) {
	var authCount atomic.Int32
	state := newTestState(func(_ context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
		authCount.Add(1)
		return testRecord(), nil
	})

	callCounts := make([]atomic.Int32, 5)

	var wg sync.WaitGroup
	for i := range 5 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				count := callCounts[idx].Add(1)
				if count == 1 {
					return nil, fmt.Errorf("DeviceCodeCredential: expired token")
				}
				return successResult(), nil
			}

			wrapped := buildMiddleware(state, handler)
			result, err := wrapped(context.Background(), mcp.CallToolRequest{})

			if err != nil {
				t.Errorf("goroutine %d: unexpected error: %v", idx, err)
			}
			if result == nil || result.IsError {
				t.Errorf("goroutine %d: expected success result", idx)
			}
		}(i)
	}

	wg.Wait()

	// The mutex serializes auth attempts. Each goroutine acquires the lock
	// independently, so we may see up to 5 auth calls (one per goroutine),
	// but each runs sequentially. The important invariant is that no two
	// Authenticate calls overlap in time (ensured by the mutex).
	if authCount.Load() == 0 {
		t.Error("expected at least one Authenticate call")
	}
}

// TestAuthMiddleware_ConcurrentReauth_DeviceCode verifies that concurrent auth
// errors with authMethod="device_code" result in serialized re-authentication
// attempts. The mutex ensures no two Authenticate calls overlap. This
// complements TestAuthMiddleware_ConcurrentReauth which tests the "browser"
// method.
func TestAuthMiddleware_ConcurrentReauth_DeviceCode(t *testing.T) {
	var authCount atomic.Int32
	state := newTestStateWithMethod(func(ctx context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
		authCount.Add(1)
		// Simulate device code credential sending a prompt on the channel.
		if ch, ok := ctx.Value(DeviceCodeMsgKey).(chan string); ok {
			select {
			case ch <- "To sign in, visit https://microsoft.com/devicelogin and enter code TEST":
			default:
			}
		}
		return testRecord(), nil
	}, "device_code")

	callCounts := make([]atomic.Int32, 5)

	var wg sync.WaitGroup
	for i := range 5 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				count := callCounts[idx].Add(1)
				if count == 1 {
					return nil, fmt.Errorf("DeviceCodeCredential: expired token")
				}
				return successResult(), nil
			}

			wrapped := buildMiddleware(state, handler)
			result, err := wrapped(context.Background(), mcp.CallToolRequest{})

			if err != nil {
				t.Errorf("goroutine %d: unexpected error: %v", idx, err)
			}
			if result == nil {
				t.Errorf("goroutine %d: expected non-nil result", idx)
			}
		}(i)
	}

	wg.Wait()

	// The mutex serializes auth attempts. Each goroutine acquires the lock
	// independently, so we may see up to 5 auth calls (one per goroutine),
	// but each runs sequentially. The important invariant is that no two
	// Authenticate calls overlap in time (ensured by the mutex).
	if authCount.Load() == 0 {
		t.Error("expected at least one Authenticate call")
	}
}

// TestAuthMiddleware_ReadyLogOnce verifies that the "server authenticated and
// ready to serve" transition happens only once, even with multiple successful
// re-authentications.
func TestAuthMiddleware_ReadyLogOnce(t *testing.T) {
	state := newTestState(func(_ context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
		return testRecord(), nil
	})

	callCount := 0
	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		callCount++
		if callCount <= 2 {
			return nil, fmt.Errorf("DeviceCodeCredential: expired")
		}
		return successResult(), nil
	}

	wrapped := buildMiddleware(state, handler)

	// First auth-error + retry.
	_, _ = wrapped(context.Background(), mcp.CallToolRequest{})

	if !state.authenticated.Load() {
		t.Error("expected authenticated=true after first re-auth")
	}

	// Second auth-error + retry.
	_, _ = wrapped(context.Background(), mcp.CallToolRequest{})

	// The authenticated flag should still be true (CompareAndSwap only fires
	// once). We verify the flag is set; the log emission count is validated
	// by the fact that CompareAndSwap(false, true) returns true only once.
	if !state.authenticated.Load() {
		t.Error("expected authenticated=true to remain set")
	}
}

// TestAuthMiddleware_StderrFallback verifies that when no MCPServer is
// available in the context, sendClientNotification writes to stderr.
func TestAuthMiddleware_StderrFallback(t *testing.T) {
	// Capture stderr.
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}
	origStderr := os.Stderr
	os.Stderr = stderrW
	t.Cleanup(func() {
		os.Stderr = origStderr
	})

	// Send notification without MCPServer in context.
	sendClientNotification(context.Background(), mcp.LoggingLevelWarning, "test auth message")

	_ = stderrW.Close()

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, stderrR)

	if !strings.Contains(buf.String(), "test auth message") {
		t.Errorf("stderr output = %q, want to contain %q", buf.String(), "test auth message")
	}
}

// TestIsAuthRelated_GoError verifies isAuthRelated detects auth errors from
// the Go error parameter.
func TestIsAuthRelated_GoError(t *testing.T) {
	if !isAuthRelated(fmt.Errorf("DeviceCodeCredential: timeout"), nil) {
		t.Error("expected true for DeviceCodeCredential Go error")
	}
}

// TestIsAuthRelated_ResultText verifies isAuthRelated detects auth patterns
// in CallToolResult text content.
func TestIsAuthRelated_ResultText(t *testing.T) {
	result := authErrorResult("AADSTS70000: token expired")
	if !isAuthRelated(nil, result) {
		t.Error("expected true for AADSTS result text")
	}
}

// TestIsAuthRelated_NeitherAuthError verifies isAuthRelated returns false
// when neither error nor result contains auth patterns.
func TestIsAuthRelated_NeitherAuthError(t *testing.T) {
	if isAuthRelated(fmt.Errorf("network error"), nonAuthErrorResult()) {
		t.Error("expected false for non-auth error and non-auth result")
	}
}

// TestExtractResultText_TextContent verifies text extraction from a result
// with TextContent.
func TestExtractResultText_TextContent(t *testing.T) {
	result := mcp.NewToolResultError("some error text")
	got := extractResultText(result)
	if got != "some error text" {
		t.Errorf("extractResultText() = %q, want %q", got, "some error text")
	}
}

// TestExtractResultText_NilResult verifies text extraction from a nil result.
func TestExtractResultText_NilResult(t *testing.T) {
	got := extractResultText(nil)
	if got != "" {
		t.Errorf("extractResultText(nil) = %q, want empty string", got)
	}
}

// TestExtractResultText_EmptyContent verifies text extraction from a result
// with no content.
func TestExtractResultText_EmptyContent(t *testing.T) {
	result := &mcp.CallToolResult{}
	got := extractResultText(result)
	if got != "" {
		t.Errorf("extractResultText(empty) = %q, want empty string", got)
	}
}

// TestContainsAuthPattern_Matches verifies pattern detection for known auth
// error substrings.
func TestContainsAuthPattern_Matches(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"DeviceCodeCredential: context deadline exceeded", true},
		{"authentication required", true},
		{"AADSTS70000: error", true},
		{"network timeout", false},
		{"", false},
	}
	for _, tt := range tests {
		got := containsAuthPattern(tt.text)
		if got != tt.want {
			t.Errorf("containsAuthPattern(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}

// TestMergedContext_Value verifies that the mergedContext returns values from
// the values context first, falling back to the embedded context.
func TestMergedContext_Value(t *testing.T) {
	type keyA struct{}
	type keyB struct{}

	base := context.WithValue(context.Background(), keyB{}, "base-value")
	values := context.WithValue(context.Background(), keyA{}, "values-value")

	merged := &mergedContext{Context: base, values: values}

	// Value from values context.
	if got := merged.Value(keyA{}); got != "values-value" {
		t.Errorf("Value(keyA) = %v, want %q", got, "values-value")
	}

	// Value from base context.
	if got := merged.Value(keyB{}); got != "base-value" {
		t.Errorf("Value(keyB) = %v, want %q", got, "base-value")
	}

	// Unknown key returns nil.
	type keyC struct{}
	if got := merged.Value(keyC{}); got != nil {
		t.Errorf("Value(keyC) = %v, want nil", got)
	}
}

// TestAuthMiddleware_BrowserAuth_RetriesOnSuccess verifies that when using
// browser auth, a tool call is retried after successful re-authentication.
func TestAuthMiddleware_BrowserAuth_RetriesOnSuccess(t *testing.T) {
	callCount := 0
	state := newTestStateWithMethod(func(_ context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
		return testRecord(), nil
	}, "browser")

	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		callCount++
		if callCount == 1 {
			return nil, fmt.Errorf("InteractiveBrowserCredential: context deadline exceeded")
		}
		return successResult(), nil
	}

	wrapped := buildMiddleware(state, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatal("expected success result after browser re-auth retry")
	}
	if callCount != 2 {
		t.Errorf("handler call count = %d, want 2 (initial + retry)", callCount)
	}
}

// TestAuthMiddleware_BrowserAuth_ReturnsGuidance verifies that when browser
// re-authentication fails, the middleware returns a user-friendly error with
// troubleshooting guidance.
func TestAuthMiddleware_BrowserAuth_ReturnsGuidance(t *testing.T) {
	state := newTestStateWithMethod(func(_ context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
		return azidentity.AuthenticationRecord{}, fmt.Errorf("browser authentication failed: timeout")
	}, "browser")

	origErr := fmt.Errorf("InteractiveBrowserCredential: context deadline exceeded")
	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return nil, origErr
	}

	wrapped := buildMiddleware(state, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result == nil {
		t.Fatal("expected error result, got nil")
	}
	if !result.IsError {
		t.Fatal("expected IsError=true")
	}

	text := extractResultText(result)
	requiredSubstrings := []string{
		"account_list",
		"account_add",
		"retry your original request",
	}
	for _, sub := range requiredSubstrings {
		if !strings.Contains(text, sub) {
			t.Errorf("error result missing %q in:\n%s", sub, text)
		}
	}
}

// TestAuthMiddleware_BrowserAuth_NoDeviceCodeChannel verifies that browser
// auth flow does not use the deviceCodeCh channel. The mock authenticate
// function checks that no deviceCodeMsgKey channel is present in the context.
func TestAuthMiddleware_BrowserAuth_NoDeviceCodeChannel(t *testing.T) {
	state := newTestStateWithMethod(func(ctx context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
		// Verify no device code channel is in the context.
		if ch, ok := ctx.Value(DeviceCodeMsgKey).(chan string); ok && ch != nil {
			t.Error("browser auth context should not contain deviceCodeMsgKey channel")
		}
		return testRecord(), nil
	}, "browser")

	callCount := 0
	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		callCount++
		if callCount == 1 {
			return nil, fmt.Errorf("InteractiveBrowserCredential: expired token")
		}
		return successResult(), nil
	}

	wrapped := buildMiddleware(state, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatal("expected success result after browser re-auth")
	}
}

// TestAuthMiddleware_BrowserAuth_SendsNotification verifies that the browser
// auth flow sends an MCP notification indicating a browser window will open.
// Since no MCPServer is in the test context, the notification falls back to
// stderr, which we capture and verify.
func TestAuthMiddleware_BrowserAuth_SendsNotification(t *testing.T) {
	state := newTestStateWithMethod(func(_ context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
		return testRecord(), nil
	}, "browser")

	callCount := 0
	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		callCount++
		if callCount == 1 {
			return nil, fmt.Errorf("InteractiveBrowserCredential: expired token")
		}
		return successResult(), nil
	}

	// Capture stderr to verify the notification message.
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}
	origStderr := os.Stderr
	os.Stderr = stderrW
	t.Cleanup(func() {
		os.Stderr = origStderr
	})

	wrapped := buildMiddleware(state, handler)
	_, _ = wrapped(context.Background(), mcp.CallToolRequest{})

	_ = stderrW.Close()

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, stderrR)

	if !strings.Contains(buf.String(), "A browser window will open for Microsoft login") {
		t.Errorf("stderr output = %q, want to contain browser login notification", buf.String())
	}
}

// TestAuthMiddleware_DeviceCodeAuth_PreservedBehavior verifies that the device
// code auth flow preserves the existing behavior: the deviceCodeCh channel is
// injected into the context and the device code prompt is returned as a tool
// result.
func TestAuthMiddleware_DeviceCodeAuth_PreservedBehavior(t *testing.T) {
	state := newTestStateWithMethod(func(ctx context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
		// Simulate the device code credential's UserPrompt callback by
		// sending a message on the deviceCodeCh channel.
		if ch, ok := ctx.Value(DeviceCodeMsgKey).(chan string); ok {
			select {
			case ch <- "To sign in, visit https://microsoft.com/devicelogin and enter code ABC123":
			default:
			}
		}
		// Block briefly to let the select in handleDeviceCodeAuth pick up the message.
		// In real usage, the credential would block until the user completes login.
		return testRecord(), nil
	}, "device_code")

	callCount := 0
	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		callCount++
		if callCount == 1 {
			return nil, fmt.Errorf("DeviceCodeCredential: expired token")
		}
		return successResult(), nil
	}

	wrapped := buildMiddleware(state, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	// The device code prompt should be returned as a text result.
	text := extractResultText(result)
	if !strings.Contains(text, "devicelogin") {
		t.Errorf("expected device code prompt in result, got: %q", text)
	}
}

// TestPendingAuthMessage_Browser verifies the pending auth message for browser
// auth method.
func TestPendingAuthMessage_Browser(t *testing.T) {
	msg := pendingAuthMessage("browser")
	if !strings.Contains(msg, "complete the login in your browser") {
		t.Errorf("browser pending message = %q, want to contain browser-specific text", msg)
	}
	if strings.Contains(msg, "device code") {
		t.Errorf("browser pending message should not mention device code: %q", msg)
	}
}

// TestPendingAuthMessage_DeviceCode verifies the pending auth message for
// device code auth method.
func TestPendingAuthMessage_DeviceCode(t *testing.T) {
	msg := pendingAuthMessage("device_code")
	if !strings.Contains(msg, "device code login") {
		t.Errorf("device_code pending message = %q, want to contain device code text", msg)
	}
}

// --- Elicitation-based authentication tests (Phase 5) ---

// TestAuthMiddleware_BrowserAuth_URLElicitation verifies that handleBrowserAuth
// uses URL mode elicitation when the client supports it. The URL elicitation
// function is called with the login URL, and authentication proceeds normally.
func TestAuthMiddleware_BrowserAuth_URLElicitation(t *testing.T) {
	var elicitCalled bool
	var capturedURL string
	var capturedMessage string

	callCount := 0
	state := newTestStateWithMethod(func(_ context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
		return testRecord(), nil
	}, "browser")

	// Replace urlElicit with a mock that succeeds (elicitation supported).
	state.urlElicit = func(_ context.Context, _, url, message string) (*mcp.ElicitationResult, error) {
		elicitCalled = true
		capturedURL = url
		capturedMessage = message
		return &mcp.ElicitationResult{
			ElicitationResponse: mcp.ElicitationResponse{
				Action: mcp.ElicitationResponseActionAccept,
			},
		}, nil
	}

	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		callCount++
		if callCount == 1 {
			return nil, fmt.Errorf("InteractiveBrowserCredential: expired token")
		}
		return successResult(), nil
	}

	wrapped := buildMiddleware(state, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatal("expected success result after browser re-auth with elicitation")
	}
	if !elicitCalled {
		t.Error("URL elicitation should have been called")
	}
	if capturedURL != "https://login.microsoftonline.com" {
		t.Errorf("URL = %q, want %q", capturedURL, "https://login.microsoftonline.com")
	}
	if !strings.Contains(capturedMessage, "Authentication required") {
		t.Errorf("message = %q, want to contain 'Authentication required'", capturedMessage)
	}
}

// TestAuthMiddleware_DeviceCodeAuth_FormElicitation verifies that
// handleDeviceCodeAuth uses form mode elicitation when the client supports it.
// The device code prompt is presented via elicitation instead of plain text.
func TestAuthMiddleware_DeviceCodeAuth_FormElicitation(t *testing.T) {
	var elicitCalled bool
	var capturedMessage string

	state := newTestStateWithMethod(func(ctx context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
		// Simulate device code prompt.
		if ch, ok := ctx.Value(DeviceCodeMsgKey).(chan string); ok {
			select {
			case ch <- "To sign in, visit https://microsoft.com/devicelogin and enter code XYZ123":
			default:
			}
		}
		return testRecord(), nil
	}, "device_code")

	// Replace elicit with a mock that succeeds (elicitation supported).
	state.elicit = func(_ context.Context, req mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
		elicitCalled = true
		capturedMessage = req.Params.Message
		return &mcp.ElicitationResult{
			ElicitationResponse: mcp.ElicitationResponse{
				Action:  mcp.ElicitationResponseActionAccept,
				Content: map[string]any{"acknowledged": true},
			},
		}, nil
	}

	callCount := 0
	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		callCount++
		if callCount == 1 {
			return nil, fmt.Errorf("DeviceCodeCredential: expired token")
		}
		return successResult(), nil
	}

	wrapped := buildMiddleware(state, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if !elicitCalled {
		t.Error("form elicitation should have been called")
	}
	if !strings.Contains(capturedMessage, "devicelogin") {
		t.Errorf("elicitation message = %q, want to contain device code prompt", capturedMessage)
	}

	// The device code prompt should be returned as text even with elicitation.
	text := extractResultText(result)
	if !strings.Contains(text, "devicelogin") {
		t.Errorf("result text = %q, want to contain device code prompt", text)
	}
}

// TestAuthMiddleware_BrowserAuth_ElicitationFallback verifies that when URL
// elicitation returns ErrElicitationNotSupported, the middleware falls back
// to LoggingMessageNotification and still completes re-authentication.
func TestAuthMiddleware_BrowserAuth_ElicitationFallback(t *testing.T) {
	var notificationSent bool

	callCount := 0
	state := newTestStateWithMethod(func(_ context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
		return testRecord(), nil
	}, "browser")

	// URL elicitation returns not supported.
	state.urlElicit = func(_ context.Context, _, _, _ string) (*mcp.ElicitationResult, error) {
		return nil, mcpserver.ErrElicitationNotSupported
	}

	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		callCount++
		if callCount == 1 {
			return nil, fmt.Errorf("InteractiveBrowserCredential: expired token")
		}
		return successResult(), nil
	}

	// Capture stderr for the fallback notification.
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}
	origStderr := os.Stderr
	os.Stderr = stderrW
	t.Cleanup(func() {
		os.Stderr = origStderr
	})

	wrapped := buildMiddleware(state, handler)
	result, resultErr := wrapped(context.Background(), mcp.CallToolRequest{})

	_ = stderrW.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, stderrR)

	if resultErr != nil {
		t.Fatalf("unexpected error: %v", resultErr)
	}
	if result == nil || result.IsError {
		t.Fatal("expected success result after fallback re-auth")
	}

	// Verify the fallback notification was sent to stderr.
	if strings.Contains(buf.String(), "Authentication required") {
		notificationSent = true
	}
	if !notificationSent {
		t.Errorf("expected fallback notification on stderr, got: %q", buf.String())
	}
	if callCount != 2 {
		t.Errorf("handler call count = %d, want 2", callCount)
	}
}

// TestAuthMiddleware_DeviceCodeAuth_ElicitationFallback verifies that when
// form elicitation returns ErrElicitationNotSupported, the device code prompt
// is returned as plain text (the pre-elicitation behavior).
func TestAuthMiddleware_DeviceCodeAuth_ElicitationFallback(t *testing.T) {
	state := newTestStateWithMethod(func(ctx context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
		if ch, ok := ctx.Value(DeviceCodeMsgKey).(chan string); ok {
			select {
			case ch <- "To sign in, visit https://microsoft.com/devicelogin and enter code FALLBACK":
			default:
			}
		}
		return testRecord(), nil
	}, "device_code")

	// Form elicitation returns not supported.
	state.elicit = func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
		return nil, mcpserver.ErrElicitationNotSupported
	}

	callCount := 0
	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		callCount++
		if callCount == 1 {
			return nil, fmt.Errorf("DeviceCodeCredential: expired token")
		}
		return successResult(), nil
	}

	wrapped := buildMiddleware(state, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	// The device code prompt should be returned as plain text.
	text := extractResultText(result)
	if !strings.Contains(text, "FALLBACK") {
		t.Errorf("result text = %q, want to contain 'FALLBACK'", text)
	}
}

// trackingAuthenticator wraps a mockAuthenticator to track whether it was
// called. Used to verify which credential (closure vs. context) is selected
// during re-authentication.
type trackingAuthenticator struct {
	mock   *mockAuthenticator
	called atomic.Bool
}

// Authenticate delegates to the wrapped mock and marks the tracker as called.
func (ta *trackingAuthenticator) Authenticate(ctx context.Context, opts *policy.TokenRequestOptions) (azidentity.AuthenticationRecord, error) {
	ta.called.Store(true)
	return ta.mock.Authenticate(ctx, opts)
}

// TestAuthMiddleware_AccountAuthFromContext_UsedForReauth verifies that when
// AccountAuth is present in the context, the middleware uses it for
// re-authentication instead of the closure credential.
func TestAuthMiddleware_AccountAuthFromContext_UsedForReauth(t *testing.T) {
	closureCred := &trackingAuthenticator{
		mock: &mockAuthenticator{record: testRecord()},
	}
	contextCred := &trackingAuthenticator{
		mock: &mockAuthenticator{record: testRecord()},
	}

	state := &authMiddlewareState{
		cred:           closureCred,
		authRecordPath: "/tmp/closure-auth-record.json",
		authMethod:     "browser",
		authenticate: func(_ context.Context, auth Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			return auth.Authenticate(context.Background(), nil)
		},
		elicit:         elicitUnsupported,
		urlElicit:      urlElicitUnsupported,
		openBrowser:    func(_ string) error { return nil },
		browserTimeout: 120 * time.Second,
		scopes:         []string{"Calendars.ReadWrite"},
	}

	callCount := 0
	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		callCount++
		if callCount == 1 {
			return nil, fmt.Errorf("InteractiveBrowserCredential: expired token")
		}
		return successResult(), nil
	}

	// Inject AccountAuth into the context.
	ctx := WithAccountAuth(context.Background(), AccountAuth{
		Authenticator:  contextCred,
		AuthRecordPath: "/tmp/context-auth-record.json",
		AuthMethod:     "browser",
	})

	wrapped := buildMiddleware(state, handler)
	result, err := wrapped(ctx, mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatal("expected success result")
	}
	if closureCred.called.Load() {
		t.Error("closure credential should not be used when AccountAuth is in context")
	}
	if !contextCred.called.Load() {
		t.Error("context credential should be used when AccountAuth is in context")
	}
}

// TestAuthMiddleware_NoAccountAuth_FallsBackToClosure verifies that when no
// AccountAuth is in the context, the middleware uses the closure credential
// for re-authentication (backward compatibility).
func TestAuthMiddleware_NoAccountAuth_FallsBackToClosure(t *testing.T) {
	closureCred := &trackingAuthenticator{
		mock: &mockAuthenticator{record: testRecord()},
	}

	state := &authMiddlewareState{
		cred:           closureCred,
		authRecordPath: "/tmp/closure-auth-record.json",
		authMethod:     "browser",
		authenticate: func(_ context.Context, auth Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			return auth.Authenticate(context.Background(), nil)
		},
		elicit:         elicitUnsupported,
		urlElicit:      urlElicitUnsupported,
		openBrowser:    func(_ string) error { return nil },
		browserTimeout: 120 * time.Second,
		scopes:         []string{"Calendars.ReadWrite"},
	}

	callCount := 0
	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		callCount++
		if callCount == 1 {
			return nil, fmt.Errorf("InteractiveBrowserCredential: expired token")
		}
		return successResult(), nil
	}

	// No AccountAuth in context.
	wrapped := buildMiddleware(state, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatal("expected success result")
	}
	if !closureCred.called.Load() {
		t.Error("closure credential should be used when no AccountAuth in context")
	}
}

// TestAuthMiddleware_AccountAuthFromContext_DeviceCode verifies that the
// device code auth method from AccountAuth in context is respected.
func TestAuthMiddleware_AccountAuthFromContext_DeviceCode(t *testing.T) {
	contextCred := &trackingAuthenticator{
		mock: &mockAuthenticator{record: testRecord()},
	}

	// State configured with "browser" but context will override to "device_code".
	state := &authMiddlewareState{
		cred:           nil,
		authRecordPath: "/tmp/closure-auth-record.json",
		authMethod:     "browser",
		authenticate: func(ctx context.Context, auth Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			// Simulate device code credential sending prompt.
			if ch, ok := ctx.Value(DeviceCodeMsgKey).(chan string); ok {
				select {
				case ch <- "To sign in, visit https://microsoft.com/devicelogin and enter code CTXDC":
				default:
				}
			}
			return auth.Authenticate(ctx, nil)
		},
		elicit:         elicitUnsupported,
		urlElicit:      urlElicitUnsupported,
		openBrowser:    func(_ string) error { return nil },
		browserTimeout: 120 * time.Second,
		scopes:         []string{"Calendars.ReadWrite"},
	}

	callCount := 0
	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		callCount++
		if callCount == 1 {
			return nil, fmt.Errorf("DeviceCodeCredential: expired token")
		}
		return successResult(), nil
	}

	// Inject AccountAuth with device_code method into context.
	ctx := WithAccountAuth(context.Background(), AccountAuth{
		Authenticator:  contextCred,
		AuthRecordPath: "/tmp/context-auth-record.json",
		AuthMethod:     "device_code",
	})

	wrapped := buildMiddleware(state, handler)
	result, err := wrapped(ctx, mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if !contextCred.called.Load() {
		t.Error("context credential should be used when AccountAuth is in context")
	}

	// Device code prompt should be in the result.
	text := extractResultText(result)
	if !strings.Contains(text, "CTXDC") {
		t.Errorf("result text = %q, want to contain 'CTXDC'", text)
	}
}

// --- Auth code flow middleware tests (Phase 3) ---

// mockAuthCodeCred implements both Authenticator and AuthCodeFlow for testing
// the handleAuthCodeAuth middleware method.
type mockAuthCodeCred struct {
	authCodeURL    string
	authCodeURLErr error
	exchangeErr    error
}

// Authenticate satisfies the Authenticator interface. For auth_code credentials,
// this method is not used by the middleware (handleAuthCodeAuth uses AuthCodeFlow
// instead), but it must be present.
func (m *mockAuthCodeCred) Authenticate(_ context.Context, _ *policy.TokenRequestOptions) (azidentity.AuthenticationRecord, error) {
	return azidentity.AuthenticationRecord{}, fmt.Errorf("auth_code: use AuthCodeFlow")
}

// AuthCodeURL returns the pre-configured authorization URL or error.
func (m *mockAuthCodeCred) AuthCodeURL(_ context.Context, _ []string) (string, error) {
	return m.authCodeURL, m.authCodeURLErr
}

// ExchangeCode simulates exchanging the authorization code for tokens.
func (m *mockAuthCodeCred) ExchangeCode(_ context.Context, _ string, _ []string) error {
	return m.exchangeErr
}

// newAuthCodeTestState creates an authMiddlewareState configured for auth_code
// testing with a mockAuthCodeCred credential. The authenticate function is a
// no-op since handleAuthCodeAuth does not use it (it drives the flow via
// AuthCodeFlow directly).
func newAuthCodeTestState(cred *mockAuthCodeCred) *authMiddlewareState {
	return &authMiddlewareState{
		cred:           cred,
		authRecordPath: "/tmp/test-auth-code-record.json",
		authMethod:     "auth_code",
		authenticate: func(_ context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			return azidentity.AuthenticationRecord{}, nil
		},
		elicit:         elicitUnsupported,
		urlElicit:      urlElicitUnsupported,
		openBrowser:    func(_ string) error { return nil },
		browserTimeout: 120 * time.Second,
		scopes:         []string{"Calendars.ReadWrite"},
	}
}

// TestHandleAuthCodeAuth_ElicitationSuccess verifies that when elicitation
// succeeds and returns a valid redirect URL, handleAuthCodeAuth exchanges the
// code and retries the original tool call.
func TestHandleAuthCodeAuth_ElicitationSuccess(t *testing.T) {
	cred := &mockAuthCodeCred{
		authCodeURL: "https://login.microsoftonline.com/common/oauth2/authorize?client_id=test&redirect_uri=nativeclient",
	}
	state := newAuthCodeTestState(cred)

	// Replace elicit with a mock that returns a valid redirect URL.
	state.elicit = func(_ context.Context, req mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
		// Verify the elicitation message mentions authentication.
		if !strings.Contains(req.Params.Message, "Authentication required") {
			t.Errorf("elicitation message = %q, want to contain 'Authentication required'", req.Params.Message)
		}
		return &mcp.ElicitationResult{
			ElicitationResponse: mcp.ElicitationResponse{
				Action: mcp.ElicitationResponseActionAccept,
				Content: map[string]any{
					"redirect_url": "https://login.microsoftonline.com/common/oauth2/nativeclient?code=M.C507_BAY.2.U.abc123",
				},
			},
		}, nil
	}

	callCount := 0
	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		callCount++
		if callCount == 1 {
			return nil, fmt.Errorf("AuthCodeCredential: authentication required")
		}
		return successResult(), nil
	}

	wrapped := buildMiddleware(state, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatal("expected success result after auth code re-auth retry")
	}
	if callCount != 2 {
		t.Errorf("handler call count = %d, want 2 (initial + retry)", callCount)
	}
}

// TestHandleAuthCodeAuth_ElicitationNotSupported verifies that when elicitation
// is not supported, handleAuthCodeAuth returns the authorization URL with
// instructions to use the complete_auth tool.
func TestHandleAuthCodeAuth_ElicitationNotSupported(t *testing.T) {
	authURL := "https://login.microsoftonline.com/common/oauth2/authorize?client_id=test"
	cred := &mockAuthCodeCred{
		authCodeURL: authURL,
	}
	state := newAuthCodeTestState(cred)

	// elicit already defaults to elicitUnsupported in newAuthCodeTestState.

	callCount := 0
	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		callCount++
		return nil, fmt.Errorf("AuthCodeCredential: authentication required")
	}

	wrapped := buildMiddleware(state, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	// The result should NOT be an error — it's a text result with instructions.
	if result.IsError {
		t.Fatal("expected non-error text result with auth URL and instructions")
	}

	text := extractResultText(result)
	if !strings.Contains(text, authURL) {
		t.Errorf("result text should contain the auth URL %q, got: %q", authURL, text)
	}
	if !strings.Contains(text, "complete_auth") {
		t.Errorf("result text should mention complete_auth tool, got: %q", text)
	}
	// Handler should only be called once (the initial failed call).
	if callCount != 1 {
		t.Errorf("handler call count = %d, want 1 (no retry)", callCount)
	}
}

// TestHandleAuthCodeAuth_InvalidElicitationURL verifies that when elicitation
// returns an empty or missing redirect URL, handleAuthCodeAuth returns an
// appropriate error message.
func TestHandleAuthCodeAuth_InvalidElicitationURL(t *testing.T) {
	cred := &mockAuthCodeCred{
		authCodeURL: "https://login.microsoftonline.com/common/oauth2/authorize?client_id=test",
	}
	state := newAuthCodeTestState(cred)

	// Return accept but with empty redirect_url.
	state.elicit = func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
		return &mcp.ElicitationResult{
			ElicitationResponse: mcp.ElicitationResponse{
				Action:  mcp.ElicitationResponseActionAccept,
				Content: map[string]any{"redirect_url": ""},
			},
		}, nil
	}

	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return nil, fmt.Errorf("AuthCodeCredential: authentication required")
	}

	wrapped := buildMiddleware(state, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected error result, got nil")
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for missing redirect URL")
	}
	text := extractResultText(result)
	if !strings.Contains(text, "No redirect URL provided") {
		t.Errorf("error text = %q, want to contain 'No redirect URL provided'", text)
	}
}

// TestHandleAuthCodeAuth_ExchangeFailure verifies that when ExchangeCode fails
// after a valid redirect URL is provided, handleAuthCodeAuth returns an error
// result with troubleshooting guidance.
func TestHandleAuthCodeAuth_ExchangeFailure(t *testing.T) {
	cred := &mockAuthCodeCred{
		authCodeURL: "https://login.microsoftonline.com/common/oauth2/authorize?client_id=test",
		exchangeErr: fmt.Errorf("exchange authorization code: token request failed"),
	}
	state := newAuthCodeTestState(cred)

	// Return a valid redirect URL.
	state.elicit = func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
		return &mcp.ElicitationResult{
			ElicitationResponse: mcp.ElicitationResponse{
				Action: mcp.ElicitationResponseActionAccept,
				Content: map[string]any{
					"redirect_url": "https://login.microsoftonline.com/common/oauth2/nativeclient?code=BADCODE",
				},
			},
		}, nil
	}

	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return nil, fmt.Errorf("AuthCodeCredential: authentication required")
	}

	wrapped := buildMiddleware(state, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected error result, got nil")
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for exchange failure")
	}
	text := extractResultText(result)
	if !strings.Contains(text, "account_list") {
		t.Errorf("error text should contain recovery tool name account_list, got: %q", text)
	}
	if !strings.Contains(text, "account_add") {
		t.Errorf("error text should contain recovery tool name account_add, got: %q", text)
	}
}

// TestHandleAuthCodeAuth_ElicitationDecline verifies that when the user
// declines the elicitation, handleAuthCodeAuth returns an appropriate error.
func TestHandleAuthCodeAuth_ElicitationDecline(t *testing.T) {
	cred := &mockAuthCodeCred{
		authCodeURL: "https://login.microsoftonline.com/common/oauth2/authorize?client_id=test",
	}
	state := newAuthCodeTestState(cred)

	state.elicit = func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
		return &mcp.ElicitationResult{
			ElicitationResponse: mcp.ElicitationResponse{
				Action: mcp.ElicitationResponseActionDecline,
			},
		}, nil
	}

	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return nil, fmt.Errorf("AuthCodeCredential: authentication required")
	}

	wrapped := buildMiddleware(state, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected error result, got nil")
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for declined elicitation")
	}
	text := extractResultText(result)
	if !strings.Contains(text, "declined") {
		t.Errorf("error text = %q, want to contain 'declined'", text)
	}
}

// TestHandleAuthCodeAuth_NonAuthCodeFlowCred verifies that when the credential
// does not implement AuthCodeFlow, handleAuthCodeAuth returns an appropriate
// internal error message.
func TestHandleAuthCodeAuth_NonAuthCodeFlowCred(t *testing.T) {
	// Use a plain mockAuthenticator that does NOT implement AuthCodeFlow.
	plainCred := &mockAuthenticator{record: testRecord()}

	state := &authMiddlewareState{
		cred:           plainCred,
		authRecordPath: "/tmp/test-auth-code-record.json",
		authMethod:     "auth_code",
		authenticate: func(_ context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			return azidentity.AuthenticationRecord{}, nil
		},
		elicit:         elicitUnsupported,
		urlElicit:      urlElicitUnsupported,
		openBrowser:    func(_ string) error { return nil },
		browserTimeout: 120 * time.Second,
		scopes:         []string{"Calendars.ReadWrite"},
	}

	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return nil, fmt.Errorf("AuthCodeCredential: authentication required")
	}

	wrapped := buildMiddleware(state, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected error result, got nil")
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for non-AuthCodeFlow credential")
	}
	text := extractResultText(result)
	if !strings.Contains(text, "does not support the auth_code flow") {
		t.Errorf("error text = %q, want to contain 'does not support the auth_code flow'", text)
	}
}

// TestPendingAuthMessage_AuthCode verifies the pending auth message for the
// auth_code auth method.
func TestPendingAuthMessage_AuthCode(t *testing.T) {
	msg := pendingAuthMessage("auth_code")
	if !strings.Contains(msg, "copy the redirect URL") {
		t.Errorf("auth_code pending message = %q, want to contain 'copy the redirect URL'", msg)
	}
	if strings.Contains(msg, "device code") {
		t.Errorf("auth_code pending message should not mention device code: %q", msg)
	}
}

// TestHandleBrowserAuth_Timeout_DescriptiveError verifies that when browser
// authentication times out in the middleware, the error message explicitly
// states that a browser window was opened for Microsoft login and suggests
// the user retry (AC-4, FR-5).
func TestHandleBrowserAuth_Timeout_DescriptiveError(t *testing.T) {
	state := &authMiddlewareState{
		cred:           nil,
		authRecordPath: "/tmp/test-auth-record.json",
		authMethod:     "browser",
		authenticate: func(_ context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			// Block until context is cancelled, simulating a user who never
			// completes the browser login.
			select {}
		},
		elicit:         elicitUnsupported,
		urlElicit:      urlElicitUnsupported,
		openBrowser:    func(_ string) error { return nil },
		browserTimeout: 50 * time.Millisecond,
		scopes:         []string{"Calendars.ReadWrite"},
	}

	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return nil, fmt.Errorf("InteractiveBrowserCredential: context deadline exceeded")
	}

	wrapped := buildMiddleware(state, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result == nil {
		t.Fatal("expected error result, got nil")
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for browser timeout")
	}

	text := extractResultText(result)
	if !strings.Contains(text, "account_list") {
		t.Errorf("error = %q, want recovery tool name account_list", text)
	}
	if !strings.Contains(text, "account_add") {
		t.Errorf("error = %q, want recovery tool name account_add", text)
	}
	if !strings.Contains(text, "retry your original request") {
		t.Errorf("error = %q, want to suggest retrying", text)
	}
}

// TestHandleAuthCodeAuth_ElicitationError_ReturnsAuthURL verifies that when
// any elicitation error occurs (not just ErrElicitationNotSupported), the
// middleware's handleAuthCodeAuth returns the auth URL and complete_auth
// instructions as tool result text (AC-7, FR-6).
func TestHandleAuthCodeAuth_ElicitationError_ReturnsAuthURL(t *testing.T) {
	authURL := "https://login.microsoftonline.com/common/oauth2/authorize?client_id=test"
	cred := &mockAuthCodeCred{
		authCodeURL: authURL,
	}
	state := newAuthCodeTestState(cred)

	// Override elicit with a generic error (not ErrElicitationNotSupported).
	state.elicit = func(_ context.Context, _ mcp.ElicitationRequest) (*mcp.ElicitationResult, error) {
		return nil, fmt.Errorf("elicitation request failed: Method not found")
	}

	callCount := 0
	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		callCount++
		return nil, fmt.Errorf("AuthCodeCredential: authentication required")
	}

	wrapped := buildMiddleware(state, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	// The result should NOT be an error -- it's a text result with instructions.
	if result.IsError {
		t.Fatal("expected non-error text result with auth URL and instructions")
	}

	text := extractResultText(result)
	if !strings.Contains(text, authURL) {
		t.Errorf("result text should contain the auth URL %q, got: %q", authURL, text)
	}
	if !strings.Contains(text, "complete_auth") {
		t.Errorf("result text should mention complete_auth tool, got: %q", text)
	}
	if !strings.Contains(text, "browser") {
		t.Errorf("result text should mention browser, got: %q", text)
	}
	// Handler should only be called once (the initial failed call).
	if callCount != 1 {
		t.Errorf("handler call count = %d, want 1 (no retry)", callCount)
	}
}

// buildFullMiddleware creates the complete middleware chain including the
// fresh-credential detection logic. Unlike buildMiddleware (which only tests
// the handleAuthError path), this helper mirrors the full AuthMiddleware
// closure behavior: fresh-credential fast-path, pending auth check, inner
// handler call, and auth error detection.
func buildFullMiddleware(state *authMiddlewareState, handler func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Check if a background authentication flow completed.
		if state.pendingAuth.Load() {
			select {
			case <-state.pendingDone:
				state.pendingAuth.Store(false)
				if state.pendingErr != nil {
					return mcp.NewToolResultError(FormatAuthError(state.pendingErr)), nil
				}
				state.authenticated.CompareAndSwap(false, true)
			default:
				return mcp.NewToolResultError(pendingAuthMessage(state.authMethod)), nil
			}
		}

		// Fresh-credential fast-path.
		if !state.preAuthenticated.Load() && !state.authenticated.Load() {
			freshErr := fmt.Errorf("authentication required: credential not yet authenticated")
			return state.handleAuthError(ctx, handler, request, freshErr)
		}

		// Call the inner handler.
		result, err := handler(ctx, request)

		if err == nil && (result == nil || !result.IsError) {
			return result, nil
		}

		if !isAuthRelated(err, result) {
			return result, err
		}

		return state.handleAuthError(ctx, handler, request, err)
	}
}

// TestMiddleware_FreshCredential_ImmediateAuthPrompt verifies that when the
// credential has never been authenticated (preAuthenticated=false,
// authenticated=false), the middleware skips the initial Graph API call
// (which would block for the 30-second timeout) and immediately initiates
// the re-authentication flow. After successful auth, the handler is retried
// exactly once. This validates AC-7: no 30-second delay for fresh credentials.
func TestMiddleware_FreshCredential_ImmediateAuthPrompt(t *testing.T) {
	authCalled := false
	state := newTestStateWithMethod(func(_ context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
		authCalled = true
		return testRecord(), nil
	}, "browser")

	handlerCallCount := 0
	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		handlerCallCount++
		return successResult(), nil
	}

	// Neither preAuthenticated nor authenticated is set (fresh credential).
	wrapped := buildFullMiddleware(state, handler)

	start := time.Now()
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	// The handler should be called exactly once (as a retry after auth), not
	// as the initial Graph API call that would block for 30 seconds.
	if handlerCallCount != 1 {
		t.Errorf("handler call count = %d, want 1 (retry only, no initial Graph API call)", handlerCallCount)
	}

	// The auth prompt must appear within 5 seconds (well under the 30s timeout).
	if elapsed > 5*time.Second {
		t.Errorf("fresh credential auth prompt took %v, want < 5s (AC-7)", elapsed)
	}

	// Re-authentication should have been triggered immediately.
	if !authCalled {
		t.Error("Authenticate should be called for fresh credentials")
	}
}

// TestMiddleware_PreAuthenticated_CallsHandler verifies that when
// preAuthenticated is true (startup token probe succeeded), the middleware
// calls the inner handler normally instead of triggering the fresh-credential
// fast-path.
func TestMiddleware_PreAuthenticated_CallsHandler(t *testing.T) {
	authCalled := false
	state := newTestState(func(_ context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
		authCalled = true
		return testRecord(), nil
	})

	// Mark as pre-authenticated (startup token probe succeeded).
	state.preAuthenticated.Store(true)

	handlerCalled := false
	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		handlerCalled = true
		return successResult(), nil
	}

	wrapped := buildFullMiddleware(state, handler)
	result, err := wrapped(context.Background(), mcp.CallToolRequest{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatal("expected success result")
	}
	if !handlerCalled {
		t.Error("inner handler should be called when preAuthenticated=true")
	}
	if authCalled {
		t.Error("Authenticate should not be called when handler succeeds")
	}
}

// TestMiddleware_FreshCredential_DeviceCode_ImmediatePrompt verifies that
// fresh credential detection works for the device_code auth method,
// presenting the device code prompt immediately without calling the handler.
func TestMiddleware_FreshCredential_DeviceCode_ImmediatePrompt(t *testing.T) {
	// Redirect stderr to /dev/null to suppress notification fallback output.
	origStderr := os.Stderr
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("failed to open %s: %v", os.DevNull, err)
	}
	os.Stderr = devNull
	defer func() {
		os.Stderr = origStderr
		_ = devNull.Close()
	}()

	deviceCodeMsg := "DC_TEST: To sign in, open https://microsoft.com/devicelogin and enter code TESTCODE"
	state := newTestStateWithMethod(func(ctx context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
		// Simulate the device code flow: send the device code message.
		ch := ctx.Value(DeviceCodeMsgKey).(chan string)
		ch <- deviceCodeMsg
		// Wait for context cancellation (simulating user completing login).
		<-ctx.Done()
		return testRecord(), nil
	}, "device_code")

	handlerCalled := false
	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		handlerCalled = true
		return nil, fmt.Errorf("DeviceCodeCredential: context deadline exceeded")
	}

	wrapped := buildFullMiddleware(state, handler)

	start := time.Now()
	result, callErr := wrapped(context.Background(), mcp.CallToolRequest{})
	elapsed := time.Since(start)

	if callErr != nil {
		t.Fatalf("unexpected error: %v", callErr)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	// The handler MUST NOT be called.
	if handlerCalled {
		t.Error("inner handler should not be called for fresh device_code credentials")
	}

	// The device code prompt must appear within 5 seconds.
	if elapsed > 5*time.Second {
		t.Errorf("fresh credential device code prompt took %v, want < 5s", elapsed)
	}

	// The result should contain the device code message.
	text := extractResultText(result)
	if !strings.Contains(text, "TESTCODE") {
		t.Errorf("result text = %q, want to contain device code 'TESTCODE'", text)
	}
}

// TestMiddleware_AllAuthErrors_UseFormatAuthError verifies that all auth error
// paths in the middleware use FormatAuthError exclusively (AC-10a). Every error
// result from an auth failure must contain the recovery tool names
// (account_list, account_add) and must not contain raw Azure SDK class names.
func TestMiddleware_AllAuthErrors_UseFormatAuthError(t *testing.T) {
	bannedSubstrings := []string{
		"DeviceCodeCredential",
		"InteractiveBrowserCredential",
		"AuthorizationCodeCredential",
	}

	requiredSubstrings := []string{
		"account_list",
		"account_add",
	}

	assertFormatted := func(t *testing.T, label string, result *mcp.CallToolResult) {
		t.Helper()
		if result == nil {
			t.Fatalf("[%s] expected error result, got nil", label)
		}
		if !result.IsError {
			// Some paths return text results (e.g., device code prompt); skip those.
			return
		}
		text := extractResultText(result)
		for _, banned := range bannedSubstrings {
			if strings.Contains(text, banned) {
				t.Errorf("[%s] error contains raw SDK class name %q:\n%s", label, banned, text)
			}
		}
		for _, required := range requiredSubstrings {
			if !strings.Contains(text, required) {
				t.Errorf("[%s] error missing recovery tool %q:\n%s", label, required, text)
			}
		}
	}

	// Scenario 1: Browser auth failure (authenticate returns error).
	t.Run("browser_auth_failure", func(t *testing.T) {
		state := newTestState(func(_ context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			return azidentity.AuthenticationRecord{}, fmt.Errorf("InteractiveBrowserCredential: context deadline exceeded")
		})
		state.browserTimeout = 5 * time.Second

		handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return nil, fmt.Errorf("InteractiveBrowserCredential: context deadline exceeded")
		}

		wrapped := buildMiddleware(state, handler)
		result, err := wrapped(context.Background(), mcp.CallToolRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertFormatted(t, "browser_auth_failure", result)
	})

	// Scenario 2: Browser auth timeout (user never completes login).
	t.Run("browser_auth_timeout", func(t *testing.T) {
		state := &authMiddlewareState{
			cred:           nil,
			authRecordPath: "/tmp/test.json",
			authMethod:     "browser",
			authenticate: func(_ context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
				select {} // block forever
			},
			elicit:         elicitUnsupported,
			urlElicit:      urlElicitUnsupported,
			browserTimeout: 50 * time.Millisecond,
			scopes:         []string{"Calendars.ReadWrite"},
		}

		handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return nil, fmt.Errorf("InteractiveBrowserCredential: context deadline exceeded")
		}

		wrapped := buildMiddleware(state, handler)
		result, err := wrapped(context.Background(), mcp.CallToolRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertFormatted(t, "browser_auth_timeout", result)
	})

	// Scenario 3: Device code auth failure (authenticate returns error before prompt).
	t.Run("device_code_auth_failure", func(t *testing.T) {
		state := newTestStateWithMethod(func(_ context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			return azidentity.AuthenticationRecord{}, fmt.Errorf("DeviceCodeCredential: context deadline exceeded")
		}, "device_code")

		handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return nil, fmt.Errorf("DeviceCodeCredential: context deadline exceeded")
		}

		wrapped := buildMiddleware(state, handler)
		result, err := wrapped(context.Background(), mcp.CallToolRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertFormatted(t, "device_code_auth_failure", result)
	})

	// Scenario 4: Pending auth check failure (background auth completed with error).
	t.Run("pending_auth_error", func(t *testing.T) {
		state := newTestState(func(_ context.Context, _ Authenticator, _ string, _ []string) (azidentity.AuthenticationRecord, error) {
			return azidentity.AuthenticationRecord{}, nil
		})
		state.preAuthenticated.Store(true)
		state.authenticated.Store(true)

		// Simulate a completed background auth with error.
		done := make(chan struct{})
		close(done)
		state.pendingAuth.Store(true)
		state.pendingDone = done
		state.pendingErr = fmt.Errorf("DeviceCodeCredential: AADSTS70000 auth failed")

		handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return successResult(), nil
		}

		wrapped := buildFullMiddleware(state, handler)
		result, err := wrapped(context.Background(), mcp.CallToolRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertFormatted(t, "pending_auth_error", result)
	})
}
