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

import { readFile, access } from 'node:fs/promises'
import { join } from 'node:path'
import { puppeteer } from './site.puppeteer.mjs'
import { serve } from './site.serve.mjs'
import { PAGES } from './site.pages.mjs'

const CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'
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
  'index.html': 11853,
  'quickstart.html': 7390,
  'concepts.html': 15582,
  'troubleshooting.html': 17468,
}

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

console.log(failures.length === 0 ? 'content-check: all assertions hold' : `content-check: ${failures.length} failing`)
process.exit(failures.length === 0 ? 0 : 1)
