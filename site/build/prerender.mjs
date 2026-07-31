/**
 * prerender.mjs - post-build pre-render of the landing page.
 *
 * Runs after both the client build (dist/) and the SSR build (dist-ssr/). It imports
 * the SSR-compiled render function, produces the landing-page HTML with react-dom/server,
 * and injects it into the empty <div id="root"> that the client build emits, so the
 * published document carries the full text without JavaScript (CR-0070 FR-8, FR-9).
 *
 * It is a plain .mjs script, not TypeScript, so Node can run it directly with no
 * transpiler: it only imports the already-compiled SSR bundle (JavaScript) and touches
 * the filesystem. It fails loudly (non-zero exit) rather than publish a document whose
 * root is empty (FR-12), which is the exact failure the pre-render exists to prevent.
 *
 * @agents-index Post-build script: injects the react-dom/server render into dist/index.html, failing the build if the root would be empty.
 */
import { readFileSync, writeFileSync, readdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'

const here = dirname(fileURLToPath(import.meta.url))
const siteRoot = resolve(here, '..')
const indexPath = resolve(siteRoot, 'dist/index.html')
const indexMdPath = resolve(siteRoot, 'dist/index.md')
const ssrEntry = resolve(siteRoot, 'dist-ssr/entry-server.js')

/** Matches the empty root container the client build emits, tolerating whitespace. */
const EMPTY_ROOT_RE = /<div id="root">\s*<\/div>/

/**
 * fail prints a diagnostic and exits non-zero, so a broken pre-render aborts the build
 * instead of publishing an empty page.
 *
 * @param {string} message  The reason the pre-render cannot proceed.
 */
function fail(message) {
  console.error(`prerender: ${message}`)
  process.exit(1)
}

const { render, renderIndexMarkdown } = await import(ssrEntry).catch((err) =>
  fail(`could not load SSR bundle at ${ssrEntry}: ${err?.message ?? err}`),
)

if (typeof render !== 'function') {
  fail('SSR bundle does not export a render() function')
}

if (typeof renderIndexMarkdown !== 'function') {
  fail('SSR bundle does not export a renderIndexMarkdown() function')
}

const appHtml = render()
if (typeof appHtml !== 'string' || appHtml.trim().length === 0) {
  fail('render() produced no markup; refusing to publish an empty root')
}

let template
try {
  template = readFileSync(indexPath, 'utf8')
} catch (err) {
  fail(`could not read ${indexPath}: ${err?.message ?? err}`)
}

if (!EMPTY_ROOT_RE.test(template)) {
  fail('dist/index.html has no empty <div id="root"> to inject into; the build output changed shape')
}

const injected = template.replace(EMPTY_ROOT_RE, `<div id="root">${appHtml}</div>`)

// Final safety check: the published root must contain rendered markup (FR-12).
if (/<div id="root">\s*<\/div>/.test(injected)) {
  fail('root is still empty after injection; refusing to publish')
}

// Inline the build's stylesheet into every page and preload the fonts each one
// paints its Largest Contentful Paint element in.
//
// Both are applied to the documentation pages as well as the landing page. They were
// previously landing-page only, which left the doc pages with a render-blocking
// stylesheet request that Lighthouse mobile costed at 750 to 900 ms, and with no font
// preload at all, so their prose swapped fonts late and charged the reflow to
// Cumulative Layout Shift. Inlining is affordable now that the stylesheet is Latin
// subsets only: it dropped from 88 KB to 39 KB when the unused Cyrillic, Greek,
// Vietnamese, and extended-Latin faces were removed.
//
// Font choice per page is deliberate rather than blanket: preloading a font a page
// does not paint above the fold wastes bandwidth on the critical path. The landing
// page's LCP element is the hero <h1> in Inter; the doc pages lead with prose in Inter
// and code blocks in Geist Mono.
const assetsDir = resolve(siteRoot, 'dist/assets')

/** Resolves a content-hashed woff2 by filename stem, or null when absent. */
function findFont(stem) {
  const f = readdirSync(assetsDir).find((n) => n.startsWith(stem) && n.endsWith('.woff2'))
  return f ? `<link rel="preload" as="font" type="font/woff2" crossorigin href="/assets/${f}" />` : null
}

const STYLESHEET_LINK = /<link\b[^>]*\brel="stylesheet"[^>]*\bhref="(\/assets\/[^"]+\.css)"[^>]*>/g

/** Matches the module script tag Vite emits for a page's client entry. */
const MODULE_SCRIPT = /<script type="module"[^>]*\bsrc="(\/assets\/[^"]+\.js)"[^>]*><\/script>/

/**
 * Rewrites the landing page's module script into a bootstrap that injects it after load.
 *
 * The landing page's client bundle is 137 KB gzipped: React, GSAP, ScrollTrigger, Lenis
 * and the component tree. None of it is needed to read the page — the root is
 * pre-rendered, so the full content, styles and fonts are on screen without it — but as a
 * `<script>` in the document it is discovered during the initial parse and competes for
 * bandwidth with the HTML and the fonts that the first render actually needs.
 *
 * Measured on Lighthouse mobile: with the tag in the document the landing page's simulated
 * Largest Contentful Paint is 2,555 ms and Performance 0.96; with the script absent
 * entirely it is 1,803 ms and 1.00. The 750 ms gap is almost exactly the bundle's download
 * time at the emulated 1,475 Kbps. Neither `fetchpriority="low"` nor deferring *execution*
 * inside the module changes it, because the cost is the transfer, not the evaluation.
 *
 * So the transfer is moved behind the main content's paint. The rule the bootstrap applies
 * is causal rather than a tuned delay: it waits for the browser to report a Largest
 * Contentful Paint entry — the moment the page's main content is actually on screen — and
 * then for the first idle period, and only then requests the bundle. On a slow connection
 * that is exactly the behaviour wanted: nothing competes with the content a visitor is
 * waiting to read. On a fast one it costs a few milliseconds.
 *
 * The trade-off is real and worth stating plainly: the motion layer, the copy-to-clipboard
 * buttons, the tabs and the accordions stay inert until the injected script arrives. All of
 * the page's text, styling and layout are present throughout, because the root is
 * pre-rendered; what is delayed is behaviour, not content.
 *
 * Two fallbacks, so the page cannot end up permanently inert: `load` triggers the same path
 * for browsers without LCP reporting, and a timeout bounds the idle wait.
 *
 * @param html  The page HTML, after CSS inlining.
 * @param pageName  The page being transformed, for diagnostics.
 * @returns The HTML with the script tag replaced by the bootstrap.
 */
