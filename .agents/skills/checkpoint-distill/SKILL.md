---
name: checkpoint-distill
description: Slash command that reads the durable artifacts a completed unit of work leaves behind, identifies knowledge that should outlive the change, ranks it by leverage against decay risk, and on per-tier approval writes it into the project's standing instructions as narrative in which every rule travels with the reasoning that produced it. Trigger with /checkpoint-distill CR-XXXX or /checkpoint-distill --branch.
license: Apache-2.0
metadata:
  copyright: Copyright Daniel Grenemark 2026
  version: "0.0"
---

# /checkpoint-distill

Turns the artifacts of finished work into standing guidance a future session inherits. A coding agent is episodic: each session begins without the context of the last one — what was tried, what failed, what stuck, and why. Standing instructions carry that across the gap, but only if something deliberately puts the knowledge there. This skill reads the durable record a unit of work leaves behind, extracts the constraints, failure narratives, and foot-guns worth keeping, ranks them, and — on explicit per-tier approval — writes them into the project's standing instructions as narrative rather than as a list of bare rules.

A bare rule does not survive contact with inconvenience: "do X" tells a future reader what, not why, so the first time the constraint is awkward it gets stripped as arbitrary. The reasoning is the load-bearing part. That is why this skill writes narrative carrying the mechanism, the cost, and the history behind each rule, never a stripped constraint.

**Usage:**

| Invocation | Scope |
|---|---|
| `/checkpoint-distill CR-XXXX` | Change Request scoped: analyses that Change Request's durable artifacts. **This is the default mode.** |
| `/checkpoint-distill --branch` | Branch scoped: analyses the current branch's commits, delimited by the merge base with the default branch. |

Both scopes name a semantic unit of work. There is deliberately **no** mode that analyses an arbitrary count of most-recent commits: a commit count is neither a semantically bounded unit nor stable across a merge, so a window of *N* commits silently changes meaning the moment history is squashed. If a scope is not supplied and cannot be resolved, the skill asks rather than inventing a commit window.

## Invocation Contract

The two scopes above are the only two the skill accepts. Everything below constrains how they behave.

### Change Request scope is the default

Invoked with a Change Request identifier and no other argument, the skill scopes the entire analysis to that Change Request's artifacts. It bounds the work by the unit the identifier names, never by a count of recent commits. Change Request scope is the default because it is the scope whose material survives a squash merge: the tracked documents a Change Request leaves behind persist unchanged through the merge, whereas commit-borne reasoning does not.

### Branch scope is delimited by the merge base

Invoked with `--branch`, the skill analyses the commits on the current branch that are not present on the default branch. The analysed range is delimited by the **merge base** of the current branch with the default branch — the commit at which the branch diverged — so the range is exactly the branch's own work and nothing inherited from the trunk. The range is defined by that divergence point, not by a fixed number of commits back from the tip.

### Refusal on an unresolvable identifier

Before any analysis, the skill resolves the supplied Change Request identifier to its governing document.

- **If the identifier resolves to no Change Request document:** the skill **MUST** refuse to run, **MUST** report which identifier could not be resolved, and **MUST** produce no analysis. It does not fall back to a commit window, guess a neighbouring identifier, or analyse anything in place of the document it could not find. A named refusal is the correct outcome, because an analysis of the wrong unit is worse than none.

## Analysis Is Read-Only by Default

The skill's default behaviour is **analysis without modification**.

- In its default mode the skill reads its inputs, presents its findings, and **stops**. It **MUST NOT** modify any file, and it **MUST NOT** write to the standing instructions, while analysing.
- Writing happens **only** after explicit approval, and approval is **per tier**: the findings are ranked into tiers, and the user may accept one tier while declining another. Only an approved tier is ever written.
- There is deliberately **no** invocation that writes every tier without the user having selected them. No flag, argument, or mode applies all findings in one step. The gate between a candidate and the standing instructions is always a human choosing that tier, because a wrong rule written into standing instructions is authoritative and unexamined, and inherited by every future session. Skipping the selection is the one thing this skill will not offer.

