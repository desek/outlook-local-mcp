// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// This file provides the serializers for the Teams domain. Six resource shapes
// reach a caller, a chat, a chat message, a channel message, a reply, an online
// meeting, and a transcript, and each gets its own summary and raw projection
// with a field set chosen here rather than derived by dropping empty values from
// the raw payload, so a tier's shape does not change with how completely Graph
// populated the record.
//
// Three of those shapes are the same SDK type. A chat message, a channel
// message, and a reply are all models.ChatMessageable, but they answer different
// questions and carry different identifiers: a chat message is located by its
// chatId, a channel message by the teamId and channelId in its channelIdentity,
// and a reply by the replyToId naming its parent. Collapsing them onto one
// serializer would put a nil chatId in every channel result and an absent
// channelIdentity in every chat result, so each carries the identifiers a caller
// needs to make the next call and no others.
//
// Body content is the other axis. A Teams message body is arbitrarily long and a
// transcript longer still, so the summary tier carries a truncated preview and
// the raw tier carries the whole thing, which is what the body-escalation rule
// asks the verb descriptions to disclose.
//
// @agents-index: Summary and raw serializers for Graph chats, Teams messages,
// online meetings, and call transcripts, projecting each onto the Teams domain's
// output tiers.
package tools

import (
	"time"

	"github.com/desek/outlook-local-mcp/internal/graph"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
)

// teamsPreviewRunes bounds the preview a summary-tier body or a default-tier
// transcript carries. It is counted in runes, not bytes, so a multi-byte
// character is never split in half. The value is large enough for the model to
// judge whether the full text is worth a raw fetch and small enough that a
// listing of many messages stays affordable.
const teamsPreviewRunes = 500

// teamsTruncationMarker is appended to a preview that was cut short, so a caller
// can tell a genuinely short body from a long one it has only seen the head of.
const teamsTruncationMarker = "... (truncated; request output=raw for the full text)"

// TeamsPreview truncates arbitrary Teams content to the preview length, on a
// rune boundary, appending the truncation marker only when it actually cut. A
// Teams message and a call transcript both arrive from Graph as whole content
// with no service-supplied preview field, unlike a mail bodyPreview, so the
// preview has to be produced here.
//
// Parameters:
//   - content: the full text, which may be empty.
//
// Returns the content unchanged when it is within the preview length, and the
// leading teamsPreviewRunes runes followed by the truncation marker otherwise.
//
// Side effects: none.
func TeamsPreview(content string) string {
	runes := []rune(content)
	if len(runes) <= teamsPreviewRunes {
		return content
	}
	return string(runes[:teamsPreviewRunes]) + teamsTruncationMarker
}

// SerializeSummaryChat projects a chat onto the field set a caller choosing
// which conversation to read actually needs: what the chat is called, what kind
// it is, when it last moved, and the identifier every chat-message verb is keyed
// by. A one-to-one chat commonly carries no topic, so the preview of its last
// message is included: it is often the only thing distinguishing one untitled
// chat from another in a listing.
//
// Side effects: none.
func SerializeSummaryChat(chat models.Chatable) map[string]any {
	if chat == nil {
		return map[string]any{}
	}

	return map[string]any{
		"id":                  graph.SafeStr(chat.GetId()),
		"topic":               graph.SafeStr(chat.GetTopic()),
		"chatType":            teamsEnumStr(chat.GetChatType()),
		"lastUpdatedDateTime": teamsTimeStr(chat.GetLastUpdatedDateTime()),
		"lastMessagePreview":  chatLastMessagePreview(chat),
		"webUrl":              graph.SafeStr(chat.GetWebUrl()),
	}
}

