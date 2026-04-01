package tools_test

import (
	"strings"
	"testing"

	"github.com/desek/outlook-local-mcp/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
)

// TestCreateEvent_DescriptionContainsAttendeeGuidance verifies that the
// create_event tool description and its body/location parameter descriptions
// contain the attendee quality guidance text added by CR-0039.
func TestCreateEvent_DescriptionContainsAttendeeGuidance(t *testing.T) {
	tool := tools.NewCreateEventTool()

	// Main tool description must mention attendees guidance.
	if !strings.Contains(tool.Description, "attendees") {
		t.Errorf("tool description missing attendee guidance:\n  got: %s", tool.Description)
	}

	// Body parameter must mention "recommended".
	bodyProp, ok := tool.InputSchema.Properties["body"].(map[string]any)
	if !ok {
		t.Fatal("missing or invalid 'body' parameter")
	}
	bodyDesc, _ := bodyProp["description"].(string)
	if !strings.Contains(strings.ToLower(bodyDesc), "recommended") {
		t.Errorf("body description missing 'recommended':\n  got: %s", bodyDesc)
	}

	// Location parameter must mention "recommended".
	locProp, ok := tool.InputSchema.Properties["location"].(map[string]any)
	if !ok {
		t.Fatal("missing or invalid 'location' parameter")
	}
	locDesc, _ := locProp["description"].(string)
	if !strings.Contains(strings.ToLower(locDesc), "recommended") {
		t.Errorf("location description missing 'recommended':\n  got: %s", locDesc)
	}
}

// TestUpdateEvent_DescriptionContainsAttendeeGuidance verifies that the
// update_event tool description and its body/location parameter descriptions
// contain the attendee quality guidance text added by CR-0039.
func TestUpdateEvent_DescriptionContainsAttendeeGuidance(t *testing.T) {
	tool := tools.NewUpdateEventTool()

	// Main tool description must mention attendees guidance.
	if !strings.Contains(tool.Description, "attendees") {
		t.Errorf("tool description missing attendee guidance:\n  got: %s", tool.Description)
	}

	// Body parameter must mention "recommended".
	bodyProp, ok := tool.InputSchema.Properties["body"].(map[string]any)
	if !ok {
		t.Fatal("missing or invalid 'body' parameter")
	}
	bodyDesc, _ := bodyProp["description"].(string)
	if !strings.Contains(strings.ToLower(bodyDesc), "recommended") {
		t.Errorf("body description missing 'recommended':\n  got: %s", bodyDesc)
	}

	// Location parameter must mention "recommended".
	locProp, ok := tool.InputSchema.Properties["location"].(map[string]any)
	if !ok {
		t.Fatal("missing or invalid 'location' parameter")
	}
	locDesc, _ := locProp["description"].(string)
	if !strings.Contains(strings.ToLower(locDesc), "recommended") {
		t.Errorf("location description missing 'recommended':\n  got: %s", locDesc)
	}
}

// TestCreateEvent_DescriptionContainsConfirmationGuidance verifies that the
// create_event tool description contains user confirmation instruction text
// added by CR-0053.
func TestCreateEvent_DescriptionContainsConfirmationGuidance(t *testing.T) {
	tool := tools.NewCreateEventTool()

	if !strings.Contains(tool.Description, "confirmation") {
		t.Errorf("tool description missing 'confirmation':\n  got: %s", tool.Description)
	}
	if !strings.Contains(tool.Description, "MUST present") {
		t.Errorf("tool description missing 'MUST present':\n  got: %s", tool.Description)
	}
}

// TestCreateEvent_DescriptionContainsExternalWarningGuidance verifies that the
// create_event tool description contains external attendee warning text
// added by CR-0053.
func TestCreateEvent_DescriptionContainsExternalWarningGuidance(t *testing.T) {
	tool := tools.NewCreateEventTool()

	if !strings.Contains(tool.Description, "external") {
		t.Errorf("tool description missing 'external':\n  got: %s", tool.Description)
	}
	if !strings.Contains(tool.Description, "domain") {
		t.Errorf("tool description missing 'domain':\n  got: %s", tool.Description)
	}
}

// TestUpdateEvent_DescriptionContainsConfirmationGuidance verifies that the
// update_event tool description contains user confirmation instruction text
// added by CR-0053.
func TestUpdateEvent_DescriptionContainsConfirmationGuidance(t *testing.T) {
	tool := tools.NewUpdateEventTool()

	if !strings.Contains(tool.Description, "confirmation") {
		t.Errorf("tool description missing 'confirmation':\n  got: %s", tool.Description)
	}
	if !strings.Contains(tool.Description, "MUST present") {
		t.Errorf("tool description missing 'MUST present':\n  got: %s", tool.Description)
	}
}

