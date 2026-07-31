/**
 * sitemap.ts - builds sitemap.xml from the published page registry.
 *
 * The sitemap is generated from the same PAGES list the SEO layer uses (CR-0070 FR-19),
 * so every emitted HTML page appears exactly once with an absolute apex URL and a
 * lastmod, and a page cannot be published without also being listed. It is emitted as a
 * build output (FR-54), not hand-placed on gh-pages.
 *
 * @agents-index Generates sitemap.xml from the page registry with absolute apex URLs and a lastmod.
 */
import { LAST_UPDATED_ISO } from '../src/site.meta'
import { canonicalUrl, PAGES } from './seo.pages'

/**
 * buildSitemap renders the sitemap.xml document for every registered page.
 *
 * @param lastmod  The ISO date used as <lastmod> for every entry; defaults to the shared
 *   editorial date.
 * @returns The complete sitemap.xml document, newline-terminated.
 */
export function buildSitemap(lastmod: string = LAST_UPDATED_ISO): string {
  const urls = PAGES.map(
    (page) =>
      `  <url>\n    <loc>${canonicalUrl(page)}</loc>\n    <lastmod>${lastmod}</lastmod>\n  </url>`,
  ).join('\n')
  return `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${urls}\n</urlset>\n`
}
