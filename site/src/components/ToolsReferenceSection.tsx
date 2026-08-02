import { useState, useRef, useMemo, useCallback } from 'react'
import gsap from 'gsap'
import { useGSAP } from '@gsap/react'
import { domains, domainCount, fullVerbCount, defaultVerbCount } from '../surface'

/**
 * A rendered verb: one `operation` value of an aggregate domain tool. Derived from the
 * generated surface manifest, never transcribed.
 */
interface VerbRow {
  name: string
  description: string
  gate: string | null
}

/** A rendered category: one aggregate domain tool and the verbs it dispatches. */
interface DomainCategory {
  id: string
  label: string
  verbs: VerbRow[]
}

/**
 * TOOL_CATEGORIES is the tools reference, derived from the surface manifest so the site
 * states no verb name or count of its own. Each category is one aggregate domain tool; its
 * rows are the `operation` values that tool dispatches, not flat top-level tool names.
 */
const TOOL_CATEGORIES: DomainCategory[] = domains.map((domain) => ({
  id: domain.name,
  label: domain.name,
  verbs: domain.verbs.map((verb) => ({
    name: verb.name,
    description: verb.summary,
    gate: verb.gate,
  })),
}))

export default function ToolsReferenceSection() {
  const sectionRef = useRef<HTMLElement>(null)
  const [isOpen, setIsOpen] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const [expandedCategories, setExpandedCategories] = useState<Set<string>>(new Set())
  const contentRef = useRef<HTMLDivElement>(null)
  const headingRef = useRef<HTMLButtonElement>(null)

  useGSAP(() => {
    const mm = gsap.matchMedia()
    mm.add('(prefers-reduced-motion: no-preference)', () => {
      if (headingRef.current) {
        gsap.from(headingRef.current, {
          opacity: 0, y: 20, duration: 0.5,
          ease: 'cubic-bezier(0, 0, 0.58, 1)',
          scrollTrigger: { trigger: headingRef.current, start: 'top 85%' },
        })
      }
    })
  }, { scope: sectionRef })

  /* ── Filter verbs by search query ── */
  const filteredCategories = useMemo(() => {
    if (!searchQuery.trim()) return TOOL_CATEGORIES
    const q = searchQuery.toLowerCase()
    return TOOL_CATEGORIES
      .map((cat) => ({
        ...cat,
        verbs: cat.verbs.filter(
          (t) => t.name.toLowerCase().includes(q) || t.description.toLowerCase().includes(q),
        ),
      }))
      .filter((cat) => cat.verbs.length > 0)
  }, [searchQuery])

  const totalVerbs = useMemo(
    () => filteredCategories.reduce((sum, cat) => sum + cat.verbs.length, 0),
    [filteredCategories],
  )

  const toggleCategory = useCallback((id: string) => {
    setExpandedCategories((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }, [])

  const toggleMain = useCallback(() => {
    setIsOpen((prev) => !prev)
    if (!isOpen) {
      // Expand all categories by default when opening
      setExpandedCategories(new Set(TOOL_CATEGORIES.map((c) => c.id)))
    }
  }, [isOpen])

  return (
    <section ref={sectionRef} id="tools-reference" className="relative bg-white py-12 sm:py-16">
      <div className="container-page">
        {/* ── Accordion trigger ── */}
        <button
          ref={headingRef}
          onClick={toggleMain}
          className={`w-full flex items-center justify-between py-5 border-b transition-colors duration-200 group ${
            isOpen ? 'border-brand-lime/30' : 'border-brand-dark/10'
          }`}
          aria-expanded={isOpen}
          aria-controls="tools-reference-content"
        >
          <span className="flex items-center gap-3">
            {isOpen && (
              <span className="w-0.5 h-6 bg-brand-lime rounded-full" />
            )}
            <span className="font-mono text-label font-semibold tracking-[0.18em] text-brand-dark uppercase">
              {fullVerbCount} Verbs Across {domainCount} Tools — View Full Reference
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
          <div ref={contentRef} id="tools-reference-content" className="pt-6 pb-2">
            {/* Surface note: each domain is one aggregate MCP tool; the rows below are its
                operation values. The verb counts are stated as the full surface and the
                default configuration so neither number is ambiguous. */}
            <p className="text-sm text-gray-400 font-sans mb-4">
              Each domain is one aggregate MCP tool. The rows are its{' '}
              <code className="font-mono text-xs bg-brand-off-white px-1.5 py-0.5 rounded text-brand-dark">
                operation
              </code>{' '}
              values. {fullVerbCount} verbs across {domainCount} tools with every gate open,{' '}
              {defaultVerbCount} in the default configuration.
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
                  placeholder="Search verbs..."
                  className="w-full pl-10 pr-4 py-2.5 rounded-lg border border-gray-100 bg-brand-off-white text-sm font-sans text-brand-dark placeholder:text-gray-200 focus:outline-none focus:border-brand-lime/50 focus:ring-1 focus:ring-brand-lime/20 transition-colors"
                  aria-label="Search verbs by name or description"
                />
              </div>
              <span className="mt-1.5 block text-xs text-gray-200 font-mono">
                {totalVerbs} verb{totalVerbs !== 1 ? 's' : ''} found
              </span>
            </div>

            {/* Category accordions */}
            <div className="space-y-1">
              {filteredCategories.map((category) => (
                <div key={category.id} className="border-b border-brand-dark/5">
                  <button
                    onClick={() => toggleCategory(category.id)}
                    className="w-full flex items-center justify-between py-3 text-left group"
                    aria-expanded={expandedCategories.has(category.id)}
                  >
                    <div className="flex items-center gap-3">
                      <span className="font-mono text-[10px] tracking-wider text-lime-dark uppercase font-semibold">
                        {category.label}
                      </span>
                      <span className="text-xs text-gray-200 font-mono">
                        {category.verbs.length}
                      </span>
                    </div>
                    <svg
                      width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                      strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"
                      className={`text-gray-400 transition-transform duration-200 ${
                        expandedCategories.has(category.id) ? 'rotate-180' : ''
                      }`}
                    >
                      <polyline points="6 9 12 15 18 9" />
                    </svg>
                  </button>

                  {expandedCategories.has(category.id) && (
                    <div className="pb-3">
                      <table className="w-full text-left">
                        <thead>
                          <tr>
                            <th className="pb-2 pr-4 font-mono text-[9px] tracking-wider text-lime-dark uppercase font-semibold w-48">
                              Operation
                            </th>
                            <th className="pb-2 font-mono text-[9px] tracking-wider text-lime-dark uppercase font-semibold">
                              Description
                            </th>
                          </tr>
                        </thead>
                        <tbody>
                          {category.verbs.map((verb, i) => (
                            <tr
                              key={verb.name}
                              className={i % 2 === 0 ? 'bg-brand-dark/[0.02]' : ''}
                            >
                              <td className="py-2 pr-4 font-mono text-sm text-brand-dark">
                                <span className="bg-brand-off-white px-1.5 py-0.5 rounded text-xs">
                                  {category.label} · {verb.name}
                                </span>
                              </td>
                              <td className="py-2 text-sm text-gray-400 font-sans">
                                {verb.description}
                                {verb.gate && (
                                  <span className="ml-2 inline-block font-mono text-[9px] tracking-wider text-lime-dark uppercase">
                                    gated by {verb.gate}
                                  </span>
                                )}
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  )}
                </div>
              ))}

              {filteredCategories.length === 0 && (
                <p className="py-8 text-center text-sm text-gray-400 font-sans">
                  No verbs match &ldquo;{searchQuery}&rdquo;
                </p>
              )}
            </div>
          </div>
        )}
      </div>
    </section>
  )
}
