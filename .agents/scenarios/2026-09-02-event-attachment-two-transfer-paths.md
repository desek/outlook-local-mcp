---
date: 2026-09-02
source: cr (CR-0081)
surface: cli (MCP stdio client driving the built ./outlook-local-mcp binary)
outcome: not-run
runs: "0 of 0 attempted; derived from the CR, not yet run against a live mailbox"
coverage_gap: false
---

# Reading and adding attachments on a calendar event

## Status

**Derived, not yet run.** This artifact is the executable form of the acceptance
requirement, written so the run can be performed without re-deriving it. No run has
happened: the scenario needs a live authenticated mailbox and an interactively granted
keychain entry, and neither was available to the session that wrote it. The `outcome` and
`runs` frontmatter fields say so, and they MUST be rewritten by whoever performs the first
run rather than left as they are.

Nothing below has been observed. Every unit test covering these three verbs drives an
`httptest` server, and the chunked path serves its own upload URL in process, so **the
chunked transfer has never moved a byte against Microsoft Graph on the event navigation.**
Four things are unconfirmed against the live service and are what this scenario exists to
settle: that `POST /me/events/{id}/attachments/createUploadSession` accepts the same
request body shape the message navigation does, the `Content-Range` contract the service
accepts on the event path, the shape of the `Location` header on the completing `201`, and
whether an attachment added through the session path is visible to a subsequent
`list_event_attachments` rather than only to the write's own confirmation.

## Goal

A person has an event on their calendar and wants to see which files are attached to it,
download one of them, and attach two files of their own: a small one that fits in a single
request, and one large enough that the service requires a chunked upload session. They then
want to confirm both files are actually on the event.

The surface is the MCP server as an assistant drives it, over stdio, against the built
binary, with the `calendar` aggregate tool and its `operation` verb dispatch. It is not the
handler constructors in `internal/tools/`.

## Preconditions

* An authenticated account is present in the token cache and resolves as the default
  account. The write verb mutates a real calendar; there is no dry-run path.
* All three verbs register unconditionally, so no feature flag is needed. This is itself
  worth confirming at step 1 against a server started with both mail flags off.
* `OUTLOOK_MCP_READ_ONLY` is unset or false. Under read-only, `add_event_attachment` is
  refused by design, which is a separate assertion and not this scenario.
* `OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES` is above the large file's size for the duration of
  the run. The default is `10485760`, so a large file below ten megabytes needs no change; a
  larger one is refused before any upload and the run grades nothing.
* An event exists on the calendar that already carries at least one attachment, so steps 2
  and 3 read something the run did not itself write. A real received invitation with an
  agenda document is the intended case. Record its identifier as `EVENT`.
* Two local files exist and their exact byte sizes are recorded before the run: one strictly
  below `3000000` decoded bytes, and one at or above it. The boundary is the routing
  constant, not a round binary figure.
* The binary under test is rebuilt at the path `.mcp.json` names before the run, and its
  reported commit matches `git rev-parse --short HEAD`. A run against a stale binary
  produces a report that looks authoritative and describes a different commit:

  ```bash
  go build -ldflags="-X main.commit=$(git rev-parse --short HEAD) -X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o ./outlook-local-mcp ./cmd/outlook-local-mcp
  ```

* On macOS, the binary is signed with a stable identity and the keychain prompt is answered
  interactively once before any headless run. An ad-hoc-signed binary blocks on an invisible
  prompt at the first authenticated call, which reads as a timeout.

## Steps at the user surface

Driven as MCP `tools/call` requests against the built server over stdio. Record the value
each step reports; several later steps consume them.

1. `tools/list`. Confirm four top-level tools and that the `calendar` operation enum
   contains all three attachment verbs, on a server started with both mail flags off.
2. `calendar` with `operation="list_event_attachments"`, `event_id=EVENT`. Record the
   identifier of one pre-existing attachment as `EXISTING` and its reported size.
3. `calendar` with `operation="get_event_attachment"`, `event_id=EVENT`,
   `attachment_id=EXISTING`.
4. `calendar` with `operation="add_event_attachment"`, `event_id=EVENT`, `name` the small
   file's name, `mime_type` its type, `content_bytes` the small file base64-encoded. Record
   the reported attachment identifier as `SMALL`.
5. `calendar` with `operation="add_event_attachment"`, `event_id=EVENT`, `name` the large
   file's name, `mime_type` its type, `content_bytes` the large file base64-encoded. Record
   the reported attachment identifier as `LARGE`. This is the step that has never been
   exercised against the live service.
6. `calendar` with `operation="list_event_attachments"`, `event_id=EVENT`. This is the
   read-back that grades steps 4 and 5 against the calendar rather than against the
   confirmations.
7. `calendar` with `operation="get_event_attachment"`, `event_id=EVENT`,
   `attachment_id=LARGE`.
8. `calendar` with `operation="add_event_attachment"`, `event_id=EVENT`, a valid `name`, and
   `content_bytes` set to a string that is not valid base64.
9. `calendar` with `operation="add_event_attachment"`, `event_id="AAAAAAAAAAAAAAAAAAAAAA=="`
   and otherwise valid arguments. This grades the subject fetch as the gate it is specified
   to be: a nonexistent event must refuse the call before any byte moves.

## Success condition

Graded on the calendar state read back in steps 6 and 7, and on the refusals in steps 8 and
9, never on the wording of the write confirmations. A confirmation is what the scenario is
testing, so it cannot also be the thing that grades it.

* Step 1 reports exactly four tools, and the calendar enum contains
  `list_event_attachments`, `get_event_attachment`, and `add_event_attachment`.
* Step 2 enumerates the event's attachments with names, sizes, and content types, and
  returns no content bytes at any tier below `raw`.
* Step 3 returns `EXISTING` with base64 content whose decoded length equals the size step 2
  reported.
* Step 4's confirmation names the event subject, `EVENT`, the file name, the byte size, an
  attachment identifier, and the direct transfer path. The subject is the one the calendar
  actually holds, which is what proves the subject fetch ran rather than echoing an input.
* Step 5's confirmation names the same six things and the upload-session transfer path. Its
  byte size equals the large file's recorded size, which is what proves every chunk arrived
  rather than a prefix of them.
* Step 6 lists the pre-existing attachments plus exactly two more, whose identifiers are
  `SMALL` and `LARGE` and whose sizes equal the two recorded file sizes. An identifier
  reported in step 4 or 5 that does not appear here falsifies the confirmation, and is the
  failure the chunked path is most likely to produce.
* Step 7 resolves `LARGE` and returns content whose decoded length equals the large file's
  recorded size. An upload session the service discarded returns a not-found here while step
  5 reported success, which is the partial-success failure the verb is written to make
  impossible.
* Step 8 fails naming `content_bytes` and stating the correction, and adds nothing: a step 6
  repeated after it lists the same set.
* Step 9 fails naming the event, and adds nothing.

## Restoration

The calendar is a real one and this scenario mutates it. After grading, remove the two
attachments the run added. There is no delete verb for event attachments, which is recorded
out of scope by the change request, so removal is manual: delete `SMALL` and `LARGE` from
the event in Outlook, then re-run step 2 and confirm only the pre-existing attachments
remain. Restore `OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES` to its prior value if the run raised
it. Steps 8 and 9 mutate nothing when they behave as specified; if either succeeded, remove
what it added and record that as the scenario's finding.

The absence of a delete verb is itself worth recording against the run: it is what makes
this scenario's restoration a manual step rather than a driven one.

## Run log

No runs. The first run appends its record here: the date, the driver invocation, the value
observed for each success-condition bullet, and how many attempts of how many passed.
