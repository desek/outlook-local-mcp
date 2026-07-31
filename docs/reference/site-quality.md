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

### The localhost measurement overstates the landing page's LCP

Measured locally, the landing page's LCP is a linear function of bundle size: the library
floor — React, ReactDOM, GSAP, ScrollTrigger and Lenis, hydrating a component that returns
one empty `<div>` — is **108.79 KB gzipped**, or 590 ms, and the whole shipped bundle is
137.6 KB, of which the site's own code is 29 KB and the five capability diagrams 9.3 KB.
With no script element at all the page audits LCP 0.98 at 1,803 ms; a 20-byte stub and a
4.5 KB vanilla layer both score 0.99.

**Do not conclude from that arithmetic that 100 is unreachable.** It was concluded once,
and CI disproved it: the same build measures **LCP 1,660 to 1,664 ms and Performance 1.00
on all three runs** on a healthy runner, with the motion design and interactivity intact.

The difference is the instrument. Lantern charges every byte fetched before the *observed*
paint, and a local static server delivers the whole document inside about 90 ms, so the
client bundle always lands inside that window and is always charged — no matter how it is
scheduled. A real network separates the requests, the pre-render step's deferral puts the
bundle after the LCP entry, and it drops out of the graph.

So localhost systematically overstates LCP for any deferred resource, and it is the wrong
environment in which to decide that a performance target cannot be met. Take that decision
from CI. The bundle-size model above is correct *for localhost* and should be read as
describing it.

### What CI can and cannot measure here

CI does *not* simply score lower than a developer machine, which is what was assumed
before the gate had ever run there. Measured across four runs on GitHub's runners:

| page | CI Performance | CI TBT | CI LCP |
|---|---|---|---|
| index.html, runner at `benchmarkIndex` ≈ 3,100 | **1.00 / 1.00 / 1.00** | 45 to 53 ms | 1,660 to 1,664 ms |
| index.html, runner at 2,100 to 2,500 | 0.66 to 0.97 | 193 to 1,675 ms | 1,658 to 2,108 ms |
| the three documentation pages | 1.00 every run | 0 ms every run | 1,506 to 1,665 ms |

The documentation pages are stable to the millisecond. The landing page is not: Total
Blocking Time varies thirtyfold on *identical commits*, and because TBT carries 30% of the
Performance weight, the category follows it from 0.66 to 1.00. The spread tracks the
runner's `benchmarkIndex` almost exactly, so it is contention, not the site.

Two contributing effects, neither of which a visitor experiences. The first of each three
runs is systematically the worst on every metric, a cold start the median only partly
absorbs. And on the slower runners TBT reaches the hundreds of milliseconds against 8 to
12 ms locally, far beyond what a 15% hardware difference explains, which points at
software rendering: a runner with no GPU pushes this page's canvas and compositing work
onto the main thread.

So the landing page's Performance category and TBT are **not asserted**. A gate that
varies eightfold on unchanged input cannot discriminate a regression. What is asserted
there is what is stable: Accessibility, Best Practices and SEO at 100, plus LCP and CLS.
Its Performance score is still a number worth measuring — it is simply measured locally
and recorded in the CR, not gated in CI.

Note that the landing page's LCP is *better* on CI (≈1,660 ms) than locally (≈2,553 ms),
because the deferred bundle falls outside Lantern's LCP graph there. The 2,700 ms budget
is the local ceiling, which is the stricter of the two environments.

### Attribute a CI failure before recalibrating it

A local number and a CI number disagreeing does not make CI wrong. The first CI run of
this gate failed on a `concepts.html` Cumulative Layout Shift of 0.016 that measures 0.000
locally, and it was a **real defect**: the documentation pages preloaded only the 400
weights while rendering bold prose and bold inline code above the fold, so `inter-700` and
`geist-mono-600` arrived late. It hid on macOS only because the fallback font's metrics
happen to place the swap in nearly the same position. Chrome named both files in the
`layout-shifts` audit.

The discriminator is variance, and it is reliable:

* **Identical across runs → a defect.** The CLS was `0.01601972601202666` three times.
* **Swinging across runs → the environment.** TBT was 1,197 / 193 / 689 ms.

Download the `lighthouse-reports` artifact and read the audit's own attribution before
touching a threshold. That artifact only exists because `include-hidden-files: true` is
set on the upload step — `.lighthouseci` is a dotted directory, and without it the step
reports success and uploads nothing, which is how the very first failure left no evidence.

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

## The dependency gate measures what ships, not what builds

`site.yml` runs `pnpm --dir site audit --prod --audit-level=moderate` before the build,
and fails the job on any finding. The scope is deliberate and is the whole point of the
gate, so it is worth being precise about what the instrument does and does not look at.

`--prod` audits only the packages resolved from `dependencies` in `site/package.json`,
which at the time of writing is seven runtime packages, plus their transitive closure. It
is exactly the tree whose code is bundled and pre-rendered into `dist/` and published to
`gh-pages`. `--audit-level=moderate` sets the failure floor: a low-severity advisory is
reported but does not fail the job, moderate and above does.

