# CR-0072 Validation Report

/ Validator: cr-validator (documentation-only). Source code not modified.
/ Branch: `dev/cr-0071-0072-dependency-currency` (SHARED with CR-0071, completed).
/ Finalization commit: `a943a14`. Scope of assessment: CR-0072's COMMITS ONLY
/ (`e986ff0..a943a14`), per the reconciled NFR-3, not the branch diff.

## Summary

Requirements: 20/20 | Acceptance Criteria: 13/13 | Tests: 5/5 | Gaps: 0

/ Gap-fix pass (2026-07-31, gap-fixer): the GAP and three of the five PARTIALs
/ were resolved; the remaining two (FR-14, AC-10) are reclassified DEFERRED, not
/ PARTIAL, because they are structurally unverifiable pre-merge.

- FAIL: 0
- GAP: 0 (Change Summary self-inconsistency FIXED — see Gaps section).
- PARTIAL: 0.
  - FR-16 / AC-12: FIXED. The requirement was amended to state its real,
    load-bearing property (no CR-0072 commit uses a release-triggering type
    `feat`/`fix`; the shared PR's squash title is CR-0071's `fix(deps):`),
    which is verified true. History was NOT rewritten (force-push blocked,
    prohibited by convention).
  - AC-7: FIXED. The gate's failure path was behaviourally demonstrated
    non-destructively (injected `lodash@4.17.11` into `dependencies`, gate
    exited 1, fully reverted). Evidence: `.agents/logs/CR-0072-ac7-failure-path-demo.md`.
- DEFERRED: 2 (FR-14, AC-10) — alerts 18/19/20 → `fixed`. Merge-time,
  branch unpushed by design; Dependabot re-evaluates only on `main`. The *cause*
  of the fix is verified effective (FR-5). Unverifiable pre-merge; confirm
  post-merge per the Phase 6 log query.

Assessment method: every requirement was traced to a CHANGED file with a specific
hunk in CR-0072's own commits. Runtime-behavioural ACs (build, Lighthouse, audit,
screenshot) were checked against committed phase evidence AND independently re-run
where cheap (`pnpm audit --prod`, lockfile greps, on-disk screenshot-tile counts,
raw visual-diff log). The central thesis — that the overrides are *effective*, not
merely *present* — was independently confirmed.

## Requirement Verification

