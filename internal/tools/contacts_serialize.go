// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the serializers for the contacts domain. A saved contact
// and a relevance-ranked person answer the same question — who is this name and
// what address do I write to — but they are unrelated resources in the Graph
// SDK: a contact carries EmailAddressable entries and a person carries
// ScoredEmailAddressable entries, and the two element types share no interface.
// Each resource therefore gets its own summary and raw serializer rather than
// one function parameterised over a common address type, which would not
// type-check.
//
// @agents-index: Summary and raw serializers for Graph contacts and people,
// projecting each resource's own address type into the contacts domain's
// output tiers.
package tools

import (
	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// Source labels distinguishing where a search match came from. A saved contact
// is something the user wrote down; a ranked person is someone Graph inferred
// the user corresponds with. A caller resolving a name needs to tell them
// apart, so the label travels with every match rather than being implied by
// position in the merged list.
const (
	contactSourceContact = "personal contact"
	contactSourcePerson  = "ranked person"
)

// SerializeSummaryContact projects a saved contact onto the field set a
// name-to-address resolution actually needs: who the contact is, the address to
// write to, and the identifier to fetch the full record with. The field set is
// chosen here rather than derived by dropping empty values from the raw
// payload, so the summary tier's shape is stable regardless of how sparsely the
// mailbox populated the contact.
//
// emailAddress carries the first entry of the contact's address collection, the
// one a caller composing a message should use, and emailAddresses carries every
// entry so no address the contact holds is lost to the tier.
//
// Side effects: none.
func SerializeSummaryContact(contact models.Contactable) map[string]any {
	if contact == nil {
		return map[string]any{}
	}

	addresses := contactEmailAddresses(contact)
	return map[string]any{
		"id":             graph.SafeStr(contact.GetId()),
		"displayName":    graph.SafeStr(contact.GetDisplayName()),
		"emailAddress":   firstAddress(addresses),
		"emailAddresses": addresses,
	}
}

// SerializeContact projects a saved contact onto the raw tier: the names,
// addresses, phone numbers, and postal addresses the verb's description
// promises, so a caller that escalated to raw needs no further request to see
// what the contact holds.
//
// Side effects: none.
func SerializeContact(contact models.Contactable) map[string]any {
	if contact == nil {
		return map[string]any{}
	}

	emails := make([]map[string]string, 0, len(contact.GetEmailAddresses()))
	for _, entry := range contact.GetEmailAddresses() {
		if entry == nil {
			continue
		}
		emails = append(emails, map[string]string{
			"name":    graph.SafeStr(entry.GetName()),
			"address": graph.SafeStr(entry.GetAddress()),
		})
	}

	return map[string]any{
		"id":              graph.SafeStr(contact.GetId()),
		"displayName":     graph.SafeStr(contact.GetDisplayName()),
		"givenName":       graph.SafeStr(contact.GetGivenName()),
		"surname":         graph.SafeStr(contact.GetSurname()),
		"nickName":        graph.SafeStr(contact.GetNickName()),
		"companyName":     graph.SafeStr(contact.GetCompanyName()),
		"jobTitle":        graph.SafeStr(contact.GetJobTitle()),
		"department":      graph.SafeStr(contact.GetDepartment()),
		"officeLocation":  graph.SafeStr(contact.GetOfficeLocation()),
		"emailAddresses":  emails,
		"businessPhones":  contact.GetBusinessPhones(),
		"homePhones":      contact.GetHomePhones(),
		"mobilePhone":     graph.SafeStr(contact.GetMobilePhone()),
		"businessAddress": serializePhysicalAddress(contact.GetBusinessAddress()),
		"homeAddress":     serializePhysicalAddress(contact.GetHomeAddress()),
		"otherAddress":    serializePhysicalAddress(contact.GetOtherAddress()),
		"personalNotes":   graph.SafeStr(contact.GetPersonalNotes()),
	}
}

// SerializeSummaryPerson projects a relevance-ranked person onto the same
// resolution field set as the contact summary, so a merged search result reads
// uniformly whichever resource a match came from. The relevance score of the
// leading address is carried because it is the only signal distinguishing a
// person Graph is confident about from one it merely observed once.
//
// Side effects: none.
func SerializeSummaryPerson(person models.Personable) map[string]any {
	if person == nil {
		return map[string]any{}
	}

	addresses, score := personEmailAddresses(person)
	return map[string]any{
		"id":             graph.SafeStr(person.GetId()),
		"displayName":    graph.SafeStr(person.GetDisplayName()),
		"emailAddress":   firstAddress(addresses),
		"emailAddresses": addresses,
		"relevanceScore": score,
	}
}

// SerializePerson projects a relevance-ranked person onto the raw tier. A
// person carries no per-address name in the SDK, so each address is rendered
// with its relevance score alone and the person's own display name is the label
// a reader attaches to it.
//
// Side effects: none.
func SerializePerson(person models.Personable) map[string]any {
	if person == nil {
		return map[string]any{}
	}

	scored := make([]map[string]any, 0, len(person.GetScoredEmailAddresses()))
	for _, entry := range person.GetScoredEmailAddresses() {
		if entry == nil {
			continue
		}
		record := map[string]any{
			"address":        graph.SafeStr(entry.GetAddress()),
			"relevanceScore": 0.0,
		}
		if s := entry.GetRelevanceScore(); s != nil {
			record["relevanceScore"] = *s
		}
		scored = append(scored, record)
	}

	phones := make([]map[string]string, 0, len(person.GetPhones()))
	for _, phone := range person.GetPhones() {
		if phone == nil {
			continue
		}
		record := map[string]string{
			"number": graph.SafeStr(phone.GetNumber()),
			"type":   "",
		}
		if t := phone.GetTypeEscaped(); t != nil {
			record["type"] = t.String()
		}
		phones = append(phones, record)
	}

	result := map[string]any{
		"id":                   graph.SafeStr(person.GetId()),
		"displayName":          graph.SafeStr(person.GetDisplayName()),
		"givenName":            graph.SafeStr(person.GetGivenName()),
		"surname":              graph.SafeStr(person.GetSurname()),
		"companyName":          graph.SafeStr(person.GetCompanyName()),
		"jobTitle":             graph.SafeStr(person.GetJobTitle()),
		"department":           graph.SafeStr(person.GetDepartment()),
		"officeLocation":       graph.SafeStr(person.GetOfficeLocation()),
		"userPrincipalName":    graph.SafeStr(person.GetUserPrincipalName()),
		"isFavorite":           graph.SafeBool(person.GetIsFavorite()),
		"scoredEmailAddresses": scored,
		"phones":               phones,
	}

	if pt := person.GetPersonType(); pt != nil {
		result["personType"] = map[string]string{
			"class":    graph.SafeStr(pt.GetClass()),
			"subclass": graph.SafeStr(pt.GetSubclass()),
		}
	}

	return result
}

// LabelContactMatch stamps a serialized contacts-domain record with the
// resource it came from, so a merged search result states its provenance per
// match instead of leaving the caller to infer it.
//
// Parameters:
//   - record: a serialized contact or person map, mutated in place.
//   - source: one of contactSourceContact or contactSourcePerson.
//
// Returns the same map, so the call composes inside an append.
func LabelContactMatch(record map[string]any, source string) map[string]any {
	record["source"] = source
	return record
}

// contactEmailAddresses reduces a contact's address collection to the address
// strings alone, dropping entries Graph returned without an address so an empty
// string never reaches a caller as if it were one.
func contactEmailAddresses(contact models.Contactable) []string {
	addresses := make([]string, 0, len(contact.GetEmailAddresses()))
	for _, entry := range contact.GetEmailAddresses() {
		if entry == nil {
			continue
		}
		if address := graph.SafeStr(entry.GetAddress()); address != "" {
			addresses = append(addresses, address)
		}
	}
	return addresses
}

// personEmailAddresses reduces a person's scored address collection to the
// address strings alone and returns the relevance score of the leading entry,
// which is the score a caller weighing one candidate against another needs.
func personEmailAddresses(person models.Personable) ([]string, float64) {
	addresses := make([]string, 0, len(person.GetScoredEmailAddresses()))
	score := 0.0
	for _, entry := range person.GetScoredEmailAddresses() {
		if entry == nil {
			continue
		}
		address := graph.SafeStr(entry.GetAddress())
		if address == "" {
			continue
		}
		if len(addresses) == 0 {
			if s := entry.GetRelevanceScore(); s != nil {
				score = *s
			}
		}
		addresses = append(addresses, address)
	}
	return addresses, score
}

// firstAddress returns the leading address of a reduced address list, or an
// empty string when the record carries none.
func firstAddress(addresses []string) string {
	if len(addresses) == 0 {
		return ""
	}
	return addresses[0]
}

// serializePhysicalAddress projects a Graph postal address into a flat map,
// returning an empty map when the contact holds none, so the raw tier's shape
// does not change with how completely the contact was filled in.
func serializePhysicalAddress(address models.PhysicalAddressable) map[string]string {
	if address == nil {
		return map[string]string{}
	}
	return map[string]string{
		"street":          graph.SafeStr(address.GetStreet()),
		"city":            graph.SafeStr(address.GetCity()),
		"state":           graph.SafeStr(address.GetState()),
		"postalCode":      graph.SafeStr(address.GetPostalCode()),
		"countryOrRegion": graph.SafeStr(address.GetCountryOrRegion()),
	}
}
