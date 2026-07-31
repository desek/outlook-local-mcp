/**
 * provenance.ts - build provenance record derived from the environment.
 *
 * Provenance identifies which commit, at what time, and in which CI run produced a
 * given deploy. CR-0070 FR-26 to FR-30 require it to be readable without executing
 * JavaScript (as <meta> tags, injected by the companion Vite plugin) and available
 * as a fetchable /build-info.json.
 *
 * The load-bearing rule (FR-29, FR-30) is that provenance must never lie. Values are
 * read from the CI environment at build time and are never committed as literals. A
 * build with no CI environment still succeeds, but it marks itself explicitly as a
 * local build ("local") rather than fabricating a commit or run identifier that would
 * misidentify the artifact. Provenance that can lie is worse than none.
 *
 * @agents-index Build provenance record: reads commit, run, and env from CI or falls back to an explicit local-build marker.
 */

/**
 * The sentinel used for every identity field when no CI environment is present.
 * A crawler or a support conversation reading "local" knows the artifact was not
 * produced by the deployment workflow and carries no authoritative commit or run.
 */
export const LOCAL_MARKER = 'local'

/**
 * BuildProvenance is the machine-readable provenance record.
 *
 * @property commit  The commit SHA the site was built from, or LOCAL_MARKER for a local build.
 * @property buildTime  The build instant in UTC, ISO 8601. This is the real build time in both CI and local builds; it identifies nothing about origin, so it is not fabricated.
 * @property run  The workflow run identifier that produced the build, or LOCAL_MARKER for a local build.
 * @property environment  Either "ci" or "local", stated explicitly so a reader never has to infer origin from a sentinel value.
 */
export interface BuildProvenance {
  commit: string
  buildTime: string
  run: string
  environment: 'ci' | 'local'
}

/**
 * computeProvenance derives the provenance record from a process environment.
 *
 * GitHub Actions sets GITHUB_ACTIONS=true, GITHUB_SHA, and GITHUB_RUN_ID. When those
 * CI markers are present the values are taken verbatim; otherwise every identity
 * field collapses to LOCAL_MARKER and the environment is reported as "local". The
 * build timestamp is always the genuine current UTC instant.
 *
 * @param env  The environment to read, defaulting to process.env. Injected for testability.
 * @param now  The build instant, defaulting to the current time. Injected for testability.
 * @returns A fully-populated BuildProvenance that never fabricates an origin.
 */
export function computeProvenance(
  env: NodeJS.ProcessEnv = process.env,
  now: Date = new Date(),
): BuildProvenance {
  const isCi = env.GITHUB_ACTIONS === 'true' && Boolean(env.GITHUB_SHA)
  return {
    commit: isCi ? String(env.GITHUB_SHA) : LOCAL_MARKER,
    buildTime: now.toISOString(),
    run: isCi ? String(env.GITHUB_RUN_ID ?? LOCAL_MARKER) : LOCAL_MARKER,
    environment: isCi ? 'ci' : 'local',
  }
}
