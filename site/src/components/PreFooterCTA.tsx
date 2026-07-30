import { useRef } from 'react'
import gsap from 'gsap'
import { useGSAP } from '@gsap/react'
import DotGridOverlay from './DotGridOverlay'
import NotchCornerMask from './NotchCornerMask'

export default function PreFooterCTA() {
  const sectionRef = useRef<HTMLElement>(null)
  const headingRef = useRef<HTMLHeadingElement>(null)
  const ctaRef = useRef<HTMLAnchorElement>(null)
  const linkRef = useRef<HTMLAnchorElement>(null)

  useGSAP(() => {
    const mm = gsap.matchMedia()
    mm.add('(prefers-reduced-motion: no-preference)', () => {
      gsap.from(headingRef.current, {
        opacity: 0, y: 30, duration: 0.6,
        ease: 'cubic-bezier(0, 0, 0.58, 1)',
        scrollTrigger: { trigger: headingRef.current, start: 'top 85%' },
      })
      gsap.from(ctaRef.current, {
        opacity: 0, y: 20, duration: 0.5, delay: 0.2,
        ease: 'cubic-bezier(0, 0, 0.58, 1)',
        scrollTrigger: { trigger: headingRef.current, start: 'top 85%' },
      })
      gsap.from(linkRef.current, {
        opacity: 0, y: 15, duration: 0.4, delay: 0.35,
        ease: 'cubic-bezier(0, 0, 0.58, 1)',
        scrollTrigger: { trigger: headingRef.current, start: 'top 85%' },
      })
    })
  }, { scope: sectionRef })

  return (
    <section
      ref={sectionRef}
      id="pre-footer-cta"
      className="relative flex items-center justify-center"
      style={{ height: '100vh', backgroundColor: 'var(--color-brand-dark)' }}
    >
      <DotGridOverlay />
      <NotchCornerMask position="top" color="#ffffff" radius={60} />

      <div className="relative z-10 flex flex-col items-center text-center px-6">
        <h2
          ref={headingRef}
          className="text-[1.75rem] sm:text-[2.25rem] lg:text-display font-sans font-normal text-white tracking-tight leading-tight max-w-2xl"
        >
          Your AI assistant just got a upgraded.
        </h2>

        {/* Frosted-glass CTA button */}
        <a
          ref={ctaRef}
          href="#getting-started"
          className="mt-10 inline-flex items-center justify-center font-mono text-label font-semibold tracking-[0.18em] text-white uppercase rounded-lg px-12 py-6 sm:px-[50px] sm:py-[25px] hover:bg-white/25 transition-all duration-200 w-full sm:w-auto max-w-xs sm:max-w-none"
          style={{
            background: 'rgba(255,255,255,0.15)',
            backdropFilter: 'blur(27px)',
            WebkitBackdropFilter: 'blur(27px)',
            borderRadius: '8px',
          }}
          onClick={(e) => {
            e.preventDefault()
            document.querySelector('#getting-started')?.scrollIntoView({ behavior: 'smooth' })
          }}
        >
          Install Now
        </a>

        {/* Secondary link */}
        <a
          ref={linkRef}
          href="https://github.com/desek/outlook-local-mcp"
          target="_blank"
          rel="noopener"
          className="mt-5 font-mono text-label font-semibold tracking-[0.18em] text-brand-lime uppercase hover:text-white transition-colors duration-200"
        >
          View on GitHub <span>&#8599;</span>
        </a>
      </div>
    </section>
  )
}
