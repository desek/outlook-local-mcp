// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file contains the serializer tests for the Teams domain: that each tier
// carries the field set its verbs promise, that the three message shapes are
// keyed by the identifiers their own reads need, that a long body is previewed
// in the summary tier and whole in the raw one, and that a sparsely populated
// record still projects to a stable shape rather than a shorter map.
//
// @agents-index: Serializer tests for the Teams domain covering the summary and
// raw projections of chats, messages, replies, meetings, and transcripts.
package tools

import (
	"strings"
	"testing"
	"time"

	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// teamsTestTime is a fixed instant every timestamped fixture uses, so a test
// asserting a rendered timestamp asserts the formatting and not the clock.
var teamsTestTime = time.Date(2026, 4, 1, 9, 30, 0, 0, time.UTC)

// newTestItemBody builds a message body carrying HTML content, which is the
// common Teams case and the one whose content type a reader has to be told.
func newTestItemBody(content string) models.ItemBodyable {
	body := models.NewItemBody()
	body.SetContent(&content)
	contentType := models.HTML_BODYTYPE
	body.SetContentType(&contentType)
	return body
}

// newTestChatMessage builds a chat message with a sender, a body, and one
// attachment, which is the shape the chat reads project.
func newTestChatMessage() models.ChatMessageable {
	msg := models.NewChatMessage()
	id, chatID := "msg-1", "19:chat-1"
	msg.SetId(&id)
	msg.SetChatId(&chatID)
	msg.SetBody(newTestItemBody("Shipping Friday."))
	msg.SetCreatedDateTime(&teamsTestTime)

	name := "Alex Stone"
	user := models.NewIdentity()
	user.SetDisplayName(&name)
	from := models.NewChatMessageFromIdentitySet()
	from.SetUser(user)
	msg.SetFrom(from)

	attName := "plan.pdf"
	att := models.NewChatMessageAttachment()
	att.SetName(&attName)
	msg.SetAttachments([]models.ChatMessageAttachmentable{att})

	return msg
}

// newTestChannelMessage builds a top-level channel message, whose defining
// feature is the channelIdentity carrying the team and channel identifiers a
// subsequent read is addressed by.
func newTestChannelMessage() models.ChatMessageable {
	msg := models.NewChatMessage()
	id, subject := "cmsg-1", "Release plan"
	msg.SetId(&id)
	msg.SetSubject(&subject)
	msg.SetBody(newTestItemBody("Agenda attached."))
	msg.SetCreatedDateTime(&teamsTestTime)

	teamID, channelID := "team-1", "19:channel-1"
	identity := models.NewChannelIdentity()
	identity.SetTeamId(&teamID)
	identity.SetChannelId(&channelID)
	msg.SetChannelIdentity(identity)

	return msg
}

// TestSerializeSummaryChatCarriesResolutionFields validates that the chat
// summary carries what a caller picking a conversation needs, including the last
// message preview that distinguishes untitled one-to-one chats from each other.
func TestSerializeSummaryChatCarriesResolutionFields(t *testing.T) {
	chat := models.NewChat()
	id, topic := "19:chat-1", "Release"
	chat.SetId(&id)
	chat.SetTopic(&topic)
	chatType := models.GROUP_CHATTYPE
	chat.SetChatType(&chatType)
	chat.SetLastUpdatedDateTime(&teamsTestTime)

	preview := models.NewChatMessageInfo()
	preview.SetBody(newTestItemBody("Shipping Friday."))
	chat.SetLastMessagePreview(preview)

	result := SerializeSummaryChat(chat)

	if got := result["id"]; got != "19:chat-1" {
		t.Errorf("id = %v, want %q", got, "19:chat-1")
	}
	if got := result["chatType"]; got != "group" {
		t.Errorf("chatType = %v, want %q", got, "group")
	}
	if got := result["lastUpdatedDateTime"]; got != "2026-04-01T09:30:00Z" {
		t.Errorf("lastUpdatedDateTime = %v, want %q", got, "2026-04-01T09:30:00Z")
	}
	if got := result["lastMessagePreview"]; got != "Shipping Friday." {
		t.Errorf("lastMessagePreview = %v, want %q", got, "Shipping Friday.")
	}
}

// TestSerializeChatProjectsSparseRecordToStableShape validates that a chat Graph
// returned with nothing but an identifier still projects every raw field, so a
// consumer never has to distinguish an absent key from an empty value.
func TestSerializeChatProjectsSparseRecordToStableShape(t *testing.T) {
	chat := models.NewChat()
	id := "19:chat-empty"
	chat.SetId(&id)

	result := SerializeChat(chat)

	for _, key := range []string{
		"id", "topic", "chatType", "createdDateTime", "lastUpdatedDateTime",
		"lastMessagePreview", "members", "isHiddenForAllMembers", "tenantId",
		"webUrl", "onlineMeetingJoinWebUrl",
	} {
		if _, ok := result[key]; !ok {
			t.Errorf("raw chat is missing key %q", key)
		}
	}

	members, ok := result["members"].([]map[string]any)
	if !ok || len(members) != 0 {
		t.Errorf("members = %#v, want an empty slice", result["members"])
	}
}

// TestSerializeSummaryChatMessageKeysByChat validates that the chat message
// summary carries the chat identifier its follow-up reads are addressed by, the
// sender's display name, and an attachment count rather than the attachments.
func TestSerializeSummaryChatMessageKeysByChat(t *testing.T) {
	result := SerializeSummaryChatMessage(newTestChatMessage())

	if got := result["chatId"]; got != "19:chat-1" {
		t.Errorf("chatId = %v, want %q", got, "19:chat-1")
	}
	if got := result["from"]; got != "Alex Stone" {
		t.Errorf("from = %v, want %q", got, "Alex Stone")
	}
	if got := result["attachmentCount"]; got != 1 {
		t.Errorf("attachmentCount = %v, want 1", got)
	}
	if _, present := result["teamId"]; present {
		t.Error("a chat message summary must not carry a teamId; it has no channel identity")
	}
}

// TestSerializeSummaryChannelMessageKeysByTeamAndChannel validates that the
// channel message summary carries both channel coordinates, since neither
// addresses a channel read on its own, and the subject that titles the thread.
func TestSerializeSummaryChannelMessageKeysByTeamAndChannel(t *testing.T) {
	result := SerializeSummaryChannelMessage(newTestChannelMessage())

	if got := result["teamId"]; got != "team-1" {
		t.Errorf("teamId = %v, want %q", got, "team-1")
	}
	if got := result["channelId"]; got != "19:channel-1" {
		t.Errorf("channelId = %v, want %q", got, "19:channel-1")
	}
	if got := result["subject"]; got != "Release plan" {
		t.Errorf("subject = %v, want %q", got, "Release plan")
	}
}

// TestSerializeSummaryMessageReplyCarriesParentIdentifier validates that a reply
// summary states which message it hangs under, which is the field distinguishing
// it from the top-level message shapes.
func TestSerializeSummaryMessageReplyCarriesParentIdentifier(t *testing.T) {
	msg := models.NewChatMessage()
	id, parent := "reply-1", "cmsg-1"
	msg.SetId(&id)
	msg.SetReplyToId(&parent)
	msg.SetBody(newTestItemBody("Agreed."))

	result := SerializeSummaryMessageReply(msg)

	if got := result["replyToId"]; got != "cmsg-1" {
		t.Errorf("replyToId = %v, want %q", got, "cmsg-1")
	}
	if got := result["bodyPreview"]; got != "Agreed." {
		t.Errorf("bodyPreview = %v, want %q", got, "Agreed.")
	}
}

// TestSummaryPreviewsBodyWhileRawCarriesWholeText validates the body escalation
// at the serializer layer: the summary tier truncates and says so, the raw tier
// returns every character.
func TestSummaryPreviewsBodyWhileRawCarriesWholeText(t *testing.T) {
	long := strings.Repeat("a", teamsPreviewRunes+120)
	msg := models.NewChatMessage()
	id := "msg-long"
	msg.SetId(&id)
	msg.SetBody(newTestItemBody(long))

	preview, _ := SerializeSummaryChatMessage(msg)["bodyPreview"].(string)
	if !strings.HasSuffix(preview, teamsTruncationMarker) {
		t.Errorf("summary body of a long message must state that it was truncated; got %q", preview)
	}
	if got := len([]rune(preview)); got != teamsPreviewRunes+len([]rune(teamsTruncationMarker)) {
		t.Errorf("preview length = %d runes, want %d", got, teamsPreviewRunes+len([]rune(teamsTruncationMarker)))
	}

	if got := SerializeChatMessage(msg)["body"]; got != long {
		t.Error("the raw tier must carry the whole body, not a preview")
	}
	if got := SerializeChatMessage(msg)["bodyContentType"]; got != "html" {
		t.Errorf("bodyContentType = %v, want %q", got, "html")
	}
}

// TestTeamsPreviewCutsOnRuneBoundary validates that truncation never splits a
// multi-byte character, which would put an invalid rune into a tool result.
func TestTeamsPreviewCutsOnRuneBoundary(t *testing.T) {
	content := strings.Repeat("ä", teamsPreviewRunes+10)

	preview := TeamsPreview(content)

	body := strings.TrimSuffix(preview, teamsTruncationMarker)
	if got := len([]rune(body)); got != teamsPreviewRunes {
		t.Errorf("preview body = %d runes, want %d", got, teamsPreviewRunes)
	}
	for _, r := range body {
		if r != 'ä' {
			t.Fatalf("preview contains a character the source did not: %q", r)
		}
	}
}

// TestTeamsPreviewLeavesShortContentUntouched validates that content within the
// preview length is returned verbatim, with no marker implying a truncation that
// did not happen.
func TestTeamsPreviewLeavesShortContentUntouched(t *testing.T) {
	if got := TeamsPreview("Shipping Friday."); got != "Shipping Friday." {
		t.Errorf("TeamsPreview() = %q, want the content unchanged", got)
	}
	if got := TeamsPreview(""); got != "" {
		t.Errorf("TeamsPreview(\"\") = %q, want the empty string", got)
	}
}

// TestSerializeOnlineMeetingCarriesTranscriptChainFields validates that the
// meeting projections carry the meeting-scoped identifier the transcript verbs
// are keyed by and the transcription setting that explains an empty transcript
// list without a further request.
func TestSerializeOnlineMeetingCarriesTranscriptChainFields(t *testing.T) {
	meeting := models.NewOnlineMeeting()
	id, subject, joinURL := "meeting-1", "Weekly sync", "https://teams.microsoft.com/l/meetup-join/x"
	meeting.SetId(&id)
	meeting.SetSubject(&subject)
	meeting.SetJoinWebUrl(&joinURL)
	meeting.SetStartDateTime(&teamsTestTime)
	allow := true
	meeting.SetAllowTranscription(&allow)

	organizerName := "Alex Stone"
	user := models.NewIdentity()
	user.SetDisplayName(&organizerName)
	identity := models.NewIdentitySet()
	identity.SetUser(user)
	organizer := models.NewMeetingParticipantInfo()
	organizer.SetIdentity(identity)
	participants := models.NewMeetingParticipants()
	participants.SetOrganizer(organizer)
	meeting.SetParticipants(participants)

	summary := SerializeSummaryOnlineMeeting(meeting)
	if got := summary["id"]; got != "meeting-1" {
		t.Errorf("summary id = %v, want %q", got, "meeting-1")
	}
	if got := summary["joinWebUrl"]; got != joinURL {
		t.Errorf("summary joinWebUrl = %v, want %q", got, joinURL)
	}

	raw := SerializeOnlineMeeting(meeting)
	if got := raw["allowTranscription"]; got != true {
		t.Errorf("allowTranscription = %v, want true", got)
	}
	if got := raw["organizer"]; got != "Alex Stone" {
		t.Errorf("organizer = %v, want %q", got, "Alex Stone")
	}
}

// TestSerializeSummaryTranscriptCarriesNoContent validates that listing
// transcripts costs metadata only. A meeting can hold several, and inlining even
// a preview of each would cost more than the fetch the listing exists to target.
func TestSerializeSummaryTranscriptCarriesNoContent(t *testing.T) {
	transcript := models.NewCallTranscript()
	id, meetingID := "transcript-1", "meeting-1"
	transcript.SetId(&id)
	transcript.SetMeetingId(&meetingID)
	transcript.SetCreatedDateTime(&teamsTestTime)

	result := SerializeSummaryTranscript(transcript)

	if got := result["meetingId"]; got != "meeting-1" {
		t.Errorf("meetingId = %v, want %q", got, "meeting-1")
	}
	for _, key := range []string{"content", "contentPreview"} {
		if _, present := result[key]; present {
			t.Errorf("a transcript summary must carry no %q", key)
		}
	}
}

// TestSerializeTranscriptEscalatesOverTextSize validates the transcript's
// escalation: the same fetched content is truncated by default and returned
// whole under the raw tier, with the truncation flag stating which happened.
func TestSerializeTranscriptEscalatesOverTextSize(t *testing.T) {
	transcript := models.NewCallTranscript()
	id := "transcript-1"
	transcript.SetId(&id)
	content := []byte("WEBVTT\n\n" + strings.Repeat("b", teamsPreviewRunes+50))

	preview := SerializeTranscript(transcript, content, false)
	if _, present := preview["content"]; present {
		t.Error("the default tier must not carry the full transcript content")
	}
	if preview["contentTruncated"] != true {
		t.Errorf("contentTruncated = %v, want true", preview["contentTruncated"])
	}

	full := SerializeTranscript(transcript, content, true)
	if got := full["content"]; got != string(content) {
		t.Error("the raw tier must carry the whole transcript content")
	}
	if full["contentTruncated"] != false {
		t.Errorf("contentTruncated = %v, want false under the raw tier", full["contentTruncated"])
	}
}

// TestSerializeTranscriptDoesNotFlagShortContent validates that a transcript
// shorter than the preview length is reported as complete, so a caller is not
// told to escalate for text it already has in full.
func TestSerializeTranscriptDoesNotFlagShortContent(t *testing.T) {
	transcript := models.NewCallTranscript()
	id := "transcript-short"
	transcript.SetId(&id)

	result := SerializeTranscript(transcript, []byte("WEBVTT\n\nHello."), false)

	if result["contentTruncated"] != false {
		t.Errorf("contentTruncated = %v, want false for a short transcript", result["contentTruncated"])
	}
	if got := result["contentPreview"]; got != "WEBVTT\n\nHello." {
		t.Errorf("contentPreview = %v, want the content unchanged", got)
	}
}

// TestTeamsSerializersTolerateNil validates that every serializer returns an
// empty map rather than panicking on a nil resource, which is what a Graph
// collection containing a nil element would otherwise cause mid-listing.
func TestTeamsSerializersTolerateNil(t *testing.T) {
	cases := map[string]map[string]any{
		"SerializeSummaryChat":           SerializeSummaryChat(nil),
		"SerializeChat":                  SerializeChat(nil),
		"SerializeSummaryChatMessage":    SerializeSummaryChatMessage(nil),
		"SerializeChatMessage":           SerializeChatMessage(nil),
		"SerializeSummaryChannelMessage": SerializeSummaryChannelMessage(nil),
		"SerializeChannelMessage":        SerializeChannelMessage(nil),
		"SerializeSummaryMessageReply":   SerializeSummaryMessageReply(nil),
		"SerializeMessageReply":          SerializeMessageReply(nil),
		"SerializeSummaryOnlineMeeting":  SerializeSummaryOnlineMeeting(nil),
		"SerializeOnlineMeeting":         SerializeOnlineMeeting(nil),
		"SerializeSummaryTranscript":     SerializeSummaryTranscript(nil),
		"SerializeTranscript":            SerializeTranscript(nil, []byte("x"), true),
	}

	for name, result := range cases {
		if len(result) != 0 {
			t.Errorf("%s(nil) = %v, want an empty map", name, result)
		}
	}
}

// TestSerializeMessageReplyCarriesEveryLocatingIdentifier validates that the raw
// reply projection carries the parent identifier plus whichever of the chat or
// channel coordinates the reply came from, so a caller needs no further
// resolution to address it.
func TestSerializeMessageReplyCarriesEveryLocatingIdentifier(t *testing.T) {
	msg := newTestChannelMessage()
	parent := "cmsg-1"
	msg.SetReplyToId(&parent)

	result := SerializeMessageReply(msg)

	for key, want := range map[string]string{
		"replyToId": "cmsg-1",
		"teamId":    "team-1",
		"channelId": "19:channel-1",
		"chatId":    "",
	} {
		if got := result[key]; got != want {
			t.Errorf("%s = %v, want %q", key, got, want)
		}
	}
}
