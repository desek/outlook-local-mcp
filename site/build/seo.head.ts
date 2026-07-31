/**
 * seo.head.ts - renders the canonical, Open Graph, and Twitter card head fragment.
 *
 * Produces the crawler-facing metadata every page carries (CR-0070 FR-21 to FR-23):
 * a canonical link, the six required Open Graph properties, and the four required
 * Twitter card properties, all built from the apex origin so no absolute URL points at
 * the old github.io project path. The build:updated meta carries the editorial date as
 * a machine-readable companion to the visible last-updated line (FR-45).
 *
 * @agents-index Renders the per-page canonical, Open Graph, and Twitter card meta tags from the page registry.
 */
import { SITE_ORIGIN, LAST_UPDATED_ISO } from '../src/site.meta'
import { canonicalUrl, type PageSeo } from './seo.pages'

/**
 * OG_SITE_NAME is the og:site_name value, shared across every page.
 */
const OG_SITE_NAME = 'Outlook Local MCP'

/**
 * OG_IMAGE is the absolute URL of the social share image. The icon is served from the
 * site's own origin (FR-24, and NFR-3: no third-party asset), so the card image never
 * leaves the apex.
 */
const OG_IMAGE = `${SITE_ORIGIN}/icon.png`

/**
 * OG_IMAGE_ALT is descriptive alt text for the share image, so the Open Graph image
 * carries meaning rather than a bare URL (FR-24).
 */
const OG_IMAGE_ALT = 'The Outlook Local MCP wordmark'

/**
 * escapeAttr escapes the characters unsafe in a double-quoted HTML attribute value.
 * The inputs are build-controlled registry strings, but titles and descriptions contain
 * apostrophes and ampersands, so escaping keeps the emitted head well-formed.
 *
 * @param s  The raw attribute value.
 * @returns The escaped value, safe inside double quotes.
 */
function escapeAttr(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

/**
 * renderSeoHead renders the SEO head fragment for one page.
 *
 * @param page  The page whose metadata is rendered.
 * @returns An HTML fragment of newline-separated <link> and <meta> tags, indented for
 *   insertion before </head>.
 */
export function renderSeoHead(page: PageSeo): string {
  const url = canonicalUrl(page)
  const title = escapeAttr(page.title)
  const description = escapeAttr(page.description)
  const ogType = page.key === 'index' ? 'website' : 'article'
  const tags = [
    `<link rel="canonical" href="${url}" />`,
    `<meta property="og:title" content="${title}" />`,
    `<meta property="og:description" content="${description}" />`,
    `<meta property="og:type" content="${ogType}" />`,
    `<meta property="og:url" content="${url}" />`,
    `<meta property="og:image" content="${OG_IMAGE}" />`,
    `<meta property="og:image:alt" content="${OG_IMAGE_ALT}" />`,
    `<meta property="og:site_name" content="${OG_SITE_NAME}" />`,
    `<meta name="twitter:card" content="summary_large_image" />`,
    `<meta name="twitter:title" content="${title}" />`,
    `<meta name="twitter:description" content="${description}" />`,
    `<meta name="twitter:image" content="${OG_IMAGE}" />`,
    `<meta name="twitter:image:alt" content="${OG_IMAGE_ALT}" />`,
    `<meta name="build:updated" content="${LAST_UPDATED_ISO}" />`,
  ]
  return tags.map((tag) => `    ${tag}`).join('\n')
}
