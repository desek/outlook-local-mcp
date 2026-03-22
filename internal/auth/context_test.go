package auth

import (
	"context"
	"testing"

	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
)

func TestWithGraphClient_RoundTrip(t *testing.T) {
	client := &msgraphsdk.GraphServiceClient{}
	ctx := WithGraphClient(context.Background(), client)

	got, ok := GraphClientFromContext(ctx)
	if !ok {
		t.Fatal("expected ok=true, got false")
	}
	if got != client {
		t.Error("retrieved client does not match stored client")
	}
}

func TestGraphClientFromContext_MissingKey(t *testing.T) {
	got, ok := GraphClientFromContext(context.Background())
	if ok {
		t.Fatal("expected ok=false for missing key")
	}
	if got != nil {
		t.Error("expected nil client for missing key")
	}
}

func TestGraphClientFromContext_NilContext(t *testing.T) {
	//nolint:staticcheck // intentionally testing nil context behavior
	got, ok := GraphClientFromContext(nil)
	if ok {
		t.Fatal("expected ok=false for nil context")
	}
	if got != nil {
		t.Error("expected nil client for nil context")
	}
}

func TestWithAccountAuth_RoundTrip(t *testing.T) {
	auth := AccountAuth{
		AuthRecordPath: "/tmp/record.json",
		AuthMethod:     "browser",
	}
	ctx := WithAccountAuth(context.Background(), auth)

	got, ok := AccountAuthFromContext(ctx)
	if !ok {
		t.Fatal("expected ok=true, got false")
	}
	if got.AuthRecordPath != auth.AuthRecordPath {
		t.Errorf("AuthRecordPath = %q, want %q", got.AuthRecordPath, auth.AuthRecordPath)
	}
	if got.AuthMethod != auth.AuthMethod {
		t.Errorf("AuthMethod = %q, want %q", got.AuthMethod, auth.AuthMethod)
	}
}

func TestAccountAuthFromContext_MissingKey(t *testing.T) {
	got, ok := AccountAuthFromContext(context.Background())
	if ok {
		t.Fatal("expected ok=false for missing key")
	}
	if got.AuthRecordPath != "" || got.AuthMethod != "" {
		t.Error("expected zero-value AccountAuth for missing key")
	}
}
