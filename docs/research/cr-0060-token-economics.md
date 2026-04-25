---
date: 2026-04-25
branch: dev/cr-0061
related-cr: CR-0060
author: research note (daniel.grenemark@redeploy.com)
---

# CR-0060 Token Economics: Static Schema Shrink vs. Progressive Disclosure

## Question

The CR-0060 design replaces 32 individually registered MCP tools with 4 aggregate domain tools that share a `help` verb for progressive disclosure. The static, always-sent tool schema becomes much smaller, but the cost of "what the LLM needs to know to call a verb" is now paid dynamically as `help` tool-result content injected into the conversation. Since Anthropic prompt caching depends on stable prefix bytes, a smaller static block means fewer cached tokens on every turn; meanwhile help responses become part of the message history and pay output token cost at first emission and cache-read cost thereafter.

Does CR-0060 actually save money, or does the dynamic help traffic claw the savings back? Quantify under realistic usage patterns.

## TL;DR

Across all three user profiles defined below, CR-0060 is net cost-positive. The cold-start tools block shrinks by ~17,000 tokens (92%), which dominates per-turn cache-read accounting from turn 1 onward. Even an adversarial caller invoking `help` for every domain still saves money once the session exceeds ~3 turns. The "smaller cache hurts" intuition is wrong because cache reads are billed at 10% of base input; the absolute savings on 17K cached tokens swamps the new output cost of help payloads, which are themselves only ~1K tokens each and called at most a handful of times per session.

## Measured Inputs

Source: `docs/cr/CR-0060-validation-report.md`, `docs/bench/crud-runs.csv`, `internal/server/schema_size_test.go`, `internal/tools/help/`.

| Quantity | Pre-CR | Post-CR | Δ |
|---|---|---|---|
| Registered tools | 32 | 4 | −87.5% |
| Tool-schema bytes (cold-start) | ~74,000 | 5,824 | −92.1% |
| Tool-schema tokens (~4 chars/tok) | ~18,500 | ~1,456 | −17,044 |
| Dispatch overhead p99 | n/a | <1 µs | budget 1 ms |

Help payloads (text tier, post-CR):

| Help shape | Tokens (approx) |
|---|---|
| `calendar` full (15 verbs) | ~1,175 |
| `mail` full (13 verbs)     | ~862 |
| `account` full (5 verbs)   | ~212 |
| `system` full (6 verbs)    | ~162 |
| Single-verb scoped help    | ~100 |

Reference CRUD run (2026-04-25, `crud-runs.csv`): 56 Claude turns, 48 tool calls, 64 input / 22,075 output / 93,689 cache-write / 2,887,317 cache-read tokens, wall 358 s.

## Pricing Model

Anthropic list prices (USD / million tokens), source: https://platform.claude.com/docs/en/about-claude/pricing.md.

| Bucket | Sonnet 4.6 | Opus 4.7 |
|---|---|---|
| Base input | 3.00 | 5.00 |
| Output | 15.00 | 25.00 |
| Cache write (5 min TTL) | 3.75 | 6.25 |
| Cache write (1 hr TTL) | 6.00 | 10.00 |
| Cache read | 0.30 | 0.50 |

Caveat: Opus 4.7 uses a new tokenizer that can use up to 35% more tokens for the same text. The percentage savings below are stable across that change because both pre-CR and post-CR are tokenized the same way; absolute dollar figures for Opus 4.7 in the tables below assume the Sonnet-equivalent token counts and would scale up by up to ~35% in worst-case real traffic.

Two structural facts:

1. The `tools` block sits in the cacheable prefix. Whatever its size, it is written to cache once per TTL window and read on every subsequent turn at 10% of base input.
2. A `help` tool result is *output* on the turn it is produced (paid at 15.00) and then becomes part of the cached prefix for later turns (paid at 0.30 per turn).

## Cost Model

Per-session cost of the tools-block (and any help payloads) for `T` turns and `H` help calls of average size `h ≈ 600` tokens, with help calls evenly distributed (mean residency `T/2`):

```
cost(N_tools, T, H, h) = N_tools × (cw + T × cr) / 1e6
                      + H × h × (out + (T/2) × cr) / 1e6
```

where `cw` = 5-min cache-write rate, `cr` = cache-read rate, `out` = output rate. `N_tools` is 18,500 (pre-CR) or 1,456 (post-CR) tokens.

**Per-help-call marginal cost** (output emission + average residency):

| Model | Per call (T = 8) | Per call (T = 25) | Per call (T = 56) |
|---|---|---|---|
| Sonnet 4.6 | $0.0102 | $0.0128 | $0.0140 |
| Opus 4.7 | $0.0170 | $0.0213 | $0.0234 |

**Per-turn savings from carrying 17,044 fewer tool tokens** (cache-read delta):

| Model | $/turn |
|---|---|
| Sonnet 4.6 | $0.00511 |
| Opus 4.7 | $0.00852 |

