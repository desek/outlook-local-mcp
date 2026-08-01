#!/usr/bin/env node
/**
 * @agents-index Resolve puppeteer-core from the site workspace by package metadata rather than by internal file path.
 *
 * Purpose:
 *   The screenshot, content-check, and contrast-audit harnesses all drive headless
 *   Chrome through `puppeteer-core`, which is a devDependency of `site/` rather than of
 *   the repository root. A bare `import 'puppeteer-core'` therefore does not resolve:
 *   Node walks `.agents/scripts/node_modules`, `.agents/node_modules`, and
 *   `<root>/node_modules`, none of which contain it.
 *
 *   The previous workaround imported a hardcoded internal path,
 *   `../../site/node_modules/puppeteer-core/lib/esm/puppeteer/puppeteer-core.js`. That
 *   is not a supported entry point, and puppeteer-core v25 removed it: the package
 *   dropped the `esm/` directory (`lib/esm/puppeteer/...` became `lib/puppeteer/...`)
 *   and published an `exports` map instead. All three harnesses failed at import with
 *   ERR_MODULE_NOT_FOUND, and because they run only locally and never in CI, no
 *   workflow would have caught it. See CR-0072 and docs/reference/site-quality.md.
 *
 *   This module resolves the package the way Node itself would, honouring `exports`
 *   and `main`, while pointing the lookup at the `site/` workspace. Internal layout may
 *   change again across majors; the resolved entry point is a published contract.
 *
 * Usage:
 *   import { puppeteer } from './site.puppeteer.mjs'
 *
 * Exports:
 *   puppeteer  The puppeteer-core module's default export, exposing `launch`.
 *
 * Side effects: none at import beyond loading puppeteer-core itself.
 * Throws: if puppeteer-core is not installed under `site/`, with a message naming the
 *   install command, because the bare resolver error does not say where to look.
 */

import { createRequire } from 'node:module'
import { fileURLToPath, pathToFileURL } from 'node:url'

const SITE_DIR = fileURLToPath(new URL('../../site/', import.meta.url))
const require = createRequire(import.meta.url)

let entry
try {
  entry = require.resolve('puppeteer-core', { paths: [SITE_DIR] })
} catch (cause) {
  throw new Error(
    `puppeteer-core is not installed under ${SITE_DIR}. Run: pnpm --dir site install`,
    { cause },
  )
}

const loaded = await import(pathToFileURL(entry).href)

/**
 * The puppeteer-core module. v24 resolves to a CommonJS entry and v25 to an ESM one,
 * so the default export is unwrapped defensively rather than assumed.
 */
export const puppeteer = loaded.default ?? loaded
