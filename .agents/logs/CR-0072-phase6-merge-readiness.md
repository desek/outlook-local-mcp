# CR-0072 Phase 6: Merge-readiness record ("Close the loop")

Prepared 2026-07-31 on branch `dev/cr-0071-0072-dependency-currency` at
checkpoint `473d01a` (Phase 5 complete, this record adds Phase 6). CR-0072's
implementation stages (Phases 1-5) all landed on this shared branch after
CR-0071's.

This record is PREPARATION ONLY. Phase 6's terminal actions — squash-merge,
and confirming the site alerts transition to `fixed` — are irreversible human
decisions taken against the default branch and are **not** executed here. The
branch is **not yet pushed**, **no pull request exists**, and the shared PR is
governed by CR-0071. This file records the current state and the exact actions
left for the human so the loop can be closed correctly, and cross-references
the CR-0071 record so there is ONE combined picture for the shared PR.

## 1. Resolved versions in `site/pnpm-lock.yaml` (live at `473d01a`)

The `pnpm` overrides in `site/pnpm-workspace.yaml` (`tmp: ^0.2.7`,
`uuid: ^11.1.1`) collapse every transitive copy to a single patched version.
The lockfile carries **no** residual vulnerable entry — no `tmp@0.0.33`,
`tmp@0.1.0`, `uuid@8.x/9.x/10.x`; grep confirms only the two below resolve.

| Package | Resolved in lockfile | Lines |
|---------|----------------------|-------|
| `tmp`  | `0.2.7` (only version) | 1605, 3324 |
| `uuid` | `11.1.1` (only version) | 1656, 3380 |

## 2. Live Dependabot alert state (read-only, `gh api`, 2026-07-31)

All three site alerts are **open** at record time — expected, because the
branch is unpushed and Dependabot re-evaluates `site/pnpm-lock.yaml` only when
the change reaches the default branch (`main`).

| # | State now | GHSA | Sev | Package | Vulnerable range | Resolved to | Clears? |
|---|-----------|------|-----|---------|------------------|-------------|---------|
| 18 | open | GHSA-52f5-9888-hmc6 | low    | `tmp`  | `<= 0.2.3`  | `0.2.7` | Yes — `0.2.7 > 0.2.3` |
| 19 | open | GHSA-w5hq-g745-h8pq | medium | `uuid` | `< 11.1.1`  | `11.1.1` | Yes — `11.1.1 >= 11.1.1`, and not in the `>=12.0.0 <12.0.1` / `>=13.0.0 <13.0.1` sub-ranges |
| 20 | open | GHSA-ph9p-34f9-6g65 | high   | `tmp`  | `< 0.2.6`   | `0.2.7` | Yes — `0.2.7 >= 0.2.6` |

Both `tmp` alerts (#18 low, #20 high) are cleared by the single `tmp@0.2.7`
resolution, which is at or above both patched floors (`0.2.4` and `0.2.6`).
`uuid@11.1.1` is exactly the first patched version for #19 and sits outside all
three vulnerable sub-ranges.

**Expected transition:** all three should **auto-close to `fixed`** once the
change reaches `main`. Per **AC-10** and **FR-14**, none of the three is to be
closed by **dismissal** — the fix is a real version change, so dismissal is
neither needed nor permitted here.

## 3. Merge instruction (shared branch and PR with CR-0071)

CR-0072 shares its branch and its single pull request with CR-0071. Per
**CR-0072 FR-16** and **CR-0071 FR-17**:

* CR-0072's own commits carry `chore(site):`-style subjects (Phases 1-5:
  `checkpoint(CR-0072): phase N: ...` plus one `docs(CR-0072):` correction),
  because in isolation CR-0072 does not affect the distributed binary and
  `release-please` should produce no release for it alone.
* **The squash-merge title of the combined PR is CR-0071's `fix(deps):`**, not
  `chore(...)`. The combined PR **does** affect the distributed binary via
  CR-0071's Go module bumps and toolchain move, so `fix(deps):` governs and
  yields the intended **0.5.0 -> 0.5.1** patch release. CR-0072's site/CI
  changes ride along under that title.

