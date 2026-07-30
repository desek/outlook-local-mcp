interface Props {
  className?: string
}

export default function CapabilityMultiAccount({ className }: Props) {
  return (
    <svg
      data-diagram="multi-account"
      viewBox="0 0 480 340"
      xmlns="http://www.w3.org/2000/svg"
      aria-label="Multi-account management diagram"
      className={className}
      style={{ width: '100%', height: '100%' }}
    >
      {/* ── Definitions ── */}
      <defs>
        {/* Card shadow filter */}
        <filter id="card-shadow" x="-10%" y="-10%" width="120%" height="130%">
          <feDropShadow dx="0" dy="4" stdDeviation="6" floodColor="#000000" floodOpacity="0.5" />
        </filter>
        {/* Hub glow filter */}
        <filter id="hub-glow" x="-20%" y="-20%" width="140%" height="140%">
          <feDropShadow dx="0" dy="0" stdDeviation="8" floodColor="#abff02" floodOpacity="0.35" />
        </filter>
        {/* Lime dot glow */}
        <filter id="dot-glow" x="-100%" y="-100%" width="300%" height="300%">
          <feDropShadow dx="0" dy="0" stdDeviation="3" floodColor="#abff02" floodOpacity="0.8" />
        </filter>
        {/* Dashed line pattern */}
        <marker id="arrow" markerWidth="6" markerHeight="6" refX="3" refY="3" orient="auto">
          <circle cx="3" cy="3" r="1.5" fill="#abff02" fillOpacity="0.6" />
        </marker>
      </defs>

      {/* ══════════════════════════════════════
          Account Card 1 — top-left (work)
          rotate -8deg around its center
          ══════════════════════════════════════ */}
      <g transform="rotate(-8, 85, 90)" filter="url(#card-shadow)">
        {/* Card body */}
        <rect x="18" y="42" width="134" height="82" rx="8" fill="#052424" />
        {/* Card top accent bar */}
        <rect x="18" y="42" width="134" height="3" rx="8" fill="#abff02" fillOpacity="0.5" />
        {/* Avatar — hexagon approximated as polygon */}
        <polygon
          points="40,67 50,61 60,67 60,79 50,85 40,79"
          fill="#1a2a2a"
          stroke="#abff02"
          strokeWidth="1.5"
        />
        <text x="50" y="76" textAnchor="middle" fontFamily="monospace" fontSize="9" fill="#abff02" fontWeight="600">W</text>
        {/* Email label */}
        <text x="72" y="73" fontFamily="monospace" fontSize="8.5" fill="#ffffff" fontWeight="500">work@company</text>
        <text x="72" y="84" fontFamily="monospace" fontSize="7.5" fill="#7f7f7f">.com</text>
        {/* Status row */}
        <circle cx="30" cy="108" r="4" fill="#abff02" filter="url(#dot-glow)">
          <animate attributeName="r" values="4;5.5;4" dur="2.4s" repeatCount="indefinite" />
          <animate attributeName="fillOpacity" values="1;0.7;1" dur="2.4s" repeatCount="indefinite" />
        </circle>
        <text x="40" y="112" fontFamily="monospace" fontSize="7.5" fill="#abff02" letterSpacing="1">AUTHENTICATED</text>
        {/* Token isolation badge */}
        <rect x="100" y="101" width="44" height="14" rx="4" fill="#1a2a2a" stroke="#abff02" strokeWidth="0.75" strokeOpacity="0.4" />
        <text x="122" y="111" textAnchor="middle" fontFamily="monospace" fontSize="6.5" fill="#7f7f7f">token:A</text>
      </g>

      {/* ══════════════════════════════════════
          Account Card 2 — center (personal)
          no rotation — front card
          ══════════════════════════════════════ */}
      <g filter="url(#card-shadow)">
        {/* Card body */}
        <rect x="56" y="130" width="134" height="82" rx="8" fill="#052424" />
        {/* Card top accent bar */}
        <rect x="56" y="130" width="134" height="3" rx="8" fill="#abff02" fillOpacity="0.7" />
        {/* Avatar — circle shape */}
        <circle cx="96" cy="158" r="13" fill="#1a2a2a" stroke="#abff02" strokeWidth="1.5" />
        <text x="96" y="163" textAnchor="middle" fontFamily="monospace" fontSize="9" fill="#abff02" fontWeight="600">P</text>
        {/* Email label */}
        <text x="116" y="155" fontFamily="monospace" fontSize="8.5" fill="#ffffff" fontWeight="500">personal@</text>
        <text x="116" y="166" fontFamily="monospace" fontSize="8.5" fill="#ffffff">outlook.com</text>
        {/* Status row */}
        <circle cx="68" cy="196" r="4" fill="#abff02" filter="url(#dot-glow)">
          <animate attributeName="r" values="4;5.5;4" dur="2.8s" begin="0.4s" repeatCount="indefinite" />
          <animate attributeName="fillOpacity" values="1;0.7;1" dur="2.8s" begin="0.4s" repeatCount="indefinite" />
        </circle>
        <text x="78" y="200" fontFamily="monospace" fontSize="7.5" fill="#abff02" letterSpacing="1">AUTHENTICATED</text>
        {/* Token isolation badge */}
        <rect x="140" y="189" width="44" height="14" rx="4" fill="#1a2a2a" stroke="#abff02" strokeWidth="0.75" strokeOpacity="0.4" />
        <text x="162" y="199" textAnchor="middle" fontFamily="monospace" fontSize="6.5" fill="#7f7f7f">token:B</text>
      </g>

      {/* ══════════════════════════════════════
          Account Card 3 — bottom-right (team)
          rotate +6deg
          ══════════════════════════════════════ */}
      <g transform="rotate(6, 115, 265)" filter="url(#card-shadow)">
        {/* Card body */}
        <rect x="48" y="228" width="134" height="82" rx="8" fill="#052424" />
        {/* Card top accent bar */}
        <rect x="48" y="228" width="134" height="3" rx="8" fill="#abff02" fillOpacity="0.5" />
        {/* Avatar — diamond shape */}
        <polygon
          points="80,247 92,238 104,247 92,256"
          fill="#1a2a2a"
          stroke="#abff02"
          strokeWidth="1.5"
        />
        <text x="92" y="251" textAnchor="middle" fontFamily="monospace" fontSize="9" fill="#abff02" fontWeight="600">T</text>
        {/* Email label */}
        <text x="114" y="249" fontFamily="monospace" fontSize="8.5" fill="#ffffff" fontWeight="500">team@org</text>
        <text x="114" y="260" fontFamily="monospace" fontSize="7.5" fill="#7f7f7f">.com</text>
        {/* Status row */}
        <circle cx="60" cy="294" r="4" fill="#abff02" filter="url(#dot-glow)">
          <animate attributeName="r" values="4;5.5;4" dur="3.1s" begin="0.8s" repeatCount="indefinite" />
          <animate attributeName="fillOpacity" values="1;0.7;1" dur="3.1s" begin="0.8s" repeatCount="indefinite" />
        </circle>
        <text x="70" y="298" fontFamily="monospace" fontSize="7.5" fill="#abff02" letterSpacing="1">AUTHENTICATED</text>
        {/* Token isolation badge */}
        <rect x="130" y="287" width="44" height="14" rx="4" fill="#1a2a2a" stroke="#abff02" strokeWidth="0.75" strokeOpacity="0.4" />
        <text x="152" y="297" textAnchor="middle" fontFamily="monospace" fontSize="6.5" fill="#7f7f7f">token:C</text>
      </g>

      {/* ══════════════════════════════════════
          Connecting dashed lines
          from each card edge to the hub
          ══════════════════════════════════════ */}
      {/* Line 1 — work card to hub */}
      <line
        x1="152" y1="83"
        x2="295" y2="165"
        stroke="#abff02"
        strokeWidth="1"
        strokeOpacity="0.35"
        strokeDasharray="5 4"
        markerEnd="url(#arrow)"
      />
      {/* Line 2 — personal card to hub */}
      <line
        x1="190" y1="171"
        x2="292" y2="172"
        stroke="#abff02"
        strokeWidth="1"
        strokeOpacity="0.45"
        strokeDasharray="5 4"
        markerEnd="url(#arrow)"
      />
      {/* Line 3 — team card to hub */}
      <line
        x1="175" y1="258"
        x2="295" y2="182"
        stroke="#abff02"
        strokeWidth="1"
        strokeOpacity="0.35"
        strokeDasharray="5 4"
        markerEnd="url(#arrow)"
      />

      {/* ── Lock icon on line 1 ── */}
      <g transform="translate(218, 118)">
        <rect x="-7" y="-7" width="14" height="14" rx="3" fill="#0a1010" stroke="#7f7f7f" strokeWidth="0.75" />
        {/* Lock body */}
        <rect x="-3.5" y="0" width="7" height="5" rx="1" fill="#7f7f7f" />
        {/* Lock shackle */}
        <path d="M -2.5 0 L -2.5 -3 Q 0 -5 2.5 -3 L 2.5 0" fill="none" stroke="#7f7f7f" strokeWidth="1.2" />
        <circle cx="0" cy="2.5" r="1" fill="#0a1010" />
      </g>

      {/* ── Lock icon on line 2 ── */}
      <g transform="translate(240, 171)">
        <rect x="-7" y="-7" width="14" height="14" rx="3" fill="#0a1010" stroke="#7f7f7f" strokeWidth="0.75" />
        <rect x="-3.5" y="0" width="7" height="5" rx="1" fill="#7f7f7f" />
        <path d="M -2.5 0 L -2.5 -3 Q 0 -5 2.5 -3 L 2.5 0" fill="none" stroke="#7f7f7f" strokeWidth="1.2" />
        <circle cx="0" cy="2.5" r="1" fill="#0a1010" />
      </g>

      {/* ── Lock icon on line 3 ── */}
      <g transform="translate(228, 225)">
        <rect x="-7" y="-7" width="14" height="14" rx="3" fill="#0a1010" stroke="#7f7f7f" strokeWidth="0.75" />
        <rect x="-3.5" y="0" width="7" height="5" rx="1" fill="#7f7f7f" />
        <path d="M -2.5 0 L -2.5 -3 Q 0 -5 2.5 -3 L 2.5 0" fill="none" stroke="#7f7f7f" strokeWidth="1.2" />
        <circle cx="0" cy="2.5" r="1" fill="#0a1010" />
      </g>

      {/* ══════════════════════════════════════
          MCP Server Hub node
          ══════════════════════════════════════ */}
      <g filter="url(#hub-glow)">
        <rect x="294" y="138" width="120" height="66" rx="10" fill="#052424" stroke="#abff02" strokeWidth="1.5" />
        {/* Hub top label strip */}
        <rect x="294" y="138" width="120" height="20" rx="10" fill="#abff02" fillOpacity="0.12" />
        <rect x="294" y="148" width="120" height="10" fill="#abff02" fillOpacity="0.12" />
        {/* Hub label */}
        <text x="354" y="152" textAnchor="middle" fontFamily="monospace" fontSize="8" fill="#abff02" letterSpacing="1.5" fontWeight="700">MCP SERVER</text>
        {/* Terminal prompt icon — spinning */}
        <g transform="translate(354, 177)">
          <animateTransform
            attributeName="transform"
            type="rotate"
            from="0 354 177"
            to="360 354 177"
            dur="12s"
            repeatCount="indefinite"
            additive="sum"
          />
          <circle cx="0" cy="0" r="12" fill="#1a2a2a" stroke="#abff02" strokeWidth="1" strokeOpacity="0.5" />
          {/* Terminal > prompt */}
          <text x="0" y="4" textAnchor="middle" fontFamily="monospace" fontSize="10" fill="#abff02" fontWeight="700">{'>'}</text>
        </g>
        {/* Hub sub-label */}
        <text x="354" y="196" textAnchor="middle" fontFamily="monospace" fontSize="7" fill="#7f7f7f">account_list · account_add</text>
      </g>

      {/* ══════════════════════════════════════
          Keychain icon — OS-native secure storage
          positioned below the hub
          ══════════════════════════════════════ */}
      <g transform="translate(354, 230)">
        {/* Keychain ring */}
        <circle cx="0" cy="0" r="16" fill="none" stroke="#7f7f7f" strokeWidth="1.5" strokeOpacity="0.6" strokeDasharray="3 2" />
        {/* Key icon body */}
        <circle cx="-3" cy="0" r="6" fill="#1a2a2a" stroke="#7f7f7f" strokeWidth="1.2" />
        <circle cx="-3" cy="0" r="3" fill="#0a1010" />
        <rect x="2" y="-1.5" width="12" height="3" rx="1" fill="#1a2a2a" stroke="#7f7f7f" strokeWidth="1" />
        <rect x="10" y="1.5" width="3" height="3" rx="0.5" fill="#7f7f7f" />
        <rect x="13" y="1.5" width="2" height="3" rx="0.5" fill="#7f7f7f" />
        {/* Label */}
        <text x="0" y="28" textAnchor="middle" fontFamily="monospace" fontSize="7" fill="#7f7f7f" letterSpacing="0.5">OS KEYCHAIN</text>
        {/* Connector line to hub */}
        <line x1="0" y1="-16" x2="0" y2="-72" stroke="#7f7f7f" strokeWidth="0.75" strokeOpacity="0.4" strokeDasharray="3 3" />
      </g>
    </svg>
  )
}
