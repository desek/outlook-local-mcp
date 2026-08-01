---
name: iterate-ledger
description: Session ledger for a last-mile iteration session against an implemented Change Request. Records every attempt, its verification evidence, and its disposition, then distils the session into recommended patterns and anti-patterns at close.
cr: "CR-0074"
status: "open"
opened: "2026-08-01"
closed: ""
source-branch: "fix/cr-0074-doc-anchors"
source-commit: "96a215b"
worktree: "/Users/desek/Repo/desek/outlook-local-mcp"
---

# Iteration Session Ledger: closing the harness prompt-drift class

## Session Context

* **Governing Change Request:** CR-0074, which fixed `get_docs` so that a
  heading carrying an explicit `{#custom-id}` tag resolves under that id,
  replaced the hand-listed anchor test with corpus-derived tests, wired one
  `.agents/scripts` harness into CI, and corrected a set of enumerated
  parameter errors in `docs/prompts/mcp-tool-crud-test.md`.

* **Gap being closed:** the anchor half of CR-0074 is settled and verified
  live. The prompt half is not. CR-0074 corrected the specific drifted
  parameters it enumerated, and the very next `make crud-test` run
  (`2026-08-01T22-11-14`, against the fixed build `96a215b`) immediately
  surfaced two more instances of the same class:

  * Step 31 names `to` for `create_draft`; the registry parameter is
    `to_recipients`.
  * Step 36 instructs enumerating attachment IDs via `get_message` with
    `output: "summary"`, but that output carries no attachments array at all,
    only `hasAttachments: true`. `list_attachments` is required.

  Both were found by the harness rather than by any check, and both are the
  same defect shape CR-0074 set out to correct. Fixing instances one at a time
  has now failed to close the class twice, which is the gap this session
  exists to address. CR-0074 already learned the corpus-versus-hardcoded-list
  lesson for documentation anchors; the prompt has not had it applied.

  A third finding from the same run, `get_free_busy` text output returning
  unlocalised UTC times with no timezone label
  (`internal/tools/text_format.go:210`), is recorded here as observed but is
  **new behaviour outside CR-0074's scope**, not a last-mile gap in what
  CR-0074 delivered. It is noted so it is not lost, and it is a candidate for
  its own Change Request rather than for this session.

* **Starting point:** branch `fix/cr-0074-doc-anchors` at commit `96a215b`,
  working tree `/Users/desek/Repo/desek/outlook-local-mcp`.

  Note that pull request #40 is open against this branch and all eight of its
  checks are green. Attempts in this session add commits to that branch, so
  each will re-trigger those checks.

## Attempt Ledger

### Attempt 1 — fix all three findings from the 2026-08-01T22-11-14 harness run

* **State:** settled

* **Hypothesis:** the three findings the harness reported are each independently
  fixable, and all three were verified against source before the attempt rather
  than taken from the report:

  1. **`get_free_busy` text is unlocalised.** `FormatFreeBusyText`
     (`internal/tools/text_format.go:210`) prints `bp.Start` and `bp.End` raw.
     The cause is upstream: `get_free_busy.go:271-276` reads only
     `s.GetDateTime()` and discards `s.GetTimeZone()`, so the timezone Graph
     returns is thrown away before formatting. Events do not have this problem
     because they carry a precomputed `displayTime` built by
     `graph.FormatDisplayTime(startDT, endDT, startTZ, endTZ, isAllDay)`.
     Applying that same helper to busy periods should make free/busy text
     consistent with event text.

  2. **Step 31 names a parameter that does not exist.** The prompt line 574
     passes `to:`; `create_draft.go:39` registers `to_recipients`.

  3. **Step 36 prescribes an impossible enumeration.** The prompt line 620
     instructs calling `get_message` with `output: "summary"` to enumerate
     attachment IDs. That output carries no attachments array, only
     `hasAttachments`. `list_attachments` (`internal/tools/list_attachments.go`,
     taking `message_id`) is the verb that returns attachment IDs.

  The user directed all three in one instruction. They are recorded as one
  attempt but are separable, so a split verdict is expressible as
  `partially-kept`.

  Note that finding 1 is a change to shipped behaviour rather than a last-mile
  gap in CR-0074, and the Session Context above had scoped it out. It is
  included here because the user directed it.

* **Surface touched:** `internal/tools/get_free_busy.go`,
  `internal/tools/text_format.go`, `docs/prompts/mcp-tool-crud-test.md`, plus
  tests for the free/busy formatting change.

* **Verification evidence:**

  * **Fix 1, formatter.** `TestFormatFreeBusyText_PrefersDisplayTime` added and
    **verified to fail on the unfixed formatter**, reproducing the reported
    symptom exactly: `1. CRUD test event\n   2026-08-08T12:00:00 -
    2026-08-08T13:00:00 | Busy`. The formatter change was reverted for that
    check and restored, with `git diff` confirmed clean before the real run.
    `TestFormatFreeBusyText` was extended to assert the raw-ISO fallback path
    still applies when `DisplayTime` is empty.

  * **Fix 1, population path, and the correction it forced.** The formatter
    test supplies `DisplayTime` by hand, so it says nothing about the code that
    reads the timezone from Graph. An attempt to verify that live over MCP
    stdio **failed to produce evidence**: `initialize` responded, but
    `tools/call` never returned within 150 s and the process had to be killed.
    The probable cause is macOS keychain ACLs being per-binary, so a binary
    built to a scratch path raises a GUI prompt nothing can answer in a
    non-interactive session. That line was abandoned rather than pursued.

    Because live verification was unavailable, the gap was closed in code
    instead. The inline timezone handling first written for this attempt was
    replaced by a call to the existing `formatEventDisplayTime`, so free/busy
    and event listings share one implementation rather than two, and
    `TestFormatEventDisplayTime_CarriesTimezone` now covers that shared helper
    for both call sites. That helper had no test at all beforehand.

    That new test failed on first run, and the failure was in the **test**, not
    the code: it asserted `14:00` while the helper renders 12-hour time. The
    observed output, `Sat Aug 8, 2:00 PM - 3:00 PM`, is the correct
    localisation of a 14:00 W. Europe start. The assertion was corrected to
    `2:00 PM` and `3:00 PM`.

  * **Fixes 2 and 3, prompt.** Both corrections verified against the live
    registry before editing: `create_draft.go:39` registers `to_recipients`,
    and `list_attachments.go:50` takes `message_id` while `get_message`
    returns no attachment IDs at any tier. Fix 3 additionally records *why*
    `list_attachments` is required, so the next reader does not re-derive it.

  * **Gates.** `make ci` exit 0, `make security` exit 0. All four free/busy and
    display-time tests pass.

  * **Not verified.** Live end-to-end behaviour of `get_free_busy` against real
    Graph. The population path is now covered by a unit test rather than by
    observation, and the next `make crud-test` run is what will confirm it in
    situ, at Step 11.

* **Disposition:** kept

  Recorded from the user's verdict, "If it works. Keep it", after the evidence
  above was reported. The condition was treated as load-bearing rather than
  rhetorical: it is what prompted the live-verification attempt, and when that
  attempt produced no evidence, what prompted closing the coverage gap in code
  instead of recording `kept` on the strength of the formatter test alone.

## Distillation

### Recommended Patterns

<!-- Empty until close. -->

### Anti-Patterns

<!-- Empty until close. -->
