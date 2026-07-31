---
name: writing-guidance
description: How an approved distillation candidate is written into standing instructions — narrative carrying mechanism, cost, and history, placed into a structure discovered by reading the target rather than assumed, with out-of-project workarounds marked and given a retirement path.
metadata:
  copyright: Copyright Daniel Grenemark 2026
  version: "0.1"
---

# Writing Approved Guidance

Load this once a tier has been approved and before writing anything into the standing instructions.

**How** a rule is written is the substance of this phase: a rule recorded badly is re-litigated or stripped, so the writing carries as much weight as the selection.

## Write narrative, never a bare constraint

Approved candidates are written as **narrative prose**, not as a list of stripped rules. "Do X" tells a future reader what, not why, and the first time X is inconvenient it is changed or removed as arbitrary. So every rule written carries three things travelling with it:

- **The mechanism** — what makes the rule work, the thing it relies on to hold.
- **The cost of breaking it** — what goes wrong, concretely, when a future session violates it unknowingly.
- **The history** — what was tried before this rule stuck, and why that earlier approach failed.

A reader who has the mechanism, the cost, and the history can evaluate whether the rule still applies to their situation; a reader who has only the rule can only guess, and guesses get the rule deleted. The reasoning is the load-bearing part, and it is written into the guidance itself, never left behind in the source it came from.

**Worked shape (generic).** A stripped line reads: *"Always resolve the handle before releasing the lock."* The narrative form reads: *"Resolve the handle before releasing the lock. The lock is what guarantees the handle table is stable while you read it (mechanism); release it first and a concurrent writer can recycle the slot, so the handle you resolve points at the wrong object and the corruption surfaces far from here (cost); an earlier version resolved lazily after release to shorten the critical section, and that is exactly the race it introduced (history)."* The rule is the same; only the second form survives someone deciding the lock is held too long.

## Discover the target's structure by reading it

The target document's organising structure is **discovered by reading it**, never assumed. Projects organise standing instructions differently — some by topic, some by workflow stage, some as a flat list, some with a document-wide index or cross-reference convention. The skill reads the target in full and determines its actual shape before writing a word, then places each addition where that shape says it belongs. An addition written to an assumed structure the document does not use is visibly foreign and gets reverted, so nothing about sectioning, indexing, or naming is presumed in advance.

Additions **match the target's existing voice, formatting, and cross-referencing conventions** — the same heading depth, the same list or prose style, the same way it links between related rules — so a distilled addition is indistinguishable in form from what a human author wrote there.

## Cross-reference what already exists, never restate it

Where a rule being written **already appears elsewhere in the target document**, the addition **cross-references the existing rule rather than restating it**, following whatever cross-reference convention the document already uses. Restating a rule in a second location means the two copies drift apart and a reader cannot tell which is authoritative. The reconciliation done during candidate identification already reduced a partially-covered candidate to its uncovered gap; application honours that by linking to the covered part instead of duplicating it.

## Correct drift, do not supplement it

Where the analysis found an existing statement that **current reality now contradicts**, application **corrects that statement in place** — it does not add a new, true statement alongside the stale one. A stale claim left standing actively misleads, and a reader encountering both cannot tell which is current; that is worse than a missing claim. The correction rewrites the contradicted statement to match reality, preserving its surrounding context and voice.

## What the written guidance must not carry

Written guidance describes the **practice** and **MUST NOT** name the Change Request, iteration session, or commit that produced it. The source citation lives in the analysis report for the reader's verification; it does not travel into the guidance. See the governance reference boundary in `SKILL.md`.

## Writing an out-of-project workaround

A candidate classified as out-of-project is written as a **workaround with an expiry condition**, never as a standing rule. The distinction must survive into the written guidance, because a reader who cannot tell a workaround from a rule will treat both as permanent.

- **Mark it as a workaround where it is written.** The guidance states plainly that the practice compensates for an external defect rather than expressing a project preference. A reader encountering it should immediately know it is not the project's own choice.
- **Name the upstream thing and the defect** in the guidance itself, so a reader can tell what would have to change for the workaround to become unnecessary.
- **Carry the re-test condition into the written text.** The check that reveals the defect is fixed belongs beside the workaround, not only in the analysis report. Without it the workaround has no retirement path and will outlive the bug it compensates for.
- **Keep it separable.** Place workarounds where they can be reviewed and removed as a group — a distinct section, or clearly marked in place — rather than interleaved with rules the project owns. When the upstream fix lands, the reader should be able to find and delete the workaround without re-reading everything.

The mechanism, cost, and history rules above still apply: the mechanism is what the workaround relies on, the cost is what breaks without it, and the history is the defect that made it necessary.
