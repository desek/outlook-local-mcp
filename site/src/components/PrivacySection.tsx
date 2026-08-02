import { useRef, useState } from 'react'
import gsap from 'gsap'
import { ScrollTrigger } from 'gsap/ScrollTrigger'
import { useGSAP } from '@gsap/react'

const TRUST_BULLETS = [
  {
    title: 'OS-Native Token Storage',
    detail: 'OAuth tokens stored in macOS Keychain, Linux libsecret, or Windows DPAPI. Your credentials never leave your operating system\'s secure enclave.',
    icon: 'keychain',
  },
  {
    title: 'AES-256-GCM Fallback',
    detail: 'When the OS keychain is unavailable, tokens are encrypted with AES-256-GCM in a local file. No plaintext credentials, ever.',
    icon: 'encrypt',
  },
  {
    title: 'Outbound Only',
    detail: 'Outbound requests reach only Microsoft\'s own endpoints — the Graph API and the Identity Platform — plus any telemetry endpoint you configure. No third party relays your data. The one inbound socket is a temporary loopback port opened only for interactive browser sign-in.',
    icon: 'network',
  },
  {
    title: 'PII Sanitization',
    detail: 'Structured logging with PII sanitization enabled by default. Event subjects, attendee emails, and message content are stripped from logs.',
    icon: 'sanitize',
  },
  {
    title: 'OData Injection Protection',
    detail: 'All user inputs validated and escaped before reaching the Graph API. OData query injection is blocked at the request construction layer.',
    icon: 'shield',
  },
  {
    title: 'Read-Only Mode',
    detail: 'Set OUTLOOK_MCP_READ_ONLY=true to disable all write operations. Perfect for evaluation or security-restricted environments.',
    icon: 'readonly',
  },
] as const

const SECURITY_ICON_PATHS: Record<string, string> = {
  keychain: 'M12 2a5 5 0 0 0-5 5v3H5a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-8a2 2 0 0 0-2-2h-2V7a5 5 0 0 0-5-5zm-3 5a3 3 0 1 1 6 0v3H9V7zm3 8a1.5 1.5 0 1 1 0 3 1.5 1.5 0 0 1 0-3z',
  encrypt: 'M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z',
  network: 'M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5',
  sanitize: 'M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z M14 2v6h6 M16 13H8 M16 17H8 M10 9H8',
  shield: 'M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z M9 12l2 2 4-4',
  readonly: 'M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z M12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6z',
}

