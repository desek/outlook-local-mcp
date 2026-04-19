// Package tools_test contains cross-cutting annotation tests for all 23 MCP
// tools. Each test verifies that the five MCP annotations (Title,
// ReadOnlyHint, DestructiveHint, IdempotentHint, OpenWorldHint) match the
// values specified in the CR-0052 annotation matrix.
package tools_test

import (
	"testing"

	"github.com/desek/outlook-local-mcp/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
)

// annotationExpectation holds the expected annotation values for a single tool.
type annotationExpectation struct {
	title       string
	readOnly    bool
	destructive bool
	idempotent  bool
	openWorld   bool
}

// assertAnnotations is a test helper that verifies all five MCP annotations on
// the given tool match the expected values. It fails the test with descriptive
// messages for any mismatch.
//
// Parameters:
//   - t: the testing context.
//   - tool: the mcp.Tool to inspect.
//   - want: the expected annotation values.
func assertAnnotations(t *testing.T, tool mcp.Tool, want annotationExpectation) {
	t.Helper()

	ann := tool.Annotations

	if ann.Title != want.title {
		t.Errorf("Title = %q, want %q", ann.Title, want.title)
	}
	if ann.ReadOnlyHint == nil {
		t.Error("ReadOnlyHint is nil, want non-nil")
	} else if *ann.ReadOnlyHint != want.readOnly {
		t.Errorf("ReadOnlyHint = %v, want %v", *ann.ReadOnlyHint, want.readOnly)
	}
	if ann.DestructiveHint == nil {
		t.Error("DestructiveHint is nil, want non-nil")
	} else if *ann.DestructiveHint != want.destructive {
		t.Errorf("DestructiveHint = %v, want %v", *ann.DestructiveHint, want.destructive)
	}
	if ann.IdempotentHint == nil {
		t.Error("IdempotentHint is nil, want non-nil")
	} else if *ann.IdempotentHint != want.idempotent {
		t.Errorf("IdempotentHint = %v, want %v", *ann.IdempotentHint, want.idempotent)
	}
	if ann.OpenWorldHint == nil {
		t.Error("OpenWorldHint is nil, want non-nil")
	} else if *ann.OpenWorldHint != want.openWorld {
		t.Errorf("OpenWorldHint = %v, want %v", *ann.OpenWorldHint, want.openWorld)
	}
}

// --- Read tools (11) ---

// TestListCalendarsToolAnnotations verifies all 5 annotations on calendar_list.
func TestListCalendarsToolAnnotations(t *testing.T) {
	assertAnnotations(t, tools.NewListCalendarsTool(), annotationExpectation{
		title: "List Calendars", readOnly: true, destructive: false, idempotent: true, openWorld: true,
	})
}

// TestListEventsToolAnnotations verifies all 5 annotations on calendar_list_events.
func TestListEventsToolAnnotations(t *testing.T) {
	assertAnnotations(t, tools.NewListEventsTool(), annotationExpectation{
		title: "List Calendar Events", readOnly: true, destructive: false, idempotent: true, openWorld: true,
	})
}

// TestGetEventToolAnnotations verifies all 5 annotations on calendar_get_event.
func TestGetEventToolAnnotations(t *testing.T) {
	assertAnnotations(t, tools.NewGetEventTool(), annotationExpectation{
		title: "Get Calendar Event", readOnly: true, destructive: false, idempotent: true, openWorld: true,
	})
}

// TestSearchEventsToolAnnotations verifies all 5 annotations on calendar_search_events.
func TestSearchEventsToolAnnotations(t *testing.T) {
	assertAnnotations(t, tools.NewSearchEventsTool(false), annotationExpectation{
		title: "Search Calendar Events", readOnly: true, destructive: false, idempotent: true, openWorld: true,
	})
}

// TestGetFreeBusyToolAnnotations verifies all 5 annotations on calendar_get_free_busy.
func TestGetFreeBusyToolAnnotations(t *testing.T) {
	assertAnnotations(t, tools.NewGetFreeBusyTool(), annotationExpectation{
		title: "Get Free/Busy Schedule", readOnly: true, destructive: false, idempotent: true, openWorld: true,
	})
}

