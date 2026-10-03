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
 *     - every question-form section kicker is answered by the declarative heading that
 *       follows it, so each section answers before it elaborates (CR-0073 AC-11)
 *     - the trust anchor pages resolve at their extensionless URLs (CR-0077 AC-1)
 *     - the footer crawl path is intact in both directions, by href (CR-0077 AC-3)
 *     - each page's canonical names its own URL and it carries exactly the schema.org
 *       entities its registry key declares, and none of the landing page's (CR-0077 AC-2a)
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
  'index.html': 12021,
  'quickstart.html': 7390,
  'concepts.html': 15582,
  'troubleshooting.html': 17468,
  'about/index.html': 1845,
  'contact/index.html': 1719,
  'privacy/index.html': 2729,
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
 *
 * The landing-page floor was then raised again by CR-0073 phase 4, from 11,714 to 11,942,
 * an increase of 228 characters measured on the corrected build with this script's own
 * method. Phase 4 corrected the prose rather than a figure: the privacy section's outbound
 * claim was restated accurately (naming Microsoft's endpoints plus the optional telemetry
 * endpoint, and acknowledging the loopback port that interactive sign-in binds, per FR-18
 * and FR-19), and three landing section kickers were rephrased as questions their sections
 * answer (FR-20, FR-21). The net is more prose, not less, so the floor rises to lock the
 * added content in as the new minimum. The other three page floors did not move, because
 * phase 4 touched no content outside the landing page.
 *
 * The landing-page floor was raised a fourth time by CR-0077 phase 6, from 12,002 to
 * 12,021, an increase of 19 characters measured on a clean build with this script's own
 * method. The increase is accounted for in full and exactly by phase 4's footer nav
 * (FR-6a), which adds three link labels to the always-rendered bottom bar: "About" (5) plus
 * "Contact" (7) plus "Privacy" (7) is 19 characters. No prose was added or removed, so the
 * floor rises only by the labels, and it rises rather than staying put because the links
 * are the crawl path to the trust anchor pages: a build that drops them must fail here.
 *
 * The landing-page floor was raised a third time by the CR-0073 iteration session, from
 * 11,942 to 12,002, an increase of 60 characters. The session replaced the count-led
 * wording a visitor meets first with outcome-led wording: the hero stat now names what the
 * server reaches rather than how many verbs it registers, the capability section answers
 * its question with what the reader can ask for, and the tools-reference entry point
 * promises the reference rather than counting it. The counts survive inside the expanded
 * reference, still read from the manifest. The replacement prose is marginally longer than
 * the figures it replaced, so the floor rises to lock it in. The other three page floors
 * did not move, because the session touched no content outside the landing page.
 *
 * The three trust anchor page floors are first baselines, not re-baselines. CR-0077 phase 5
 * enrolled the pages in this check with no floor at all, so `text < undefined` was false and
 * they reported ok while nothing was enforced on them. Phase 6 measured each on a clean
 * build with this script's own method and recorded the measured value as the floor: about
 * 1,845, contact 1,719, privacy 2,729. The instrument was validated before the figures were
 * trusted, by running it twice over the unchanged build; both runs returned identical
 * lengths for all seven pages, so the noise floor is 0 characters and any later shortfall is
 * a real loss of content rather than measurement drift.
 *
 * These pages are short, and that is precisely what makes their floors load-bearing. Their
 * whole purpose is to be legible to an agent that executes no JavaScript, and each is small
 * enough that a prerender or template regression could reduce it to a near-empty shell
 * without looking obviously broken. Privacy is the largest of the three because it
 * enumerates what leaves the machine; contact is the smallest because it is one addressable
 * route to a human.
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

