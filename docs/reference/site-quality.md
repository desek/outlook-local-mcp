# Site quality: measuring the website, and what the measurements mean

Contributor-facing. Not embedded in the binary and not published to the site.

The website is gated on Lighthouse, W3C validity, WCAG contrast, and a pixel comparison
against the previous build. This document covers how to run those checks and — more
usefully — the things that were learned the expensive way about what they actually
measure. Several plausible optimisations move nothing, and several "lossless" changes are
not lossless.

## Running the checks

The harness lives in `.agents/scripts/`. Every script is standalone, takes an optional
`dist` directory, and exits non-zero on failure.

```bash
pnpm --dir site run build                          # must precede every check below
pnpm --dir site run lighthouse                     # the gate; thresholds in site/lighthouserc.json
node .agents/scripts/site.lighthouse.summary.mjs   # per-page scores and Core Web Vitals medians
node .agents/scripts/site.validate.mjs             # W3C Nu, real errors separated from CSS-profile lag
node .agents/scripts/site.contrast.audit.mjs       # every rendered text node, 4 pages x 4 widths
node .agents/scripts/site.content.check.mjs        # text without JS, one h1, SeeDocs anchors, crawler files
node .agents/scripts/site.screenshot.mjs <out-dir> # deterministic capture, 135 tiles
node .agents/scripts/site.visual.diff.mjs <a> <b>  # per-channel comparison of two capture sets
```

A visual comparison needs a baseline built from the commit being compared against, which
means a worktree:

```bash
git worktree add /tmp/before <commit>
ln -s "$PWD/site/node_modules" /tmp/before/site/node_modules
(cd /tmp/before/site && ./node_modules/.bin/tsc -b && ./node_modules/.bin/vite build \
  && ./node_modules/.bin/vite build --ssr src/entry-server.tsx --outDir dist-ssr \
  && node build/prerender.mjs)
node .agents/scripts/site.screenshot.mjs /tmp/shots-before /tmp/before/site/dist
node .agents/scripts/site.screenshot.mjs /tmp/shots-after
node .agents/scripts/site.visual.diff.mjs /tmp/shots-before /tmp/shots-after
```

## The screenshot harness is only trustworthy because five clocks are frozen

Capture determinism is the whole basis of the visual gate, and it took five separate
fixes. Two captures of the *same* build initially differed on 104 of 143 tiles. They now
differ on 0 of 135, worst tile 99.989%, against a 99% threshold.

| source of nondeterminism | why it escapes the obvious fixes | how it is controlled |
|---|---|---|
| `scroll-behavior: smooth` in `index.css` | animates a programmatic scroll, so tiles capture mid-scroll | `window.scrollTo({behavior: 'instant'})` |
| canvases seeded by `Math.random`, advanced by `requestAnimationFrame` | the hero background and brand particles never converge | seeded PRNG plus a virtual clock the harness pumps a fixed number of frames |
| `IntersectionObserver` and `ResizeObserver` | deliver against real time, so a canvas starts animating on a different frame each run | replaced with synchronous versions recomputed from geometry each pumped frame |
| mid-flight GSAP reveal state | a scrubbed section is legitimately mid-animation at a given scroll offset | capture under `prefers-reduced-motion: reduce`, which every section has an explicit branch for, giving the settled appearance |
| **SVG SMIL** (`<animate>`, `<animateMotion>`) | runs on the SVG document timeline — touched by neither the virtual clock nor reduced-motion emulation | `svg.pauseAnimations()` and `svg.setCurrentTime(0)` on every root |

SMIL is the one to remember. It failed *intermittently*, leaving a single tile straddling
the threshold at 98.8% to 99.2% and failing about half of runs, which is worse than
failing outright: it produced a green result most of the time and a false regression the
rest.

## What Lighthouse actually measures here, and what it does not

`lhci` uses **simulated** throttling. Metrics are not observed; they are computed by the
Lantern model from the observed trace. Three consequences, all measured:

* **Scheduling is invisible.** Lantern charges every byte fetched before the *observed*
  paint, and on the local static server the whole document arrives inside 90 ms. Deferring
  the module's evaluation behind a dynamic import, setting `fetchpriority="low"`, and
  injecting the script only after a `largest-contentful-paint` PerformanceObserver entry
  each moved LCP by **under 10 ms**. Only the byte count moves it.