// TestListMailFoldersToolAnnotations verifies all 5 annotations on mail_list_folders.
func TestListMailFoldersToolAnnotations(t *testing.T) {
	assertAnnotations(t, tools.NewListMailFoldersTool(), annotationExpectation{
		title: "List Mail Folders", readOnly: true, destructive: false, idempotent: true, openWorld: true,
	})
}

// TestListMessagesToolAnnotations verifies all 5 annotations on mail_list_messages.
func TestListMessagesToolAnnotations(t *testing.T) {
	assertAnnotations(t, tools.NewListMessagesTool(), annotationExpectation{
		title: "List Email Messages", readOnly: true, destructive: false, idempotent: true, openWorld: true,
	})
}

// TestSearchMessagesToolAnnotations verifies all 5 annotations on mail_search_messages.
func TestSearchMessagesToolAnnotations(t *testing.T) {
	assertAnnotations(t, tools.NewSearchMessagesTool(), annotationExpectation{
		title: "Search Email Messages", readOnly: true, destructive: false, idempotent: true, openWorld: true,
	})
}

// TestGetConversationToolAnnotations verifies all 5 annotations on mail_get_conversation (CR-0058).
func TestGetConversationToolAnnotations(t *testing.T) {
	assertAnnotations(t, tools.NewGetConversationTool(), annotationExpectation{
		title: "Get Email Conversation", readOnly: true, destructive: false, idempotent: true, openWorld: true,
	})
}

// TestGetAttachmentToolAnnotations verifies all 5 annotations on mail_get_attachment (CR-0058).
func TestGetAttachmentToolAnnotations(t *testing.T) {
	assertAnnotations(t, tools.NewGetAttachmentTool(), annotationExpectation{
		title: "Get Email Attachment", readOnly: true, destructive: false, idempotent: true, openWorld: true,
	})
}

// TestGetMessageToolAnnotations verifies all 5 annotations on mail_get_message.
func TestGetMessageToolAnnotations(t *testing.T) {
	assertAnnotations(t, tools.NewGetMessageTool(), annotationExpectation{
		title: "Get Email Message", readOnly: true, destructive: false, idempotent: true, openWorld: true,
	})
}

// TestListAccountsToolAnnotations verifies all 5 annotations on account_list.
func TestListAccountsToolAnnotations(t *testing.T) {
	assertAnnotations(t, tools.NewListAccountsTool(), annotationExpectation{
		title: "List Accounts", readOnly: true, destructive: false, idempotent: true, openWorld: false,
	})
}

// TestStatusToolAnnotations verifies all 5 annotations on status.
func TestStatusToolAnnotations(t *testing.T) {
	assertAnnotations(t, tools.NewStatusTool(), annotationExpectation{
		title: "Server Status", readOnly: true, destructive: false, idempotent: true, openWorld: false,
	})
}

// --- Non-destructive write tools (9) ---

// TestCreateEventToolAnnotations verifies all 5 annotations on calendar_create_event.
func TestCreateEventToolAnnotations(t *testing.T) {
	assertAnnotations(t, tools.NewCreateEventTool(), annotationExpectation{
		title: "Create Calendar Event", readOnly: false, destructive: false, idempotent: false, openWorld: true,
	})
}

// TestUpdateEventToolAnnotations verifies all 5 annotations on calendar_update_event.
func TestUpdateEventToolAnnotations(t *testing.T) {
	assertAnnotations(t, tools.NewUpdateEventTool(), annotationExpectation{
		title: "Update Calendar Event", readOnly: false, destructive: false, idempotent: true, openWorld: true,
	})
}

// TestRespondEventToolAnnotations verifies all 5 annotations on calendar_respond_event.
func TestRespondEventToolAnnotations(t *testing.T) {
	assertAnnotations(t, tools.NewRespondEventTool(), annotationExpectation{
		title: "Respond to Event", readOnly: false, destructive: false, idempotent: true, openWorld: true,
	})
}

// TestRescheduleEventToolAnnotations verifies all 5 annotations on calendar_reschedule_event.
func TestRescheduleEventToolAnnotations(t *testing.T) {
	assertAnnotations(t, tools.NewRescheduleEventTool(), annotationExpectation{
		title: "Reschedule Event", readOnly: false, destructive: false, idempotent: true, openWorld: true,
	})
}

