/**
 * @agents-index Page-init script that makes the site render deterministically for pixel comparison.
 *
 * Purpose:
 *   Two of the site's visuals (the hero background and the brand-reveal particles) are
 *   canvases driven by `requestAnimationFrame` and seeded with `Math.random`, and the
 *   GSAP/Lenis stack is likewise rAF-driven. Screenshotting them as-is produces a
 *   different image on every run, which would drown a visual-regression comparison in
 *   noise and make the "no visual regression" acceptance criterion unmeasurable.
 *
 *   This module supplies an init script (installed via `evaluateOnNewDocument`, so it
 *   runs before any page script) that replaces the two sources of nondeterminism:
 *
 *     1. A *virtual clock*. `requestAnimationFrame` becomes a queue that only advances
 *        when the harness calls `window.__pump(frames)`, and `performance.now`/`Date.now`
 *        report the virtual time. Animation state then depends solely on the number of
 *        frames pumped, which the harness holds fixed across builds.
 *     2. A *seeded* `Math.random` (mulberry32). Particle initialisation becomes a pure
 *        function of the seed, so the same build draws the same particles every time.
 *     3. A *synchronous* `IntersectionObserver`. The canvases only animate while their
 *        element intersects the viewport, so the number of frames they have advanced
 *        depends on when the observer callback is delivered — which the platform
 *        schedules against real time. Recomputing intersection from geometry at the top
 *        of every pumped frame makes visibility a pure function of scroll position and
 *        frame count instead.
 *     4. A *synchronous* `ResizeObserver`, for the same reason: the canvases re-seed
 *        their backing store from the observed size, so a resize delivered on a
 *        different frame shifts the whole animation by that many frames.
 *
 *   The consequence is that a pixel difference between two captures means the *code*
 *   changed, which is exactly the signal the acceptance criterion is asking for.
 *
 * Usage (as a module):
 *   import { DETERMINISM_INIT } from './site.determinism.mjs'
 *   await page.evaluateOnNewDocument(DETERMINISM_INIT)
 *   // ...after load:
 *   await page.evaluate(() => window.__pump(60))
 */

/**
 * Source of the init script, as a function suitable for `page.evaluateOnNewDocument`.
 *
 * Side effects (inside the page): replaces `Math.random`, `requestAnimationFrame`,
 * `cancelAnimationFrame`, `performance.now` and `Date.now`, and defines `window.__pump`.
 */
export const DETERMINISM_INIT = () => {
  // Seeded PRNG (mulberry32) so particle layouts repeat exactly across runs.
  let seed = 0x9e3779b9
  Math.random = () => {
    seed |= 0
    seed = (seed + 0x6d2b79f5) | 0
    let t = Math.imul(seed ^ (seed >>> 15), 1 | seed)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }

  // Virtual clock: animation time advances only when the harness pumps frames.
  let virtualTime = 0
  const origin = 1767225600000 // fixed wall-clock epoch, so Date.now is stable too
  let callbacks = []
  let nextHandle = 1

  window.requestAnimationFrame = (cb) => {
    callbacks.push({ handle: nextHandle, cb })
    return nextHandle++
  }
  window.cancelAnimationFrame = (handle) => {
    callbacks = callbacks.filter((entry) => entry.handle !== handle)
  }
  performance.now = () => virtualTime
  Date.now = () => origin + virtualTime

  // Synchronous IntersectionObserver: intersection is recomputed from live geometry at
  // the top of every pumped frame, and the callback runs inline when the state flips.
  const observers = new Set()

  class SynchronousIntersectionObserver {
    constructor(callback) {
      this.callback = callback
      this.states = new Map()
      observers.add(this)
    }

    observe(target) {
      this.states.set(target, null)
      this.settle()
    }

    unobserve(target) {
      this.states.delete(target)
    }

    disconnect() {
      this.states.clear()
      observers.delete(this)
    }

    takeRecords() {
      return []
    }

    /** Recompute every observed target and fire the callback for those that changed. */
    settle() {
      const entries = []
      for (const [target, previous] of this.states) {
        const rect = target.getBoundingClientRect()
        const isIntersecting =
          rect.bottom > 0 &&
          rect.right > 0 &&
          rect.top < (window.innerHeight || 0) &&
          rect.left < (window.innerWidth || 0)
        if (isIntersecting === previous) continue
        this.states.set(target, isIntersecting)
        entries.push({
          target,
          isIntersecting,
          intersectionRatio: isIntersecting ? 1 : 0,
          boundingClientRect: rect,
          intersectionRect: rect,
          rootBounds: null,
          time: virtualTime,
        })
      }
      if (entries.length) this.callback(entries, this)
    }
  }

  window.IntersectionObserver = SynchronousIntersectionObserver

  class SynchronousResizeObserver {
    constructor(callback) {
      this.callback = callback
      this.sizes = new Map()
      observers.add(this)
    }

    observe(target) {
      this.sizes.set(target, null)
      this.settle()
    }

    unobserve(target) {
      this.sizes.delete(target)
    }

    disconnect() {
      this.sizes.clear()
      observers.delete(this)
    }

    /** Fire the callback for every target whose border-box size changed since last frame. */
    settle() {
      const entries = []
      for (const [target, previous] of this.sizes) {
        const rect = target.getBoundingClientRect()
        const key = `${rect.width}x${rect.height}`
        if (key === previous) continue
        this.sizes.set(target, key)
        entries.push({
          target,
          contentRect: rect,
          borderBoxSize: [{ inlineSize: rect.width, blockSize: rect.height }],
          contentBoxSize: [{ inlineSize: rect.width, blockSize: rect.height }],
          devicePixelContentBoxSize: [{ inlineSize: rect.width, blockSize: rect.height }],
        })
      }
      if (entries.length) this.callback(entries, this)
    }
  }

  window.ResizeObserver = SynchronousResizeObserver

  /**
   * Run `frames` animation frames of `dt` milliseconds each.
   * Each frame drains the queue snapshot, so callbacks that re-register (the normal
   * animation-loop shape) run exactly once per pumped frame rather than spinning.
   */
  window.__pump = (frames = 60, dt = 1000 / 60) => {
    for (let i = 0; i < frames; i++) {
      virtualTime += dt
      for (const observer of observers) observer.settle()
      const batch = callbacks
      callbacks = []
      for (const entry of batch) {
        try {
          entry.cb(virtualTime)
        } catch {
          /* a throwing animation callback must not abort the pump */
        }
      }
    }
  }
}