function deferClientScript(html, pageName) {
  const match = html.match(MODULE_SCRIPT)
  if (!match) {
    fail(`no <script type="module"> found in dist/${pageName} to defer; the build output changed shape`)
  }
  const bootstrap =
    `<script>(function(){var done=false;` +
    `var load=function(){if(done)return;done=true;` +
    `var s=document.createElement('script');s.type='module';s.crossOrigin='anonymous';` +
    `s.src=${JSON.stringify(match[1])};document.head.appendChild(s)};` +
    `var soon=function(){('requestIdleCallback'in window)?requestIdleCallback(load,{timeout:2000}):setTimeout(load,200)};` +
    `try{new PerformanceObserver(function(list,obs){if(list.getEntries().length){obs.disconnect();soon()}})` +
    `.observe({type:'largest-contentful-paint',buffered:true})}catch(e){}` +
    `addEventListener('load',soon,{once:true});setTimeout(soon,3000)})()</script>`
  return html.replace(MODULE_SCRIPT, bootstrap)
}

/**
 * Inlines the stylesheet and injects the given font preloads into one page.
 * Returns the transformed HTML. Fails the build if a referenced stylesheet cannot
 * be read, since shipping a page with no styles is worse than not shipping.
 */
function optimisePage(html, pageName, fontStems) {
  let inlined = 0
  const withCss = html.replace(STYLESHEET_LINK, (_m, href) => {
    let css
    try {
      css = readFileSync(resolve(siteRoot, `dist${href}`), 'utf8')
    } catch (err) {
      fail(`could not read stylesheet ${href} to inline it into ${pageName}: ${err?.message ?? err}`)
    }
    inlined += 1
    return `<style>${css}</style>`
  })
  if (inlined === 0) {
    fail(`no local <link rel="stylesheet"> found in dist/${pageName} to inline; the build output changed shape`)
  }
  const preloads = fontStems.map(findFont).filter(Boolean).join('\n    ')
  return preloads ? withCss.replace('</title>', `</title>\n    ${preloads}`) : withCss
}

// The landing page's above-the-fold band is the hero: the <h1> and subhead in Inter, and
// the badge row, install command and nav button in Geist Mono. Lighthouse attributed the
// whole of the page's 0.094 Cumulative Layout Shift to three fonts arriving late —
// inter-500, geist-mono-400 and geist-mono-600 — none of which this list previously named,
// while it preloaded inter-600 and inter-700, which the hero does not paint at all.
const HERO_FONTS = [
  'inter-latin-400-normal',
  'inter-latin-500-normal',
  'geist-mono-latin-400-normal',
  'geist-mono-latin-600-normal',
]
// The documentation pages lead with prose in Inter and inline code in Geist Mono, and
// both appear in bold above the fold: `**text**` and `**`code`**` are ordinary in these
// documents. The bold faces were missing from this list, which cost `concepts.html` a
// Cumulative Layout Shift of 0.016 — the same defect as the landing page's, on the page
// set that was not re-checked after the landing page was fixed.
//
// It did not reproduce on macOS, where the fallback font's metrics happen to make the
// swap land in nearly the same place, and was only visible on a Linux CI runner. Chrome
// named both files outright in the `layout-shifts` audit; the local zero was the
// misleading measurement, not the CI figure.
const DOC_FONTS = [
  'inter-latin-400-normal',
  'inter-latin-700-normal',
  'geist-mono-latin-400-normal',
  'geist-mono-latin-600-normal',
]

// Only the landing page's script is deferred. The documentation pages' entry is 26 bytes
// (it exists solely so Vite bundles the stylesheet for them), so there is nothing to move.
writeFileSync(
  indexPath,
  deferClientScript(optimisePage(injected, 'index.html', HERO_FONTS), 'index.html'),
  'utf8',
)

for (const page of ['concepts.html', 'quickstart.html', 'troubleshooting.html']) {
  const path = resolve(siteRoot, `dist/${page}`)
  let html
  try {
    html = readFileSync(path, 'utf8')
  } catch (err) {
    fail(`documentation page dist/${page} is missing: ${err?.message ?? err}`)
  }
  writeFileSync(path, optimisePage(html, page, DOC_FONTS), 'utf8')
}

console.log(`prerender: injected ${appHtml.length} chars into dist/index.html`)

// Emit the Markdown representation of the landing page (FR-31 to FR-33). It is derived
// from the same render as the HTML above, so the two cannot describe different content.
const indexMd = renderIndexMarkdown()
if (typeof indexMd !== 'string' || indexMd.trim().length === 0) {
  fail('renderIndexMarkdown() produced no Markdown; refusing to publish an empty /index.md')
}
if (!/```mermaid/.test(indexMd)) {
  fail('index.md carries no Mermaid fences; the SVG diagrams were not converted (FR-33)')
}
writeFileSync(indexMdPath, indexMd, 'utf8')
console.log(`prerender: wrote ${indexMd.length} chars to dist/index.md`)
