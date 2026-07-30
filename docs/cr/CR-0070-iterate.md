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
* **Cross-checked against Google's infrastructure after the fact.** A named Cloudflare
  tunnel was stood up at `preview.outlook-local-mcp.com` and PageSpeed Insights was run
  against it (Lighthouse 13.4.1, mobile). It disagrees with the local run in ways that
  bear directly on this attempt's disposition:

  | metric | PSI | local lhci |
  |---|---|---|
  | Performance | 0.91 | 0.93 |
  | Accessibility | 0.96 | 0.96 |
  | Best Practices | 0.96 | 1.00 |
  | SEO | 1.00 | 1.00 |
  | LCP | **2.4 s, inside budget** | 2,565 ms, over budget |
  | CLS | **0** | 0.094 |
  | Speed Index | 4.8 s | 2,253 ms |
  | TBT | 160 ms | about 20 ms |

  Both of the metrics this attempt targeted pass when Google measures them. The local
  CLS of 0.094 had no attributable element and reads as an artifact of the headless run
  rather than anything a visitor experiences. What drags PSI's Performance down is Speed
  Index at 4.8 s, which is tunnel latency (edge to a laptop rather than edge to Fastly)
  and should be discounted. Two of the three console errors behind PSI's Best Practices
  0.96 come from Cloudflare's `email-decode.min.js`, injected by the proxy and absent on
  GitHub Pages, so 1.00 is the honest figure there.

  Neither measurement is the deployed truth: local understates Speed Index and
  overstates CLS, PSI overstates Speed Index. The real numbers are likely better than
  both, and only a deploy settles it. That is a catch-22 worth naming, because the gate
  blocks the deploy and only the deploy yields a trustworthy measurement.

  PSI did surface one real defect the local run did not: Accessibility 0.96 is
  **colour contrast**, not the `role=tab` issue. `#abff02` lime on `#ffffff` measures
  1.23, and `#6f6f6f` on the off-white `#f0f0f0` section background measures 4.4,
  just under the 4.5 AA threshold. The second is a Phase 5 change that did not go far
  enough: the darkened grey was checked against white but not against `#f0f0f0`.

* **Why the work stopped here:** closing the remaining 0.02 means reducing DOM size or
  deferring hydration. Both alter the adopted v3 markup, which the CR's Out of Scope
  section defers, and that boundary has already produced one contradiction when the
  Accessibility floor forced colour edits. Deferring hydration additionally touches the
  GSAP and Lenis motion layer that was the reason v3 was chosen over the rebuild. That
  is a change-owner decision, not an agent one.
* **Disposition:** pending

### Attempt 2 — Hero install command: thin scrollbar, aligned copy button, desktop expansion

* **State:** settled
* **Hypothesis:** the change owner asked for three things on the hero install command
  row: the `scrollbar-width: thin; scrollbar-color: gray transparent` treatment, the
  copy button on the same centre line as the `go install` text (the scrollbar was
  pushing the text up), and, where the viewport allows, the container growing to fit
  the command so no scrollbar appears at all.
* **Surface touched:** `site/src/index.css`, `site/src/use-scrollbar-height.ts` (new),
  `site/src/components/HeroSection.tsx`.
* **What was changed:**
  1. `scrollbar-width: thin` and `scrollbar-color: gray transparent` on the scroller.
  2. The container grows to fit at the `lg` breakpoint (`lg:w-fit lg:max-w-full`), so
     the command stops overflowing and no scrollbar materialises on desktop.
  3. `padding-top: var(--sb)` on the scroller, with `--sb` measured at runtime by a new
     `useScrollbarHeight` hook.
* **Two approaches measured and rejected before the third worked.** Both are recorded
  because each looked correct and failed silently:
  * **`min-height` on the scroller.** Makes the box taller but leaves the scrollbar
    inside it, so the text still centres above the scrollbar. This had already been
    established on the abandoned CR-0069 and was not retried here.
  * **`padding-bottom` plus an equal negative `margin-bottom`.** The reasoning was that
    flex centring measures the margin box, so reserving the strip as padding and
    removing it from the margin box would centre the row as though the scrollbar were
    absent. Measured: the offset stayed at **5.8px**, because the visible content sits
    at the top of a box that now extends lower, so the text is pushed up by half the
    padding instead. This is the approach the earlier CR-0069 ledger recommended, and
    it is wrong for this geometry.
  * **What actually works** is `padding-top` equal to the scrollbar height. The
    scrollbar consumes the bottom of the content area, so matching padding above
    restores symmetry and the text centres against the element rather than against the
    space left above the scrollbar.
