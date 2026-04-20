package graph

import (
	"fmt"

	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// ProvenanceGUID is a dedicated UUID namespace for the provenance extended
// property. This GUID is used in the MAPI named property ID string to avoid
// collisions with other applications that may set extended properties on
// calendar events.
const ProvenanceGUID = "73830cef-ea4a-4459-b555-80a4619f667d"

// BuildProvenancePropertyID constructs the full MAPI named property ID string
// from the configured provenance tag name. The resulting string follows the
// format required by the Microsoft Graph API for single-value extended
// properties: "String {GUID} Name <tagName>".
//
// Parameters:
//   - tagName: the provenance tag name (e.g., "com.github.desek.outlook-local-mcp.created").
//
// Returns the full property ID string suitable for use with
// SetSingleValueExtendedProperties, $expand filters, and $filter clauses.
func BuildProvenancePropertyID(tagName string) string {
	return fmt.Sprintf("String {%s} Name %s", ProvenanceGUID, tagName)
}

// NewProvenanceProperty creates a new SingleValueLegacyExtendedPropertyable
// configured with the given property ID and a value of "true". The returned
// property is ready to be set on a models.Event via
// SetSingleValueExtendedProperties before POSTing to the Graph API.
//
// Parameters:
//   - propertyID: the full property ID string from BuildProvenancePropertyID.
//
// Returns a configured SingleValueLegacyExtendedPropertyable with id and
// value set.
//
// Side effects: none.
func NewProvenanceProperty(propertyID string) models.SingleValueLegacyExtendedPropertyable {
	prop := models.NewSingleValueLegacyExtendedProperty()
	prop.SetId(&propertyID)
	val := "true"
	prop.SetValue(&val)
	return prop
}

// HasProvenanceTag checks whether the given event has a single-value extended
// property matching the specified provenance property ID. It iterates over the
// event's SingleValueExtendedProperties and returns true if any property's ID
// matches propertyID.
//
// Parameters:
//   - event: a models.Eventable whose extended properties are checked.
//   - propertyID: the full property ID string to match against.
//
// Returns true if the provenance property is present on the event, false
// otherwise. Returns false if event is nil or has no extended properties.
//
// Side effects: none.
func HasProvenanceTag(event models.Eventable, propertyID string) bool {
	if event == nil {
		return false
	}
	props := event.GetSingleValueExtendedProperties()
	for _, p := range props {
		if p.GetId() != nil && *p.GetId() == propertyID {
			return true
		}
	}
	return false
}

// HasMessageProvenanceTag checks whether the given message has a single-value
// extended property matching the specified provenance property ID. It iterates
// over the message's SingleValueExtendedProperties and returns true if any
// property's ID matches propertyID. This is the mail counterpart to
// HasProvenanceTag for events.
//
// Parameters:
//   - msg: a models.Messageable whose extended properties are checked.
//   - propertyID: the full property ID string to match against.
//
// Returns true if the provenance property is present on the message, false
// otherwise. Returns false if msg is nil or has no extended properties.
//
// Side effects: none.
func HasMessageProvenanceTag(msg models.Messageable, propertyID string) bool {
	if msg == nil {
		return false
	}
	props := msg.GetSingleValueExtendedProperties()
	for _, p := range props {
		if p.GetId() != nil && *p.GetId() == propertyID {
			return true
		}
	}
	return false
}

// ProvenanceExpandFilter returns the OData $expand clause value for requesting
// the provenance extended property on event read operations (get_event,
// list_events, search_events). The returned string is suitable for use as the
// value of the $expand query parameter.
//
// Parameters:
//   - propertyID: the full property ID string from BuildProvenancePropertyID.
//
// Returns the $expand filter string.
//
// Side effects: none.
func ProvenanceExpandFilter(propertyID string) string {
	return fmt.Sprintf("singleValueExtendedProperties($filter=id eq '%s')", propertyID)
}
