---
name: scoring-and-tiers
description: How a surviving distillation candidate is scored across three dimensions, sorted into three tiers, and presented in an analysis report that stays scannable in about a minute.
metadata:
  copyright: Copyright Daniel Grenemark 2026
  version: "0.1"
---

# Scoring, Tiers, and the Analysis Report

Load this when ranking candidates and composing the report, after candidate identification is complete.

## Three scoring dimensions

Each surviving candidate is scored on three dimensions:

1. **Leverage** — how much future work the rule saves or protects. How often the situation it governs recurs, and how many sessions inherit the benefit.
2. **Decay risk** — how likely the knowledge is to be lost or re-litigated if it is not written down. Reasoning that lives only in a session's memory, or in commits about to be squashed, has high decay risk; a fact already half-visible in the code has less.
3. **The cost of the rule being broken** — what it costs when a future session violates the constraint unknowingly. A foot-gun that wastes an hour scores below an invariant whose breach corrupts data.

## Three tiers, must-add to optional

The scored candidates are sorted into exactly three tiers, ordered by priority:

1. **Must add** — high leverage, high decay risk, or high cost of breakage. Knowledge whose loss the project cannot afford.
2. **Recommended** — clearly worth adding, but the project survives a delay.
3. **Optional** — genuine but marginal; a reader may reasonably decline it.

The tiers exist to keep the must-add set small, so the standing instructions grow slowly enough to stay read.

When two candidates carry equivalent leverage but belong to different categories, **a failure narrative outranks the others**. A failure narrative prevents work that has already been proven wasteful — the single most expensive thing a future session can repeat — so at equal leverage it earns the higher tier over an invariant, pattern, or foot-gun of the same weight.

## The analysis report

The report is read-only output: it presents the tiered candidates and stops, awaiting per-tier approval. It **MUST** be **scannable in about a minute**. The tiering serves that budget — a reader who trusts the must-add tier can act on it alone — but each entry must also be terse enough to read at a glance.

Every candidate in the report states three things, and no more than it needs to:

- **What it is** — the knowledge in a sentence, and its category.
- **Where it would live** — the section of the standing instructions it belongs in, or the existing statement it corrects.
- **Why it matters** — the leverage, and its source citation (file location or commit hash) so the reader can verify the claim against its origin.

## Ruled-out candidates are stated, never dropped

A candidate the analysis considers and decides **not** to propose — because it is already documented, because it does not generalise beyond its session, or because its reasoning could not be reconstructed — **MUST** be reported as ruled out, with the reason it was ruled out. It is never dropped silently.

Stating the exclusion and its reason lets the reader catch a wrong call the skill made, and prevents the same candidate from being re-examined from scratch on the next run. A silent omission is indistinguishable from an oversight; a stated one is a decision the reader can review.
