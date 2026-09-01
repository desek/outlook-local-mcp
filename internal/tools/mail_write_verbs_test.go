package tools

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/desek/outlook-local-mcp/internal/auth"
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/mark3labs/mcp-go/mcp"
)

// mailWriteVerb describes one of the four received-message write verbs for the
// assertions that must hold across all of them. It exists so a fifth verb
// added to this family is covered by adding one row rather than by remembering
// to repeat five tests.
type mailWriteVerb struct {
	// name is the verb name, used as the subtest name.
	name string
	// newHandler constructs the verb's handler.
	newHandler func(graph.RetryConfig, time.Duration) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
	// args is a well-formed argument set for the verb.
	args map[string]any
	// method is the single HTTP method the success path must issue.
	method string
	// response is the canned Graph response for the success path.
	response string
}

// mailWriteVerbs enumerates the four verbs added by CR-0078.
func mailWriteVerbs() []mailWriteVerb {
	return []mailWriteVerb{
		{
			name:       "move_message",
			newHandler: NewHandleMoveMessage,
			args: map[string]any{
				"message_id":            "msg-1",
				"destination_folder_id": "folder-archive",
			},
			method:   http.MethodPost,
			response: `{"id":"msg-2","subject":"Quarterly report"}`,
		},
		{
			name:       "set_flag",
			newHandler: NewHandleSetFlag,
			args: map[string]any{
				"message_id":  "msg-1",
				"flag_status": "flagged",
			},
			method:   http.MethodPatch,
			response: `{"id":"msg-1","subject":"Quarterly report","flag":{"flagStatus":"flagged"}}`,
		},
		{
			name:       "set_categories",
			newHandler: NewHandleSetCategories,
			args: map[string]any{
				"message_id": "msg-1",
				"categories": "Red category, Blue category",
			},
			method:   http.MethodPatch,
			response: `{"id":"msg-1","subject":"Quarterly report","categories":["Red category","Blue category"]}`,
		},
		{
			name:       "mark_read",
			newHandler: NewHandleMarkRead,
			args: map[string]any{
				"message_id": "msg-1",
				"is_read":    true,
			},
			method:   http.MethodPatch,
			response: `{"id":"msg-1","subject":"Quarterly report","isRead":true}`,
		},
	}
}

// requestFor builds a request carrying a copy of the verb's argument set, so a
// subtest can mutate it without affecting the table.
func (v mailWriteVerb) requestFor(overrides map[string]any) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	args := make(map[string]any, len(v.args)+len(overrides))
	for k, val := range v.args {
		args[k] = val
	}
	for k, val := range overrides {
		args[k] = val
	}
	req.Params.Arguments = args
	return req
}

// TestMailWriteVerbs_SingleGraphRequestOnSuccess verifies NFR-6 and FR-11 for
// every verb at once: one request on the success path, of the expected method,
// with no preceding GET. A read-modify-write or a reinstated isDraft guard
// would show here as a second request.
func TestMailWriteVerbs_SingleGraphRequestOnSuccess(t *testing.T) {
	for _, v := range mailWriteVerbs() {
		t.Run(v.name, func(t *testing.T) {
			rec := &patchRecorder{response: v.response}
			client, srv := newTestGraphClient(t, rec)
			defer srv.Close()
			ctx := auth.WithGraphClient(context.Background(), client)

			result, err := v.newHandler(graph.RetryConfig{}, 30*time.Second)(ctx, v.requestFor(nil))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.IsError {
				t.Fatalf("unexpected tool error: %s", resultText(t, result))
			}
			if len(rec.methods) != 1 {
				t.Fatalf("expected exactly one Graph request, got %v", rec.methods)
			}
			if rec.methods[0] != v.method {
				t.Errorf("expected a %s, got %s", v.method, rec.methods[0])
			}
		})
	}
}

// TestMailWriteVerbs_RequireMessageID verifies FR-10 across the family: a
// missing message_id is refused by name and no Graph request is issued.
func TestMailWriteVerbs_RequireMessageID(t *testing.T) {
	for _, v := range mailWriteVerbs() {
		t.Run(v.name, func(t *testing.T) {
			rec := &patchRecorder{response: v.response}
			client, srv := newTestGraphClient(t, rec)
			defer srv.Close()
			ctx := auth.WithGraphClient(context.Background(), client)

			req := v.requestFor(nil)
			args, _ := req.Params.Arguments.(map[string]any)
			delete(args, "message_id")

			result, err := v.newHandler(graph.RetryConfig{}, 30*time.Second)(ctx, req)
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
		})
	}
}