* **Why the height is measured rather than hard-coded:** it is 11px on this machine, 0
  wherever the platform uses overlay scrollbars, and 0 again once the content stops
  overflowing. A fixed value would introduce the very offset it was meant to remove on
  any of those. `useScrollbarHeight` publishes `offsetHeight - clientHeight` as `--sb`
  and recomputes on resize. Without JavaScript it is absent and the padding resolves to
  `0px`, which is exactly the pre-existing layout, never worse.
* **Verification evidence:** measured in-page with a geometry probe comparing the
  bounding-box centres of the `<code>` element and the copy button, across viewports.

  | viewport | scrollbar height | padding-top applied | scrolls | centre delta |
  |---|---|---|---|---|
  | 1280 | 0 | 0px | no | **0.3px** |
  | 1024 | 0 | 0px | no | **0.3px** |
  | 900 | 11px | 11px | yes | **0.3px** (was 5.8) |
  | 640 | 11px | 11px | yes | **0.3px** (was 5.8) |
  | 500 | 11px | 11px | yes | **-0.8px** (was 5.8) |

  All three requirements hold: the scrollbar is thin and grey-on-transparent where it
  appears, the button sits within a pixel of the text centre at every width, and at the
  `lg` breakpoint and above the container fits the command with no scrollbar at all.

  `make ci` exits 0. No regression: text without JavaScript unchanged at 12,360 /
  15,534 / 7,062 / 17,452 characters, one `<h1>` per page.
* **Not verified:** wide-viewport screenshots (1440 and above) could not be captured.
  Headless Chrome hangs under `--virtual-time-budget` because the scrub-driven parallax
  keeps a `requestAnimationFrame` ticker alive so virtual time never exhausts, and
  without the virtual clock `--dump-dom` fires before the probe's timeout. The
  quantitative probe succeeded at 1024 and 1280, where the desktop path is already
  exercised, and wider viewports only give the container more room.
* **Disposition:** kept

### Attempt 3 — Accessibility and markup validity: contrast, duplicate SVG ids, div-in-button, orphan tabs

* **State:** open
* **Hypothesis:** the four bounded defects blocking the gate (contrast below AA,
  duplicate SVG `defs` ids, `<div>` inside `<button>`, and `role="tab"` with no
  associated panel) can each be fixed to an exact measured target without altering the
  design, unblocking Accessibility 1.00 and zero real W3C errors independently of the
  uncertain performance work.
* **Surface touched:** a measurement harness under `.agents/scripts/`, plus
  `site/src/index.css`, `site/src/components/{GettingStarted,ToolsReference,ConfigReference}Section.tsx`,
  `site/src/components/DotGridOverlay.tsx`, `site/src/components/svg/Capability*.tsx`,
  and Footer, Platform, Capabilities, Hero.
* **The harness came first, because none of these were measurable.** Attempt 1 could not
  capture wide viewports at all, and the CR's "no visual regression" criterion had no
  instrument behind it. Six scripts now exist: deterministic screenshot capture, a
  dependency-free PNG decoder and per-channel diff, W3C Nu validation that partitions
  real errors from the checker's CSS-profile lag, an exhaustive DOM contrast audit, a
  content-regression check, and a Lighthouse summariser.

  Making capture deterministic was the whole difficulty, and it is worth recording why,
  because the naive approach fails silently. Two captures of the *same* build differed
  on 104 of 143 tiles. Four causes, each removed in turn:
  1. `scroll-behavior: smooth` in `index.css` animates a programmatic scroll, so tiles
     were captured mid-scroll. Fixed with `behavior: 'instant'`.
  2. The canvases seed particles from `Math.random` and advance on `requestAnimationFrame`.
     Fixed with a seeded PRNG and a virtual clock the harness pumps a fixed number of
     frames.
  3. `IntersectionObserver` and `ResizeObserver` deliver against real time, so the frame
     on which a canvas started animating varied. Both replaced with synchronous versions
     recomputed from geometry each pumped frame.
  4. The residue was mid-flight GSAP reveal state. Capturing under
     `prefers-reduced-motion: reduce` removes it, and every scroll-driven section already
     has an explicit reduce branch that sets its final state, so this captures the
     settled appearance rather than an arbitrary animation frame.

  Two captures of the same build now differ on 0 of 135 tiles, worst tile 99.78% within
  +/-2. That noise floor is what makes the +/-2 on >=99% criterion meaningful.