It **excludes** the entire `devDependencies` closure. That is where the toolchain lives
(`@lhci/cli`, `vite`, `puppeteer-core`, the TypeScript compiler) and it is where the site's
open advisories have historically landed, because a Lighthouse CLI drags in a large
transitive tree that accretes advisories no visitor ever executes. Those findings are real,
but they are build-time only: `lhci autorun` runs after the build against its output and
contributes no byte to `dist/`. The gate does not look at them, on purpose.

### Why the noisy tree is watched by Dependabot and the narrow one by CI

The build-time tree is not unwatched. The `npm` ecosystem entry in
`.github/dependabot.yml` covers `/site` in full, dev dependencies included, and that is the
correct home for it, because the two instruments are given deliberately different scopes.

The CI audit is a **blocking** gate: a finding stops a deploy. A blocking gate must have a
near-zero false-alarm rate or it does not survive contact with someone trying to ship on a
Friday, and it acquires `continue-on-error` within a month. Restricting it to the runtime
tree, which is small, changes rarely, and is clean today, is what buys it that property.

Dependabot on the full tree is a **notification**, not a gate. Lower precision is
acceptable there because the cost of an over-report is a pull request someone reads and
closes, not a blocked deploy. Giving the noisy instrument the wide scope and the blocking
instrument the narrow one is what keeps both alive. Widening the CI audit to the full tree
would fail the job today for advisories nobody will act on, which is the failure mode that
gets quality gates disabled rather than fixed.

### The standing rule: moving a package into `dependencies` enters it into the gate

The entire boundary rests on the vulnerable packages being confined to `devDependencies`,
and nothing in `package.json` enforces that placement. Moving one line from
`devDependencies` to `dependencies` brings that package, and everything it pulls in, into
the shipped tree and therefore into the `--prod` audit. That is the intended behaviour, not
an accident: the gate is the enforcement that the boundary still holds, so a package that
crosses the line is audited from that commit forward, and the exclusion cannot silently
outlive its justification. A contributor who narrows or widens the gate's scope, or raises
its threshold, records the reason here rather than reaching for `continue-on-error`.

## Two dependency traps found by measurement, not by reading docs

Both of these were established by running the tooling during CR-0072 and watching what it
actually did, which is the standard the rest of this document is written to.

### pnpm 11 does not read overrides from `package.json`

The three site advisories were closed with `pnpm.overrides`, forcing patched transitive
versions upstream never shipped. The trap is where the override block lives. pnpm 11.18.0,
the version pinned in `packageManager`, does **not** read a `pnpm` field from
`package.json`: it emits `[WARN] ... "pnpm.overrides" ... ignored` and carries on. An
override placed there leaves the lockfile fully vulnerable while every surface reading
suggests it was applied, which is the worst kind of null change, one that looks like a
result. The overrides live in `site/pnpm-workspace.yaml`, alongside the pnpm 11 settings
this repo already keeps there (`onlyBuiltDependencies`). Confirm an override took by
grepping the resolved version out of `site/pnpm-lock.yaml`, not by trusting that
`package.json` looks right.

### `puppeteer-core` is load-bearing, and a `site/`-scoped search says otherwise

`puppeteer-core` is a `devDependency` pinned to an exact version with no caret, and a
search restricted to `site/` and `.github/` finds no importer and concludes it is an
orphan that can be removed or bumped freely. That conclusion is wrong, and CR-0072
established it is wrong by looking wider. Three harness scripts import it through a deep
internal ESM path:

* `.agents/scripts/site.screenshot.mjs`
* `.agents/scripts/site.content.check.mjs`
* `.agents/scripts/site.contrast.audit.mjs`

They reach into `.../puppeteer-core/lib/esm/puppeteer/puppeteer-core.js`, an internal
layout with no cross-major compatibility guarantee, which is why the pin is exact and why
the 24.x to 25.x major bump is out of scope. `site.screenshot.mjs` is the very harness this
document opens with, the one the visual gate depends on. The pin is not cargo cult; a
search that stopped at `site/` simply missed the consumers, and that is worth recording
because the missed search is the lesson.

## The dependency-currency noise floor

CR-0072 Phase 1 baselined the Lighthouse gate on the unchanged tree by running the full
build-and-`lighthouse` sequence twice, so the currency and override bumps could be judged
against a known spread rather than against a single number. The measured spread was tight:
category scores identical to `0.00` across both runs, and LCP within `2 ms`.

Read that floor with the same suspicion this document applies to every other instrument.
Two runs of a median-of-3 is a **coarse** floor: with six underlying runs it barely samples
the runner variance, and most of the apparent tightness comes from `lhci`'s own
median-of-3 smoothing rather than from low per-run variance. It is enough to say a
post-change score inside `0.00` is not a regression; it is not enough to characterise the
runner. The wider CI variance measured on the landing page above, TBT swinging thirtyfold
on identical commits, is the honest picture of per-run spread, and it is why the landing
page's Performance category is not asserted at all.

The discriminator from the CI section applies unchanged here: a metric that is **identical
across runs and still fails** is a defect in the site, while one that **swings** is the
environment. A currency bump that moves a score by more than the baseline spread is a
signal to investigate; one that moves it by less is "not measured", neither confirmed
unchanged nor confirmed regressed.