The mechanics of input resolution, candidate identification and ranking, the approval exchange, and the written output are specified in the sections that follow.

## Input Resolution

Before any finding is produced, the skill resolves the inputs its scope makes available and records which were present and which were absent. What it resolves depends on the scope.

### Change Request scope: four inputs, ranked by durability

A Change Request scoped run gathers up to four inputs. The first three are tracked files that survive a squash merge and are the load-bearing material; the fourth is commit-borne and available only before the branch merges.

| Input | Conventional location | Survives a squash merge |
|---|---|---|
| The Change Request document | `docs/cr/CR-XXXX-*.md` | yes |
| The validation report | `docs/cr/CR-XXXX-validation-report.md` | yes |
| The iteration ledger | `docs/cr/CR-XXXX-iterate.md` | yes |
| Checkpoint commits for the identifier | Git history, matched by subject scope (below) | no |

Resolve each of the three files at its conventional path. The Change Request document itself must resolve, or the run refuses per the invocation contract. The validation report and the iteration ledger are optional: a project may run neither an iteration session nor a validation pass, so either or both may be absent, and their absence is a normal state rather than an error.

**Checkpoint commits span two subject scopes.** A single Change Request's commits are written under two distinct subject forms, and gathering only one silently drops the other. Match **both**:

```bash
# The plain form: the core implementation workflow's checkpoints.
git log --grep '^checkpoint(CR-XXXX):'

# The iteration-session variant: carries the iteration-session reasoning.
git log --grep '^checkpoint(CR-XXXX-iterate):'

# Both together, which is what this skill gathers.
git log --grep '^checkpoint(CR-XXXX\(-iterate\)\?):'
```

The iteration-session commits carry the hypotheses, discarded attempts, and partial-keep reasoning that the plain implementation commits do not, so omitting the `-iterate` scope would drop the highest-value commit-borne material. Gathering for an identifier **MUST** match both scopes.

### Branch scope: the commit range delimited by the merge base

A branch scoped run has no Change Request document to resolve. Its input is the commits on the current branch that are not present on the default branch. Delimit that range by the **merge base** of the current branch with the default branch — the commit at which the branch diverged — so the range is exactly the branch's own work:

```bash
# The commits unique to this branch, delimited by the divergence point.
git log $(git merge-base HEAD <default-branch>)..HEAD
```

The range is defined by that divergence point, never by a fixed number of commits back from the tip. The default branch is discovered from the repository at run time rather than assumed to carry any particular name.

## Availability Report

The availability report is the load-bearing part of input resolution. It **MUST** precede any finding, so the reader learns which inputs the analysis rested on at the point they begin reading it, not after they have already trusted its conclusions.

- The report **MUST** name every input that was found and every input that was absent. An absent validation report or iteration ledger is stated as absent, not passed over in silence.
- When commit-borne input was unavailable, the report **MUST** say so plainly, and the analysis **MUST NOT** present itself as equivalent to one that had it. An analysis that could read the checkpoint commits and one that could not are different analyses drawing on different material; the reader needs to know which one they are holding. A thinner analysis that hides its thinness is the failure this report exists to prevent.

**Checkpoint commits do not survive a squash merge.** This project squash-merges pull requests, so a branch's individual checkpoint commits never reach the default branch — ten commits carrying per-phase reasoning collapse into one whose body is a pull request summary. Reading those commits therefore works **only in the window between the work finishing and the branch merging**. A run that needs the commit-borne input **MUST** happen before the branch merges; once it has merged, only the three file-borne inputs remain, and the availability report states that the commits were unavailable.

## Graceful Degradation

