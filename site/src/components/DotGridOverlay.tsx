/**
 * DotGridOverlay — Recurring dot-grid motif for dark sections.
 * ~80px cells with tiny lime-green 3x3px square dots at intersections.
 */
import { useId } from 'react'

export default function DotGridOverlay({ className = '' }: { className?: string }) {
  // Unique per instance, so the desktop and mobile copies of this SVG do not share ids.
  const uid = useId()

  return (
    <div
      className={`absolute inset-0 z-0 pointer-events-none overflow-hidden ${className}`}
      aria-hidden="true"
    >
      <svg width="100%" height="100%" xmlns="http://www.w3.org/2000/svg">
        <defs>
          <pattern
            id={`dot-grid-pattern-${uid}`}
            x="0"
            y="0"
            width="80"
            height="80"
            patternUnits="userSpaceOnUse"
          >
            {/* Grid lines — extremely faint */}
            <line x1="0" y1="0" x2="80" y2="0" stroke="var(--color-brand-lime)" strokeOpacity="0.08" strokeWidth="1" />
            <line x1="0" y1="0" x2="0" y2="80" stroke="var(--color-brand-lime)" strokeOpacity="0.08" strokeWidth="1" />
            {/* Dot at intersection */}
            <rect x="-1.5" y="-1.5" width="3" height="3" fill="var(--color-brand-lime)" opacity="0.5" />
          </pattern>
        </defs>
        <rect width="100%" height="100%" fill={`url(#dot-grid-pattern-${uid})`} />
      </svg>
    </div>
  )
}
