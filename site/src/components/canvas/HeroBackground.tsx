import { useEffect, useRef } from 'react'

// ── Theme color constants (from index.css) ──────────────────────────────────
const BG_DARK  = '#050e10'
const BG_MID   = '#0a1a1a'
const GRID_COLOR = 'rgba(30, 60, 55, 0.55)'   // desaturated teal, very muted
const GRID_PULSE = 'rgba(50, 90, 70, 0.9)'     // slightly brighter at intersections
const GLOW_COLOR = 'rgba(80, 40, 10, 0.18)'    // warm amber hint, very faint
const PARTICLE_COLORS = [
  'rgba(100, 140, 120, 0.18)',  // desaturated green-gray
  'rgba(80,  110,  90, 0.14)',
  'rgba(60,   90,  70, 0.12)',
  'rgba(120, 160, 140, 0.10)',
]

// ── Connection pulse: AI model → MS Graph sources ───────────────────────────
const PULSE_LINE_COLOR = 'rgba(171, 255, 2, 0.06)'   // lime trace at rest
const PULSE_GLOW_COLOR = [171, 255, 2] as const       // lime RGB for pulse head
const PULSE_CYCLE      = 4.0                           // seconds per full pulse cycle
const PULSE_HEAD_LEN   = 0.18                          // fraction of path length that glows

// ── Types ────────────────────────────────────────────────────────────────────
interface Particle {
  x: number
  y: number
  vx: number
  vy: number
  r: number
  colorIdx: number
  alpha: number
}

interface GridIntersection {
  x: number
  y: number
  phase: number   // phase offset for pulse animation
}

// ── Component ────────────────────────────────────────────────────────────────
interface HeroBackgroundProps {
  className?: string
}