// SerializeChat projects a chat onto the raw tier, adding the membership and
// creation metadata a caller that escalated is asking for. Members are rendered
// by display name and role because who is in a chat is what identifies it once
// the topic is empty.
//
// Side effects: none.
func SerializeChat(chat models.Chatable) map[string]any {
	if chat == nil {
		return map[string]any{}
	}

	members := make([]map[string]any, 0, len(chat.GetMembers()))
	for _, member := range chat.GetMembers() {
		if member == nil {
			continue
		}
		members = append(members, map[string]any{
			"id":          graph.SafeStr(member.GetId()),
			"displayName": graph.SafeStr(member.GetDisplayName()),
			"roles":       teamsStrings(member.GetRoles()),
		})
	}

	return map[string]any{
		"id":                      graph.SafeStr(chat.GetId()),
		"topic":                   graph.SafeStr(chat.GetTopic()),
		"chatType":                teamsEnumStr(chat.GetChatType()),
		"createdDateTime":         teamsTimeStr(chat.GetCreatedDateTime()),
		"lastUpdatedDateTime":     teamsTimeStr(chat.GetLastUpdatedDateTime()),
		"lastMessagePreview":      chatLastMessagePreview(chat),
		"members":                 members,
		"isHiddenForAllMembers":   graph.SafeBool(chat.GetIsHiddenForAllMembers()),
		"tenantId":                graph.SafeStr(chat.GetTenantId()),
		"webUrl":                  graph.SafeStr(chat.GetWebUrl()),
		"onlineMeetingJoinWebUrl": chatOnlineMeetingJoinURL(chat),
	}
}

// SerializeSummaryChatMessage projects a message in a chat onto the field set a
// caller scanning a conversation needs: who said it, when, the head of what they
// said, and the two identifiers (chat and message) that address the full read and
// the replies.
//
// Side effects: none.
func SerializeSummaryChatMessage(msg models.ChatMessageable) map[string]any {
	if msg == nil {
		return map[string]any{}
	}

	return map[string]any{
		"id":              graph.SafeStr(msg.GetId()),
		"chatId":          graph.SafeStr(msg.GetChatId()),
		"from":            teamsSenderName(msg),
		"createdDateTime": teamsTimeStr(msg.GetCreatedDateTime()),
		"bodyPreview":     TeamsPreview(teamsBodyContent(msg.GetBody())),
		"attachmentCount": len(msg.GetAttachments()),
		"messageType":     teamsEnumStr(msg.GetMessageType()),
		"webUrl":          graph.SafeStr(msg.GetWebUrl()),
	}
}

// SerializeChatMessage projects a message in a chat onto the raw tier, carrying
// the full body rather than its preview, which is the escalation the verb's
// description promises, plus the edit and deletion timestamps that tell a reader
// whether what they are looking at is what was originally said.
//
// Side effects: none.
func SerializeChatMessage(msg models.ChatMessageable) map[string]any {
	if msg == nil {
		return map[string]any{}
	}

	result := teamsMessageRawCommon(msg)
	result["chatId"] = graph.SafeStr(msg.GetChatId())
	return result
}

// SerializeSummaryChannelMessage projects a top-level channel message onto the
// scanning field set, keyed by the team and channel identifiers its
// channelIdentity carries rather than by a chatId, which a channel message does
// not have. The subject is included because a channel post, unlike a chat
// message, commonly has one and it is the thread's title.
//
// Side effects: none.
func SerializeSummaryChannelMessage(msg models.ChatMessageable) map[string]any {
	if msg == nil {
		return map[string]any{}
	}

	teamID, channelID := teamsChannelIdentity(msg)
	return map[string]any{
		"id":              graph.SafeStr(msg.GetId()),
		"teamId":          teamID,
		"channelId":       channelID,
		"subject":         graph.SafeStr(msg.GetSubject()),
		"from":            teamsSenderName(msg),
		"createdDateTime": teamsTimeStr(msg.GetCreatedDateTime()),
		"bodyPreview":     TeamsPreview(teamsBodyContent(msg.GetBody())),
		"attachmentCount": len(msg.GetAttachments()),
		"webUrl":          graph.SafeStr(msg.GetWebUrl()),
	}
}

