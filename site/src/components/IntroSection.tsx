import { useRef } from 'react'
import gsap from 'gsap'
import { useGSAP } from '@gsap/react'
import NotchCornerMask from './NotchCornerMask'

const FACTS = [
  { label: 'OAuth 2.0', desc: 'Device code, browser, or PKCE' },
  { label: 'OS Keychain', desc: 'macOS, Linux, Windows native storage' },
  { label: 'Graph API v1.0', desc: 'Calendar + Mail scopes only' },
] as const

const TYPED_WORD = 'locally'

export default function IntroSection() {
  const sectionRef = useRef<HTMLElement>(null)
  const headingRef = useRef<HTMLHeadingElement>(null)
  const charRefs = useRef<(HTMLSpanElement | null)[]>([])
  const factRefs = useRef<(HTMLDivElement | null)[]>([])

  useGSAP(() => {
    const mm = gsap.matchMedia()

    mm.add('(prefers-reduced-motion: no-preference)', () => {
      // Heading entrance
      gsap.from(headingRef.current, {
        opacity: 0,
        y: 30,
        duration: 0.6,
        ease: 'cubic-bezier(0, 0, 0.58, 1)',
        scrollTrigger: { trigger: headingRef.current, start: 'top 85%' },
      })

      // Typed animation for "locally"
      const chars = charRefs.current.filter(Boolean)
      if (chars.length > 0) {
        gsap.fromTo(
          chars,
          { opacity: 0.2, color: 'var(--color-brand-dark)' },
          {
            opacity: 1,
            color: 'var(--color-brand-lime)',
            stagger: 0.06,
            duration: 0.1,
            ease: 'none',
            scrollTrigger: { trigger: chars[0], start: 'top 70%' },
            onComplete: () => {
              gsap.to(chars, { color: 'var(--color-brand-dark)', duration: 0.4, ease: 'cubic-bezier(0, 0, 0.58, 1)' })
            },
          },
        )
      }

      // Fact grid stagger
      gsap.from(factRefs.current.filter(Boolean), {
        opacity: 0,
        y: 20,
        stagger: 0.1,
        duration: 0.4,
        ease: 'cubic-bezier(0, 0, 0.58, 1)',
        scrollTrigger: { trigger: factRefs.current[0], start: 'top 85%' },
      })
    })

    mm.add('(prefers-reduced-motion: reduce)', () => {
      gsap.set([headingRef.current, ...charRefs.current.filter(Boolean), ...factRefs.current.filter(Boolean)], {
        opacity: 1,
        y: 0,
      })
    })
  }, { scope: sectionRef })

  return (
    <section
      ref={sectionRef}
      id="intro"
      className="relative bg-white py-24 sm:py-32 lg:py-40"
    >
      {/* Notch mask from dark hero */}
      <NotchCornerMask position="top" color="#000000" radius={60} />

      <div className="container-page">
        <div className="flex flex-col lg:flex-row lg:items-start lg:gap-16">
          {/* ── Left: Display heading ── */}
          <div className="lg:w-[45%]">
            <h2
              ref={headingRef}
              className="text-[1.75rem] sm:text-[2.25rem] lg:text-display font-sans font-normal text-brand-dark tracking-tight leading-tight"
            >
              Connect Claude to your calendar.
              <br />
              No servers. No registration.
              <br />
              Just your data,{' '}
              <span className="inline-flex">
                {TYPED_WORD.split('').map((char, i) => (
                  <span
                    key={i}
                    ref={(el) => { charRefs.current[i] = el }}
                    className="inline-block"
                    style={{ color: 'var(--color-brand-dark)' }}
                  >
                    {char}
                  </span>
                ))}
              </span>
              .
            </h2>

            {/* Body text */}
            <p className="mt-6 sm:mt-8 text-base text-gray-400 font-sans leading-relaxed max-w-md">
              A Model Context Protocol server that connects Claude — or any MCP client —
              directly to Microsoft Calendar and Mail via the Graph API. All data stays
              on your machine. OAuth tokens live in your OS keychain. The server process
              never leaves localhost.
            </p>
          </div>

          {/* ── Right: Fact grid ── */}
          <div className="lg:w-[55%] mt-10 lg:mt-0">
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-6 sm:gap-8">
              {FACTS.map((fact, i) => (
                <div
                  key={fact.label}
                  ref={(el) => { factRefs.current[i] = el }}
                  className="bg-brand-off-white rounded-lg p-6 sm:p-8"
                >
                  <span className="font-mono text-label font-semibold tracking-[0.18em] text-lime-dark uppercase block mb-2">
                    {fact.label}
                  </span>
                  <span className="text-sm text-gray-600 font-sans leading-relaxed">
                    {fact.desc}
                  </span>
                </div>
              ))}
            </div>

            {/* Tech stack inline callout */}
            <div className="mt-8 grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4">
              {[
                { label: 'Language', value: 'Go 1.24+' },
                { label: 'Protocol', value: 'MCP (JSON-RPC)' },
                { label: 'API', value: 'Graph API v1.0' },
                { label: 'License', value: 'MIT' },
                { label: 'Auth', value: 'OAuth 2.0' },
                { label: 'Token Storage', value: 'OS Keychain' },
                { label: 'Logging', value: 'Structured JSON' },
                { label: 'Resilience', value: 'Exp. backoff' },
              ].map((item) => (
                <div key={item.label} className="flex flex-col gap-0.5">
                  <span className="font-mono text-[10px] sm:text-label font-semibold tracking-[0.18em] text-lime-dark uppercase">
                    {item.label}
                  </span>
                  <span className="text-sm text-brand-dark font-sans">
                    {item.value}
                  </span>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}
