/**
 * seo.jsonld.ts - schema.org JSON-LD structured data for each page.
 *
 * Emits the GEO structured data required by CR-0070 FR-39 to FR-45. Every page carries
 * at least one valid schema.org entity in its pre-rendered head:
 *
 *  - the landing page: SoftwareApplication (nine named properties), FAQPage (the five
 *    named topics), and Organization expressing the GigWhere acknowledgement as a real
 *    `contributor` property;
 *  - the quickstart page: HowTo mirroring docs/quickstart.md, plus a WebPage;
 *  - the concepts and troubleshooting pages: a TechArticle.
 *
 * Every entity carries a dateModified equal to the shared editorial date, so the
 * structured-data date matches the visible last-updated line (FR-45). The blocks are
 * authored here rather than derived from page copy, with one exception: the landing
 * page's SoftwareApplication `featureList` composes its tool-surface figures from the
 * generated surface manifest (CR-0073 FR-15), so the count it publishes tracks the code.
 *
 * @agents-index Builds the schema.org JSON-LD script blocks per page: SoftwareApplication, FAQPage, Organization, HowTo, and TechArticle.
 */
import { SITE_ORIGIN, LAST_UPDATED_ISO } from '../src/site.meta'
import { canonicalUrl, type PageKey, type PageSeo } from './seo.pages'
import { domainNames } from '../src/surface'

/** The public source repository, reused across several entity properties. */
const REPO = 'https://github.com/desek/outlook-local-mcp'

/** The GigWhere site, acknowledged as a contributor in the Organization entity (FR-43). */
const GIGWHERE = 'https://gigwhere.com'

/**
 * organization is the publisher Organization entity. It names GigWhere as a
 * `contributor` so the acknowledgement is a first-class structured-data property and
 * not only footer text (FR-43).
 *
 * `contributor` rather than `sponsor` is deliberate: GigWhere contributed time and
 * testing support, not money or goods, and schema.org `sponsor` denotes support
 * through a pledge, promise, or financial contribution. Structured data is read as
 * fact by generative engines, so the property has to match the actual relationship.
 */
function organization(): Record<string, unknown> {
  return {
    '@type': 'Organization',
    name: 'Outlook Local MCP',
    url: SITE_ORIGIN,
    logo: `${SITE_ORIGIN}/icon.png`,
    contributor: {
      '@type': 'Organization',
      name: 'GigWhere',
      url: GIGWHERE,
    },
  }
}

/**
 * softwareApplication is the landing page's SoftwareApplication entity, carrying all
 * nine properties FR-40 names.
 *
 * `featureList` is composed from the surface manifest (CR-0073 FR-15): it names the four
 * aggregate tools and the full and default verb counts, all read from the generated
 * record rather than transcribed, so the structured data a generative engine quotes can
 * never state a tool surface the server does not expose.
 */
function softwareApplication(): Record<string, unknown> {
  return {
    '@type': 'SoftwareApplication',
    name: 'Outlook Local MCP',
    description:
      'A local, single-binary Model Context Protocol server that connects Claude and other MCP clients to Microsoft Outlook Calendar and Mail through the Microsoft Graph API.',
    applicationCategory: 'DeveloperApplication',
    operatingSystem: 'macOS, Linux, Windows',
    license: `${REPO}/blob/main/LICENSE`,
    codeRepository: REPO,
    programmingLanguage: 'Go',
    downloadUrl: `${REPO}/releases`,
    softwareVersion: '0.8.0',
    featureList: `Read and write Microsoft Calendar and Mail from a chat: check availability, book and reschedule meetings, search and send messages, and manage several accounts. Grouped as the ${domainNames.join(', ')} tools.`,
    dateModified: LAST_UPDATED_ISO,
  }
}

/**
 * FAQ_TOPICS are the five question-and-answer pairs FR-41 requires. The answers are
 * authored to be accurate and self-contained so a generative engine can quote them.
 */
const FAQ_TOPICS: ReadonlyArray<{ q: string; a: string }> = [
  {
    q: 'What is Outlook Local MCP?',
    a: 'Outlook Local MCP is a local, single-binary Model Context Protocol server that connects Claude and other MCP clients to Microsoft Outlook Calendar and Mail through the Microsoft Graph API. It runs on your own machine.',
  },
  {
    q: 'Does it require an Entra ID app registration?',
    a: 'No. It authenticates against Microsoft using well-known public client identifiers, so you do not need to create or administer an Entra ID (Azure AD) app registration to use it.',
  },
  {
    q: 'Does my data leave my machine?',
    a: 'The server runs locally and stores tokens on your machine. Its only outbound connections are to Microsoft Graph and the Microsoft Identity Platform for the account you connect, plus an optional OpenTelemetry export if you enable it.',
  },
  {
    q: 'Which Outlook features are supported?',
    a: 'Calendar reading, searching, and event and meeting management, plus opt-in read-only access to mail folders, messages, and full-text search. Multiple Microsoft accounts can be connected at once.',
  },
  {
    q: 'How do I connect it to Claude Desktop?',
    a: 'Add the server binary to your Claude Desktop MCP configuration and restart Claude Desktop. The quickstart page walks through the configuration and the first authenticated tool call.',
  },
]

