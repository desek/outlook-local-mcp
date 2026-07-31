/**
 * main.tsx - client entry point.
 *
 * Registers the GSAP plugins, hydrates the pre-rendered root, and installs the reveal
 * safety net. Nothing is awaited before hydration.
 *
 * Why nothing is awaited. The version this was vendored from wrapped the render in an
 * async bootstrap that awaited Mock Service Worker's `worker.start()` before calling
 * `createRoot().render()`. That is the defect that took the live site down: if the worker
 * failed to register for any reason, React never mounted and the page stayed blank. MSW
 * was removed with it; it served a single `/api/tools` endpoint no component consumed, its
 * mock data still described the retired flat tool naming, and it shipped roughly 250 KB
 * into a static marketing build.
 *
 * The root is pre-rendered at build time (react-dom/server, see entry-server.tsx), so this
 * entry HYDRATES the existing markup rather than replacing it. Hydration attaches behaviour
 * without discarding the server-rendered content, so a slow or failed script load still
 * leaves the full text on screen. installSafetySweep is the net for GSAP's hidden-start
 * reveals that never fire (CR-0070 FR-11).
 *
 * When this module runs is not decided here. The pre-render step rewrites the landing
 * page's `<script type="module">` into a bootstrap that injects it once the page has
 * loaded, so this file's cost never competes with the first render. See
 * `build/prerender.mjs`.
 *
 * @agents-index Client entry: registers GSAP plugins, hydrates the pre-rendered root, and installs the reveal safety net; nothing awaited before hydration.
 */
import { StrictMode } from 'react'
import { hydrateRoot } from 'react-dom/client'
import gsap from 'gsap'
import { useGSAP } from '@gsap/react'
import { ScrollTrigger } from 'gsap/ScrollTrigger'
import App from './App'
import { installSafetySweep } from './safety.sweep'
import './index.css'

gsap.registerPlugin(useGSAP, ScrollTrigger)

hydrateRoot(
  document.getElementById('root')!,
  <StrictMode>
    <App />
  </StrictMode>,
)

installSafetySweep()
