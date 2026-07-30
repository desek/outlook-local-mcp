import { useRef, useState, useCallback } from 'react'
import gsap from 'gsap'
import { useGSAP } from '@gsap/react'
import DotGridOverlay from './DotGridOverlay'
import NotchCornerMask from './NotchCornerMask'

const PLATFORMS = [
  { platform: 'macOS (Apple Silicon)', binary: 'arm64', docker: 'linux/arm64', icon: '🍎' },
  { platform: 'Linux (x86_64)', binary: 'amd64', docker: 'linux/amd64', icon: '🐧' },
  { platform: 'Linux (ARM)', binary: 'arm64', docker: 'linux/arm64', icon: '🐧' },
  { platform: 'Windows', binary: 'amd64', docker: null, icon: '🪟' },
] as const

const DOCKER_IMAGE = 'ghcr.io/desek/outlook-local-mcp:latest'

export default function PlatformSection() {
  const sectionRef = useRef<HTMLElement>(null)
  const headingRef = useRef<HTMLHeadingElement>(null)
  const eyebrowRef = useRef<HTMLSpanElement>(null)
  const desktopTableRef = useRef<HTMLDivElement>(null)
  const mobileTableRef = useRef<HTMLDivElement>(null)
  const [copiedDocker, setCopiedDocker] = useState(false)
  const [expandedMobile, setExpandedMobile] = useState<number | null>(null)

  const handleCopyDocker = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(`docker pull ${DOCKER_IMAGE}`)
      setCopiedDocker(true)
      setTimeout(() => setCopiedDocker(false), 2000)
    } catch {
      // fallback
    }
  }, [])

  useGSAP(() => {
    const mm = gsap.matchMedia()
    mm.add('(prefers-reduced-motion: no-preference)', () => {
      gsap.from(eyebrowRef.current, {
        opacity: 0, y: 20, duration: 0.5, delay: 0.15,
        ease: 'cubic-bezier(0, 0, 0.58, 1)',
        scrollTrigger: { trigger: eyebrowRef.current, start: 'top 85%' },
      })
      gsap.from(headingRef.current, {
        opacity: 0, y: 30, duration: 0.6,
        ease: 'cubic-bezier(0, 0, 0.58, 1)',
        scrollTrigger: { trigger: headingRef.current, start: 'top 85%' },
      })
      gsap.from(desktopTableRef.current, {
        opacity: 0, y: 20, duration: 0.5, delay: 0.3,
        ease: 'cubic-bezier(0, 0, 0.58, 1)',
        scrollTrigger: { trigger: desktopTableRef.current, start: 'top 85%' },
      })
    })
  }, { scope: sectionRef })

  return (
    <section ref={sectionRef} id="platforms" className="relative py-20 sm:py-28 lg:py-32" style={{ backgroundColor: 'var(--color-brand-dark)' }}>
      <DotGridOverlay />
      <NotchCornerMask position="top" color="#ffffff" radius={60} />

      <div className="container-page relative z-10">
        <span
          ref={eyebrowRef}
          className="font-mono text-label font-semibold tracking-[0.18em] text-brand-lime uppercase block mb-3"
        >
          Platform Support
        </span>
        <h2
          ref={headingRef}
          className="text-[1.25rem] sm:text-[1.5rem] lg:text-section-heading font-sans font-normal text-white tracking-tight leading-tight mb-10 sm:mb-14"
        >
          Run anywhere your AI assistant runs.
        </h2>

        {/* ── Desktop table ── */}
        <div ref={desktopTableRef} className="hidden sm:block">
          <div className="overflow-hidden rounded-lg border border-white/10">
            <table className="w-full text-left">
              <thead>
                <tr className="border-b border-white/10">
                  <th className="px-5 py-3 font-mono text-label font-semibold tracking-[0.18em] text-brand-lime uppercase">
                    Platform
                  </th>
                  <th className="px-5 py-3 font-mono text-label font-semibold tracking-[0.18em] text-brand-lime uppercase">
                    Binary
                  </th>
                  <th className="px-5 py-3 font-mono text-label font-semibold tracking-[0.18em] text-brand-lime uppercase">
                    Docker
                  </th>
                </tr>
              </thead>
              <tbody>
                {PLATFORMS.map((p, i) => (
                  <tr key={p.platform} className={`border-b border-white/5 ${i % 2 === 0 ? 'bg-white/[0.02]' : ''}`}>
                    <td className="px-5 py-3 text-sm text-white font-sans">
                      <span className="mr-2">{p.icon}</span>
                      {p.platform}
                    </td>
                    <td className="px-5 py-3 font-mono text-sm text-brand-lime">
                      {p.binary ?? <span className="text-white/20">—</span>}
                    </td>
                    <td className="px-5 py-3 font-mono text-sm text-brand-lime">
                      {p.docker ?? <span className="text-white/20">—</span>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>

        {/* ── Mobile cards ── */}
        <div ref={mobileTableRef} className="sm:hidden space-y-2">
          {PLATFORMS.map((p, i) => (
            <button
              key={p.platform}
              onClick={() => setExpandedMobile(expandedMobile === i ? null : i)}
              className="w-full text-left bg-white/[0.04] border border-white/10 rounded-lg px-4 py-3"
              aria-expanded={expandedMobile === i}
            >
              <div className="flex items-center justify-between">
                <span className="text-sm text-white font-sans">
                  <span className="mr-2">{p.icon}</span>
                  {p.platform}
                </span>
                <svg
                  width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                  strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"
                  className={`text-white/40 transition-transform duration-200 ${expandedMobile === i ? 'rotate-180' : ''}`}
                >
                  <polyline points="6 9 12 15 18 9" />
                </svg>
              </div>
              {expandedMobile === i && (
                <div className="mt-3 pt-3 border-t border-white/10 grid grid-cols-2 gap-3">
                  <div>
                    <span className="font-mono text-[9px] tracking-wider text-brand-lime uppercase block mb-1">Binary</span>
                    <span className="font-mono text-sm text-white">{p.binary ?? '—'}</span>
                  </div>
                  <div>
                    <span className="font-mono text-[9px] tracking-wider text-brand-lime uppercase block mb-1">Docker</span>
                    <span className="font-mono text-sm text-white">{p.docker ?? '—'}</span>
                  </div>
                </div>
              )}
            </button>
          ))}
        </div>

        {/* ── Docker image info ── */}
        <div className="mt-8 flex flex-col sm:flex-row items-start sm:items-center gap-3 sm:gap-4">
          <span className="font-mono text-label font-semibold tracking-[0.18em] text-white/50 uppercase">
            Docker Image
          </span>
          <button
            onClick={handleCopyDocker}
            className="flex items-center gap-2 bg-white/[0.06] border border-white/10 rounded-lg px-4 py-2 group hover:border-brand-lime/30 transition-colors duration-200"
            aria-label="Copy Docker pull command"
          >
            <code className="font-mono text-sm text-brand-lime">{DOCKER_IMAGE}</code>
            {copiedDocker ? (
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="text-brand-lime shrink-0">
                <polyline points="20 6 9 17 4 12" />
              </svg>
            ) : (
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="text-white/40 group-hover:text-brand-lime shrink-0">
                <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
                <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
              </svg>
            )}
          </button>
          <span className="text-xs text-white/40 font-sans">scratch-based, &lt; 20 MB</span>
        </div>
      </div>
    </section>
  )
}
