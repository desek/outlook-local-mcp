/**
 * @agents-index Static file server over the built site, shared by every site measurement script.
 *
 * Purpose:
 *   The site harness (screenshots, DOM audits, validation) must load the *built*
 *   pages over HTTP rather than file:// so that absolute asset paths, the service
 *   of `.md`/`.txt` crawler files, and relative navigation behave exactly as they
 *   do in production. Lighthouse's `staticDistDir` does the same thing internally;
 *   this module gives the rest of the harness an equivalent, so every tool measures
 *   the identical artefact.
 *
 * Usage (as a module):
 *   import { serve } from './site.serve.mjs'
 *   const { origin, close } = await serve('site/dist')
 *
 * Exports:
 *   serve(root) -> Promise<{ origin: string, port: number, close: () => Promise<void> }>
 */

import { createServer } from 'node:http'
import { readFile } from 'node:fs/promises'
import { extname, join, normalize } from 'node:path'

/** MIME types for every extension the built site emits. */
const TYPES = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.md': 'text/markdown; charset=utf-8',
  '.txt': 'text/plain; charset=utf-8',
  '.xml': 'application/xml; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.woff': 'font/woff',
  '.woff2': 'font/woff2',
}

/**
 * Start a static server rooted at `root`.
 *
 * A bare directory request resolves to `index.html`; an unknown path returns 404
 * rather than falling back to the SPA shell, so a missing artefact surfaces as a
 * measurable failure instead of a silently-correct-looking page.
 *
 * @param {string} root Directory to serve (the built `dist`).
 * @returns {Promise<{origin: string, port: number, close: () => Promise<void>}>}
 *   The bound origin and a close handle. Side effect: binds an ephemeral port on 127.0.0.1.
 */
export async function serve(root) {
  const server = createServer(async (req, res) => {
    let path = decodeURIComponent(new URL(req.url, 'http://localhost').pathname)
    if (path.endsWith('/')) path += 'index.html'
    const file = join(root, normalize(path).replace(/^(\.\.[/\\])+/, ''))
    try {
      const body = await readFile(file)
      res.writeHead(200, {
        'content-type': TYPES[extname(file)] ?? 'application/octet-stream',
        'cache-control': 'no-store',
      })
      res.end(body)
    } catch {
      res.writeHead(404, { 'content-type': 'text/plain' })
      res.end('not found')
    }
  })

  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve))
  const { port } = server.address()
  return {
    origin: `http://127.0.0.1:${port}`,
    port,
    close: () => new Promise((resolve) => server.close(resolve)),
  }
}
