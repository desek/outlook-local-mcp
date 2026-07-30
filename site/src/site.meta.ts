/**
 * site.meta.ts - shared, framework-agnostic site identity constants.
 *
 * These values are the single source of truth for the deployment origin and the
 * editorial "last updated" date, imported by both the React tree (the footer renders
 * the visible date) and the build-time SEO plugin (which emits the same date as the
 * JSON-LD dateModified). Keeping them in one module is what makes the visible date and
 * the structured-data date provably equal (CR-0070 FR-45) rather than two literals that
 * can drift apart.
 *
 * SITE_ORIGIN is the apex custom domain settled by FR-1; every absolute URL the site
 * emits (canonical, Open Graph, Twitter, sitemap, JSON-LD) is built from it, so a host
 * change is a one-line edit here.
 *
 * @agents-index Shared site constants: apex origin and the editorial last-updated date, consumed by both the React tree and the build SEO layer.
 */

/**
 * SITE_ORIGIN is the absolute apex origin the site is served from (CR-0070 FR-1).
 * It carries no trailing slash so callers concatenate a leading-slash path cleanly.
 */
export const SITE_ORIGIN = 'https://outlook-local-mcp.com'

/**
 * LAST_UPDATED_ISO is the editorial last-updated date in ISO 8601 (YYYY-MM-DD).
 * Used verbatim as the JSON-LD dateModified so a crawler reads a valid schema.org date.
 */
export const LAST_UPDATED_ISO = '2026-07-30'

/**
 * LAST_UPDATED_DISPLAY is the human-readable rendering of LAST_UPDATED_ISO shown to
 * visitors. It must describe the same day as LAST_UPDATED_ISO; the two are asserted
 * equal in the metadata suite, so edit them together.
 */
export const LAST_UPDATED_DISPLAY = 'July 30, 2026'
