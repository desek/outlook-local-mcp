#!/usr/bin/env node
/**
 * @agents-index WCAG contrast ratio calculator and minimal-adjustment solver for site colours.
 *
 * Purpose:
 *   Fixing a contrast failure by eye tends to overshoot, which changes the design more
 *   than accessibility requires. This computes the WCAG 2.x ratio for a pair, and — given
 *   a foreground that must sit on a known background — solves for the smallest adjustment
 *   that reaches the target: either the darkest-still-legible lightness for an opaque
 *   colour, or the lowest alpha for a `color/opacity` utility.
 *
 * Usage:
 *   node .agents/scripts/contrast.mjs ratio <fg> <bg>
 *   node .agents/scripts/contrast.mjs solve-alpha <fg> <bg> [target]
 *   node .agents/scripts/contrast.mjs solve-shade <fg> <bg> [target]
 *
 * Parameters:
 *   fg, bg   Hex colours, with or without a leading '#'.
 *   target   Required ratio. Defaults to 4.5 (WCAG AA for normal-size text).
 *
 * `solve-alpha` finds the lowest alpha at which `fg` composited over `bg` meets target.
 * `solve-shade` scales `fg` toward black (on light backgrounds) or white (on dark ones)
 * by the smallest factor that meets target, preserving hue.
 */

/**
 * Parse a hex colour into 0-255 channels.
 *
 * @param {string} hex Colour such as `#abff02` or `abff02`.
 * @returns {[number, number, number]} Red, green and blue channels.
 */
function parse(hex) {
  const value = hex.replace('#', '')
  return [0, 2, 4].map((i) => parseInt(value.slice(i, i + 2), 16))
}

/**
 * Format channels back to a hex string.
 *
 * @param {number[]} rgb Red, green and blue, 0-255 (rounded internally).
 * @returns {string} Lowercase `#rrggbb`.
 */
function format(rgb) {
  return `#${rgb.map((c) => Math.round(c).toString(16).padStart(2, '0')).join('')}`
}

/**
 * Relative luminance per WCAG 2.x.
 *
 * @param {number[]} rgb Channels, 0-255.
 * @returns {number} Luminance in [0, 1].
 */
function luminance(rgb) {
  const [r, g, b] = rgb.map((c) => {
    const s = c / 255
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

/**
 * WCAG contrast ratio between two colours.
 *
 * @param {number[]} a First colour's channels.
 * @param {number[]} b Second colour's channels.
 * @returns {number} Ratio in [1, 21].
 */
function ratio(a, b) {
  const [high, low] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (high + 0.05) / (low + 0.05)
}

/**
 * Composite a colour over a background at a given alpha.
 *
 * @param {number[]} fg Foreground channels.
 * @param {number[]} bg Background channels.
 * @param {number} alpha Opacity in [0, 1].
 * @returns {number[]} The resulting opaque channels.
 */
function over(fg, bg, alpha) {
  return fg.map((c, i) => c * alpha + bg[i] * (1 - alpha))
}

const [mode, fgArg, bgArg, targetArg] = process.argv.slice(2)
const target = Number(targetArg ?? 4.5)

if (!mode || !fgArg || !bgArg) {
  console.error('usage: node .agents/scripts/contrast.mjs <ratio|solve-alpha|solve-shade> <fg> <bg> [target]')
  process.exit(2)
}

const fg = parse(fgArg)
const bg = parse(bgArg)

if (mode === 'ratio') {
  console.log(`${format(fg)} on ${format(bg)}: ${ratio(fg, bg).toFixed(2)}:1`)
} else if (mode === 'solve-alpha') {
  for (let percent = 1; percent <= 100; percent++) {
    const composite = over(fg, bg, percent / 100)
    if (ratio(composite, bg) >= target) {
      console.log(`alpha ${percent}% -> ${format(composite)} = ${ratio(composite, bg).toFixed(2)}:1 (target ${target})`)
      process.exit(0)
    }
  }
  console.log(`no alpha reaches ${target}:1`)
  process.exit(1)
} else if (mode === 'solve-shade') {
  const towardWhite = luminance(bg) < 0.18
  for (let step = 0; step <= 100; step++) {
    const factor = step / 100
    const shifted = fg.map((c) => (towardWhite ? c + (255 - c) * factor : c * (1 - factor)))
    if (ratio(shifted, bg) >= target) {
      console.log(
        `shift ${step}% toward ${towardWhite ? 'white' : 'black'} -> ${format(shifted)} = ` +
          `${ratio(shifted, bg).toFixed(2)}:1 (target ${target})`,
      )
      process.exit(0)
    }
  }
  console.log(`no shade reaches ${target}:1`)
  process.exit(1)
} else {
  console.error(`unknown mode: ${mode}`)
  process.exit(2)
}