* **The landing page's LCP is a linear function of bundle size.**
  `LCP = 1,803 ms + 4.34 ms per KB gzipped`, which reproduces every measurement taken to
  within a millisecond. 1,803 ms is the page's floor with no script at all.
* **The prominent diagnostics name correlates, not causes.** `dom-size` headlined 1,830
  elements; stripping 1,192 of them made LCP slightly *worse*. `unused-javascript`
  headlined 52 KiB and was irrelevant. The causes were in the audit details: the
  `layout-shifts` audit named three specific font files, and the LCP phase table named
  transfer rather than evaluation.

### Why the landing page is held to 96 and the documentation pages to 100

The library floor — React, ReactDOM, GSAP, ScrollTrigger and Lenis, hydrating a component
that returns one empty `<div>` — is **108.79 KB gzipped**, or 590 ms. The site's own code
is 29 KB of the 137.6 KB shipped, and the five capability diagrams only 9.3 KB of that.

With no script element at all the landing page audits FCP 1.00, Speed Index 1.00, TBT
1.00, CLS 1.00 and **LCP 0.98**, a weighted 0.995 that reaches 1.00 only by rounding. A
20-byte stub and a 4.5 KB vanilla-JavaScript layer both score 0.99. So 100 on that page
requires shipping *no JavaScript*, not a lighter framework, and the thresholds in
`site/lighthouserc.json` reflect the measured ceiling rather than an aspiration.

### Thresholds are hardware-sensitive

Scores were calibrated on a machine reporting `benchmarkIndex` ≈ 2,970. CI runners are
typically 700 to 1,500, and a slower CPU inflates the observed timings the model scales,
so the same build scores lower there. Headroom is thin by design — the landing page at
0.970 against a 0.96 floor, the documentation pages at 1.000 against 1.00. **A first
failure on new hardware is a calibration problem, not a regression**; take the numbers
from CI, which is where the gate runs, and read `lighthouse-reports` from the workflow
artifact for the attribution.

## Font subsetting is not a lossless optimisation

Subsetting the five above-the-fold faces from 94 KB to 43 KB is worth 148 ms of LCP on
every page. It has been implemented twice and reverted twice, and is currently **not** in
the build.

Both `pyftsubset` and harfbuzz (`subset-font`) change how text rasterises, with an
identical failure signature to three decimal places. Text geometry is provably unaffected
— the first six paragraphs of `concepts.html` occupy the same rectangles to a hundredth of
a pixel and the document is the same height — but glyph edge antialiasing differs by up to
164/255 on individual subpixels, putting 109 of 135 tiles outside a ±2 tolerance.

Two specific traps, both invisible without a pixel comparison:

* `--no-hinting` changes rasterisation. It saves about 10% more and is not worth it.
* Restricting `--layout-features` to `kern,liga,clig,calt` looks like the complete set for
  western text and is not: it silently drops `ccmp`, `locl`, `mark` and `rlig`.

Reinstating it is a change-owner decision — 148 ms against slightly different edge
smoothing, with layout untouched — not a technical one.

## Accessibility

`site.contrast.audit.mjs` walks every rendered text node at four widths and computes the
effective background by climbing to the nearest opaque ancestor. Prefer it over the
Lighthouse audit, which reports only the emulated viewport and truncates its node list: it
named 2 failures where the exhaustive walk found 40.

Two implementation notes:

* Tailwind v4 emits `color-mix()` for its opacity utilities, so a computed colour comes
  back as `color(srgb 1 1 1 / 0.5)` rather than `rgba()`. Parse colours by painting them
  into a canvas and reading the pixel back; pattern-matching a subset silently mis-reads
  the rest, and did.
* Contrast fixes belong on the tokens the design system already defines —
  `--color-lime-dark` exists for small text on light backgrounds — not on a new token
  invented for the occasion.

Do not add a roving `tabindex` to a tab strip to satisfy a validator. It is genuinely part
of the ARIA tabs pattern, but without arrow-key handling it makes every unselected tab
unreachable by keyboard: a validator message traded for a real defect. `aria-controls` and
`aria-labelledby` alone satisfy the checker and break nothing.

## W3C validation

`site.validate.mjs` posts each built page to the Nu checker and partitions the result.
Around 54 messages per page are the checker's CSS profile lagging shipping CSS
(`@property`, `margin-trim` and similar); they are reported as ignored rather than
suppressed, so the exclusion stays visible. Real errors must be zero.