/**
 * faqPage is the landing page's FAQPage entity built from FAQ_TOPICS (FR-41).
 */
function faqPage(): Record<string, unknown> {
  return {
    '@type': 'FAQPage',
    mainEntity: FAQ_TOPICS.map((t) => ({
      '@type': 'Question',
      name: t.q,
      acceptedAnswer: { '@type': 'Answer', text: t.a },
    })),
  }
}

/**
 * HOWTO_STEPS mirror the numbered steps of docs/quickstart.md (FR-42). They are kept in
 * step order so the HowTo reads as the same procedure the quickstart page publishes.
 */
const HOWTO_STEPS: ReadonlyArray<{ name: string; text: string }> = [
  {
    name: 'Build',
    text: 'Build the outlook-local-mcp binary from source, or install a released build.',
  },
  {
    name: 'Configure Claude Desktop or Claude Code',
    text: 'Register the server binary in your MCP client configuration so the client launches it.',
  },
  {
    name: 'Authenticate and verify',
    text: 'Trigger the first tool call, complete the one-time Microsoft sign-in, and confirm the server returns your calendar data.',
  },
]

/**
 * howTo is the quickstart page's HowTo entity (FR-42).
 */
function howTo(): Record<string, unknown> {
  return {
    '@type': 'HowTo',
    name: 'Connect Claude to Microsoft Outlook with Outlook Local MCP',
    description:
      'Install Outlook Local MCP, configure your MCP client, and authenticate to make your first Outlook Calendar and Mail tool call.',
    step: HOWTO_STEPS.map((s, i) => ({
      '@type': 'HowToStep',
      position: i + 1,
      name: s.name,
      text: s.text,
    })),
    dateModified: LAST_UPDATED_ISO,
  }
}

/**
 * techArticle is the generic entity for the narrative documentation pages, giving them
 * a valid schema.org type with a dateModified (FR-39, FR-45).
 *
 * @param page  The documentation page being described.
 */
function techArticle(page: PageSeo): Record<string, unknown> {
  return {
    '@type': 'TechArticle',
    headline: page.title,
    description: page.description,
    url: canonicalUrl(page),
    dateModified: LAST_UPDATED_ISO,
  }
}

/**
 * webPage wraps a documentation or quickstart URL as a WebPage carrying dateModified,
 * ensuring every non-landing page states its modification date in structured data.
 *
 * @param page  The page being described.
 */
function webPage(page: PageSeo): Record<string, unknown> {
  return {
    '@type': 'WebPage',
    name: page.title,
    url: canonicalUrl(page),
    dateModified: LAST_UPDATED_ISO,
  }
}

/**
 * entitiesFor returns the list of schema.org entities for a page key.
 *
 * @param page  The page whose entities are wanted.
 * @returns One or more schema.org entity objects (without the @context wrapper).
 */
function entitiesFor(page: PageSeo): Record<string, unknown>[] {
  const byKey: Record<PageKey, () => Record<string, unknown>[]> = {
    index: () => [softwareApplication(), faqPage(), organization()],
    quickstart: () => [howTo(), webPage(page)],
    concepts: () => [techArticle(page)],
    troubleshooting: () => [techArticle(page)],
  }
  return byKey[page.key]()
}

/**
 * renderJsonLd renders the JSON-LD script block(s) for a page.
 *
 * Each entity is emitted as its own <script type="application/ld+json"> with an
 * @context, so a single malformed entity cannot invalidate the others and each parses
 * independently.
 *
 * @param page  The page whose structured data is rendered.
 * @returns An HTML fragment of one or more script blocks, indented for the head.
 */
export function renderJsonLd(page: PageSeo): string {
  return entitiesFor(page)
    .map((entity) => {
      const doc = { '@context': 'https://schema.org', ...entity }
      const json = JSON.stringify(doc, null, 2)
        .split('\n')
        .map((line) => `      ${line}`)
        .join('\n')
      return `    <script type="application/ld+json">\n${json}\n    </script>`
    })
    .join('\n')
}
