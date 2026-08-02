#!/usr/bin/env node
/**
 * @agents-index Assert the built site has not regressed in crawler-visible content.
 *
 * Purpose:
 *   Most of CR-0070's value is content a crawler can read without executing JavaScript.
 *   The performance and validity work in this iteration is licensed to change markup and
 *   hydration freely, which makes it entirely possible to improve a Lighthouse score by
 *   deleting the very content the CR exists to publish. This script is the guard against
 *   that: it re-measures, on every candidate build, the properties the CR promised.
 *
 *     - text without JavaScript, per page, against the pre-change floor
 *     - exactly one `<h1>` per page
 *     - every `SeeDocs` anchor in the Go verb registry resolves to an id in the built docs
 *     - the six crawler files exist
 *     - `/index.md` still carries its Mermaid diagrams
 *     - no bare tool-surface figure is transcribed into a source file (CR-0073)
 *     - the served landing page presents verbs as operation values, not flat tool
 *       names, and names no domain the manifest does not have (CR-0073)
 *
 * Usage:
 *   node .agents/scripts/site.content.check.mjs [dist-dir]
 *
 * Parameters:
 *   dist-dir  Built site to check. Defaults to `site/dist`.
 *
 * Side effects: launches headless Chrome with JavaScript disabled and binds a local port.
 * Exits 0 when every assertion holds, 1 otherwise.
 */

import { readFile, access, readdir } from 'node:fs/promises'
import { join } from 'node:path'
import { puppeteer } from './site.puppeteer.mjs'
import { serve } from './site.serve.mjs'
import { PAGES } from './site.pages.mjs'

// Chrome binary to drive. Defaults to the macOS Google Chrome bundle so local runs on a
// developer machine need no configuration. CI (ubuntu-latest) has no such bundle, so the
// workflow exports CHROME_PATH pointing at the runner's pre-installed Chrome; the override
// keeps local usage unchanged while letting the same script run in CI.
const CHROME = process.env.CHROME_PATH ?? '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'
const distDir = process.argv[2] ?? 'site/dist'

/**
 * Minimum no-JavaScript text length per page. A candidate build may publish more prose
 * but never less.
 *
 * These floors were measured on the pre-change build (commit 036219a) with the method this
 * script uses: `document.body.textContent.length` with JavaScript disabled.
 *
 * The CR-0070 iteration ledger quotes a different set — 12,360 / 15,534 / 7,062 / 17,452 —
 * and the difference was chased down rather than left as an unexplained mismatch. Three of
 * the four are `innerText` measurements, and the current build exceeds all three:
 * concepts 15,535 against 15,534, troubleshooting 17,501 against 17,452, quickstart 7,383
 * against 7,062.
 *
 * The landing page's 12,360 cannot be reproduced by any method on either build:
 * `innerText` gives 6,536 and `textContent` 11,853. `innerText` is much lower there
 * because the page renders desktop and mobile branches of each section and hides one with
 * `display: none`, which `innerText` correctly omits and a crawler reading the markup does
 * not. The quoted figure most likely predates a copy change.
 *
 * Hence `textContent`, and hence floors this script can reproduce from the unmodified
 * build. That is the assertion that actually matters — no prose was dropped — and it is
 * measured the same way on both sides of the comparison.
 */
const TEXT_FLOOR = {
  'index.html': 11714,
  'quickstart.html': 7390,
  'concepts.html': 15582,
  'troubleshooting.html': 17468,
}

/**
 * The landing-page floor was re-baselined by CR-0073, from 11,853 to 11,714, a reduction
 * of 139 characters measured on the corrected build with this script's own method
 * (`document.body.textContent.length`, JavaScript disabled).
 *
 * The reduction is accounted for in full by the removal of the obsolete flat tool-name
 * inventory. CR-0073 replaced the hand-written flat tool names the landing page rendered
 * (the long `calendar_list_events` form, shown in the capability cards and the capability
 * SVG diagrams) with the shorter `operation` values the aggregate-tool interface actually
 * exposes, read from the generated surface manifest. Shorter names over the always-rendered
 * capability content are a net loss of characters even though a few more verbs are now
 * listed; no prose section was shortened or deleted. The other three page floors did not
 * move, because CR-0073 changed no content outside the landing page's tool-surface figures.
 */

/** Crawler-facing files the CR requires the build to emit. */
const CRAWLER_FILES = ['robots.txt', 'sitemap.xml', 'llms.txt', 'index.md', 'CNAME', 'build-info.json']