Suggested combined title (from the CR-0071 record):
`fix(deps): bring the Go module graph and toolchain current (CR-0071, CR-0072)`

## 4. Deliberately deferred item

`vite` `8.1.5 -> 8.2.0` is **deliberately NOT in this CR**. It is a build-only
minor that appeared after CR-0072 was authored; because `vite` is the core
bundler a minor can alter build output, rendering, and the Lighthouse verdict,
so folding it in would require the same screenshot and Lighthouse verification
the runtime bumps received. It is left to the **new `npm` Dependabot entry this
CR adds** to `.github/dependabot.yml`, which will raise it as its own PR with
its own verification. Recorded as UNRESOLVED in the CR's Current State; not in
the Proposed Change or Requirements.

## 5. Combined outstanding actions for the human (BOTH CRs, one shared PR)

Ordered. Items 1-2 and 6-8 come from
`.agents/logs/CR-0071-phase8-merge-readiness.md`; items 3-5 are CR-0072's.

1. **Merge release PR #28** (`chore(main): release 0.5.0`) to `main` **first**,
   so this combined work lands as a clean 0.5.1. [CR-0071]
2. **Push the branch** `dev/cr-0071-0072-dependency-currency` and open a
   **single** pull request for the combined CR-0071 + CR-0072 change against
   `main`. (Both CRs' commits are already on the branch.) [CR-0071 + CR-0072]
3. **Squash-merge with a `fix(deps):` title** — NOT `chore(...)`. Expected
   release: **0.5.0 -> 0.5.1**. [CR-0071 FR-17 governs; CR-0072 FR-16 defers]
4. **Confirm the three site alerts (#18, #19, #20) transition to `fixed`**
   after the merge reaches `main`, and confirm **none was dismissed** (AC-10):
   ```
   gh api repos/desek/outlook-local-mcp/dependabot/alerts --paginate \
     -q '.[] | select(.dependency.manifest_path=="site/pnpm-lock.yaml") | {number,state}'
   ```
   Expect #18/#19/#20 all `state: fixed`. Do NOT dismiss any of them. [CR-0072]
5. **Confirm all sixteen go.mod alerts (#2-#17) transition to `fixed`** after
   the merge reaches `main` (AC-9); expect empty output for the open query in
   the CR-0071 record. No go.mod alert should need a written dismissal. [CR-0071]
6. **Close PR #23** (the deadlocked Dependabot `kiota-http-go 1.5.4->1.5.5` PR)
   **as superseded** with a comment linking this PR. Do not merge it — the
   branch already carries `kiota-http-go v1.5.6`. [CR-0071]
7. **Verify the release**: `release-please` should open/update a release PR
   bumping to **0.5.1**; merging it produces the patch release users take. [CR-0071]
8. **OUTSTANDING — run `make crud-test` manually against a test tenant.** This
   is the runtime behavioural check for the mcp-go `v0.45.0 -> v0.57.0`
   migration (CR-0071 Phases 5-6, Risk 1). It was **not** run in the
   implementation environment and is **not** in any CI workflow: it needs live
   Microsoft 365 credentials plus an authenticated `claude` CLI and performs
   real create/update/delete against a live mailbox and calendar. The static
   substitute (`TestVerbInventoryUnchangedAfterUpgrade` + unchanged
   `extension/manifest.json`) proves the MCP surface is identical but does NOT
   exercise runtime behaviour through mcp-go v0.57.0. Run it against a
   disposable test mailbox before relying on the release and record the run
   under `.agents/logs/`. [CR-0071 — still open]

## 6. Constraints honoured by this phase

No branch push, no PR create/merge/close, no alert dismissal, no merge to
`main` was performed. All GitHub reads were via `gh api`. The items above are
recorded for the human to execute.

## Cross-reference

Combined picture spans two records for the one shared PR:

* `.agents/logs/CR-0071-phase8-merge-readiness.md` — Go module alerts (#2-#17),
  PR #23/#28 sequencing, `make crud-test` verification (still outstanding).
* `.agents/logs/CR-0072-phase6-merge-readiness.md` — this file: site alerts
  (#18-#20), the `fix(deps):` merge-title coupling, and the deferred `vite` bump.