// The published-page registry, read from the build's own source rather than restated here
// (CR-0077 AC-2a, AC-3). Every page's output file, canonical path, and assigned schema.org
// entities are the build's to declare; deriving them means a page added, renamed, or
// re-keyed there is covered by the assertions below without editing this script.
const REGISTRY = await readPageRegistry()
const SITE_ORIGIN = await readSiteOrigin()
/** Top-level entity types the landing page owns; no other page may assert them (AC-2a). */
const LANDING_ENTITIES = REGISTRY.find((p) => p.key === 'index')?.entities ?? []

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

  // Answer-first sections assertion (CR-0073 AC-11): every question-form section kicker on
  // the landing page must be answered by the declarative heading that follows it, so a
  // reader who reads only the heading gets the answer before any elaboration. The cases are
  // derived from the rendered page (every kicker whose text is a question), not a hand-kept
  // list, so a new question-form section is covered without editing this script.
  await assertAnswerFirstSections(site.origin, browser)

  // Extensionless resolution (CR-0077 AC-1): a directory-index page must answer at the
  // extensionless URL it publishes as canonical, the way GitHub Pages serves it. Driven
  // over HTTP against the same server the rest of the harness measures through, so the
  // criterion is exercised at the URL it is written about rather than at the file path.
  await assertExtensionlessPaths(site.origin)
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

// The same assertion over the trust anchor pages' Markdown sources (CR-0077 FR-7a). Those
// pages are prose, not components, so their figures would never have been seen by a scan
// restricted to `.ts`/`.tsx`; FR-7 would have had no check behind it. The rule is the
// generated manifest's, not a denylist's, so a figure typed into this prose is rejected
// here for the same reason one typed into a component is.
await assertNoBareClaims('site/content')

// Tool-surface shape assertion (CR-0073 AC-5, AC-6): the served landing page must present
// verbs as `operation` values of the aggregate domain tools, never as flat top-level tool
// names, and must not name a domain the server does not have. The obsolete flat names are
// derived from the manifest itself (every `domain_verb` concatenation), so a verb added or
// renamed in the code is covered without editing this script.
await assertToolSurfaceShape(join(distDir, 'index.html'))

// Footer crawl path (CR-0077 FR-6a, FR-6b, AC-3): the trust anchor links are the only
// route from the landing page to the trust anchor pages, and the landing link back is the
// only route out of them. A floor assertion catches their deletion but not their
// retargeting, so the hrefs themselves are asserted here, on the pre-rendered markup.
await assertFooterLinks()

// Page identity (CR-0077 AC-2a): each page's canonical must name its own URL, and no page
// but the landing page may assert the landing page's entities. The failure this guards is
// silent by construction, a path collision makes every page claim to be `/` while the
// build, the render, and the validator all stay green.
await assertPageIdentity()

/**
 * Fail when a question-form section kicker on the landing page is not answered by the
 * declarative heading that immediately follows it (CR-0073 FR-20, AC-11).
 *
 * The landing page introduces each section with a short uppercase kicker (a `text-label`
 * span). Where that kicker is phrased as a question a user asks (FR-21), the section must
 * answer it first: the very next heading must be a non-empty declarative sentence that does
 * not itself end in a question mark. The kickers are read from the rendered, JavaScript-free
 * markup, so every question-form section is checked and none has to be named here; a section
 * added with a question kicker but no answering heading fails this assertion.
 *
 * @param {string} origin Origin of the served site (for example http://127.0.0.1:PORT).
 * @param {import('puppeteer-core').Browser} browser Already-launched headless browser.
 */
