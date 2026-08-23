/**
 * Reads the project's released version from the repository's release record, so the site
 * never transcribes a version literal of its own (CR-0077 FR-8).
 *
 * `.release-please-manifest.json` is the authoritative record: release-please rewrites it
 * on every release and it is tracked in the repository, so it is the one value that cannot
 * drift from what was actually published. The alternatives were rejected because they do
 * not carry a release version at build time: the provenance pipeline deliberately records
 * which build produced the artefact and not which version it is, `internal/buildinfo` is
 * populated at Go link time and no Go binary runs during the site build, and
 * `site/package.json` is unmaintained at `0.0.0`.
 *
 * @agents-index Reads the released version from the repository-root release-please manifest for the site's structured data.
 */
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

/** The release record, relative to this module: the repository root is two levels above site/build/. */
const MANIFEST = '.release-please-manifest.json'

/**
 * releaseVersion reads the released version from the repository-root release-please
 * manifest. The path is resolved from this module's location, matching how the SEO plugin
 * reaches the repository root for llms.txt.
 *
 * @returns The bare semantic version under the manifest's `"."` key, in the form
 *   schema.org's `softwareVersion` expects (no `v` prefix).
 * @throws Error if the manifest is missing or carries no `"."` key. There is deliberately
 *   no fallback: a default would publish a version nobody released, which is the failure
 *   this module exists to remove (CR-0077 AC-6b).
 */
export function releaseVersion(): string {
  const manifestPath = fileURLToPath(new URL(`../../${MANIFEST}`, import.meta.url))
  let raw: string
  try {
    raw = readFileSync(manifestPath, 'utf8')
  } catch {
    throw new Error(
      `${MANIFEST} is missing (expected at the repository root, resolved to ${manifestPath}); ` +
        'the site reads the published software version from it and cannot build without it',
    )
  }
  const version = (JSON.parse(raw) as Record<string, unknown>)['.']
  if (typeof version !== 'string' || version.length === 0) {
    throw new Error(
      `${MANIFEST} (at ${manifestPath}) has no "." key holding the released version; ` +
        'the site reads the published software version from it and cannot build without it',
    )
  }
  return version
}