// SerializeChannelMessage projects a top-level channel message onto the raw
// tier: the full body plus the team and channel identifiers, so a caller that
// escalated can both read the post and address its replies without a further
// resolution step.
//
// Side effects: none.
func SerializeChannelMessage(msg models.ChatMessageable) map[string]any {
	if msg == nil {
		return map[string]any{}
	}

	teamID, channelID := teamsChannelIdentity(msg)
	result := teamsMessageRawCommon(msg)
	result["teamId"] = teamID
	result["channelId"] = channelID
	return result
}

// SerializeSummaryMessageReply projects a reply onto the scanning field set.
// A reply's distinguishing field is replyToId, which names the parent it hangs
// under, so it is carried in the summary rather than left to the raw tier: a
// caller reading a merged list of replies needs to know which thread each is in.
//
// Side effects: none.
func SerializeSummaryMessageReply(msg models.ChatMessageable) map[string]any {
	if msg == nil {
		return map[string]any{}
	}

	teamID, channelID := teamsChannelIdentity(msg)
	return map[string]any{
		"id":              graph.SafeStr(msg.GetId()),
		"replyToId":       graph.SafeStr(msg.GetReplyToId()),
		"chatId":          graph.SafeStr(msg.GetChatId()),
		"teamId":          teamID,
		"channelId":       channelID,
		"from":            teamsSenderName(msg),
		"createdDateTime": teamsTimeStr(msg.GetCreatedDateTime()),
		"bodyPreview":     TeamsPreview(teamsBodyContent(msg.GetBody())),
		"attachmentCount": len(msg.GetAttachments()),
	}
}

// SerializeMessageReply projects a reply onto the raw tier: the full body plus
// every identifier that locates it, the parent's and whichever of the chat or
// channel coordinates the reply came from.
//
// Side effects: none.
func SerializeMessageReply(msg models.ChatMessageable) map[string]any {
	if msg == nil {
		return map[string]any{}
	}

	teamID, channelID := teamsChannelIdentity(msg)
	result := teamsMessageRawCommon(msg)
	result["chatId"] = graph.SafeStr(msg.GetChatId())
	result["teamId"] = teamID
	result["channelId"] = channelID
	return result
}

// SerializeSummaryOnlineMeeting projects an online meeting onto the field set
// the transcript chain needs: the meeting-scoped identifier that list_transcripts
// and get_transcript are keyed by, the subject and times that confirm the right
// meeting was resolved, and the join URL that a calendar event supplies as the
// resolution input.
//
// Side effects: none.
func SerializeSummaryOnlineMeeting(meeting models.OnlineMeetingable) map[string]any {
	if meeting == nil {
		return map[string]any{}
	}

	return map[string]any{
		"id":            graph.SafeStr(meeting.GetId()),
		"subject":       graph.SafeStr(meeting.GetSubject()),
		"startDateTime": teamsTimeStr(meeting.GetStartDateTime()),
		"endDateTime":   teamsTimeStr(meeting.GetEndDateTime()),
		"joinWebUrl":    graph.SafeStr(meeting.GetJoinWebUrl()),
	}
}

