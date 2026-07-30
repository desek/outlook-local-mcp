/**
 * doc.pages.ts - build-time generation of the documentation HTML entries.
 *
 * The site publishes the three narrative docs (concepts, quickstart, troubleshooting)
 * as crawlable HTML pages at stable URLs (CR-0070 FR-13). Each page is generated from
 * the Markdown in docs/ at build time and emitted as a Vite HTML input, so Vite hashes
 * its assets and the provenance plugin injects its head meta exactly as it does for the
 * landing page. The Markdown stays the single source of truth and is never copied into
 * site/ (FR-14).
 *
 * generateDocPages is the load-bearing guard for FR-16: if a consumed Markdown file has
 * been renamed or removed it throws, naming the missing file, so the build fails loudly
 * rather than publishing a site that has silently lost a page.
 *
 * @agents-index Generates the concepts/quickstart/troubleshooting HTML entries from docs/*.md, failing loudly on a missing file.
 */
import { readFileSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { renderMarkdown } from './doc.markdown'

/**
 * DocPage describes one publishable documentation page.
 *
 * @property slug  The output basename; the page is served at "/<slug>.html".
 * @property source  The Markdown file path relative to the repository root.
 */
export interface DocPage {
  slug: string
  source: string
}

/**
 * DOC_PAGES is the fixed set of narrative docs published to the site. Adding a page is
 * a deliberate edit here; the set is not discovered, so a stray Markdown file cannot
 * silently become a public page.
 */
export const DOC_PAGES: readonly DocPage[] = [
  { slug: 'concepts', source: 'docs/concepts.md' },
  { slug: 'quickstart', source: 'docs/quickstart.md' },
  { slug: 'troubleshooting', source: 'docs/troubleshooting.md' },
]

/**
 * pageTemplate wraps a rendered Markdown fragment in a full HTML document.
 *
 * The head carries a title and description and a module script that pulls in the
 * shared stylesheet; the provenance plugin injects its meta tags before </head> at
 * build time. The body holds exactly one <h1> (the document's own top heading) inside
 * a <main>, so the page satisfies the single-<h1> rule without pre-rendering React.
 *
 * @param title  The page title, from the document's first level-1 heading.
 * @param body  The rendered HTML fragment for the document body.
 * @returns A complete HTML document string.
 */
function pageTemplate(title: string, body: string): string {
  return `<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <link rel="icon" type="image/svg+xml" href="/icon.svg" />
    <link rel="icon" type="image/png" href="/icon.png" />
    <title>${escapeHtml(title)} — Outlook Local MCP</title>
    <meta name="description" content="${escapeHtml(title)} documentation for Outlook Local MCP, the local Model Context Protocol server for Microsoft Outlook Calendar and Mail." />
    <script type="module" src="/src/docs.entry.ts"></script>
  </head>
  <body class="antialiased">
    <main class="doc-page">
${body}
    </main>
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
 * root) and rendered with Go-compatible heading anchors, then written as "<slug>.html"
 * at the site root so Vite can treat it as an HTML input.
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
    const outPath = resolve(siteRoot, `${page.slug}.html`)
    writeFileSync(outPath, pageTemplate(title, html), 'utf8')
    outputs.push(outPath)
  }
  return outputs
}
