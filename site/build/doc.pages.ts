/**
 * doc.pages.ts - build-time generation of the Markdown-sourced HTML entries.
 *
 * The site publishes the three narrative docs (concepts, quickstart, troubleshooting)
 * as crawlable HTML pages at stable URLs (CR-0070 FR-13), and the three trust anchor
 * pages (about, contact, privacy) authored under site/content/ (CR-0077). Each page is
 * generated from its Markdown at build time and emitted as a Vite HTML input, so Vite
 * hashes its assets and the provenance and SEO plugins inject its head meta exactly as
 * they do for the landing page. Repository Markdown stays the single source of truth and
 * is never copied into site/ (FR-14).
 *
 * generateDocPages is the load-bearing guard for FR-16: if a consumed Markdown file has
 * been renamed or removed it throws, naming the missing file, so the build fails loudly
 * rather than publishing a site that has silently lost a page.
 *
 * @agents-index Generates the doc and trust anchor HTML entries from Markdown, failing loudly on a missing file.
 */
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { renderMarkdown } from './doc.markdown'
import { pageForFile } from './seo.pages'
import { LAST_UPDATED_ISO, LAST_UPDATED_DISPLAY } from '../src/site.meta'

/**
 * DocPage describes one publishable Markdown-sourced page.
 *
 * The output path is stated explicitly rather than derived from a slug, because the two
 * shapes in use are not interchangeable: the narrative docs emit flat ("concepts.html",
 * served at "/concepts.html"), while the trust anchor pages emit as directory indexes
 * ("about/index.html") so an extensionless request for "/about" resolves on GitHub Pages
 * (CR-0077). It MUST match the `file` of the corresponding entry in seo.pages.ts, which
 * is what gives the page its canonical URL and JSON-LD.
 *
 * @property out  The output HTML path relative to the site root, using forward slashes.
 * @property source  The Markdown file path relative to the repository root.
 */
export interface DocPage {
  out: string
  source: string
}

/**
 * DOC_PAGES is the fixed set of Markdown-sourced pages published to the site. Adding a
 * page is a deliberate edit here; the set is not discovered, so a stray Markdown file
 * cannot silently become a public page.
 */
export const DOC_PAGES: readonly DocPage[] = [
  { out: 'concepts.html', source: 'docs/concepts.md' },
  { out: 'quickstart.html', source: 'docs/quickstart.md' },
  { out: 'troubleshooting.html', source: 'docs/troubleshooting.md' },
  { out: 'about/index.html', source: 'site/content/about.md' },
  { out: 'contact/index.html', source: 'site/content/contact.md' },
  { out: 'privacy/index.html', source: 'site/content/privacy.md' },
]

/**
 * pageTemplate wraps a rendered Markdown fragment in a full HTML document.
 *
 * The head carries a title and description and a module script that pulls in the
 * shared stylesheet; the provenance plugin injects its meta tags before </head> at
 * build time. The body holds exactly one <h1> (the document's own top heading) inside
 * a <main>, so the page satisfies the single-<h1> rule without pre-rendering React.
 *
 * The footer links every generated page back to the site root and to the three trust
 * anchor pages, so no generated page is a crawl dead end and each trust anchor page
 * reaches the other two (CR-0077 FR-6b). The narrative docs share this footer, which is
 * intended: the links are site-wide, not page-specific.
 *
 * @param title  The page title, from the document's first level-1 heading.
 * @param body  The rendered HTML fragment for the document body.
 * @param description  The meta description text.
 * @returns A complete HTML document string.
 */
function pageTemplate(title: string, body: string, description: string): string {
  return `<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <link rel="icon" type="image/svg+xml" href="/icon.svg" />
    <link rel="icon" type="image/png" href="/icon.png" />
    <title>${escapeHtml(title)} — Outlook Local MCP</title>
    <meta name="description" content="${escapeHtml(description)}" />
    <script type="module" src="/src/docs.entry.ts"></script>
  </head>
  <body class="antialiased">
    <main class="doc-page">
${body}
    </main>
    <footer class="doc-footer">
      <p><a href="/">Home</a> &middot; <a href="/about">About</a> &middot; <a href="/contact">Contact</a> &middot; <a href="/privacy">Privacy</a></p>
      <p>Last updated <time datetime="${LAST_UPDATED_ISO}">${escapeHtml(LAST_UPDATED_DISPLAY)}</time></p>
    </footer>
  </body>
</html>
`
}

/**
 * escapeHtml escapes the five characters that are unsafe in HTML attribute and text
 * contexts, used only for the small amount of build-controlled text (titles) placed
 * into the template.
 *
 * @param s  The raw string.
 * @returns The escaped string.
 */
function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

/**
 * generateDocPages renders every DocPage to an HTML file at the given site root.
 *
 * Each Markdown source is read relative to the repository root (the parent of the site
 * root) and rendered with Go-compatible heading anchors, then written to the page's
 * declared output path below the site root so Vite can treat it as an HTML input. The
 * containing directory is created first, because a directory-index page writes into a
 * directory that does not exist in the tracked tree (CR-0077).
 *
 * The meta description is taken from the SEO registry entry for the same output path, so
 * the document description and the Open Graph and Twitter descriptions the SEO plugin
 * injects are one fact rather than two that can drift apart.
 *
 * @param siteRoot  Absolute path to the site/ directory.
 * @returns The list of generated absolute HTML file paths, for use as Vite inputs.
 * @throws Error naming the file if a source Markdown file cannot be read (FR-16).
 */
export function generateDocPages(siteRoot: string): string[] {
  const repoRoot = resolve(siteRoot, '..')
  const outputs: string[] = []
  for (const page of DOC_PAGES) {
    const sourcePath = resolve(repoRoot, page.source)
    let markdown: string
    try {
      markdown = readFileSync(sourcePath, 'utf8')
    } catch {
      throw new Error(
        `documentation source ${page.source} is missing (expected at ${sourcePath}); ` +
          'the site build consumes it and cannot publish without it',
      )
    }
    const { html, title } = renderMarkdown(markdown)
    const seo = pageForFile(page.out)
    if (!seo) {
      throw new Error(
        `generated page ${page.out} has no entry in site/build/seo.pages.ts; ` +
          'it would publish without a canonical URL, social card, or JSON-LD',
      )
    }
    const outPath = resolve(siteRoot, page.out)
    mkdirSync(dirname(outPath), { recursive: true })
    writeFileSync(outPath, pageTemplate(title, html, seo.description), 'utf8')
    outputs.push(outPath)
  }
  return outputs
}
