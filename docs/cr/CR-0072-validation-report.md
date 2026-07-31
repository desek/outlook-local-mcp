# CR-0072 Validation Report

/ Validator: cr-validator (documentation-only). Source code not modified.
/ Branch: `dev/cr-0071-0072-dependency-currency` (SHARED with CR-0071, completed).
/ Finalization commit: `a943a14`. Scope of assessment: CR-0072's COMMITS ONLY
/ (`e986ff0..a943a14`), per the reconciled NFR-3, not the branch diff.

## Summary

Requirements: 18/20 | Acceptance Criteria: 10/13 | Tests: 5/5 | Gaps: 1

- FAIL: 0
- PARTIAL: 5 (FR-14, FR-16, AC-7, AC-10, AC-12) — of which FR-14 and AC-10 are
  deferred-to-merge and inherently unverifiable pre-merge (branch unpushed by design).
- GAP: 1 (Change Summary self-inconsistency — a CR-prose defect, not an implementation defect).

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
| FR-14 | Alerts 18/19/20 in state `fixed` after merge | PARTIAL | DEFERRED-TO-MERGE, unverifiable pre-merge. Branch unpushed by design; Dependabot re-evaluates only on `main`. The *cause* of the fix is verified effective (FR-5: only patched tmp/uuid resolve). Phase 6 log documents expected auto-close to `fixed`, none dismissed. State transition itself is unobservable now |
| FR-15 | Each override records its removal condition | PASS | Removal conditions committed as comments directly above the `overrides` block in `site/pnpm-workspace.yaml` (durable, greppable — stronger than a PR comment). Both `tmp` and `uuid` conditions stated with a checkable `pnpm why` procedure |
| FR-16 | Commits carry `chore(site):` subjects | PARTIAL | Release-safety INTENT met (no commit triggers an independent release; squash-title `fix(deps):` governs per CR-0071). But the LITERAL prefix is absent: commits carry `checkpoint(CR-0072): ...` and one `docs(CR-0072): ...`, not `chore(site):`. See Gaps |
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
| AC-7 | Gate catches a runtime dependency becoming vulnerable | PARTIAL | Mechanism present and correct (audit --prod, moderate floor, before build, no continue-on-error). But the FAILURE PATH is unexercised: no test injects a vulnerable dependency into `dependencies` and observes the job fail. Design-verified, not behaviourally tested. The CR Test Strategy does not enumerate an injection step |
| AC-8 | Currency and overrides separately bisectable, each passes gate | PASS | Separate commits 21632f9 / 64b085c (NFR-4). Phase 2 commit body records its own passing build + Lighthouse + 135/135 visual; Phase 3 log records the same |
| AC-9 | Dependabot covers the site | PASS | npm on /site added AFTER github-actions (append point honoured); version updates grouped, security ungrouped (FR-9). All three ecosystems present: gomod + github-actions (CR-0071) + npm (CR-0072) |
| AC-10 | Site alerts reach terminal state `fixed`, none dismissed | PARTIAL | DEFERRED-TO-MERGE, unverifiable pre-merge (same basis as FR-14). Phase 6 log documents the expected `fixed` transition and the "no dismissal" rule |
| AC-11 | Dev/prod boundary documented and bounded | PASS | site-quality.md covers --prod scope, exclusion rationale, and the dependencies-crossing rule (FR-12 + FR-13) |
| AC-12 | This CR's commits touch nothing outside the site | PARTIAL | (a) subject-prefix clause: commits are `checkpoint(CR-0072):`/`docs(CR-0072):`, not `chore(site):` — see FR-16. (b) file-scope: no internal/, cmd/, go.mod, go.sum, or embedded docs touched (NFR-3 clean); the CR's own `docs/cr/CR-0072-*.md` was edited (governance-intrinsic) — a literal deviation from "docs/ other than site-quality.md" but the spirit (no server code/embedded docs) holds. (c) squash-title `fix(deps):` deferred-to-merge |
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

1. **CR Change Summary is self-inconsistent (documentation GAP, not implementation).**
   Paragraph 2, line 30: "it brings the **four** outdated packages current" contradicts
   the corrected paragraph 1 (line 21: "brings the runtime pair and `@vitejs/plugin-react`
   current" = three) and the Current State table (five packages: three brought current,
   `puppeteer-core` retained, `vite` deferred). The reviewer corrected the first paragraph
   and the table but left the "four ... current" claim stale. *Suggested minimal fix:* in
   line 30 change "four outdated packages current" to "three outdated packages current"
   (gsap, lenis, @vitejs/plugin-react), consistent with the rest of the corrected CR.
   The IMPLEMENTATION is correct; only the CR prose drifts.

2. **AC-7 failure path is unexercised (PARTIAL, not a blocking defect).** The audit gate's
   *negative* behaviour — job fails when a moderate-or-higher vulnerable package enters
   `dependencies` — is asserted by construction but never demonstrated. No test injects a
   vulnerable runtime dependency and observes the failure, and the Test Strategy does not
   enumerate such a manual step. This is inherent to a "gate that is green today" and is
   acceptable, but it should not be read as behaviourally proven. *Suggested minimal
   step (optional):* record a one-off local run adding a known-vulnerable package to
   `dependencies` and confirming the audit step exits nonzero, under `.agents/logs/`.

### Deferred-to-merge (NOT defects — inherent to an unpushed branch)

- **FR-14 / AC-10** (alerts 18/19/20 → `fixed`, none dismissed): the branch is unpushed by
  design and Dependabot only re-evaluates `site/pnpm-lock.yaml` on `main`. The remediation
  is verified *effective* (FR-5), so the terminal transition is expected but cannot be
  observed pre-merge. Confirm post-merge per the Phase 6 log query.
- **FR-16 / AC-12 squash-title and subject prefix**: individual commits use the repo's
  standard `checkpoint(CR-XXXX):` governance subjects rather than `chore(site):`; neither
  `checkpoint` nor `docs` triggers a `release-please` release, so the requirement's stated
  *purpose* (no independent release) holds. The governing `fix(deps):` squash-title is a
  merge-time action.

### Independent instrument checks performed

- `pnpm --dir site audit --prod --audit-level=moderate` re-run today → "No known
  vulnerabilities found" (confirms the CR's central claim and that the overrides removed
  the tmp/uuid findings; a separate, unrelated build-time-only advisory GHSA-mh99-v99m-4gvg
  now appears in the FULL tree — out of scope, does not reach `--prod`).
- Lockfile greps for tmp/uuid re-run independently — single patched version each, no residue.
- Screenshot tile counts (135/135) and raw visual-diff log inspected on disk — the
  screenshot-comparison Quality-Standards box is backed by a real diff run, not a bare tick.
