import { useState, useRef, useCallback, useId } from 'react'
import gsap from 'gsap'
import { useGSAP } from '@gsap/react'
import { domainCount } from '../surface'

/**
 * tabDomId builds a stable, document-unique id for one tab or panel of a tab group.
 *
 * The ARIA tabs pattern requires each tab to name its panel through `aria-controls` and
 * each panel to name its tab through `aria-labelledby`, which means both need ids. Tab
 * labels are prose ("Go Install", "Claude Desktop"), so they are slugified rather than
 * used directly, and everything is scoped by the component instance's `useId` value so a
 * second instance of the section cannot collide with the first.
 *
 * @param scope The component instance scope, from `useId`.
 * @param group The tab group name, e.g. `install` or `config`.
 * @param part  The part being identified: a tab label, or `panel`.
 * @returns An id safe to use as an HTML id and in an IDREF attribute.
 */
function tabDomId(scope: string, group: string, part: string): string {
  return `${scope}${group}-${part.toLowerCase().replace(/[^a-z0-9]+/g, '-')}`
}

/* ── Install methods ── */
const INSTALL_TABS = ['Go Install', 'Docker', 'Claude Desktop', 'Build from Source'] as const
type InstallTab = (typeof INSTALL_TABS)[number]

const INSTALL_CONTENT: Record<InstallTab, { code?: string; language?: string; description?: string; isButton?: boolean }> = {
  'Go Install': {
    code: 'go install github.com/desek/outlook-local-mcp/cmd/outlook-local-mcp@latest',
    language: 'bash',
  },
  'Docker': {
    code: 'docker pull ghcr.io/desek/outlook-local-mcp:latest',
    language: 'bash',
  },
  'Claude Desktop': {
    description: 'Download the .mcpb extension from GitHub Releases. No terminal required.',
    isButton: true,
  },
  'Build from Source': {
    code: `git clone https://github.com/desek/outlook-local-mcp
cd outlook-local-mcp
go build ./cmd/outlook-local-mcp`,
    language: 'bash',
  },
}

/* ── Config targets ── */
const CONFIG_TABS = ['Claude Desktop', 'Claude Code'] as const
type ConfigTab = (typeof CONFIG_TABS)[number]

const CONFIG_CONTENT: Record<ConfigTab, string> = {
  'Claude Desktop': JSON.stringify({
    mcpServers: {
      'outlook-local': {
        command: 'outlook-local-mcp',
        env: { OUTLOOK_MCP_DEFAULT_TIMEZONE: 'America/New_York' },
      },
    },
  }, null, 2),
  'Claude Code': JSON.stringify({
    mcpServers: {
      'outlook-local': {
        command: '/path/to/outlook-local-mcp',
        env: { OUTLOOK_MCP_DEFAULT_TIMEZONE: 'America/New_York' },
      },
    },
  }, null, 2),
}