// SerializeOnlineMeeting projects an online meeting onto the raw tier. The
// transcription setting is carried because it answers, without a further
// request, why a meeting a caller expected a recap for lists no transcripts.
//
// Side effects: none.
func SerializeOnlineMeeting(meeting models.OnlineMeetingable) map[string]any {
	if meeting == nil {
		return map[string]any{}
	}

	return map[string]any{
		"id":                    graph.SafeStr(meeting.GetId()),
		"subject":               graph.SafeStr(meeting.GetSubject()),
		"startDateTime":         teamsTimeStr(meeting.GetStartDateTime()),
		"endDateTime":           teamsTimeStr(meeting.GetEndDateTime()),
		"creationDateTime":      teamsTimeStr(meeting.GetCreationDateTime()),
		"joinWebUrl":            graph.SafeStr(meeting.GetJoinWebUrl()),
		"videoTeleconferenceId": graph.SafeStr(meeting.GetVideoTeleconferenceId()),
		"externalId":            graph.SafeStr(meeting.GetExternalId()),
		"isBroadcast":           graph.SafeBool(meeting.GetIsBroadcast()),
		"allowTranscription":    graph.SafeBool(meeting.GetAllowTranscription()),
		"allowRecording":        graph.SafeBool(meeting.GetAllowRecording()),
		"organizer":             meetingOrganizerName(meeting),
		"meetingSpokenLanguage": graph.SafeStr(meeting.GetMeetingSpokenLanguageTag()),
	}
}

// SerializeSummaryTranscript projects a transcript's metadata onto the field set
// a caller choosing which transcript to fetch needs. It carries no content: a
// meeting can hold several transcripts and a listing that inlined even a preview
// of each would cost more than the fetch it is meant to help avoid.
//
// Side effects: none.
func SerializeSummaryTranscript(transcript models.CallTranscriptable) map[string]any {
	if transcript == nil {
		return map[string]any{}
	}

	return map[string]any{
		"id":              graph.SafeStr(transcript.GetId()),
		"meetingId":       graph.SafeStr(transcript.GetMeetingId()),
		"callId":          graph.SafeStr(transcript.GetCallId()),
		"createdDateTime": teamsTimeStr(transcript.GetCreatedDateTime()),
		"endDateTime":     teamsTimeStr(transcript.GetEndDateTime()),
	}
}

// SerializeTranscript projects one transcript's metadata together with its
// WEBVTT content. A callTranscript carries no Graph-supplied preview field, so
// the content is always fetched and this function decides how much of it the
// caller sees: the whole thing under the raw tier, and a truncated preview
// otherwise. The escalation the verb offers is therefore over returned text
// size, not over the number of Graph requests.
//
// Parameters:
//   - transcript: the transcript metadata; nil yields an empty map.
//   - content: the fetched WEBVTT bytes, which may be empty when the meeting was
//     not transcribed.
//   - full: true for the raw tier, which returns the whole content under
//     "content"; false returns a preview under "contentPreview" alongside a
//     "contentTruncated" flag stating whether anything was withheld.
//
// Side effects: none.
func SerializeTranscript(transcript models.CallTranscriptable, content []byte, full bool) map[string]any {
	if transcript == nil {
		return map[string]any{}
	}

	result := map[string]any{
		"id":                   graph.SafeStr(transcript.GetId()),
		"meetingId":            graph.SafeStr(transcript.GetMeetingId()),
		"callId":               graph.SafeStr(transcript.GetCallId()),
		"createdDateTime":      teamsTimeStr(transcript.GetCreatedDateTime()),
		"endDateTime":          teamsTimeStr(transcript.GetEndDateTime()),
		"transcriptContentUrl": graph.SafeStr(transcript.GetTranscriptContentUrl()),
		"contentCorrelationId": graph.SafeStr(transcript.GetContentCorrelationId()),
	}

	text := string(content)
	if full {
		result["content"] = text
		result["contentTruncated"] = false
		return result
	}

	preview := TeamsPreview(text)
	result["contentPreview"] = preview
	result["contentTruncated"] = preview != text
	return result
}

// The label a search hit carries stating which collection its message came
// from. A hit is the entry point to the domain, and the two collections are read
// by different verbs taking different identifiers, so the record says which
// follow-up applies rather than leaving a caller to infer it from which
// identifier fields happen to be populated.
const (
	teamsHitSourceChat    = "chat"
	teamsHitSourceChannel = "channel"
)

