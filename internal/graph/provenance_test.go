package graph

import (
	"testing"

	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// TestBuildProvenancePropertyID validates that BuildProvenancePropertyID
// constructs the correct property ID string from the default tag name.
func TestBuildProvenancePropertyID(t *testing.T) {
	got := BuildProvenancePropertyID("com.github.desek.outlook-local-mcp.created")
	want := "String {73830cef-ea4a-4459-b555-80a4619f667d} Name com.github.desek.outlook-local-mcp.created"

	if got != want {
		t.Errorf("BuildProvenancePropertyID() = %q, want %q", got, want)
	}
}

// TestBuildProvenancePropertyID_CustomTag validates that BuildProvenancePropertyID
// constructs the correct property ID string from a custom tag name.
func TestBuildProvenancePropertyID_CustomTag(t *testing.T) {
	got := BuildProvenancePropertyID("com.contoso.my-agent.created")
	want := "String {73830cef-ea4a-4459-b555-80a4619f667d} Name com.contoso.my-agent.created"

	if got != want {
		t.Errorf("BuildProvenancePropertyID() = %q, want %q", got, want)
	}
}

// TestNewProvenanceProperty validates that NewProvenanceProperty returns a
// property with the correct ID and value of "true".
func TestNewProvenanceProperty(t *testing.T) {
	propID := "String {73830cef-ea4a-4459-b555-80a4619f667d} Name test.tag"
	prop := NewProvenanceProperty(propID)

	if prop.GetId() == nil || *prop.GetId() != propID {
		t.Errorf("prop.GetId() = %v, want %q", prop.GetId(), propID)
	}

	if prop.GetValue() == nil || *prop.GetValue() != "true" {
		t.Errorf("prop.GetValue() = %v, want %q", prop.GetValue(), "true")
	}
}

// TestHasProvenanceTag_Present validates that HasProvenanceTag returns true
// when the provenance property is present on the event.
func TestHasProvenanceTag_Present(t *testing.T) {
	propID := "String {73830cef-ea4a-4459-b555-80a4619f667d} Name test.tag"

	event := models.NewEvent()
	prop := models.NewSingleValueLegacyExtendedProperty()
	prop.SetId(&propID)
	val := "true"
	prop.SetValue(&val)
	event.SetSingleValueExtendedProperties([]models.SingleValueLegacyExtendedPropertyable{prop})

	if !HasProvenanceTag(event, propID) {
		t.Error("HasProvenanceTag() = false, want true")
	}
}

// TestHasProvenanceTag_Absent validates that HasProvenanceTag returns false
// when the event has no extended properties.
func TestHasProvenanceTag_Absent(t *testing.T) {
	propID := "String {73830cef-ea4a-4459-b555-80a4619f667d} Name test.tag"

	event := models.NewEvent()

	if HasProvenanceTag(event, propID) {
		t.Error("HasProvenanceTag() = true, want false")
	}
}

// TestHasProvenanceTag_OtherProperty validates that HasProvenanceTag returns
// false when the event has a different extended property.
func TestHasProvenanceTag_OtherProperty(t *testing.T) {
	propID := "String {73830cef-ea4a-4459-b555-80a4619f667d} Name test.tag"
	otherID := "String {00000000-0000-0000-0000-000000000000} Name other.tag"

	event := models.NewEvent()
	prop := models.NewSingleValueLegacyExtendedProperty()
	prop.SetId(&otherID)
	val := "true"
	prop.SetValue(&val)
	event.SetSingleValueExtendedProperties([]models.SingleValueLegacyExtendedPropertyable{prop})

	if HasProvenanceTag(event, propID) {
		t.Error("HasProvenanceTag() = true, want false")
	}
}

// TestProvenanceExpandFilter validates that ProvenanceExpandFilter returns the
// correct OData $expand clause string.
func TestProvenanceExpandFilter(t *testing.T) {
	propID := "String {73830cef-ea4a-4459-b555-80a4619f667d} Name test.tag"
	got := ProvenanceExpandFilter(propID)
	want := "singleValueExtendedProperties($filter=id eq 'String {73830cef-ea4a-4459-b555-80a4619f667d} Name test.tag')"

	if got != want {
		t.Errorf("ProvenanceExpandFilter() = %q, want %q", got, want)
	}
}
