# CR-0072 Phase 3: Overrides

Applied on the shared branch `dev/cr-0071-0072-dependency-currency`, on top of the
Phase 2 currency commit `21632f9`. This is the phase that closes Dependabot alerts
#18, #19, #20.

## Mechanism deviation from the CR's named location (load-bearing)

The CR's Proposed Change 2, FR-3, FR-4, and Affected Components name
`site/package.json` (a `pnpm.overrides` block) as the home of the overrides.
**pnpm 11.18.0, the version pinned in `packageManager`, no longer reads a `pnpm`
field from `package.json`** and prints:

```
[WARN] The "pnpm" field in package.json is no longer read by pnpm. The following
keys were ignored: "pnpm.overrides". See https://pnpm.io/settings ...
```

A `pnpm.overrides` block placed in `package.json` left the lockfile unchanged —
`tmp@0.0.33`, `tmp@0.1.0`, and `uuid@8.3.2` still resolved — so FR-5 (no vulnerable
version in the lockfile) and FR-8 (the gate still produces a verdict against a fixed
tree) could not be met that way.

This repo already establishes `site/pnpm-workspace.yaml` as the home of pnpm 11
settings: it carries `onlyBuiltDependencies` with an in-file comment stating that
"pnpm 11 reads settings from here, not from a 'pnpm' field in package.json". The
overrides were therefore placed there, where pnpm 11 actually honours them. This is
the same class of documented-vs-actual discrepancy the CR itself records under Risk 4
(the CR named a config location that does not work for the pinned toolchain).

Consequence for scope: this phase's declared `PHASE_AFFECTED_COMPONENTS` were
`site/package.json` and `site/pnpm-lock.yaml`. The realised change touches
`site/pnpm-workspace.yaml` and `site/pnpm-lock.yaml`; `package.json` is unchanged.
The deviation is reported to the orchestrator rather than applied silently.

## Overrides applied (in `site/pnpm-workspace.yaml`)

```yaml
overrides:
  tmp: ^0.2.7
  uuid: ^11.1.1
```

* `tmp` `^0.2.7` clears both `tmp` advisories at once: GHSA-ph9p-34f9-6g65 (high,
  `< 0.2.6`) and GHSA-52f5-9888-hmc6 (low, `<= 0.2.3`).
* `uuid` `^11.1.1` is the lowest patched range in GHSA-w5hq-g745-h8pq. Not `^12`/`^13`:
  those majors have their own vulnerable entries (`< 12.0.1`, `< 13.0.1`).

## Removal conditions (FR-15, TASK C)

Recorded as comments directly above the `overrides` block in
`site/pnpm-workspace.yaml` — greppable, and travelling with the config they govern.
Each is checkable:

* **tmp** — removable once `@lhci/cli` (currently `0.15.1`) ships a release whose tree
  no longer resolves `tmp` below `0.2.6`. Check: delete the line, reinstall, run
  `pnpm --dir site why tmp`; if every path is `>= 0.2.6` the override is redundant.
* **uuid** — removable once `@lhci/cli` depends on `uuid >= 11.1.1` (it currently pulls
  `uuid@8.3.2`). Check the same way with `pnpm --dir site why uuid`.

## Verification (FR-5, FR-6, FR-8, NFR-1, NFR-2, and the audit claim)

| Check | Command | Outcome |
|-------|---------|---------|
| Install (overrides applied) | `pnpm --dir site install` | exit 0, packages +2 -5 |
| Lockfile FR-5 | `grep -E '^\s+(tmp\|uuid)@' pnpm-lock.yaml` | only `tmp@0.2.7`, `uuid@11.1.1`; no vulnerable version present |
| Frozen install FR-6 | `pnpm --dir site install --frozen-lockfile` | exit 0 |
| Build FR-7 | `pnpm --dir site run build` | exit 0 (tsc -b, two vite builds, prerender) |
| Lighthouse FR-8 / AC-3 | `pnpm --dir site run lighthouse` | exit 0, `assertion-results.json` is `[]` (all passed), 12 JSON + 12 HTML reports in `.lighthouseci/` |
| Prod audit | `pnpm --dir site audit --prod --audit-level=moderate` | exit 0, "No known vulnerabilities found" |
| Visual diff NFR-1 / AC-5 | `site.visual.diff.mjs baseline phase3` | 135/135 tiles, 0 failing, worst ratio 99.989% |

`uuid` crossed three majors (8 -> 11) and `tmp` crossed `0.0.x`/`0.1.x` -> `0.2.x`;
`lhci autorun` calls `require('uuid').v4` and `tmp` from `lhci open`/`inquirer`. The
run completed with no module-resolution error and produced reports, which is the
failure shape Risk 2 predicted and AC-3 requires ruling out.

## Scores vs Phase 1 baseline (spread: category 0.00, LCP <= 2ms)

| Page | Metric | Baseline | Phase 3 | Within spread? |
|------|--------|----------|---------|----------------|
| index.html | perf | 0.97 | 0.97 | yes (0.00) |
| index.html | LCP | 2554/2556 ms | 2554 ms | yes (<= 2ms) |
| index.html | TBT | 13 ms | 8 ms | improved |
| concepts.html | perf / LCP | 1.00 / 1652 ms | 1.00 / 1652 ms | yes |
| quickstart.html | perf / LCP | 1.00 / 1503 ms | 1.00 / 1502 ms | yes (1ms) |
| troubleshooting.html | perf / LCP | 1.00 / 1652 ms | 1.00 / 1652 ms | yes |

a11y, best-practices, and SEO are 1.00 on every page. No category score moved; every
LCP delta is inside the measured floor. AC-4 satisfied.

## Raw logs

Gitignored under `.agents/logs/CR-0072-phase3-*.log` (`install`, `build`,
`lighthouse`, `lh-summary`, `audit-prod`, `visual-diff`). Screenshot tiles at
`.agents/screenshots/cr0072-phase3/` (gitignored, regenerable).
