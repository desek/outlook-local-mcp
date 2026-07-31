---
name: iterate-ledger
description: Session ledger for a last-mile iteration session against an implemented Change Request. Records every attempt, its verification evidence, and its disposition, then distils the session into recommended patterns and anti-patterns at close.
cr: "CR-XXXX"
status: "{open | closed}"
opened: "{YYYY-MM-DD when the session was opened}"
closed: "{YYYY-MM-DD when the session was closed, blank while open}"
source-branch: "{Git branch the session started from, from `git rev-parse --abbrev-ref HEAD`}"
source-commit: "{short commit hash the session started from, from `git rev-parse --short HEAD`}"
worktree: "{absolute path of the working tree the session was opened in, from `git rev-parse --show-toplevel`}"
---

<!--
=============================================================================
ITERATION SESSION LEDGER
=============================================================================

This ledger is maintained by the agent during a last-mile iteration session.
It is the sole durable record of session state: it survives context loss, so a
fresh agent reconstructs the session from this file plus the checkpoint commits
alone.

APPEND-ONLY. The ledger is append-only within a session. Never delete, rewrite,
or overwrite an earlier entry. A discarded attempt is retained in full for the
life of the session and beyond close: it is the anti-pattern evidence that
exists nowhere else, and removing it defeats the entire purpose of the ledger.
When a later attempt reverses or supersedes an earlier one, record that reversal
as a new entry that references the earlier one, rather than editing the earlier
entry.

ENTRY STATES. An entry is written when an attempt STARTS, before the code change
is made, and is marked `open` until the user renders a verdict. Once a
disposition is recorded the entry becomes `settled`. This split is what makes an
interrupted session recoverable: a fresh agent can tell an attempt that was
abandoned mid-flight from one that was simply never judged. A session MUST NOT
be closed while any entry is still `open`.

FRONTMATTER. No `metadata.copyright` or `metadata.version` field appears in this
frontmatter, consistent with the convention for documents under `docs/cr/`.
=============================================================================
-->

# Iteration Session Ledger: {short title of what this session is closing}

## Session Context

<!--
What this session is trying to close, and the state it starts from. Reference
the governing Change Request rather than restating it: the CR records what was
specified beforehand, this ledger records what was attempted afterwards.
-->

* **Governing Change Request:** CR-XXXX — {one-line reminder of what it delivered}
* **Gap being closed:** {what about the delivered implementation is not yet what was wanted}
* **Starting point:** branch `{source-branch}` at commit `{source-commit}`, working tree `{worktree}`

## Attempt Ledger

<!--
One numbered entry per attempt, in the order attempted. Each entry is readable
in isolation: a reader recovering context can understand any single attempt
without reading the whole ledger.

Copy the shape below for each new attempt. Write the entry (state `open`) when
the attempt STARTS; fill in the disposition and flip the state to `settled` once
the user renders a verdict.
-->

### Attempt 1 — {short name of the hypothesis}

* **State:** {open | settled}
* **Hypothesis:** {what is being tested — the change believed to close the gap, and why}
* **Surface touched:** {the files or surfaces changed by this attempt}
* **Verification evidence:** {the observed behaviour after the change — checks run, output seen, what the evidence shows. Reported to the user before a disposition is requested.}
* **Disposition:** {kept | discarded | partially-kept}

<!--
DISPOSITIONS. Exactly one of the following, recorded verbatim from the user's
verdict — never inferred by the agent:

  kept            The attempt worked and survives in full. The code change is
                  checkpointed together with this entry.

  discarded       The attempt did not work and was reverted in full from the
                  working tree. No code survives; this entry alone is the
                  record. It is retained for the life of the session.

  partially-kept  Part of the attempt survives and part was reverted. The entry
                  MUST state both sides of the split explicitly — see the two
                  fields below, which appear only for this disposition.
-->

<!--
For a `partially-kept` disposition, add these two fields to the entry above:

* **Portion kept:** {which part of the attempt survives, and where it now lives}
* **Portion reverted:** {which part was reverted from the working tree, and why it did not survive}
-->

## Distillation

<!--
Left empty until the session is closed. On close, set the frontmatter `status`
to `closed`, record the `closed` date, and populate the two lists below by
distilling the settled entries above. Recommended patterns come from what
worked; anti-patterns come from what was discarded, including the reason each
approach did not work. This distillation is then handed to the existing
distillation workflow; the guidance it writes into standing instructions
describes the practice and never names this Change Request or session.
-->

### Recommended Patterns

<!-- What worked, expressed as durable guidance. Empty until close. -->

### Anti-Patterns

<!-- What did not work, drawn from the discarded attempts, with the reason each failed. Empty until close. -->
