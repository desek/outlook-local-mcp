/**
 * seo.plugin.ts - Vite plugin for the crawler surface and per-page SEO metadata.
 *
 * Implements the head-injection and root-file half of CR-0070 Phase 4:
 *
 *  1. transformIndexHtml injects the canonical, Open Graph, Twitter card, and JSON-LD
 *     into every page's <head> before </head>, keyed off the HTML file being built, so
 *     each page carries its own absolute-apex metadata (FR-21 to FR-23, FR-39 to FR-45).
 *  2. generateBundle emits sitemap.xml from the page registry (FR-19) and copies the
 *     repository-root llms.txt into the output byte-for-byte (FR-20), so both are build
 *     outputs a rebuild cannot revert (FR-54) and the served llms.txt cannot diverge
 *     from the tracked one (AC-5).
 *
 * robots.txt is a static public/ asset rather than an emitted one; Vite copies public/
 * verbatim, which already makes it a build output. Keeping it a real file lets the
 * deliberate AI-crawler allowance be reviewed as source (FR-18).
 *
 * @agents-index Vite plugin: injects per-page SEO head metadata and emits sitemap.xml and the copied llms.txt.
 */
import { basename } from 'node:path'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import type { Plugin } from 'vite'
import { pageForFile } from './seo.pages'
import { renderSeoHead } from './seo.head'
import { renderJsonLd } from './seo.jsonld'
import { buildSitemap } from './sitemap'

/**
 * repoRootLlmsTxt reads the repository-root llms.txt so the served copy is byte-identical
 * to the tracked source (AC-5). The path is resolved from this module's location: the
 * repository root is two levels above site/build/.
 *
 * @returns The raw llms.txt contents.
 * @throws Error if llms.txt is missing, failing the build loudly rather than shipping a
 *   site without it (FR-20).
 */
function repoRootLlmsTxt(): string {
  const here = fileURLToPath(new URL('.', import.meta.url))
  const llmsPath = fileURLToPath(new URL('../../llms.txt', import.meta.url))
  try {
    return readFileSync(llmsPath, 'utf8')
  } catch {
    throw new Error(
      `llms.txt is missing (expected at the repository root, resolved from ${here}); ` +
        'the site build copies it and cannot publish without it',
    )
  }
}

/**
 * seoPlugin builds the Vite plugin.
 *
 * @returns A Vite Plugin injecting per-page SEO head metadata and emitting sitemap.xml
 *   and the copied llms.txt.
 */
export function seoPlugin(): Plugin {
  return {
    name: 'seo-crawler-surface',
    // Inject the per-page metadata immediately before </head> for the matching page.
    transformIndexHtml: {
      order: 'pre',
      handler(html: string, ctx) {
        const file = basename(ctx.path || ctx.filename || '')
        const page = pageForFile(file)
        if (!page) return html
        const head = `${renderSeoHead(page)}\n${renderJsonLd(page)}\n  </head>`
        return html.replace('</head>', head)
      },
    },
    // Emit the crawler root files as build outputs.
    generateBundle() {
      this.emitFile({
        type: 'asset',
        fileName: 'sitemap.xml',
        source: buildSitemap(),
      })
      this.emitFile({
        type: 'asset',
        fileName: 'llms.txt',
        source: repoRootLlmsTxt(),
      })
    },
  }
}
