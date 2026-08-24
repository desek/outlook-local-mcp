/**
 * seo.pages.ts - the canonical registry of published pages and their SEO identity.
 *
 * Every page the build emits is described once here: its output filename, its canonical
 * path, and the title and description used for both the document and its social
 * metadata. The SEO plugin, the sitemap generator, and the JSON-LD builder all read
 * this list, so the set of pages, their canonical URLs, and their social cards can never
 * disagree (CR-0070 FR-19, FR-21 to FR-23).
 *
 * The set mirrors the Vite HTML inputs: the landing page, the three generated
 * documentation pages, and the three trust anchor pages. Adding a page is a deliberate
 * edit here.
 *
 * Pages are identified by their dist-relative output path, not by a bare filename. The
 * trust anchor pages are emitted as directory indexes (about/index.html) so an
 * extensionless request resolves on GitHub Pages, and three files named index.html
 * cannot be told apart by basename alone (CR-0077).
 *
 * @agents-index Registry of published pages with canonical path, title, and description; the single source for SEO, sitemap, and JSON-LD.
 */
import { SITE_ORIGIN } from '../src/site.meta'

/**
 * PageKey identifies a page by the JSON-LD entity family it carries, not merely its
 * URL, so the JSON-LD builder can branch on it without re-parsing filenames.
 */
export type PageKey =
  | 'index'
  | 'concepts'
  | 'quickstart'
  | 'troubleshooting'
  | 'about'
  | 'contact'
  | 'privacy'

/**
 * PageSeo is the SEO identity of one published page.
 *
 * @property key  The stable page key, also the JSON-LD selector.
 * @property file  The emitted HTML path relative to the site root, without a leading
 *   slash (the transformIndexHtml match target). A directory-index page states the
 *   whole path, for example "about/index.html".
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
 * The landing description composes its tool-surface figures from the generated surface
 * manifest (CR-0073), so the meta description can never state a count the server does not
 * expose. No figure is transcribed here.
 */
export const PAGES: readonly PageSeo[] = [
  {
    key: 'index',
    file: 'index.html',
    path: '/',
    title: "Outlook Local MCP — Your AI's Native Interface to Outlook",
    description:
      'A Model Context Protocol server that connects Claude directly to Microsoft Calendar and Mail. Ask it to check your week, book a meeting, find a thread, or send a reply. No Entra ID setup. No cloud middleman. 100% local, zero-config auth.',
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
  {
    key: 'about',
    file: 'about/index.html',
    path: '/about',
    title: 'About — Outlook Local MCP',
    description:
      'What Outlook Local MCP is, who builds it, the open source licence it ships under, and how it relates to the Microsoft Graph API it calls on your behalf.',
  },
  {
    key: 'contact',
    file: 'contact/index.html',
    path: '/contact',
    title: 'Contact — Outlook Local MCP',
    description:
      'How to reach the Outlook Local MCP project: the public GitHub repository, its issue tracker for bugs and feature requests, and what to include in a report.',
  },
  {
    key: 'privacy',
    file: 'privacy/index.html',
    path: '/privacy',
    title: 'Privacy — Outlook Local MCP',
    description:
      'Where your mail and calendar data goes: the server runs locally, tokens stay in the operating system keychain, and the only outbound calls are to Microsoft.',
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
 * pageForFile resolves a page by the site-relative HTML path Vite is transforming.
 *
 * The match is on the whole path, not the basename: the trust anchor pages are emitted
 * as directory indexes, so "about/index.html", "contact/index.html", and the landing
 * page's own "index.html" all share a basename and only the path tells them apart
 * (CR-0077). Leading slashes and Windows separators are normalised so "/about/index.html"
 * and "about\\index.html" both resolve.
 *
 * @param file  The site-relative HTML path, for example "index.html" or "/about/index.html".
 * @returns The matching PageSeo, or undefined if the path is not a registered page.
 */
export function pageForFile(file: string): PageSeo | undefined {
  const normalised = file.replace(/\\/g, '/').replace(/^\/+/, '')
  return PAGES.find((p) => p.file === normalised)
}
