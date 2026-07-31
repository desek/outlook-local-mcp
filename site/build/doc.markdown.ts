/**
 * doc.markdown.ts - Markdown-to-HTML rendering for the documentation pages.
 *
 * Converts a narrative docs/*.md file into the HTML fragment published on the site,
 * assigning every heading the anchor id the Go server expects (CR-0070 FR-13 to
 * FR-15). The Markdown in docs/ remains the single source of truth; this module only
 * transforms it at build time and never stores a copy.
 *
 * Heading ids come exclusively from resolveHeadingAnchor (the byte-for-byte mirror of
 * the Go slugger), never from marked's default slugger, because a disagreement would
 * break `system.help` deep links silently. The trailing `{#override}` metadata is
 * stripped from the visible heading text.
 *
 * @agents-index Renders a docs Markdown file to an HTML fragment with Go-compatible heading anchors.
 */
import { Marked } from 'marked'
import { resolveHeadingAnchor, stripOverride } from './doc.anchor'

/**
 * RenderedDoc is the result of rendering one Markdown document.
 *
 * @property html  The HTML fragment (the document body, no <html>/<head> wrapper).
 * @property title  The text of the first level-1 heading, used as the page title.
 */
export interface RenderedDoc {
  html: string
  title: string
}

/**
 * renderMarkdown converts a Markdown string to an HTML fragment.
 *
 * A per-call Marked instance is used so the custom heading renderer and captured
 * title never leak between documents. The heading renderer strips any `{#override}`
 * from the visible text, renders the remaining inline Markdown, and stamps the
 * resolved anchor id onto the element.
 *
 * @param markdown  The Markdown source of one documentation file.
 * @returns The rendered HTML fragment and the first h1's text as the title.
 * @throws Error if the document contains no level-1 heading, since the page needs a
 *   single <h1> (FR-10) and a title.
 */
export function renderMarkdown(markdown: string): RenderedDoc {
  let title: string | null = null
  const marked = new Marked({
    // Deterministic output: no smartypants or GitHub-only extensions that could
    // shift text and diverge from the source the anchors are computed against.
    gfm: true,
  })
  marked.use({
    renderer: {
      heading(token) {
        const { anchor } = resolveHeadingAnchor(token.text)
        // Render the inline content, then strip the override literal so the visible
        // heading never shows the `{#...}` metadata.
        const inner = stripOverride(this.parser.parseInline(token.tokens))
        if (token.depth === 1 && title === null) {
          title = stripOverride(token.text).trim()
        }
        return `<h${token.depth} id="${anchor}">${inner}</h${token.depth}>\n`
      },
    },
  })
  const html = marked.parse(markdown) as string
  if (title === null) {
    throw new Error('document has no level-1 heading; a single <h1> and a title are required')
  }
  return { html, title }
}