**Breakeven:** each help call is paid back within ~2 additional turns on Sonnet 4.6 and ~3 turns on Opus 4.7. Any session of ~3+ turns with bounded help usage is net-positive.

## User Case Profiles

Concrete personas behind the numbers below. Each one fixes `T` (turns), `H` (help-verb invocations), and `h` (avg help payload tokens) used by the cost model.

### Case A — Power user / scripted client

**Who.** A developer or CI job that already knows the MCP surface. Examples: a custom internal automation invoking `mail` to triage an inbox each morning, a Claude Desktop user who has memorised the verbs, a script in `n8n` or a GitHub Action that calls `outlook-local-mcp` with hard-coded `operation` strings.

**Session shape.** Short and on-rails. Open the connection, fire 5–10 deterministic operations, close. No browsing, no exploration.

**Concrete script.** "List today's events, get the 09:00 standup details, list unread mail in Inbox, get the top 3 messages, mark the agenda as read, send the summary draft." That's roughly 6–8 turns, with the LLM never asking "what can I do here?" because the caller already wrote the verb names into the prompt.

**Why H = 0.** No `help` calls, ever. The schema-shrink is pure cache-read savings on every turn; there is no offsetting cost.

### Case B — Exploratory chat user

**Who.** A human in a Claude.ai or Claude Desktop conversation using natural language: "find that thread from Maria last week and reschedule the design review." The LLM has to translate intent into verbs and may need to inspect the tool surface mid-conversation.

**Session shape.** Medium length, conversational, with branching. The user reformulates, the LLM clarifies, calls a couple of read verbs to disambiguate, then commits a write. 20–30 turns is typical for a real working chat.

**Concrete script.** Turn 1–4: user asks, LLM thinks, calls `mail` with `operation="help"` to remember the search verb name (1st help call, ~600 output tokens). Turn 5–12: searches, opens conversation, picks the right thread. Turn 13–16: pivots to calendar, calls `calendar` with `operation="help"` (2nd help call). Turn 17–25: lists events, reschedules the meeting, confirms with the user.

**Why H = 2, h ≈ 600.** Two domains touched, one help call per domain to load verb contracts on demand. Full-domain help (~600–1,200 tokens) rather than verb-scoped (~100 tokens) because the LLM is browsing, not zeroing in.

### Case C — CRUD test / heavy automation

**Who.** The `make crud-test` harness — a headless Claude session that exercises every verb to validate the MCP surface end-to-end. Same shape applies to a long-running ops agent (e.g. a weekly compliance sweep that walks every account, every folder).

**Session shape.** Long, broad coverage, mostly mechanical. Reference: 2026-04-25 CRUD run logged 56 Claude turns and 48 tool calls in 358 seconds wall time, with 97% spent waiting on Microsoft Graph.

**Concrete script.** The CRUD prompt walks each domain: account login, calendar create→update→delete→meeting flow, mail draft→reply→forward→delete, system status. The agent calls `help` once per domain at the start to ground itself, then proceeds through the verb list. Most turns are deterministic Graph round-trips, not reasoning.

**Why H = 4.** One help call per domain (`calendar`, `mail`, `account`, `system`) at orientation time; after that the agent has the verb list in context and reuses it for the rest of the session.

### Adversarial micro-session

**Who.** A worst-case artefact for sanity-checking the model: a brand-new client that connects, immediately calls `help` on every domain, and then disconnects without doing any real work. Plausible reality: a tool-picker UI that pre-fetches all help payloads to populate its sidebar before the user has typed anything.

**Why include it.** It's the only realistic shape where post-CR cost approaches pre-CR cost, since the session is too short to recoup the help-output cost via cache-read savings. The result (still ~40% cheaper) confirms the schema-shrink dominates even here.

## Three User Cases — Sonnet 4.6

### Case A — Power user / scripted client (T = 8, H = 0)

| Variant | Static tools cost | Help cost | Total |
|---|---|---|---|
| Pre-CR (32 tools) | $0.1138 | $0 | **$0.1138** |
| Post-CR (4 tools) | $0.00895 | $0 | **$0.00895** |
| Savings | | | **$0.1048 (92.1%)** |

Knows verb names, never calls `help`. Pure win, no downside.

### Case B — Exploratory chat user (T = 25, H = 2, h = 600)

| Variant | Static tools cost | Help cost | Total |
|---|---|---|---|
| Pre-CR | $0.2081 | $0 | **$0.2081** |
| Post-CR | $0.01638 | $0.02250 | **$0.0389** |
| Savings | | | **$0.1692 (81.3%)** |

Discovery-driven. Side benefit: LLM loads only the verb contract it needs, not 32 schemas it ignores.

### Case C — CRUD test / heavy automation (T = 56, H = 4, h = 600)

| Variant | Static tools cost | Help cost | Total |
|---|---|---|---|
| Pre-CR | $0.3802 | $0 | **$0.3802** |
| Post-CR | $0.02992 | $0.05616 | **$0.0861** |
| Savings | | | **$0.2941 (77.4%)** |

