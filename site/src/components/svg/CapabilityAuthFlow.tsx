interface Props {
  className?: string
}

export default function CapabilityAuthFlow({ className }: Props) {
  return (
    <svg
      viewBox="0 0 480 360"
      xmlns="http://www.w3.org/2000/svg"
      aria-label="Zero-Config Auth flow diagram showing three authentication methods"
      className={className}
      style={{ width: '100%', height: '100%' }}
    >
      <defs>
        {/* Card shadow */}
        <filter id="af-card-shadow" x="-10%" y="-10%" width="120%" height="130%">
          <feDropShadow dx="0" dy="3" stdDeviation="5" floodColor="#000000" floodOpacity="0.55" />
        </filter>
        {/* Lime glow for trigger / result boxes */}
        <filter id="af-lime-glow" x="-20%" y="-20%" width="140%" height="140%">
          <feDropShadow dx="0" dy="0" stdDeviation="7" floodColor="#abff02" floodOpacity="0.3" />
        </filter>
        {/* Dot glow */}
        <filter id="af-dot-glow" x="-100%" y="-100%" width="300%" height="300%">
          <feDropShadow dx="0" dy="0" stdDeviation="3" floodColor="#abff02" floodOpacity="0.9" />
        </filter>
        {/* Arrowhead marker — lime */}
        <marker id="af-arrow-lime" markerWidth="7" markerHeight="7" refX="5" refY="3.5" orient="auto">
          <polygon points="0 0, 7 3.5, 0 7" fill="#abff02" fillOpacity="0.7" />
        </marker>
        {/* Arrowhead marker — gray */}
        <marker id="af-arrow-gray" markerWidth="7" markerHeight="7" refX="5" refY="3.5" orient="auto">
          <polygon points="0 0, 7 3.5, 0 7" fill="#454742" />
        </marker>
        {/* Flow dot animation path — trigger to device-code */}
        <path id="af-path-1" d="M 240 70 L 108 118" fill="none" />
        {/* Flow dot animation path — trigger to browser */}
        <path id="af-path-2" d="M 240 70 L 240 118" fill="none" />
        {/* Flow dot animation path — trigger to auth-code */}
        <path id="af-path-3" d="M 240 70 L 372 118" fill="none" />
        {/* Flow dot animation path — device-code to token */}
        <path id="af-path-4" d="M 108 202 L 240 250" fill="none" />
        {/* Flow dot animation path — browser to token */}
        <path id="af-path-5" d="M 240 202 L 240 250" fill="none" />
        {/* Flow dot animation path — auth-code to token */}
        <path id="af-path-6" d="M 372 202 L 240 250" fill="none" />
      </defs>

      {/* ══════════════════════════════════════
          TRIGGER BOX — "First Tool Call"
          ══════════════════════════════════════ */}
      <g filter="url(#af-lime-glow)">
        <rect x="152" y="18" width="176" height="54" rx="8" fill="#052424" stroke="#abff02" strokeWidth="1.5" />
        {/* Top accent strip */}
        <rect x="152" y="18" width="176" height="3" rx="8" fill="#abff02" />
      </g>
      {/* Terminal prompt icon */}
      <g transform="translate(175, 45)">
        <rect x="-10" y="-10" width="20" height="20" rx="4" fill="#1a2a2a" />
        <text x="0" y="4" textAnchor="middle" fontFamily="monospace" fontSize="11" fill="#abff02" fontWeight="700">{'>'}</text>
      </g>
      {/* Label */}
      <text x="195" y="38" fontFamily="monospace" fontSize="7.5" fill="#abff02" letterSpacing="1.2" fontWeight="700">FIRST TOOL CALL</text>
      <text x="195" y="54" fontFamily="monospace" fontSize="8" fill="#ffffff" fontWeight="400">"List my calendars"</text>

      {/* ══════════════════════════════════════
          FLOW ARROWS — trigger to auth methods
          ══════════════════════════════════════ */}
      {/* Arrow to Device Code */}
      <line x1="200" y1="72" x2="108" y2="118"
        stroke="#abff02" strokeWidth="1" strokeOpacity="0.45" strokeDasharray="5 4"
        markerEnd="url(#af-arrow-lime)" />
      {/* Arrow to Browser */}
      <line x1="240" y1="72" x2="240" y2="118"
        stroke="#abff02" strokeWidth="1" strokeOpacity="0.55" strokeDasharray="5 4"
        markerEnd="url(#af-arrow-lime)" />
      {/* Arrow to Auth Code */}
      <line x1="280" y1="72" x2="372" y2="118"
        stroke="#abff02" strokeWidth="1" strokeOpacity="0.45" strokeDasharray="5 4"
        markerEnd="url(#af-arrow-lime)" />

      {/* Animated flow dots — trigger to methods */}
      <circle r="3.5" fill="#abff02" filter="url(#af-dot-glow)">
        <animateMotion dur="1.6s" begin="0s" repeatCount="indefinite" path="M 200 72 L 108 118" />
        <animate attributeName="opacity" values="0;1;1;0" dur="1.6s" repeatCount="indefinite" />
      </circle>
      <circle r="3.5" fill="#abff02" filter="url(#af-dot-glow)">
        <animateMotion dur="1.4s" begin="0.3s" repeatCount="indefinite" path="M 240 72 L 240 118" />
        <animate attributeName="opacity" values="0;1;1;0" dur="1.4s" begin="0.3s" repeatCount="indefinite" />
      </circle>
      <circle r="3.5" fill="#abff02" filter="url(#af-dot-glow)">
        <animateMotion dur="1.6s" begin="0.6s" repeatCount="indefinite" path="M 280 72 L 372 118" />
        <animate attributeName="opacity" values="0;1;1;0" dur="1.6s" begin="0.6s" repeatCount="indefinite" />
      </circle>

      {/* ══════════════════════════════════════
          AUTH METHOD CARD 1 — Device Code (default)
          x=40, y=118, w=136, h=84
          ══════════════════════════════════════ */}
      <g filter="url(#af-card-shadow)">
        <rect x="40" y="118" width="136" height="84" rx="8" fill="#052424" />
        {/* Top accent */}
        <rect x="40" y="118" width="136" height="3" rx="8" fill="#1a2a2a" />
      </g>
      {/* Monitor icon */}
      <g transform="translate(62, 142)">
        <rect x="-12" y="-10" width="24" height="16" rx="2" fill="none" stroke="#7f7f7f" strokeWidth="1.2" />
        <line x1="0" y1="6" x2="0" y2="10" stroke="#7f7f7f" strokeWidth="1.2" />
        <line x1="-5" y1="10" x2="5" y2="10" stroke="#7f7f7f" strokeWidth="1.2" />
        {/* Screen glow */}
        <rect x="-9" y="-7" width="18" height="10" rx="1" fill="#1a2a2a" />
        <text x="0" y="0" textAnchor="middle" fontFamily="monospace" fontSize="5.5" fill="#abff02">ABCD</text>
      </g>
      {/* Card title */}
      <text x="84" y="135" fontFamily="monospace" fontSize="8" fill="#ffffff" fontWeight="600">Device Code</text>
      {/* Code value */}
      <rect x="52" y="156" width="112" height="16" rx="3" fill="#0a1010" />
      <text x="108" y="167" textAnchor="middle" fontFamily="monospace" fontSize="8.5" fill="#abff02" letterSpacing="1.5" fontWeight="700">ABCD-1234</text>
      {/* URL */}
      <text x="108" y="183" textAnchor="middle" fontFamily="monospace" fontSize="6.5" fill="#7f7f7f">microsoft.com/devicelogin</text>
      {/* Headless label */}
      <text x="108" y="196" textAnchor="middle" fontFamily="monospace" fontSize="6" fill="#454742" letterSpacing="0.5">headless environments</text>

      {/* DEFAULT badge with pulse */}
      <g transform="translate(108, 118)">
        {/* Outer pulse ring */}
        <circle cx="0" cy="0" r="0" fill="none" stroke="#abff02" strokeWidth="1">
          <animate attributeName="r" values="8;14;8" dur="2s" repeatCount="indefinite" />
          <animate attributeName="stroke-opacity" values="0.8;0;0.8" dur="2s" repeatCount="indefinite" />
        </circle>
        <rect x="-18" y="-8" width="36" height="16" rx="8" fill="#abff02" />
        <text x="0" y="4" textAnchor="middle" fontFamily="monospace" fontSize="6.5" fill="#052424" fontWeight="700" letterSpacing="0.5">DEFAULT</text>
      </g>

      {/* ══════════════════════════════════════
          AUTH METHOD CARD 2 — Browser
          x=172, y=118, w=136, h=84
          ══════════════════════════════════════ */}
      <g filter="url(#af-card-shadow)">
        <rect x="172" y="118" width="136" height="84" rx="8" fill="#052424" />
        <rect x="172" y="118" width="136" height="3" rx="8" fill="#1a2a2a" />
      </g>
      {/* Browser window icon */}
      <g transform="translate(194, 142)">
        <rect x="-12" y="-10" width="24" height="18" rx="2" fill="none" stroke="#7f7f7f" strokeWidth="1.2" />
        {/* Browser toolbar */}
        <line x1="-12" y1="-4" x2="12" y2="-4" stroke="#7f7f7f" strokeWidth="0.8" strokeOpacity="0.5" />
        <circle cx="-8" cy="-7" r="1.5" fill="#454742" />
        <circle cx="-4" cy="-7" r="1.5" fill="#454742" />
        <circle cx="0" cy="-7" r="1.5" fill="#7f7f7f" />
        {/* MS logo bars */}
        <rect x="-5" y="-1" width="4.5" height="4.5" rx="0.5" fill="#abff02" fillOpacity="0.8" />
        <rect x="0.5" y="-1" width="4.5" height="4.5" rx="0.5" fill="#abff02" fillOpacity="0.5" />
        <rect x="-5" y="4.5" width="4.5" height="4.5" rx="0.5" fill="#abff02" fillOpacity="0.5" />
        <rect x="0.5" y="4.5" width="4.5" height="4.5" rx="0.5" fill="#abff02" fillOpacity="0.3" />
      </g>
      <text x="216" y="135" fontFamily="monospace" fontSize="8" fill="#ffffff" fontWeight="600">Browser</text>
      {/* Microsoft login label */}
      <text x="240" y="165" textAnchor="middle" fontFamily="monospace" fontSize="7" fill="#7f7f7f">Microsoft login</text>
      {/* Localhost callback badge */}
      <rect x="184" y="172" width="112" height="16" rx="3" fill="#0a1010" />
      <text x="240" y="183" textAnchor="middle" fontFamily="monospace" fontSize="6.5" fill="#abff02" letterSpacing="0.5">localhost callback</text>
      {/* System browser label */}
      <text x="240" y="196" textAnchor="middle" fontFamily="monospace" fontSize="6" fill="#454742" letterSpacing="0.5">opens system browser</text>

      {/* ══════════════════════════════════════
          AUTH METHOD CARD 3 — Auth Code PKCE
          x=304, y=118, w=136, h=84
          ══════════════════════════════════════ */}
      <g filter="url(#af-card-shadow)">
        <rect x="304" y="118" width="136" height="84" rx="8" fill="#052424" />
        <rect x="304" y="118" width="136" height="3" rx="8" fill="#1a2a2a" />
      </g>
      {/* Terminal icon */}
      <g transform="translate(326, 142)">
        <rect x="-12" y="-10" width="24" height="18" rx="2" fill="#0a1010" stroke="#7f7f7f" strokeWidth="1.2" />
        <text x="-7" y="-2" fontFamily="monospace" fontSize="6" fill="#abff02">$_</text>
        {/* PKCE flow arrows inside terminal */}
        <line x1="-7" y1="2" x2="2" y2="2" stroke="#abff02" strokeWidth="0.8" strokeOpacity="0.7" markerEnd="url(#af-arrow-lime)" />
        <line x1="7" y1="5" x2="-2" y2="5" stroke="#7f7f7f" strokeWidth="0.8" strokeOpacity="0.7" markerEnd="url(#af-arrow-gray)" />
      </g>
      <text x="348" y="135" fontFamily="monospace" fontSize="8" fill="#ffffff" fontWeight="600">Auth Code</text>
      {/* PKCE label */}
      <rect x="316" y="155" width="112" height="14" rx="3" fill="#0a1010" />
      <text x="372" y="165" textAnchor="middle" fontFamily="monospace" fontSize="7" fill="#abff02" letterSpacing="0.8">PKCE flow</text>
      {/* Headless/remote label */}
      <rect x="316" y="174" width="112" height="14" rx="3" fill="#1a2a2a" />
      <text x="372" y="184" textAnchor="middle" fontFamily="monospace" fontSize="6.5" fill="#7f7f7f">headless / remote</text>
      <text x="372" y="196" textAnchor="middle" fontFamily="monospace" fontSize="6" fill="#454742" letterSpacing="0.5">no browser needed</text>

      {/* ══════════════════════════════════════
          FLOW ARROWS — auth methods to token
          ══════════════════════════════════════ */}
      {/* Arrow from Device Code */}
      <line x1="108" y1="202" x2="205" y2="250"
        stroke="#abff02" strokeWidth="1" strokeOpacity="0.45" strokeDasharray="5 4"
        markerEnd="url(#af-arrow-lime)" />
      {/* Arrow from Browser */}
      <line x1="240" y1="202" x2="240" y2="250"
        stroke="#abff02" strokeWidth="1" strokeOpacity="0.55" strokeDasharray="5 4"
        markerEnd="url(#af-arrow-lime)" />
      {/* Arrow from Auth Code */}
      <line x1="372" y1="202" x2="275" y2="250"
        stroke="#abff02" strokeWidth="1" strokeOpacity="0.45" strokeDasharray="5 4"
        markerEnd="url(#af-arrow-lime)" />

      {/* Animated flow dots — methods to token */}
      <circle r="3" fill="#abff02" filter="url(#af-dot-glow)">
        <animateMotion dur="1.5s" begin="1.8s" repeatCount="indefinite" path="M 108 202 L 205 250" />
        <animate attributeName="opacity" values="0;1;1;0" dur="1.5s" begin="1.8s" repeatCount="indefinite" />
      </circle>
      <circle r="3" fill="#abff02" filter="url(#af-dot-glow)">
        <animateMotion dur="1.2s" begin="2.0s" repeatCount="indefinite" path="M 240 202 L 240 250" />
        <animate attributeName="opacity" values="0;1;1;0" dur="1.2s" begin="2.0s" repeatCount="indefinite" />
      </circle>
      <circle r="3" fill="#abff02" filter="url(#af-dot-glow)">
        <animateMotion dur="1.5s" begin="2.2s" repeatCount="indefinite" path="M 372 202 L 275 250" />
        <animate attributeName="opacity" values="0;1;1;0" dur="1.5s" begin="2.2s" repeatCount="indefinite" />
      </circle>

      {/* ══════════════════════════════════════
          TOKEN CACHED result box
          ══════════════════════════════════════ */}
      <g filter="url(#af-lime-glow)">
        <rect x="136" y="250" width="208" height="60" rx="8" fill="#052424" stroke="#abff02" strokeWidth="1.5" strokeOpacity="0.7" />
        {/* Bottom accent strip */}
        <rect x="136" y="302" width="208" height="8" rx="8" fill="#abff02" fillOpacity="0.15" />
      </g>
      {/* Keychain / OS icon */}
      <g transform="translate(165, 280)">
        {/* Key ring */}
        <circle cx="0" cy="0" r="9" fill="none" stroke="#7f7f7f" strokeWidth="1.2" strokeOpacity="0.6" />
        <circle cx="-2" cy="0" r="4" fill="#1a2a2a" stroke="#7f7f7f" strokeWidth="1" />
        <circle cx="-2" cy="0" r="2" fill="#0a1010" />
        <rect x="2" y="-1" width="9" height="2.5" rx="0.8" fill="#1a2a2a" stroke="#7f7f7f" strokeWidth="0.8" />
        <rect x="8" y="1.5" width="2.5" height="2.5" rx="0.4" fill="#7f7f7f" />
      </g>
      {/* Token Cached label */}
      <text x="185" y="264" fontFamily="monospace" fontSize="7.5" fill="#abff02" letterSpacing="1.2" fontWeight="700">TOKEN CACHED</text>
      {/* Detail text */}
      <text x="185" y="278" fontFamily="monospace" fontSize="7.5" fill="#ffffff">OS keychain · silent refresh</text>
      <text x="185" y="291" fontFamily="monospace" fontSize="7" fill="#7f7f7f">90 day expiry · auto-renew</text>

      {/* ══════════════════════════════════════
          "No Entra ID Required" callout badge
          ══════════════════════════════════════ */}
      <g transform="translate(240, 334)">
        <rect x="-100" y="-12" width="200" height="24" rx="12" fill="#1a2a2a" stroke="#abff02" strokeWidth="1" strokeOpacity="0.5" />
        {/* Checkmark */}
        <g transform="translate(-83, 0)">
          <circle cx="0" cy="0" r="7" fill="#abff02" fillOpacity="0.15" stroke="#abff02" strokeWidth="1" />
          <polyline points="-3,0 -1,3 4,-3" fill="none" stroke="#abff02" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        </g>
        <text x="5" y="4" textAnchor="middle" fontFamily="monospace" fontSize="7.5" fill="#abff02" letterSpacing="0.8" fontWeight="600">No Entra ID Required</text>
      </g>

      {/* ══════════════════════════════════════
          "3 Methods" eyebrow label — top-right
          ══════════════════════════════════════ */}
      <text x="456" y="30" textAnchor="end" fontFamily="monospace" fontSize="7" fill="#454742" letterSpacing="1">3 METHODS</text>
      <text x="456" y="42" textAnchor="end" fontFamily="monospace" fontSize="7" fill="#454742" letterSpacing="1">ZERO CONFIG</text>

      {/* ══════════════════════════════════════
          Subtle dot-grid background texture
          ══════════════════════════════════════ */}
      {/* Row 1 */}
      <circle cx="24" cy="24" r="1" fill="#abff02" fillOpacity="0.06" />
      <circle cx="56" cy="24" r="1" fill="#abff02" fillOpacity="0.06" />
      <circle cx="424" cy="24" r="1" fill="#abff02" fillOpacity="0.06" />
      <circle cx="456" cy="24" r="1" fill="#abff02" fillOpacity="0.06" />
      <circle cx="24" cy="336" r="1" fill="#abff02" fillOpacity="0.06" />
      <circle cx="456" cy="336" r="1" fill="#abff02" fillOpacity="0.06" />
    </svg>
  )
}
