---
date: 2026-09-02
source: cr (CR-0079)
surface: cli (MCP stdio client driving the built ./outlook-local-mcp binary)
outcome: not-run
runs: "0 of 0 attempted; derived from the CR, not yet run against a live mailbox"
coverage_gap: false
---

# Attaching a small and a large file to a draft

## Status

**Derived, not yet run.** This artifact is the executable form of the acceptance
requirement, written so the run can be performed without re-deriving it. No run has
happened: the scenario needs a live authenticated mailbox and a paid harness run, and
neither was available to the session that wrote it. The `outcome` and `runs` frontmatter
fields say so, and they MUST be rewritten by whoever performs the first run rather than
left as they are.

Nothing below has been observed. Every unit test covering this verb drives an `httptest`
server, and the chunked path serves its own upload URL in process, so **the chunked
transfer has never been exercised against Microsoft Graph.** Three specific things are
unconfirmed against the live service and are what this scenario exists to settle: the
`Content-Range` contract the service actually accepts, the shape of the `Location` header
on the completing `201`, and whether the service's own inline threshold agrees with the
decimal three million the routing constant uses.

## Goal

A person has drafted a message and wants to attach two files to it: a small one that fits
in a single request, and one large enough that the service refuses a single request and
requires a chunked upload session. They then want to confirm both files are actually on
the draft.

The surface is the MCP server as an assistant drives it, over stdio, against the built
binary, with the `mail` aggregate tool and its `operation` verb dispatch. It is not the
handler constructors in `internal/tools/`.

## Preconditions

* An authenticated account is present in the token cache and resolves as the default
  account. The verb writes to a real mailbox; there is no dry-run path.
* `OUTLOOK_MCP_MAIL_MANAGE_ENABLED=true`, which is what registers `add_attachment`. With
  `OUTLOOK_MCP_MAIL_ENABLED` alone the verb is absent from the operation enum.
* `OUTLOOK_MCP_READ_ONLY` is unset or false. Under read-only every write step below is
  refused by design, which is a separate assertion and not this scenario.
* `OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES` is raised above the large file's size for the
  duration of the run. The default is `10485760`, so a large file below ten megabytes needs
  no change; a larger one is refused before any upload and the run grades nothing.
* Two local files exist and their exact byte sizes are recorded before the run: one
  strictly below `3000000` decoded bytes, and one at or above it. The boundary is the
  routing constant, not a round binary figure.
* The binary under test is rebuilt at the path `.mcp.json` names before the run, and its
  reported commit matches `git rev-parse --short HEAD`. A run against a stale binary
  produces a report that looks authoritative and describes a different commit:

  ```bash
  go build -ldflags="-X main.commit=$(git rev-parse --short HEAD) -X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o ./outlook-local-mcp ./cmd/outlook-local-mcp
  ```

* On macOS, the binary is signed with a stable identity and the keychain prompt is
  answered interactively once before any headless run. An ad-hoc-signed binary blocks on
  an invisible prompt at the first authenticated call, which reads as a timeout.

## Steps at the user surface

Driven as MCP `tools/call` requests against the built server over stdio. Record the value
each step reports; several later steps consume them.

1. `mail` with `operation="create_draft"`, any subject. Record the draft identifier as
   `DRAFT` and the subject as `SUBJECT`.
2. `mail` with `operation="add_attachment"`, `message_id=DRAFT`, `name` the small file's
   name, `mime_type` its type, `content_bytes` the small file base64-encoded. Record the
   reported attachment identifier as `SMALL`.
3. `mail` with `operation="add_attachment"`, `message_id=DRAFT`, `name` the large file's
   name, `mime_type` its type, `content_bytes` the large file base64-encoded. Record the
   reported attachment identifier as `LARGE`. This is the step that has never been
   exercised against the live service.
4. `mail` with `operation="list_attachments"`, `message_id=DRAFT`. This is the read-back
   that grades steps 2 and 3 against the mailbox rather than against the confirmations.
5. `mail` with `operation="get_attachment"`, `message_id=DRAFT`, `attachment_id=LARGE`.
6. `mail` with `operation="list_messages"`, scoped to the Inbox. Choose any received
   message and record its identifier as `RECEIVED`.
7. `mail` with `operation="add_attachment"`, `message_id=RECEIVED`, with the small file's
   arguments. This grades the draft guard against a message the service genuinely reports
   as not a draft, rather than against a fixture that says so.
8. `mail` with `operation="add_attachment"`, `message_id=DRAFT`, `name` a file name, and
   `content_bytes` set to a string that is not valid base64.

## Success condition

Graded on the mailbox state read back in steps 4 and 5, and on the refusals in steps 7 and
8, never on the wording of the write confirmations. A confirmation is what the scenario is
testing, so it cannot also be the thing that grades it.

* Step 2's confirmation names `SUBJECT`, `DRAFT`, the file name, the byte size, an
  attachment identifier, and the direct transfer path.
* Step 3's confirmation names the same six things and the upload-session transfer path.
  Its byte size equals the large file's recorded size, which is what proves every chunk
  arrived rather than a prefix of them.
* Step 4 lists exactly two attachments, whose identifiers are `SMALL` and `LARGE` and
  whose sizes equal the two recorded file sizes. An identifier reported in step 2 or 3
  that does not appear here falsifies the confirmation, and is the failure the chunked
  path is most likely to produce.
* Step 5 resolves `LARGE` and reports its content type and size. An upload session that
  the service discarded returns a not-found here while step 3 reported success, which is
  the partial-success failure the verb is written to make impossible.
* Step 7 fails, stating the message is not a draft, and the mailbox is unchanged.
* Step 8 fails naming `content_bytes`, and no attachment is added: a step 4 repeated after
  it still lists exactly two.

## Restoration

The mailbox is a real one and this scenario mutates it. After grading, delete the draft:
`mail` with `operation="delete_draft"`, `message_id=DRAFT`. This removes both attachments
with it, so no per-attachment cleanup is needed. Restore
`OUTLOOK_MCP_MAX_ATTACHMENT_SIZE_BYTES` to its prior value if the run raised it. Step 7
mutates nothing when it behaves as specified; if it succeeded, remove the attachment it
added from `RECEIVED` and record that as the scenario's finding.

## Run log

No runs. The first run appends its record here: the date, the driver invocation, the value
observed for each success-condition bullet, and how many attempts of how many passed.