export default function HeroBackground({ className }: HeroBackgroundProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const rafRef    = useRef<number>(0)
  const visibleRef = useRef(true)

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    // ── Sizing ──────────────────────────────────────────────────────────────
    const dpr = Math.min(window.devicePixelRatio ?? 1, 2)
    let W = 0
    let H = 0

    function resize() {
      if (!canvas) return
      const rect = canvas.getBoundingClientRect()
      W = rect.width
      H = rect.height
      canvas.width  = Math.round(W * dpr)
      canvas.height = Math.round(H * dpr)
      ctx!.setTransform(dpr, 0, 0, dpr, 0, 0)
      initScene()
    }

    // ── Scene state ─────────────────────────────────────────────────────────
    let particles: Particle[] = []
    let intersections: GridIntersection[] = []
    const GRID_COLS = 18
    let cellW = 0

    function initScene() {
      // Perspective grid: vertical lines fan out, horizontal lines compress near top
      cellW = W / GRID_COLS

      // Build intersection list (skip the top ~20% — vanishing point area)
      intersections = []
      const horizLines = 12
      for (let row = 1; row <= horizLines; row++) {
        const ty = H * 0.2 + (H * 0.85 - H * 0.2) * Math.pow(row / horizLines, 1.6)
        for (let col = 0; col <= GRID_COLS; col++) {
          const perspectiveShift = (ty / H - 0.2) * 0.5
          const baseX = col * cellW
          const shiftedX = W / 2 + (baseX - W / 2) * (0.4 + perspectiveShift)
          intersections.push({ x: shiftedX, y: ty, phase: Math.random() * Math.PI * 2 })
        }
      }

      // Particles — keep count small and spread them away from center text zone
      const COUNT = Math.min(50, Math.floor((W * H) / 22000))
      particles = Array.from({ length: COUNT }, () => spawnParticle(true))
    }

    function spawnParticle(anywhere: boolean): Particle {
      let x: number, y: number
      if (anywhere) {
        x = Math.random() * W
        y = Math.random() * H
      } else {
        // Re-spawn at edges
        const edge = Math.floor(Math.random() * 4)
        x = edge === 0 ? 0 : edge === 1 ? W : Math.random() * W
        y = edge === 2 ? 0 : edge === 3 ? H : Math.random() * H
      }
      // Drift very slowly; bias slightly upward
      const speed = 0.08 + Math.random() * 0.12
      const angle = -Math.PI / 2 + (Math.random() - 0.5) * Math.PI * 0.6
      return {
        x,
        y,
        vx: Math.cos(angle) * speed,
        vy: Math.sin(angle) * speed,
        r: 1 + Math.random() * 1.5,
        colorIdx: Math.floor(Math.random() * PARTICLE_COLORS.length),
        alpha: 0.1 + Math.random() * 0.2,
      }
    }

    // ── Draw ────────────────────────────────────────────────────────────────
    function draw(ts: number) {
      ctx!.clearRect(0, 0, W, H)

      // 1. Background gradient
      const bg = ctx!.createLinearGradient(0, 0, 0, H)
      bg.addColorStop(0, BG_DARK)
      bg.addColorStop(1, BG_MID)
      ctx!.fillStyle = bg
      ctx!.fillRect(0, 0, W, H)

      // 2. Perspective grid
      drawGrid(ts)

      // 3. Radial glow — center-top
      const glowX = W * 0.5
      const glowY = H * 0.22
      const glowR = Math.min(W, H) * 0.55
      const glow = ctx!.createRadialGradient(glowX, glowY, 0, glowX, glowY, glowR)
      glow.addColorStop(0, GLOW_COLOR)
      glow.addColorStop(1, 'rgba(0,0,0,0)')
      ctx!.fillStyle = glow
      ctx!.fillRect(0, 0, W, H)

      // 4. Connection pulse (AI → Outlook sources)
      drawConnectionPulse(ts)

      // 5. Particles
      drawParticles()

      // 6. Vignette — darken edges so text center stays clear
      const vig = ctx!.createRadialGradient(W / 2, H / 2, H * 0.1, W / 2, H / 2, Math.max(W, H) * 0.75)
      vig.addColorStop(0, 'rgba(0,0,0,0)')
      vig.addColorStop(1, 'rgba(0,0,0,0.55)')
      ctx!.fillStyle = vig
      ctx!.fillRect(0, 0, W, H)
    }

    function drawGrid(ts: number) {
      const horizLines = 12
      ctx!.save()

      // Vertical lines fanning from vanishing point at top-center
      ctx!.lineWidth = 0.5
      for (let col = 0; col <= GRID_COLS; col++) {
        const baseX = col * cellW
        const vanishX = W / 2
        const vanishY = H * 0.05

        // bottom x keeps full spread, top converges to vanish point
        const bottomY = H * 1.02
        const fraction = 1  // at bottom
        const bottomX = vanishX + (baseX - W / 2) * fraction

        const grad = ctx!.createLinearGradient(vanishX, vanishY, bottomX, bottomY)
        grad.addColorStop(0, 'rgba(30, 60, 55, 0.0)')
        grad.addColorStop(0.35, GRID_COLOR)
        grad.addColorStop(1, 'rgba(20, 45, 38, 0.3)')
        ctx!.strokeStyle = grad

        ctx!.beginPath()
        ctx!.moveTo(vanishX, vanishY)
        ctx!.lineTo(bottomX, bottomY)
        ctx!.stroke()
      }

      // Horizontal lines (perspective foreshortened)
      for (let row = 1; row <= horizLines; row++) {
        const ty = H * 0.2 + (H * 0.85 - H * 0.2) * Math.pow(row / horizLines, 1.6)
        const perspectiveShift = (ty / H - 0.2) * 0.5
        const xStart = W / 2 + (-W / 2 - W * 0.1) * (0.4 + perspectiveShift)
        const xEnd   = W / 2 + ( W / 2 + W * 0.1) * (0.4 + perspectiveShift)

        // Fade toward top (distant) and toward very bottom
        const rowFade = row / horizLines
        const alpha = rowFade < 0.3
          ? rowFade / 0.3 * 0.4
          : rowFade > 0.85 ? (1 - rowFade) / 0.15 * 0.35
          : 0.4

        ctx!.strokeStyle = `rgba(30, 60, 55, ${alpha})`
        ctx!.lineWidth = 0.5
        ctx!.beginPath()
        ctx!.moveTo(xStart, ty)
        ctx!.lineTo(xEnd, ty)
        ctx!.stroke()
      }

      // Intersection pulses
      const t = ts / 1000
      for (const pt of intersections) {
        const pulse = (Math.sin(t * 0.6 + pt.phase) + 1) / 2  // 0..1
        if (pulse < 0.7) continue  // only render near peak

        const a = (pulse - 0.7) / 0.3 * 0.35  // max alpha 0.35
        ctx!.beginPath()
        ctx!.arc(pt.x, pt.y, 2, 0, Math.PI * 2)
        ctx!.fillStyle = GRID_PULSE.replace('0.9', String(a.toFixed(2)))
        ctx!.fill()
      }

      ctx!.restore()
    }

    function drawParticles() {
      for (const p of particles) {
        ctx!.beginPath()
        ctx!.arc(p.x, p.y, p.r, 0, Math.PI * 2)
        ctx!.fillStyle = PARTICLE_COLORS[p.colorIdx] ?? '#ffffff'
        ctx!.globalAlpha = p.alpha
        ctx!.fill()
        ctx!.globalAlpha = 1
      }
    }

    // ── Connection pulse: trunk → 90° L-shaped branches ────────────────────
    //
    //                          ┌──────────── endpoint (up far)
    //                     ┌────┘
    //                     │ ┌────────────── endpoint (up near)
    //                     │ │
    //  origin ────────────┼─┼────────────── endpoint (straight)
    //                     │
    //                     └──────────────── endpoint (down)
    //
    // Each branch except "straight" is an L-shape: vertical then horizontal.
    // Different path lengths → pulses arrive at different times.

    interface Branch {
      /** Vertical offset from fork (negative = up, 0 = straight, positive = down) */
      dy: number
      /** Total path length in px (vertical segment + horizontal segment) */
      totalLen: number
    }

    interface PulseLayout {
      originX: number; originY: number
      forkX: number;   forkY: number
      endX: number
      trunkLen: number
      branches: Branch[]
    }

    function getPulseLayout(): PulseLayout {
      const originX = W * 0.08
      const originY = H * 0.50
      const forkX   = W * 0.55
      const forkY   = originY
      const endX    = W * 0.92

      const branchDx = endX - forkX
      // 4 branches: up far, up near, straight, down
      const dyValues = [
        -H * 0.22,   // up far
        -H * 0.11,   // up near
         0,           // straight through
         H * 0.18,   // down
      ]

      const branches: Branch[] = dyValues.map((dy) => ({
        dy,
        totalLen: Math.abs(dy) + branchDx,  // vertical + horizontal
      }))

      return {
        originX, originY, forkX, forkY, endX,
        trunkLen: forkX - originX,
        branches,
      }
    }

    /**
     * Sample a point along a branch's L-shaped path.
     * d = distance in px from the fork point (0 = at fork, totalLen = at endpoint).
     * Straight branch (dy=0) is just horizontal.
     * Others: first |dy| px are vertical, then the rest is horizontal.
     */
    function sampleBranch(
      layout: PulseLayout, branch: Branch, d: number,
    ): [number, number] {
      const { forkX, forkY, endX } = layout
      const vertLen = Math.abs(branch.dy)
      if (d <= vertLen) {
        // Vertical segment
        const dir = branch.dy < 0 ? -1 : 1
        return [forkX, forkY + dir * d]
      }
      // Horizontal segment
      const hFrac = (d - vertLen) / (endX - forkX)
      return [forkX + (endX - forkX) * hFrac, forkY + branch.dy]
    }

    /** Sample a point along the full path (trunk + branch) given distance in px from origin */
    function sampleFullPath(
      layout: PulseLayout, branch: Branch, dist: number,
    ): [number, number] {
      const { originX, originY, trunkLen } = layout
      if (dist <= trunkLen) {
        return [originX + dist, originY]
      }
      return sampleBranch(layout, branch, dist - trunkLen)
    }

    function drawConnectionPulse(ts: number) {
      const layout = getPulseLayout()
      const { originX, originY, forkX, forkY, endX, trunkLen, branches } = layout
      const [r, g, b] = PULSE_GLOW_COLOR

      ctx!.save()
      ctx!.strokeStyle = PULSE_LINE_COLOR
      ctx!.lineWidth = 1

      // ── Static trace: trunk ──
      ctx!.beginPath()
      ctx!.moveTo(originX, originY)
      ctx!.lineTo(forkX, forkY)
      ctx!.stroke()

      // ── Static trace: L-shaped branches ──
      for (const br of branches) {
        if (br.dy === 0) {
          // Straight
          ctx!.beginPath()
          ctx!.moveTo(forkX, forkY)
          ctx!.lineTo(endX, forkY)
          ctx!.stroke()
        } else {
          // Vertical then horizontal
          ctx!.beginPath()
          ctx!.moveTo(forkX, forkY)
          ctx!.lineTo(forkX, forkY + br.dy)
          ctx!.lineTo(endX, forkY + br.dy)
          ctx!.stroke()
        }
      }

      // ── Animated pulses ──
      // Each branch has a different total length so the pulse finishes at
      // different times, creating an organic staggered arrival.
      const steps = 32
      ctx!.lineCap = 'round'

      for (const br of branches) {
        const fullLen = trunkLen + br.totalLen

        // Head length in px — same visual length regardless of path length
        const headPx = PULSE_HEAD_LEN * (trunkLen + (branches[0]?.totalLen ?? 0))

        // Time-based progress: constant speed across all branches.
        // Longest branch sets the cycle; shorter branches finish earlier
        // and idle until the next cycle.
        const longestLen = trunkLen + Math.max(...branches.map((b) => b.totalLen))
        const speed = longestLen / PULSE_CYCLE  // px per second

        const elapsed = (ts / 1000) % PULSE_CYCLE
        const dist = elapsed * speed  // px from origin

        // Head and tail positions clamped to this branch's full length
        const headDist = Math.min(dist, fullLen)
        const tailDist = Math.max(0, dist - headPx)

        // Nothing visible if tail is already past the end
        if (tailDist >= fullLen) continue

        const drawTail = Math.min(tailDist, fullLen)
        const drawHead = Math.min(headDist, fullLen)
        if (drawHead <= drawTail) continue

        // Draw pulse as segmented strokes with ramping alpha
        ctx!.lineWidth = 2
        for (let s = 0; s < steps; s++) {
          const d0 = drawTail + (drawHead - drawTail) * (s / steps)
          const d1 = drawTail + (drawHead - drawTail) * ((s + 1) / steps)
          const segT = (s + 0.5) / steps
          const alpha = segT * segT * 0.55

          const [x1, y1] = sampleFullPath(layout, br, d0)
          const [x2, y2] = sampleFullPath(layout, br, d1)

          ctx!.beginPath()
          ctx!.moveTo(x1, y1)
          ctx!.lineTo(x2, y2)
          ctx!.strokeStyle = `rgba(${r}, ${g}, ${b}, ${alpha})`
          ctx!.stroke()
        }

        // Glow dot at pulse head
        const [hx, hy] = sampleFullPath(layout, br, drawHead)
        const dotGlow = ctx!.createRadialGradient(hx, hy, 0, hx, hy, 8)
        dotGlow.addColorStop(0, `rgba(${r}, ${g}, ${b}, 0.6)`)
        dotGlow.addColorStop(0.4, `rgba(${r}, ${g}, ${b}, 0.15)`)
        dotGlow.addColorStop(1, `rgba(${r}, ${g}, ${b}, 0)`)
        ctx!.fillStyle = dotGlow
        ctx!.fillRect(hx - 8, hy - 8, 16, 16)
      }

      // ── Origin node (AI model) ──
      const nodeGlow = ctx!.createRadialGradient(originX, originY, 0, originX, originY, 12)
      nodeGlow.addColorStop(0, `rgba(${r}, ${g}, ${b}, 0.25)`)
      nodeGlow.addColorStop(0.5, `rgba(${r}, ${g}, ${b}, 0.06)`)
      nodeGlow.addColorStop(1, `rgba(${r}, ${g}, ${b}, 0)`)
      ctx!.fillStyle = nodeGlow
      ctx!.beginPath()
      ctx!.arc(originX, originY, 12, 0, Math.PI * 2)
      ctx!.fill()

      ctx!.beginPath()
      ctx!.arc(originX, originY, 2.5, 0, Math.PI * 2)
      ctx!.fillStyle = `rgba(${r}, ${g}, ${b}, 0.5)`
      ctx!.fill()

      // ── Fork node ──
      ctx!.beginPath()
      ctx!.arc(forkX, forkY, 2, 0, Math.PI * 2)
      ctx!.fillStyle = `rgba(${r}, ${g}, ${b}, 0.3)`
      ctx!.fill()

      // ── Endpoint nodes ──
      for (const br of branches) {
        ctx!.beginPath()
        ctx!.arc(endX, forkY + br.dy, 2, 0, Math.PI * 2)
        ctx!.fillStyle = `rgba(${r}, ${g}, ${b}, 0.3)`
        ctx!.fill()
      }

      ctx!.restore()
    }

    function updateParticles() {
      for (const p of particles) {
        p.x += p.vx
        p.y += p.vy
        // Recycle when off-canvas
        if (p.x < -10 || p.x > W + 10 || p.y < -10 || p.y > H + 10) {
          Object.assign(p, spawnParticle(false))
        }
      }
    }

    // ── Animation loop ───────────────────────────────────────────────────────
    function loop(ts: number) {
      if (visibleRef.current) {
        updateParticles()
        draw(ts)
      }
      rafRef.current = requestAnimationFrame(loop)
    }

    // ── ResizeObserver ────────────────────────────────────────────────────────
    const ro = new ResizeObserver(() => resize())
    ro.observe(canvas)
    resize()

    // ── IntersectionObserver ──────────────────────────────────────────────────
    const io = new IntersectionObserver(
      ([entry]) => { if (entry) visibleRef.current = entry.isIntersecting },
      { threshold: 0 }
    )
    io.observe(canvas)

    rafRef.current = requestAnimationFrame(loop)

    return () => {
      cancelAnimationFrame(rafRef.current)
      ro.disconnect()
      io.disconnect()
    }
  }, [])

  return (
    <canvas
      ref={canvasRef}
      className={className}
      style={{ display: 'block', width: '100%', height: '100%' }}
      aria-hidden="true"
    />
  )
}