* **What was changed, and why each:**
  1. **Contrast.** The audit found 40 failing text elements, not the two the PSI report
     named. `--color-gray-400` `#6f6f6f` -> `#6d6d6d` (4.54:1 on `#f0f0f0`, 5.17:1 on
     white); the three `01`/`02`/`03` step numbers moved from `text-brand-lime` to the
     `text-lime-dark` token the codebase already defines for light backgrounds (5.57:1);
     `text-white/30` -> `/50` and `text-white/40` -> `/60` on the dark sections (5.02:1
     and 6.63:1 at worst), which preserves the three-step visual hierarchy.
  2. **Duplicate SVG ids.** 29 duplicates, from the desktop and mobile branches rendering
     the same five SVG components plus the dot-grid overlay. Each component now derives
     its ids from React's `useId`, which is stable across prerender and hydration.
  3. **`<div>` inside `<button>`.** Both accordion triggers wrapped their label in a
     `div`, which is flow content and invalid inside a button. Changed to `span`; because
     Tailwind's `flex` sets the display property directly and flex items are blockified,
     the layout is identical.
  4. **Orphan `role="tab"`.** The install group had a panel but no `aria-controls` link
     to it; the config group had no panel at all. Both groups now carry
     `aria-controls`/`aria-labelledby` id pairs scoped by `useId`. A roving `tabindex`
     was deliberately *not* added: it is part of the ARIA pattern but useless without
     arrow-key handling, and adding it alone would make the unselected tabs unreachable
     by keyboard, trading a validator message for a real regression.
* **The duplicate ids were not a cosmetic validity issue.** The second instance's
  `url(#id)` references were binding to the first instance's definitions, inside a
  hidden subtree, so on mobile the Multi-Account diagram rendered with its three account
  cards entirely invisible (only the dashed connectors and lock glyphs painted) and the
  Zero-Config Auth diagram lost every card fill, border, glow and arrowhead. Mobile is
  the viewport Lighthouse emulates and the one most visitors use. The screenshot diff is
  what surfaced this: the fix "regressed" nine tiles, and the tiles turned out to be the
  repair.
* **Verification evidence:**
  - W3C Nu real errors on `index.html`: 32 -> 0. The other three pages were already 0.
    The 54 CSS-profile-lag messages per page (`@property`, `margin-trim`) are reported
    separately and excluded, as the goal specifies.
  - DOM contrast audit: 40 -> 0 failing text elements across 4 pages x 4 widths.
  - Visual diff: the contrast change touched one tile beyond tolerance (the mobile
    footer, 98.84%, which is the intended colour change); the id change corrected nine
    tiles as described; the markup change moved nothing at all (0 of 135 failing).
  - Content check: text without JavaScript, one `<h1>` per page, 10/10 `SeeDocs`
    anchors, six crawler files and five Mermaid fences all hold.
* **Disposition:** pending

### Attempt 4 — Landing-page performance: the gate is unblocked, and 1.00 is shown to be out of reach

* **State:** open
* **Hypothesis:** the landing page's Performance 0.93 and Cumulative Layout Shift 0.094
  can be closed by attacking what the Lighthouse report actually attributes them to,
  rather than by the DOM-size and unused-JavaScript figures the diagnostics headline.
* **Surface touched:** `site/build/prerender.mjs`, `site/src/index.css`,
  `site/src/main.tsx`, `site/lighthouserc.json`.
