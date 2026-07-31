import { useState, useRef, useCallback, useEffect } from 'react'
import gsap from 'gsap'
import { useGSAP } from '@gsap/react'

const NAV_LINKS = [
  { label: 'Features', href: '#capabilities' },
  { label: 'Install', href: '#getting-started' },
  { label: 'Reference', href: '#tools-reference' },
  { label: 'GitHub', href: 'https://github.com/desek/outlook-local-mcp', external: true },
] as const

export default function Navigation() {
  const [mobileOpen, setMobileOpen] = useState(false)
  const navRef = useRef<HTMLElement>(null)
  const drawerRef = useRef<HTMLDivElement>(null)
  const hamburgerRef = useRef<HTMLButtonElement>(null)

  /* ── Entrance animation ── */
  useGSAP(() => {
    gsap.from(navRef.current, {
      y: -20,
      opacity: 0,
      duration: 0.3,
      ease: 'cubic-bezier(0, 0, 0.58, 1)',
      delay: 0.1,
    })
  }, { scope: navRef })

  /* ── Smooth scroll handler ── */
  const handleNavClick = useCallback(
    (e: React.MouseEvent<HTMLAnchorElement>, href: string, external?: boolean) => {
      if (external) return
      e.preventDefault()
      const target = document.querySelector(href)
      if (target) {
        target.scrollIntoView({ behavior: 'smooth' })
      }
      setMobileOpen(false)
    },
    [],
  )

  /* ── Close drawer on Escape ── */
  useEffect(() => {
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && mobileOpen) {
        setMobileOpen(false)
        hamburgerRef.current?.focus()
      }
    }
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [mobileOpen])

  /* ── Lock body scroll when drawer open ── */
  useEffect(() => {
    document.body.style.overflow = mobileOpen ? 'hidden' : ''
    return () => { document.body.style.overflow = '' }
  }, [mobileOpen])

  /* ── Drawer animation ── */
  useGSAP(() => {
    if (!drawerRef.current) return
    if (mobileOpen) {
      gsap.fromTo(drawerRef.current,
        { opacity: 0, y: -20 },
        { opacity: 1, y: 0, duration: 0.25, ease: 'power2.out' },
      )
    }
  }, { dependencies: [mobileOpen] })

  return (
    <>
      <nav
        ref={navRef}
        id="main-nav"
        className="fixed top-4 left-0 right-0 z-50 flex justify-center px-5 lg:px-0"
        role="navigation"
        aria-label="Main navigation"
      >
        <div
          className="nav-pill flex items-center justify-between w-full lg:w-[960px] px-5 lg:px-6"
          style={{ height: 64 }}
        >
          {/* ── Wordmark ── */}
          <a
            href="#"
            className="font-sans text-sm font-medium text-white tracking-wide whitespace-nowrap"
            onClick={(e) => {
              e.preventDefault()
              window.scrollTo({ top: 0, behavior: 'smooth' })
            }}
          >
            outlook-local-mcp
          </a>

          {/* ── Desktop links ── */}
          <div className="hidden lg:flex items-center gap-8">
            {NAV_LINKS.map((link) => (
              <a
                key={link.label}
                href={link.href}
                className="relative text-sm text-white/80 tracking-wide transition-colors duration-200 hover:text-brand-lime focus-visible:outline-2 focus-visible:outline-brand-lime focus-visible:outline-offset-4 group"
                onClick={(e) => handleNavClick(e, link.href, 'external' in link && link.external)}
                {...('external' in link && link.external ? { target: '_blank', rel: 'noopener' } : {})}
              >
                {link.label}
                {/* Lime dot on hover */}
                <span className="absolute -bottom-1.5 left-1/2 -translate-x-1/2 w-1 h-1 rounded-full bg-brand-lime opacity-0 group-hover:opacity-100 transition-opacity duration-200" />
              </a>
            ))}

            {/* CTA Button */}
            <a
              href="#getting-started"
              className="bg-white text-brand-dark rounded-lg px-8 py-3 font-mono text-label font-semibold tracking-[0.136em] uppercase hover:bg-brand-lime hover:text-brand-dark transition-colors duration-200"
              onClick={(e) => handleNavClick(e, '#getting-started')}
            >
              Install Now
            </a>
          </div>

          {/* ── Mobile hamburger ── */}
          <div className="flex lg:hidden items-center gap-3">
            <button
              ref={hamburgerRef}
              onClick={() => setMobileOpen((v) => !v)}
              className="w-11 h-11 flex flex-col items-center justify-center gap-1.5 text-white"
              aria-expanded={mobileOpen}
              aria-controls="mobile-nav-drawer"
              aria-label={mobileOpen ? 'Close menu' : 'Open menu'}
            >
              <span
                className={`block w-6 h-0.5 bg-current transition-transform duration-200 ${mobileOpen ? 'translate-y-1 rotate-45' : ''}`}
              />
              <span
                className={`block w-6 h-0.5 bg-current transition-opacity duration-200 ${mobileOpen ? 'opacity-0' : ''}`}
              />
              <span
                className={`block w-6 h-0.5 bg-current transition-transform duration-200 ${mobileOpen ? '-translate-y-1 -rotate-45' : ''}`}
              />
            </button>
          </div>
        </div>
      </nav>

      {/* ── Mobile drawer ── */}
      {mobileOpen && (
        <div
          ref={drawerRef}
          id="mobile-nav-drawer"
          className="fixed inset-0 z-40 bg-black/95 flex flex-col items-center justify-center gap-8 lg:hidden"
          role="dialog"
          aria-modal="true"
          aria-label="Mobile navigation"
        >
          {NAV_LINKS.map((link) => (
            <a
              key={link.label}
              href={link.href}
              className="text-2xl text-white font-sans font-medium tracking-wide hover:text-brand-lime transition-colors duration-200 focus-visible:outline-2 focus-visible:outline-brand-lime"
              onClick={(e) => handleNavClick(e, link.href, 'external' in link && link.external)}
              {...('external' in link && link.external ? { target: '_blank', rel: 'noopener' } : {})}
            >
              {link.label}
            </a>
          ))}
          <a
            href="#getting-started"
            className="mt-4 w-full max-w-xs bg-white text-brand-dark rounded-lg px-8 py-4 font-mono text-label font-semibold tracking-[0.136em] uppercase text-center hover:bg-brand-lime transition-colors duration-200"
            onClick={(e) => handleNavClick(e, '#getting-started')}
          >
            Install Now
          </a>
        </div>
      )}
    </>
  )
}
