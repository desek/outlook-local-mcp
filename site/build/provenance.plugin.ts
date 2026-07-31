/**
 * provenance.plugin.ts - Vite plugin that stamps build provenance into the artifact.
 *
 * Implements CR-0070 FR-26 to FR-30 by doing two things at build time:
 *
 *  1. Injecting the provenance record as <meta> tags into the <head> of every HTML
 *     entry, so it is readable without executing JavaScript (FR-27).
 *  2. Emitting /build-info.json at the site root carrying the same fields (FR-28).
 *
 * The <meta> tags are injected into the head, never the body. This is deliberate and
 * forward-compatible with Phase 3: pre-rendering (react-dom/server) replaces the
 * empty <div id="root"> in the body with rendered markup and leaves the head
 * untouched, so provenance survives that change without any edit here. build-info.json
 * is a standalone emitted asset and is likewise independent of the body.
 *
 * @agents-index Vite plugin: injects provenance meta tags into every HTML head and emits build-info.json.
 */
import type { Plugin } from 'vite'
import { computeProvenance, type BuildProvenance } from './provenance'

/**
 * Meta tag names carrying provenance. Namespaced under "build:" so they are
 * unambiguous and greppable, and so they cannot collide with SEO meta added in later
 * phases.
 */
const META = {
  commit: 'build:commit',
  buildTime: 'build:time',
  run: 'build:run',
  environment: 'build:environment',
} as const

/**
 * renderMetaTags renders the provenance record as a block of <meta> tags.
 *
 * @param p  The provenance record to render.
 * @returns An HTML fragment of newline-separated <meta> tags, indented for the head.
 */
function renderMetaTags(p: BuildProvenance): string {
  return [
    `<meta name="${META.commit}" content="${p.commit}" />`,
    `<meta name="${META.buildTime}" content="${p.buildTime}" />`,
    `<meta name="${META.run}" content="${p.run}" />`,
    `<meta name="${META.environment}" content="${p.environment}" />`,
  ]
    .map((tag) => `    ${tag}`)
    .join('\n')
}

/**
 * provenancePlugin builds the Vite plugin.
 *
 * The provenance record is computed once, at plugin construction, so the meta tags
 * and build-info.json describe the same instant and the same origin. FR-30's
 * local-build fallback is handled entirely inside computeProvenance.
 *
 * @returns A Vite Plugin that injects provenance meta tags and emits build-info.json.
 */
export function provenancePlugin(): Plugin {
  const provenance = computeProvenance()
  return {
    name: 'build-provenance',
    // Inject the meta tags immediately before </head> for every HTML entry.
    transformIndexHtml: {
      order: 'pre',
      handler(html: string) {
        return html.replace('</head>', `${renderMetaTags(provenance)}\n  </head>`)
      },
    },
    // Emit /build-info.json alongside the HTML so the same fields are fetchable.
    generateBundle() {
      this.emitFile({
        type: 'asset',
        fileName: 'build-info.json',
        source: `${JSON.stringify(provenance, null, 2)}\n`,
      })
    },
  }
}