// TestCreateMeetingToolAnnotations verifies all 5 annotations on
// calendar_create_meeting (CR-0054).
func TestCreateMeetingToolAnnotations(t *testing.T) {
	assertAnnotations(t, tools.NewCreateMeetingTool(), annotationExpectation{
		title: "Create Calendar Meeting", readOnly: false, destructive: false, idempotent: false, openWorld: true,
	})
}

// TestUpdateMeetingToolAnnotations verifies all 5 annotations on
// calendar_update_meeting (CR-0054).
func TestUpdateMeetingToolAnnotations(t *testing.T) {
	assertAnnotations(t, tools.NewUpdateMeetingTool(), annotationExpectation{
		title: "Update Calendar Meeting", readOnly: false, destructive: false, idempotent: true, openWorld: true,
	})
}

// TestRescheduleMeetingToolAnnotations verifies all 5 annotations on
// calendar_reschedule_meeting (CR-0054).
func TestRescheduleMeetingToolAnnotations(t *testing.T) {
	assertAnnotations(t, tools.NewRescheduleMeetingTool(), annotationExpectation{
		title: "Reschedule Meeting", readOnly: false, destructive: false, idempotent: true, openWorld: true,
	})
}

// TestAddAccountToolAnnotations verifies all 5 annotations on account_add.
func TestAddAccountToolAnnotations(t *testing.T) {
	assertAnnotations(t, tools.NewAddAccountTool(), annotationExpectation{
		title: "Add Account", readOnly: false, destructive: false, idempotent: false, openWorld: true,
	})
}

// TestCompleteAuthToolAnnotations verifies all 5 annotations on complete_auth.
func TestCompleteAuthToolAnnotations(t *testing.T) {
	assertAnnotations(t, tools.NewCompleteAuthTool(), annotationExpectation{
		title: "Complete Authentication", readOnly: false, destructive: false, idempotent: false, openWorld: true,
	})
}

// --- Destructive tools (3) ---

// TestDeleteEventToolAnnotations verifies all 5 annotations on calendar_delete_event.
func TestDeleteEventToolAnnotations(t *testing.T) {
	assertAnnotations(t, tools.NewDeleteEventTool(), annotationExpectation{
		title: "Delete Calendar Event", readOnly: false, destructive: true, idempotent: true, openWorld: true,
	})
}

// TestCancelMeetingToolAnnotations verifies all 5 annotations on calendar_cancel_meeting.
func TestCancelMeetingToolAnnotations(t *testing.T) {
	assertAnnotations(t, tools.NewCancelMeetingTool(), annotationExpectation{
		title: "Cancel Calendar Meeting", readOnly: false, destructive: true, idempotent: true, openWorld: true,
	})
}

// TestLoginAccount_Annotations verifies all 5 annotations on account_login (CR-0056).
func TestLoginAccount_Annotations(t *testing.T) {
	assertAnnotations(t, tools.NewLoginAccountTool(), annotationExpectation{
		title: "Log In Account", readOnly: false, destructive: false, idempotent: false, openWorld: true,
	})
}

// TestLogoutAccount_Annotations verifies all 5 annotations on account_logout (CR-0056).
func TestLogoutAccount_Annotations(t *testing.T) {
	assertAnnotations(t, tools.NewLogoutAccountTool(), annotationExpectation{
		title: "Log Out Account", readOnly: false, destructive: false, idempotent: true, openWorld: false,
	})
}

// TestRefreshAccount_Annotations verifies all 5 annotations on account_refresh (CR-0056).
func TestRefreshAccount_Annotations(t *testing.T) {
	assertAnnotations(t, tools.NewRefreshAccountTool(), annotationExpectation{
		title: "Refresh Account Token", readOnly: false, destructive: false, idempotent: true, openWorld: true,
	})
}

// TestRemoveAccountToolAnnotations verifies all 5 annotations on account_remove.
func TestRemoveAccountToolAnnotations(t *testing.T) {
	assertAnnotations(t, tools.NewRemoveAccountTool(), annotationExpectation{
		title: "Remove Account", readOnly: false, destructive: true, idempotent: true, openWorld: false,
	})
}
