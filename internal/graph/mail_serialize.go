package graph

import (
	"encoding/base64"
	"time"

	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// serializeEmailAddress extracts name and address from an EmailAddressable
// into a map[string]string. Returns nil when ea is nil.
//
// Parameters:
//   - ea: an EmailAddressable from the Microsoft Graph SDK. May be nil.
//
// Returns a map with "name" and "address" keys, or nil if ea is nil.
//
// Side effects: none.
func serializeEmailAddress(ea models.EmailAddressable) map[string]string {
	if ea == nil {
		return nil
	}
	return map[string]string{
		"name":    SafeStr(ea.GetName()),
		"address": SafeStr(ea.GetAddress()),
	}
}

// serializeRecipient extracts the email address from a Recipientable into a
// map[string]string. Returns nil when r is nil or when its email address is
// nil.
//
// Parameters:
//   - r: a Recipientable from the Microsoft Graph SDK. May be nil.
//
// Returns a map with "name" and "address" keys, or nil if r or its email
// address is nil.
//
// Side effects: none.
func serializeRecipient(r models.Recipientable) map[string]string {
	if r == nil {
		return nil
	}
	return serializeEmailAddress(r.GetEmailAddress())
}

// serializeRecipients converts a slice of Recipientable into a slice of
// maps. Each recipient is serialized via serializeRecipient; nil recipients
// are skipped. Returns an empty slice (not nil) when the input is nil or
// empty.
//
// Parameters:
//   - recipients: a slice of Recipientable from the Microsoft Graph SDK.
//     May be nil.
//
// Returns a slice of maps with "name" and "address" keys for each non-nil
// recipient. Always returns a non-nil slice.
//
// Side effects: none.
func serializeRecipients(recipients []models.Recipientable) []map[string]string {
	if len(recipients) == 0 {
		return []map[string]string{}
	}
	result := make([]map[string]string, 0, len(recipients))
	for _, r := range recipients {
		if m := serializeRecipient(r); m != nil {
			result = append(result, m)
		}
	}
	return result
}

// serializeFlag extracts the flag status from a FollowupFlagable into a
// string. Returns an empty string when flag is nil or its status is nil.
//
// Parameters:
//   - flag: a FollowupFlagable from the Microsoft Graph SDK. May be nil.
//
// Returns the string representation of the flag status, or "" if nil.
//
// Side effects: none.
func serializeFlag(flag models.FollowupFlagable) string {
	if flag == nil {
		return ""
	}
	if status := flag.GetFlagStatus(); status != nil {
		return status.String()
	}
	return ""
}

// serializeBody extracts content and contentType from an ItemBodyable into
// a map[string]string. Returns nil when body is nil.
//
// Parameters:
//   - body: an ItemBodyable from the Microsoft Graph SDK. May be nil.
//
// Returns a map with "contentType" and "content" keys, or nil if body is nil.
//
// Side effects: none.
func serializeBody(body models.ItemBodyable) map[string]string {
	if body == nil {
		return nil
	}
	ct := ""
	if t := body.GetContentType(); t != nil {
		ct = t.String()
	}
	return map[string]string{
		"contentType": ct,
		"content":     SafeStr(body.GetContent()),
	}
}

// serializeInternetMessageHeaders converts a slice of
// InternetMessageHeaderable into a slice of maps with "name" and "value"
// keys. Returns an empty slice when headers is nil or empty.
//
// Parameters:
//   - headers: a slice of InternetMessageHeaderable from the Microsoft Graph
//     SDK. May be nil.
//
// Returns a slice of maps. Always returns a non-nil slice.
//
// Side effects: none.
func serializeInternetMessageHeaders(headers []models.InternetMessageHeaderable) []map[string]string {
	if len(headers) == 0 {
		return []map[string]string{}
	}
	result := make([]map[string]string, 0, len(headers))
	for _, h := range headers {
		if h == nil {
			continue
		}
		result = append(result, map[string]string{
			"name":  SafeStr(h.GetName()),
			"value": SafeStr(h.GetValue()),
		})
	}
	return result
}

// safeTimeStr formats a *time.Time pointer as an RFC3339 string. Returns an
// empty string when t is nil.
//
// Parameters:
//   - t: a pointer to time.Time. May be nil.
//
// Returns the RFC3339 string representation, or "" if t is nil.
//
// Side effects: none.
func safeTimeStr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// messageSummaryKeys lists the field names included in the summary message
// format. These fields are returned by SerializeSummaryMessage and extracted
// by ToSummaryMessageMap.
var messageSummaryKeys = []string{
	"id", "subject", "bodyPreview", "from", "toRecipients",
	"receivedDateTime", "importance", "isRead", "hasAttachments",
	"conversationId", "webLink", "categories", "flag",
}

// SerializeMessage extracts the full set of fields from a models.Messageable
// into a map[string]any suitable for JSON serialization in MCP tool responses.
// All pointer fields are accessed through SafeStr, SafeBool, and safeTimeStr
// helpers to prevent nil dereference panics. Nested objects (body, from,
// recipients, flag, headers) are nil-checked before field extraction.
//
// This function returns all summary fields plus additional detail fields:
// body, ccRecipients, bccRecipients, sentDateTime, conversationIndex,
// internetMessageId, parentFolderId, replyTo, and internetMessageHeaders.
//
// Parameters:
//   - msg: a models.Messageable representing an email message from the
//     Microsoft Graph API. All getter methods may return nil.
//
// Returns a map containing the full message fields. Nested object fields
// use sub-maps or slices of sub-maps. Nil collections default to empty
// slices.
//
// Side effects: none.
func SerializeMessage(msg models.Messageable) map[string]any {
	result := map[string]any{
		// Summary fields.
		"id":               SafeStr(msg.GetId()),
		"subject":          SafeStr(msg.GetSubject()),
		"bodyPreview":      SafeStr(msg.GetBodyPreview()),
		"toRecipients":     serializeRecipients(msg.GetToRecipients()),
		"receivedDateTime": safeTimeStr(msg.GetReceivedDateTime()),
		"isRead":           SafeBool(msg.GetIsRead()),
		"hasAttachments":   SafeBool(msg.GetHasAttachments()),
		"conversationId":   SafeStr(msg.GetConversationId()),
		"webLink":          SafeStr(msg.GetWebLink()),
		"flag":             serializeFlag(msg.GetFlag()),

		// Full-only fields.
		"ccRecipients":           serializeRecipients(msg.GetCcRecipients()),
		"bccRecipients":          serializeRecipients(msg.GetBccRecipients()),
		"sentDateTime":           safeTimeStr(msg.GetSentDateTime()),
		"internetMessageId":      SafeStr(msg.GetInternetMessageId()),
		"parentFolderId":         SafeStr(msg.GetParentFolderId()),
		"replyTo":                serializeRecipients(msg.GetReplyTo()),
		"internetMessageHeaders": serializeInternetMessageHeaders(msg.GetInternetMessageHeaders()),
	}

	// From: single Recipientable, may be nil.
	if from := serializeRecipient(msg.GetFrom()); from != nil {
		result["from"] = from
	}

	// Importance: enum with String() method.
	if imp := msg.GetImportance(); imp != nil {
		result["importance"] = imp.String()
	} else {
		result["importance"] = ""
	}

	// Categories: default to empty slice if nil.
	if cats := msg.GetCategories(); cats != nil {
		result["categories"] = cats
	} else {
		result["categories"] = []string{}
	}

	// Body: nested ItemBodyable with contentType and content.
	if body := serializeBody(msg.GetBody()); body != nil {
		result["body"] = body
	}

	// ConversationIndex: []byte encoded as base64.
	if ci := msg.GetConversationIndex(); ci != nil {
		result["conversationIndex"] = base64.StdEncoding.EncodeToString(ci)
	} else {
		result["conversationIndex"] = ""
	}

	return result
}

// SerializeSummaryMessage extracts a minimal set of fields from a
// models.Messageable into a map[string]any suitable for compact summary
// responses. Unlike SerializeMessage, full-only fields (body, ccRecipients,
// bccRecipients, sentDateTime, conversationIndex, internetMessageId,
// parentFolderId, replyTo, internetMessageHeaders) are excluded.
//
// Parameters:
//   - msg: a models.Messageable representing an email message from the
//     Microsoft Graph API. All getter methods may return nil.
//
// Returns a map containing only the summary fields. All values have safe
// defaults when the source field is nil.
//
// Side effects: none.
func SerializeSummaryMessage(msg models.Messageable) map[string]any {
	result := map[string]any{
		"id":               SafeStr(msg.GetId()),
		"subject":          SafeStr(msg.GetSubject()),
		"bodyPreview":      SafeStr(msg.GetBodyPreview()),
		"toRecipients":     serializeRecipients(msg.GetToRecipients()),
		"receivedDateTime": safeTimeStr(msg.GetReceivedDateTime()),
		"isRead":           SafeBool(msg.GetIsRead()),
		"hasAttachments":   SafeBool(msg.GetHasAttachments()),
		"conversationId":   SafeStr(msg.GetConversationId()),
		"webLink":          SafeStr(msg.GetWebLink()),
		"flag":             serializeFlag(msg.GetFlag()),
	}

	// From: single Recipientable, may be nil.
	if from := serializeRecipient(msg.GetFrom()); from != nil {
		result["from"] = from
	}

	// Importance: enum with String() method.
	if imp := msg.GetImportance(); imp != nil {
		result["importance"] = imp.String()
	} else {
		result["importance"] = ""
	}

	// Categories: default to empty slice if nil.
	if cats := msg.GetCategories(); cats != nil {
		result["categories"] = cats
	} else {
		result["categories"] = []string{}
	}

	return result
}

// SerializeAttachment extracts attachment metadata and, when the attachment
// is a FileAttachment, the base64-encoded content bytes. The returned map is
// suitable for JSON serialization in MCP tool responses.
//
// Fields always present: id, name, contentType, size, isInline,
// lastModifiedDateTime, odataType. When att is a FileAttachment, the map also
// includes contentBytes (base64 string).
//
// Parameters:
//   - att: an Attachmentable from the Microsoft Graph SDK. May be nil.
//
// Returns a map containing the attachment fields. Returns an empty map when
// att is nil.
//
// Side effects: none.
func SerializeAttachment(att models.Attachmentable) map[string]any {
	if att == nil {
		return map[string]any{}
	}
	result := map[string]any{
		"id":                   SafeStr(att.GetId()),
		"name":                 SafeStr(att.GetName()),
		"contentType":          SafeStr(att.GetContentType()),
		"isInline":             SafeBool(att.GetIsInline()),
		"lastModifiedDateTime": "",
	}
	if sz := att.GetSize(); sz != nil {
		result["size"] = int64(*sz)
	} else {
		result["size"] = int64(0)
	}
	if t := att.GetLastModifiedDateTime(); t != nil {
		result["lastModifiedDateTime"] = t.Format(time.RFC3339)
	}
	if ot := att.GetOdataType(); ot != nil {
		result["odataType"] = *ot
	} else {
		result["odataType"] = ""
	}
	if fa, ok := att.(*models.FileAttachment); ok && fa != nil {
		if b := fa.GetContentBytes(); b != nil {
			result["contentBytes"] = base64.StdEncoding.EncodeToString(b)
		} else {
			result["contentBytes"] = ""
		}
	}
	return result
}

// SerializeSummaryAttachment returns a compact attachment map suitable for
// the "summary" output mode of mail_get_attachment. The field set matches
// SerializeAttachment but is intentionally frozen here to document the
// summary contract; today the two maps are equivalent because attachment
// metadata is already small.
//
// Parameters:
//   - att: an Attachmentable from the Microsoft Graph SDK. May be nil.
//
// Returns a map containing the summary attachment fields.
//
// Side effects: none.
func SerializeSummaryAttachment(att models.Attachmentable) map[string]any {
	full := SerializeAttachment(att)
	summaryKeys := []string{"id", "name", "contentType", "size", "isInline", "contentBytes"}
	out := make(map[string]any, len(summaryKeys))
	for _, k := range summaryKeys {
		if v, ok := full[k]; ok {
			out[k] = v
		}
	}
	return out
}

// SerializeConversationThread wraps an ordered slice of per-message maps with
// the shared conversationId and a message count. The slice is assumed to be
// sorted chronologically (oldest first).
//
// Parameters:
//   - conversationID: the conversation identifier shared by every message.
//   - messages: the serialized message maps in chronological order.
//
// Returns a map with "conversationId", "count", and "messages" keys.
//
// Side effects: none.
func SerializeConversationThread(conversationID string, messages []map[string]any) map[string]any {
	return map[string]any{
		"conversationId": conversationID,
		"count":          len(messages),
		"messages":       messages,
	}
}

// ToSummaryMessageMap converts a full serialized message map (from
// SerializeMessage) to summary format by extracting only the summary fields.
// Fields not present in the full map are omitted from the result.
//
// Parameters:
//   - full: a map produced by SerializeMessage with the full field set.
//
// Returns a new map containing only the summary fields that exist in the
// full map.
//
// Side effects: none.
func ToSummaryMessageMap(full map[string]any) map[string]any {
	result := make(map[string]any, len(messageSummaryKeys))
	for _, key := range messageSummaryKeys {
		if v, ok := full[key]; ok {
			result[key] = v
		}
	}
	return result
}