export default function GettingStartedSection() {
  const sectionRef = useRef<HTMLElement>(null)
  const headingRef = useRef<HTMLHeadingElement>(null)
  const eyebrowRef = useRef<HTMLSpanElement>(null)
  const [installTab, setInstallTab] = useState<InstallTab>('Go Install')
  const [configTab, setConfigTab] = useState<ConfigTab>('Claude Desktop')

  // Scope for the tab and panel ids that wire `aria-controls` to `aria-labelledby`.
  const tabScope = useId()
  const [copiedBlock, setCopiedBlock] = useState<string | null>(null)

  const handleCopy = useCallback(async (text: string, blockId: string) => {
    try {
      await navigator.clipboard.writeText(text)
      setCopiedBlock(blockId)
      setTimeout(() => setCopiedBlock(null), 2000)
    } catch {
      // fallback
    }
  }, [])

  /* ── Section heading entrance ── */
  useGSAP(() => {
    const mm = gsap.matchMedia()
    mm.add('(prefers-reduced-motion: no-preference)', () => {
      gsap.from(headingRef.current, {
        opacity: 0, y: 30, duration: 0.6,
        ease: 'cubic-bezier(0, 0, 0.58, 1)',
        scrollTrigger: { trigger: headingRef.current, start: 'top 85%' },
      })
      gsap.from(eyebrowRef.current, {
        opacity: 0, y: 20, duration: 0.5, delay: 0.15,
        ease: 'cubic-bezier(0, 0, 0.58, 1)',
        scrollTrigger: { trigger: headingRef.current, start: 'top 85%' },
      })
    })
  }, { scope: sectionRef })

  const installContent = INSTALL_CONTENT[installTab]
  const configContent = CONFIG_CONTENT[configTab]

  return (
    <section ref={sectionRef} id="getting-started" className="relative bg-white py-20 sm:py-28 lg:py-36">
      <div className="container-page">
        {/* ── Header ── */}
        <span
          ref={eyebrowRef}
          className="font-mono text-label font-semibold tracking-[0.18em] text-lime-dark uppercase block mb-3"
        >
          How do you get started?
        </span>
        <h2
          ref={headingRef}
          className="text-[1.75rem] sm:text-[2.25rem] lg:text-section-heading font-sans font-normal text-brand-dark tracking-tight leading-tight mb-16 sm:mb-20"
        >
          Install. Configure. Done.
        </h2>

        <div className="space-y-16 sm:space-y-20">
          {/* ═══════════════════════════════════
              STEP 1: INSTALL
              ═══════════════════════════════════ */}
          <div className="flex flex-col lg:flex-row lg:gap-16">
            <div className="lg:w-32 shrink-0 mb-4 lg:mb-0">
              <span className="font-mono text-label font-semibold tracking-[0.18em] text-lime-dark">
                01
              </span>
              <h3 className="text-lg font-sans font-normal text-brand-dark mt-1">
                Install
              </h3>
            </div>

            <div className="flex-1 max-w-2xl">
              {/* Tab strip */}
              <div className="flex flex-wrap gap-1 mb-4" role="tablist" aria-label="Install method">
                {INSTALL_TABS.map((tab) => (
                  <button
                    key={tab}
                    role="tab"
                    id={tabDomId(tabScope, 'install', tab)}
                    aria-selected={installTab === tab}
                    aria-controls={tabDomId(tabScope, 'install', 'panel')}
                    onClick={() => setInstallTab(tab)}
                    className={`px-3 py-1.5 font-mono text-[10px] sm:text-label tracking-wider uppercase rounded-md transition-colors duration-200 ${
                      installTab === tab
                        ? 'bg-brand-dark text-white'
                        : 'bg-brand-off-white text-gray-600 hover:bg-gray-100'
                    }`}
                  >
                    {tab}
                  </button>
                ))}
              </div>

              {/* Content panel */}
              <div
                className="relative"
                role="tabpanel"
                id={tabDomId(tabScope, 'install', 'panel')}
                aria-labelledby={tabDomId(tabScope, 'install', installTab)}
              >
                {installContent.code ? (
                  <CodeBlock
                    code={installContent.code}
                    blockId="install"
                    copied={copiedBlock === 'install'}
                    onCopy={handleCopy}
                  />
                ) : (
                  <div className="bg-brand-off-white rounded-lg p-6">
                    <p className="text-sm text-gray-600 font-sans leading-relaxed mb-4">
                      {installContent.description}
                    </p>
                    {installContent.isButton && (
                      <a
                        href="https://github.com/desek/outlook-local-mcp/releases"
                        target="_blank"
                        rel="noopener"
                        className="inline-flex items-center gap-2 bg-brand-dark text-white font-mono text-label font-semibold tracking-[0.136em] uppercase px-6 py-3 rounded-lg hover:bg-brand-lime hover:text-brand-dark transition-colors duration-200"
                      >
                        Download .mcpb
                        <span>&#8599;</span>
                      </a>
                    )}
                  </div>
                )}
              </div>
            </div>
          </div>

          {/* ═══════════════════════════════════
              STEP 2: CONFIGURE
              ═══════════════════════════════════ */}
          <div className="flex flex-col lg:flex-row lg:gap-16">
            <div className="lg:w-32 shrink-0 mb-4 lg:mb-0">
              <span className="font-mono text-label font-semibold tracking-[0.18em] text-lime-dark">
                02
              </span>
              <h3 className="text-lg font-sans font-normal text-brand-dark mt-1">
                Configure
              </h3>
            </div>

            <div className="flex-1 max-w-2xl">
              {/* Config sub-tabs */}
              <div className="flex gap-1 mb-4" role="tablist" aria-label="Configuration target">
                {CONFIG_TABS.map((tab) => (
                  <button
                    key={tab}
                    role="tab"
                    id={tabDomId(tabScope, 'config', tab)}
                    aria-selected={configTab === tab}
                    aria-controls={tabDomId(tabScope, 'config', 'panel')}
                    onClick={() => setConfigTab(tab)}
                    className={`px-3 py-1.5 font-mono text-[10px] sm:text-label tracking-wider uppercase rounded-md transition-colors duration-200 ${
                      configTab === tab
                        ? 'bg-brand-dark text-white'
                        : 'bg-brand-off-white text-gray-600 hover:bg-gray-100'
                    }`}
                  >
                    {tab}
                  </button>
                ))}
              </div>

              <div
                role="tabpanel"
                id={tabDomId(tabScope, 'config', 'panel')}
                aria-labelledby={tabDomId(tabScope, 'config', configTab)}
              >
                <CodeBlock
                  code={configContent}
                  blockId="config"
                  copied={copiedBlock === 'config'}
                  onCopy={handleCopy}
                  language="json"
                />
              </div>

              {/* Quick config callouts */}
              <div className="mt-6 grid grid-cols-2 gap-3">
                {[
                  { name: 'DEFAULT_TIMEZONE', hint: 'America/New_York' },
                  { name: 'AUTH_METHOD', hint: 'device | browser | authcode' },
                  { name: 'READ_ONLY', hint: 'true to disable writes' },
                  { name: 'MAIL_ENABLED', hint: 'true to enable mail tools' },
                ].map((v) => (
                  <div key={v.name} className="bg-brand-off-white rounded-md p-3">
                    <span className="font-mono text-[10px] tracking-wider text-lime-dark uppercase font-semibold block">
                      {v.name}
                    </span>
                    <span className="text-xs text-gray-400 font-sans">{v.hint}</span>
                  </div>
                ))}
              </div>
            </div>
          </div>

          {/* ═══════════════════════════════════
              STEP 3: FIRST RUN
              ═══════════════════════════════════ */}
          <div className="flex flex-col lg:flex-row lg:gap-16">
            <div className="lg:w-32 shrink-0 mb-4 lg:mb-0">
              <span className="font-mono text-label font-semibold tracking-[0.18em] text-lime-dark">
                03
              </span>
              <h3 className="text-lg font-sans font-normal text-brand-dark mt-1">
                First Run
              </h3>
            </div>

            <div className="flex-1 max-w-2xl">
              <p className="text-base text-gray-400 font-sans leading-relaxed mb-6">
                No credential setup before first use. On first tool call, a device code URL displays.
                Complete auth once in a browser. Tokens are cached in your OS keychain for ~90 days.
              </p>

              {/* Terminal mockup showing device code output */}
              <div className="code-block p-4 sm:p-5">
                <div className="flex items-center gap-2 mb-4">
                  <span className="w-3 h-3 rounded-full bg-[#ef5350]" />
                  <span className="w-3 h-3 rounded-full bg-[#ffb300]" />
                  <span className="w-3 h-3 rounded-full bg-[#66bb6a]" />
                  <span className="ml-2 text-[10px] text-white/50 font-mono">terminal</span>
                </div>
                <pre className="font-mono text-sm text-white/80 leading-relaxed overflow-x-auto">
                  <span className="text-gray-200">$</span> outlook-local-mcp{'\n'}
                  <span className="text-white/60">INFO  </span>MCP server starting on stdio...{'\n'}
                  <span className="text-white/60">INFO  </span>No accounts configured yet.{'\n'}
                  <span className="text-white/60">INFO  </span>Authentication required for first account.{'\n'}
                  {'\n'}
                  <span className="text-brand-lime">To sign in, use a web browser to open</span>{'\n'}
                  <span className="text-brand-lime">https://aka.ms/devicelogin</span>{'\n'}
                  <span className="text-brand-lime">and enter the code: </span>
                  <span className="text-white font-semibold">ABCD-EFGH</span>{'\n'}
                  {'\n'}
                  <span className="text-white/60">INFO  </span>Authentication successful.{'\n'}
                  <span className="text-white/60">INFO  </span>Token cached in OS keychain (~90 day expiry).{'\n'}
                  <span className="text-white/60">INFO  </span>
                  <span className="text-brand-lime">Ready.</span> {domainCount} tools registered.
                </pre>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}

/* ── Reusable code block with copy button ── */
function CodeBlock({
  code,
  blockId,
  copied,
  onCopy,
  language: _language,
}: {
  code: string
  blockId: string
  copied: boolean
  onCopy: (text: string, id: string) => void
  language?: string
}) {
  return (
    <div className="code-block relative group">
      <pre className="p-4 sm:p-5 font-mono text-sm text-white/90 leading-relaxed overflow-x-auto whitespace-pre-wrap">
        {code.split('\n').map((line, i) => (
          <span key={i} className="block">
            {line.startsWith('#') ? (
              <span className="text-brand-lime/60">{line}</span>
            ) : line.startsWith('{') || line.startsWith('}') || line.includes(':') ? (
              <span>
                {line.split(/(".*?")/g).map((part, j) =>
                  part.startsWith('"') ? (
                    <span key={j} className="text-white">{part}</span>
                  ) : (
                    <span key={j} className="text-white/70">{part}</span>
                  ),
                )}
              </span>
            ) : (
              <span className="text-white/90">{line}</span>
            )}
          </span>
        ))}
      </pre>

      {/* Copy button */}
      <button
        onClick={() => onCopy(code, blockId)}
        className="absolute top-3 right-3 w-9 h-9 flex items-center justify-center text-brand-lime/70 hover:text-brand-lime bg-black/30 rounded-md opacity-0 group-hover:opacity-100 transition-opacity duration-200"
        aria-label={`Copy ${blockId} code`}
      >
        {copied ? (
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <polyline points="20 6 9 17 4 12" />
          </svg>
        ) : (
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
            <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
          </svg>
        )}
      </button>
    </div>
  )
}
