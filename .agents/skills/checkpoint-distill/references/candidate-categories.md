---
name: candidate-categories
description: The five categories a distillation candidate may belong to, the source-tracing rule every candidate must satisfy, how an iteration ledger's closing findings are treated as input rather than passthrough, and how a candidate is classified as in-project knowledge or an out-of-project workaround.
metadata:
  copyright: Copyright Daniel Grenemark 2026
  version: "0.1"
---

# Candidate Categories and Sourcing

Load this when identifying candidates, after the standing instructions have been read in full.

## The five candidate categories

Candidates are drawn from exactly five categories. Every proposal names the category it belongs to, so the reader can weigh it against its kind.

1. **Invariants the code now depends on that nothing explains.** A constraint the implementation silently relies on — an ordering, a shared assumption, a contract between two components — that is load-bearing but undocumented, so the next change breaks it without warning.
2. **Failure narratives.** A path that was tried and abandoned, with the reason it failed. This is the highest-value category and the hardest to reconstruct after the fact: a pattern that worked is visible in the code, but a pattern that was eliminated is visible nowhere, so the next agent attempts it again at full cost unless something marks it as already ruled out.
3. **Reusable patterns.** A shape or approach that solved the problem well and will recur — worth naming so it is reached for deliberately rather than reinvented.
4. **Foot-guns that cost real debugging time.** A sharp edge that already consumed a measurable amount of someone's session — a non-obvious ordering, a silent failure mode, a tool that lies about success. The evidence of cost is what qualifies it; a hypothetical hazard that cost nothing is not yet a candidate.
5. **Drift.** An existing statement in the standing instructions that current reality now contradicts. Drift is not an addition — it is a correction the analysis surfaces, so a stale rule is fixed rather than left to mislead alongside a newer one.

## Every candidate traces to a source

Every candidate **MUST** trace to a specific source artifact, identified by **file location** (path and section, heading, or line) or by **commit hash**. A candidate with no citable origin is not a finding — it is the skill's own inference, and inference is exactly what this analysis exists to replace with evidence.

Where the reasoning behind a candidate **cannot be reconstructed from its sources** — the ledger records that an approach was discarded but not why, or a validation gap is noted without the cause that produced it — the skill **MUST NOT** record the candidate on a guessed rationale. It **MUST** instead query for the missing context and leave the candidate out until the reasoning is supplied. A rule written on an invented "why" is worse than no rule, because it is authoritative and unexamined; the missing reasoning is requested, never inferred.

## The iteration ledger is input, not passthrough

The iteration ledger's closing findings are the richest single input this analysis has, and the most easily mishandled. They are **input to be reconciled and ranked**, never content to be copied through. The skill **MUST NOT** write a ledger's closing findings into the standing instructions unranked.

The distinction is the point of the skill. **The ledger records what one session learned** — unranked, scoped to that session, unreconciled against anything the project already knows. **This skill decides what every future session should know** — which of those findings generalise beyond the session that produced them, which are already documented, and where each one ranks against every other candidate. A single session cannot see whether its own finding generalises or duplicates existing guidance; that judgement requires the wider view this analysis takes.

So every ledger finding is treated as a raw candidate: reconciled against the standing instructions, deduplicated, scored, and tiered alongside candidates from every other input — or, where it does not survive that reconciliation, ruled out with its reason stated.

## Origin: in-project or out-of-project

The five categories above describe *what kind* of knowledge a candidate is. Origin describes *whose* knowledge it is, and it is classified separately, before ranking, because it determines whether the candidate is a rule or a workaround.

**In-project.** The knowledge describes this project: a convention it adopted, a constraint its own code relies on, an invariant between its own components. The project owns it, can change it, and the knowledge stays true until the project itself changes. In-project candidates become standing instructions in the ordinary way.

**Out-of-project.** The knowledge describes a defect, quirk, or limitation in something the project depends on but does not control — an external tool, a hosted service, an API, a library, a harness or agent capability. The project cannot fix it; it can only route around it. What gets recorded is therefore a **workaround**, and a workaround differs from a rule in three ways that matter:

1. **It expires.** When the upstream defect is fixed, the workaround becomes unnecessary — and if it was written as a permanent rule, it becomes an actively false statement that still reads like settled practice.
2. **It cannot be fixed here.** No amount of work in this repository resolves it. The most a project can do is route around it and, where it matters, report it upstream.
3. **It needs a re-test condition.** Because it expires, it must carry the means of discovering that it has. A workaround with no way to check whether it is still needed is never removed, and accumulates as permanent scar tissue over someone else's bug.

Out-of-project candidates are usually the majority. A report that does not separate them leaves the reader unable to tell which findings are their own project's knowledge and which are compensation for an external defect — and those two need very different treatment, review cadence, and lifetime.

### What an out-of-project candidate carries

An out-of-project candidate **MUST** record four things, and a candidate missing the fourth is incomplete rather than merely terse:

- **The upstream thing** — what is depended on, named specifically enough to check later.
- **The observed defect** — what it does, and what was expected instead. The observation, not a theory about the cause.
- **The workaround** — what the project does instead, and what that costs.
- **How to test whether it is still needed** — the concrete check that reveals the defect has been fixed. This is what lets the workaround be retired rather than inherited indefinitely.

### The tiebreak when a candidate has both causes

Some candidates have an internal and an external cause at once: the project does something unusual *because* a dependency misbehaves. Classify by the **cause that would have to change** for the knowledge to stop being true. If fixing the upstream thing would make the practice unnecessary, it is out-of-project, however much project-side code the workaround touches. If the practice would survive the upstream fix, it is in-project.
