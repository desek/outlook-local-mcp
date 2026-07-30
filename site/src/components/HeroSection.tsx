import { useRef, useState, useCallback } from 'react'
import gsap from 'gsap'
import { useGSAP } from '@gsap/react'
import HeroBackground from './canvas/HeroBackground'
import { useScrollbarHeight } from '../use-scrollbar-height'

const INSTALL_CMD = 'go install github.com/desek/outlook-local-mcp/cmd/outlook-local-mcp@latest'

const STATS = [
  { id: 'local', label: '100% LOCAL', desc: 'No intermediate servers' },
  { id: 'azure', label: 'ZERO ENTRA ID', desc: 'Pre-authorized client ID' },
  { id: 'tools', label: '23 MCP TOOLS', desc: 'Calendar, mail, multi-account' },
] as const

export default function HeroSection() {
  const sectionRef = useRef<HTMLElement>(null)
  const headlineRef = useRef<HTMLHeadingElement>(null)
  const subheadRef = useRef<HTMLParagraphElement>(null)
  const badgeRefs = useRef<(HTMLDivElement | null)[]>([])
  const installRef = useRef<HTMLDivElement>(null)
  const scrollerRef = useRef<HTMLDivElement>(null)
  const scrollIndicatorRef = useRef<HTMLDivElement>(null)
  const [copied, setCopied] = useState(false)

  useScrollbarHeight(scrollerRef)

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(INSTALL_CMD)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      // Fallback — select text
    }
  }, [])

  /* ── Hero scroll journey: scroll-position-bound GSAP timeline ── */
  useGSAP(() => {
    const mm = gsap.matchMedia()

    mm.add('(prefers-reduced-motion: no-preference)', () => {
      // All hero content: time-based staggered entrance
      gsap.set(headlineRef.current, { opacity: 0, y: 30 })
      gsap.set(subheadRef.current, { opacity: 0, y: 20 })
      gsap.set(badgeRefs.current, { opacity: 0, y: 15 })
      gsap.set(installRef.current, { opacity: 0, y: 20 })

      gsap.to(headlineRef.current, { opacity: 1, y: 0, duration: 0.6, ease: 'power2.out', delay: 0.15 })
      gsap.to(subheadRef.current, { opacity: 1, y: 0, duration: 0.5, ease: 'power2.out', delay: 0.35 })
      gsap.to(badgeRefs.current, { opacity: 1, y: 0, stagger: 0.08, duration: 0.4, ease: 'power2.out', delay: 0.55 })
      gsap.to(installRef.current, { opacity: 1, y: 0, duration: 0.5, ease: 'power2.out', delay: 0.8 })

      // Scroll indicator pulse
      gsap.to(scrollIndicatorRef.current, {
        y: 8,
        opacity: 0.4,
        duration: 1.2,
        ease: 'sine.inOut',
        yoyo: true,
        repeat: -1,
      })
    })

    mm.add('(prefers-reduced-motion: reduce)', () => {
      gsap.set([headlineRef.current, subheadRef.current, installRef.current], { opacity: 1, y: 0 })
      gsap.set(badgeRefs.current, { opacity: 1, y: 0 })
    })
  }, { scope: sectionRef })

  return (
    <section
      ref={sectionRef}
      id="hero"
      className="relative"
    >
      <div
        className="relative w-full overflow-hidden flex flex-col items-center justify-center"
        style={{ height: '100vh' }}
      >
          {/* ── Background: atmospheric dark canvas ── */}
          <HeroBackground className="absolute inset-0 z-0 pointer-events-none" />

          {/* ── Content ── */}
          <div className="relative z-10 container-page flex flex-col items-center text-center px-5 lg:px-0">
            {/* Headline */}
            <h1
              ref={headlineRef}
              className="text-[2rem] sm:text-[2.5rem] lg:text-hero font-sans font-normal text-white tracking-tight leading-tight max-w-4xl"
              style={{ textShadow: '0 2px 20px rgba(0,0,0,0.8)' }}
            >
              Your AI's native
              <br />
              interface to Outlook.
            </h1>

            {/* Stat badges */}
            <div className="mt-8 sm:mt-10 flex flex-col sm:flex-row items-center gap-4 sm:gap-8">
              {STATS.map((stat, i) => (
                <div
                  key={stat.id}
                  ref={(el) => { badgeRefs.current[i] = el }}
                  className="flex flex-col items-center gap-1"
                >
                  <span className="font-mono text-label font-semibold tracking-[0.18em] text-brand-lime">
                    {stat.label}
                  </span>
                  <span className="text-xs text-white/50 font-sans">
                    {stat.desc}
                  </span>
                </div>
              ))}
            </div>

            {/* Install command block */}
            <div
              ref={installRef}
              className="mt-8 sm:mt-10 w-full max-w-3xl lg:w-fit lg:max-w-full"
            >
              <div
                className="code-block flex items-center gap-3 px-4 py-3 sm:px-5 sm:py-4"
                role="group"
                aria-label="Install command"
              >
                <div ref={scrollerRef} className="install-scroller flex-1 min-w-0 overflow-x-auto">
                  <code
                    className="text-white/90 font-mono whitespace-nowrap"
                    style={{ fontSize: 'clamp(0.55rem, 2.2vw, 1rem)' }}
                  >
                    <span className="text-brand-lime">$</span>{' '}
                    {INSTALL_CMD}
                  </code>
                </div>
                <button
                  onClick={handleCopy}
                  className="shrink-0 w-11 h-11 flex items-center justify-center text-brand-lime hover:text-white transition-colors duration-200 rounded-md"
                  aria-label="Copy install command"
                >
                  {copied ? (
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                      <polyline points="20 6 9 17 4 12" />
                    </svg>
                  ) : (
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                      <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
                      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
                    </svg>
                  )}
                </button>
              </div>

              {/* Secondary CTA */}
              <div className="mt-4 flex flex-col sm:flex-row items-center justify-center gap-3">
                <a
                  href="https://github.com/desek/outlook-local-mcp/releases"
                  target="_blank"
                  rel="noopener"
                  className="text-sm font-mono tracking-wide text-white/60 hover:text-brand-lime transition-colors duration-200"
                >
                  Download Claude Desktop extension (.mcpb)
                  <span className="ml-1">&#8599;</span>
                </a>
              </div>
            </div>
          </div>

          {/* ── Scroll indicator ── */}
          <div
            ref={scrollIndicatorRef}
            className="absolute bottom-8 left-1/2 -translate-x-1/2 flex flex-col items-center gap-2 z-10"
          >
            <span className="font-mono text-label font-semibold tracking-[0.18em] text-white/50 uppercase">
              Scroll to explore
            </span>
            <svg width="16" height="24" viewBox="0 0 16 24" fill="none" className="text-white/40">
              <path d="M8 4v16M8 20l-4-4M8 20l4-4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </div>
      </div>

    </section>
  )
}
