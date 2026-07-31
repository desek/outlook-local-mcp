/**
 * index.md.emit.ts - derives the landing page Markdown from its pre-rendered HTML.
 *
 * CR-0070 FR-31 to FR-33 require a Markdown representation of the landing page at
 * /index.md that is generated from the same source as the landing page HTML (so the two
 * cannot diverge, FR-32), is never hand-authored, and re-expresses the five SVG diagrams
 * as Mermaid fences (FR-33).
 *
 * This module takes the exact HTML string react-dom/server produces for the landing page
 * and walks it once, emitting the headings, paragraphs, and list items in document order
 * as Markdown. Two transformations carry the diagram requirement and de-duplicate the
 * responsive layout:
 *
 *  - each SVG bearing a `data-diagram` attribute is replaced by the Mermaid fence for
 *    that key (emitted once per key), and every other, decorative SVG is dropped;
 *  - blocks are de-duplicated by their normalised text, which collapses the desktop and
 *    mobile copies of the same content the landing page renders twice.
 *
 * Deriving from the rendered HTML is what makes the Markdown provably the same content:
 * there is no second copy to maintain.
 *
 * @agents-index Converts the pre-rendered landing HTML into Markdown, substituting Mermaid for the data-diagram SVGs and de-duplicating the responsive copies.
 */
import { renderDiagramMarkdown } from './index.md.mermaid'

/** Marker substituted for a diagram SVG before block extraction, carrying its key. */
const DIAGRAM_MARKER = /@@DIAGRAM:([a-z-]+)@@/

/**
 * decodeEntities decodes the HTML entities react-dom/server emits in text content.
 *
 * @param s  The escaped text.
 * @returns The text with the common named and numeric entities resolved.
 */
function decodeEntities(s: string): string {
  return s
    .replace(/&amp;/g, '&')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&#x27;/g, "'")
    .replace(/&nbsp;/g, ' ')
    .replace(/&#8599;/g, '')
}

/**
 * stripTags removes any remaining inline HTML tags from a block's inner HTML and
 * collapses whitespace to single spaces, yielding the block's plain text.
 *
 * @param inner  The inner HTML of a block element.
 * @returns The decoded, whitespace-collapsed text content.
 */
function stripTags(inner: string): string {
  const withBreaks = inner.replace(/<br\s*\/?>/gi, ' ')
  const text = withBreaks.replace(/<[^>]+>/g, '')
  return decodeEntities(text).replace(/\s+/g, ' ').trim()
}

/**
 * substituteDiagrams replaces every data-diagram SVG with a text marker and strips all
 * other SVGs and the script and style blocks, so block extraction sees clean HTML.
 *
 * @param html  The raw pre-rendered HTML.
 * @returns HTML with diagram markers in place of the diagram SVGs and no other SVG,
 *   script, or style content.
 */
function substituteDiagrams(html: string): string {
  return html
    .replace(/<script\b[^>]*>[\s\S]*?<\/script>/gi, '')
    .replace(/<style\b[^>]*>[\s\S]*?<\/style>/gi, '')
    .replace(
      /<svg\b[^>]*\bdata-diagram="([a-z-]+)"[^>]*>[\s\S]*?<\/svg>/gi,
      (_m, key: string) => `\n@@DIAGRAM:${key}@@\n`,
    )
    .replace(/<svg\b[^>]*>[\s\S]*?<\/svg>/gi, '')
}

/**
 * blockToMarkdown converts one matched block to its Markdown line, or null if the block
 * is empty.
 *
 * @param tag  The lowercased block tag name (h1-h4, p, li, blockquote).
 * @param text  The block's plain text.
 * @returns The Markdown line, or null to skip an empty block.
 */
function blockToMarkdown(tag: string, text: string): string | null {
  if (!text) return null
  switch (tag) {
    case 'h1':
      return `# ${text}`
    case 'h2':
      return `## ${text}`
    case 'h3':
      return `### ${text}`
    case 'h4':
      return `#### ${text}`
    case 'li':
      return `- ${text}`
    case 'blockquote':
      return `> ${text}`
    default:
      return text
  }
}

/**
 * htmlToLandingMarkdown converts the pre-rendered landing HTML to Markdown.
 *
 * @param html  The HTML string react-dom/server produced for the landing page.
 * @returns The Markdown document, newline-terminated. Headings, paragraphs, and list
 *   items appear in document order; the five diagrams appear as Mermaid fences; the
 *   desktop and mobile duplicates are collapsed.
 */
export function htmlToLandingMarkdown(html: string): string {
  const prepared = substituteDiagrams(html)
  // One pass over both the diagram markers and the block elements, preserving order.
  const pattern = new RegExp(
    `${DIAGRAM_MARKER.source}|<(h[1-4]|p|li|blockquote)\\b[^>]*>([\\s\\S]*?)<\\/\\2>`,
    'gi',
  )
  const lines: string[] = []
  const seenText = new Set<string>()
  const seenDiagram = new Set<string>()
  let match: RegExpExecArray | null
  while ((match = pattern.exec(prepared)) !== null) {
    const diagramKey = match[1]
    if (diagramKey) {
      if (seenDiagram.has(diagramKey)) continue
      const md = renderDiagramMarkdown(diagramKey)
      if (md) {
        seenDiagram.add(diagramKey)
        lines.push(md)
      }
      continue
    }
    const tag = (match[2] ?? '').toLowerCase()
    const text = stripTags(match[3] ?? '')
    const md = blockToMarkdown(tag, text)
    if (md === null) continue
    const dedupeKey = `${tag}:${text}`
    if (seenText.has(dedupeKey)) continue
    seenText.add(dedupeKey)
    lines.push(md)
  }
  return `${lines.join('\n\n')}\n`
}