* **What was changed, and why each:**
  1. **The above-the-fold font preloads were the wrong four.** Lighthouse attributed the
     *whole* of the 0.094 CLS to three fonts arriving late: inter-500, geist-mono-400 and
     geist-mono-600. The preload list named none of them, and did preload inter-600 and
     inter-700, which the hero never paints. Corrected to the four the hero actually uses.
     **CLS 0.094 to 0.000 exactly**, and Performance 0.93 to 0.95.

     This also corrects attempt 1's attribution, which recorded that the CLS "has no
     attributable element and did not move when the font work landed, which rules out font
     swap". It was font swap; the earlier run simply preloaded the wrong faces, so the
     shift did not move and the cause looked excluded.
  2. **`content-visibility: auto` on the five capability diagrams.** They are 1,192 of the
     page's ~1,870 elements and all below the fold. Applied to the `<svg>` elements rather
     than to a section: each is sized entirely by its container, so skipping its subtree
     cannot change document height or move the pinned ScrollTrigger measurements. Main
     thread Rendering 488 ms to 80 ms, Style and Layout 518 ms to 437 ms, TBT 26 ms to
     8 ms, Performance 0.95 to 0.96.
  3. **The client bundle is fetched after the main content has painted.** The landing
     page's `<script type="module">` is rewritten by the pre-render step into a bootstrap
     that waits for the browser's Largest Contentful Paint entry, then for idle, and only
     then requests the bundle. Performance 0.96 to 0.97. The trade-off is stated in the
     code: the tabs, accordions, copy buttons and motion layer are inert until it arrives.
     Every word, style and layout is present throughout, because the root is pre-rendered.
* **Why 1.00 and LCP 1,700 ms are not reachable, measured rather than argued.** Four
  hypotheses were tested and three were falsified:

  | change | LCP | verdict |
  |---|---|---|
  | defer the module's *evaluation* (dynamic import after paint) | 2,557 ms | no effect |
  | `fetchpriority="low"` on the script tag | 2,570 ms | no effect |
  | halve the DOM (strip the duplicate diagram markup) | 2,619 ms | no effect |
  | serve a 20-byte bundle, everything else identical | **1,803 ms** | **the lever** |

  Largest Contentful Paint here is a function of bytes on the critical path and nothing
  else reachable: 1,803 ms of floor, plus 750 ms for the bundle's 137 KB at the emulated
  1,475 Kbps. Lantern charges every byte fetched before the observed paint, and on a local
  server the whole document arrives inside 90 ms, so no amount of scheduling removes it.

  1,700 ms is therefore below the floor the page has with *no JavaScript at all*, and
  100 in Performance needs the bundle down to roughly 20 KB — which means removing the
  GSAP and Lenis motion design that CR-0070 adopted v3 in order to keep. That is a
  product decision for the change owner, not a defect.

  The fonts were then tested as the other half of the critical path, to be sure the
  conclusion was not an artefact of looking only at the bundle. Removing all five faces
  outright — well past anything acceptable — reaches 1,952 ms and **0.99**, still not
  1.00. Subsetting them properly (94 KB to 43 KB, Carry-forward records why it was
  reverted) buys 148 ms and leaves the score at 0.97. Both roads end in the same place:
  with fonts perfectly optimised the bundle would still have to fall to roughly 27 KB,
  and React alone is about 45 KB.
* **FR-36 amended accordingly, and tightened in every direction the evidence allows.**
  Accessibility, Best Practices and SEO go from "at least 95" to **100 on every page**;
  documentation-page Performance goes from "at least 95" to **100**; the landing page is
  held at **96** against a measured 0.97. Per-page budgets for LCP, CLS and TBT are now
  asserted in their own right, so a regression fails the gate even when the rounded
  category score does not move. NFR-1 and AC-22 are untouched: they are *field*
  thresholds measured live by PageSpeed Insights, not this lab figure.
