---
name: iterate-ledger
description: Session ledger for the last-mile iteration session against CR-0070, the site v3 adoption with SEO and GEO foundation, Markdown representation, build provenance, Lighthouse budget, and CI deployment. Records every attempt, its verification evidence, and its disposition, then distils the session into recommended patterns and anti-patterns at close.
cr: "CR-0070"
status: "open"
opened: "2026-07-30"
closed: ""
source-branch: "dev/site-v3"
source-commit: "772f512"
worktree: "/Users/desek/Repo/desek/outlook-local-mcp"
---

# Iteration Session Ledger: Closing the Lighthouse Gate on the Landing Page

## Session Context

* **Governing Change Request:** CR-0070, which adopted the separately developed v3
  site as the tracked project website, pre-rendered it so its content exists without
  JavaScript, added the crawler surface and structured data, emitted a Markdown
  representation of the landing page with its diagrams as Mermaid, stamped build
  provenance into the artifact, added the GigWhere acknowledgement, and moved
  deployment into a GitHub Actions workflow publishing to `gh-pages`.
* **Gap being closed:** the CR agent team ran all five phases, but the finalizer
  **refused**. FR-36 requires a Lighthouse Performance score of at least 0.95 and the
  landing page measured below it. Because the Lighthouse gate sits before the publish
  step in the deploy workflow, the CR cannot ship: merging as-is means the workflow
  fails and `gh-pages` is never updated. A second, smaller gap is a scope
  contradiction the CR text still carries (see below).
* **Starting point:** branch `dev/site-v3` at commit `772f512`, working tree
  `/Users/desek/Repo/desek/outlook-local-mcp`.

### State at session open

Recorded so a re-hydrated agent can tell what was already true before the first
attempt, rather than attributing it to session work.

* All five CR phases are implemented and independently verified. Phase checkpoints:
  `96befd0`, `a483293`, `46aaf37`, `efd931b`, `a05abb3`, plus reviewer `130be38`.
* The site pre-renders four pages with 7,062 to 17,452 characters of text without
  JavaScript, one `<h1>` each, and no hidden content in the pre-rendered markup.
* All 10 verb `SeeDocs` anchors resolve, generated with the repository's own
  `headingToAnchor` from `internal/tools/get_docs.go`.
* Crawler surface, `/index.md` with five Mermaid fences, provenance in both the head
  and `/build-info.json`, and the GigWhere backlink are all in place and verified.
* `make ci` exits 0. The site builds with pnpm.
* The Pages custom domain is re-enabled.

### Known open items, carried in from implementation

Context, not session attempts.

* **The finalizer refused and the CR is still `proposed`.** The validator, gap-fixer,
  and doc-updater never ran.
* **A live regression is public.** Re-enabling the Pages custom domain means the apex
  now serves the old `gh-pages` build, whose assets are at `/outlook-local-mcp/...`
  and 404 at the root, so `https://outlook-local-mcp.com` is currently a blank page.
  It clears when this branch merges and the workflow deploys, which the failing gate
  currently prevents. Temporarily disabling the custom domain would restore the
  working project-page site in the meantime.
* **A scope contradiction in the CR text.** Out of Scope says "Layout, typography, and
  colour changes are deferred", but reaching the FR-36 Accessibility floor required
  WCAG AA contrast fixes and heading-level corrections. The changes are minimal and
  correct; the CR forbids them on paper. The finalizer flagged this as needing
  reconciliation.
* **Copy correction is deliberately deferred** to a follow-up CR and enumerated in the
  CR's "Deferred to a follow-up CR" section. The site therefore still ships the
  "23 MCP tools" claim, the Diagnostics category, and inherited absolute security
  claims. That is intended, not a defect.
* **A minor defect, unfixed:** `/index.md` renders `### Calendar Management14 tools`,
  adjacent element text concatenated without a separator.

### Project checks used by this session

The skill's generic text names `bats -r tests/`. That does not apply to this
repository. The checks run for each attempt are:

```bash
make ci                            # Go pipeline, unaffected by site changes but must stay green
pnpm --dir site run build          # tsc -b, vite build, SSR build, prerender
pnpm --dir site run lighthouse     # lhci autorun, 4 pages, 3 runs each, mobile emulation
```

Lighthouse numbers in this ledger are medians of 3 runs per page under lhci's default
mobile emulation, and are lab measurements. Field data from real users cannot be
collected until the site is deployed.

