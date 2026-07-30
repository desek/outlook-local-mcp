import { resolve, basename } from 'node:path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { provenancePlugin } from './build/provenance.plugin'
import { generateDocPages } from './build/doc.pages'

// The site root is this config file's directory. Documentation pages are generated
// into it as HTML inputs and the landing page (index.html) sits here too.
const siteRoot = __dirname

export default defineConfig(({ isSsrBuild }) => {
  // For the client build and dev server the documentation pages are generated from
  // docs/*.md and registered as Vite HTML inputs, so each is hashed and receives its
  // provenance head meta exactly like the landing page. generateDocPages throws,
  // naming the file, if a consumed doc is missing (CR-0070 FR-16), failing the build
  // loudly. The SSR build renders only entry-server.tsx, so it neither generates the
  // pages nor registers the multi-page inputs.
  const docInputs = isSsrBuild ? [] : generateDocPages(siteRoot)

  return {
    // Served from the apex custom domain outlook-local-mcp.com, so assets resolve
    // at the root. The '/outlook-local-mcp/' base this file shipped with is correct
    // only for a github.io PROJECT page, where GitHub serves the branch root at
    // that subpath. With a custom domain the site moves to '/', the prefix stops
    // resolving, and every asset 404s. That is exactly what took the live site down
    // and it is the single highest-risk line in this config.
    base: '/',
    // provenancePlugin stamps build provenance (commit, UTC build time, workflow run)
    // into every HTML head as <meta> tags and emits /build-info.json, injected from the
    // CI environment with an explicit local-build fallback (CR-0070 FR-26 to FR-30).
    plugins: [react(), tailwindcss(), provenancePlugin()],
    resolve: {
      dedupe: ['react', 'react-dom'],
    },
    build: isSsrBuild
      ? {}
      : {
          rollupOptions: {
            // The landing page plus the three generated documentation pages, each a
            // separate Vite HTML entry pre-rendered without a router (CR-0070 FR-13).
            input: {
              index: resolve(siteRoot, 'index.html'),
              ...Object.fromEntries(
                docInputs.map((p) => [basename(p, '.html'), p]),
              ),
            },
          },
        },
  }
})