/** Minimum Mermaid fences in the generated Markdown representation of the landing page. */
const MIN_MERMAID_FENCES = 5

const failures = []

/**
 * Record a failed assertion.
 *
 * @param {string} message Human-readable description of what did not hold.
 */
function fail(message) {
  failures.push(message)
  console.log(`FAIL ${message}`)
}

const site = await serve(distDir)
const browser = await puppeteer.launch({ executablePath: CHROME, headless: true })

try {
  for (const page of PAGES) {
    const tab = await browser.newPage()
    await tab.setJavaScriptEnabled(false)
    await tab.goto(`${site.origin}/${page}`, { waitUntil: 'domcontentloaded', timeout: 60_000 })
    // `textContent`, not `innerText`: a crawler reads the serialised text of the markup,
    // including text the visual layout happens to clip or collapse. It is also the
    // measurement the pre-change floors below were taken with.
    const { text, headings } = await tab.evaluate(() => ({
      text: document.body.textContent.length,
      headings: document.querySelectorAll('h1').length,
    }))
    await tab.close()

    const floor = TEXT_FLOOR[page]
    if (text < floor) fail(`${page}: text without JavaScript ${text} < floor ${floor}`)
    else console.log(`ok ${page}: ${text} chars without JavaScript (floor ${floor})`)

    if (headings !== 1) fail(`${page}: ${headings} <h1> elements, expected exactly 1`)
  }
} finally {
  await browser.close()
  await site.close()
}

// SeeDocs anchors: every `slug#anchor` the Go registry publishes must exist in the built page.
const anchors = await collectSeeDocsAnchors()
let resolved = 0
for (const reference of anchors) {
  const [slug, anchor] = reference.split('#')
  const html = await readFile(join(distDir, `${slug}.html`), 'utf8').catch(() => '')
  if (html.includes(`id="${anchor}"`)) resolved++
  else fail(`SeeDocs anchor does not resolve: ${reference}`)
}
console.log(`ok SeeDocs: ${resolved}/${anchors.length} anchors resolve`)

for (const file of CRAWLER_FILES) {
  try {
    await access(join(distDir, file))
  } catch {
    fail(`crawler file missing: ${file}`)
  }
}
console.log(`ok crawler files: ${CRAWLER_FILES.length} expected`)

