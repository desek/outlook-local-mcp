// Package tools — this file holds the projection of a reply collection, used
// by the channel reply listing.
//
// @agents-index: projects a Teams reply collection through the tier serializer.
package tools

import "github.com/microsoftgraph/msgraph-sdk-go/models"

// serializeMessageReplyCollection projects a reply collection through the
// tier's serializer, preserving the order Graph returned them in. The reply
// serializers are used rather than the chat-message ones because a reply's
// distinguishing field is the parent it hangs under, and this is the one shape
// that carries it in the summary tier.
//
// Parameters:
//   - resp: the collection response, which may be nil when Graph returned no body.
//   - outputMode: the resolved output tier.
//
// Returns one serialized map per reply, in the order received.
func serializeMessageReplyCollection(resp models.ChatMessageCollectionResponseable, outputMode string) []map[string]any {
	if resp == nil {
		return []map[string]any{}
	}
	replies := make([]map[string]any, 0, len(resp.GetValue()))
	for _, msg := range resp.GetValue() {
		if outputMode == "raw" {
			replies = append(replies, SerializeMessageReply(msg))
		} else {
			replies = append(replies, SerializeSummaryMessageReply(msg))
		}
	}
	return replies
}
