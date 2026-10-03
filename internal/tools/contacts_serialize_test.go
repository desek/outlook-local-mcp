// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains the serializer tests for the contacts domain: that the
// summary tier carries the resolution field set for both resources, that every
// address survives the projection, and that a sparsely populated record still
// projects to a stable shape.
//
// @agents-index: Serializer tests for the contacts domain covering the summary
// and raw projections of saved contacts and relevance-ranked people.
package tools

import (
	"testing"

	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// newTestContact builds a saved contact carrying two addresses, which is the
// case that distinguishes a summary tier keeping every address from one keeping
// only the leading one.
func newTestContact() models.Contactable {
	contact := models.NewContact()
	id := "AAMkContact1"
	name := "Alex Stone"
	company := "Northwind"
	mobile := "+1 555 0111"
	contact.SetId(&id)
	contact.SetDisplayName(&name)
	contact.SetCompanyName(&company)
	contact.SetMobilePhone(&mobile)
	contact.SetBusinessPhones([]string{"+1 555 0100"})

	workName, workAddress := "Alex Stone", "alex@example.com"
	work := models.NewEmailAddress()
	work.SetName(&workName)
	work.SetAddress(&workAddress)

	homeAddress := "alex@home.example"
	home := models.NewEmailAddress()
	home.SetAddress(&homeAddress)

	contact.SetEmailAddresses([]models.EmailAddressable{work, home})

	street, city := "1 Main Street", "Redmond"
	postal := models.NewPhysicalAddress()
	postal.SetStreet(&street)
	postal.SetCity(&city)
	contact.SetBusinessAddress(postal)

	return contact
}

// newTestPerson builds a ranked person carrying one scored address, the shape
// the people endpoint returns for a correspondent Graph is confident about.
func newTestPerson() models.Personable {
	person := models.NewPerson()
	id := "person-1"
	name := "Alex Ranked"
	person.SetId(&id)
	person.SetDisplayName(&name)

	address := "alex.ranked@example.com"
	score := 12.5
	scored := models.NewScoredEmailAddress()
	scored.SetAddress(&address)
	scored.SetRelevanceScore(&score)
	person.SetScoredEmailAddresses([]models.ScoredEmailAddressable{scored})

	return person
}

// TestSerializeSummaryContact_CarriesResolutionFields validates that the
// summary tier states who the contact is, the address to write to, and the
// identifier to escalate with.
func TestSerializeSummaryContact_CarriesResolutionFields(t *testing.T) {
	result := SerializeSummaryContact(newTestContact())

	if result["displayName"] != "Alex Stone" {
		t.Errorf("displayName = %v, want Alex Stone", result["displayName"])
	}
	if result["emailAddress"] != "alex@example.com" {
		t.Errorf("emailAddress = %v, want alex@example.com", result["emailAddress"])
	}
	if result["id"] != "AAMkContact1" {
		t.Errorf("id = %v, want AAMkContact1", result["id"])
	}
}

// TestSerializeSummaryContact_KeepsEveryAddress validates that the summary tier
// loses no address the contact holds, so a caller need not escalate to raw to
// discover a second one.
func TestSerializeSummaryContact_KeepsEveryAddress(t *testing.T) {
	addresses, ok := SerializeSummaryContact(newTestContact())["emailAddresses"].([]string)
	if !ok {
		t.Fatal("emailAddresses is not a string slice")
	}
	if len(addresses) != 2 {
		t.Fatalf("address count = %d, want 2", len(addresses))
	}
	if addresses[1] != "alex@home.example" {
		t.Errorf("second address = %q, want alex@home.example", addresses[1])
	}
}

// TestSerializeContact_CarriesPhonesAndPostalAddress validates that the raw
// tier answers the detail the summary tier deliberately omits.
func TestSerializeContact_CarriesPhonesAndPostalAddress(t *testing.T) {
	result := SerializeContact(newTestContact())

	if result["mobilePhone"] != "+1 555 0111" {
		t.Errorf("mobilePhone = %v, want +1 555 0111", result["mobilePhone"])
	}
	business, ok := result["businessAddress"].(map[string]string)
	if !ok {
		t.Fatal("businessAddress is not a string map")
	}
	if business["city"] != "Redmond" {
		t.Errorf("business city = %q, want Redmond", business["city"])
	}
	emails, ok := result["emailAddresses"].([]map[string]string)
	if !ok {
		t.Fatal("raw emailAddresses is not a slice of maps")
	}
	if len(emails) != 2 || emails[0]["name"] != "Alex Stone" {
		t.Errorf("raw emailAddresses = %v, want both entries with the per-address name", emails)
	}
}

// TestSerializeSummaryPerson_CarriesRelevanceScore validates that the ranking
// signal, the only thing distinguishing a person from a contact, reaches the
// summary tier.
func TestSerializeSummaryPerson_CarriesRelevanceScore(t *testing.T) {
	result := SerializeSummaryPerson(newTestPerson())

	if result["emailAddress"] != "alex.ranked@example.com" {
		t.Errorf("emailAddress = %v, want alex.ranked@example.com", result["emailAddress"])
	}
	score, ok := result["relevanceScore"].(float64)
	if !ok || score != 12.5 {
		t.Errorf("relevanceScore = %v, want 12.5", result["relevanceScore"])
	}
}

// TestSerializePerson_CarriesScoredAddresses validates that the raw tier states
// each address with its own score rather than only the leading one.
func TestSerializePerson_CarriesScoredAddresses(t *testing.T) {
	scored, ok := SerializePerson(newTestPerson())["scoredEmailAddresses"].([]map[string]any)
	if !ok {
		t.Fatal("scoredEmailAddresses is not a slice of maps")
	}
	if len(scored) != 1 {
		t.Fatalf("scored address count = %d, want 1", len(scored))
	}
	if scored[0]["address"] != "alex.ranked@example.com" || scored[0]["relevanceScore"] != 12.5 {
		t.Errorf("scored address = %v, want the address and its score", scored[0])
	}
}

// TestSerializeSummary_EmptyRecordsProjectStably validates that a record Graph
// returned with nothing populated still projects to the tier's field set, so a
// caller reading the summary never has to distinguish a missing key from an
// empty value.
func TestSerializeSummary_EmptyRecordsProjectStably(t *testing.T) {
	contact := SerializeSummaryContact(models.NewContact())
	for _, key := range []string{"id", "displayName", "emailAddress", "emailAddresses"} {
		if _, ok := contact[key]; !ok {
			t.Errorf("contact summary is missing %q", key)
		}
	}

	person := SerializeSummaryPerson(models.NewPerson())
	for _, key := range []string{"id", "displayName", "emailAddress", "emailAddresses", "relevanceScore"} {
		if _, ok := person[key]; !ok {
			t.Errorf("person summary is missing %q", key)
		}
	}
}

// TestLabelContactMatch_StatesSource validates that a merged match carries the
// collection it came from, since a saved contact and an inferred person carry
// different confidence.
func TestLabelContactMatch_StatesSource(t *testing.T) {
	record := LabelContactMatch(SerializeSummaryContact(newTestContact()), contactSourceContact)
	if record["source"] != contactSourceContact {
		t.Errorf("source = %v, want %q", record["source"], contactSourceContact)
	}
}
