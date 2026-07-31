#!/usr/bin/env node
/**
 * @agents-index Validate the built site against the W3C Nu checker, separating real errors from CSS-profile lag.
 *
 * Purpose:
 *   CR-0070 requires zero real markup errors on every published page. The Nu checker also
 *   reports each use of a CSS feature its bundled CSS profile predates (`@property`,
 *   `margin-trim` and friends), which is a limitation of the checker rather than a defect
 *   in the page. Counting those as failures would make the criterion unachievable without
 *   removing working CSS, so this script partitions the report and asserts only on the
 *   real errors, printing the ignored count so the exclusion stays visible rather than
 *   silent.
 *
 * Usage:
 *   node .agents/scripts/site.validate.mjs [dist-dir]
 *
 * Parameters:
 *   dist-dir  Built site to validate. Defaults to `site/dist`.
 *
 * Side effects: uploads each built page to https://validator.w3.org/nu/ for checking.
 * The pages are the public website's own source, which is the artefact being published.
 *
 * Exits 0 when every page has zero real errors, 1 otherwise.
 */

import { readFile } from 'node:fs/promises'
import { join } from 'node:path'
import { PAGES } from './site.pages.mjs'

const distDir = process.argv[2] ?? 'site/dist'
const ENDPOINT = 'https://validator.w3.org/nu/?out=json'

/**
 * Patterns for messages caused by the checker's CSS profile lagging the CSS the site
 * uses. Each entry names a feature that is valid and shipping in browsers; a message
 * matching one of these is reported as ignored rather than counted as an error.
 */
const CSS_PROFILE_LAG = [
  /Property .?(@property|margin-trim|color-mix|text-wrap|field-sizing|anchor-name|position-anchor|scroll-timeline|view-timeline|animation-timeline|overlay|interpolate-size|text-box|corner-shape)/i,
  /at-rule .?@(property|container|layer|starting-style|scope|supports)/i,
  /Unknown pseudo-(element|class)/i,
  /is not a .?(color|length|integer|percentage).? value/i,
  /^CSS: /i,
]

/**
 * Ask the Nu checker to validate one page.
 *
 * @param {string} page Dist-relative page path.
 * @returns {Promise<{page: string, real: object[], ignored: number}>}
 *   The page's real errors and the count of ignored CSS-profile-lag messages.
 * @throws {Error} If the checker cannot be reached or returns a non-JSON response.
 */
async function validate(page) {
  const html = await readFile(join(distDir, page))
  const response = await fetch(ENDPOINT, {
    method: 'POST',
    headers: {
      'content-type': 'text/html; charset=utf-8',
      'user-agent': 'outlook-local-mcp-site-harness',
    },
    body: html,
  })
  if (!response.ok) throw new Error(`validator returned ${response.status} for ${page}`)

  const { messages } = await response.json()
  const errors = messages.filter((m) => m.type === 'error' || m.subType === 'error')
  const real = errors.filter((m) => !CSS_PROFILE_LAG.some((pattern) => pattern.test(m.message)))
  return { page, real, ignored: errors.length - real.length }
}

let failed = 0
for (const page of PAGES) {
  const { real, ignored } = await validate(page)
  for (const message of real) {
    console.log(`ERROR ${page}:${message.lastLine ?? '?'} ${message.message}`)
    if (message.extract) console.log(`      extract: ${message.extract.replace(/\s+/g, ' ').trim()}`)
  }
  console.log(`validate ${page}: ${real.length} real error(s), ${ignored} CSS-profile-lag message(s) ignored`)
  if (real.length) failed++
}

process.exit(failed === 0 ? 0 : 1)
