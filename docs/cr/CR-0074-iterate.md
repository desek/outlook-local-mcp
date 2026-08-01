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

<!--
No attempts yet. The user names the next thing to try; the agent then makes the
change, runs the checks, reports the evidence, and records the verdict here
verbatim.
-->

## Distillation

### Recommended Patterns

<!-- Empty until close. -->

### Anti-Patterns

<!-- Empty until close. -->
