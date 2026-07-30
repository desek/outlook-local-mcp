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
import { readFileSync, writeFileSync } from 'node:fs'
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

writeFileSync(indexPath, injected, 'utf8')
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
