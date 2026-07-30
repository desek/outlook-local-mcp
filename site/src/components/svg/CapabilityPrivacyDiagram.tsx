interface Props {
  className?: string
}

export default function CapabilityPrivacyDiagram({ className }: Props) {
  // Design tokens
  // --color-brand-lime:  #abff02
  // --color-brand-dark:  #052424
  // --color-gray-900:    #0a1010
  // --color-gray-800:    #1a2a2a
  // --color-gray-600:    #454742
  // --color-gray-200:    #c2c2c2

  return (
    <svg
      data-diagram="privacy"
      viewBox="0 0 640 400"
      xmlns="http://www.w3.org/2000/svg"
      className={className}
      aria-label="Privacy and security architecture diagram"
      role="img"
    >
      <defs>
        {/* Lime glow filter */}
        <filter id="lime-glow" x="-20%" y="-20%" width="140%" height="140%">
          <feGaussianBlur stdDeviation="3" result="blur" />
          <feMerge>
            <feMergeNode in="blur" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>

        {/* Arrow marker — lime */}
        <marker
          id="arrow-lime"
          markerWidth="8"
          markerHeight="8"
          refX="6"
          refY="3"
          orient="auto"
        >
          <path d="M0,0 L0,6 L8,3 z" fill="#abff02" />
        </marker>

        {/* Arrow marker — red */}
        <marker
          id="arrow-red"
          markerWidth="8"
          markerHeight="8"
          refX="6"
          refY="3"
          orient="auto"
        >
          <path d="M0,0 L0,6 L8,3 z" fill="#ef4444" />
        </marker>

        {/* Gradient for data flow dots path */}
        <linearGradient id="dot-grad" x1="0%" y1="0%" x2="100%" y2="0%">
          <stop offset="0%" stopColor="#abff02" />
          <stop offset="100%" stopColor="#abff02" stopOpacity="0.4" />
        </linearGradient>

        {/* Clip path for "Your Machine" box */}
        <clipPath id="clip-machine">
          <rect x="12" y="60" width="258" height="300" rx="10" />
        </clipPath>
      </defs>

      {/* ── Background ── */}
      <rect width="640" height="400" fill="#0a1010" rx="12" />

      {/* Subtle dot grid */}
      <pattern id="dot-grid" x="0" y="0" width="20" height="20" patternUnits="userSpaceOnUse">
        <circle cx="10" cy="10" r="0.8" fill="rgba(171,255,2,0.06)" />
      </pattern>
      <rect width="640" height="400" fill="url(#dot-grid)" rx="12" />

      {/* ══════════════════════════════════════════
          LEFT — "Your Machine" box
          ══════════════════════════════════════════ */}
      <rect
        x="12" y="60" width="258" height="300"
        rx="10" ry="10"
        fill="rgba(5,36,36,0.7)"
        stroke="#abff02"
        strokeWidth="1.5"
      />

      {/* "Your Machine" label */}
      <text
        x="141" y="52"
        textAnchor="middle"
        fontFamily="'Geist Mono', 'Fira Code', monospace"
        fontSize="9"
        fontWeight="600"
        letterSpacing="0.18em"
        fill="#abff02"
        textDecoration="none"
      >
        YOUR MACHINE
      </text>

      {/* MCP Server node */}
      <rect
        x="46" y="90" width="190" height="52"
        rx="26" ry="26"
        fill="#1a2a2a"
        stroke="#abff02"
        strokeWidth="1.5"
      />
      <text
        x="141" y="112"
        textAnchor="middle"
        fontFamily="'Geist Mono', 'Fira Code', monospace"
        fontSize="10"
        fontWeight="700"
        fill="#abff02"
        letterSpacing="0.08em"
      >
        MCP Server
      </text>
      <text
        x="141" y="128"
        textAnchor="middle"
        fontFamily="'Geist Mono', 'Fira Code', monospace"
        fontSize="8"
        fill="rgba(171,255,2,0.55)"
        letterSpacing="0.05em"
      >
        outlook-local-mcp
      </text>

      {/* OS Keychain node */}
      <rect
        x="46" y="184" width="190" height="52"
        rx="8" ry="8"
        fill="#1a2a2a"
        stroke="rgba(171,255,2,0.4)"
        strokeWidth="1"
      />

      {/* Lock icon in keychain node */}
      {/* Lock body */}
      <rect x="57" y="203" width="12" height="10" rx="2" fill="#abff02" />
      {/* Lock shackle */}
      <path d="M59,203 Q59,197 63,197 Q67,197 67,203" fill="none" stroke="#abff02" strokeWidth="1.5" />
      {/* Lock keyhole */}
      <circle cx="63" cy="207" r="1.5" fill="#0a1010" />
      <rect x="62" y="207" width="2" height="3" fill="#0a1010" />

      <text
        x="76" y="207"
        fontFamily="'Geist Mono', 'Fira Code', monospace"
        fontSize="10"
        fontWeight="600"
        fill="rgba(255,255,255,0.85)"
      >
        OS Keychain
      </text>
      <text
        x="76" y="220"
        fontFamily="'Geist Mono', 'Fira Code', monospace"
        fontSize="8"
        fill="rgba(255,255,255,0.35)"
        letterSpacing="0.05em"
      >
        macOS · libsecret · DPAPI
      </text>

      {/* Keychain pulse ring animation */}
      <circle cx="63" cy="208" r="10" fill="none" stroke="#abff02" strokeWidth="0.8" opacity="0.6">
        <animate
          attributeName="r"
          values="10;18;10"
          dur="2.8s"
          repeatCount="indefinite"
        />
        <animate
          attributeName="opacity"
          values="0.6;0;0.6"
          dur="2.8s"
          repeatCount="indefinite"
        />
      </circle>

      {/* AES-256-GCM encrypted file cache node */}
      <rect
        x="46" y="278" width="190" height="52"
        rx="8" ry="8"
        fill="#1a2a2a"
        stroke="rgba(171,255,2,0.25)"
        strokeWidth="1"
        strokeDasharray="4 2"
      />
      <text
        x="141" y="301"
        textAnchor="middle"
        fontFamily="'Geist Mono', 'Fira Code', monospace"
        fontSize="9"
        fontWeight="700"
        fill="rgba(171,255,2,0.7)"
        letterSpacing="0.06em"
      >
        AES-256-GCM
      </text>
      <text
        x="141" y="316"
        textAnchor="middle"
        fontFamily="'Geist Mono', 'Fira Code', monospace"
        fontSize="8"
        fill="rgba(255,255,255,0.35)"
        letterSpacing="0.04em"
      >
        Encrypted file cache (fallback)
      </text>

      {/* MCP Server → OS Keychain arrow (vertical) */}
      <line
        x1="141" y1="142"
        x2="141" y2="183"
        stroke="#abff02"
        strokeWidth="1.5"
        markerEnd="url(#arrow-lime)"
        strokeOpacity="0.7"
      />

      {/* OS Keychain → AES Cache arrow (vertical) */}
      <line
        x1="141" y1="237"
        x2="141" y2="277"
        stroke="rgba(171,255,2,0.4)"
        strokeWidth="1"
        markerEnd="url(#arrow-lime)"
        strokeDasharray="4 2"
      />
      <text
        x="150" y="261"
        fontFamily="'Geist Mono', 'Fira Code', monospace"
        fontSize="7"
        fill="rgba(171,255,2,0.4)"
        letterSpacing="0.08em"
      >
        FALLBACK
      </text>

      {/* ══════════════════════════════════════════
          CENTER — Network Boundary
          ══════════════════════════════════════════ */}
      <line
        x1="310" y1="55"
        x2="310" y2="370"
        stroke="rgba(255,255,255,0.12)"
        strokeWidth="1"
        strokeDasharray="5 4"
      />
      <text
        x="310" y="44"
        textAnchor="middle"
        fontFamily="'Geist Mono', 'Fira Code', monospace"
        fontSize="7"
        fontWeight="600"
        letterSpacing="0.18em"
        fill="rgba(255,255,255,0.25)"
      >
        NETWORK BOUNDARY
      </text>

      {/* ══════════════════════════════════════════
          RIGHT — "Microsoft Cloud" box
          ══════════════════════════════════════════ */}
      <rect
        x="370" y="60" width="220" height="200"
        rx="10" ry="10"
        fill="rgba(26,42,42,0.4)"
        stroke="rgba(194,194,194,0.25)"
        strokeWidth="1"
      />
      <text
        x="480" y="52"
        textAnchor="middle"
        fontFamily="'Geist Mono', 'Fira Code', monospace"
        fontSize="9"
        fontWeight="600"
        letterSpacing="0.18em"
        fill="rgba(194,194,194,0.6)"
      >
        MICROSOFT CLOUD
      </text>

      {/* Cloud icon helper — reusable group shapes */}
      {/* Graph API node */}
      <rect
        x="388" y="90" width="184" height="60"
        rx="8" ry="8"
        fill="#1a2a2a"
        stroke="rgba(194,194,194,0.3)"
        strokeWidth="1"
      />
      {/* Cloud icon for Graph API */}
      <circle cx="406" cy="117" r="7" fill="rgba(194,194,194,0.2)" stroke="rgba(194,194,194,0.4)" strokeWidth="1" />
      <circle cx="413" cy="113" r="5" fill="rgba(194,194,194,0.2)" stroke="rgba(194,194,194,0.4)" strokeWidth="1" />
      <circle cx="421" cy="116" r="6" fill="rgba(194,194,194,0.2)" stroke="rgba(194,194,194,0.4)" strokeWidth="1" />
      <rect x="400" y="117" width="27" height="5" fill="rgba(194,194,194,0.2)" />

      <text
        x="436" y="112"
        fontFamily="'Geist Mono', 'Fira Code', monospace"
        fontSize="10"
        fontWeight="600"
        fill="rgba(255,255,255,0.8)"
      >
        Graph API
      </text>
      <text
        x="436" y="125"
        fontFamily="'Geist Mono', 'Fira Code', monospace"
        fontSize="7"
        fill="rgba(255,255,255,0.3)"
        letterSpacing="0.05em"
      >
        graph.microsoft.com
      </text>

      {/* Identity Platform node */}
      <rect
        x="388" y="172" width="184" height="60"
        rx="8" ry="8"
        fill="#1a2a2a"
        stroke="rgba(194,194,194,0.3)"
        strokeWidth="1"
      />
      {/* Cloud icon for Identity */}
      <circle cx="406" cy="199" r="7" fill="rgba(194,194,194,0.15)" stroke="rgba(194,194,194,0.35)" strokeWidth="1" />
      <circle cx="413" cy="195" r="5" fill="rgba(194,194,194,0.15)" stroke="rgba(194,194,194,0.35)" strokeWidth="1" />
      <circle cx="421" cy="198" r="6" fill="rgba(194,194,194,0.15)" stroke="rgba(194,194,194,0.35)" strokeWidth="1" />
      <rect x="400" y="199" width="27" height="5" fill="rgba(194,194,194,0.15)" />

      <text
        x="436" y="194"
        fontFamily="'Geist Mono', 'Fira Code', monospace"
        fontSize="10"
        fontWeight="600"
        fill="rgba(255,255,255,0.8)"
      >
        Identity Platform
      </text>
      <text
        x="436" y="207"
        fontFamily="'Geist Mono', 'Fira Code', monospace"
        fontSize="7"
        fill="rgba(255,255,255,0.3)"
        letterSpacing="0.05em"
      >
        login.microsoftonline.com
      </text>

      {/* ══════════════════════════════════════════
          ARROWS — MCP Server → Graph API (HTTPS)
          ══════════════════════════════════════════ */}
      {/* Horizontal arrow from MCP Server right edge to Graph API left edge */}
      <path
        d="M 270,116 L 386,116"
        fill="none"
        stroke="#abff02"
        strokeWidth="1.8"
        markerEnd="url(#arrow-lime)"
        filter="url(#lime-glow)"
      />
      <text
        x="316" y="110"
        textAnchor="middle"
        fontFamily="'Geist Mono', 'Fira Code', monospace"
        fontSize="8"
        fontWeight="600"
        fill="#abff02"
        letterSpacing="0.08em"
      >
        HTTPS
      </text>

      {/* Animated data-flow dot on HTTPS arrow */}
      <circle r="3.5" fill="#abff02" opacity="0.9">
        <animateMotion
          path="M 270,116 L 386,116"
          dur="2.2s"
          repeatCount="indefinite"
          calcMode="linear"
        />
        <animate
          attributeName="opacity"
          values="0;1;1;0"
          keyTimes="0;0.1;0.85;1"
          dur="2.2s"
          repeatCount="indefinite"
        />
      </circle>

      {/* Second dot offset */}
      <circle r="3.5" fill="#abff02" opacity="0.5">
        <animateMotion
          path="M 270,116 L 386,116"
          dur="2.2s"
          begin="1.1s"
          repeatCount="indefinite"
          calcMode="linear"
        />
        <animate
          attributeName="opacity"
          values="0;0.6;0.6;0"
          keyTimes="0;0.1;0.85;1"
          dur="2.2s"
          begin="1.1s"
          repeatCount="indefinite"
        />
      </circle>

      {/* ══════════════════════════════════════════
          ARROW — MCP Server → Identity Platform (OAuth 2.0)
          ══════════════════════════════════════════ */}
      {/* Elbow: from MCP right edge, down then right to Identity Platform */}
      <path
        d="M 270,116 Q 328,116 328,199 L 386,199"
        fill="none"
        stroke="#abff02"
        strokeWidth="1.8"
        markerEnd="url(#arrow-lime)"
        filter="url(#lime-glow)"
      />
      <text
        x="354" y="213"
        textAnchor="middle"
        fontFamily="'Geist Mono', 'Fira Code', monospace"
        fontSize="8"
        fontWeight="600"
        fill="#abff02"
        letterSpacing="0.08em"
      >
        OAuth 2.0
      </text>

      {/* Animated data-flow dot on OAuth arrow */}
      <circle r="3" fill="#abff02" opacity="0.9">
        <animateMotion
          path="M 270,116 Q 328,116 328,199 L 386,199"
          dur="3s"
          begin="0.5s"
          repeatCount="indefinite"
          calcMode="linear"
        />
        <animate
          attributeName="opacity"
          values="0;1;1;0"
          keyTimes="0;0.08;0.88;1"
          dur="3s"
          begin="0.5s"
          repeatCount="indefinite"
        />
      </circle>

      {/* ══════════════════════════════════════════
          Third-party Servers — Crossed out
          ══════════════════════════════════════════ */}
      <rect
        x="370" y="285" width="220" height="60"
        rx="8" ry="8"
        fill="rgba(239,68,68,0.06)"
        stroke="rgba(239,68,68,0.3)"
        strokeWidth="1"
        strokeDasharray="5 3"
      />
      <text
        x="480" y="310"
        textAnchor="middle"
        fontFamily="'Geist Mono', 'Fira Code', monospace"
        fontSize="10"
        fontWeight="600"
        fill="rgba(239,68,68,0.55)"
      >
        Third-party Servers
      </text>
      <text
        x="480" y="326"
        textAnchor="middle"
        fontFamily="'Geist Mono', 'Fira Code', monospace"
        fontSize="7"
        fill="rgba(239,68,68,0.35)"
        letterSpacing="0.08em"
      >
        Proxies · SaaS middleware · Clouds
      </text>

      {/* Red X mark */}
      <line x1="375" y1="290" x2="585" y2="340" stroke="#ef4444" strokeWidth="2.5" strokeOpacity="0.75" />
      <line x1="585" y1="290" x2="375" y2="340" stroke="#ef4444" strokeWidth="2.5" strokeOpacity="0.75" />

      {/* Arrow from MCP to third-party — blocked */}
      <path
        d="M 270,200 Q 300,200 300,315 L 368,315"
        fill="none"
        stroke="rgba(239,68,68,0.4)"
        strokeWidth="1.2"
        strokeDasharray="4 3"
        markerEnd="url(#arrow-red)"
      />

      {/* NO label on blocked arrow */}
      <text
        x="295" y="273"
        textAnchor="middle"
        fontFamily="'Geist Mono', 'Fira Code', monospace"
        fontSize="8"
        fontWeight="700"
        fill="rgba(239,68,68,0.7)"
        letterSpacing="0.1em"
      >
        BLOCKED
      </text>

      {/* ══════════════════════════════════════════
          Bottom label
          ══════════════════════════════════════════ */}
      <text
        x="320" y="388"
        textAnchor="middle"
        fontFamily="'Geist Mono', 'Fira Code', monospace"
        fontSize="8"
        fontWeight="700"
        letterSpacing="0.18em"
        fill="rgba(171,255,2,0.55)"
      >
        100% LOCAL — All data processing occurs on your machine
      </text>
    </svg>
  )
}
