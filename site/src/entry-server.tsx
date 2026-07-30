/**
 * entry-server.tsx - server-render entry for build-time pre-rendering.
 *
 * The landing page is a single-page React app with no router, so pre-rendering it is a
 * one-shot react-dom/server render at build time rather than a routing framework or a
 * headless browser (CR-0070 FR-8, FR-9). This module exports the render function the
 * post-build prerender script calls; it produces the same tree the client mounts, so
 * hydration matches.
 *
 * It deliberately does NOT import the client entry (main.tsx) or register GSAP plugins:
 * renderToString never runs effects, and everything that touches window, document, or a
 * canvas lives inside effects and event handlers, so the tree renders cleanly on the
 * server without any browser-API guards firing.
 *
 * @agents-index Server render entry: renders App to an HTML string via renderToString for build-time pre-rendering.
 */
import { StrictMode } from 'react'
import { renderToString } from 'react-dom/server'
import App from './App'

/**
 * render produces the pre-rendered HTML for the landing page.
 *
 * @returns The App tree serialized to an HTML string, wrapped in StrictMode to match
 *   the client's hydration root exactly.
 */
export function render(): string {
  return renderToString(
    <StrictMode>
      <App />
    </StrictMode>,
  )
}