The skill is usable in a project that has neither an iteration ledger nor a validation report. When an optional input does not exist, the run **MUST** proceed on the inputs that do rather than failing — degrading to the Change Request document alone if that is all that resolves. Each absent input is named in the availability report, so degradation is visible rather than silent. The only input whose absence stops the run is the Change Request document in a Change Request scoped run, because there is then no unit of work to analyse; that refusal is governed by the invocation contract above.

## Candidate Identification

Once the inputs are resolved and their availability reported, the skill identifies the knowledge worth promoting. This is the read-only heart of the analysis: nothing is written here, and every candidate that emerges is a proposal for later per-tier approval, not a decision.

### Read the standing instructions in full before proposing anything

Before a single candidate is identified, the skill **MUST** read the project's standing instructions **in their entirety**. Already-documented knowledge must never be proposed again — re-proposing what the reader already has wastes the reader's minute and trains them to skim the report, which defeats the tiering that keeps the report short. Reading first is what makes an analysis idempotent: re-running it over an unchanged scope proposes nothing new, because everything promotable is already present.

Coverage is rarely all-or-nothing. Where the standing instructions **partially** cover a candidate — they state the rule but not its mechanism, or warn of the foot-gun but not the specific case that triggers it — the candidate is the **uncovered gap**, never the whole topic. The skill proposes the missing piece and cross-references what already exists, rather than restating the covered part.

### Categories, sourcing, and the ledger

Candidates are drawn from exactly five categories — undocumented invariants, failure narratives, reusable patterns, foot-guns, and drift — and every candidate **MUST** trace to a specific source artifact by file location or commit hash. An iteration ledger's closing findings are **input to be reconciled and ranked**, never copied through unranked.

**Read [references/candidate-categories.md](references/candidate-categories.md)** before identifying candidates, for the full category definitions, the sourcing rule, and why a ledger finding is a raw candidate rather than a conclusion.

### Classify every candidate by origin

Before ranking, every candidate **MUST** be classified, and the report **MUST** state the class for each:

- **In-project** — knowledge the project owns and can change. It belongs in the standing instructions and holds until the project itself changes.
- **Out-of-project** — a defect or quirk in something the project depends on but does not control. The project can only route around it, so the knowledge is a **workaround**, not a rule.

A workaround **expires** when the upstream defect is fixed; written as a permanent rule it becomes false the moment that happens, while reading like settled practice nobody dares remove. Out-of-project candidates are usually the majority, so a report that does not separate them leaves the reader unable to tell their own project's knowledge from scar tissue over someone else's bug.

An out-of-project candidate **MUST** also record the upstream thing, the observed defect, the workaround, and **how to test whether it is still needed** — a workaround with no re-test condition can never be retired.

**Read [references/candidate-categories.md](references/candidate-categories.md)** for the classification rules, the tiebreak when a candidate has both causes, and what an out-of-project entry carries.

## Scoring and Ranking

Each surviving candidate is scored on **leverage**, **decay risk**, and **the cost of the rule being broken**, then sorted into three tiers: must add, recommended, optional. At equal leverage a failure narrative outranks the other categories. The resulting report is read-only, presents each candidate's what, where, and why, and **MUST** be scannable in about a minute. A candidate ruled out is reported with its reason, never dropped silently.

**Read [references/scoring-and-tiers.md](references/scoring-and-tiers.md)** when ranking candidates and composing the report, for the scoring definitions, the tier boundaries, and the report's required shape.

## Approval

Analysis and application are two separate acts, and the gate between them is the user. Nothing the analysis produced is written until the user has approved it, and approval is granted **one tier at a time**.

- **Analysis modifies nothing.** Presenting the tiered report is the end of the read-only phase. No file is touched, and the standing instructions are not written, until an approval is given. The run stops at the report and waits.
- **Approval is per tier.** The user may approve the must-add tier and decline the rest, approve must-add and recommended but not optional, or decline everything. Each tier is accepted or refused on its own, and **only an approved tier is written**. A user who approves one tier while declining another gets exactly the approved tier applied and nothing else.
- **No invocation writes every tier without selection.** There is no flag, argument, or mode that applies all findings in one step. The gate is always a human choosing the tier, because a wrong rule written into standing instructions is authoritative, unexamined, and inherited by every future session — so skipping the selection is the one shortcut this skill will not offer.

