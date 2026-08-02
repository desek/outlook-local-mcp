---
name: iterate-ledger
description: Session ledger for a last-mile iteration session against an implemented Change Request. Records every attempt, its verification evidence, and its disposition, then distils the session into recommended patterns and anti-patterns at close.
cr: "CR-0074"
status: "closed"
opened: "2026-08-01"
closed: "2026-08-02"
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

  * **Addendum, 2026-08-02, after the disposition was recorded.** The
    outstanding item above is now closed by observation. `make crud-test` run
    `2026-08-02T08-04-06`, against a rebuilt binary at `81b2fbd`, reports
    **PASS 47, FAIL 0, SKIP 9**, and Step 11 reads `Busy block 14:00-15:00,
    status=busy`. The previous run reported `12:00:00 - 13:00:00` for the same
    14:00 CEST event, so the population path is confirmed against real Graph
    rather than only by unit test. Steps 31 and 36 also passed with no
    workaround, where previously the agent had to fall back after `to` and
    after the `get_message` summary yielded no attachment IDs.

    The run additionally reported *no* discrepancies between `help` output and
    the prompt's parameter names, the first such run in this session's history.
    That confirms the known instances are fixed. It is not evidence the drift
    class is closed: the two preceding rounds of instance-patching each also
    produced a clean-looking run, and the next run still surfaced new
    instances.

    This addendum is appended rather than folded into the evidence above,
    because the disposition was rendered when the item was genuinely
    unverified, and rewriting that would misrepresent what was known at the
    time.

* **Disposition:** kept

  Recorded from the user's verdict, "If it works. Keep it", after the evidence
  above was reported. The condition was treated as load-bearing rather than
  rhetorical: it is what prompted the live-verification attempt, and when that
  attempt produced no evidence, what prompted closing the coverage gap in code
  instead of recording `kept` on the strength of the formatter test alone.

## Distillation

One attempt, disposition `kept`. The session is short, so the distillation
below is drawn from what happened *within* that attempt as well as from its
outcome. Where a lesson comes from a sub-step rather than from a discarded
attempt, that is stated, so the evidence is not overclaimed.

### Recommended Patterns

* **Prove a new test fails before trusting it.** The free/busy test was run
  against the unfixed formatter first and reproduced the reported symptom
  exactly. Only then was it trusted as evidence the fix worked. A test written
  after a fix and never seen to fail asserts nothing about the fix.

* **Treat a conditional verdict as load-bearing.** "If it works" was read as a
  condition to discharge rather than a formality. Discharging it revealed that
  the formatter test proved nothing about the code reading the timezone from
  the API, which is where the defect actually lived.

* **When live verification is unavailable, close the gap with coverage rather
  than declaring the condition met.** Live end-to-end checking was blocked by
  the environment. The response was to make the uncovered path testable and
  test it, not to record the verdict on the strength of the test that already
  passed.

* **Prefer reusing the existing helper over reimplementing its logic at a new
  call site.** The first version of the fix inlined timezone handling that an
  existing helper already performed. Replacing the inline version with a call
  to that helper removed a second implementation that would have drifted, and
  the test then written for it covers both call sites at once.

* **A helper used by two call sites and tested by neither is where defects
  live.** The shared display-time helper had no test at all. The defect was not
  in the formatter that had tests; it was in the untested path feeding it.

* **A failing test is a question, not a verdict.** The new population-path test
  failed on first run, and the test was wrong, not the code: it asserted a
  24-hour rendering where the helper emits 12-hour. Reading the actual output
  before assuming a defect avoided "fixing" correct behaviour.

* **Rebuild the binary a harness is configured to drive before running it.**
  The harness drives a built artefact by path, not the working tree. A run
  against a stale build silently tests code that is not the code under
  judgement, and its report will look authoritative while describing the wrong
  commit.

### Anti-Patterns

**No attempt in this session was discarded**, so this list does not carry the
discarded-attempt evidence a longer session would produce. What follows is
drawn from approaches abandoned *within* the kept attempt, and from a pattern
visible across the session's history. Stated as such rather than presented as
settled discarded entries.

* **Fixing drift instance by instance does not close the class.** Three
  successive rounds corrected the specific wrong parameter names then known.
  Each produced a clean-looking verification run, and two of the three were
  followed by a run surfacing new instances of the same defect. A check that
  derives its cases from the authoritative source, rather than a list of known
  bad cases, is what closes such a class. The pattern here mirrors the
  hand-listed test that let the governing defect through in the first place.

* **Do not build to a scratch path to drive a credentialed server.** An attempt
  to verify live built the binary to a temporary location. On macOS, keychain
  ACLs are per-binary, so the new path raised a GUI authorisation prompt that
  nothing in a non-interactive session could answer. The process hung until
  killed, twice, producing no evidence. Where a credentialed local service must
  be exercised, drive the artefact the credential is already bound to.

* **Do not count structured report rows with a pattern that also matches the
  report's own summary.** A naive count of status markers matched the tally row
  `| FAIL | 0 |` and produced a "FAIL=1" reading on a run with zero failures.
  Anchor row patterns to the row shape, and prefer a document's own stated
  totals over recomputing them.

* **One clean verification run is not proof a class of defect is closed.** It
  is evidence the known instances are fixed. The distinction matters when the
  same clean signal has previously preceded new instances of the same defect.
