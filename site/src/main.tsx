/**
 * main.tsx - client entry point.
 *
 * Renders synchronously. The version this was vendored from wrapped the render in
 * an async bootstrap that awaited Mock Service Worker's `worker.start()` before
 * calling `createRoot().render()`. That is the defect that took the live site
 * down: if the worker fails to register for any reason, React never mounts and the
 * page stays blank. Nothing may be awaited before the first render.
 *
 * MSW itself was removed with it. It served a single `/api/tools` endpoint that no
 * component consumed, its mock data still described the retired flat tool naming,
 * and it shipped roughly 250 KB into a static marketing build.
 *
 * The root is pre-rendered at build time (react-dom/server, see entry-server.tsx), so
 * this entry HYDRATES the existing markup rather than replacing it. Hydration attaches
 * behaviour without discarding the server-rendered content, so a slow or failed script
 * load still leaves the full text on screen. installSafetySweep is the net for GSAP's
 * hidden-start reveals that never fire (CR-0070 FR-11).
 *
 * @agents-index Client entry: registers GSAP plugins, hydrates the pre-rendered root, and installs the reveal safety net; nothing awaited before first paint.
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