async function assertAnswerFirstSections(origin, browser) {
  const tab = await browser.newPage()
  await tab.setJavaScriptEnabled(false)
  await tab.goto(`${origin}/index.html`, { waitUntil: 'domcontentloaded', timeout: 60_000 })
  const questions = await tab.evaluate(() => {
    const out = []
    for (const kicker of document.querySelectorAll('[class*="text-label"]')) {
      const q = (kicker.textContent ?? '').trim()
      if (!q.endsWith('?')) continue
      let sib = kicker.nextElementSibling
      while (sib && !/^H[1-6]$/.test(sib.tagName)) sib = sib.nextElementSibling
      const answer = sib ? (sib.textContent ?? '').trim() : ''
      out.push({ q, answer, answered: answer.length > 0 && !answer.endsWith('?') })
    }
    return out
  })
  await tab.close()

  if (questions.length === 0) {
    fail('answer-first: no question-form section kicker found on the landing page (AC-11)')
    return
  }
  let bad = 0
  for (const { q, answer, answered } of questions) {
    if (!answered) {
      bad++
      fail(`answer-first: question kicker "${q}" is not answered by a declarative heading (found "${answer}")`)
    }
  }
  if (bad === 0) {
    console.log(`ok answer-first: ${questions.length} question-form kickers each answered by a declarative heading`)
  }
}

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
 * Walks every `.ts`/`.tsx`/`.md` file beneath `root`, excluding the generated manifest, and
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
 * Yield every `.ts`/`.tsx`/`.md` file beneath a directory, skipping the generated manifest tree.
 *
 * `.md` is included so page prose is scanned as well as page components (CR-0077 FR-7a): a
 * page authored in Markdown states its claims in exactly the same way a component does, and
 * an extension filter is not a meaningful boundary for where a transcribed figure can hide.
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
    } else if (/\.(ts|tsx|md)$/.test(entry.name)) {
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

/**
 * Read the published-page registry from the build source that owns it.
 *
 * The registry is `site/build/seo.pages.ts`, and the schema.org entities each page key
 * carries are declared by `entitiesFor` in `site/build/seo.jsonld.ts`. Both are parsed
 * rather than restated, so this script asserts the pages the build actually declares.
 *
 * @returns {Promise<{key: string, file: string, path: string, entities: string[]}[]>}
 *   One entry per published page. Empty only if the parse found nothing, which is
 *   reported as a failure by the caller of the assertions.
 */
async function readPageRegistry() {
  const source = await readFile('site/build/seo.pages.ts', 'utf8')
  const entities = await readEntityTypes()
  const entries = []
  for (const m of source.matchAll(/key:\s*'([^']+)',\s+file:\s*'([^']+)',\s+path:\s*'([^']+)',/g)) {
    entries.push({ key: m[1], file: m[2], path: m[3], entities: entities[m[1]] ?? [] })
  }
  if (entries.length === 0) {
    fail('page registry: parsed no entries from site/build/seo.pages.ts (its PAGES literal shape changed; update readPageRegistry)')
  }
  return entries
}

/**
 * Map each page key to the top-level schema.org `@type` values its entities declare.
 *
 * `entitiesFor` names a builder function per page key, and each builder's first `@type`
 * is the entity it produces; both are read out of the source so a re-keyed or retyped
 * entity is picked up without editing this script.
 *
 * @returns {Promise<Record<string, string[]>>} Page key to its top-level entity types.
 */
async function readEntityTypes() {
  const source = await readFile('site/build/seo.jsonld.ts', 'utf8')
  const builders = new Map()
  for (const m of source.matchAll(/function\s+(\w+)\s*\([^)]*\)[^{]*\{[\s\S]*?'@type':\s*'(\w+)'/g)) {
    if (!builders.has(m[1])) builders.set(m[1], m[2])
  }
  const byKey = {}
  for (const m of source.matchAll(/^\s*(\w+):\s*\(\)\s*=>\s*\[([^\]]*)\],?$/gm)) {
    const types = [...m[2].matchAll(/(\w+)\s*\(/g)].map((c) => builders.get(c[1])).filter(Boolean)
    if (types.length > 0) byKey[m[1]] = types
  }
  return byKey
}

/**
 * Read the apex origin every canonical URL is built from.
 *
 * @returns {Promise<string>} The value of `SITE_ORIGIN`, or an empty string if absent.
 */
async function readSiteOrigin() {
  const source = await readFile('site/src/site.meta.ts', 'utf8')
  return /SITE_ORIGIN\s*=\s*'([^']+)'/.exec(source)?.[1] ?? ''
}

/**
 * Fail when the trust anchor crawl path is broken in either direction (CR-0077 AC-3).
 *
 * The landing page must link every directory-index page, and every generated page must
 * link those pages and the site root. The link targets are asserted, not just the labels:
 * the text floors already catch a deleted link, but an href retargeted to a wrong or
 * external URL leaves the labels in place and every other assertion green, while the
 * pages it was supposed to reach become uncrawlable.
 */
