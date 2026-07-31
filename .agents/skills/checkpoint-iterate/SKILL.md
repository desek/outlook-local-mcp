---
name: checkpoint-iterate
description: Slash command that runs a last-mile iteration session against an implemented Change Request. Opens an attempt ledger, records every attempt with an explicit kept, discarded, or partially-kept disposition, checkpoint-commits code continuously, and closes by distilling the session into patterns and anti-patterns. Trigger with /checkpoint-iterate [CR-XXXX], /checkpoint-iterate close CR-XXXX, or /checkpoint-iterate status CR-XXXX.
license: Apache-2.0
metadata:
  copyright: Copyright Daniel Grenemark 2026
  version: "0.0"
---

# /checkpoint-iterate

Runs an **iteration session** that closes the last-mile gap between an implemented Change Request and the behaviour that was actually wanted. A specification is written before the code exists, so the delivered result is approximately right rather than exactly right. Closing that gap is an interactive loop: the user names what to try, the agent makes the change and reports the evidence, the user renders the verdict. This skill records that loop in a ledger at `docs/cr/{CR_ID}-iterate.md`, so the reasoning — including every discarded attempt — survives the session instead of evaporating with the agent's context.

**Usage:**

| Invocation | Effect |
|---|---|
| `/checkpoint-iterate CR-XXXX` | Opens a session against that Change Request, or resumes one already open |
| `/checkpoint-iterate close CR-XXXX` | Closes the active session and distils the ledger |
| `/checkpoint-iterate status CR-XXXX` | Reports the active session, its attempt count, and its dispositions so far |

Every invocation **MUST** identify its governing Change Request. There is no implicit "current session".

## Role Split

The division of labour governs every step below. It is fixed and **MUST NOT** be reassigned.

- **The user** initiates the session, names each thing to try next, and renders the verdict on each result. The verdict is the user's judgment; the agent records it and **MUST NOT** infer, assume, propose, or substitute its own.
- **The agent** makes the code change, runs the project's checks, reports the evidence, then writes the ledger entry and creates the commit. The agent is the recorder for every attempt, because it is the party present at all of them and already writing to the repository.

The user is therefore never asked to maintain a notebook alongside the work. Their contribution is direction and judgment; the recording is a side effect of the work the agent was already doing.

**Initiation is user-only.** A session **MUST** be opened by explicit user invocation. It **MUST NOT** be started automatically, and **MUST NOT** be spawned by the implementation pipeline. The whole point is that a human has looked at the delivered result and judged it not yet right.

## Workflow

Follow these steps in order.

### Step 1: Resolve and Validate the Governing Change Request

Read the identifier from `$ARGUMENTS` (for `close` and `status`, it follows the sub-command word).

- Confirm the governing Change Request document exists at `docs/cr/{CR_ID}-*.md`.
- **If the document does not exist:** refuse to open a session, report which identifier could not be resolved, and **STOP**. No ledger is created for an unresolved identifier.
- **If the identifier is omitted** and more than one ledger is open in the working tree: refuse to act, list the open ledgers, and **STOP** rather than guessing which session is meant.

### Step 2: Create or Resume the Ledger

Check for an existing ledger at `docs/cr/{CR_ID}-iterate.md`.

- **If none exists:** create it from the bundled template at `templates/ITERATE.md`. Record in its frontmatter the governing Change Request, an open status, the start date, the branch and commit the session starts from, and the working tree it was opened in.
- **If one exists and is open:** resume it. Append to the existing ledger — **MUST NOT** recreate, rewrite, or remove any previously recorded entry. Resuming reconstructs session state without asking the user further questions.

When resuming, before proposing any new attempt the agent reads the governing Change Request and every settled entry in the ledger, including discarded ones, then reports the recovered state: what has been settled, what approaches were eliminated, and what was in flight. An approach a settled entry records as `discarded` **MUST NOT** be proposed again without stating that it was already eliminated and why it is being revisited.

### Step 3: Loop Over Attempts

Each attempt is one pass through this loop. Repeat until the user closes the session.

1. **The user names the next thing to try.**
2. **The agent makes the code change** for that hypothesis and **runs the project's checks** (`bats -r tests/`).
3. **The agent reports the verification evidence to the user** — what was changed, what the checks produced. Evidence is reported **BEFORE** a disposition is requested, so the verdict is rendered against observed behaviour rather than an expectation.
4. **The user renders the verdict:** keep, discard, or keep part.
5. **The agent writes the ledger entry**, recording the hypothesis, the surface touched, the evidence, and the disposition — which is exactly one of `kept`, `discarded`, or `partially-kept`. The recorded disposition is the one the user supplied, transcribed verbatim; the agent does not infer it.
6. **The agent creates the commit** on its own initiative, without waiting to be asked, using the existing checkpoint commit workflow.

A `discarded` entry is never deleted from the ledger — retaining it is the entire point, because it is anti-pattern evidence that exists nowhere else.

### Step 4: Status

On a `status` invocation, report the active session for the given Change Request, its attempt count, and the dispositions recorded to date. Read-only: report state without modifying the ledger or Git.

### Step 5: Close

On a `close` invocation, end the session and distil the ledger. A session **MUST NOT** be closed while any entry remains open; report the open entry as requiring a disposition and stop. The full commit protocol and closing distillation are specified in the sections below.

## Commit Protocol