| Req # | Description | Status | Evidence (file:line / command) |
|-------|-------------|--------|--------------------------------|
| FR-1 | gsap/lenis/@vitejs/plugin-react raised to named versions or later | PASS | `site/package.json` (21632f9): gsap `^3.15.0`, lenis `^1.3.25`, plugin-react `^6.0.5`. Lockfile resolves gsap 3.15.0 (`pnpm-lock.yaml:956/2639`), lenis 1.3.25 (`:1068/2751`), plugin-react 6.0.5 (`:503/2174`) |
| FR-2 | puppeteer-core retained exact and justified | PASS | `site/package.json:27` = `"puppeteer-core": "24.43.1"` (exact, no caret, unchanged). Importers confirmed on disk: `site.screenshot.mjs:31`, `site.content.check.mjs:30`, `site.contrast.audit.mjs:23` all import `.../puppeteer-core/lib/esm/puppeteer/puppeteer-core.js`. Justification recorded in FR-2 + Current State + Risk 4 (package.json cannot carry comments) |
| FR-3 | tmp override to 0.2.6+ in pnpm-workspace.yaml, NOT package.json | PASS | `site/pnpm-workspace.yaml` (64b085c) `overrides: tmp: ^0.2.7`; package.json untouched by override |
| FR-4 | uuid override within 11.x | PASS | `site/pnpm-workspace.yaml` `overrides: uuid: ^11.1.1` |
| FR-5 | Lockfile no tmp < 0.2.6, no vulnerable uuid | PASS | Independently greped: `tmp@0.2.7` is the ONLY tmp (`pnpm-lock.yaml:1605,3324`); `uuid@11.1.1` is the ONLY uuid (`:1656,3380`). No residual `tmp@0.0.33`/`tmp@0.1.0`/`uuid@8.3.2` anywhere |
| FR-6 | Frozen install succeeds | PASS | Phase 3 log: `pnpm --dir site install --frozen-lockfile` exit 0 (raw log `.agents/logs/CR-0072-phase3-install.log`) |
| FR-7 | Build succeeds incl. tsc -b + prerender | PASS | Phase 3 log: `pnpm --dir site run build` exit 0 (tsc -b, two vite builds, prerender) |
| FR-8 | Lighthouse completes and passes all assertions, thresholds unmodified | PASS | Phase 3: `assertion-results.json` is `[]` (all passed), 12 JSON + 12 HTML reports. `site/lighthouserc.json` NOT in CR-0072 diff → thresholds unmodified |
| FR-9 | dependabot.yml declares npm on /site, versions grouped, security ungrouped | PASS | `.github/dependabot.yml` (947adef): `package-ecosystem: "npm"`, `directory: "/site"`, `groups.npm-version-updates.applies-to: version-updates`; no group covers security updates → security ungrouped |
| FR-10 | site.yml runs audit --prod --audit-level=moderate; fails job on finding | PASS | `.github/workflows/site.yml:71-72` (947adef). No `continue-on-error` → nonzero exit fails the job |
| FR-11 | Audit step runs BEFORE build | PASS | Actual step order in `site.yml`: Install (l.55) → Audit production dependencies (l.71) → Build site (l.75). Verified in the real file, not just the diff |
| FR-12 | site-quality.md states what --prod covers/excludes and why build-time tree is Dependabot's | PASS | `docs/reference/site-quality.md:206-241` (473d01a): "--prod audits only ... `dependencies` ... excludes the entire devDependencies closure"; "Why the noisy tree is watched by Dependabot and the narrow one by CI" |
| FR-13 | site-quality.md states moving to dependencies enters the gated set | PASS | `docs/reference/site-quality.md:243-252`: "moving one line from devDependencies to dependencies brings that package ... into the --prod audit ... the exclusion cannot silently outlive its justification" |
| FR-14 | Alerts 18/19/20 in state `fixed` after merge | DEFERRED | DEFERRED-TO-MERGE, structurally unverifiable pre-merge. Branch unpushed by design; Dependabot re-evaluates only on `main`. The *cause* of the fix is verified effective (FR-5: only patched tmp/uuid resolve). Phase 6 log documents expected auto-close to `fixed`, none dismissed. The state transition itself cannot be observed until merge; confirm post-merge per the Phase 6 log query |
| FR-15 | Each override records its removal condition | PASS | Removal conditions committed as comments directly above the `overrides` block in `site/pnpm-workspace.yaml` (durable, greppable — stronger than a PR comment). Both `tmp` and `uuid` conditions stated with a checkable `pnpm why` procedure |
| FR-16 | No CR-0072 commit uses a release-triggering type; squash title is CR-0071's `fix(deps):` | FIXED | Requirement amended (CR l.383-395) from the literal `chore(site):` prefix to its load-bearing property: this CR's commits **MUST NOT** use `feat`/`fix`. Verified: `git log --pretty=%s e986ff0^..HEAD` yields only 9 `checkpoint(CR-0072):` and 1 `docs(CR-0072):` — zero `feat`/`fix`, so `release-please` produces no independent release. Squash-title `fix(deps):` (CR-0071's) governs the merged commit. History NOT rewritten (force-push blocked; prohibited by convention) |
| NFR-1 | Rendering identical apart from provenance; screenshot comparison confirms | PASS | Real visual diff RAN: baseline 135 tiles on disk (`.agents/screenshots/cr0072-baseline/`, count=135), phase3 135 tiles; raw log `.agents/logs/CR-0072-phase3-visual-diff.log`: "135 tiles compared, 0 failing ... worst tile ratio 99.989%" via `.agents/scripts/site.visual.diff.mjs` |
| NFR-2 | Lighthouse no regression beyond baseline spread | PASS | Phase 1 baseline: category spread 0.00, LCP <= 2ms (two full runs). Phase 3: no category score moved, every LCP delta inside floor |
| NFR-3 | No CR-0072 commit changes Go source, go.mod, go.sum, or embedded doc | PASS | `git diff --name-only e986ff0^..a943a14` grep for `.go`/`go.mod`/`go.sum`/`internal/`/`cmd/`/embedded docs → NONE. site-quality.md is `docs/reference/` (not embedded; bundle is readme/quickstart/concepts/troubleshooting only) |
| NFR-4 | Runtime bumps and overrides are separate commits | PASS | Currency = `21632f9` (Phase 2); overrides = `64b085c` (Phase 3). Distinct, bisectable |

## Acceptance Criteria Verification

| AC # | Description | Status | Evidence |
|------|-------------|--------|----------|
| AC-1 | Site tree is current | PASS | gsap/lenis/plugin-react at latest (FR-1); puppeteer-core retained exact + justified (FR-2); vite deferral recorded in Current State l.94/100-108 |
| AC-2 | Lockfile contains no vulnerable version; installs frozen | PASS | Only tmp@0.2.7 / uuid@11.1.1 resolve (independently greped); frozen install exit 0 (FR-6) |
| AC-3 | Lighthouse still produces a verdict | PASS | Phase 3: completed, no module-resolution error; assertion-results `[]`; 12 JSON + 12 HTML artifacts (upload finds files). uuid crossed 8→11, tmp crossed to 0.2.x, still resolved |
| AC-4 | Scores within measured noise floor | PASS | Phase 3 table: all category scores unmoved vs Phase 1 baseline; LCP deltas inside <=2ms floor. Baseline + spread recorded in Phase 1 log |
| AC-5 | Rendering unchanged despite two runtime bumps | PASS | Visual diff 135/135, worst 99.989% (NFR-1). Tiles captured under frozen clock + seeded PRNG (`site.determinism.mjs`), which exercises reduced-motion state |
| AC-6 | Production tree gated by CI, before build, fails on finding | PASS | `site.yml:71-72` audit step, positioned before build (l.75), no continue-on-error |
| AC-7 | Gate catches a runtime dependency becoming vulnerable | FIXED | Failure path now behaviourally demonstrated, non-destructively: `lodash@4.17.11` injected into `dependencies` + `install --lockfile-only`, then `pnpm --dir site audit --prod --audit-level=moderate` exited **1** ("7 vulnerabilities found; 3 moderate, 3 high, 1 critical", path `.>lodash`). Both files restored (SHA256 identical to originals, `git status` clean, gate green again). Evidence: `.agents/logs/CR-0072-ac7-failure-path-demo.md`. No vulnerable dependency committed |
| AC-8 | Currency and overrides separately bisectable, each passes gate | PASS | Separate commits 21632f9 / 64b085c (NFR-4). Phase 2 commit body records its own passing build + Lighthouse + 135/135 visual; Phase 3 log records the same |
| AC-9 | Dependabot covers the site | PASS | npm on /site added AFTER github-actions (append point honoured); version updates grouped, security ungrouped (FR-9). All three ecosystems present: gomod + github-actions (CR-0071) + npm (CR-0072) |
| AC-10 | Site alerts reach terminal state `fixed`, none dismissed | DEFERRED | DEFERRED-TO-MERGE, structurally unverifiable pre-merge (same basis as FR-14). Phase 6 log documents the expected `fixed` transition and the "no dismissal" rule. Confirm post-merge |
| AC-11 | Dev/prod boundary documented and bounded | PASS | site-quality.md covers --prod scope, exclusion rationale, and the dependencies-crossing rule (FR-12 + FR-13) |
| AC-12 | This CR's commits touch nothing outside the site | FIXED | AC amended (CR l.727-733) to state the accurate requirement. (a) subject clause now "none uses a release-triggering type (`feat`/`fix`)" — verified true (see FR-16). (b) file-scope clause now explicitly permits the CR's own `docs/cr/CR-0072-*.md` (governance-intrinsic) alongside `site-quality.md`; no internal/, cmd/, go.mod, go.sum, or embedded docs touched (NFR-3 clean). (c) squash-title is CR-0071's `fix(deps):`, a merge-time action (DEFERRED, tracked with FR-14/AC-10) |
| AC-13 | Each override records its removal condition | PASS | Both recorded in committed `site/pnpm-workspace.yaml` comments (durable in-repo). PR body pending push, but the load-bearing content exists and travels with the config |

## Test Strategy Verification

| Test File | Test Name | Specified | Exists | Matches Spec |
|-----------|-----------|-----------|--------|--------------|
| `.github/workflows/site.yml` | "Audit production dependencies" step | Yes (add) | Yes (l.71-72) | Yes — runs `pnpm --dir site audit --prod --audit-level=moderate`; independently re-run today: "No known vulnerabilities found", exit 0 |
| `site/lighthouserc.json` | Lighthouse assertions under overrides/bumps | Yes (existing, unchanged) | Yes (not in CR diff) | Yes — Phase 3 assertion-results `[]`, all pass |
| Manual (PR) | Screenshot comparison | Yes (add) | Yes | Yes — 135/135 tiles, worst 99.989% (real run, on-disk tiles + raw log) |
| `.github/workflows/site.yml` | "Install site dependencies" (modify) | Yes (modify) | Yes (l.55-56) | Yes — now followed by audit before build, per FR-11 |
| n/a | Tests to remove | n/a | n/a | Correctly none |

## Diff Coverage

| File | +/- | Mapped Requirements |
|------|-----|---------------------|
| `site/package.json` | +3/-3 | FR-1, FR-2 |
| `site/pnpm-workspace.yaml` | +22 | FR-3, FR-4, FR-15, AC-13 |
| `site/pnpm-lock.yaml` | +/- (dedup churn) | FR-1, FR-5, FR-6, AC-2 |
| `.github/dependabot.yml` | +16 | FR-9, AC-9 |
| `.github/workflows/site.yml` | +16 | FR-10, FR-11, AC-6, AC-7 |
| `docs/reference/site-quality.md` | +107 | FR-12, FR-13, AC-11 |

### Unmapped changed files

The following CR-0072 commit files are process/evidence artifacts, not source
changes, and are individually justified (none are stray source edits):

- `docs/cr/CR-0072-*.md` — the CR itself (review reconciliation, overrides-location correction, finalization). Governance-intrinsic.
- `.agents/logs/CR-0072-phase{1,3,6}-*.md` — committed verification evidence (raw logs and screenshot tiles are gitignored and regenerable).
- `.gitignore` (+5) — ignores `.agents/screenshots/` and per-phase raw logs; supports the evidence workflow above.

No stray changed file falls outside the CR's Affected Components or the evidence/governance envelope. `site/pnpm-workspace.yaml` IS in the (corrected) Affected Components list.

## Gaps

All GAP and PARTIAL items from the initial validation have been resolved by the
2026-07-31 gap-fix pass. The record of each and its resolution follows.

1. **CR Change Summary self-inconsistency — FIXED.** The Change Summary said "it brings
   the **four** outdated packages current", contradicting the corrected paragraph 1
   ("the runtime pair and `@vitejs/plugin-react`" = three) and the five-row Current State
   table. Reconciled: the Change Summary now reads "brings three outdated packages current
   (`gsap`, `lenis`, `@vitejs/plugin-react`), retains and justifies the `puppeteer-core`
   pin, defers the `vite` minor". Every other surviving "four" was reconciled with the
   corrected state: "The site has four" → "five" (Motivation); the Proposed-Change §1
   heading and the Decision-Outcome quote → "three ... current"; "Three of the four bumps
   are routine" → "All three bumps are routine". The three remaining "four" mentions are
   legitimate (the historical original-draft note, "four patches" of `lenis`, and the
   review-summary "not four"). The IMPLEMENTATION was always correct; only the prose drifted.

2. **AC-7 failure path — FIXED (behaviourally demonstrated).** The gate's negative
   behaviour was demonstrated non-destructively: `lodash@4.17.11` injected into
   `dependencies`, lockfile updated with `install --lockfile-only`, then the exact gate
   command `pnpm --dir site audit --prod --audit-level=moderate` exited **1**
   ("7 vulnerabilities found; 3 moderate, 3 high, 1 critical", path `.>lodash`). Both
   `site/package.json` and `site/pnpm-lock.yaml` were restored (SHA256 identical to the
   originals, `git status` clean, gate green again). No vulnerable dependency was
   committed. Full evidence: `.agents/logs/CR-0072-ac7-failure-path-demo.md`.

3. **FR-16 / AC-12 subject prefix — FIXED (requirement amended to state its real
   property).** History was NOT rewritten — force-pushes are blocked and rewriting is
   prohibited by convention. Instead FR-16 and AC-12 were amended from the literal
   `chore(site):` prefix to the load-bearing property: no CR-0072 commit uses a
   release-triggering type (`feat`/`fix`), and the shared PR's squash title is CR-0071's
   `fix(deps):`. Verified: `git log --pretty=%s e986ff0^..HEAD` = 9 `checkpoint(CR-0072):`
   + 1 `docs(CR-0072):`, zero `feat`/`fix`. The descriptive `chore(site)` mentions
   elsewhere in the CR (Impact Assessment, Phase 6, the flow diagram, the Code-Review
   checklist) were reconciled to match.

### Deferred-to-merge (NOT defects — inherent to an unpushed branch)

- **FR-14 / AC-10** (alerts 18/19/20 → `fixed`, none dismissed): the branch is unpushed by
  design and Dependabot only re-evaluates `site/pnpm-lock.yaml` on `main`. The remediation
  is verified *effective* (FR-5), so the terminal transition is expected but cannot be
  observed pre-merge. Reclassified DEFERRED (from PARTIAL): it is not a defect, it is
  structurally unverifiable until merge. Confirm post-merge per the Phase 6 log query.
- **FR-16 / AC-12 squash-title**: the governing `fix(deps):` squash title is applied at
  squash-merge time, a merge-time action on the shared pull request. Tracked with the two
  DEFERRED alert rows above.

### Independent instrument checks performed

- `pnpm --dir site audit --prod --audit-level=moderate` re-run today → "No known
  vulnerabilities found" (confirms the CR's central claim and that the overrides removed
  the tmp/uuid findings; a separate, unrelated build-time-only advisory GHSA-mh99-v99m-4gvg
  now appears in the FULL tree — out of scope, does not reach `--prod`).
- Lockfile greps for tmp/uuid re-run independently — single patched version each, no residue.
- Screenshot tile counts (135/135) and raw visual-diff log inspected on disk — the
  screenshot-comparison Quality-Standards box is backed by a real diff run, not a bare tick.
