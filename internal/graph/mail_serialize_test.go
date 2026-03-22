package graph

import (
	"testing"
	"time"

	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// buildFullMessage constructs a fully populated Message with all fields set
// for use in serialization tests. It is a test helper that returns a
// models.Messageable with representative values for every field that
// SerializeMessage extracts.
func buildFullMessage() models.Messageable {
	msg := models.NewMessage()

	// Simple string fields.
	msg.SetId(ptr("msg-001"))
	msg.SetSubject(ptr("Q1 Planning"))
	msg.SetBodyPreview(ptr("Let's discuss the Q1 plan."))
	msg.SetConversationId(ptr("conv-abc"))
	msg.SetWebLink(ptr("https://outlook.office.com/mail/item/msg-001"))
	msg.SetInternetMessageId(ptr("<msg-001@example.com>"))
	msg.SetParentFolderId(ptr("folder-inbox"))

	// Boolean fields.
	msg.SetIsRead(boolPtr(true))
	msg.SetHasAttachments(boolPtr(true))

	// Importance enum.
	imp := models.HIGH_IMPORTANCE
	msg.SetImportance(&imp)

	// Categories.
	msg.SetCategories([]string{"Project", "Urgent"})

	// From recipient.
	fromEmail := models.NewEmailAddress()
	fromEmail.SetName(ptr("Alice"))
	fromEmail.SetAddress(ptr("alice@example.com"))
	from := models.NewRecipient()
	from.SetEmailAddress(fromEmail)
	msg.SetFrom(from)

	// To recipients.
	toEmail := models.NewEmailAddress()
	toEmail.SetName(ptr("Bob"))
	toEmail.SetAddress(ptr("bob@example.com"))
	toRecip := models.NewRecipient()
	toRecip.SetEmailAddress(toEmail)
	msg.SetToRecipients([]models.Recipientable{toRecip})

	// CC recipients.
	ccEmail := models.NewEmailAddress()
	ccEmail.SetName(ptr("Carol"))
	ccEmail.SetAddress(ptr("carol@example.com"))
	ccRecip := models.NewRecipient()
	ccRecip.SetEmailAddress(ccEmail)
	msg.SetCcRecipients([]models.Recipientable{ccRecip})

	// BCC recipients.
	bccEmail := models.NewEmailAddress()
	bccEmail.SetName(ptr("Dave"))
	bccEmail.SetAddress(ptr("dave@example.com"))
	bccRecip := models.NewRecipient()
	bccRecip.SetEmailAddress(bccEmail)
	msg.SetBccRecipients([]models.Recipientable{bccRecip})

	// Reply-to recipients.
	replyEmail := models.NewEmailAddress()
	replyEmail.SetName(ptr("Alice Reply"))
	replyEmail.SetAddress(ptr("alice-reply@example.com"))
	replyRecip := models.NewRecipient()
	replyRecip.SetEmailAddress(replyEmail)
	msg.SetReplyTo([]models.Recipientable{replyRecip})

	// Body.
	body := models.NewItemBody()
	bodyType := models.HTML_BODYTYPE
	body.SetContentType(&bodyType)
	body.SetContent(ptr("<p>Let's discuss the Q1 plan.</p>"))
	msg.SetBody(body)

	// Dates.
	received := time.Date(2026, 3, 19, 14, 30, 0, 0, time.UTC)
	msg.SetReceivedDateTime(&received)
	sent := time.Date(2026, 3, 19, 14, 29, 0, 0, time.UTC)
	msg.SetSentDateTime(&sent)

	// Conversation index.
	msg.SetConversationIndex([]byte{0x01, 0x02, 0x03})

	// Flag.
	flag := models.NewFollowupFlag()
	flagStatus := models.FLAGGED_FOLLOWUPFLAGSTATUS
	flag.SetFlagStatus(&flagStatus)
	msg.SetFlag(flag)

	// Internet message headers.
	header := models.NewInternetMessageHeader()
	header.SetName(ptr("X-Custom-Header"))
	header.SetValue(ptr("custom-value"))
	msg.SetInternetMessageHeaders([]models.InternetMessageHeaderable{header})

	return msg
}

// TestSerializeMessage_Full validates that SerializeMessage correctly extracts
// all fields from a fully populated Messageable, including nested objects for
// body, from, recipients, flag, and internet message headers.
func TestSerializeMessage_Full(t *testing.T) {
	msg := buildFullMessage()
	result := SerializeMessage(msg)

	// Verify simple string fields.
	strChecks := map[string]string{
		"id":                "msg-001",
		"subject":           "Q1 Planning",
		"bodyPreview":       "Let's discuss the Q1 plan.",
		"conversationId":    "conv-abc",
		"webLink":           "https://outlook.office.com/mail/item/msg-001",
		"internetMessageId": "<msg-001@example.com>",
		"parentFolderId":    "folder-inbox",
		"receivedDateTime":  "2026-03-19T14:30:00Z",
		"sentDateTime":      "2026-03-19T14:29:00Z",
		"importance":        "high",
		"flag":              "flagged",
	}
	for key, want := range strChecks {
		got, ok := result[key].(string)
		if !ok {
			t.Errorf("%s is %T, want string", key, result[key])
			continue
		}
		if got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}

	// Verify boolean fields.
	boolChecks := map[string]bool{
		"isRead":         true,
		"hasAttachments": true,
	}
	for key, want := range boolChecks {
		got, ok := result[key].(bool)
		if !ok {
			t.Errorf("%s is %T, want bool", key, result[key])
			continue
		}
		if got != want {
			t.Errorf("%s = %v, want %v", key, got, want)
		}
	}

	// Verify from.
	from, ok := result["from"].(map[string]string)
	if !ok {
		t.Fatalf("from is %T, want map[string]string", result["from"])
	}
	if from["name"] != "Alice" || from["address"] != "alice@example.com" {
		t.Errorf("from = %v, want name=Alice address=alice@example.com", from)
	}

	// Verify toRecipients.
	toRecips, ok := result["toRecipients"].([]map[string]string)
	if !ok {
		t.Fatalf("toRecipients is %T, want []map[string]string", result["toRecipients"])
	}
	if len(toRecips) != 1 || toRecips[0]["name"] != "Bob" {
		t.Errorf("toRecipients = %v, want [{name:Bob address:bob@example.com}]", toRecips)
	}

	// Verify ccRecipients.
	ccRecips, ok := result["ccRecipients"].([]map[string]string)
	if !ok {
		t.Fatalf("ccRecipients is %T, want []map[string]string", result["ccRecipients"])
	}
	if len(ccRecips) != 1 || ccRecips[0]["name"] != "Carol" {
		t.Errorf("ccRecipients = %v, want [{name:Carol}]", ccRecips)
	}

	// Verify bccRecipients.
	bccRecips, ok := result["bccRecipients"].([]map[string]string)
	if !ok {
		t.Fatalf("bccRecipients is %T, want []map[string]string", result["bccRecipients"])
	}
	if len(bccRecips) != 1 || bccRecips[0]["name"] != "Dave" {
		t.Errorf("bccRecipients = %v, want [{name:Dave}]", bccRecips)
	}

	// Verify replyTo.
	replyTo, ok := result["replyTo"].([]map[string]string)
	if !ok {
		t.Fatalf("replyTo is %T, want []map[string]string", result["replyTo"])
	}
	if len(replyTo) != 1 || replyTo[0]["address"] != "alice-reply@example.com" {
		t.Errorf("replyTo = %v, want [{address:alice-reply@example.com}]", replyTo)
	}

	// Verify body.
	body, ok := result["body"].(map[string]string)
	if !ok {
		t.Fatalf("body is %T, want map[string]string", result["body"])
	}
	if body["contentType"] != "html" {
		t.Errorf("body.contentType = %q, want %q", body["contentType"], "html")
	}
	if body["content"] != "<p>Let's discuss the Q1 plan.</p>" {
		t.Errorf("body.content = %q, want %q", body["content"], "<p>Let's discuss the Q1 plan.</p>")
	}

	// Verify categories.
	cats, ok := result["categories"].([]string)
	if !ok {
		t.Fatalf("categories is %T, want []string", result["categories"])
	}
	if len(cats) != 2 || cats[0] != "Project" || cats[1] != "Urgent" {
		t.Errorf("categories = %v, want [Project Urgent]", cats)
	}

	// Verify conversationIndex (base64 encoded).
	ci, ok := result["conversationIndex"].(string)
	if !ok {
		t.Fatalf("conversationIndex is %T, want string", result["conversationIndex"])
	}
	if ci != "AQID" {
		t.Errorf("conversationIndex = %q, want %q", ci, "AQID")
	}

	// Verify internetMessageHeaders.
	headers, ok := result["internetMessageHeaders"].([]map[string]string)
	if !ok {
		t.Fatalf("internetMessageHeaders is %T, want []map[string]string", result["internetMessageHeaders"])
	}
	if len(headers) != 1 || headers[0]["name"] != "X-Custom-Header" || headers[0]["value"] != "custom-value" {
		t.Errorf("internetMessageHeaders = %v, want [{name:X-Custom-Header value:custom-value}]", headers)
	}
}

// TestSerializeMessage_NilFields validates that SerializeMessage does not panic
// when all optional getter methods on the Messageable return nil. A freshly
// constructed Message with no setters called has nil for all optional fields.
func TestSerializeMessage_NilFields(t *testing.T) {
	msg := models.NewMessage()

	result := SerializeMessage(msg)

	// Verify no panic occurred and string fields have safe defaults.
	strFields := []string{
		"id", "subject", "bodyPreview", "conversationId", "webLink",
		"internetMessageId", "parentFolderId", "receivedDateTime",
		"sentDateTime", "importance", "flag", "conversationIndex",
	}
	for _, key := range strFields {
		got, ok := result[key].(string)
		if !ok {
			t.Errorf("%s is %T, want string", key, result[key])
			continue
		}
		if got != "" {
			t.Errorf("%s = %q, want %q", key, got, "")
		}
	}

	// Boolean fields default to false.
	boolFields := []string{"isRead", "hasAttachments"}
	for _, key := range boolFields {
		got, ok := result[key].(bool)
		if !ok {
			t.Errorf("%s is %T, want bool", key, result[key])
			continue
		}
		if got {
			t.Errorf("%s = %v, want false", key, got)
		}
	}

	// Categories default to empty slice.
	cats, ok := result["categories"].([]string)
	if !ok {
		t.Fatalf("categories is %T, want []string", result["categories"])
	}
	if len(cats) != 0 {
		t.Errorf("categories length = %d, want 0", len(cats))
	}

	// Recipient slices default to empty.
	recipientFields := []string{"toRecipients", "ccRecipients", "bccRecipients", "replyTo"}
	for _, key := range recipientFields {
		got, ok := result[key].([]map[string]string)
		if !ok {
			t.Errorf("%s is %T, want []map[string]string", key, result[key])
			continue
		}
		if len(got) != 0 {
			t.Errorf("%s length = %d, want 0", key, len(got))
		}
	}

	// Internet message headers default to empty slice.
	headers, ok := result["internetMessageHeaders"].([]map[string]string)
	if !ok {
		t.Fatalf("internetMessageHeaders is %T, want []map[string]string", result["internetMessageHeaders"])
	}
	if len(headers) != 0 {
		t.Errorf("internetMessageHeaders length = %d, want 0", len(headers))
	}

	// From should not be present when nil.
	if _, exists := result["from"]; exists {
		t.Error("from should not be present when message has no from recipient")
	}

	// Body should not be present when nil.
	if _, exists := result["body"]; exists {
		t.Error("body should not be present when message has no body")
	}
}

// TestSerializeSummaryMessage validates that SerializeSummaryMessage returns
// only the summary fields and excludes full-only fields like body,
// ccRecipients, bccRecipients, sentDateTime, conversationIndex,
// internetMessageId, parentFolderId, replyTo, and internetMessageHeaders.
func TestSerializeSummaryMessage(t *testing.T) {
	msg := buildFullMessage()
	result := SerializeSummaryMessage(msg)

	// Verify summary fields are present.
	summaryFields := []string{
		"id", "subject", "bodyPreview", "from", "toRecipients",
		"receivedDateTime", "importance", "isRead", "hasAttachments",
		"conversationId", "webLink", "categories", "flag",
	}
	for _, key := range summaryFields {
		if _, exists := result[key]; !exists {
			t.Errorf("summary field %q is missing", key)
		}
	}

	// Verify full-only fields are absent.
	fullOnlyFields := []string{
		"body", "ccRecipients", "bccRecipients", "sentDateTime",
		"conversationIndex", "internetMessageId", "parentFolderId",
		"replyTo", "internetMessageHeaders",
	}
	for _, key := range fullOnlyFields {
		if _, exists := result[key]; exists {
			t.Errorf("full-only field %q should not be present in summary", key)
		}
	}

	// Verify a few field values.
	if got := result["id"]; got != "msg-001" {
		t.Errorf("id = %q, want %q", got, "msg-001")
	}
	if got := result["subject"]; got != "Q1 Planning" {
		t.Errorf("subject = %q, want %q", got, "Q1 Planning")
	}
	from, ok := result["from"].(map[string]string)
	if !ok {
		t.Fatalf("from is %T, want map[string]string", result["from"])
	}
	if from["name"] != "Alice" {
		t.Errorf("from.name = %q, want %q", from["name"], "Alice")
	}
}

// TestToSummaryMessageMap validates that ToSummaryMessageMap correctly
// extracts only summary keys from a full serialized message map and
// discards full-only fields.
func TestToSummaryMessageMap(t *testing.T) {
	msg := buildFullMessage()
	full := SerializeMessage(msg)
	summary := ToSummaryMessageMap(full)

	// Verify summary fields are present.
	summaryFields := []string{
		"id", "subject", "bodyPreview", "from", "toRecipients",
		"receivedDateTime", "importance", "isRead", "hasAttachments",
		"conversationId", "webLink", "categories", "flag",
	}
	for _, key := range summaryFields {
		if _, exists := summary[key]; !exists {
			t.Errorf("summary field %q is missing after conversion", key)
		}
	}

	// Verify full-only fields are absent.
	fullOnlyFields := []string{
		"body", "ccRecipients", "bccRecipients", "sentDateTime",
		"conversationIndex", "internetMessageId", "parentFolderId",
		"replyTo", "internetMessageHeaders",
	}
	for _, key := range fullOnlyFields {
		if _, exists := summary[key]; exists {
			t.Errorf("full-only field %q should not be present in summary", key)
		}
	}

	// Verify values match the full map for summary keys.
	if got := summary["id"]; got != full["id"] {
		t.Errorf("id = %v, want %v", got, full["id"])
	}
	if got := summary["subject"]; got != full["subject"] {
		t.Errorf("subject = %v, want %v", got, full["subject"])
	}
}

// TestSerializeRecipients validates that serializeRecipients correctly
// converts a slice of Recipientable into a slice of maps, handles nil
// recipients, and returns an empty slice for nil input.
func TestSerializeRecipients(t *testing.T) {
	t.Run("multiple recipients", func(t *testing.T) {
		email1 := models.NewEmailAddress()
		email1.SetName(ptr("Alice"))
		email1.SetAddress(ptr("alice@example.com"))
		recip1 := models.NewRecipient()
		recip1.SetEmailAddress(email1)

		email2 := models.NewEmailAddress()
		email2.SetName(ptr("Bob"))
		email2.SetAddress(ptr("bob@example.com"))
		recip2 := models.NewRecipient()
		recip2.SetEmailAddress(email2)

		result := serializeRecipients([]models.Recipientable{recip1, recip2})

		if len(result) != 2 {
			t.Fatalf("result length = %d, want 2", len(result))
		}
		if result[0]["name"] != "Alice" || result[0]["address"] != "alice@example.com" {
			t.Errorf("result[0] = %v, want name=Alice address=alice@example.com", result[0])
		}
		if result[1]["name"] != "Bob" || result[1]["address"] != "bob@example.com" {
			t.Errorf("result[1] = %v, want name=Bob address=bob@example.com", result[1])
		}
	})

	t.Run("nil input", func(t *testing.T) {
		result := serializeRecipients(nil)
		if result == nil {
			t.Fatal("result is nil, want empty slice")
		}
		if len(result) != 0 {
			t.Errorf("result length = %d, want 0", len(result))
		}
	})

	t.Run("empty input", func(t *testing.T) {
		result := serializeRecipients([]models.Recipientable{})
		if result == nil {
			t.Fatal("result is nil, want empty slice")
		}
		if len(result) != 0 {
			t.Errorf("result length = %d, want 0", len(result))
		}
	})

	t.Run("recipient with nil email address", func(t *testing.T) {
		recip := models.NewRecipient()
		// Do not set email address — it will be nil.
		result := serializeRecipients([]models.Recipientable{recip})
		// Recipient with nil email returns nil from serializeRecipient,
		// so it is skipped.
		if len(result) != 0 {
			t.Errorf("result length = %d, want 0 (nil email recipient skipped)", len(result))
		}
	})
}