async function assertFooterLinks() {
  const anchors = REGISTRY.filter((p) => p.file.endsWith('/index.html') && p.path !== '/').map((p) => p.path)
  if (anchors.length === 0) {
    fail('footer links: the registry declares no directory-index page, so no crawl path can be asserted')
    return
  }
  let bad = 0
  for (const page of REGISTRY) {
    const html = await readFile(join(distDir, page.file), 'utf8').catch(() => '')
    if (!html) {
      bad++
      fail(`footer links: could not read ${page.file}`)
      continue
    }
    // The landing page links outward only; a generated page also links back to the root.
    const required = page.file === 'index.html' ? anchors : [...anchors, '/']
    for (const href of required) {
      if (!html.includes(`href="${href}"`)) {
        bad++
        fail(`footer links: ${page.file} carries no href="${href}" (the footer is the only crawl path to that page)`)
      }
    }
  }
  if (bad === 0) console.log(`ok footer links: ${anchors.join(', ')} linked from every page, root linked back from each generated page`)
}

/**
 * Fail when a page's canonical URL or structured-data identity is not its own (AC-2a).
 *
 * Two properties per registry page. Its canonical must name its own path, and its
 * top-level JSON-LD entities must be exactly the set its page key declares, with none of
 * the landing page's entities appearing anywhere on a subpage. The failure mode this
 * exists for is silent: a path collision in the SEO plugin leaves every page building,
 * rendering, and validating cleanly while all of them claim to be the landing page.
 */
async function assertPageIdentity() {
  let bad = 0
  for (const page of REGISTRY) {
    const html = await readFile(join(distDir, page.file), 'utf8').catch(() => '')
    if (!html) {
      bad++
      fail(`page identity: could not read ${page.file}`)
      continue
    }

    const canonical = /<link[^>]+rel="canonical"[^>]*href="([^"]+)"/.exec(html)?.[1] ?? ''
    const expected = `${SITE_ORIGIN}${page.path}`
    if (canonical !== expected) {
      bad++
      fail(`page identity: ${page.file} canonical is "${canonical}", expected "${expected}"`)
    }

    const found = []
    for (const block of html.matchAll(/<script type="application\/ld\+json">([\s\S]*?)<\/script>/g)) {
      try {
        found.push(JSON.parse(block[1])['@type'])
      } catch {
        bad++
        fail(`page identity: ${page.file} carries an unparseable application/ld+json block`)
      }
    }
    const got = [...found].sort().join(', ')
    const want = [...page.entities].sort().join(', ')
    if (got !== want) {
      bad++
      fail(`page identity: ${page.file} declares entities [${got}], expected exactly [${want}] from its registry key "${page.key}"`)
    }

    if (page.key !== 'index') {
      for (const entity of LANDING_ENTITIES) {
        if (html.includes(`"${entity}"`)) {
          bad++
          fail(`page identity: ${page.file} asserts the landing page's "${entity}" entity, which only / may state`)
        }
      }
    }
  }
  if (bad === 0) console.log(`ok page identity: ${REGISTRY.length} pages, each with its own canonical and exactly its assigned entities`)
}

/**
 * Fail when a directory-index page does not answer at its extensionless canonical URL.
 *
 * The trust anchor pages publish extensionless canonical URLs and are emitted as
 * directory indexes, so the claim that those URLs serve is a property of the server, not
 * of the file. It is asserted here over HTTP against the harness server, which resolves
 * an extensionless path the way GitHub Pages does (CR-0077 AC-1).
 *
 * @param {string} origin Origin of the served site.
 */
async function assertExtensionlessPaths(origin) {
  const paths = REGISTRY.filter((p) => p.file.endsWith('/index.html') && p.path !== '/').map((p) => p.path)
  let bad = 0
  for (const path of paths) {
    const response = await fetch(`${origin}${path}`)
    const body = await response.text()
    if (response.status !== 200 || body.length === 0) {
      bad++
      fail(`extensionless path ${path}: HTTP ${response.status}, ${body.length} bytes (expected 200 and a non-empty document)`)
    }
  }
  if (bad === 0) console.log(`ok extensionless paths: ${paths.join(', ')} each resolve to their directory index`)
}

console.log(failures.length === 0 ? 'content-check: all assertions hold' : `content-check: ${failures.length} failing`)
process.exit(failures.length === 0 ? 0 : 1)