Code changes made during a session are committed continuously through the existing checkpoint commit workflow (`/checkpoint-commit`), without waiting to be asked. The session does not invent a parallel commit mechanism; it reuses the checkpoint one and distinguishes its commits by **scoping the identifier**, not by changing the type.

**Every session commit uses the scoped subject form:**

```text
checkpoint({CR_ID}-iterate): {summary}
```

The unsuffixed form `checkpoint({CR_ID}): {summary}` **MUST NOT** be used by a session — it is reserved for the core agentic implementation workflow.

**Why the scope is suffixed rather than the type replaced.** Reusing the `checkpoint` type keeps session work visible to the context-recovery history query, whose pattern matches `^checkpoint.*:` — which is correct, because iteration work is part of what a later session needs to recover. The `-iterate` suffix on the scope keeps session work separable from implementation work by a subject-line query alone:

```bash
git log --grep '^checkpoint(CR-XXXX):'          # core implementation only
git log --grep '^checkpoint(CR-XXXX-iterate):'  # iteration session only
git log --grep '^checkpoint.*:'                 # both, the default recovery view
```

**Code and evidence are committed atomically.** A checkpoint that contains code changes **MUST** also contain, in the same commit, the ledger entry for the attempt it embodies. Change and evidence are never separated across commits.

**A discarded attempt leaves no code.** After the working tree is reverted, its commit touches the ledger alone — still a session checkpoint carrying the same scoped subject, identified by the path it changed rather than by a different subject convention:

```bash
git log --grep '^checkpoint(CR-XXXX-iterate):' -- docs/cr/CR-XXXX-iterate.md
```

## Re-hydration and Concurrency

**Re-hydration after context loss.** The ledger lives on disk, so it survives context loss entirely. Recovering a cleared or new session takes exactly **one** user action — the same invocation used to open the session — after which the agent reconstructs state with no further questions:

1. Read the governing Change Request and the full ledger, including **every** settled entry (kept, discarded, and partially-kept alike).
2. Read the checkpoint commits for that Change Request via the context-recovery history query.
3. Compare the working tree against the last commit. Uncommitted changes belong to the open entry, if there is one.
4. Report the recovered state to the user: what has been settled, what approaches were eliminated, and what was in flight.
5. If an entry is still open, reconcile the uncommitted working-tree changes against it and **request its disposition before starting any new attempt**. Resuming without settling it would fold unjudged work into the next attempt and misattribute it.

Reading the discarded entries matters as much as reading the kept ones: a re-hydrated agent **MUST NOT** re-propose an approach a settled entry records as `discarded` without stating that it was already eliminated and why it is being revisited.

**Concurrency.** Two sessions may run at once, but not in the same working tree. The constraint is Git, not the ledger: the checkpoint staging would otherwise sweep one session's in-flight changes into the other's commit, misattributing work and cross-contaminating both ledgers. Three rules make concurrency safe:

- **One active session per working tree.** A second concurrent session runs in its own Git worktree. This is the primary isolation and the only mechanism that fully separates the two. (Creating the worktree stays with the user; the skill does not create it.)
- **Scoped staging.** A session stages **only** the paths it touched — its ledger and the files of the attempt in hand — and **MUST NOT** stage the entire working tree. This bounds the damage if two sessions do share a tree.
- **Explicit identification.** Every invocation names its Change Request; there is no implicit "current session". Where an invocation omits the identifier and more than one ledger is open in the working tree, the skill refuses and lists the open ledgers rather than selecting one.

**Foreign-worktree detection.** The ledger records the working tree it was opened in. A session resumed in a working tree other than the one it records **MUST** be detected and reported to the user, rather than proceeding silently against a different tree.

## Closing

On a `close` invocation, once no entry remains open:

1. **Set the ledger status to closed and record the closing date** in the frontmatter.
2. **Populate the distillation section**, separating the findings into two lists: what worked, expressed as **recommended patterns**, and what did not, expressed as **anti-patterns**. The anti-patterns are drawn from the discarded entries and state the reason each approach did not work — this is the knowledge that exists nowhere else and the reason the ledger is kept.
3. **Hand the result to the existing distillation workflow** (`/checkpoint-distill`), which routes durable practices into the project's standing instructions. The session **MUST NOT** define a competing distillation mechanism or a second destination; the ledger is a strictly richer input than commit history because it contains the discarded attempts commit history omits.

> **Prerequisite:** the closing hand-off consumes the distillation workflow, which this repository ships as a sibling skill. Installing this skill on its own still gives a working open, iterate, and record loop; only the close-step hand-off needs the distillation skill present alongside it.

**Governance reference boundary.** Guidance written into the project's standing instructions as a result of distillation **MUST** describe the *practice* and **MUST NOT** name the Change Request or the session that produced it. Standing instructions are prohibited territory for governance identifiers, so distilled guidance names patterns and anti-patterns, never the document or ledger they came from. (The ledger itself lives under `docs/cr/`, which is permitted territory, so it may name its governing Change Request freely.)

## Safety Rules

- **MUST NOT** perform destructive Git operations: `git reset`, `git rebase`, `git commit --amend`, `git push --force`. Reverting a discarded attempt is limited to the working tree.
- **MUST** record the user's verdict verbatim and **MUST NOT** infer, assume, or substitute a disposition.
- **MUST** report evidence before requesting a disposition.
- **MUST** refuse to open a session against a Change Request whose document does not exist, naming the unresolved identifier.
