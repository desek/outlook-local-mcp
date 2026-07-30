import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { provenancePlugin } from './build/provenance.plugin'

export default defineConfig({
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
})
