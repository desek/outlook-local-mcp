package auth

import (
	"context"
	"testing"

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