* **Verification evidence:** `pnpm --dir site run lighthouse` **exits 0**.

  | page | perf | was | LCP ms | was | CLS | was | TBT ms |
  |---|---|---|---|---|---|---|---|
  | index.html | **0.97** | 0.93 | 2,554 | 2,565 | **0.000** | 0.094 | 11 |
  | quickstart.html | 1.00 | 1.00 | 1,502 | 1,502 | 0.000 | 0.000 | 0 |
  | concepts.html | 1.00 | 1.00 | 1,652 | 1,652 | 0.000 | 0.000 | 0 |
  | troubleshooting.html | 1.00 | 1.00 | 1,651 | 1,652 | 0.000 | 0.000 | 0 |

  Accessibility, Best Practices and SEO are 1.00 on all four pages. W3C Nu real errors
  are 0 on all four. The contrast audit is 0 failures across 4 pages x 4 widths. The
  visual diff against the pre-performance build is 0 of 135 tiles failing, so none of this
  changed what the site looks like. Hydration, tab switching and the injected bundle were
  checked directly in a browser with no console errors. `make ci` exits 0.
* **Disposition:** pending

### Attempt 5 — Crawler surface: the agent list, and the rich-result claims

* **State:** open
* **Hypothesis:** two of the carry-forward corrections from the SEO and GEO research are
  bounded enough to land now: FR-18's agent list, which is the only item in this session
  with a citation rationale behind it, and the CR's implied rich-result eligibility for
  entities that cannot produce one.
* **Surface touched:** `site/public/robots.txt`, and FR-18, FR-41, FR-42, AC-4 and AC-8 of
  CR-0070.
* **What was changed, and why each:**
  1. **`robots.txt` named the wrong agents.** It told a future maintainer not to disallow
     `GPTBot`, `ClaudeBot` and `Google-Extended`, which are **training-consent** tokens
     and have no bearing on whether the project is cited, while omitting every
     **retrieval-for-citation** agent: `ChatGPT-User`, `OAI-SearchBot`, `Claude-User`,
     `Claude-SearchBot`, `Perplexity-User` and `Applebot`. Nothing was ever blocked — the
     file is a bare `User-agent: *` with `Allow: /` — so this was misleading documentation
     rather than a live defect, but it is documentation whose whole purpose is to stop
     someone adding a costly `Disallow` later. The file now sets out the two classes
     separately, and notes that ChatGPT Search draws on the Bing index, so permission here
     is necessary but not sufficient. FR-18 and AC-4 were rewritten to match, and record
     what the earlier list got wrong.
  2. **The CR implied rich results that cannot exist.** `FAQPage` was deprecated (notice
     2026-05-07, tooling removed June 2026) and `HowTo` retired in 2023, and
     `SoftwareApplication` qualifies only with an `aggregateRating` or `review` that this
     project will not self-publish, because Google's policy forbids self-serving reviews.
     AC-8 required the Google Rich Results Test to report no errors, which no build of
     this site can satisfy; it now asserts structured-data *validity*, which the build can
     check, and says why the stronger claim was dropped. FR-41 and FR-42 keep both
     entities — they are shipped and cost nothing — on an explicitly speculative
     LLM-parsing rationale rather than an implied search-visibility one.
* **Not done, and deliberately:** the research's remaining items are unchanged in
  Carry-forward. IndexNow submission is a new requirement rather than a correction, the
  Lighthouse gate's GEO framing is addressed by FR-36's rewrite in attempt 4, and the
  `llms.txt` "structural advantage" wording and the discovery-channel question are
  change-owner calls.
* **Verification evidence:** the built `dist/robots.txt` carries the new content, the six
  crawler files are still present, and the content check passes unchanged.
* **Disposition:** pending

## Carry-forward

Items this session has identified but not actioned. They are not attempts and carry no
disposition; they are recorded here so they survive the session and can be routed at
close.

### TODO: reconcile CR-0070 against the SEO and GEO research

`.agents/explore/2026-07-30-seo-geo-state-of-the-art.md` (105 agents, 23 sources, 25
claims adversarially verified) contradicts several requirements this CR implemented.
Recorded here so the corrections are not lost; they belong in the CR text and, for the
first item, in shipped code.

**Status after attempts 4 and 5.** The first three items below are **done**: the agent
list is corrected in `robots.txt` and FR-18, the `FAQPage` and `HowTo` rationale is stated
as speculative in FR-41 and FR-42, and AC-8 no longer asserts a Rich Results pass. The
Lighthouse item is **done** by FR-36's rewrite, which now justifies the gate on user
experience and classic SEO and claims no discovery effect. The remaining three — the
`llms.txt` wording, IndexNow, and the discovery-channel question — are **open** and are
change-owner decisions rather than corrections.

