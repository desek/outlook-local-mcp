import { useRef } from 'react'
import gsap from 'gsap'
import { ScrollTrigger } from 'gsap/ScrollTrigger'
import { useGSAP } from '@gsap/react'
import CapabilityCalendar from './svg/CapabilityCalendar'
import CapabilityMultiAccount from './svg/CapabilityMultiAccount'
import CapabilityMailSearch from './svg/CapabilityMailSearch'
import CapabilityPrivacyDiagram from './svg/CapabilityPrivacyDiagram'
import CapabilityAuthFlow from './svg/CapabilityAuthFlow'
import { domainByName } from '../surface'

/**
 * capabilityVerbNames returns the default-exposed verbs of a domain, as `operation`
 * values (no flat prefix), read from the surface manifest. A capability card that names a
 * domain renders these rather than a hand-written list, so the site states no verb name of
 * its own (CR-0073).
 *
 * @param domain  The aggregate tool name (for example "calendar").
 * @returns The verb names exposed under the default configuration, in manifest order.
 */
function capabilityVerbNames(domain: string): string[] {
  const d = domainByName(domain)
  return d ? d.verbs.filter((v) => v.gate === null).map((v) => v.name) : []
}

const CAPABILITIES = [
  {
    id: '01',
    label: 'Calendar Management',
    domain: 'calendar',
    summary: 'Read, search, create, update, and delete calendar events and meetings. Check free/busy availability across accounts.',
    keyDetail: 'Meeting tools include attendee confirmation guidance and extra warnings for external attendees — the LLM won\'t accidentally spam a meeting invite without a check.',
  },
  {
    id: '02',
    label: 'Multi-Account',
    domain: 'account',
    summary: 'Add, list, and remove Microsoft accounts at runtime. Each account gets isolated token storage. Accounts persist across restarts.',
    keyDetail: 'Lazy auth — no credentials required at startup. Authentication triggers on first tool call per account.',
  },
  {
    id: '03',
    label: 'Mail Access (opt-in)',
    domain: 'mail',
    summary: 'Read-only access to mailbox folders, messages, and full-text search via KQL. Disabled by default; enabled with one env var.',
    keyDetail: 'Opt-in only. Set OUTLOOK_MCP_MAIL_ENABLED=true. Never writes to mail.',
  },
  {
    id: '04',
    label: 'Local Privacy & Security',
    summary: 'No data routing through third parties. Every credential and token stays on your machine.',
    keyDetail: 'AES-256-GCM encrypted file fallback when OS keychain is unavailable. OData injection protection on all inputs.',
    bullets: [
      'OAuth tokens in macOS Keychain / Linux libsecret / Windows DPAPI',
      'AES-256-GCM encrypted file fallback',
      'Only outbound: Graph API + Microsoft Identity Platform',
      'PII sanitization built into structured logging',
      'OData injection protection on all inputs',
      'Read-only mode via single env var',
    ],
  },
  {
    id: '05',
    label: 'Zero-Config Auth',
    summary: 'Three auth methods, all requiring ZERO ENTRA ID app registration.',
    keyDetail: 'Token expiry is ~90 days. Silent refresh is automatic. First-time auth is a one-time browser action.',
    authMethods: [
      { name: 'Device code (default)', desc: 'Displays URL + code. Works in headless environments.' },
      { name: 'Interactive browser', desc: 'Opens system browser, listens on localhost.' },
      { name: 'Authorization code (PKCE)', desc: 'For fully headless/remote setups.' },
    ],
  },
] as const