// SerializeTeamsSearchHit projects one ranked search hit onto the output tier.
// The message inside the hit is projected by whichever of the chat and channel
// serializers matches it, so a chat hit carries the chatId its reads are keyed
// by and a channel hit carries the teamId and channelId theirs are, and the hit
// itself contributes the rank, the service's highlighted snippet, and the source
// label naming which of the two applies.
//
// Parameters:
//   - hit: one ranked hit; nil, or one whose resource is not a chat message,
//     yields nil.
//   - full: true for the raw tier, which carries the whole message body; false
//     for the summary tier, which carries its preview.
//
// Returns nil for a hit this domain cannot address, so a caller appends only the
// hits a follow-up verb can act on. Only chatMessage results are requested, so a
// nil is an unexpected service response rather than an ordinary case.
//
// Side effects: none.
func SerializeTeamsSearchHit(hit models.SearchHitable, full bool) map[string]any {
	if hit == nil {
		return nil
	}
	msg, ok := hit.GetResource().(models.ChatMessageable)
	if !ok || msg == nil {
		return nil
	}

	source := teamsHitSourceChat
	if teamID, _ := teamsChannelIdentity(msg); teamID != "" {
		source = teamsHitSourceChannel
	}

	record := teamsSearchHitMessage(msg, source, full)
	record["source"] = source
	record["rank"] = graph.SafeInt32(hit.GetRank())
	record["hitSummary"] = graph.SafeStr(hit.GetSummary())
	return record
}

// teamsSearchHitMessage projects the message inside a hit through the serializer
// for the collection it came from, at the requested tier.
func teamsSearchHitMessage(msg models.ChatMessageable, source string, full bool) map[string]any {
	if source == teamsHitSourceChannel {
		if full {
			return SerializeChannelMessage(msg)
		}
		return SerializeSummaryChannelMessage(msg)
	}
	if full {
		return SerializeChatMessage(msg)
	}
	return SerializeSummaryChatMessage(msg)
}

// teamsMessageRawCommon builds the raw-tier fields every Teams message shares,
// whichever collection it came from. The caller adds the identifiers that locate
// it, which is the only thing separating the three message shapes at this tier.
func teamsMessageRawCommon(msg models.ChatMessageable) map[string]any {
	attachments := make([]map[string]any, 0, len(msg.GetAttachments()))
	for _, att := range msg.GetAttachments() {
		if att == nil {
			continue
		}
		attachments = append(attachments, map[string]any{
			"id":          graph.SafeStr(att.GetId()),
			"name":        graph.SafeStr(att.GetName()),
			"contentType": graph.SafeStr(att.GetContentType()),
			"contentUrl":  graph.SafeStr(att.GetContentUrl()),
		})
	}

	mentions := make([]string, 0, len(msg.GetMentions()))
	for _, mention := range msg.GetMentions() {
		if mention == nil {
			continue
		}
		if text := graph.SafeStr(mention.GetMentionText()); text != "" {
			mentions = append(mentions, text)
		}
	}

	return map[string]any{
		"id":                   graph.SafeStr(msg.GetId()),
		"subject":              graph.SafeStr(msg.GetSubject()),
		"from":                 teamsSenderName(msg),
		"body":                 teamsBodyContent(msg.GetBody()),
		"bodyContentType":      teamsBodyContentType(msg.GetBody()),
		"createdDateTime":      teamsTimeStr(msg.GetCreatedDateTime()),
		"lastModifiedDateTime": teamsTimeStr(msg.GetLastModifiedDateTime()),
		"lastEditedDateTime":   teamsTimeStr(msg.GetLastEditedDateTime()),
		"deletedDateTime":      teamsTimeStr(msg.GetDeletedDateTime()),
		"importance":           teamsEnumStr(msg.GetImportance()),
		"messageType":          teamsEnumStr(msg.GetMessageType()),
		"replyToId":            graph.SafeStr(msg.GetReplyToId()),
		"locale":               graph.SafeStr(msg.GetLocale()),
		"attachments":          attachments,
		"mentions":             mentions,
		"webUrl":               graph.SafeStr(msg.GetWebUrl()),
	}
}

