#!/usr/bin/env node
/**
 * @agents-index Summarise an lhci run as per-page category scores and Core Web Vitals medians.
 *
 * Purpose:
 *   `lhci autorun` prints only the assertions that failed, which is the wrong shape for
 *   iterating on a score: it tells you that you missed, not by how much or where. This
 *   reads the reports lhci already wrote to `.lighthouseci/` and prints, per page, the
 *   four category scores and the median LCP, CLS and TBT, so a change can be attributed
 *   to the metric it moved.
 *
 * Usage:
 *   node .agents/scripts/site.lighthouse.summary.mjs [lhci-dir]
 *
 * Parameters:
 *   lhci-dir  Directory holding lhci's JSON reports. Defaults to `site/.lighthouseci`.
 *
 * Exits 0 always; this reports, it does not assert. The gate is `lighthouserc.json`.
 */

import { readdir, readFile } from 'node:fs/promises'
import { basename, join } from 'node:path'

const dir = process.argv[2] ?? 'site/.lighthouseci'

/** Audits to report alongside the category scores, with the unit to print. */
const METRICS = [
  ['largest-contentful-paint', 'LCP', 'ms'],
  ['cumulative-layout-shift', 'CLS', ''],
  ['total-blocking-time', 'TBT', 'ms'],
]

/**
 * Median of a numeric list.
 *
 * @param {number[]} values Sample values; must be non-empty.
 * @returns {number} The middle value, or the mean of the two middle values.
 */
function median(values) {
  const sorted = [...values].sort((a, b) => a - b)
  const middle = sorted.length >> 1
  return sorted.length % 2 ? sorted[middle] : (sorted[middle - 1] + sorted[middle]) / 2
}

const byPage = new Map()
for (const file of (await readdir(dir)).filter((f) => f.startsWith('lhr-') && f.endsWith('.json'))) {
  const report = JSON.parse(await readFile(join(dir, file), 'utf8'))
  const page = basename(new URL(report.finalDisplayedUrl ?? report.finalUrl).pathname)
  if (!byPage.has(page)) byPage.set(page, [])
  byPage.get(page).push(report)
}

for (const [page, reports] of [...byPage].sort()) {
  const score = (id) => median(reports.map((r) => r.categories[id].score)).toFixed(2)
  const parts = [
    `perf ${score('performance')}`,
    `a11y ${score('accessibility')}`,
    `bp ${score('best-practices')}`,
    `seo ${score('seo')}`,
  ]
  for (const [id, label, unit] of METRICS) {
    const value = median(reports.map((r) => r.audits[id].numericValue))
    parts.push(`${label} ${unit === 'ms' ? Math.round(value) : value.toFixed(3)}${unit}`)
  }
  console.log(`${page.padEnd(22)} ${parts.join('  ')}   (${reports.length} runs)`)
}
