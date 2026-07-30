/**
 * safety.sweep.ts - reveal safety net for hidden-start scroll animations.
 *
 * The landing page reveals many elements with GSAP hidden-start animations
 * (`gsap.from`/`gsap.set` to opacity 0, restored when a ScrollTrigger fires). That
 * pattern has a sharp edge, and it is the reason CR-0070 Phase 3 exists: an element
 * given a hidden start state whose trigger never fires — because it has no scroll
 * distance to travel, or because ScrollTrigger measured the layout wrong — stays
 * transparent permanently. On CR-0069 this shipped as content that was invisible to
 * real users even though the pre-rendered markup contained it.
 *
 * This module is the safety net FR-11 mandates: a one-shot sweep, run a short time
 * after load, that clears the inline opacity and transform GSAP leaves behind on any
 * element still transparent. It touches only inline styles GSAP set, and it skips
 * scrub-driven crossfade sections (marked `data-scrub-section`), whose panels are
 * transparent by design and are re-asserted by GSAP on the next scroll. It never
 * hides anything; it can only reveal, so it cannot itself defeat the pre-render.
 *
 * @agents-index Client safety net: after load, clears leftover opacity/transform on stuck-transparent reveal elements.
 */

/**
 * DELAY_MS is how long after load the sweep waits before revealing stragglers. It is
 * long enough for legitimate above-the-fold entrance animations (all well under a
 * second) to finish, so the sweep only ever catches genuinely stuck elements.
 */
const DELAY_MS = 2500

/**
 * clearIfTransparent clears the inline opacity and transform GSAP set on an element,
 * but only when the element is currently transparent. Elements GSAP has already
 * revealed (opacity restored to 1) are left untouched.
 *
 * @param el  The element to inspect and possibly reveal.
 */
function clearIfTransparent(el: HTMLElement): void {
  // Only act on elements GSAP left transparent via an inline opacity. Reading the
  // inline style (not the computed one) keeps the sweep to elements an animation set,
  // and avoids fighting CSS-driven visibility such as responsive display toggles.
  const inlineOpacity = el.style.opacity
  if (inlineOpacity === '' || Number(inlineOpacity) > 0) return
  el.style.removeProperty('opacity')
  el.style.removeProperty('transform')
}

/**
 * runSweep performs a single reveal pass over the landing-page content.
 *
 * It selects elements carrying an inline opacity inside <main>, excludes any within a
 * scrub-driven crossfade section, and clears the leftover transform/opacity on those
 * still transparent.
 */
function runSweep(): void {
  const candidates = document.querySelectorAll<HTMLElement>('main [style*="opacity"]')
  candidates.forEach((el) => {
    if (el.closest('[data-scrub-section]')) return
    clearIfTransparent(el)
  })
}

/**
 * installSafetySweep schedules the reveal sweep to run once, a short time after the
 * page has loaded. Idempotent import-time side effect; safe to import from the client
 * entry.
 *
 * Under reduced motion the entrance animations are already disabled and content is
 * shown immediately, so the sweep is harmless there too and needs no special-casing.
 */
export function installSafetySweep(): void {
  if (typeof window === 'undefined') return
  const schedule = () => window.setTimeout(runSweep, DELAY_MS)
  if (document.readyState === 'complete') {
    schedule()
  } else {
    window.addEventListener('load', schedule, { once: true })
  }
}
