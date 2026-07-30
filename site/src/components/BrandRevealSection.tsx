import { useRef } from 'react'
import gsap from 'gsap'
import { ScrollTrigger } from 'gsap/ScrollTrigger'
import { useGSAP } from '@gsap/react'
import DotGridOverlay from './DotGridOverlay'
import BrandRevealParticles from './three/BrandRevealParticles'
import NotchCornerMask from './NotchCornerMask'

export default function BrandRevealSection() {
  const sectionRef = useRef<HTMLElement>(null)
  const desktopWrapperRef = useRef<HTMLDivElement>(null)
  const desktopBgRef = useRef<HTMLDivElement>(null)
  const desktopPreLabelRef = useRef<HTMLSpanElement>(null)
  const desktopHeadingRef = useRef<HTMLHeadingElement>(null)
  const desktopDotGridRef = useRef<HTMLDivElement>(null)

  const mobileWrapperRef = useRef<HTMLDivElement>(null)
  const mobileBgRef = useRef<HTMLDivElement>(null)
  const mobilePreLabelRef = useRef<HTMLSpanElement>(null)
  const mobileHeadingRef = useRef<HTMLHeadingElement>(null)

  useGSAP(() => {
    const mm = gsap.matchMedia()

    /* ── Desktop: pinned scroll-driven transition ── */
    mm.add('(min-width: 1024px) and (prefers-reduced-motion: no-preference)', () => {
      // Pre-label fade in
      gsap.set(desktopPreLabelRef.current, { opacity: 0, y: 20 })
      gsap.set(desktopHeadingRef.current, { opacity: 0, scale: 1 })

      const tl = gsap.timeline({
        scrollTrigger: {
          trigger: desktopWrapperRef.current,
          pin: true,
          scrub: 1,
          start: 'top top',
          end: '+=1100',
        },
      })

      // Phase 1: Pre-label enters
      tl.to(desktopPreLabelRef.current, { opacity: 1, y: 0, duration: 0.1 })
      tl.to({}, { duration: 0.1 }) // hold

      // Phase 2: Main heading appears
      tl.to(desktopHeadingRef.current, { opacity: 1, duration: 0.15 })
      tl.to({}, { duration: 0.15 }) // hold

      // Phase 3: Background transitions dark → light, text color inverts
      tl.to(desktopBgRef.current, { backgroundColor: 'var(--color-brand-off-white)', duration: 0.3 }, 'transition')
      tl.to(desktopHeadingRef.current, { color: 'var(--color-brand-black)', duration: 0.3 }, 'transition')
      tl.to(desktopPreLabelRef.current, { color: 'var(--color-brand-black)', opacity: 0.5, duration: 0.3 }, 'transition')
      tl.to(desktopDotGridRef.current, { opacity: 0, duration: 0.2 }, 'transition')
      tl.to(desktopHeadingRef.current, { scale: 0.85, duration: 0.2 }, 'transition+=0.1')

      tl.to({}, { duration: 0.15 }) // hold at end
    })

    /* ── Desktop: reduced motion ── */
    mm.add('(min-width: 1024px) and (prefers-reduced-motion: reduce)', () => {
      gsap.set(desktopPreLabelRef.current, { opacity: 1, y: 0 })
      gsap.set(desktopHeadingRef.current, { opacity: 1 })
    })

    /* ── Mobile: IntersectionObserver-triggered animation ── */
    mm.add('(max-width: 1023px) and (prefers-reduced-motion: no-preference)', () => {
      gsap.set(mobilePreLabelRef.current, { opacity: 0, y: 20 })
      gsap.set(mobileHeadingRef.current, { opacity: 0, y: 30 })

      gsap.to(mobilePreLabelRef.current, {
        opacity: 1, y: 0, duration: 0.5,
        ease: 'cubic-bezier(0, 0, 0.58, 1)',
        scrollTrigger: { trigger: mobileWrapperRef.current, start: 'top 70%' },
      })
      gsap.to(mobileHeadingRef.current, {
        opacity: 1, y: 0, duration: 0.6, delay: 0.15,
        ease: 'cubic-bezier(0, 0, 0.58, 1)',
        scrollTrigger: { trigger: mobileWrapperRef.current, start: 'top 70%' },
      })

      // Background transition on mobile via ScrollTrigger
      gsap.to(mobileBgRef.current, {
        backgroundColor: 'var(--color-brand-off-white)', duration: 0.5,
        scrollTrigger: { trigger: mobileWrapperRef.current, start: 'top 40%', scrub: 1, end: 'bottom 60%' },
      })
      gsap.to(mobileHeadingRef.current, {
        color: 'var(--color-brand-black)', duration: 0.5,
        scrollTrigger: { trigger: mobileWrapperRef.current, start: 'top 40%', scrub: 1, end: 'bottom 60%' },
      })
    })

    mm.add('(max-width: 1023px) and (prefers-reduced-motion: reduce)', () => {
      gsap.set(mobilePreLabelRef.current, { opacity: 1, y: 0 })
      gsap.set(mobileHeadingRef.current, { opacity: 1, y: 0 })
    })
  }, { scope: sectionRef })

  /* Suppress unused import lint */
  void ScrollTrigger

  return (
    <section ref={sectionRef} id="brand-reveal" className="relative" data-scrub-section>
      {/* ═══ DESKTOP ═══ */}
      <div
        ref={desktopWrapperRef}
        className="hidden lg:flex relative items-center justify-center"
        style={{ height: '100vh', overflow: 'hidden' }}
      >
        {/* Background layer */}
        <div
          ref={desktopBgRef}
          className="absolute inset-0"
          style={{ backgroundColor: 'var(--color-brand-dark)' }}
        />

        {/* Dot grid */}
        <div ref={desktopDotGridRef} className="absolute inset-0 z-0">
          <DotGridOverlay />
        </div>

        {/* Three.js particle field — desktop only, behind dot-grid and text */}
        <div className="hidden lg:block absolute inset-0 z-0 pointer-events-none" aria-hidden="true">
          <BrandRevealParticles className="w-full h-full" active={true} />
        </div>

        {/* Content */}
        <div className="relative z-10 text-center">
          <span
            ref={desktopPreLabelRef}
            className="block text-lg font-sans font-normal text-white/50 mb-4"
          >
            That&rsquo;s
          </span>
          <h2
            ref={desktopHeadingRef}
            className="text-brand-reveal font-sans font-normal text-white tracking-tight leading-none"
          >
            Outlook Local MCP.
          </h2>
        </div>

        {/* Notch transition to next section */}
        <NotchCornerMask position="bottom" color="#ffffff" radius={60} />
      </div>

      {/* ═══ MOBILE ═══ */}
      <div
        ref={mobileWrapperRef}
        className="lg:hidden relative flex items-center justify-center py-32"
        style={{ minHeight: '60vh' }}
      >
        <div
          ref={mobileBgRef}
          className="absolute inset-0"
          style={{ backgroundColor: 'var(--color-brand-dark)' }}
        />
        <DotGridOverlay />

        <div className="relative z-10 text-center px-6">
          <span
            ref={mobilePreLabelRef}
            className="block text-base font-sans font-normal text-white/50 mb-3"
          >
            That&rsquo;s
          </span>
          <h2
            ref={mobileHeadingRef}
            className="text-[2.56rem] font-sans font-normal text-white tracking-tight leading-none"
          >
            Outlook Local MCP.
          </h2>
        </div>

        <NotchCornerMask position="bottom" color="#ffffff" radius={40} />
      </div>
    </section>
  )
}
