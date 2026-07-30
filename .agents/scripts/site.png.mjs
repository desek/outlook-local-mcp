/**
 * @agents-index Minimal dependency-free PNG decoder for the site's visual-regression diff.
 *
 * Purpose:
 *   The visual-regression criterion is stated in per-channel terms ("<= 2/255 per channel
 *   on >= 99% of pixels"), which needs raw pixel access. Rather than add an image library
 *   to the repository for one comparison, this module decodes exactly the PNG subset that
 *   headless Chrome emits: non-interlaced, 8-bit-per-channel, colour type 2 (RGB) or 6
 *   (RGBA). Anything else is rejected loudly instead of being silently mis-decoded.
 *
 * Usage (as a module):
 *   import { decodePng } from './site.png.mjs'
 *   const { width, height, channels, data } = decodePng(await readFile('shot.png'))
 */

import { inflateSync } from 'node:zlib'

/** Bytes per pixel for each supported PNG colour type. */
const CHANNELS = { 2: 3, 6: 4 }

/**
 * Decode a PNG buffer into raw pixel data.
 *
 * @param {Buffer} buffer Complete PNG file contents.
 * @returns {{width: number, height: number, channels: number, data: Buffer}}
 *   Pixel data in row-major order, `channels` bytes per pixel.
 * @throws {Error} If the signature is wrong, or the image is interlaced, not 8-bit,
 *   or uses an unsupported colour type.
 */
export function decodePng(buffer) {
  if (buffer.readUInt32BE(0) !== 0x89504e47) throw new Error('not a PNG')

  let width = 0
  let height = 0
  let channels = 0
  const idat = []

  for (let offset = 8; offset < buffer.length; ) {
    const length = buffer.readUInt32BE(offset)
    const type = buffer.toString('ascii', offset + 4, offset + 8)
    const body = buffer.subarray(offset + 8, offset + 8 + length)
    offset += length + 12

    if (type === 'IHDR') {
      width = body.readUInt32BE(0)
      height = body.readUInt32BE(4)
      if (body[8] !== 8) throw new Error(`unsupported bit depth ${body[8]}`)
      channels = CHANNELS[body[9]]
      if (!channels) throw new Error(`unsupported colour type ${body[9]}`)
      if (body[12] !== 0) throw new Error('interlaced PNG is not supported')
    } else if (type === 'IDAT') {
      idat.push(body)
    } else if (type === 'IEND') {
      break
    }
  }

  return { width, height, channels, data: unfilter(inflateSync(Buffer.concat(idat)), width, height, channels) }
}

/**
 * Reverse the per-scanline PNG filters (spec section 9.2) in place into a fresh buffer.
 *
 * @param {Buffer} raw Inflated scanlines, each prefixed by its filter byte.
 * @param {number} width Image width in pixels.
 * @param {number} height Image height in pixels.
 * @param {number} bpp Bytes per pixel.
 * @returns {Buffer} Unfiltered pixel data, `width * height * bpp` bytes.
 * @throws {Error} On an unknown filter type.
 */
function unfilter(raw, width, height, bpp) {
  const stride = width * bpp
  const out = Buffer.allocUnsafe(stride * height)

  for (let y = 0; y < height; y++) {
    const filter = raw[y * (stride + 1)]
    const line = raw.subarray(y * (stride + 1) + 1, y * (stride + 1) + 1 + stride)
    const row = out.subarray(y * stride, (y + 1) * stride)
    const prior = y === 0 ? null : out.subarray((y - 1) * stride, y * stride)

    for (let x = 0; x < stride; x++) {
      const a = x >= bpp ? row[x - bpp] : 0
      const b = prior ? prior[x] : 0
      const c = prior && x >= bpp ? prior[x - bpp] : 0
      let value
      switch (filter) {
        case 0: value = line[x]; break
        case 1: value = line[x] + a; break
        case 2: value = line[x] + b; break
        case 3: value = line[x] + ((a + b) >> 1); break
        case 4: value = line[x] + paeth(a, b, c); break
        default: throw new Error(`unknown PNG filter ${filter} on row ${y}`)
      }
      row[x] = value & 0xff
    }
  }
  return out
}

/**
 * The Paeth predictor: pick whichever of left/above/upper-left is closest to a + b - c.
 *
 * @param {number} a Byte to the left.
 * @param {number} b Byte above.
 * @param {number} c Byte above-left.
 * @returns {number} The predicted byte.
 */
function paeth(a, b, c) {
  const p = a + b - c
  const pa = Math.abs(p - a)
  const pb = Math.abs(p - b)
  const pc = Math.abs(p - c)
  if (pa <= pb && pa <= pc) return a
  return pb <= pc ? b : c
}