## Application

Once a tier is approved, its candidates are written into the project's standing instructions. **How** they are written is the substance of this phase: a rule recorded badly is re-litigated or stripped, so the writing carries as much weight as the selection.

### How approved candidates are written

Approved candidates are written as **narrative prose**, never as stripped rules. Every rule carries the **mechanism** that makes it work, the **cost** of breaking it, and the **history** of what was tried before it stuck — a reader with only the rule can only guess, and guesses get the rule deleted. The target document's structure is **discovered by reading it**, never assumed. A rule that already exists elsewhere is cross-referenced rather than restated, and a statement current reality contradicts is corrected in place rather than supplemented.

**Read [references/writing-guidance.md](references/writing-guidance.md)** once a tier is approved and before writing anything, for the full voice rules, the worked shape, and the structure-discovery procedure.

### The governance reference boundary applies to written guidance

Written guidance describes the **practice** and **MUST NOT** name the Change Request, iteration session, or commit that produced it. The source citation lives in the analysis report for the reader's verification; it does not travel into the guidance. See *Governance Reference Boundary* below for the full rule.

### Never delete; raise pruning as a separate finding

Application **adds and corrects — it never deletes**. The skill **MUST NOT** remove existing content from the standing instructions. Where the analysis believes existing guidance has become obsolete and should be pruned, that removal is **raised as a separate finding for explicit approval**, never performed. Deletion is a distinct decision with its own risk, and it belongs to a human, made deliberately, not folded silently into an addition.

### Destructive Git operations are prohibited

Application creates new commits and never rewrites history. The skill **MUST NOT** perform any destructive Git operation — no `git reset`, `git rebase`, `git commit --amend`, or `git push --force`. This matches the branch-protection and linear-history rules the repository enforces, and is listed again under *Safety Rules* below.

### Close with a checkpoint commit for the governing Change Request

After the approved tiers are written, the skill creates a **checkpoint commit for the governing Change Request**, using the existing checkpoint-commit workflow so the write is linked to the unit of work that produced it. The commit records the change to the standing instructions as a normal, non-destructive commit; it does not amend or squash anything already present.

### Report what landed and what was deferred

The run closes with a **final report** stating what actually happened:

- **What landed** — which tiers were approved and written, and the additions or corrections each produced.
- **What was deferred** — every tier the user declined, each named with the reason for its deferral, so a later run knows what remains rather than re-deriving it. A deferred tier is a decision on record, not a silent omission.

## Portability

This skill encodes nothing about the structure, section naming, or subject matter of any particular project, so it is usable unchanged in any repository. It discovers the shape of the target standing instructions by reading them at the time it runs, and it never assumes a fixed sectioning, index, or naming convention. The invocation contract above refers only to concepts every project shares — a unit of work, a default branch, a merge base, a document that either resolves or does not.

## Governance Reference Boundary

Standing instructions are prohibited territory for governance identifiers. Guidance this skill writes therefore describes the **practice** and **MUST NOT** name the Change Request, iteration session, or commit that produced it. This SKILL file is itself prohibited territory: every identifier placeholder in it is written in a digitless form (`CR-XXXX`, `{CR_ID}`) precisely so no digit-form governance identifier appears where the boundary forbids it.

## Safety Rules

- **MUST** treat Change Request scope as the default and refuse, by name, an identifier that resolves to no document.
- **MUST NOT** define a mode that analyses an arbitrary count of most-recent commits.
- **MUST NOT** modify any file while in the default analysis mode.
- **MUST NOT** provide any invocation that writes every tier without the user having selected them.
- **MUST NOT** perform destructive Git operations: `git reset`, `git rebase`, `git commit --amend`, `git push --force`.
