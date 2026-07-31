/**
 * @agents-index The four published pages and four viewport widths every site measurement covers.
 *
 * Purpose:
 *   CR-0070's acceptance criteria are stated per page and per viewport. Keeping the
 *   two lists in one module means a screenshot run, a DOM audit, a validation run and
 *   a content check can never silently disagree about what "all four pages" means.
 *
 * Usage (as a module):
 *   import { PAGES, WIDTHS } from './site.pages.mjs'
 */

/** The four published HTML pages, by dist-relative path. */
export const PAGES = ['index.html', 'quickstart.html', 'concepts.html', 'troubleshooting.html']

/**
 * Viewport widths for visual-regression capture, from desktop down to a small phone.
 * 1024 is the site's desktop/mobile branch boundary, so it is deliberately included
 * on both sides of the breakpoint (1440 above, 640 and 390 below).
 */
export const WIDTHS = [1440, 1024, 640, 390]