// TestUpdateEvent_DescriptionContainsExternalWarningGuidance verifies that the
// update_event tool description contains external attendee warning text
// added by CR-0053.
func TestUpdateEvent_DescriptionContainsExternalWarningGuidance(t *testing.T) {
	tool := tools.NewUpdateEventTool()

	if !strings.Contains(tool.Description, "external") {
		t.Errorf("tool description missing 'external':\n  got: %s", tool.Description)
	}
	if !strings.Contains(tool.Description, "domain") {
		t.Errorf("tool description missing 'domain':\n  got: %s", tool.Description)
	}
}

// TestRescheduleEvent_DescriptionContainsConfirmationGuidance verifies that the
// reschedule_event tool description contains user confirmation instruction text
// added by CR-0053.
func TestRescheduleEvent_DescriptionContainsConfirmationGuidance(t *testing.T) {
	tool := tools.NewRescheduleEventTool()

	if !strings.Contains(tool.Description, "confirmation") {
		t.Errorf("tool description missing 'confirmation':\n  got: %s", tool.Description)
	}
	if !strings.Contains(tool.Description, "MUST present") {
		t.Errorf("tool description missing 'MUST present':\n  got: %s", tool.Description)
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

// TestCreateEvent_DescriptionContainsSummaryFields verifies that the
// create_event tool description specifies all required summary fields
// per CR-0053 AC-7.
func TestCreateEvent_DescriptionContainsSummaryFields(t *testing.T) {
	tool := tools.NewCreateEventTool()

	requiredFields := []string{"subject", "date/time", "attendee list", "location", "body preview"}
	for _, field := range requiredFields {
		if !strings.Contains(tool.Description, field) {
			t.Errorf("tool description missing required summary field %q:\n  got: %s", field, tool.Description)
		}
	}
}

// TestConfirmationInstructions_ScopedToAttendees verifies that all four
// confirmation instructions are conditional on attendee presence per CR-0053
// AC-8. Each description must scope its confirmation directive to scenarios
// where attendees are involved.
func TestConfirmationInstructions_ScopedToAttendees(t *testing.T) {
	tests := []struct {
		name   string
		tool   mcp.Tool
		scoped string
	}{
		{"calendar_create_event", tools.NewCreateEventTool(), "When the event includes attendees"},
		{"calendar_update_event", tools.NewUpdateEventTool(), "When the update adds or modifies attendees"},
		{"calendar_reschedule_event", tools.NewRescheduleEventTool(), "When the event has attendees"},
		{"calendar_cancel_meeting", tools.NewCancelMeetingTool(), "When the event has attendees"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(tt.tool.Description, tt.scoped) {
				t.Errorf("description missing attendee-scoping language %q:\n  got: %s", tt.scoped, tt.tool.Description)
			}
		})
	}
}

// TestConfirmationInstructions_UseMUSTKeyword verifies that all four
// confirmation instructions use the "MUST" keyword per CR-0053 AC-11.
func TestConfirmationInstructions_UseMUSTKeyword(t *testing.T) {
	tests := []struct {
		name string
		tool mcp.Tool
	}{
		{"calendar_create_event", tools.NewCreateEventTool()},
		{"calendar_update_event", tools.NewUpdateEventTool()},
		{"calendar_reschedule_event", tools.NewRescheduleEventTool()},
		{"calendar_cancel_meeting", tools.NewCancelMeetingTool()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(tt.tool.Description, "MUST") {
				t.Errorf("description missing 'MUST' keyword:\n  got: %s", tt.tool.Description)
			}
		})
	}
}

// TestConfirmationInstructions_AskUserQuestionGuidance verifies that all four
// confirmation instructions mention AskUserQuestion as a preferred UX mechanism
// per CR-0053 enhancement.
func TestConfirmationInstructions_AskUserQuestionGuidance(t *testing.T) {
	tests := []struct {
		name string
		tool mcp.Tool
	}{
		{"calendar_create_event", tools.NewCreateEventTool()},
		{"calendar_update_event", tools.NewUpdateEventTool()},
		{"calendar_reschedule_event", tools.NewRescheduleEventTool()},
		{"calendar_cancel_meeting", tools.NewCancelMeetingTool()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(tt.tool.Description, "AskUserQuestion") {
				t.Errorf("description missing 'AskUserQuestion' guidance:\n  got: %s", tt.tool.Description)
			}
		})
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
