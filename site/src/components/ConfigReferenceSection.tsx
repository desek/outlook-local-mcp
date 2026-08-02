import { useState, useRef, useMemo, useCallback } from 'react'
import gsap from 'gsap'
import { useGSAP } from '@gsap/react'
import { configVars, configVarCount } from '../surface'

/**
 * A rendered configuration variable, derived from the surface manifest.
 *
 * @property name  The full `OUTLOOK_MCP_` environment variable name (the copy target).
 * @property short  The name with the `OUTLOOK_MCP_` prefix stripped (the display label).
 * @property description  The one-line purpose of the variable.
 * @property defaultValue  The default value, or undefined when the variable has none.
 */
interface ConfigVar {
  name: string
  short: string
  description: string
  defaultValue?: string
}

/**
 * CONFIG_VARS is the configuration reference, derived from the surface manifest so the
 * site holds no variable name, default, or count of its own.
 */
const CONFIG_VARS: ConfigVar[] = configVars.map((v) => ({
  name: v.name,
  short: v.name.replace(/^OUTLOOK_MCP_/, ''),
  description: v.description,
  defaultValue: v.default || undefined,
}))

export default function ConfigReferenceSection() {
  const sectionRef = useRef<HTMLElement>(null)
  const [isOpen, setIsOpen] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const [expandedRows, setExpandedRows] = useState<Set<string>>(new Set())
  const [copiedVar, setCopiedVar] = useState<string | null>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)

  useGSAP(() => {
    const mm = gsap.matchMedia()
    mm.add('(prefers-reduced-motion: no-preference)', () => {
      if (triggerRef.current) {
        gsap.from(triggerRef.current, {
          opacity: 0, y: 20, duration: 0.5,
          ease: 'cubic-bezier(0, 0, 0.58, 1)',
          scrollTrigger: { trigger: triggerRef.current, start: 'top 85%' },
        })
      }
    })
  }, { scope: sectionRef })

  const filteredVars = useMemo(() => {
    if (!searchQuery.trim()) return CONFIG_VARS
    const q = searchQuery.toLowerCase()
    return CONFIG_VARS.filter(
      (v) =>
        v.short.toLowerCase().includes(q) ||
        v.name.toLowerCase().includes(q) ||
        v.description.toLowerCase().includes(q),
    )
  }, [searchQuery])

  const toggleRow = useCallback((variable: string) => {
    setExpandedRows((prev) => {
      const next = new Set(prev)
      if (next.has(variable)) {
        next.delete(variable)
      } else {
        next.add(variable)
      }
      return next
    })
  }, [])

  const handleCopyVar = useCallback(async (fullVar: string) => {
    try {
      await navigator.clipboard.writeText(fullVar)
      setCopiedVar(fullVar)
      setTimeout(() => setCopiedVar(null), 2000)
    } catch {
      // fallback
    }
  }, [])

  return (
    <section ref={sectionRef} id="config-reference" className="relative bg-white pb-12 sm:pb-16">
      <div className="container-page">
        {/* ── Accordion trigger ── */}
        <button
          ref={triggerRef}
          onClick={() => setIsOpen((v) => !v)}
          className={`w-full flex items-center justify-between py-5 border-b transition-colors duration-200 ${
            isOpen ? 'border-brand-lime/30' : 'border-brand-dark/10'
          }`}
          aria-expanded={isOpen}
          aria-controls="config-reference-content"
        >
          <span className="flex items-center gap-3">
            {isOpen && (
              <span className="w-0.5 h-6 bg-brand-lime rounded-full" />
            )}
            <span className="font-mono text-label font-semibold tracking-[0.18em] text-brand-dark uppercase">
              {configVarCount} Configuration Variables — View Full Reference
            </span>
          </span>
          <span className={`text-brand-dark/40 transition-transform duration-200 ${isOpen ? 'rotate-45' : ''}`}>
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <line x1="12" y1="5" x2="12" y2="19" />
              <line x1="5" y1="12" x2="19" y2="12" />
            </svg>
          </span>
        </button>

        {/* ── Expanded content ── */}
        {isOpen && (
          <div id="config-reference-content" className="pt-6 pb-2">
            {/* Prefix note */}
            <p className="text-sm text-gray-400 font-sans mb-4">
              All variables are prefixed with{' '}
              <code className="font-mono text-xs bg-brand-off-white px-1.5 py-0.5 rounded text-brand-dark">
                OUTLOOK_MCP_
              </code>
            </p>

            {/* Search bar */}
            <div className="mb-6 max-w-md">
              <div className="relative">
                <svg
                  width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                  strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"
                  className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400"
                >
                  <circle cx="11" cy="11" r="8" />
                  <line x1="21" y1="21" x2="16.65" y2="16.65" />
                </svg>
                <input
                  type="text"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  placeholder="Search variables..."
                  className="w-full pl-10 pr-4 py-2.5 rounded-lg border border-gray-100 bg-brand-off-white text-sm font-sans text-brand-dark placeholder:text-gray-200 focus:outline-none focus:border-brand-lime/50 focus:ring-1 focus:ring-brand-lime/20 transition-colors"
                  aria-label="Search configuration variables"
                />
              </div>
              <span className="mt-1.5 block text-xs text-gray-200 font-mono">
                {filteredVars.length} variable{filteredVars.length !== 1 ? 's' : ''} found
              </span>
            </div>

            {/* Variable table / list */}
            <div className="space-y-0.5">
              {filteredVars.map((v, i) => (
                <div
                  key={v.name}
                  className={`border-b border-brand-dark/5 ${i % 2 === 0 ? 'bg-brand-dark/[0.02]' : ''}`}
                >
                  <div className="flex items-center gap-3 py-2.5 px-3">
                    {/* Variable name with copy */}
                    <button
                      onClick={() => handleCopyVar(v.name)}
                      className="shrink-0 flex items-center gap-1.5 group"
                      aria-label={`Copy ${v.name}`}
                    >
                      <code className="font-mono text-xs text-brand-dark bg-brand-off-white px-1.5 py-0.5 rounded group-hover:bg-brand-lime/10 transition-colors">
                        {v.short}
                      </code>
                      {copiedVar === v.name ? (
                        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="var(--color-lime-dark)" strokeWidth="2" className="shrink-0">
                          <polyline points="20 6 9 17 4 12" />
                        </svg>
                      ) : (
                        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="shrink-0 text-gray-200 group-hover:text-lime-dark transition-colors">
                          <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
                          <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
                        </svg>
                      )}
                    </button>

                    {/* Description */}
                    <span className="flex-1 text-sm text-gray-400 font-sans truncate">
                      {v.description}
                    </span>

                    {/* Default value badge */}
                    {v.defaultValue && (
                      <span className="hidden sm:inline-block shrink-0 font-mono text-[9px] tracking-wider text-gray-200 bg-brand-off-white px-1.5 py-0.5 rounded uppercase">
                        default: {v.defaultValue}
                      </span>
                    )}

                    {/* Expand toggle */}
                    <button
                      onClick={() => toggleRow(v.name)}
                      className="shrink-0 w-6 h-6 flex items-center justify-center text-gray-400 hover:text-brand-dark transition-colors"
                      aria-expanded={expandedRows.has(v.name)}
                      aria-label={`${expandedRows.has(v.name) ? 'Collapse' : 'Expand'} ${v.short} details`}
                    >
                      <svg
                        width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                        strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"
                        className={`transition-transform duration-200 ${expandedRows.has(v.name) ? 'rotate-180' : ''}`}
                      >
                        <polyline points="6 9 12 15 18 9" />
                      </svg>
                    </button>
                  </div>

                  {/* Expanded detail */}
                  {expandedRows.has(v.name) && (
                    <div className="px-3 pb-3 pt-1">
                      <div className="bg-brand-off-white rounded-md p-3 text-sm text-gray-600 font-sans leading-relaxed">
                        <p className="mb-2">{v.description}</p>
                        <p className="font-mono text-xs text-gray-400">
                          Full env var:{' '}
                          <code className="text-brand-dark">{v.name}</code>
                        </p>
                        {v.defaultValue && (
                          <p className="font-mono text-xs text-gray-400 mt-1">
                            Default: <code className="text-brand-dark">{v.defaultValue}</code>
                          </p>
                        )}
                      </div>
                    </div>
                  )}
                </div>
              ))}

              {filteredVars.length === 0 && (
                <p className="py-8 text-center text-sm text-gray-400 font-sans">
                  No variables match &ldquo;{searchQuery}&rdquo;
                </p>
              )}
            </div>
          </div>
        )}
      </div>
    </section>
  )
}