export default function PrivacySection() {
  const sectionRef = useRef<HTMLElement>(null)
  const headingRef = useRef<HTMLHeadingElement>(null)
  const eyebrowRef = useRef<HTMLSpanElement>(null)
  const bulletRefs = useRef<(HTMLDivElement | null)[]>([])
  const [hoveredIndex, setHoveredIndex] = useState<number | null>(null)

  useGSAP(() => {
    const mm = gsap.matchMedia()
    mm.add('(prefers-reduced-motion: no-preference)', () => {
      const targets = [
        eyebrowRef.current,
        headingRef.current,
        ...bulletRefs.current.filter(Boolean),
      ].filter(Boolean)

      gsap.set(targets, { opacity: 0, y: 15 })

      ScrollTrigger.create({
        trigger: sectionRef.current,
        start: 'top 80%',
        once: true,
        onEnter: () => {
          gsap.to(targets, {
            opacity: 1, y: 0, stagger: 0.06, duration: 0.4, ease: 'power2.out',
          })
        },
      })
    })

    mm.add('(prefers-reduced-motion: reduce)', () => {
      // Ensure everything is visible without animation
      const targets = [
        eyebrowRef.current,
        headingRef.current,
        ...bulletRefs.current.filter(Boolean),
      ].filter(Boolean)
      gsap.set(targets, { opacity: 1, y: 0 })
    })
  }, { scope: sectionRef })

  return (
    <section ref={sectionRef} id="privacy" className="relative bg-white py-16 sm:py-20 lg:py-24">
      <div className="container-page">
        <div className="flex flex-col lg:flex-row lg:gap-16">
          {/* ── Left: Heading + bullets ── */}
          <div className="lg:w-[45%]">
            <span
              ref={eyebrowRef}
              className="font-mono text-label font-semibold tracking-[0.18em] text-lime-dark uppercase block mb-3"
            >
              Is your data private?
            </span>
            <h2
              ref={headingRef}
              className="text-[1.75rem] sm:text-[2.25rem] lg:text-section-heading font-sans font-normal text-brand-dark tracking-tight leading-tight mb-8 sm:mb-10"
            >
              Every credential stays on your machine.
            </h2>
            <p className="text-base text-gray-400 font-sans leading-relaxed mb-8 max-w-md">
              No credential and no message body is relayed through a third party. The only
              outbound destinations are Microsoft&rsquo;s own endpoints, plus any telemetry
              endpoint you configure. Verifiable, auditable, explainable to your security team.
            </p>

            {/* Bullet list */}
            <div className="space-y-3">
              {TRUST_BULLETS.map((bullet, i) => (
                <div
                  key={bullet.title}
                  ref={(el) => { bulletRefs.current[i] = el }}
                  className={`flex items-start gap-4 p-4 rounded-lg border-l-2 transition-all duration-200 cursor-default ${
                    hoveredIndex === i
                      ? 'border-l-brand-lime bg-brand-lime/[0.04]'
                      : 'border-l-gray-100 bg-transparent'
                  }`}
                  onMouseEnter={() => setHoveredIndex(i)}
                  onMouseLeave={() => setHoveredIndex(null)}
                >
                  <div className="shrink-0 w-8 h-8 flex items-center justify-center">
                    <svg
                      width="20" height="20" viewBox="0 0 24 24" fill="none"
                      stroke={hoveredIndex === i ? 'var(--color-brand-lime)' : 'var(--color-lime-mid)'}
                      strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"
                      className="transition-colors duration-200"
                    >
                      <path d={SECURITY_ICON_PATHS[bullet.icon] ?? SECURITY_ICON_PATHS['shield']!} />
                    </svg>
                  </div>
                  <div>
                    <h3 className="text-sm font-sans font-medium text-brand-dark mb-1">
                      {bullet.title}
                    </h3>
                    <p className="text-sm text-gray-400 font-sans leading-relaxed">
                      {bullet.detail}
                    </p>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* ── Right: Architecture diagram ── */}
          <div className="lg:w-[55%] mt-10 lg:mt-0">
            <div className="lg:sticky lg:top-24 w-full rounded-lg bg-gray-900 overflow-hidden" style={{ aspectRatio: '16/10' }}>
              {/* ASSET: CapabilityPrivacyDiagram.tsx */}
              <div className="w-full h-full flex flex-col items-center justify-center p-8 relative">
                {/* Simplified architecture diagram as inline SVG */}
                <svg viewBox="0 0 800 500" className="w-full h-auto max-w-lg" fill="none" xmlns="http://www.w3.org/2000/svg">
                  {/* Outer boundary: "Your Machine" */}
                  <rect
                    x="40" y="40" width="520" height="420"
                    rx="12"
                    stroke="rgba(171,255,2,0.4)"
                    strokeWidth="2"
                    strokeDasharray="8 4"
                  />
                  <text x="60" y="75" fill="rgba(171,255,2,0.6)" fontFamily="monospace" fontSize="11" letterSpacing="2">
                    YOUR MACHINE
                  </text>

                  {/* MCP Client box */}
                  <rect x="80" y="120" width="200" height="80" rx="8" fill="#0f1f1f" stroke="rgba(255,255,255,0.15)" strokeWidth="1" />
                  <text x="180" y="155" fill="white" fontFamily="monospace" fontSize="11" textAnchor="middle" letterSpacing="1">
                    MCP CLIENT
                  </text>
                  <text x="180" y="175" fill="rgba(255,255,255,0.5)" fontFamily="monospace" fontSize="10" textAnchor="middle">
                    (Claude)
                  </text>

                  {/* MCP Server box */}
                  <rect x="80" y="250" width="200" height="80" rx="8" fill="#0f1f1f" stroke="rgba(171,255,2,0.3)" strokeWidth="1" />
                  <text x="180" y="285" fill="#abff02" fontFamily="monospace" fontSize="11" textAnchor="middle" letterSpacing="1">
                    MCP SERVER
                  </text>
                  <text x="180" y="305" fill="rgba(255,255,255,0.5)" fontFamily="monospace" fontSize="10" textAnchor="middle">
                    (outlook-local-mcp)
                  </text>

                  {/* OS Keychain box */}
                  <rect x="80" y="380" width="200" height="60" rx="8" fill="#0f1f1f" stroke="rgba(255,255,255,0.15)" strokeWidth="1" />
                  <text x="180" y="415" fill="white" fontFamily="monospace" fontSize="11" textAnchor="middle" letterSpacing="1">
                    OS KEYCHAIN
                  </text>

                  {/* Arrows: Client ↔ Server */}
                  <line x1="180" y1="200" x2="180" y2="250" stroke="rgba(171,255,2,0.6)" strokeWidth="1.5" markerEnd="url(#arrowGreen)" />
                  <line x1="190" y1="250" x2="190" y2="200" stroke="rgba(171,255,2,0.6)" strokeWidth="1.5" markerEnd="url(#arrowGreen)" />

                  {/* Arrow: Server → Keychain */}
                  <line x1="180" y1="330" x2="180" y2="380" stroke="rgba(171,255,2,0.6)" strokeWidth="1.5" markerEnd="url(#arrowGreen)" />

                  {/* External: Graph API box */}
                  <rect x="600" y="250" width="180" height="80" rx="8" fill="transparent" stroke="rgba(255,255,255,0.2)" strokeWidth="1" strokeDasharray="4 3" />
                  <text x="690" y="285" fill="rgba(255,255,255,0.5)" fontFamily="monospace" fontSize="10" textAnchor="middle" letterSpacing="1">
                    MICROSOFT
                  </text>
                  <text x="690" y="305" fill="rgba(255,255,255,0.5)" fontFamily="monospace" fontSize="10" textAnchor="middle">
                    GRAPH API
                  </text>

                  {/* Arrow: Server → Graph API (exits boundary) */}
                  <line x1="280" y1="290" x2="600" y2="290" stroke="rgba(255,255,255,0.3)" strokeWidth="1.5" markerEnd="url(#arrowWhite)" />
                  <text x="440" y="278" fill="rgba(171,255,2,0.4)" fontFamily="monospace" fontSize="9" textAnchor="middle">
                    OUTBOUND ONLY
                  </text>

                  {/* Arrow markers */}
                  <defs>
                    <marker id="arrowGreen" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
                      <path d="M0,0 L8,3 L0,6" fill="rgba(171,255,2,0.6)" />
                    </marker>
                    <marker id="arrowWhite" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
                      <path d="M0,0 L8,3 L0,6" fill="rgba(255,255,255,0.3)" />
                    </marker>
                  </defs>
                </svg>
              </div>
            </div>
          </div>
        </div>

        {/* ── Observability callout ── */}
        <div className="mt-12 sm:mt-16 p-6 bg-brand-off-white rounded-lg">
          <div className="flex flex-col sm:flex-row items-start sm:items-center gap-4">
            <span className="font-mono text-label font-semibold tracking-[0.18em] text-lime-dark uppercase shrink-0">
              Observability
            </span>
            <p className="text-sm text-gray-600 font-sans leading-relaxed">
              Optional OpenTelemetry export (OTLP gRPC) — zero overhead when disabled.
              Per-tool audit logging, structured JSON output, configurable log levels.
              Exponential backoff retry on transient Graph API errors. Graceful SIGINT/SIGTERM shutdown.
            </p>
          </div>
        </div>
      </div>
    </section>
  )
}
