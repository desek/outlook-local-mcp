/**
 * docs.entry.ts - client entry for the generated documentation pages.
 *
 * The documentation pages (concepts, quickstart, troubleshooting) are static HTML
 * generated from Markdown; their full text exists without JavaScript (CR-0070 FR-8).
 * This entry exists only so Vite bundles the shared stylesheet into each page and
 * rewrites the asset URLs. It intentionally renders nothing and awaits nothing, so a
 * failure to load JavaScript never hides the already-present content.
 *
 * @agents-index Docs-page client entry: imports the shared stylesheet only, renders nothing, awaits nothing.
 */
import './index.css'