* **FR-18's agent list is wrong, and this is the one that costs citations.** Vendors
  separate training tokens from retrieval-for-citation tokens. `GPTBot` and `ClaudeBot`
  are **training only**; naming them is a training-consent decision, not a citation
  lever. The citation-relevant agents are missing: `ChatGPT-User`, `Claude-User`,
  `Claude-SearchBot`, and, since Applebot renders, `Applebot-Extended`. Our shipped
  `robots.txt` uses a bare `User-agent: *` with `Allow: /`, so nothing is blocked
  today, but the comment naming the wrong five agents is misleading documentation.
* **`FAQPage` is deprecated** (notice 2026-05-07, tooling support removed June 2026)
  and **`HowTo` was retired in 2023**. Both are mandated by FR-41 and FR-42 and both
  are shipped. They have no rich-result value. Keep them only on a speculative
  LLM-parsing rationale and say so in the CR rather than implying eligibility.
* **`SoftwareApplication` cannot qualify for a rich result here.** Google requires
  `aggregateRating` or `review`, and self-publishing a rating violates its
  self-serving review policy. AC-8's Rich Results Test assertion will never pass for
  it and is now partly meaningless for `FAQPage`.
* **The Lighthouse gate has no GEO justification.** Zero claims connecting performance
  to AI citation survived verification. FR-36 to FR-38 and NFR-1 are perfectly well
  justified on user-experience and classic-SEO grounds; the CR should simply not imply
  a GEO rationale.
* **`llms.txt` and `/index.md` are neither justified nor condemned.** Google is
  explicitly neutral, a 300k-domain study found no clear effect, and no other vendor
  has addressed them. The CR's Motivation currently asserts a "structural advantage no
  amount of metadata provides", which the evidence does not support. Soften to a stated
  bet. The Mermaid conversion retains its first-principles argument.
* **Possible missing requirement:** ChatGPT Search draws on the Bing index, so
  robots.txt permission is necessary but not sufficient. Phase 5 covers Search Console
  and Bing registration but not IndexNow submission.
* **Strategic open question worth more than any on-page tactic:** for a niche
  open-source developer tool, discovery may flow through MCP-server registries and
  GitHub, Hacker News, and Reddit co-occurrence rather than the project site. The
  research could not settle it, and it may dominate everything in this CR.

Pre-rendering is confirmed as the highest-value requirement, which validates the CR's
central thesis, with the honesty caveat that the evidence is third-party edge telemetry
rather than vendor confirmation and should be cited as such.

### TODO: font subsetting, built and then reverted, worth 148 ms

Attempt 4 built a post-build step that subsets each emitted face to the 111 characters the
site can actually display, cutting the five above-the-fold woff2 files from 94 KB to 43 KB.
Measured: **148 ms of Largest Contentful Paint on every page** (index 2,554 to 2,406;
concepts 1,652 to 1,503; quickstart 1,502 to 1,352; troubleshooting 1,651 to 1,502). No
category score moved, on any page.

It was reverted for one reason: `pyftsubset` is a Python tool, so the Node build would gain
a Python plus `fonttools` plus `brotli` dependency, which CI does not have and which sits
awkwardly against NFR-2's "no network access beyond the package registry". A JavaScript
subsetter with a bundled harfbuzz (`subset-font` and similar) would avoid that entirely,
and is the way to pick this up.

Two findings worth keeping, because both cost a full measure-and-diff cycle to find and are
invisible without a pixel comparison:

* **`--no-hinting` changes rasterisation.** It saves about 10% more and moved 110 of 135
  screenshot tiles outside the visual tolerance, on text that was otherwise identical.
* **Restricting `--layout-features` does too.** `kern,liga,clig,calt` looks like the
  complete set for western text and is not: it drops `ccmp`, `locl`, `mark` and `rlig`,
  which the default set keeps, with the same 110-tile result. `--layout-features=*` alone
  did not fix it either; `--glyph-names`, `--symbol-cmap`, `--legacy-cmap` and
  `--notdef-outline` were also required before the diff came back clean.

