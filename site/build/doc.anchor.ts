/**
 * doc.anchor.ts - heading-to-anchor slug generation for the documentation pages.
 *
 * The site publishes docs/concepts.md, docs/quickstart.md, and docs/troubleshooting.md
 * as HTML. Verb `SeeDocs` values in the Go server (for example
 * "concepts#mail-gating") deep-link into those pages, so the anchor id a heading
 * receives in the published HTML MUST match what the Go server computes, or every
 * `system.help` deep link breaks silently (CR-0070 FR-15).
 *
 * The only safe way to guarantee agreement is to mirror the production algorithm
 * byte for byte rather than trust a Markdown renderer's default slugger. The mirror
 * here reproduces `headingToAnchor` in internal/tools/get_docs.go exactly: lower-case,
 * keep a-z, 0-9, and '-', map spaces to hyphens, and drop everything else. Explicit
 * `{#anchor}` overrides in a heading win over the computed slug, matching how the docs
 * are authored.
 *
 * @agents-index Anchor slugger mirroring the Go headingToAnchor byte-for-byte, plus {#override} parsing.
 */

/**
 * Matches a trailing explicit anchor override of the form `{#custom-id}`, allowing
 * surrounding whitespace. The captured group is the raw id with no braces or hash.
 */
const OVERRIDE_RE = /\s*\{#([A-Za-z0-9-]+)\}\s*$/

/**
 * headingToAnchor converts a Markdown heading string to its anchor form.
 *
 * This is a byte-for-byte mirror of headingToAnchor in internal/tools/get_docs.go.
 * It lower-cases, keeps a-z / 0-9 / '-', maps spaces to '-', and drops every other
 * character. It MUST NOT diverge from the Go implementation.
 *
 * @param heading  The raw heading text (the source after the leading '#' markers).
 * @returns The anchor slug, for use as an element id.
 */
export function headingToAnchor(heading: string): string {
  const lower = heading.trim().toLowerCase()
  let out = ''
  for (const r of lower) {
    if ((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r === '-') {
      out += r
    } else if (r === ' ') {
      out += '-'
    }
    // Everything else is dropped, exactly as the Go implementation does.
  }
  return out
}

/**
 * ResolvedHeading is the outcome of interpreting a Markdown heading's text.
 *
 * @property anchor  The id to assign the heading element (override if present, else computed).
 * @property override  The explicit `{#...}` id if the heading declared one, otherwise null.
 */
export interface ResolvedHeading {
  anchor: string
  override: string | null
}

/**
 * resolveHeadingAnchor determines the anchor id for a heading.
 *
 * An explicit `{#custom-id}` suffix takes precedence (FR-15); otherwise the anchor is
 * computed from the heading text with the display override stripped so the trailing
 * `{#...}` never leaks into the slug.
 *
 * @param text  The heading's raw text, possibly ending in a `{#override}` suffix.
 * @returns The resolved anchor and the override if one was present.
 */
export function resolveHeadingAnchor(text: string): ResolvedHeading {
  const match = text.match(OVERRIDE_RE)
  if (match) {
    return { anchor: match[1]!, override: match[1]! }
  }
  return { anchor: headingToAnchor(text), override: null }
}

/**
 * stripOverride removes a trailing `{#...}` suffix from a string.
 *
 * Used to keep the override out of the rendered heading text, since it is metadata
 * that selects the id and must never appear as visible content.
 *
 * @param text  A string that may end in a `{#override}` suffix.
 * @returns The string with any trailing override removed.
 */
export function stripOverride(text: string): string {
  return text.replace(OVERRIDE_RE, '')
}
