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
		tools.NewCancelEventTool(),
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
