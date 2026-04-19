// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides shared helpers used by the mail draft management tools
// (mail_create_draft, mail_create_reply_draft, mail_create_forward_draft,
// mail_update_draft). The helpers convert comma-separated recipient strings to
// Graph SDK Recipientable slices, build ItemBody from content/content_type
// parameters, and format draft confirmation text.
package tools

import (
	"fmt"
	"strings"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// BuildRecipients converts a slice of validated email address strings into
// a slice of models.Recipientable for the Graph SDK. Each recipient is
// constructed with its address set on a fresh EmailAddress, with no display
// name.
//
// Parameters:
//   - addrs: a slice of validated email address strings.
//
// Returns the Recipientable slice, or nil when addrs is empty.
//
// Side effects: none.
func BuildRecipients(addrs []string) []models.Recipientable {
	if len(addrs) == 0 {
		return nil
	}
	out := make([]models.Recipientable, 0, len(addrs))
	for _, a := range addrs {
		addr := a
		ea := models.NewEmailAddress()
		ea.SetAddress(&addr)
		r := models.NewRecipient()
		r.SetEmailAddress(ea)
		out = append(out, r)
	}
	return out
}

// BuildDraftBody constructs a models.ItemBody from the body content and
// content_type parameters. content_type is case-insensitive and defaults to
// "text" when empty. "html" selects models.HTML_BODYTYPE; any other value
// has already been rejected by ValidateContentType.
//
// Parameters:
//   - content: the body string (plain text or HTML).
//   - contentType: the content type string ("text" or "html"); empty defaults
//     to "text".
//
// Returns a configured ItemBody.
//
// Side effects: none.
func BuildDraftBody(content, contentType string) models.ItemBodyable {
	body := models.NewItemBody()
	ct := models.TEXT_BODYTYPE
	if strings.EqualFold(contentType, "html") {
		ct = models.HTML_BODYTYPE
	}
	body.SetContentType(&ct)
	body.SetContent(&content)
	return body
}

// FormatDraftConfirmation formats a concise plain-text confirmation for draft
// write operations. The output includes the action verb, subject (or
// "(No subject)" when empty), the draft ID, and a trailing line explaining
// that the draft is available in the user's Drafts folder and is not sent
// automatically.
//
// Parameters:
//   - action: the action verb (e.g., "created", "updated", "deleted").
//   - subject: the draft subject/title. May be empty.
//   - draftID: the Graph API message ID of the draft.
//
// Returns a multi-line text confirmation.
//
// Side effects: none.
func FormatDraftConfirmation(action, subject, draftID string) string {
	display := subject
	if display == "" {
		display = "(No subject)"
	}
	return fmt.Sprintf("Draft %s: %q\nID: %s\nThe draft is available in the Drafts folder for review and manual send.", action, display, draftID)
}

// MaybeSetMailProvenance stamps the MCP provenance extended property on the
// given message when propertyID is non-empty. When propertyID is empty the
// call is a no-op, keeping provenance opt-in via the ProvenanceTag server
// config.
//
// Parameters:
//   - msg: a models.Messageable (typically a draft) on which to set the
//     extended property.
//   - propertyID: the full provenance property ID from
//     graph.BuildProvenancePropertyID. Empty disables tagging.
//
// Side effects: mutates msg by calling SetSingleValueExtendedProperties.
func MaybeSetMailProvenance(msg models.Messageable, propertyID string) {
	if propertyID == "" || msg == nil {
		return
	}
	msg.SetSingleValueExtendedProperties(
		[]models.SingleValueLegacyExtendedPropertyable{
			graph.NewProvenanceProperty(propertyID),
		},
	)
}
