/**
 * index.md.mermaid.ts - Mermaid translations of the landing page's five SVG diagrams.
 *
 * The landing page draws its five capability and privacy diagrams as inline SVG, which
 * carries no meaning once flattened for a retrieval pipeline. CR-0070 FR-33 requires the
 * Markdown representation at /index.md to re-express each of them as a Mermaid fenced
 * code block, which a model can read, quote, and reason about.
 *
 * There is no automatic SVG-to-Mermaid conversion: each diagram is an authored
 * translation of what its SVG depicts, keyed by the `data-diagram` attribute its React
 * component stamps onto the root <svg>. The Markdown emitter looks the key up here and
 * substitutes the fence where the SVG sat.
 *
 * @agents-index Authored Mermaid translations of the five landing-page SVG diagrams, keyed by their data-diagram value.
 */

/**
 * DiagramMermaid pairs a diagram's caption with its Mermaid source.
 *
 * @property caption  A short label emitted as a bold line above the fence, so the
 *   diagram is titled in the Markdown.
 * @property mermaid  The Mermaid diagram body, without the surrounding fence markers.
 */
export interface DiagramMermaid {
  caption: string
  mermaid: string
}

/**
 * DIAGRAM_MERMAID maps each landing-page diagram key to its Mermaid translation. The
 * keys match the `data-diagram` attribute values on the SVG components under
 * src/components/svg/.
 */
export const DIAGRAM_MERMAID: Record<string, DiagramMermaid> = {
  calendar: {
    caption: 'Calendar management operations',
    mermaid: `flowchart LR
    Client["MCP client (Claude)"] --> Calendar["Calendar domain"]
    Calendar --> Read["List and get events"]
    Calendar --> Search["Search events and free/busy"]
    Calendar --> Write["Create and update events and meetings"]
    Calendar --> Manage["Respond, reschedule, cancel, delete"]`,
  },
  'multi-account': {
    caption: 'Multi-account isolation',
    mermaid: `flowchart TD
    Work["work@company.com (token A)"] --> Server["MCP server"]
    Personal["personal@outlook.com (token B)"] --> Server
    Team["team@org.com (token C)"] --> Server
    Server --> Keychain["OS keychain (isolated token storage per account)"]`,
  },
  'mail-search': {
    caption: 'Opt-in mail access and search',
    mermaid: `flowchart LR
    Query["KQL search, for example from:alice AND capacity plan"] --> Mail["Mail domain (read only)"]
    Mail --> Folders["List folders"]
    Mail --> Messages["List and get messages"]
    Mail --> Results["Full-text search results"]`,
  },
  privacy: {
    caption: 'Local privacy boundary',
    mermaid: `flowchart TD
    subgraph Machine["Your machine"]
        MCPClient["MCP client (Claude)"] <--> MCPServer["MCP server (outlook-local-mcp)"]
        MCPServer --> Keychain["OS keychain"]
    end
    MCPServer -->|"outbound only"| Graph["Microsoft Graph API"]
    MCPServer -.->|"blocked"| ThirdParty["Third-party services"]`,
  },
  'auth-flow': {
    caption: 'Zero-config authentication',
    mermaid: `flowchart TD
    Call["First tool call"] --> Method{"Auth method"}
    Method --> Device["Device code (default): show URL and code"]
    Method --> Browser["Interactive browser: localhost callback"]
    Method --> PKCE["Authorization code (PKCE): headless and remote"]
    Device --> Cached["Token cached in OS keychain"]
    Browser --> Cached
    PKCE --> Cached
    Cached --> Refresh["Silent refresh, about 90 day expiry, no Entra ID required"]`,
  },
}

/**
 * renderDiagramMarkdown renders one diagram key as a captioned Mermaid fence.
 *
 * @param key  The data-diagram key.
 * @returns The Markdown for the caption and fenced Mermaid block, or an empty string if
 *   the key is unknown (so an unrecognised marker is dropped rather than crashing).
 */
export function renderDiagramMarkdown(key: string): string {
  const entry = DIAGRAM_MERMAID[key]
  if (!entry) return ''
  return `**${entry.caption}**\n\n\`\`\`mermaid\n${entry.mermaid}\n\`\`\``
}
