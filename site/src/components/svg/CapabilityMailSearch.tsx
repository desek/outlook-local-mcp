interface CapabilityMailSearchProps {
  className?: string
}

export default function CapabilityMailSearch({ className }: CapabilityMailSearchProps) {
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

  // Email rows data
  const emails = [
    { avatar: 'A', avatarColor: '#2a4a3a', initials: 'AL', from: 'alice@contoso.com', subject: 'Sprint Planning — Q2 Roadmap', date: '9:42 AM', read: false, match: true },
    { avatar: 'B', avatarColor: '#1a2a3a', initials: 'BK', from: 'b.kumar@contoso.com', subject: 'Re: Sprint retro notes', date: 'Yesterday', read: true, match: false },
    { avatar: 'C', avatarColor: '#2a1a3a', initials: 'CJ', from: 'c.james@vendor.io', subject: 'Sprint demo recording', date: 'Mon', read: true, match: false },
    { avatar: 'D', avatarColor: '#3a2a1a', initials: 'DM', from: 'design@studio.co', subject: 'Figma assets update', date: 'Sun', read: true, match: false },
    { avatar: 'E', avatarColor: '#1a3a2a', initials: 'EM', from: 'emily@contoso.com', subject: 'Onboarding checklist', date: 'Fri', read: true, match: false },
  ]

  const tools = ['mail_list_folders', 'mail_list_messages', 'mail_search_messages', 'mail_get_message']

  // Layout dimensions
  const W = 440
  const H = 300
  const inboxW = 240
  const previewX = inboxW + 8
  const previewW = W - previewX - 4

  // Search bar
  const searchY = 10
  const searchH = 22

  // Email rows
  const rowH = 36
  const firstRowY = searchY + searchH + 8

  // Tool badges row
  const toolY = H - 30

  return (
    <svg
      viewBox={`0 0 ${W} ${H}`}
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={className}
      aria-label="Mail search illustration"
      role="img"
    >
      <defs>
        {/* Glow for lime highlight */}
        <filter id="msLimeGlow" x="-20%" y="-40%" width="140%" height="180%">
          <feGaussianBlur stdDeviation="2" result="blur" />
          <feMerge>
            <feMergeNode in="blur" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
        {/* Pulse animation for matched row */}
        <filter id="msMatchGlow" x="-5%" y="-30%" width="110%" height="160%">
          <feGaussianBlur stdDeviation="1.5" result="blur" />
          <feMerge>
            <feMergeNode in="blur" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>

      {/* ── Overall background ── */}
      <rect width={W} height={H} rx="8" fill={C.surface} />

      {/* ══════════════════════════════
          LEFT: Inbox panel
          ══════════════════════════════ */}
      <rect x="0" y="0" width={inboxW} height={H} rx="8" fill={C.bg} />

      {/* Search bar background */}
      <rect x="8" y={searchY} width={inboxW - 16} height={searchH} rx="4" fill={C.surface} />
      {/* Search icon */}
      <circle cx="21" cy={searchY + 11} r="5" stroke={C.lime} strokeWidth="1.2" fill="none" />
      <line x1="25" y1={searchY + 15} x2="28" y2={searchY + 18} stroke={C.lime} strokeWidth="1.2" strokeLinecap="round" />

      {/* KQL query text */}
      <text
        x="33"
        y={searchY + 15}
        fill={C.lime}
        fontSize="6.5"
        fontFamily="'Geist Mono', 'Fira Code', monospace"
        dominantBaseline="middle"
      >
        subject:"Sprint" AND from:alice
      </text>

      {/* Typing cursor — blinking animation */}
      <rect
        x="186"
        y={searchY + 7}
        width="1.5"
        height="9"
        fill={C.lime}
        rx="0.5"
      >
        <animate
          attributeName="opacity"
          values="1;1;0;0;1"
          keyTimes="0;0.4;0.5;0.9;1"
          dur="1.2s"
          repeatCount="indefinite"
        />
      </rect>

      {/* ── Email rows ── */}
      {emails.map((email, i) => {
        const y = firstRowY + i * rowH
        const isMatch = email.match

        return (
          <g key={email.from}>
            {/* Row background — highlight matched */}
            <rect
              x="4"
              y={y}
              width={inboxW - 8}
              height={rowH - 3}
              rx="3"
              fill={isMatch ? 'rgba(171,255,2,0.07)' : 'transparent'}
            />

            {/* Lime border for matched row */}
            {isMatch && (
              <rect
                x="4"
                y={y}
                width={inboxW - 8}
                height={rowH - 3}
                rx="3"
                fill="none"
                stroke={C.lime}
                strokeWidth="1"
                filter="url(#msMatchGlow)"
              >
                <animate
                  attributeName="opacity"
                  values="1;0.5;1"
                  dur="2s"
                  repeatCount="indefinite"
                />
              </rect>
            )}

            {/* Unread dot */}
            {!email.read && (
              <circle cx="10" cy={y + (rowH - 3) / 2} r="2.5" fill={C.lime} />
            )}

            {/* Avatar circle */}
            <circle
              cx="24"
              cy={y + (rowH - 3) / 2}
              r="9"
              fill={email.avatarColor}
              stroke={isMatch ? C.lime : C.surface2}
              strokeWidth={isMatch ? '1' : '0.5'}
            />
            <text
              x="24"
              y={y + (rowH - 3) / 2}
              fill={isMatch ? C.lime : C.gray200}
              fontSize="5"
              fontFamily="'Inter', system-ui, sans-serif"
              fontWeight="600"
              textAnchor="middle"
              dominantBaseline="central"
            >
              {email.initials}
            </text>

            {/* From + Subject */}
            <text
              x="38"
              y={y + 9}
              fill={email.read ? C.gray400 : C.white}
              fontSize="6.5"
              fontFamily="'Inter', system-ui, sans-serif"
              fontWeight={email.read ? '400' : '600'}
              dominantBaseline="middle"
            >
              {email.from.split('@')[0]}
            </text>
            <text
              x="38"
              y={y + 22}
              fill={isMatch ? C.lime : C.gray400}
              fontSize="6"
              fontFamily="'Inter', system-ui, sans-serif"
              dominantBaseline="middle"
            >
              {email.subject.length > 22 ? email.subject.slice(0, 22) + '…' : email.subject}
            </text>

            {/* Date */}
            <text
              x={inboxW - 10}
              y={y + 9}
              fill={C.gray600}
              fontSize="5.5"
              fontFamily="'Inter', system-ui, sans-serif"
              textAnchor="end"
              dominantBaseline="middle"
            >
              {email.date}
            </text>
          </g>
        )
      })}

      {/* Divider between inbox and preview */}
      <line x1={inboxW + 2} y1="0" x2={inboxW + 2} y2={toolY - 4} stroke={C.surface2} strokeWidth="1" />

      {/* ══════════════════════════════
          RIGHT: Message preview panel
          ══════════════════════════════ */}

      {/* Preview header bar */}
      <rect x={previewX} y="0" width={previewW} height="28" rx="0" fill={C.surface2} />
      <rect x={previewX} y="0" width={previewW} height="28" rx="0" fill="none" />

      {/* Subject line in preview header */}
      <text
        x={previewX + 8}
        y="10"
        fill={C.white}
        fontSize="7"
        fontFamily="'Inter', system-ui, sans-serif"
        fontWeight="600"
        dominantBaseline="hanging"
      >
        Sprint Planning — Q2 Roadmap
      </text>

      {/* From line */}
      <text
        x={previewX + 8}
        y="21"
        fill={C.lime}
        fontSize="6"
        fontFamily="'Geist Mono', 'Fira Code', monospace"
        dominantBaseline="hanging"
      >
        alice@contoso.com
      </text>

      {/* Preview body lines — simulated text */}
      {[
        { y: 36, w: 155, text: 'Hi team, sharing the updated Sprint Planning' },
        { y: 48, w: 130, text: 'doc for Q2. Please review sections 2–4' },
        { y: 60, w: 145, text: 'before Thursday. Key items: capacity plan,' },
        { y: 72, w: 110, text: 'backlog grooming, and milestone dates.' },
      ].map((line) => (
        <text
          key={line.y}
          x={previewX + 8}
          y={line.y}
          fill={C.gray200}
          fontSize="6"
          fontFamily="'Inter', system-ui, sans-serif"
          dominantBaseline="hanging"
        >
          {line.text}
        </text>
      ))}

      {/* Highlight chip: KQL match indicator */}
      <rect x={previewX + 8} y="92" width="80" height="12" rx="3" fill="rgba(171,255,2,0.15)" stroke={C.lime} strokeWidth="0.6" />
      <text
        x={previewX + 48}
        y="98"
        fill={C.lime}
        fontSize="5.5"
        fontFamily="'Geist Mono', 'Fira Code', monospace"
        textAnchor="middle"
        dominantBaseline="central"
      >
        KQL match · 1 result
      </text>

      {/* Attachment indicator */}
      <rect x={previewX + 8} y="112" width="60" height="12" rx="3" fill={C.surface2} />
      <text
        x={previewX + 38}
        y="118"
        fill={C.gray400}
        fontSize="5.5"
        fontFamily="'Inter', system-ui, sans-serif"
        textAnchor="middle"
        dominantBaseline="central"
      >
        📎 roadmap-q2.pdf
      </text>

      {/* ── Decorative dot grid on preview bg ── */}
      {[0, 1, 2, 3].map((col) =>
        [0, 1, 2, 3, 4].map((row) => (
          <circle
            key={`dot-${col}-${row}`}
            cx={previewX + 100 + col * 20}
            cy={140 + row * 20}
            r="1"
            fill="rgba(171,255,2,0.06)"
          />
        ))
      )}

      {/* ══════════════════════════════
          BOTTOM: Tool badges
          ══════════════════════════════ */}
      <rect x="0" y={toolY - 6} width={W} height="1" fill={C.surface2} />
      <rect x="0" y={toolY - 6} width={W} height={H - toolY + 6} rx="0" fill={C.surface} />
      <rect x="0" y={H - 8} width={W} height="8" rx="0" fill={C.surface} />

      {/* Rounded bottom corners overlay */}
      <rect x="0" y={toolY - 6} width={W} height={H - toolY + 6} rx="0" fill={C.surface} />

      {tools.map((tool, i) => {
        const badgeW = 96
        const gap = 8
        const totalW = tools.length * badgeW + (tools.length - 1) * gap
        const startX = (W - totalW) / 2
        const x = startX + i * (badgeW + gap)
        const isActive = tool === 'mail_search_messages'

        return (
          <g key={tool}>
            <rect
              x={x}
              y={toolY + 2}
              width={badgeW}
              height="16"
              rx="3"
              fill={isActive ? 'rgba(171,255,2,0.15)' : C.surface2}
              stroke={isActive ? C.lime : C.gray600}
              strokeWidth={isActive ? '0.8' : '0.5'}
            />
            <text
              x={x + badgeW / 2}
              y={toolY + 10}
              fill={isActive ? C.lime : C.gray400}
              fontSize="5"
              fontFamily="'Geist Mono', 'Fira Code', monospace"
              textAnchor="middle"
              dominantBaseline="central"
            >
              {tool}
            </text>
          </g>
        )
      })}
    </svg>
  )
}
