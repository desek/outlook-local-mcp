package tools

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/desek/outlook-local-mcp/internal/validate"
	"github.com/mark3labs/mcp-go/mcp"
)

// newMoveRequest builds a well-formed move_message request, so each test states
// only the argument it varies.
func newMoveRequest(args map[string]any) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	base := map[string]any{
		"message_id":            "msg-1",
		"destination_folder_id": "folder-archive",
	}
	for k, v := range args {
		base[k] = v
	}
	req.Params.Arguments = base
	return req
}

// TestMoveMessage_Success verifies that the destination is sent in the request
// body and that exactly one POST is issued, with no read-modify-write.
func TestMoveMessage_Success(t *testing.T) {
	rec := &patchRecorder{response: `{"id":"msg-2","subject":"Quarterly report"}`}
	client, srv := newTestGraphClient(t, rec)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleMoveMessage(graph.RetryConfig{}, 30*time.Second)
	result, err := handler(ctx, newMoveRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(t, result))
	}
	if len(rec.methods) != 1 || rec.methods[0] != http.MethodPost {
		t.Fatalf("expected exactly one POST and no verification GET, got %v", rec.methods)
	}
	// The pinned SDK writes the action parameter as "DestinationId", capitalised,
	// which was read from its generated serializer rather than assumed: asserting
	// the lower-camel spelling the Graph reference documents fails here. The
	// assertion is on what the SDK actually puts on the wire.
	if !strings.Contains(rec.bodies[0], `"DestinationId":"folder-archive"`) {
		t.Errorf("expected the body to carry the destination, got: %q", rec.bodies[0])
	}
}

// TestMoveMessage_ConfirmationNamesAllThreeIdentifiers verifies FR-4: the new
// identifier Graph minted, the original that no longer resolves, and the
// destination folder are all present, because a caller cannot observe any of
// the three otherwise.
func TestMoveMessage_ConfirmationNamesAllThreeIdentifiers(t *testing.T) {
	rec := &patchRecorder{response: `{"id":"msg-2","subject":"Quarterly report"}`}
	client, srv := newTestGraphClient(t, rec)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleMoveMessage(graph.RetryConfig{}, 30*time.Second)
	result, err := handler(ctx, newMoveRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(t, result)
	for _, want := range []string{"Quarterly report", "msg-2", "msg-1", "folder-archive", "no longer"} {
		if !strings.Contains(text, want) {
			t.Errorf("expected confirmation to name %q, got: %q", want, text)
		}
	}
	// The reported identifier is the one Graph returned, not the requested one.
	if !strings.Contains(text, "ID: msg-2") {
		t.Errorf("expected the new identifier on the ID line, got: %q", text)
	}
}

// TestMoveMessage_RequiresDestinationFolderID verifies that a missing
// destination is refused by name, and points at the verb that supplies one,
// before any Graph request is issued.
func TestMoveMessage_RequiresDestinationFolderID(t *testing.T) {
	rec := &patchRecorder{response: `{"id":"msg-2"}`}
	client, srv := newTestGraphClient(t, rec)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleMoveMessage(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"message_id": "msg-1"}
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error for a missing destination_folder_id")
	}
	text := resultText(t, result)
	for _, want := range []string{"destination_folder_id", "list_folders"} {
		if !strings.Contains(text, want) {
			t.Errorf("expected refusal to name %q, got: %q", want, text)
		}
	}
	if len(rec.methods) != 0 {
		t.Errorf("expected no Graph request, got %v", rec.methods)
	}
}

// TestMoveMessage_RequiresMessageID verifies that a missing message_id is
// refused by name before any Graph request is issued.
func TestMoveMessage_RequiresMessageID(t *testing.T) {
	rec := &patchRecorder{response: `{"id":"msg-2"}`}
	client, srv := newTestGraphClient(t, rec)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleMoveMessage(graph.RetryConfig{}, 30*time.Second)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"destination_folder_id": "folder-archive"}
	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error for a missing message_id")
	}
	if !strings.Contains(resultText(t, result), "message_id") {
		t.Errorf("expected refusal to name message_id, got: %q", resultText(t, result))
	}
	if len(rec.methods) != 0 {
		t.Errorf("expected no Graph request, got %v", rec.methods)
	}
}

// TestMoveMessage_InvalidIdentifiersRejectedBeforeCall verifies that both
// identifiers are validated before the Graph request. The invalid case is an
// over-length identifier because that, with emptiness, is what
// validate.ValidateResourceID rejects; it does not bound the character set.
func TestMoveMessage_InvalidIdentifiersRejectedBeforeCall(t *testing.T) {
	tooLong := strings.Repeat("a", validate.MaxResourceIDLen+1)
	cases := map[string]map[string]any{
		"message_id":            {"message_id": tooLong},
		"destination_folder_id": {"destination_folder_id": tooLong},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			rec := &patchRecorder{response: `{"id":"msg-2"}`}
			client, srv := newTestGraphClient(t, rec)
			defer srv.Close()
			ctx := auth.WithGraphClient(context.Background(), client)

			handler := NewHandleMoveMessage(graph.RetryConfig{}, 30*time.Second)
			result, err := handler(ctx, newMoveRequest(args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result.IsError {
				t.Fatalf("expected tool error for a malformed %s", name)
			}
			if !strings.Contains(resultText(t, result), name) {
				t.Errorf("expected refusal to name %q, got: %q", name, resultText(t, result))
			}
			if len(rec.methods) != 0 {
				t.Errorf("expected no Graph request, got %v", rec.methods)
			}
		})
	}
}

// TestMoveMessage_AcceptsNonDraftMessage verifies that no draft guard is
// applied: a received message is moved rather than refused for not being a
// draft, and the move is still a single request.
func TestMoveMessage_AcceptsNonDraftMessage(t *testing.T) {
	rec := &patchRecorder{response: `{"id":"msg-2","subject":"Received","isDraft":false}`}
	client, srv := newTestGraphClient(t, rec)
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleMoveMessage(graph.RetryConfig{}, 30*time.Second)
	result, err := handler(ctx, newMoveRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(t, result))
	}
	if strings.Contains(resultText(t, result), "not a draft") {
		t.Errorf("expected no draft refusal, got: %q", resultText(t, result))
	}
	if len(rec.methods) != 1 || rec.methods[0] != http.MethodPost {
		t.Errorf("expected exactly one POST and no verification GET, got %v", rec.methods)
	}
}

// TestMoveMessage_UnresolvableDestinationCarriesFix verifies NFR-4 for the
// failure a move actually hits: a destination that does not resolve returns a
// redacted Graph error carrying the instruction naming where a destination
// identifier comes from.
func TestMoveMessage_UnresolvableDestinationCarriesFix(t *testing.T) {
	client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":"ErrorItemNotFound","message":"The specified object was not found in the store."}}`))
	}))
	defer srv.Close()
	ctx := auth.WithGraphClient(context.Background(), client)

	handler := NewHandleMoveMessage(graph.RetryConfig{}, 30*time.Second)
	result, err := handler(ctx, newMoveRequest(map[string]any{"destination_folder_id": "no-such-folder"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected tool error for an unresolvable destination")
	}
	if !strings.Contains(resultText(t, result), "list_folders") {
		t.Errorf("expected the failure to name where a destination comes from, got: %q", resultText(t, result))
	}
}
