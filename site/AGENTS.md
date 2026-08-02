# AGENTS.md — website

Instructions for working in `site/`. The repository-root `AGENTS.md` still applies; this
file adds what is specific to the website and overrides nothing.

`CLAUDE.md` in this directory is a symlink to this file.

## What this is

A pre-rendered marketing and documentation site, built with Vite, React and Tailwind v4,
published to the `gh-pages` branch by CI and served from the apex `outlook-local-mcp.com`.
It is a set of separate HTML entries, not a single-page application and not a router: the
landing page plus three documentation pages generated at build time from `docs/*.md`.

```bash
pnpm --dir site install --frozen-lockfile
pnpm --dir site run dev          # Vite dev server
pnpm --dir site run build        # tsc -b, client build, SSR build, prerender
pnpm --dir site run lighthouse   # the gate; thresholds in lighthouserc.json
```

`build` is four steps and the last two matter most: `entry-server.tsx` renders the
landing page with `react-dom/server`, and `build/prerender.mjs` injects that markup into
`dist/index.html`, inlines the stylesheet, adds font preloads, and emits `/index.md`.

## Invariants

These are the ones with a history of being broken. Each fails the build or the gate.

* **The published root must never be empty.** The pre-render exists so the page's content
  is in the HTML without JavaScript. `prerender.mjs` exits non-zero rather than publish a
  document whose `<div id="root">` is empty; do not weaken that check. Nothing may be
  awaited before hydration for the same reason — an earlier version awaited a mock-service
  worker before rendering, and a registration failure left the live site blank.
* **Content must survive with JavaScript disabled.** Prose, headings, one `<h1>` per page,
  canonical tags and the crawler files are the product here. Verify with
  `node .agents/scripts/site.content.check.mjs` before claiming a change is safe.
* **`base` is `/`, not a subpath.** The site is served from a custom apex domain. A
  project-page base prefix makes every asset 404 at the root, which has taken the live
  site down once.
* **`gh-pages` is a CI-managed artifact.** Never hand-edit it. `CNAME` and the crawler
  files are build outputs inside `dist/`, so a rebuild cannot revert them.

## The design is adopted as authored

[`ARCHITECTURE.md`](ARCHITECTURE.md) is the design language and per-section intent of the
page. Layout, typography and colour changes are deferred **except where a measured
accessibility or validity threshold requires one, and then only by the smallest adjustment
that reaches the threshold.** Fix contrast on the tokens the design system already defines
rather than inventing new ones; `--color-lime-dark` exists for small text on light
backgrounds.

## Changes are measured, not eyeballed

Read [`../docs/reference/site-quality.md`](../docs/reference/site-quality.md) before any
performance work. It records what Lighthouse's simulated throttling does and does not
measure, the LCP-versus-bundle-size model, why the landing page is held to 96 while the
documentation pages are held to 100, the five clocks that must be frozen for a reproducible
screenshot, and the optimisations that look lossless and are not. Most of the obvious
levers were measured and move nothing; that document says which, so they are not
re-attempted.

The harness is `.agents/scripts/site.*.mjs` at the repository root. Each script carries its
purpose and caveats in its own docstring.

* **A claim that rendering is unchanged is asserted, not assumed.** Any change said to
  leave rendering untouched **MUST** be verified with a screenshot comparison against a
  build of the commit it is compared to. Repairs and regressions look identical in a diff
  until someone opens the tiles — the fix that gave every SVG instance its own ids
  "regressed" nine tiles, and the tiles were the repair.
* **The thresholds in `lighthouserc.json` are a measured ceiling, not an aspiration.**
  Raising one requires the measurement showing it is reachable. Lowering one requires
  saying so in the governing CR rather than editing the file quietly.
* **Attribute a CI failure before recalibrating it.** Local and CI numbers differ, and the
  difference is not automatically the runner's fault — the first CI run of this gate
  failed on a Cumulative Layout Shift that measured 0.000 locally and turned out to be a
  real defect on every platform whose fallback font differs from macOS. The rule is:
  download the `lighthouse-reports` artifact, read the audit's own attribution, and only
  then decide. A metric that is identical across runs is a defect; one that swings is the
  environment. Do not reason from "it passes on my machine".
* **Both accessibility and validity are gated.** `site.contrast.audit.mjs` walks every
  rendered text node at four widths; `site.validate.mjs` posts each page to the W3C Nu
  checker and separates real errors from the checker's CSS-profile lag. Real errors and
  contrast failures must both be zero.

## Dependencies

pnpm 11.18.0 does **not** read a `pnpm` field from `package.json`. Overrides and pnpm
settings live in `site/pnpm-workspace.yaml`, and an override placed in `package.json` is
silently ignored while appearing to have been applied. Confirm an override took by
grepping the resolved version out of `site/pnpm-lock.yaml`, not by trusting that
`package.json` looks right. That trap, the `puppeteer-core` pin being load-bearing despite
a `site/`-scoped search finding no importer, and the production-tree audit gate `site.yml`
runs are documented in
[`../docs/reference/site-quality.md`](../docs/reference/site-quality.md).

## CI

Two workflows, split by path so neither the site nor the Go application pays for the
other's build:

* `site.yml` — pull requests touching `site/**` or `docs/**`. Runs a production
  dependency audit (`pnpm --dir site audit --prod --audit-level=moderate`) before the
  build, then builds, exercises the `site.content.check.mjs` harness against the built
  `site/dist` (so a `puppeteer-core` bump that breaks the harness import fails the check
  rather than shipping silently, per CR-0074), and runs the Lighthouse gate, uploading
  the reports as an artifact on failure as well as success. The harness step sets
  `CHROME_PATH` to the runner's pre-installed Chrome; the script defaults to the local
  macOS bundle, so it runs unchanged on a developer's Mac.
* `deploy-site.yml` — push to `main`. The same build and gate, then publishes `dist/` to
  `gh-pages`. The gate runs *before* the publish step, so a regression blocks the deploy.
* `ci.yml` — the Go pipeline; ignores site-only paths.

`docs/**` triggers both site and application workflows, because those Markdown files are
embedded into the binary and also generate this site's documentation pages.