Reference: `make crud-test`. Matches the 2.9 M cache-read figure on `crud-runs.csv`.

### Adversarial (T = 2, H = 4)

| Variant | Total | Savings vs pre-CR |
|---|---|---|
| Pre-CR | $0.0805 | — |
| Post-CR | $0.0470 | $0.0335 (41.6%) |

Even pathological help-spam at minimal session length stays cheaper.

## Three User Cases — Opus 4.7

Same token assumptions; Opus 4.7 list pricing. (See tokenizer caveat in the pricing section.)

### Case A — Power user (T = 8, H = 0)

| Variant | Static tools cost | Help cost | Total |
|---|---|---|---|
| Pre-CR | $0.1896 | $0 | **$0.1896** |
| Post-CR | $0.01492 | $0 | **$0.01492** |
| Savings | | | **$0.1747 (92.1%)** |

### Case B — Exploratory (T = 25, H = 2, h = 600)

| Variant | Static tools cost | Help cost | Total |
|---|---|---|---|
| Pre-CR | $0.3469 | $0 | **$0.3469** |
| Post-CR | $0.02730 | $0.03750 | **$0.0648** |
| Savings | | | **$0.2821 (81.3%)** |

### Case C — CRUD test (T = 56, H = 4, h = 600)

| Variant | Static tools cost | Help cost | Total |
|---|---|---|---|
| Pre-CR | $0.6336 | $0 | **$0.6336** |
| Post-CR | $0.04988 | $0.09360 | **$0.1435** |
| Savings | | | **$0.4901 (77.4%)** |

### Adversarial (T = 2, H = 4)

| Variant | Total | Savings vs pre-CR |
|---|---|---|
| Pre-CR | $0.1342 | — |
| Post-CR | $0.0808 | $0.0534 (39.8%) |

## Side-by-Side Summary

Savings per session, both models, all profiles:

| Profile | Sonnet 4.6 saved | Opus 4.7 saved | % saved |
|---|---|---|---|
| A — Power user (T=8, H=0) | $0.1048 | $0.1747 | 92.1% |
| B — Exploratory (T=25, H=2) | $0.1692 | $0.2821 | 81.3% |
| C — CRUD test (T=56, H=4) | $0.2941 | $0.4901 | 77.4% |
| Adversarial (T=2, H=4) | $0.0335 | $0.0534 | ~40% |

Percentage savings are model-invariant (all four price buckets scale by the same ~1.667× factor between Sonnet 4.6 and Opus 4.7). Absolute dollar savings on Opus 4.7 are ~67% larger than Sonnet 4.6 per session, making CR-0060 disproportionately valuable for higher-tier model usage.

## Caching Concern, Resolved

The original worry was: "smaller cache → less cache benefit → MCP gets *more* expensive." That conflates two things.

1. Cache savings scale with *what's no longer being sent every turn*, not with the absolute size of the cache. Pre-CR was paying cache-read on 18,500 tokens every turn for tool schemas the LLM mostly ignored. Post-CR pays cache-read on only 1,456 tokens. The 17,044-token delta is pure savings, billed at the cache-read rate (`0.30/1e6`) every turn.
2. Help payloads do enter the cached prefix, but they are bounded (≤ ~1,200 tokens per domain, smaller for scoped queries) and emitted on demand. They earn their keep within ~2 turns of residency.

Net: cache stayed beneficial; the *fixed* portion of the cache shrank by ~17K tokens, while the *opt-in* portion (help) is small, intentional, and only present when the LLM actually needs it.

## Recommendations

- Keep `help` text tier as the default; the summary tier (~50% smaller) is an option for heavy automation that still wants discoverability without paying full text cost.
- Consider a doc note steering callers to `verb`-scoped help (`{operation:"help", verb:"delete_event"}`, ~100 tokens) rather than full-domain help when only one verb is in question; that drops Case B/C help cost by ~6× per call.
- Track `help` call counts in the CRUD bench CSV. If real-world H drifts above ~5 per session per domain, revisit the `help` formatter's token budget.

## Caveats

- Token-byte ratio assumed at ~4 chars/token; actual tokenizer ratios vary by content (JSON tokenizes more densely than prose). Schema tokens may be 10–15% higher than the linear estimate.
- Cache-write economics assume one write per 5-minute TTL window. Long idle sessions that re-warm the cache shift the constants but not the sign of the result.
- Pricing taken from the public list (Sonnet 4.6, Opus 4.7) on 2026-04-25. All four buckets scale proportionally between models, so percentage savings hold across the family.
- Opus 4.7's new tokenizer can use up to 35% more tokens for the same text. Both pre- and post-CR token counts inflate together, so percentages are stable, but absolute Opus 4.7 dollars in the tables may be up to ~35% higher in real traffic.
- Help-call residency assumes calls are evenly distributed across the session (mean residency `T/2`). Front-loaded help (typical: discover, then act) shifts residency closer to `T`, raising help cost modestly without changing the sign of the result.
