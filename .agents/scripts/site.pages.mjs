/**
 * @agents-index The published pages and four viewport widths every site measurement covers.
 *
 * Purpose:
 *   CR-0070's acceptance criteria are stated per page and per viewport. Keeping the
 *   two lists in one module means a screenshot run, a DOM audit, a validation run and
 *   a content check can never silently disagree about what "every page" means.
 *
 * Usage (as a module):
 *   import { PAGES, WIDTHS } from './site.pages.mjs'
 */

/**
 * The published HTML pages, by dist-relative path: the landing page, the three generated
 * documentation pages, and the three trust anchor pages CR-0077 adds. The trust anchor
 * pages are emitted as directory indexes so they serve from extensionless canonical URLs,
 * which is why they are named by their `index.html` inside the directory.
 */
export const PAGES = [
  'index.html',
  'quickstart.html',
  'concepts.html',
  'troubleshooting.html',
  'about/index.html',
  'contact/index.html',
  'privacy/index.html',
]

/**
 * Viewport widths for visual-regression capture, from desktop down to a small phone.
 * 1024 is the site's desktop/mobile branch boundary, so it is deliberately included
 * on both sides of the breakpoint (1440 above, 640 and 390 below).
 */
export const WIDTHS = [1440, 1024, 640, 390]
