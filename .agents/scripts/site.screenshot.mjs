#!/usr/bin/env node
/**
 * @agents-index Capture deterministic full-page screenshot tiles of the built site for visual-regression comparison.
 *
 * Purpose:
 *   CR-0070's "no visual regression" criterion compares a candidate build against the
 *   pre-change build pixel for pixel. This script produces one comparable image set per
 *   build: every published page, at every acceptance viewport width, captured under the
 *   virtual clock and seeded PRNG from `site.determinism.mjs`.
 *
 *   Pages are captured as *tiles* (one viewport-sized image per scroll step) rather than
 *   a single `fullPage` shot. Puppeteer's full-page capture resizes the viewport
 *   internally, which fires the site's ResizeObservers and re-seeds the canvases
 *   mid-capture; tiling keeps the viewport fixed so each image is reproducible. Tiles
 *   also stay well inside Chrome's maximum texture size on the long documentation pages.
 *
 * Usage:
 *   node .agents/scripts/site.screenshot.mjs <out-dir> [dist-dir]
 *
 * Parameters:
 *   out-dir   Directory to write PNG tiles into. Created if absent. Named
 *             `<page>@<width>-<tileIndex>.png`.
 *   dist-dir  Built site to capture. Defaults to `site/dist`.
 *
 * Side effects: launches headless Chrome, binds an ephemeral local port, writes PNGs.
 * Exits non-zero if Chrome cannot be launched or a page fails to load.
 */

import { mkdir, writeFile, rm } from 'node:fs/promises'
import { join } from 'node:path'
import { puppeteer } from './site.puppeteer.mjs'
import { serve } from './site.serve.mjs'
import { PAGES, WIDTHS } from './site.pages.mjs'
import { DETERMINISM_INIT } from './site.determinism.mjs'

const CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'

/** Viewport height for each tile. Fixed so tile boundaries land identically across builds. */
const TILE_HEIGHT = 900

/**
 * Frames to pump after each scroll step. This must be generous: the pinned, scrubbed
 * sections reach their scroll-driven state through Lenis, whose smoothed scroll position
 * converges over frames, and only then does ScrollTrigger advance the pinned timeline.
 * Too few frames captures a section mid-convergence, which varies between runs.
 */
const FRAMES_PER_STEP = 150

/**
 * Real milliseconds to yield after a scroll before pumping, so the browser has actually
 * dispatched the scroll event to Lenis. Without this the pump can run before the site
 * knows the scroll happened, which is the other half of the same race.
 */
const SETTLE_MS = 60

const [outDir, distDir = 'site/dist'] = process.argv.slice(2)
if (!outDir) {
  console.error('usage: node .agents/scripts/site.screenshot.mjs <out-dir> [dist-dir]')
  process.exit(2)
}

/**
 * Capture every page at every width into `outDir`.
 *
 * @returns {Promise<number>} Number of tiles written.
 */
async function capture() {
  await rm(outDir, { recursive: true, force: true })
  await mkdir(outDir, { recursive: true })

  const site = await serve(distDir)
  const browser = await puppeteer.launch({
    executablePath: CHROME,
    headless: true,
    args: ['--hide-scrollbars', '--force-device-scale-factor=1', '--disable-lcd-text'],
  })

  let tiles = 0
  try {
    for (const width of WIDTHS) {
      for (const page of PAGES) {
        tiles += await capturePage(browser, site.origin, page, width)
      }
    }
  } finally {
    await browser.close()
    await site.close()
  }
  return tiles
}

/**
 * Capture one page at one width as a sequence of viewport tiles.
 *
 * @param {import('puppeteer-core').Browser} browser Live browser.
 * @param {string} origin Origin of the static server.
 * @param {string} page Dist-relative page path.
 * @param {number} width Viewport width in CSS pixels.
 * @returns {Promise<number>} Number of tiles written for this page/width.
 */
async function capturePage(browser, origin, page, width) {
  const tab = await browser.newPage()
  // Capture the reduced-motion rendering. Every scroll-driven section has an explicit
  // `prefers-reduced-motion: reduce` branch that sets its final state directly instead of
  // animating into it, so this both removes mid-flight animation state (which varies by a
  // frame or two between runs and would swamp the comparison) and captures the settled
  // appearance of each section, which is what a regression would actually alter.
  await tab.emulateMediaFeatures([{ name: 'prefers-reduced-motion', value: 'reduce' }])
  await tab.evaluateOnNewDocument(DETERMINISM_INIT)
  await tab.setViewport({ width, height: TILE_HEIGHT, deviceScaleFactor: 1 })
  await tab.goto(`${origin}/${page}`, { waitUntil: 'networkidle0', timeout: 60_000 })
  await tab.evaluate(() => document.fonts.ready)

  // Wait for React to have hydrated before pumping any frames. The landing page's client
  // bundle is injected only after the browser reports its Largest Contentful Paint entry,
  // so hydration completes at a time that varies from run to run; pumping before it lands
  // starts the canvases at a different frame each time. Waiting here removed the last
  // source of run-to-run variance, which had one canvas tile straddling the 99% threshold.
  await tab
    .waitForFunction(
      () => {
        const root = document.getElementById('root')
        return !root || Object.keys(root).some((key) => key.startsWith('__react'))
      },
      { timeout: 30_000 },
    )
    .catch(() => {
      // The documentation pages have no root to hydrate, and a page that never hydrates
      // is still worth capturing: it is what a visitor with broken JavaScript sees.
    })

  // Freeze SVG SMIL animations. The capability diagrams animate their connector dashes
  // and cursors with <animate> and <animateMotion>, which run on the SVG document
  // timeline rather than on requestAnimationFrame, so neither the virtual clock nor
  // reduced-motion emulation touches them. Pausing each root and pinning its clock to a
  // fixed time is what makes those tiles reproducible; without it one tile sat astride
  // the 99% threshold and failed on roughly half of otherwise identical captures.
  await tab.evaluate(() => {
    for (const svg of document.querySelectorAll('svg')) {
      if (typeof svg.pauseAnimations !== 'function') continue
      svg.pauseAnimations()
      svg.setCurrentTime(0)
    }
  })

  await tab.evaluate((frames) => window.__pump(frames), FRAMES_PER_STEP)

  const height = await tab.evaluate(() => document.documentElement.scrollHeight)
  const steps = Math.max(1, Math.ceil(height / TILE_HEIGHT))
  const name = page.replace(/\.html$/, '')

  for (let index = 0; index < steps; index++) {
    // `behavior: 'instant'` is required: the site sets `scroll-behavior: smooth`, which
    // otherwise animates the jump and leaves the tile captured mid-scroll.
    await tab.evaluate((top) => window.scrollTo({ top, left: 0, behavior: 'instant' }), index * TILE_HEIGHT)
    await new Promise((resolve) => setTimeout(resolve, SETTLE_MS))
    await tab.evaluate((frames) => window.__pump(frames), FRAMES_PER_STEP)
    const shot = await tab.screenshot({ type: 'png', optimizeForSpeed: false })
    await writeFile(join(outDir, `${name}@${width}-${String(index).padStart(3, '0')}.png`), shot)
  }

  await tab.close()
  return steps
}

const written = await capture()
console.log(`screenshot: wrote ${written} tiles to ${outDir}`)