The lesson generalises past fonts: a "pure byte reduction" is only pure if something
measures the pixels, and three of the four attempts here silently were not.

### TODO: fold metrics tracking into the project so regressions are visible

Today the only enforced metric is the Lighthouse gate, and it asserts a threshold
without retaining history. A score that drifts from 1.00 to 0.96 across three commits
fails nothing until it crosses the floor, and nobody can see when it started. The same
is true of every structural property this CR added: nothing records that the landing
page had 12,360 characters of text without JavaScript yesterday and 4,000 today.

What is worth tracking, all of it already measurable by the checks this session used:

* Lighthouse category scores and the underlying metrics (LCP, CLS, TBT, Speed Index,
  FCP) per page, retained over time rather than only asserted.
* Payload budgets: JS and CSS bytes, and page HTML size. The stylesheet went from 88 KB
  to 39 KB in this session and nothing would have noticed it going back.
* Structural SEO and GEO properties: characters of text without JavaScript per page,
  `<h1>` count, canonical and Open Graph and Twitter tag counts, JSON-LD entity types,
  crawler-file presence, `SeeDocs` anchor resolution, `/index.md` word count and Mermaid
  fence count.
* W3C validator error count per page, excluding the CSS-profile lag that accounts for 54
  of the current errors on every page.

The natural home is the existing site test suite plus the deploy workflow, with a
committed snapshot so a change shows up as a diff. That is a CR-sized piece of work, not
a session attempt.

### TODO: make future agents aware of the preview tunnel and PageSpeed Insights

Both capabilities now exist and no agent will discover them from the repository. Neither
is documented anywhere durable yet; the ledger is the wrong long-term home, so at close
this should be routed to `CONTRIBUTING.md` or `docs/reference/`.

What an agent needs to know:

* **A named Cloudflare tunnel exists.** `outlook-local-mcp-preview`
  (`c321c3b3-f04b-4b49-bc51-435776798635`) serves `preview.outlook-local-mcp.com` from
  `http://localhost:8099`, so a locally built `site/dist` can be validated by external
  tools that need a public URL. Start it with
  `cloudflared tunnel run --token <token from the Cloudflare API>`.
* **Credentials live in `site/.env`**, which is gitignored: `CLOUDFLARE_API_TOKEN`,
  `CLOUDFLARE_ACCOUNT_ID`, `CLOUDFLARE_ZONE_ID`, `PAGESPEED_API_KEY`. The token is
  scoped to Tunnel:Edit, DNS:Edit, and Zone:Read on this zone only.
* **PageSpeed Insights runs non-interactively** with `PAGESPEED_API_KEY` against the
  preview hostname. Without a key the shared anonymous quota is exhausted and every call
  fails; the key is free and needs no billing account.

And, more importantly, what it must NOT conclude from them:

* **Content-Type over the tunnel is not GitHub Pages' Content-Type.** The tunnel proxies
  a local `python http.server`, so MIME types, headers, compression, and 404 handling are
  ours, not the host's. FR-34's open question about `/index.md` is not answered by any
  tunnel measurement.
* **Performance over the tunnel is pessimistic.** Traffic routes edge to laptop instead
  of edge to Fastly, which inflated Speed Index to 4.8 s against 2,253 ms locally.
  Treat SEO, Accessibility, and Best Practices as meaningful; treat Performance as
  indicative only.
* **The proxy injects its own JavaScript.** Cloudflare's `email-decode.min.js` throws a
  `TypeError` that Lighthouse charges to Best Practices, costing 0.04 on a page that
  scores 1.00 locally. It will not exist on GitHub Pages.
* **The page declares itself canonical elsewhere.** Canonical, `og:url`, sitemap `loc`,
  and JSON-LD `url` are all hardcoded to the apex, so anything indexing-related measured
  on the preview hostname is measuring a page that disclaims itself.

## Distillation

<!-- Populated on close. Empty while the session is open. -->

### Recommended Patterns

<!-- What worked, expressed as durable guidance. Empty until close. -->

### Anti-Patterns

<!-- What did not work, drawn from the discarded attempts, with the reason each failed. Empty until close. -->
