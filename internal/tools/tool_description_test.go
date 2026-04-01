package tools_test

import (
	"strings"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
)

// TestCreateEvent_DescriptionContainsMeetingRedirect verifies that the
// create_event tool description directs users to calendar_create_meeting
// for events with attendees (CR-0054).
func TestCreateEvent_DescriptionContainsMeetingRedirect(t *testing.T) {
	tool := tools.NewCreateEventTool()

	if !strings.Contains(tool.Description, "calendar_create_meeting") {
		t.Errorf("tool description missing meeting redirect:\n  got: %s", tool.Description)
	}
}

// TestCreateEvent_NoAttendeesParameter verifies that create_event has no
// attendees parameter after the event/meeting split (CR-0054).
func TestCreateEvent_NoAttendeesParameter(t *testing.T) {
	tool := tools.NewCreateEventTool()

	if _, ok := tool.InputSchema.Properties["attendees"]; ok {
		t.Error("create_event should not have an 'attendees' parameter after CR-0054 split")
	}
}

// TestCreateEvent_NoConfirmationGuidance verifies that create_event has no
// CR-0053 confirmation guidance after the event/meeting split (CR-0054).
func TestCreateEvent_NoConfirmationGuidance(t *testing.T) {
	tool := tools.NewCreateEventTool()

	if strings.Contains(tool.Description, "MUST present") {
		t.Errorf("create_event should not contain 'MUST present' after CR-0054 split:\n  got: %s", tool.Description)
	}
	if strings.Contains(tool.Description, "confirmation") {
		t.Errorf("create_event should not contain 'confirmation' after CR-0054 split:\n  got: %s", tool.Description)
	}
}

// TestCreateEvent_NoCR0039AttendeeAdvisory verifies that create_event has no
// CR-0039 attendee advisory after the event/meeting split (CR-0054).
func TestCreateEvent_NoCR0039AttendeeAdvisory(t *testing.T) {
	tool := tools.NewCreateEventTool()

	if strings.Contains(tool.Description, "attendees are included") {
		t.Errorf("create_event should not contain CR-0039 attendee advisory after CR-0054 split:\n  got: %s", tool.Description)
	}
}

// TestUpdateEvent_DescriptionContainsMeetingRedirect verifies that the
// update_event tool description directs users to calendar_update_meeting
// for attendee changes (CR-0054).
func TestUpdateEvent_DescriptionContainsMeetingRedirect(t *testing.T) {
	tool := tools.NewUpdateEventTool()

	if !strings.Contains(tool.Description, "calendar_update_meeting") {
		t.Errorf("tool description missing meeting redirect:\n  got: %s", tool.Description)
	}
}

// TestUpdateEvent_NoAttendeesParameter verifies that update_event has no
// attendees parameter after the event/meeting split (CR-0054).
func TestUpdateEvent_NoAttendeesParameter(t *testing.T) {
	tool := tools.NewUpdateEventTool()

	if _, ok := tool.InputSchema.Properties["attendees"]; ok {
		t.Error("update_event should not have an 'attendees' parameter after CR-0054 split")
	}
}

// TestUpdateEvent_NoConfirmationGuidance verifies that update_event has no
// CR-0053 confirmation guidance after the event/meeting split (CR-0054).
func TestUpdateEvent_NoConfirmationGuidance(t *testing.T) {
	tool := tools.NewUpdateEventTool()

	if strings.Contains(tool.Description, "MUST present") {
		t.Errorf("update_event should not contain 'MUST present' after CR-0054 split:\n  got: %s", tool.Description)
	}
	if strings.Contains(tool.Description, "confirmation") {
		t.Errorf("update_event should not contain 'confirmation' after CR-0054 split:\n  got: %s", tool.Description)
	}
}

// TestUpdateEvent_NoCR0039AttendeeAdvisory verifies that update_event has no
// CR-0039 attendee advisory after the event/meeting split (CR-0054).
func TestUpdateEvent_NoCR0039AttendeeAdvisory(t *testing.T) {
	tool := tools.NewUpdateEventTool()

	if strings.Contains(tool.Description, "attendees are included") {
		t.Errorf("update_event should not contain CR-0039 attendee advisory after CR-0054 split:\n  got: %s", tool.Description)
	}
}

