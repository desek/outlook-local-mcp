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
 * @agents-index Client entry: registers GSAP plugins and renders synchronously, nothing awaited before first paint.
 */
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import gsap from 'gsap'
import { useGSAP } from '@gsap/react'
import { ScrollTrigger } from 'gsap/ScrollTrigger'
import App from './App'
import './index.css'

gsap.registerPlugin(useGSAP, ScrollTrigger)

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
