#!/usr/bin/env node
/**
 * @agents-index Compare two site screenshot sets against the visual-regression tolerance.
 *
 * Purpose:
 *   CR-0070 authorises internal markup and hydration changes *provided rendering is
 *   unchanged*. That permission is only usable if "unchanged" is measured rather than
 *   judged, so this script asserts the stated tolerance: every tile must match its
 *   counterpart to within 2/255 per channel on at least 99% of pixels.
 *
 *   A tile present in one set but not the other is a failure, not a skip: a page that
 *   changed height enough to gain or lose a tile has visibly changed.
 *
 * Usage:
 *   node .agents/scripts/site.visual.diff.mjs <baseline-dir> <candidate-dir> [--tolerance N] [--min-ratio R]
 *
 * Parameters:
 *   baseline-dir  Screenshot set from the pre-change build.
 *   candidate-dir Screenshot set from the build under test.
 *   --tolerance   Maximum per-channel delta counted as a match (default 2).
 *   --min-ratio   Minimum fraction of matching pixels per tile (default 0.99).
 *
 * Exits 0 when every tile passes, 1 otherwise, printing the worst offenders.
 */

import { readdir, readFile } from 'node:fs/promises'
import { join } from 'node:path'
import { decodePng } from './site.png.mjs'

const args = process.argv.slice(2)
const [baseDir, candDir] = args.filter((a) => !a.startsWith('--'))
const tolerance = Number(flag('--tolerance') ?? 2)
const minRatio = Number(flag('--min-ratio') ?? 0.99)

if (!baseDir || !candDir) {
  console.error('usage: node .agents/scripts/site.visual.diff.mjs <baseline-dir> <candidate-dir> [--tolerance N] [--min-ratio R]')
  process.exit(2)
}

/**
 * Read the value of a `--name value` flag from argv.
 *
 * @param {string} name Flag name including leading dashes.
 * @returns {string|undefined} The value, or undefined when the flag is absent.
 */
function flag(name) {
  const index = args.indexOf(name)
  return index === -1 ? undefined : args[index + 1]
}

/**
 * Compare one tile pair.
 *
 * @param {Buffer} a Baseline PNG bytes.
 * @param {Buffer} b Candidate PNG bytes.
 * @returns {{ratio: number, worst: number, note?: string}}
 *   Fraction of pixels within tolerance, the largest per-channel delta seen, and a note
 *   when the images are not even the same size (reported as ratio 0).
 */
function compare(a, b) {
  const left = decodePng(a)
  const right = decodePng(b)
  if (left.width !== right.width || left.height !== right.height) {
    return { ratio: 0, worst: 255, note: `size ${left.width}x${left.height} vs ${right.width}x${right.height}` }
  }

  const pixels = left.width * left.height
  let within = 0
  let worst = 0
  for (let i = 0; i < pixels; i++) {
    let delta = 0
    // Compare RGB only; the alpha channel of an opaque screenshot carries no signal.
    for (let c = 0; c < 3; c++) {
      const d = Math.abs(left.data[i * left.channels + c] - right.data[i * right.channels + c])
      if (d > delta) delta = d
    }
    if (delta > worst) worst = delta
    if (delta <= tolerance) within++
  }
  return { ratio: within / pixels, worst }
}

const baseFiles = (await readdir(baseDir)).filter((f) => f.endsWith('.png')).sort()
const candFiles = new Set((await readdir(candDir)).filter((f) => f.endsWith('.png')))

const failures = []
let checked = 0
let worstRatio = 1

for (const file of baseFiles) {
  if (!candFiles.has(file)) {
    failures.push({ file, ratio: 0, worst: 255, note: 'missing in candidate' })
    continue
  }
  candFiles.delete(file)
  const result = compare(await readFile(join(baseDir, file)), await readFile(join(candDir, file)))
  checked++
  if (result.ratio < worstRatio) worstRatio = result.ratio
  if (result.ratio < minRatio) failures.push({ file, ...result })
}
for (const file of candFiles) failures.push({ file, ratio: 0, worst: 255, note: 'missing in baseline' })

failures.sort((a, b) => a.ratio - b.ratio)
for (const f of failures.slice(0, 25)) {
  console.log(`FAIL ${f.file}: ${(f.ratio * 100).toFixed(3)}% within +/-${tolerance} (worst delta ${f.worst})${f.note ? ` [${f.note}]` : ''}`)
}
console.log(
  `visual-diff: ${checked} tiles compared, ${failures.length} failing ` +
    `(tolerance +/-${tolerance}, min ratio ${minRatio}); worst tile ratio ${(worstRatio * 100).toFixed(3)}%`,
)
process.exit(failures.length === 0 ? 0 : 1)
