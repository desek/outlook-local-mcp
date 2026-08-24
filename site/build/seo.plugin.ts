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
import { isAbsolute, relative } from 'node:path'
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
 * sitePathForTransform reduces a transformIndexHtml context to the page's site-relative
 * path, which is the registry's match key.
 *
 * The basename is deliberately not used: the trust anchor pages are emitted as directory
 * indexes, so several pages share the filename index.html and only the path distinguishes
 * them. Matching on the basename would silently give every one of them the landing page's
 * canonical URL and JSON-LD (CR-0077).
 *
 * @param path  ctx.path, the request or output path, normally leading-slashed.
 * @param filename  ctx.filename, the absolute path of the source HTML file.
 * @param root  The resolved Vite root, used to relativise an absolute filename.
 * @returns The site-relative path with no leading slash, for example "about/index.html".
 */
function sitePathForTransform(path: string, filename: string, root: string): string {
  const raw = (path || filename || '').split('?')[0].split('#')[0]
  // ctx.path is site-relative but leading-slashed; ctx.filename is an absolute disk path
  // and only meaningful once relativised against the Vite root.
  const rel = root && isAbsolute(raw) && raw.startsWith(root) ? relative(root, raw) : raw
  return rel.replace(/\\/g, '/').replace(/^\/+/, '')
}

/**
 * seoPlugin builds the Vite plugin.
 *
 * @returns A Vite Plugin injecting per-page SEO head metadata and emitting sitemap.xml
 *   and the copied llms.txt.
 */
export function seoPlugin(): Plugin {
  let root = ''
  return {
    name: 'seo-crawler-surface',
    configResolved(config) {
      root = config.root
    },
    // Inject the per-page metadata immediately before </head> for the matching page.
    transformIndexHtml: {
      order: 'pre',
      handler(html: string, ctx) {
        const file = sitePathForTransform(ctx.path, ctx.filename, root)
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