// TestMailWriteVerbs_RefuseWithoutAccount verifies that every verb refuses
// before any Graph work when no account is selected, rather than panicking on a
// nil client.
func TestMailWriteVerbs_RefuseWithoutAccount(t *testing.T) {
	for _, v := range mailWriteVerbs() {
		t.Run(v.name, func(t *testing.T) {
			result, err := v.newHandler(graph.RetryConfig{}, 30*time.Second)(context.Background(), v.requestFor(nil))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result.IsError {
				t.Fatal("expected tool error when no account is selected")
			}
			if !strings.Contains(resultText(t, result), "no account selected") {
				t.Errorf("expected the no-account refusal, got: %q", resultText(t, result))
			}
		})
	}
}

// TestMailWriteVerbs_ConfirmationNamesSubjectAndIdentifier verifies FR-12 and
// FR-13 across the family: each returns a text confirmation naming the subject
// and the identifier read from the Graph response, not from the arguments.
func TestMailWriteVerbs_ConfirmationNamesSubjectAndIdentifier(t *testing.T) {
	for _, v := range mailWriteVerbs() {
		t.Run(v.name, func(t *testing.T) {
			rec := &patchRecorder{response: v.response}
			client, srv := newTestGraphClient(t, rec)
			defer srv.Close()
			ctx := auth.WithGraphClient(context.Background(), client)

			result, err := v.newHandler(graph.RetryConfig{}, 30*time.Second)(ctx, v.requestFor(nil))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			text := resultText(t, result)
			if !strings.Contains(text, "Quarterly report") {
				t.Errorf("expected confirmation to name the subject, got: %q", text)
			}
			if !strings.Contains(text, "ID: ") {
				t.Errorf("expected confirmation to name the resulting identifier, got: %q", text)
			}
		})
	}
}

// TestMailWriteVerbs_MissingSubjectFallsBack verifies that a message with no
// subject renders the shared placeholder rather than an empty quoted string,
// for every verb in the family.
func TestMailWriteVerbs_MissingSubjectFallsBack(t *testing.T) {
	for _, v := range mailWriteVerbs() {
		t.Run(v.name, func(t *testing.T) {
			rec := &patchRecorder{response: `{"id":"msg-2"}`}
			client, srv := newTestGraphClient(t, rec)
			defer srv.Close()
			ctx := auth.WithGraphClient(context.Background(), client)

			result, err := v.newHandler(graph.RetryConfig{}, 30*time.Second)(ctx, v.requestFor(nil))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(resultText(t, result), "(No subject)") {
				t.Errorf("expected the no-subject placeholder, got: %q", resultText(t, result))
			}
		})
	}
}

// TestMailWriteVerbs_GraphFailureCarriesFix verifies NFR-4 across the family:
// a Graph failure returns a correction naming what to supply, so a headless
// caller receives the fix rather than only the diagnosis.
func TestMailWriteVerbs_GraphFailureCarriesFix(t *testing.T) {
	for _, v := range mailWriteVerbs() {
		t.Run(v.name, func(t *testing.T) {
			client, srv := newTestGraphClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":{"code":"ErrorItemNotFound","message":"not found"}}`))
			}))
			defer srv.Close()
			ctx := auth.WithGraphClient(context.Background(), client)

			result, err := v.newHandler(graph.RetryConfig{}, 30*time.Second)(ctx, v.requestFor(nil))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result.IsError {
				t.Fatal("expected tool error for a failed Graph call")
			}
			if !strings.Contains(resultText(t, result), "mail.list_messages") {
				t.Errorf("expected the failure to name where a message_id comes from, got: %q", resultText(t, result))
			}
		})
	}
}

// TestMailWriteVerbs_PropertyWritesAreIdempotent verifies NFR-8: repeating a
// call with identical arguments against the same end state produces the same
// confirmation text. move_message is excluded because it mints a new identifier
// on every successful call and is classified non-idempotent for that reason.
func TestMailWriteVerbs_PropertyWritesAreIdempotent(t *testing.T) {
	for _, v := range mailWriteVerbs() {
		if v.name == "move_message" {
			continue
		}
		t.Run(v.name, func(t *testing.T) {
			rec := &patchRecorder{response: v.response}
			client, srv := newTestGraphClient(t, rec)
			defer srv.Close()
			ctx := auth.WithGraphClient(context.Background(), client)
			handler := v.newHandler(graph.RetryConfig{}, 30*time.Second)

			first, err := handler(ctx, v.requestFor(nil))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			second, err := handler(ctx, v.requestFor(nil))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resultText(t, first) != resultText(t, second) {
				t.Errorf("expected an identical confirmation on repeat, got %q then %q",
					resultText(t, first), resultText(t, second))
			}
			if len(rec.bodies) != 2 || rec.bodies[0] != rec.bodies[1] {
				t.Errorf("expected an identical request body on repeat, got %v", rec.bodies)
			}
		})
	}
}