// teamsChannelIdentity reduces a message's channelIdentity to the team and
// channel identifiers, returning empty strings for a message that carries none.
// Both are returned together because neither addresses a channel read on its own.
func teamsChannelIdentity(msg models.ChatMessageable) (string, string) {
	identity := msg.GetChannelIdentity()
	if identity == nil {
		return "", ""
	}
	return graph.SafeStr(identity.GetTeamId()), graph.SafeStr(identity.GetChannelId())
}

// teamsSenderName returns the display name of whoever sent a message. A Teams
// message can be sent by a user, an application, or a device, and only the user
// case has a name a reader recognises, so an application-sent message renders as
// a stated absence rather than as a blank.
func teamsSenderName(msg models.ChatMessageable) string {
	from := msg.GetFrom()
	if from == nil {
		return ""
	}
	if user := from.GetUser(); user != nil {
		return graph.SafeStr(user.GetDisplayName())
	}
	if app := from.GetApplication(); app != nil {
		return graph.SafeStr(app.GetDisplayName())
	}
	return ""
}

// teamsBodyContent returns a message body's text, or the empty string when the
// message carries no body, so a nil body never reaches a caller as a nil map
// entry it has to guard against.
func teamsBodyContent(body models.ItemBodyable) string {
	if body == nil {
		return ""
	}
	return graph.SafeStr(body.GetContent())
}

// teamsBodyContentType returns a body's content type as its string name. It
// matters to a reader because a Teams body is commonly HTML, and text extracted
// from it reads differently from a plain-text body.
func teamsBodyContentType(body models.ItemBodyable) string {
	if body == nil {
		return ""
	}
	return teamsEnumStr(body.GetContentType())
}

// chatLastMessagePreview returns the head of a chat's last message, which is
// what distinguishes one untitled one-to-one chat from another in a listing.
func chatLastMessagePreview(chat models.Chatable) string {
	preview := chat.GetLastMessagePreview()
	if preview == nil {
		return ""
	}
	return TeamsPreview(teamsBodyContent(preview.GetBody()))
}

// chatOnlineMeetingJoinURL returns the join URL of the meeting a chat belongs
// to, when it belongs to one. It is the bridge from a meeting chat to the
// transcript chain, which is keyed by the online meeting that URL resolves to.
func chatOnlineMeetingJoinURL(chat models.Chatable) string {
	info := chat.GetOnlineMeetingInfo()
	if info == nil {
		return ""
	}
	return graph.SafeStr(info.GetJoinWebUrl())
}

// meetingOrganizerName returns the organizer's display name, or an empty string
// when the meeting carries no participant information, so the raw tier's shape
// does not depend on how completely Graph populated the meeting.
func meetingOrganizerName(meeting models.OnlineMeetingable) string {
	participants := meeting.GetParticipants()
	if participants == nil {
		return ""
	}
	organizer := participants.GetOrganizer()
	if organizer == nil {
		return ""
	}
	identity := organizer.GetIdentity()
	if identity == nil {
		return ""
	}
	if user := identity.GetUser(); user != nil {
		return graph.SafeStr(user.GetDisplayName())
	}
	return ""
}

// teamsEnumStr renders any Graph enum pointer as its string name, or the empty
// string when it is nil. Every Graph enum implements String(), so one constraint
// covers all of them and each call site avoids its own nil check.
func teamsEnumStr[T interface{ String() string }](value *T) string {
	if value == nil {
		return ""
	}
	return (*value).String()
}

// teamsTimeStr renders a Graph timestamp in RFC 3339, or the empty string when
// the record carries none.
func teamsTimeStr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// teamsStrings normalises a possibly-nil Graph string slice to an empty slice,
// so a JSON consumer sees [] rather than null for a record with none.
func teamsStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
