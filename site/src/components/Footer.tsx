const PRODUCT_LINKS = [
  { label: 'Features', href: '#capabilities' },
  { label: 'Getting Started', href: '#getting-started' },
  { label: 'Tool Reference', href: '#tools-reference' },
  { label: 'Config Reference', href: '#config-reference' },
] as const

const DEVELOPER_LINKS = [
  { label: 'GitHub', href: 'https://github.com/desek/outlook-local-mcp', external: true },
  { label: 'Report an Issue', href: 'https://github.com/desek/outlook-local-mcp/issues', external: true },
  { label: 'Changelog', href: 'https://github.com/desek/outlook-local-mcp/releases', external: true },
] as const

export default function Footer() {
  const handleInternalClick = (e: React.MouseEvent<HTMLAnchorElement>, href: string) => {
    if (href.startsWith('#')) {
      e.preventDefault()
      document.querySelector(href)?.scrollIntoView({ behavior: 'smooth' })
    }
  }

  return (
    <footer className="relative py-16 sm:py-20 lg:py-24" style={{ backgroundColor: 'var(--color-brand-dark)' }}>
      <div className="container-page">
        {/* ── Main grid ── */}
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-10 lg:gap-8">
          {/* Column 1: Wordmark */}
          <div className="col-span-2 lg:col-span-1">
            <a
              href="#"
              className="text-white font-sans text-base font-medium tracking-wide"
              onClick={(e) => {
                e.preventDefault()
                window.scrollTo({ top: 0, behavior: 'smooth' })
              }}
            >
              outlook-local-mcp
            </a>
            <p className="mt-3 text-sm text-white/50 font-sans leading-relaxed max-w-xs">
              A Model Context Protocol server for Microsoft Calendar &amp; Mail.
              All data stays on your machine.
            </p>
            <div className="mt-4 inline-flex items-center gap-2 bg-white/[0.06] border border-white/10 rounded-md px-3 py-1.5">
              <span className="w-2 h-2 rounded-full bg-brand-lime" />
              <span className="font-mono text-[10px] tracking-wider text-white/60 uppercase">
                MIT License
              </span>
            </div>
          </div>

          {/* Column 2: Product */}
          <div>
            <h4 className="font-sans text-label font-semibold tracking-[0.18em] text-white uppercase mb-5">
              Product
            </h4>
            <ul className="space-y-3">
              {PRODUCT_LINKS.map((link) => (
                <li key={link.label}>
                  <a
                    href={link.href}
                    className="text-sm text-white/50 font-sans hover:text-brand-lime transition-colors duration-200"
                    onClick={(e) => handleInternalClick(e, link.href)}
                  >
                    {link.label}
                  </a>
                </li>
              ))}
            </ul>
          </div>

          {/* Column 3: Developer */}
          <div>
            <h4 className="font-sans text-label font-semibold tracking-[0.18em] text-white uppercase mb-5">
              Developer
            </h4>
            <ul className="space-y-3">
              {DEVELOPER_LINKS.map((link) => (
                <li key={link.label}>
                  <a
                    href={link.href}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-sm text-white/50 font-sans hover:text-brand-lime transition-colors duration-200"
                  >
                    {link.label}
                    <span className="ml-1 text-xs">&#8599;</span>
                  </a>
                </li>
              ))}
            </ul>
          </div>

          {/* Column 4: Reach Us */}
          <div className="col-span-2 lg:col-span-1">
            <h4 className="font-sans text-label font-semibold tracking-[0.18em] text-white uppercase mb-5">
              Built For
            </h4>
            <p className="text-sm text-white/50 font-sans leading-relaxed mb-5">
              Built for Claude, works with any MCP client.
              Connect your AI assistant to Microsoft Calendar and Mail
              without leaving your local machine.
            </p>

            {/* GitHub button */}
            <a
              href="https://github.com/desek/outlook-local-mcp"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-2 bg-white/[0.08] border border-white/10 rounded-lg px-4 py-2.5 hover:border-brand-lime/30 hover:bg-white/[0.12] transition-all duration-200 group"
            >
              {/* GitHub icon */}
              <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor" className="text-white/60 group-hover:text-white transition-colors">
                <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
              </svg>
              <span className="font-mono text-label font-semibold tracking-[0.136em] text-white/70 uppercase group-hover:text-white transition-colors">
                Star on GitHub
              </span>
            </a>

            {/* Social icons */}
            <div className="mt-5 flex gap-3">
              <a
                href="https://github.com/desek/outlook-local-mcp"
                target="_blank"
                rel="noopener noreferrer"
                className="w-9 h-9 flex items-center justify-center rounded-md bg-white/[0.06] text-white/40 hover:text-brand-lime hover:bg-white/[0.1] transition-all duration-200"
                aria-label="GitHub"
              >
                <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
                </svg>
              </a>
              <a
                href="https://x.com"
                target="_blank"
                rel="noopener noreferrer"
                className="w-9 h-9 flex items-center justify-center rounded-md bg-white/[0.06] text-white/40 hover:text-brand-lime hover:bg-white/[0.1] transition-all duration-200"
                aria-label="X (Twitter)"
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
                </svg>
              </a>
            </div>
          </div>
        </div>

        {/* ── Bottom bar ── */}
        <div className="mt-14 sm:mt-16 pt-6 border-t border-white/10 flex flex-col sm:flex-row items-center justify-between gap-4">
          <span className="text-xs text-white/30 font-sans">
            MIT License — outlook-local-mcp
          </span>

          {/* Platform badges */}
          <div className="flex items-center gap-1.5">
            {(['macOS', 'Linux', 'Windows', 'Docker'] as const).map((p) => (
              <span
                key={p}
                className="font-mono text-[9px] tracking-wider text-white/30 uppercase border border-white/10 rounded px-2 py-0.5"
              >
                {p}
              </span>
            ))}
          </div>

          <a
            href="https://github.com/desek/outlook-local-mcp"
            target="_blank"
            rel="noopener noreferrer"
            className="text-xs text-white/30 font-sans hover:text-brand-lime transition-colors duration-200"
          >
            github.com/desek/outlook-local-mcp
          </a>
        </div>
      </div>
    </footer>
  )
}
