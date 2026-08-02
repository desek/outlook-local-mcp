import { useId } from 'react'
interface CapabilityCalendarProps {
  className?: string
}

export default function CapabilityCalendar({ className }: CapabilityCalendarProps) {
  // Unique per instance, so the desktop and mobile copies of this SVG do not share ids.
  const uid = useId()

  // Design system colors from index.css
  const C = {
    bg:       '#052424', // --color-brand-dark
    surface:  '#0a1010', // --color-gray-900
    surface2: '#1a2a2a', // --color-gray-800
    lime:     '#abff02', // --color-brand-lime
    white:    '#ffffff',
    gray600:  '#454742', // --color-gray-600
    gray400:  '#7f7f7f', // --color-gray-400
    gray200:  '#c2c2c2', // --color-gray-200
  } as const

  // Calendar grid geometry
  const gridX      = 24
  const gridY      = 68
  const cellW      = 36
  const cellH      = 28
  const cols       = 7
  const rows       = 5
  const gridWidth  = cols * cellW   // 252
  const gridHeight = rows * cellH   // 140

  const dayLabels = ['Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa', 'Su']

  // Calendar cells: day numbers (row 0 starts on Wednesday = col 2)
  const startOffset = 2
  const totalCells  = cols * rows // 35
  const days = Array.from({ length: totalCells }, (_, idx) => {
    const dayNum = idx - startOffset + 1
    return dayNum > 0 && dayNum <= 30 ? dayNum : null
  })

  // Events: { col, row, span, color, label, opacity }
  const events = [
    { col: 2, row: 0, span: 2, color: C.lime,    opacity: 1,    label: 'Team Standup' },
    { col: 0, row: 1, span: 3, color: '#3b82f6',  opacity: 0.85, label: 'Sprint Planning' },
    { col: 4, row: 1, span: 2, color: '#a855f7',  opacity: 0.75, label: '1:1 Meeting' },
    { col: 1, row: 2, span: 4, color: '#f59e0b',  opacity: 0.8,  label: 'Design Review' },
    { col: 0, row: 3, span: 2, color: '#ef4444',  opacity: 0.75, label: 'Demo' },
    { col: 3, row: 3, span: 3, color: C.lime,    opacity: 0.6,  label: 'Retro' },
    { col: 1, row: 4, span: 3, color: '#3b82f6',  opacity: 0.7,  label: 'OKR Review' },
  ]

  // Annotation callouts: which event index they point to, tool name, offset
  const callouts = [
    {
      eventIdx: 0,
      tool: 'calendar · create_event',
      // tip lands near middle of event bar top edge
      tipColFrac: 0.5,
      tipRow: 0,
      labelX: 330,
      labelY: 74,
    },
    {
      eventIdx: 1,
      tool: 'calendar · search_events',
      tipColFrac: 0.5,
      tipRow: 1,
      labelX: 330,
      labelY: 112,
    },
    {
      eventIdx: 3,
      tool: 'calendar · get_free_busy',
      tipColFrac: 0.5,
      tipRow: 2,
      labelX: 330,
      labelY: 152,
    },
    {
      eventIdx: 4,
      tool: 'calendar · reschedule_event',
      tipColFrac: 0.5,
      tipRow: 3,
      labelX: 330,
      labelY: 192,
    },
  ]

  // Helper: pixel coords of an event bar's centre-top
  function eventTipPoint(ev: typeof events[0], colFrac: number) {
    const x = gridX + ev.col * cellW + colFrac * ev.span * cellW
    const y = gridY + ev.row * cellH + 4 // a few px below top of row
    return { x, y }
  }

  return (
    <svg
      data-diagram="calendar"
      viewBox="0 0 480 300"
      xmlns="http://www.w3.org/2000/svg"
      className={className}
      aria-label="Calendar Management visualization"
      role="img"
    >
      <defs>
        {/* Lime glow filter */}
        <filter id={`ccal-glow-${uid}`} x="-50%" y="-50%" width="200%" height="200%">
          <feGaussianBlur in="SourceGraphic" stdDeviation="2.5" result="blur" />
          <feMerge>
            <feMergeNode in="blur" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>

        {/* Scan line gradient */}
        <linearGradient id={`ccal-scan-${uid}`} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%"   stopColor={C.lime} stopOpacity="0" />
          <stop offset="40%"  stopColor={C.lime} stopOpacity="0.18" />
          <stop offset="60%"  stopColor={C.lime} stopOpacity="0.18" />
          <stop offset="100%" stopColor={C.lime} stopOpacity="0" />
        </linearGradient>

        {/* Clip to calendar grid area */}
        <clipPath id={`ccal-grid-clip-${uid}`}>
          <rect x={gridX} y={gridY} width={gridWidth} height={gridHeight} />
        </clipPath>
      </defs>

      {/* ── 1. Background ── */}
      <rect width="480" height="300" fill={C.bg} rx="8" />

      {/* ── 2. Subtle dot grid overlay ── */}
      <pattern id={`ccal-dots-${uid}`} x="0" y="0" width="24" height="24" patternUnits="userSpaceOnUse">
        <circle cx="12" cy="12" r="0.8" fill={C.lime} opacity="0.07" />
      </pattern>
      <rect width="480" height="300" fill={`url(#ccal-dots-${uid})`} rx="8" />

      {/* ── 3. Calendar card ── */}
      <rect
        x={gridX - 8} y={gridY - 36}
        width={gridWidth + 16} height={gridHeight + 52}
        rx="6"
        fill={C.surface}
        stroke={C.surface2}
        strokeWidth="1"
      />

      {/* ── 4. Calendar header bar ── */}
      <rect
        x={gridX - 8} y={gridY - 36}
        width={gridWidth + 16} height="28"
        rx="6"
        fill={C.surface2}
      />
      <rect
        x={gridX - 8} y={gridY - 22}
        width={gridWidth + 16} height="14"
        fill={C.surface2}
      />

      {/* ── 5. Month + year label ── */}
      <text
        x={gridX} y={gridY - 18}
        fontFamily="'Geist Mono', monospace"
        fontSize="9"
        fontWeight="600"
        letterSpacing="0.15em"
        fill={C.white}
        textAnchor="start"
      >
        APRIL 2026
      </text>

      {/* Nav arrows */}
      <text x={gridX + gridWidth - 20} y={gridY - 18} fontSize="9" fill={C.gray400} fontFamily="monospace">‹ ›</text>

      {/* ── 6. Day-of-week header row ── */}
      {dayLabels.map((d, i) => (
        <text
          key={d}
          x={gridX + i * cellW + cellW / 2}
          y={gridY - 4}
          fontFamily="'Geist Mono', monospace"
          fontSize="7"
          fontWeight="600"
          letterSpacing="0.1em"
          fill={i >= 5 ? C.gray600 : C.gray400}
          textAnchor="middle"
        >
          {d}
        </text>
      ))}

      {/* ── 7. Grid cell backgrounds ── */}
      {days.map((day, idx) => {
        const col = idx % cols
        const row = Math.floor(idx / cols)
        const x = gridX + col * cellW
        const y = gridY + row * cellH
        const isWeekend = col >= 5
        const isToday   = day === 6 // highlight day 6 as "today"
        return (
          <g key={idx}>
            <rect
              x={x + 1} y={y + 1}
              width={cellW - 2} height={cellH - 2}
              rx="3"
              fill={isToday ? `${C.lime}18` : isWeekend ? `${C.surface2}80` : 'transparent'}
              stroke={isToday ? `${C.lime}50` : 'none'}
              strokeWidth="0.5"
            />
            {day !== null && (
              <text
                x={x + 5} y={y + 10}
                fontFamily="'Geist Mono', monospace"
                fontSize="7"
                fill={isToday ? C.lime : isWeekend ? C.gray600 : C.gray400}
                fontWeight={isToday ? '700' : '400'}
              >
                {day}
              </text>
            )}
          </g>
        )
      })}

      {/* ── 8. Grid lines ── */}
      {Array.from({ length: cols - 1 }, (_, i) => (
        <line
          key={`v${i}`}
          x1={gridX + (i + 1) * cellW} y1={gridY}
          x2={gridX + (i + 1) * cellW} y2={gridY + gridHeight}
          stroke={C.surface2}
          strokeWidth="0.5"
          opacity="0.6"
        />
      ))}
      {Array.from({ length: rows - 1 }, (_, i) => (
        <line
          key={`h${i}`}
          x1={gridX} y1={gridY + (i + 1) * cellH}
          x2={gridX + gridWidth} y2={gridY + (i + 1) * cellH}
          stroke={C.surface2}
          strokeWidth="0.5"
          opacity="0.6"
        />
      ))}

      {/* ── 9. Event bars ── */}
      {events.map((ev, idx) => {
        const x = gridX + ev.col * cellW + 2
        const y = gridY + ev.row * cellH + cellH - 10
        const w = ev.span * cellW - 4
        const isLime = ev.color === C.lime
        return (
          <g key={idx}>
            <rect
              x={x} y={y}
              width={w} height="8"
              rx="3"
              fill={ev.color}
              opacity={ev.opacity}
            />
            {isLime && (
              <rect
                x={x} y={y}
                width={w} height="8"
                rx="3"
                fill={ev.color}
                opacity="0.3"
                filter={`url(#ccal-glow-${uid})`}
              />
            )}
          </g>
        )
      })}

      {/* ── 10. Scanning highlight line (animated) ── */}
      <rect
        x={gridX} y={gridY}
        width={gridWidth} height="28"
        fill={`url(#ccal-scan-${uid})`}
        clipPath={`url(#ccal-grid-clip-${uid})`}
      >
        <animate
          attributeName="y"
          from={gridY}
          to={gridY + gridHeight - 28}
          dur="3.5s"
          repeatCount="indefinite"
          calcMode="linear"
        />
      </rect>

      {/* ── 11. Right-panel: callout labels ── */}
      {/* Panel background */}
      <rect x="292" y="56" width="172" height="156" rx="5" fill={C.surface} stroke={C.surface2} strokeWidth="1" />

      {/* Panel header */}
      <text
        x="300" y="72"
        fontFamily="'Geist Mono', monospace"
        fontSize="7"
        fontWeight="700"
        letterSpacing="0.18em"
        fill={C.lime}
        opacity="0.7"
      >
        MCP TOOLS
      </text>

      {/* ── 12. Leader lines + callout labels ── */}
      {callouts.map((c, idx) => {
        const ev  = events[c.eventIdx]
        const tip = eventTipPoint(ev!, c.tipColFrac)
        // knee point halfway between tip and label panel edge
        const kneeX = 292
        const kneeY = c.labelY - 3
        return (
          <g key={idx}>
            {/* Dashed leader line */}
            <polyline
              points={`${tip.x},${tip.y} ${kneeX},${kneeY}`}
              fill="none"
              stroke={C.lime}
              strokeWidth="0.7"
              strokeDasharray="3,3"
              opacity="0.45"
            />
            {/* Dot at tip */}
            <circle cx={tip.x} cy={tip.y} r="2" fill={C.lime} opacity="0.6">
              {idx === 0 && (
                <animate
                  attributeName="opacity"
                  values="0.6;1;0.6"
                  dur="1.8s"
                  repeatCount="indefinite"
                />
              )}
            </circle>

            {/* Label row */}
            <rect
              x="298" y={c.labelY - 9}
              width="158" height="14"
              rx="3"
              fill={idx === 0 ? `${C.lime}18` : `${C.surface2}80`}
              stroke={idx === 0 ? `${C.lime}40` : 'none'}
              strokeWidth="0.5"
            />
            <text
              x="305" y={c.labelY}
              fontFamily="'Geist Mono', monospace"
              fontSize="7.5"
              fill={idx === 0 ? C.lime : C.gray200}
              fontWeight={idx === 0 ? '700' : '400'}
            >
              {c.tool}
            </text>

            {/* Pulsing dot for active (first) callout */}
            {idx === 0 && (
              <>
                <circle cx="293" cy={c.labelY - 2} r="2.5" fill={C.lime} opacity="0.9">
                  <animate attributeName="r" values="2.5;4;2.5" dur="1.8s" repeatCount="indefinite" />
                  <animate attributeName="opacity" values="0.9;0.3;0.9" dur="1.8s" repeatCount="indefinite" />
                </circle>
                <circle cx="293" cy={c.labelY - 2} r="2" fill={C.lime} />
              </>
            )}
          </g>
        )
      })}

      {/* ── 13. "Today" marker on active event ── */}
      <text
        x={gridX + 2 * cellW + cellW / 2}
        y={gridY - 4}
        fontFamily="'Geist Mono', monospace"
        fontSize="7"
        fontWeight="700"
        fill={C.lime}
        textAnchor="middle"
        filter={`url(#ccal-glow-${uid})`}
      >
        TODAY
      </text>

      {/* ── 14. Terminal prompt area ── */}
      <rect x="16" y="230" width="448" height="52" rx="5" fill={C.surface} stroke={C.surface2} strokeWidth="1" />

      {/* Window dots */}
      <circle cx="28" cy="241" r="3" fill="#ef4444" opacity="0.8" />
      <circle cx="38" cy="241" r="3" fill="#f59e0b" opacity="0.8" />
      <circle cx="48" cy="241" r="3" fill="#22c55e" opacity="0.8" />

      {/* Terminal label */}
      <text
        x="240" y="243"
        fontFamily="'Geist Mono', monospace"
        fontSize="7"
        fill={C.gray400}
        textAnchor="middle"
        letterSpacing="0.1em"
      >
        outlook-mcp — terminal
      </text>

      {/* Prompt line */}
      <text
        x="24" y="268"
        fontFamily="'Geist Mono', monospace"
        fontSize="8.5"
        fill={C.lime}
        fontWeight="600"
      >
        $
      </text>
      <text
        x="34" y="268"
        fontFamily="'Geist Mono', monospace"
        fontSize="8.5"
        fill={C.white}
      >
        Schedule standup tomorrow at 9am
      </text>

      {/* Blinking cursor */}
      <rect x="249" y="259" width="5" height="10" rx="1" fill={C.lime} opacity="0.9">
        <animate attributeName="opacity" values="0.9;0;0.9" dur="1.1s" repeatCount="indefinite" />
      </rect>

      {/* ── 15. "calendar · create_event" echo response ── */}
      <text
        x="24" y="279"
        fontFamily="'Geist Mono', monospace"
        fontSize="7.5"
        fill={C.gray400}
      >
        → calendar · create_event · "Team Standup" · 2026-04-07 · 09:00–09:30
      </text>

      {/* ── 16. Top-right badge: tool count ── */}
      <rect x="380" y="16" width="84" height="30" rx="5" fill={`${C.lime}18`} stroke={`${C.lime}40`} strokeWidth="1" />
      <text
        x="422" y="28"
        fontFamily="'Geist Mono', monospace"
        fontSize="15"
        fontWeight="700"
        fill={C.lime}
        textAnchor="middle"
        filter={`url(#ccal-glow-${uid})`}
      >
        14
      </text>
      <text
        x="422" y="39"
        fontFamily="'Geist Mono', monospace"
        fontSize="6.5"
        fill={C.gray200}
        textAnchor="middle"
        letterSpacing="0.12em"
      >
        MCP TOOLS
      </text>

      {/* ── 17. Section eyebrow ── */}
      <text
        x="16" y="34"
        fontFamily="'Geist Mono', monospace"
        fontSize="7"
        fontWeight="600"
        letterSpacing="0.18em"
        fill={C.lime}
        opacity="0.7"
      >
        CALENDAR MANAGEMENT
      </text>
    </svg>
  )
}
