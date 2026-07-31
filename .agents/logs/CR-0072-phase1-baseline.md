# CR-0072 Phase 1: Baseline the gate

Baseline captured on the shared branch `dev/cr-0071-0072-dependency-currency` at
commit `e986ff0`, before any site dependency change. CR-0071 is already
implemented on this branch; it touched no file under `site/`, so this baseline
describes the site exactly as CR-0070 left it.

## What ran

| Step | Command | Outcome |
|------|---------|---------|
| Install | `pnpm --dir site install --frozen-lockfile` | exit 0 |
| Build | `pnpm --dir site run build` | exit 0, both runs |
| Lighthouse run 1 | `pnpm --dir site run lighthouse` | exit 0, 4 URLs x 3 runs, all assertions passed |
| Lighthouse run 2 | `pnpm --dir site run lighthouse` | exit 0, 4 URLs x 3 runs, all assertions passed |
| Production audit | `pnpm --dir site audit --prod --audit-level=moderate` | exit 0, "No known vulnerabilities found" |
| Baseline tiles | `node .agents/scripts/site.screenshot.mjs` | exit 0, 135 tiles |

A first attempt at run 2 was interrupted mid-flight and produced no assertion
output. It was discarded and re-run in full rather than being reported as an
environment limitation; Lighthouse works in this environment, as run 1 had
already demonstrated. The numbers below are from two complete runs.

## Lighthouse scores, both runs

Each figure is `lhci`'s median over 3 Lighthouse runs per URL.

| Page | Metric | Run 1 | Run 2 | Spread |
|------|--------|-------|-------|--------|
| `index.html` | perf | 0.97 | 0.97 | 0.00 |
| | a11y / bp / seo | 1.00 | 1.00 | 0.00 |
| | LCP | 2554 ms | 2556 ms | 2 ms |
| | CLS | 0.000 | 0.000 | 0.000 |
| | TBT | 13 ms | 13 ms | 0 ms |
| `concepts.html` | perf | 1.00 | 1.00 | 0.00 |
| | LCP | 1652 ms | 1652 ms | 0 ms |
| `quickstart.html` | perf | 1.00 | 1.00 | 0.00 |
| | LCP | 1503 ms | 1502 ms | 1 ms |
| `troubleshooting.html` | perf | 1.00 | 1.00 | 0.00 |
| | LCP | 1652 ms | 1652 ms | 0 ms |

a11y, best-practices, and SEO are 1.00 on every page in both runs. CLS is 0.000
and TBT is 0 ms on every page except `index.html`.

## The measured noise floor

* **Category scores: spread 0.00.** No category score moved between runs on any
  page. Any category-score change after a dependency bump is therefore a signal,
  not noise.
* **LCP: spread <= 2 ms** (2 ms on `index.html`, 1 ms on `quickstart.html`, 0 ms
  elsewhere).
* **CLS: spread 0.000. TBT: spread 0 ms.**

**Caveat on how tight this is.** Two runs of a median-of-3 is a coarse estimate
of a noise floor, and most of the apparent stability comes from `lhci`'s own
median smoothing rather than from low per-run variance. Read the spread as "the
gate reproduces itself between invocations", which is what AC-4 needs, not as
"a single Lighthouse run varies by under 2 ms". The site's own quality
reference makes the sharper point for CI: per
`docs/reference/site-quality.md` and `site/AGENTS.md`, a metric that is
*identical* across runs and still fails is a defect, while one that swings is
the environment. Local numbers are not CI numbers, and a CI failure is
attributed from the downloaded report before anything is recalibrated.

`index.html` is held to 96 and the documentation pages to 100; the landing page
measuring 0.97 is inside its committed threshold, not a near-miss to be
tightened.

## Production dependency audit

`pnpm audit --prod --audit-level=moderate` reports **no known vulnerabilities**.
This is the fact CR-0072's central claim rests on and the state the new
`site.yml` gate is intended to hold: the three open Dependabot alerts (#18, #19,
#20) are confined to the build-time tree under `@lhci/cli`.

## Baseline screenshot tiles

135 tiles written to `.agents/screenshots/cr0072-baseline/` by
`.agents/scripts/site.screenshot.mjs`, the harness `site/AGENTS.md` prescribes.
Tiles are captured under the virtual clock and seeded PRNG from
`site.determinism.mjs`, one viewport-sized image per scroll step per page per
acceptance width. They are the comparison basis for Phase 2's rendering check
via `.agents/scripts/site.visual.diff.mjs`.

Tiles are deliberately not committed; `/.agents/screenshots/` is gitignored
because the set is bulky and regenerable. This record is the committed evidence.

## Correction to the CR: `puppeteer-core` is load-bearing

CR-0072's Current State says a search of `site/build/`, `site/src/`, `scripts/`,
`.github/workflows/`, and `site/lighthouserc.json` "finds no importer or
configuration referencing" `puppeteer-core`, and FR-2 offers removal as an
option on that basis.

**That search was incomplete and its conclusion is wrong.** `puppeteer-core` is
imported by three scripts in `.agents/scripts/`, a directory none of the earlier
searches covered:

* `.agents/scripts/site.screenshot.mjs:31`
* `.agents/scripts/site.content.check.mjs:30`
* `.agents/scripts/site.contrast.audit.mjs:23`

All three import a deep internal path,
`../../site/node_modules/puppeteer-core/lib/esm/puppeteer/puppeteer-core.js`.

The consequence is direct: `site.screenshot.mjs` is the harness `site/AGENTS.md`
mandates for the visual-regression verification this very CR requires under
NFR-1 and AC-5. Removing `puppeteer-core` would delete the instrument that
proves the CR's own rendering claim, and would also break the contrast audit and
content check.

This is CR-0072 Risk 4 materialising exactly as written: *"not found by grep is
weaker evidence than confirmed absent"*, with the CR itself noting that if the
pin proves load-bearing then *"the search that missed it is worth recording"*.
It is recorded here.

Two further consequences for Phase 2:

1. **FR-2 resolves to retain-and-justify, not remove.** The consumer is now
   named and can be stated in the comment FR-2 requires.
2. **The exact pin is now explicable and should stay exact.** The scripts reach
   into `lib/esm/puppeteer/puppeteer-core.js`, an internal layout with no
   compatibility guarantee across majors. That is a good reason to pin exactly
   and a good reason not to take the 24.x to 25.x major bump inside this CR.
   The CR already excluded that bump; this finding strengthens the case rather
   than changing it.

## Raw logs

Raw outputs are gitignored under `.agents/logs/CR-0072-phase1-*.log`
(`install`, `build-run1`, `build-run2`, `lighthouse-run1`, `lighthouse-run2`,
`lh-summary-run1`, `lh-summary-run2`, `audit-prod`). The per-page score
extraction is `.agents/scripts/site.lighthouse.summary.mjs`.