// TestRescheduleEvent_DescriptionContainsMeetingRedirect verifies that the
// reschedule_event tool description directs users to calendar_reschedule_meeting
// for events with attendees (CR-0054).
func TestRescheduleEvent_DescriptionContainsMeetingRedirect(t *testing.T) {
	tool := tools.NewRescheduleEventTool()

	if !strings.Contains(tool.Description, "calendar_reschedule_meeting") {
		t.Errorf("tool description missing meeting redirect:\n  got: %s", tool.Description)
	}
}

// TestRescheduleEvent_NoConfirmationGuidance verifies that reschedule_event has
// no CR-0053 confirmation guidance after the event/meeting split (CR-0054).
func TestRescheduleEvent_NoConfirmationGuidance(t *testing.T) {
	tool := tools.NewRescheduleEventTool()

	if strings.Contains(tool.Description, "MUST present") {
		t.Errorf("reschedule_event should not contain 'MUST present' after CR-0054 split:\n  got: %s", tool.Description)
	}
	if strings.Contains(tool.Description, "confirmation") {
		t.Errorf("reschedule_event should not contain 'confirmation' after CR-0054 split:\n  got: %s", tool.Description)
	}
}

// TestCancelMeeting_DescriptionContainsConfirmationGuidance verifies that the
// cancel_meeting tool description contains user confirmation instruction text
// added by CR-0053.
func TestCancelMeeting_DescriptionContainsConfirmationGuidance(t *testing.T) {
	tool := tools.NewCancelMeetingTool()

	if !strings.Contains(tool.Description, "confirmation") {
		t.Errorf("tool description missing 'confirmation':\n  got: %s", tool.Description)
	}
	if !strings.Contains(tool.Description, "MUST present") {
		t.Errorf("tool description missing 'MUST present':\n  got: %s", tool.Description)
	}
}

// TestCancelMeeting_DescriptionContainsExternalWarningGuidance verifies that the
// cancel_meeting tool description contains external attendee warning text
// added by CR-0053.
func TestCancelMeeting_DescriptionContainsExternalWarningGuidance(t *testing.T) {
	tool := tools.NewCancelMeetingTool()

	if !strings.Contains(tool.Description, "external") {
		t.Errorf("tool description missing 'external':\n  got: %s", tool.Description)
	}
}

// TestCancelMeeting_ConfirmationInstructions_ScopedToAttendees verifies that
// the cancel_meeting confirmation instruction is scoped to attendee presence
// per CR-0053 AC-8. After CR-0054, only cancel_meeting retains attendee-scoped
// confirmation in the event tools; create/update/reschedule event tools no
// longer carry confirmation guidance.
func TestCancelMeeting_ConfirmationInstructions_ScopedToAttendees(t *testing.T) {
	tool := tools.NewCancelMeetingTool()
	if !strings.Contains(tool.Description, "When the event has attendees") {
		t.Errorf("description missing attendee-scoping language:\n  got: %s", tool.Description)
	}
}

// TestCalendarTools_AccountParamDescription verifies that all 9 calendar tools
// have the updated account parameter description containing "the default
// account is used" and NOT containing the old elicitation-specific text
// "you will be prompted to select".
func TestCalendarTools_AccountParamDescription(t *testing.T) {
	calendarTools := []mcp.Tool{
		tools.NewListCalendarsTool(),
		tools.NewListEventsTool(),
		tools.NewGetEventTool(),
		tools.NewSearchEventsTool(false),
		tools.NewGetFreeBusyTool(),
		tools.NewCreateEventTool(),
		tools.NewUpdateEventTool(),
		tools.NewDeleteEventTool(),
		tools.NewCancelMeetingTool(),
	}

	const wantSubstring = "the default account is used"
	const bannedSubstring = "you will be prompted to select"

	for _, tool := range calendarTools {
		t.Run(tool.Name, func(t *testing.T) {
			props := tool.InputSchema.Properties
			accountProp, ok := props["account"]
			if !ok {
				t.Fatal("missing 'account' parameter")
			}

			propMap, ok := accountProp.(map[string]any)
			if !ok {
				t.Fatalf("expected map[string]any for account property, got %T", accountProp)
			}

			desc, ok := propMap["description"].(string)
			if !ok {
				t.Fatal("missing or non-string 'description' on account property")
			}

			if !strings.Contains(desc, wantSubstring) {
				t.Errorf("account description missing %q:\n  got: %s", wantSubstring, desc)
			}
			if strings.Contains(desc, bannedSubstring) {
				t.Errorf("account description contains banned text %q:\n  got: %s", bannedSubstring, desc)
			}
		})
	}
}
