/**
 * seo.pages.ts - the canonical registry of published pages and their SEO identity.
 *
 * Every page the build emits is described once here: its output filename, its canonical
 * path, and the title and description used for both the document and its social
 * metadata. The SEO plugin, the sitemap generator, and the JSON-LD builder all read
 * this list, so the set of pages, their canonical URLs, and their social cards can never
 * disagree (CR-0070 FR-19, FR-21 to FR-23).
 *
 * The set mirrors the Vite HTML inputs: the landing page plus the three generated
 * documentation pages. Adding a page is a deliberate edit here.
 *
 * @agents-index Registry of published pages with canonical path, title, and description; the single source for SEO, sitemap, and JSON-LD.
 */
import { SITE_ORIGIN } from '../src/site.meta'

/**
 * PageKey identifies a page by the JSON-LD entity family it carries, not merely its
 * URL, so the JSON-LD builder can branch on it without re-parsing filenames.
 */
export type PageKey = 'index' | 'concepts' | 'quickstart' | 'troubleshooting'

/**
 * PageSeo is the SEO identity of one published page.
 *
 * @property key  The stable page key, also the JSON-LD selector.
 * @property file  The emitted HTML filename at the site root (the transformIndexHtml match target).
 * @property path  The absolute-from-root URL path, including the leading slash.
 * @property title  The document and og:title text.
 * @property description  The meta description and og:description text.
 */
export interface PageSeo {
  key: PageKey
  file: string
  path: string
  title: string
  description: string
}

/**
 * PAGES is the ordered registry of every published page. The landing page is first so
 * the sitemap lists the site root before its subpages.
 *
 * The landing description deliberately preserves the inherited "23 MCP tools" figure:
 * copy correction is a separate CR (CR-0070 "Deferred to a follow-up CR"), so this
 * phase must not silently change the number.
 */
export const PAGES: readonly PageSeo[] = [
  {
    key: 'index',
    file: 'index.html',
    path: '/',
    title: "Outlook Local MCP — Your AI's Native Interface to Outlook",
    description:
      'A Model Context Protocol server that connects Claude directly to Microsoft Calendar and Mail. No Entra ID setup. No cloud middleman. 100% local, zero-config auth, 23 MCP tools.',
  },
  {
    key: 'concepts',
    file: 'concepts.html',
    path: '/concepts.html',
    title: 'Concepts — Outlook Local MCP',
    description:
      'Core concepts for Outlook Local MCP: output tiers, the multi-account model, mail gating, OAuth scopes, and observability for the local Microsoft Outlook MCP server.',
  },
  {
    key: 'quickstart',
    file: 'quickstart.html',
    path: '/quickstart.html',
    title: 'Quick Start — Outlook Local MCP',
    description:
      'Install Outlook Local MCP, configure Claude Desktop or Claude Code, authenticate, and make your first calendar and mail tool call, all on your local machine.',
  },
  {
    key: 'troubleshooting',
    file: 'troubleshooting.html',
    path: '/troubleshooting.html',
    title: 'Troubleshooting — Outlook Local MCP',
    description:
      'Recover from common Outlook Local MCP failures: auth errors, token refresh, Keychain access, Graph throttling, mail flags, and account lifecycle issues.',
  },
]

/**
 * canonicalUrl builds the absolute canonical URL for a page from the apex origin.
 *
 * @param page  The page whose canonical URL is wanted.
 * @returns The absolute URL, for example "https://outlook-local-mcp.com/concepts.html".
 */
export function canonicalUrl(page: PageSeo): string {
  return `${SITE_ORIGIN}${page.path}`
}

/**
 * pageForFile resolves a page by the HTML filename Vite is transforming.
 *
 * @param file  The HTML basename, for example "index.html".
 * @returns The matching PageSeo, or undefined if the file is not a registered page.
 */
export function pageForFile(file: string): PageSeo | undefined {
  return PAGES.find((p) => p.file === file)
}