## Attempt Ledger

### Attempt 1 — Reduce landing-page LCP and documentation-page CLS

* **State:** open
* **Hypothesis:** the change owner directed "attempt the LCP and CLS work" after the
  finalizer refused. Two measured breaches of the CR's NFR-1 budgets were the target:
  the landing page at LCP 2,755 ms against a 2.5 s budget, and `concepts.html` at CLS
  0.105 against a 0.1 budget. The second is notable because that page scored 0.95 while
  breaching the metric, which is exactly why NFR-1 is stated separately from the
  Lighthouse category score.
* **Surface touched:** `site/src/index.css`, `site/build/prerender.mjs`. Committed as
  `772f512`, before this ledger existed, because the work was directed and carried out
  prior to the session being opened.
* **What was changed, and why each:**
  1. **Latin-only font subsets.** The `@fontsource` full entrypoints declare every
     charset the families ship, including Cyrillic, Greek, Vietnamese, and the extended
     Latin ranges. That accounted for 50 KB of the 88 KB stylesheet in `@font-face`
     rules alone and shipped 34 woff2 files totalling 472 KB, for a site that renders
     Latin only. Now 6 rules, 6 files, 120 KB, and the stylesheet drops to 39 KB.
  2. **Prerender treatment extended to the documentation pages.** CSS inlining and font
     preloading had been landing-page only, leaving the three doc pages with a
     render-blocking stylesheet request Lighthouse costed at 750 to 900 ms and no font
     preload at all, so their prose swapped font late and charged the reflow to CLS.
     Inlining is only affordable because of the first change: 39 KB inline is
     reasonable, 88 KB was not. Preloads are chosen per page rather than blanket, since
     preloading a font a page does not paint above the fold wastes critical-path
     bandwidth.
* **Verification evidence:** measured before and after with `lhci`, 12 runs each
  (4 pages, 3 runs, mobile emulation, medians shown).

  | page | perf | was | LCP ms | was | CLS | was |
  |---|---|---|---|---|---|---|
  | concepts.html | **1.00** | 0.95 | 1,652 | 1,953 | **0.000** | 0.105 |
  | quickstart.html | **1.00** | 0.98 | 1,502 | 1,802 | **0.000** | 0.018 |
  | troubleshooting.html | **1.00** | 0.98 | 1,652 | 1,802 | **0.000** | 0.001 |
  | index.html | 0.93 | 0.91 | 2,565 | 2,755 | 0.094 | 0.094 |

  Accessibility, Best Practices, and SEO are 1.00 on every page except the landing
  page's Accessibility at 0.96. `make ci` exits 0. No regression: text without
  JavaScript, one `<h1>` per page, canonical tags, and all six crawler files unchanged.

  The `concepts.html` NFR-1 CLS breach is resolved. The landing page improved but
  **still misses the FR-36 floor at 0.93 against 0.95**, so the gate still fails and
  the CR still cannot ship.
* **Attribution for what remains, from the Lighthouse reports rather than inference:**
  82 percent of the landing page's LCP is **render delay**. TTFB is 451 ms (18 percent)
  and both load delay and load time are zero, so the residual cost is neither bytes nor
  blocking requests nor network: it is main-thread work parsing and hydrating an
  1,830-element DOM, with 2.4 s of total main-thread work and 52 KiB of unused
  JavaScript. LCP at 2,565 ms is only 65 ms over budget. The landing page's CLS of
  0.094 has **no attributable element** and did not move when the font work landed,
  which rules out font swap and points at the load-time GSAP and Lenis animation.
* **Why the work stopped here:** closing the remaining 0.02 means reducing DOM size or
  deferring hydration. Both alter the adopted v3 markup, which the CR's Out of Scope
  section defers, and that boundary has already produced one contradiction when the
  Accessibility floor forced colour edits. Deferring hydration additionally touches the
  GSAP and Lenis motion layer that was the reason v3 was chosen over the rebuild. That
  is a change-owner decision, not an agent one.
* **Disposition:** pending

## Distillation

<!-- Populated on close. Empty while the session is open. -->

### Recommended Patterns

<!-- What worked, expressed as durable guidance. Empty until close. -->

### Anti-Patterns

<!-- What did not work, drawn from the discarded attempts, with the reason each failed. Empty until close. -->
