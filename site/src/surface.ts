/**
 * surface.ts - typed access to the generated surface manifest.
 *
 * The site states no figure of its own about the server's tool surface or configuration.
 * Every count, verb name, domain name, and configuration variable the site displays is
 * read from `src/generated/surface.json`, which `cmd/gen-surface` writes from the live Go
 * verb registries and the `internal/config` inventory (CR-0073). A drift check fails the
 * build when that file no longer matches the code, so the site can never describe a tool
 * surface the server does not expose.
 *
 * This module is the single import point for the manifest: components and the build-time
 * SEO registry read the derived counts and collections from here rather than parsing the
 * JSON, or worse, transcribing a number.
 *
 * @agents-index Typed access to the generated surface manifest and the counts derived from it; the only place the site reads its figures.
 */
import manifest from './generated/surface.json'

/**
 * SurfaceVerb is one operation of an aggregate domain tool.
 *
 * @property name  The `operation` value, without the domain prefix (for example `list_events`).
 * @property summary  The one-line description rendered by the verb registry.
 * @property readOnly  Whether the verb only reads and never mutates.
 * @property gate  The `OUTLOOK_MCP_` variable that gates the verb, or null when it is always exposed.
 */
export interface SurfaceVerb {
  name: string
  summary: string
  readOnly: boolean
  gate: string | null
}

/**
 * SurfaceDomain is one aggregate MCP tool and its verbs.
 *
 * @property name  The domain (aggregate tool) name: calendar, mail, account, or system.
 * @property verbs  The ordered verbs the domain registers.
 * @property fullCount  Verbs exposed with every gate open.
 * @property defaultCount  Verbs exposed under the default configuration.
 */
export interface SurfaceDomain {
  name: string
  verbs: SurfaceVerb[]
  fullCount: number
  defaultCount: number
}

/**
 * SurfaceConfigVar is one environment variable the server reads.
 *
 * @property name  The full `OUTLOOK_MCP_` variable name.
 * @property default  The default value, or the empty string when there is none.
 * @property description  The one-line purpose of the variable.
 */
export interface SurfaceConfigVar {
  name: string
  default: string
  description: string
}

/**
 * SurfaceManifest is the whole generated record: the four domains, the totals, and the
 * configuration inventory.
 */
export interface SurfaceManifest {
  domains: SurfaceDomain[]
  totals: { fullCount: number; defaultCount: number }
  config: SurfaceConfigVar[]
}

/** The parsed manifest. Cast once here so consumers get the typed shape. */
export const surface = manifest as SurfaceManifest

/** The aggregate domain tools, in manifest order. */
export const domains: readonly SurfaceDomain[] = surface.domains

/** The configuration inventory, in manifest order. */
export const configVars: readonly SurfaceConfigVar[] = surface.config

/** The number of aggregate domain tools (calendar, mail, account, system). */
export const domainCount = surface.domains.length

/** Total verbs exposed with every gate open. */
export const fullVerbCount = surface.totals.fullCount

/** Total verbs exposed under the default configuration. */
export const defaultVerbCount = surface.totals.defaultCount

/** The number of environment variables the server reads. */
export const configVarCount = surface.config.length

/** The domain (aggregate tool) names, in manifest order. */
export const domainNames: readonly string[] = surface.domains.map((d) => d.name)

/**
 * domainByName resolves a domain by its aggregate tool name.
 *
 * @param name  The domain name (for example "calendar").
 * @returns The domain, or undefined when no such domain exists.
 */
export function domainByName(name: string): SurfaceDomain | undefined {
  return surface.domains.find((d) => d.name === name)
}
