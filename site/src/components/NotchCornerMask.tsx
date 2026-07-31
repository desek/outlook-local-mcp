/**
 * NotchCornerMask — Concave corner cutout SVG for section transitions.
 * The single most distinctive design element from Terminal Industries.
 */

interface NotchCornerMaskProps {
  position: 'top' | 'bottom'
  color: string
  radius?: number
  className?: string
}

export default function NotchCornerMask({
  position,
  color,
  radius = 80,
  className = '',
}: NotchCornerMaskProps) {
  const r = radius
  const h = r

  const topD = `M 0 0 L 0 ${h} C 0 ${h * 0.1} ${r * 0.1} 0 ${r} 0 L ${100 - r} 0 C ${100 - r * 0.1} 0 100 ${h * 0.1} 100 ${h} L 100 0 Z`
  const bottomD = `M 0 ${h} L 0 0 C 0 ${h * 0.9} ${r * 0.1} ${h} ${r} ${h} L ${100 - r} ${h} C ${100 - r * 0.1} ${h} 100 ${h * 0.9} 100 0 L 100 ${h} Z`

  return (
    <div
      className={`absolute left-0 right-0 pointer-events-none z-10 ${className}`}
      style={{
        [position === 'top' ? 'top' : 'bottom']: 0,
        height: h,
        transform: position === 'bottom' ? 'translateY(1px)' : 'translateY(-1px)',
      }}
      aria-hidden="true"
    >
      <svg
        width="100%"
        height={h}
        viewBox={`0 0 100 ${h}`}
        preserveAspectRatio="none"
        style={{ display: 'block', width: '100%', height: '100%' }}
      >
        <path d={position === 'top' ? topD : bottomD} fill={color} />
      </svg>
    </div>
  )
}
