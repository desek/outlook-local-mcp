#!/usr/bin/env node
/**
 * @agents-index Audit every rendered text node on the built site for WCAG AA contrast.
 *
 * Purpose:
 *   Lighthouse reports contrast failures only for the viewport it emulates and truncates
 *   the node list, so fixing what it prints can leave equivalent failures in the branch it
 *   did not render. This walks the live DOM at several widths, computes each text node's
 *   effective background by climbing to the nearest opaque ancestor, and reports every
 *   pair below the AA threshold — the same method axe-core uses, but exhaustive and
 *   re-runnable, so a fix can be verified rather than assumed.
 *
 * Usage:
 *   node .agents/scripts/site.contrast.audit.mjs [dist-dir]
 *
 * Parameters:
 *   dist-dir  Built site to audit. Defaults to `site/dist`.
 *
 * Side effects: launches headless Chrome and binds a local port.
 * Exits 0 when no text fails AA, 1 otherwise.
 */

import puppeteer from '../../site/node_modules/puppeteer-core/lib/esm/puppeteer/puppeteer-core.js'
import { serve } from './site.serve.mjs'
import { PAGES, WIDTHS } from './site.pages.mjs'

const CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'
const distDir = process.argv[2] ?? 'site/dist'

/**
 * Collect contrast failures on the current page.
 *
 * Runs inside the page. Large text (>= 24px, or >= 18.66px bold) is held to AA's 3:1,
 * everything else to 4.5:1, matching WCAG 1.4.3.
 *
 * @returns {Array<{selector: string, text: string, fg: string, bg: string, ratio: number, required: number}>}
 */
const collectFailures = () => {
  // Tailwind v4 emits `color-mix()` for its opacity utilities, so a computed colour can
  // come back as `color(srgb 1 1 1 / 0.5)` or `oklab(...)` rather than `rgba()`. Painting
  // the value into a canvas and reading the pixel back normalises every syntax the engine
  // accepts, instead of pattern-matching a subset and silently mis-reading the rest.
  const probe = document.createElement('canvas')
  probe.width = 1
  probe.height = 1
  const probeContext = probe.getContext('2d', { willReadFrequently: true })

  /**
   * Parse any computed colour value into 0-255 channels plus 0-1 alpha.
   *
   * @param {string} value A computed CSS colour in any syntax.
   * @returns {{r: number, g: number, b: number, a: number}}
   */
  const parse = (value) => {
    probeContext.clearRect(0, 0, 1, 1)
    probeContext.fillStyle = value
    probeContext.fillRect(0, 0, 1, 1)
    const [r, g, b, a] = probeContext.getImageData(0, 0, 1, 1).data
    return { r, g, b, a: a / 255 }
  }

  const luminance = ({ r, g, b }) => {
    const channel = (c) => {
      const s = c / 255
      return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4
    }
    return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b)
  }

  const ratio = (a, b) => {
    const [high, low] = [luminance(a), luminance(b)].sort((x, y) => y - x)
    return (high + 0.05) / (low + 0.05)
  }

  const composite = (fg, bg) => ({
    r: fg.r * fg.a + bg.r * (1 - fg.a),
    g: fg.g * fg.a + bg.g * (1 - fg.a),
    b: fg.b * fg.a + bg.b * (1 - fg.a),
    a: 1,
  })

  /** Effective background: the nearest ancestor with a non-transparent background colour. */
  const backgroundOf = (element) => {
    let current = element
    let accumulated = null
    while (current) {
      const colour = parse(getComputedStyle(current).backgroundColor)
      if (colour.a > 0) {
        accumulated = accumulated ? composite(accumulated, colour) : colour
        if (accumulated.a >= 1 || colour.a >= 1) return { ...accumulated, a: 1 }
      }
      current = current.parentElement
    }
    return { r: 255, g: 255, b: 255, a: 1 }
  }

  /** A short, human-recognisable path to the element. */
  const describe = (element) => {
    const parts = []
    let current = element
    for (let depth = 0; current && depth < 3; depth++) {
      const classes = (current.className?.baseVal ?? current.className ?? '').toString().split(/\s+/).filter(Boolean)
      parts.unshift(current.tagName.toLowerCase() + (classes.length ? `.${classes.slice(0, 3).join('.')}` : ''))
      current = current.parentElement
    }
    return parts.join(' > ')
  }

  const failures = []
  const seen = new Set()

  for (const element of document.querySelectorAll('body *')) {
    // Only elements that directly own visible text; a wrapper inherits its child's report.
    const own = [...element.childNodes]
      .filter((n) => n.nodeType === Node.TEXT_NODE)
      .map((n) => n.textContent.trim())
      .join(' ')
      .trim()
    if (!own) continue

    const style = getComputedStyle(element)
    if (style.visibility === 'hidden' || style.display === 'none' || Number(style.opacity) === 0) continue
    const rect = element.getBoundingClientRect()
    if (rect.width === 0 || rect.height === 0) continue

    const size = parseFloat(style.fontSize)
    const bold = Number(style.fontWeight) >= 700
    const required = size >= 24 || (bold && size >= 18.66) ? 3 : 4.5

    const bg = backgroundOf(element)
    const fg = composite(parse(style.color), bg)
    const value = ratio(fg, bg)
    if (value >= required) continue

    const hex = (c) => `#${[c.r, c.g, c.b].map((x) => Math.round(x).toString(16).padStart(2, '0')).join('')}`
    const key = `${describe(element)}|${hex(fg)}|${hex(bg)}`
    if (seen.has(key)) continue
    seen.add(key)

    failures.push({
      selector: describe(element),
      text: own.slice(0, 40),
      fg: hex(fg),
      bg: hex(bg),
      ratio: Number(value.toFixed(2)),
      required,
    })
  }
  return failures
}

const site = await serve(distDir)
const browser = await puppeteer.launch({ executablePath: CHROME, headless: true })
let total = 0

try {
  for (const width of WIDTHS) {
    for (const page of PAGES) {
      const tab = await browser.newPage()
      await tab.emulateMediaFeatures([{ name: 'prefers-reduced-motion', value: 'reduce' }])
      await tab.setViewport({ width, height: 900, deviceScaleFactor: 1 })
      await tab.goto(`${site.origin}/${page}`, { waitUntil: 'networkidle0', timeout: 60_000 })
      const failures = await tab.evaluate(collectFailures)
      await tab.close()

      for (const failure of failures) {
        console.log(
          `FAIL ${page}@${width} ${failure.ratio}:1 (need ${failure.required}) ${failure.fg} on ${failure.bg} ` +
            `"${failure.text}" -- ${failure.selector}`,
        )
      }
      total += failures.length
    }
  }
} finally {
  await browser.close()
  await site.close()
}

console.log(`contrast-audit: ${total} failing text element(s) across ${PAGES.length} pages x ${WIDTHS.length} widths`)
process.exit(total === 0 ? 0 : 1)
