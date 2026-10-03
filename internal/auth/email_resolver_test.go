package auth

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	kiotaauth "github.com/microsoft/kiota-abstractions-go/authentication"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
)

// TestEnsureEmail_NilClient verifies that EnsureEmail does nothing when
// the account entry has no Graph client.
func TestEnsureEmail_NilClient(t *testing.T) {
	entry := &AccountEntry{Label: "test", Client: nil}
	EnsureEmail(context.Background(), entry)
	if entry.Email != "" {
		t.Errorf("expected empty email for nil client, got %q", entry.Email)
	}
}

// TestEnsureEmail_AlreadySet verifies that EnsureEmail skips the fetch when
// the email is already populated on the entry.
func TestEnsureEmail_AlreadySet(t *testing.T) {
	entry := &AccountEntry{
		Label:  "test",
		Client: &msgraphsdk.GraphServiceClient{},
		Email:  "existing@example.com",
	}
	EnsureEmail(context.Background(), entry)
	if entry.Email != "existing@example.com" {
		t.Errorf("email changed unexpectedly: got %q", entry.Email)
	}
}

// newEmailTestClient returns a Graph client whose requests reach handler, plus
// a pointer recording the last request path the fake server received.
func newEmailTestClient(t *testing.T, handler http.HandlerFunc) (*msgraphsdk.GraphServiceClient, *string) {
	t.Helper()
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	host := strings.TrimPrefix(srv.URL, "http://")
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme, r.URL.Host = "http", host
		return http.DefaultTransport.RoundTrip(r)
	})}
	adapter, err := msgraphsdk.NewGraphRequestAdapterWithParseNodeFactoryAndSerializationWriterFactoryAndHttpClient(
		&kiotaauth.AnonymousAuthenticationProvider{}, nil, nil, httpClient)
	if err != nil {
		t.Fatalf("create adapter: %v", err)
	}
	return msgraphsdk.NewGraphServiceClient(adapter), &path
}

// roundTripFunc adapts a function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip calls f.
func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// captureLogs redirects the default slog logger to a buffer for the test.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}

// TestEnsureEmail_ForbiddenLogsFix verifies that a 403 from GET /me, the
// response a client id without User.Read gets, is logged at error level with
// the fix instruction instead of leaving the email empty silently.
func TestEnsureEmail_ForbiddenLogsFix(t *testing.T) {
	logs := captureLogs(t)
	client, path := newEmailTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":"Authorization_RequestDenied","message":"denied"}}`))
	})
	entry := &AccountEntry{Label: "work", Client: client}
	EnsureEmail(context.Background(), entry)

	// The test adapter omits the middleware that rewrites the SDK's /me
	// placeholder, so either spelling proves the resolver called GET /me.
	if *path != "/v1.0/me" && *path != "/v1.0/users/me-token-to-replace" {
		t.Errorf("request path = %q, want GET /me", *path)
	}
	if entry.Email != "" {
		t.Errorf("Email = %q, want empty", entry.Email)
	}
	out := logs.String()
	if !strings.Contains(out, "level=ERROR") || !strings.Contains(out, "User.Read") {
		t.Errorf("log lacks error level or User.Read fix: %s", out)
	}
}

// TestEnsureEmail_MissingFieldsLogsFix verifies that a GET /me body with
// neither mail nor userPrincipalName is reported with a fix instruction.
func TestEnsureEmail_MissingFieldsLogsFix(t *testing.T) {
	logs := captureLogs(t)
	client, _ := newEmailTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"u1"}`))
	})
	entry := &AccountEntry{Label: "work", Client: client}
	EnsureEmail(context.Background(), entry)

	if entry.Email != "" {
		t.Errorf("Email = %q, want empty", entry.Email)
	}
	if out := logs.String(); !strings.Contains(out, "account email missing") || !strings.Contains(out, "fix=") {
		t.Errorf("log lacks missing-email fix: %s", out)
	}
}

// TestEnsureEmail_ResolvesMail verifies the success path sets Email and logs
// no error.
func TestEnsureEmail_ResolvesMail(t *testing.T) {
	logs := captureLogs(t)
	client, _ := newEmailTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"mail":"a@example.com"}`))
	})
	entry := &AccountEntry{Label: "work", Client: client}
	EnsureEmail(context.Background(), entry)

	if entry.Email != "a@example.com" {
		t.Errorf("Email = %q, want a@example.com", entry.Email)
	}
	if strings.Contains(logs.String(), "level=ERROR") {
		t.Errorf("unexpected error log: %s", logs.String())
	}
}