export default function CapabilitiesSection() {
  const sectionRef = useRef<HTMLElement>(null)
  const wrapperRef = useRef<HTMLDivElement>(null)
  const desktopTextRefs = useRef<(HTMLDivElement | null)[]>([])
  const desktopVisualRefs = useRef<(HTMLDivElement | null)[]>([])
  const eyebrowRef = useRef<HTMLSpanElement>(null)
  const headingRef = useRef<HTMLHeadingElement>(null)
  const desktopHeadingBlockRef = useRef<HTMLDivElement>(null)
  const rightPanelRef = useRef<HTMLDivElement>(null)
  const mobileWrapperRef = useRef<HTMLDivElement>(null)
  const mobilePanelRefs = useRef<(HTMLDivElement | null)[]>([])

  /* ── GSAP scroll-pinned on desktop ── */
  useGSAP(() => {
    const mm = gsap.matchMedia()

    mm.add('(min-width: 1024px) and (prefers-reduced-motion: no-preference)', () => {
      const n = CAPABILITIES.length
      const headingBlock = desktopHeadingBlockRef.current
      const wrapper = wrapperRef.current
      if (!headingBlock || !wrapper) return

      // Compute offset to center the heading block within the pinned wrapper
      const wrapperRect = wrapper.getBoundingClientRect()
      const headingRect = headingBlock.getBoundingClientRect()
      const centerX = (wrapperRect.width / 2) - (headingRect.left - wrapperRect.left + headingRect.width / 2)
      const centerY = (wrapperRect.height / 2) - (headingRect.top - wrapperRect.top + headingRect.height / 2)

      // Set initial states
      // Heading: starts centered in viewport
      gsap.set(headingBlock, { x: centerX, y: centerY })
      // Right panel bg: starts transparent so heading is visible on white
      if (rightPanelRef.current) gsap.set(rightPanelRef.current, { opacity: 0 })
      // Capability panels: all hidden initially (heading visible first)
      desktopTextRefs.current.forEach((el) => {
        if (el) gsap.set(el, { opacity: 0 })
      })
      desktopVisualRefs.current.forEach((el) => {
        if (el) gsap.set(el, { opacity: 0 })
      })

      const tl = gsap.timeline()
      const HOLD = 0.14
      const FADE = 0.08

      // ── Phase 0: heading centered → hold → slide to left column ──
      tl.to({}, { duration: 0.12 })  // hold centered
      tl.to(headingBlock, { x: 0, y: 0, duration: 0.14, ease: 'power2.inOut' })
      tl.to({}, { duration: 0.06 })  // brief hold at final position

      // ── Phase 1: right panel bg + first capability fade in (heading stays) ──
      tl.addLabel('intro-out')
      if (rightPanelRef.current) {
        tl.to(rightPanelRef.current, { opacity: 1, duration: FADE, ease: 'power2.inOut' }, 'intro-out')
      }
      if (desktopTextRefs.current[0]) {
        tl.to(desktopTextRefs.current[0]!, { opacity: 1, duration: FADE, ease: 'power2.inOut' }, 'intro-out')
      }
      if (desktopVisualRefs.current[0]) {
        tl.to(desktopVisualRefs.current[0]!, { opacity: 1, duration: FADE, ease: 'power2.inOut' }, 'intro-out')
      }
      tl.to({}, { duration: HOLD }) // first panel hold

      // ── Phase 2+: capability crossfades ──
      for (let i = 1; i < n; i++) {
        const label = `fade-${i}`
        tl.addLabel(label)
        if (desktopTextRefs.current[i - 1]) {
          tl.to(desktopTextRefs.current[i - 1]!, { opacity: 0, duration: FADE, ease: 'power2.inOut' }, label)
        }
        if (desktopVisualRefs.current[i - 1]) {
          tl.to(desktopVisualRefs.current[i - 1]!, { opacity: 0, duration: FADE, ease: 'power2.inOut' }, label)
        }
        if (desktopTextRefs.current[i]) {
          tl.to(desktopTextRefs.current[i]!, { opacity: 1, duration: FADE, ease: 'power2.inOut' }, label)
        }
        if (desktopVisualRefs.current[i]) {
          tl.to(desktopVisualRefs.current[i]!, { opacity: 1, duration: FADE, ease: 'power2.inOut' }, label)
        }
        tl.to({}, { duration: HOLD })
      }

      ScrollTrigger.create({
        trigger: wrapperRef.current,
        pin: true,
        scrub: 1.5,
        start: 'top top',
        end: `+=${(n + 1) * 380}`,  // extra scroll distance for heading phase
        animation: tl,
        onUpdate: (self) => {
          // Map progress to capability index (skip the heading intro phase)
          const capProgress = Math.max(0, (self.progress * tl.totalDuration() - 0.32) / (tl.totalDuration() - 0.32))
          const progressIndex = Math.min(Math.floor(capProgress * n), n - 1)
          const limeColor = getComputedStyle(document.documentElement).getPropertyValue('--color-brand-lime').trim() || '#abff02'
          const dots = wrapperRef.current?.querySelectorAll('[data-dot]')
          dots?.forEach((dot, i) => {
            const el = dot as HTMLElement
            const isActive = i === progressIndex && capProgress > 0
            el.style.backgroundColor = isActive ? limeColor : 'rgba(0,0,0,0.12)'
            el.style.height = isActive ? '20px' : '6px'
            el.style.width = isActive ? '3px' : '1.5px'
          })
        },
      })
    })

    // ── Mobile: hide nav while capabilities section is pinned ──
    mm.add('(max-width: 1023px)', () => {
      const nav = document.getElementById('main-nav')
      if (nav && mobileWrapperRef.current) {
        ScrollTrigger.create({
          trigger: mobileWrapperRef.current,
          start: 'top top',
          end: 'bottom top',
          onEnter: () => gsap.to(nav, { opacity: 0, y: -12, duration: 0.25, ease: 'power2.in' }),
          onLeave: () => gsap.to(nav, { opacity: 1, y: 0, duration: 0.25, ease: 'power2.out' }),
          onEnterBack: () => gsap.to(nav, { opacity: 0, y: -12, duration: 0.25, ease: 'power2.in' }),
          onLeaveBack: () => gsap.to(nav, { opacity: 1, y: 0, duration: 0.25, ease: 'power2.out' }),
        })
      }
    })

    // ── Mobile: scroll-pinned crossfade (mirrors desktop) ──
    mm.add('(max-width: 1023px)', () => {
      const n = CAPABILITIES.length
      if (!mobileWrapperRef.current) return

      // All panels hidden except first
      mobilePanelRefs.current.forEach((el, i) => {
        if (el) gsap.set(el, { opacity: i === 0 ? 1 : 0 })
      })

      const tl = gsap.timeline()
      const HOLD = 0.15
      const FADE = 0.08

      tl.to({}, { duration: HOLD }) // hold first panel

      for (let i = 1; i < n; i++) {
        const label = `mobile-fade-${i}`
        tl.addLabel(label)
        if (mobilePanelRefs.current[i - 1]) {
          tl.to(mobilePanelRefs.current[i - 1]!, { opacity: 0, duration: FADE, ease: 'power2.inOut' }, label)
        }
        if (mobilePanelRefs.current[i]) {
          tl.to(mobilePanelRefs.current[i]!, { opacity: 1, duration: FADE, ease: 'power2.inOut' }, label)
        }
        tl.to({}, { duration: HOLD })
      }

      ScrollTrigger.create({
        trigger: mobileWrapperRef.current,
        pin: true,
        scrub: 1.5,
        start: 'top top',
        end: `+=${n * 420}`,
        animation: tl,
        onUpdate: (self) => {
          const progress = self.progress * tl.totalDuration()
          const stepSize = tl.totalDuration() / n
          const activeIndex = Math.min(Math.floor(progress / stepSize), n - 1)
          const limeColor = getComputedStyle(document.documentElement).getPropertyValue('--color-brand-lime').trim() || '#abff02'
          const dots = mobileWrapperRef.current?.querySelectorAll('[data-mobile-dot]')
          dots?.forEach((dot, i) => {
            const el = dot as HTMLElement
            el.style.backgroundColor = i === activeIndex ? limeColor : 'rgba(0,0,0,0.15)'
            el.style.width = i === activeIndex ? '20px' : '6px'
            el.style.height = i === activeIndex ? '3px' : '1.5px'
          })
        },
      })
    })

  }, { scope: sectionRef })

  return (
    <section ref={sectionRef} id="capabilities" className="relative bg-white" data-scrub-section>

      {/* ═══════════════════════════════════════
          DESKTOP: Scroll-pinned crossfade
          ═══════════════════════════════════════ */}
      <div
        ref={wrapperRef}
        className="hidden lg:flex relative"
        style={{ height: '100vh', overflow: 'hidden' }}
      >
        {/* Desktop section header — starts centered, animates to left column */}
        <div
          ref={desktopHeadingBlockRef}
          className="absolute top-24 left-0 z-20 container-page"
          style={{ width: '45%' }}
        >
          <span className="font-mono text-label font-semibold tracking-[0.18em] text-lime-dark uppercase block mb-3">
            What can it do?
          </span>
          <h2 className="text-section-heading font-sans font-normal text-brand-dark tracking-tight leading-tight">
            Ask for your week, book the meeting, find the thread, send the reply. Claude does it in Outlook, not in a copy of it.
          </h2>
        </div>

        {/* Progress indicators — vertical strip on far left */}
        <div className="absolute left-6 top-1/2 -translate-y-1/2 z-30 flex flex-col items-center gap-3">
          {CAPABILITIES.map((cap) => (
            <div
              key={cap.id}
              data-dot
              className="w-1.5 rounded-full transition-all duration-300"
              style={{ backgroundColor: 'rgba(0,0,0,0.12)', height: 6 }}
            />
          ))}
        </div>

        <div className="absolute inset-0 flex">
          {/* Left panel: text — stacked absolutely */}
          <div className="w-[45%] relative flex items-center">
            <div className="container-page w-full">
              <div className="relative" style={{ minHeight: 320 }}>
                {CAPABILITIES.map((cap, i) => {
                  const verbs = 'domain' in cap ? capabilityVerbNames(cap.domain) : []
                  return (
                  <div
                    key={cap.id}
                    ref={(el) => { desktopTextRefs.current[i] = el }}
                    className="absolute inset-0"
                  >
                    <span className="font-mono text-label font-semibold tracking-[0.18em] text-lime-dark uppercase block mb-3">
                      {cap.id}
                    </span>
                    <h3 className="text-capability font-sans font-medium text-brand-dark tracking-tight mb-4">
                      {cap.label}
                      {verbs.length > 0 && (
                        <span className="text-sm text-gray-400 ml-2 font-normal">
                          {verbs.length} verbs
                        </span>
                      )}
                    </h3>
                    <p className="text-base text-gray-400 font-sans leading-relaxed mb-4 max-w-md">
                      {cap.summary}
                    </p>
                    <p className="text-sm text-gray-600 font-sans leading-relaxed max-w-md italic">
                      {cap.keyDetail}
                    </p>

                    {/* Verb list (operation values) for calendar/mail/account */}
                    {verbs.length > 0 && (
                      <div className="mt-5 flex flex-wrap gap-2">
                        {verbs.map((verb) => (
                          <span
                            key={verb}
                            className="inline-block bg-brand-off-white text-brand-dark font-mono text-[10px] tracking-wider px-2 py-1 rounded"
                          >
                            {verb}
                          </span>
                        ))}
                      </div>
                    )}

                    {/* Bullets for privacy */}
                    {'bullets' in cap && (
                      <ul className="mt-4 space-y-2">
                        {cap.bullets.map((bullet) => (
                          <li key={bullet} className="flex items-start gap-2 text-sm text-gray-400">
                            <span className="w-1 h-1 rounded-full bg-lime-dark mt-2 shrink-0" />
                            {bullet}
                          </li>
                        ))}
                      </ul>
                    )}

                    {/* Auth methods */}
                    {'authMethods' in cap && (
                      <div className="mt-4 space-y-3">
                        {cap.authMethods.map((method) => (
                          <div key={method.name}>
                            <span className="font-mono text-[10px] tracking-wider text-lime-dark uppercase font-semibold">
                              {method.name}
                            </span>
                            <p className="text-sm text-gray-400 mt-0.5">{method.desc}</p>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                  )
                })}
              </div>
            </div>
          </div>

          {/* Right panel: visual placeholders — stacked absolutely */}
          <div ref={rightPanelRef} className="w-[55%] relative bg-gray-900">
            {CAPABILITIES.map((cap, i) => (
              <div
                key={cap.id}
                ref={(el) => { desktopVisualRefs.current[i] = el }}
                className="absolute inset-0 flex items-center justify-center p-10"
              >
                <CapabilityVisualPlaceholder index={i} label={cap.label} />
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* ═══════════════════════════════════════
          MOBILE: Scroll-pinned crossfade
          ═══════════════════════════════════════ */}
      <div
        ref={mobileWrapperRef}
        className="lg:hidden relative"
        style={{ height: '100vh', overflow: 'hidden' }}
      >
        {/* Persistent section header — always visible at top */}
        <div className="absolute top-0 left-0 right-0 z-20 px-5 pt-6 pb-4 bg-white">
          <span
            ref={eyebrowRef}
            className="font-mono text-label font-semibold tracking-[0.18em] text-lime-dark uppercase block mb-1"
          >
            What can it do?
          </span>
          <h2
            ref={headingRef}
            className="text-[1.1rem] font-sans font-normal text-brand-dark tracking-tight leading-tight"
          >
            Ask for your week, book the meeting, find the thread, send the reply. Claude does it in Outlook, not in a copy of it.
          </h2>
        </div>

        {/* Progress dots — horizontal strip at bottom */}
        <div className="absolute bottom-6 left-1/2 -translate-x-1/2 z-30 flex items-center gap-3">
          {CAPABILITIES.map((cap) => (
            <div
              key={cap.id}
              data-mobile-dot
              className="rounded-full transition-all duration-300"
              style={{ backgroundColor: 'rgba(0,0,0,0.15)', width: 6, height: 1.5 }}
            />
          ))}
        </div>

        {/* Panels — stacked absolutely below the header, crossfade via timeline */}
        {CAPABILITIES.map((cap, i) => {
          const verbs = 'domain' in cap ? capabilityVerbNames(cap.domain) : []
          return (
          <div
            key={cap.id}
            ref={(el) => { mobilePanelRefs.current[i] = el }}
            className="absolute bottom-0 left-0 right-0 flex flex-col"
            style={{ top: 84 }}
          >
            {/* Visual — top half */}
            <div className="flex-none bg-gray-900 flex items-center justify-center p-6" style={{ height: '45%' }}>
              <CapabilityVisualPlaceholder index={i} label={cap.label} />
            </div>

            {/* Text — bottom half */}
            <div className="flex-1 overflow-hidden px-5 pt-5 pb-16 bg-white">
              <span className="font-mono text-label font-semibold tracking-[0.18em] text-lime-dark uppercase block mb-2">
                {cap.id}
              </span>
              <h3 className="text-lg font-sans font-medium text-brand-dark tracking-tight mb-3">
                {cap.label}
                {verbs.length > 0 && (
                  <span className="text-sm text-gray-400 ml-2 font-normal">
                    {verbs.length} verbs
                  </span>
                )}
              </h3>
              <p className="text-sm text-gray-400 font-sans leading-relaxed mb-3">
                {cap.summary}
              </p>
              <p className="text-xs text-gray-600 font-sans leading-relaxed italic mb-3">
                {cap.keyDetail}
              </p>

              {verbs.length > 0 && (
                <div className="flex flex-wrap gap-1.5">
                  {verbs.map((verb) => (
                    <span
                      key={verb}
                      className="inline-block bg-brand-off-white text-brand-dark font-mono text-[9px] tracking-wider px-1.5 py-0.5 rounded"
                    >
                      {verb}
                    </span>
                  ))}
                </div>
              )}

              {'bullets' in cap && (
                <ul className="space-y-1.5">
                  {cap.bullets.map((bullet) => (
                    <li key={bullet} className="flex items-start gap-2 text-xs text-gray-400">
                      <span className="w-1 h-1 rounded-full bg-lime-dark mt-1.5 shrink-0" />
                      {bullet}
                    </li>
                  ))}
                </ul>
              )}

              {'authMethods' in cap && (
                <div className="space-y-2.5">
                  {cap.authMethods.map((method) => (
                    <div key={method.name}>
                      <span className="font-mono text-[10px] tracking-wider text-lime-dark uppercase font-semibold">
                        {method.name}
                      </span>
                      <p className="text-xs text-gray-400 mt-0.5">{method.desc}</p>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
          )
        })}
      </div>
    </section>
  )
}

/* ── Visual placeholder per capability ── */
function CapabilityVisualPlaceholder({ index, label }: { index: number; label: string }) {
  // Calendar Management capability (index 0) — real SVG asset
  if (index === 0) {
    return <CapabilityCalendar className="w-full h-full" />
  }

  // Multi-Account capability (index 1) — real SVG asset
  if (index === 1) {
    return <CapabilityMultiAccount className="w-full h-full" />
  }

  // Mail Search capability (index 2) — real SVG asset
  if (index === 2) {
    return <CapabilityMailSearch className="w-full h-full" />
  }

  // Privacy & Security capability (index 3) — real SVG asset
  if (index === 3) {
    return <CapabilityPrivacyDiagram className="w-full h-full" />
  }

  // Zero-Config Auth capability (index 4) — real SVG asset
  if (index === 4) {
    return <CapabilityAuthFlow className="w-full h-full" />
  }

  const assetNames = [
    'CapabilityCalendar.tsx',
    'CapabilityMultiAccount.tsx',
    'CapabilityMailSearch.tsx',
    'CapabilityPrivacyDiagram.tsx',
    'CapabilityAuthFlow.tsx',
  ] as const

  const assetName = assetNames[index] ?? 'Unknown'

  return (
    <div className="w-full h-full rounded-lg border border-white/10 bg-gray-900/50 flex flex-col items-center justify-center gap-3 p-6">
      {/* ASSET: {assetName} */}
      <div className="w-16 h-16 rounded-full border border-brand-lime/30 flex items-center justify-center">
        <span className="font-mono text-brand-lime text-lg font-semibold">
          {String(index + 1).padStart(2, '0')}
        </span>
      </div>
      <span className="font-mono text-label tracking-[0.18em] text-white/60 uppercase text-center">
        {label}
      </span>
      <span className="font-mono text-[9px] tracking-wider text-white/20 uppercase">
        Asset: {assetName}
      </span>
    </div>
  )
}
