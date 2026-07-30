/**
 * use-scrollbar-height.ts - publishes an element's horizontal scrollbar height as a CSS variable.
 *
 * A horizontal scrollbar is laid out inside its element's padding box, at the
 * bottom, so it shrinks the content area from below. Anything centred in that
 * content area therefore sits half a scrollbar height above the element's true
 * centre, which is why the install command's text drifts above the copy button
 * beside it. Measured on this project: an 11px scrollbar produced a 5.8px offset.
 *
 * The height cannot be hard-coded. It is 11px here, differs across platforms and
 * themes, and is 0 wherever the OS uses overlay scrollbars, where a fixed
 * compensation would introduce the very offset it was meant to remove. It also
 * changes when the element stops overflowing. So it is measured from the element
 * itself (`offsetHeight - clientHeight`) and published as `--sb`, which the
 * stylesheet consumes as `padding-top`, restoring symmetry between the space above
 * the text and the scrollbar below it.
 *
 * Degrades to 0 without JavaScript, which is the pre-rendered default: the layout
 * is then exactly what it was before this hook existed, never worse.
 *
 * @agents-index Measures an element's horizontal scrollbar height into a --sb CSS variable, recomputed on resize.
 */
import { useEffect, type RefObject } from 'react'

/**
 * Keeps `--sb` on the referenced element equal to its horizontal scrollbar height.
 * Recomputes on viewport resize, since overflow (and therefore the scrollbar) comes
 * and goes with available width.
 */
export function useScrollbarHeight(ref: RefObject<HTMLElement | null>): void {
  useEffect(() => {
    const el = ref.current
    if (!el) {
      return
    }
    const measure = () => {
      const sb = el.offsetHeight - el.clientHeight
      el.style.setProperty('--sb', `${sb > 0 ? sb : 0}px`)
    }
    measure()
    window.addEventListener('resize', measure)
    return () => window.removeEventListener('resize', measure)
  }, [ref])
}
