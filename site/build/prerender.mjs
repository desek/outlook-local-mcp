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

const HERO_FONTS = ['inter-latin-400-normal', 'inter-latin-600-normal', 'inter-latin-700-normal']
const DOC_FONTS = ['inter-latin-400-normal', 'geist-mono-latin-400-normal']

writeFileSync(indexPath, optimisePage(injected, 'index.html', HERO_FONTS), 'utf8')

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