const indexMd = await readFile(join(distDir, 'index.md'), 'utf8').catch(() => '')
const fences = (indexMd.match(/```mermaid/g) ?? []).length
if (fences < MIN_MERMAID_FENCES) fail(`/index.md has ${fences} Mermaid fences, expected >= ${MIN_MERMAID_FENCES}`)
else console.log(`ok /index.md: ${fences} Mermaid fences`)

// Claims assertion (CR-0073 FR-24): the site holds no figure of its own. A bare numeric
// claim about tools, verbs, domains, or configuration variables in a source file under
// site/src is a transcribed figure that the generated manifest already owns; it is
// rejected here, named by file and line, so a rename in the code cannot leave a stale
// number on the page. Manifest-derived counts reach the page as `${expr}` interpolations,
// which carry no literal digit and so never match.
await assertNoBareClaims('site/src')

// Tool-surface shape assertion (CR-0073 AC-5, AC-6): the served landing page must present
// verbs as `operation` values of the four aggregate tools, never as flat top-level tool
// names, and must not name a domain the server does not have. The obsolete flat names are
// derived from the manifest itself (every `domain_verb` concatenation), so a verb added or
// renamed in the code is covered without editing this script.
await assertToolSurfaceShape(join(distDir, 'index.html'))

/**
 * Read the distinct `slug#anchor` documentation references declared by the Go verb registry.
 *
 * Parsing the Go source directly (rather than restating the list here) means a verb that
 * adds or renames a reference is covered without editing this script.
 *
 * @returns {Promise<string[]>} Sorted distinct references.
 */
async function collectSeeDocsAnchors() {
  const { execFile } = await import('node:child_process')
  const { promisify } = await import('node:util')
  const { stdout } = await promisify(execFile)('grep', ['-rho', '--include=*.go', '"[a-z-]*#[a-z0-9-]*"', 'internal/'])
  const found = new Set()
  for (const line of stdout.split('\n')) {
    const value = line.replace(/"/g, '').trim()
    if (/^(concepts|quickstart|troubleshooting|readme)#[a-z0-9-]+$/.test(value)) found.add(value)
  }
  return [...found].sort()
}

/**
 * Fail on any bare numeric claim about the tool surface reintroduced under a source tree.
 *
 * Walks every `.ts`/`.tsx` file beneath `root`, excluding the generated manifest, and
 * matches a number immediately followed (within a few words) by tool, verb, domain, or
 * variable. Each match is reported with its file and 1-indexed line so the author is sent
 * straight to the transcribed figure. The generated `src/generated/` tree is skipped: it is
 * the manifest, the one place a number about the surface is allowed to live.
 *
 * @param {string} root Source directory to scan, relative to the process working directory.
 */
async function assertNoBareClaims(root) {
  // A number, then up to three intervening words, then the noun a surface claim is about.
  const claimRe = /\b\d+\s+(?:[A-Za-z][A-Za-z-]*\s+){0,3}(?:tools?|verbs?|domains?|variables?)\b/i
  let scanned = 0
  let flagged = 0
  for await (const file of walkSources(root)) {
    scanned++
    const source = await readFile(file, 'utf8')
    source.split('\n').forEach((line, i) => {
      const match = claimRe.exec(line)
      if (match) {
        flagged++
        fail(`bare tool-surface claim "${match[0].trim()}" at ${file}:${i + 1} (derive it from src/generated/surface.json)`)
      }
    })
  }
  if (flagged === 0) console.log(`ok claims: no bare tool-surface figure in ${scanned} source files under ${root}`)
}

/**
 * Yield every `.ts`/`.tsx` file beneath a directory, skipping the generated manifest tree.
 *
 * @param {string} dir Directory to walk.
 * @returns {AsyncGenerator<string>} Absolute-or-relative file paths, matching the input base.
 */
async function* walkSources(dir) {
  const entries = await readdir(dir, { withFileTypes: true })
  for (const entry of entries) {
    const path = join(dir, entry.name)
    if (entry.isDirectory()) {
      if (entry.name === 'generated') continue
      yield* walkSources(path)
    } else if (/\.(ts|tsx)$/.test(entry.name)) {
      yield path
    }
  }
}

/**
 * Fail when the served landing page presents the tool surface with the wrong model.
 *
 * Two properties are asserted against the built HTML. First, no obsolete flat tool name
 * appears: the obsolete names are every `domain_verb` concatenation the manifest implies,
 * so the check tracks the code rather than a hand-kept denylist. Second, no domain outside
 * the manifest's four is named as a category, checked against the specific obsolete label
 * ("Diagnostics") CR-0073 removed. Each manifest domain name is also confirmed present, so
 * the page cannot silently drop the true surface.
 *
 * @param {string} htmlPath Path to the built landing-page HTML.
 */
async function assertToolSurfaceShape(htmlPath) {
  const html = await readFile(htmlPath, 'utf8').catch(() => '')
  if (!html) {
    fail(`tool-surface shape: could not read ${htmlPath}`)
    return
  }
  const manifest = JSON.parse(await readFile('site/src/generated/surface.json', 'utf8'))
  const domains = manifest.domains ?? []

  // Every domain_verb concatenation is an obsolete flat tool name; none may be served.
  let flatHits = 0
  for (const domain of domains) {
    for (const verb of domain.verbs ?? []) {
      const flat = `${domain.name}_${verb.name}`
      if (html.includes(flat)) {
        flatHits++
        fail(`flat tool name in served HTML: "${flat}" (present verbs as operation values of the ${domain.name} tool)`)
      }
    }
  }
  if (flatHits === 0) console.log('ok tool-surface: no flat tool name in served HTML')

  // No invented domain category. "Diagnostics" is the specific label CR-0073 removed.
  if (/\bDiagnostics\b/.test(html)) fail('served HTML names a "Diagnostics" domain, which does not exist')
  else console.log('ok tool-surface: no Diagnostics category')

  // The four true domains must each still be named on the page.
  for (const domain of domains) {
    if (!html.includes(domain.name)) fail(`served HTML does not name the ${domain.name} domain`)
  }
  console.log(`ok tool-surface: ${domains.length} domains named (${domains.map((d) => d.name).join(', ')})`)
}

console.log(failures.length === 0 ? 'content-check: all assertions hold' : `content-check: ${failures.length} failing`)
process.exit(failures.length === 0 ? 0 : 1)
