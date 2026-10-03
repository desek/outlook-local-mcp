---
date: 2026-09-01
source: cr (CR-0078)
surface: cli (MCP stdio client driving the built ./outlook-local-mcp binary)
outcome: not-run
runs: "0 of 0 attempted; derived from the CR, not yet run against a live mailbox"
coverage_gap: false
---

# Managing a received message end to end

## Status

**Derived, not yet run.** This artifact is the executable form of the acceptance
requirement, written so the run can be performed without re-deriving it. No run has
happened: the scenario needs a live authenticated mailbox and a paid harness run, and
neither was available to the session that wrote it. The `outcome` and `runs` frontmatter
fields say so, and they MUST be rewritten by whoever performs the first run rather than
left as they are.

Nothing below has been observed. Every unit test covering these four verbs drives an
`httptest` server with canned JSON, so no evidence yet exists that any of the four has
issued a real Microsoft Graph call.

## Goal

A person has a message sitting in their Inbox and wants to work it: mark it for follow-up,
label it, mark it read, and file it away in another folder. They then want to open it
again where it now lives.

The surface is the MCP server as an assistant drives it, over stdio, against the built
binary, with the `mail` aggregate tool and its `operation` verb dispatch. It is not the
handler constructors in `internal/tools/`.

## Preconditions

* An authenticated account is present in the token cache and resolves as the default
  account. The four verbs write to a real mailbox; there is no dry-run path.
* `OUTLOOK_MCP_MAIL_MANAGE_ENABLED=true`, which is what registers all four verbs. With
  `OUTLOOK_MCP_MAIL_ENABLED` alone they are absent from the operation enum.
* `OUTLOOK_MCP_READ_ONLY` is unset or false. Under read-only every step below is refused
  by design, which is a separate assertion and not this scenario.
* The Inbox holds at least one message that can be modified and a second folder exists to
  move it into (Archive is the usual one).
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

1. `mail` with `operation="list_folders"`. Record the folder ID of the destination folder
   (Archive) as `DEST`.
2. `mail` with `operation="list_messages"`, scoped to the Inbox. Choose one message.
   Record its ID as `ORIGINAL` and its subject.
3. `mail` with `operation="get_message"`, `message_id=ORIGINAL`. Record the existing
   `flagStatus`, `categories`, and `isRead` values, so the mailbox can be restored.
4. `mail` with `operation="set_flag"`, `message_id=ORIGINAL`, `flag_status="flagged"`.
5. `mail` with `operation="set_categories"`, `message_id=ORIGINAL`,
   `categories="Blue category"`.
6. `mail` with `operation="mark_read"`, `message_id=ORIGINAL`, `is_read=true`.
7. `mail` with `operation="get_message"`, `message_id=ORIGINAL`, `output="raw"`. This is
   the read-back that grades steps 4 through 6 against the mailbox rather than against
   the write confirmations.
8. `mail` with `operation="move_message"`, `message_id=ORIGINAL`,
   `destination_folder_id=DEST`. Record the identifier the confirmation reports as `NEW`.
9. `mail` with `operation="get_message"`, `message_id=NEW`.
10. `mail` with `operation="get_message"`, `message_id=ORIGINAL`.

## Success condition

Graded on the mailbox state read back in steps 7, 9, and 10, never on the wording of the
write confirmations. A confirmation is what the scenario is testing, so it cannot also be
the thing that grades it.

* Step 7 reports `flagStatus` `flagged`, `categories` exactly `["Blue category"]`, and
  `isRead` true.
* Step 8's confirmation names an identifier different from `ORIGINAL`, names `DEST`, and
  states that the original identifier no longer resolves.
* Step 9 succeeds and returns the same subject recorded in step 2, with the message now in
  the destination folder.
* Step 10 fails: `ORIGINAL` no longer resolves after the move. A step 10 that succeeds
  falsifies the consequence sentence the move confirmation states, which is the single
  claim in this scenario a caller cannot verify any other way.

## Restoration

The mailbox is a real one and this scenario mutates it. After grading, restore the values
recorded in step 3 against `NEW`: set the flag status back, set the categories back
(supply an empty value to clear when there were none), and set the read state back. Then
move the message from `DEST` back to the Inbox, which mints a third identifier; that is
expected and is not a failure.

## Run log

No runs. The first run appends its record here: the date, the driver invocation, the value
observed for each success-condition bullet, and how many attempts of how many passed.
