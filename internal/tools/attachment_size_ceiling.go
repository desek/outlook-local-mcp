// Package tools provides MCP tool definitions and handler constructors for the
// Outlook Calendar MCP Server.
//
// @agents-index: The service-documented 150 MB ceiling on attachment uploads, applied by mail.add_attachment and calendar.add_event_attachment before any Graph call.
package tools

import "fmt"

// graphAttachmentCeilingBytes is the largest file the Graph upload session
// accepts. The service documents the range as 3 MB to 150 MB, so a payload
// above it is refused locally whatever OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES
// says: the configured bound has no upper limit and an unlimited value would
// otherwise send a createUploadSession the service rejects. The figure is the
// binary megabyte the service uses for its size limits.
const graphAttachmentCeilingBytes = 150 * 1024 * 1024

// attachmentOverServiceCeiling returns the refusal text for a payload above
// the service ceiling, or "" when the size is within it.
//
// The text names the documented limit and the tenant caveat, because a tenant
// message-size limit (35 MB by default in Exchange Online) can refuse a
// smaller file, and the caller cannot see that limit from here.
func attachmentOverServiceCeiling(size int) string {
	if int64(size) <= graphAttachmentCeilingBytes {
		return ""
	}
	return fmt.Sprintf(
		"attachment size %d bytes exceeds the Microsoft Graph upload limit of %d bytes (150 MB); "+
			"split the file or share it by link instead of attaching it, and note the tenant message size limit (35 MB by default in Exchange Online) can refuse smaller files; "+
			"verify by retrying with a file at or below the limit",
		size, int64(graphAttachmentCeilingBytes))
}
