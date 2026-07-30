import { useRef, useEffect } from 'react'

// ─── Constants ───────────────────────────────────────────────────────────────

const PARTICLE_COUNT = 80
const SPREAD_X = 6
const SPREAD_Y = 3.6
const DRIFT_SPEED = 0.012
const ORBITAL_SPEED = 0.008
const BG_COLOR = '#052424'

interface Particle {
  x: number
  y: number
  z: number
  vx: number
  vy: number
  vz: number
  phase: number
  r: number
  g: number
  b: number
}

function createParticles(): Particle[] {
  const particles: Particle[] = []
  const limeR = 0.35, limeG = 0.55, limeB = 0.10
  const grayR = 0.45, grayG = 0.42, grayB = 0.38

  for (let i = 0; i < PARTICLE_COUNT; i++) {
    const dx = (Math.random() - 0.5)
    const dy = (Math.random() - 0.5)
    const dz = (Math.random() - 0.5) * 0.3
    const len = Math.sqrt(dx * dx + dy * dy + dz * dz) || 1
    const t = Math.random()
    particles.push({
      x: (Math.random() - 0.5) * SPREAD_X * 2,
      y: (Math.random() - 0.5) * SPREAD_Y * 2,
      z: (Math.random() - 0.5) * SPREAD_X * 0.8,
      vx: (dx / len) * DRIFT_SPEED,
      vy: (dy / len) * DRIFT_SPEED,
      vz: (dz / len) * DRIFT_SPEED * 0.3,
      phase: Math.random() * Math.PI * 2,
      r: limeR * (1 - t) + grayR * t,
      g: limeG * (1 - t) + grayG * t,
      b: limeB * (1 - t) + grayB * t,
    })
  }
  return particles
}

// ─── BrandRevealParticles ─────────────────────────────────────────────────────

interface BrandRevealParticlesProps {
  className?: string
  active?: boolean
}

export default function BrandRevealParticles({
  className = '',
  active = true,
}: BrandRevealParticlesProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const particlesRef = useRef<Particle[]>(createParticles())
  const rafRef = useRef<number>(0)
  const timeRef = useRef<number>(0)
  const activeRef = useRef(active)
  const visibleRef = useRef(false)

  useEffect(() => {
    activeRef.current = active
  }, [active])

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    const dpr = Math.min(window.devicePixelRatio ?? 1, 2)
    let cssW = 0
    let cssH = 0

    const resize = () => {
      const rect = canvas.getBoundingClientRect()
      cssW = rect.width
      cssH = rect.height
      canvas.width = Math.round(cssW * dpr)
      canvas.height = Math.round(cssH * dpr)
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
    }
    resize()
    const ro = new ResizeObserver(resize)
    ro.observe(canvas)

    // Pause RAF when off-screen
    const io = new IntersectionObserver(
      ([entry]) => {
        const nowVisible = entry?.isIntersecting ?? false
        const wasVisible = visibleRef.current
        visibleRef.current = nowVisible
        if (nowVisible && !wasVisible) {
          timeRef.current = performance.now()
          rafRef.current = requestAnimationFrame(draw)
        }
      },
      { threshold: 0 }
    )
    io.observe(canvas)

    const FOV = 400

    const draw = (ts: number) => {
      if (!visibleRef.current || !activeRef.current) {
        rafRef.current = 0
        return
      }
      rafRef.current = requestAnimationFrame(draw)

      const dt = Math.min((ts - timeRef.current) / 1000, 0.05)
      timeRef.current = ts

      const w = cssW
      const h = cssH
      if (!w || !h) return

      ctx.fillStyle = BG_COLOR
      ctx.fillRect(0, 0, w, h)

      // Central glow
      const pulse = Math.sin(ts * 0.0004) * 0.08 + 1.0
      const cx = w / 2
      const cy = h / 2
      const glowR = Math.min(w, h) * 0.25 * pulse
      const grad = ctx.createRadialGradient(cx, cy, 0, cx, cy, glowR)
      grad.addColorStop(0, 'rgba(46, 97, 46, 0.12)')
      grad.addColorStop(1, 'rgba(46, 97, 46, 0)')
      ctx.fillStyle = grad
      ctx.beginPath()
      ctx.arc(cx, cy, glowR, 0, Math.PI * 2)
      ctx.fill()

      const t = ts * 0.001
      const scaleX = w / (SPREAD_X * 2.2)
      const scaleY = h / (SPREAD_Y * 2.2)
      const particles = particlesRef.current

      for (let i = 0; i < particles.length; i++) {
        const p = particles[i]!

        p.x += p.vx
        p.y += p.vy
        p.z += p.vz
        p.x += Math.sin(t * ORBITAL_SPEED * 60 + p.phase) * 0.0008
        p.y += Math.cos(t * ORBITAL_SPEED * 60 + p.phase * 1.3) * 0.0006

        const bX = SPREAD_X * 1.1
        const bY = SPREAD_Y * 1.4
        const bZ = SPREAD_X * 0.5
        if (Math.abs(p.x) > bX) { p.vx *= -1; p.x = Math.sign(p.x) * bX }
        if (Math.abs(p.y) > bY) { p.vy *= -1; p.y = Math.sign(p.y) * bY }
        if (Math.abs(p.z) > bZ) { p.vz *= -1; p.z = Math.sign(p.z) * bZ }

        // Perspective projection
        const depth = p.z + SPREAD_X
        const scale = FOV / (FOV + depth * 30)
        const sx = cx + p.x * scaleX * scale
        const sy = cy + p.y * scaleY * scale
        const size = Math.max(1, 2.5 * scale)
        const alpha = 0.55 * scale

        ctx.beginPath()
        ctx.arc(sx, sy, size, 0, Math.PI * 2)
        ctx.fillStyle = `rgba(${Math.round(p.r * 255)}, ${Math.round(p.g * 255)}, ${Math.round(p.b * 255)}, ${alpha})`
        ctx.fill()
      }

      void dt
    }

    return () => {
      if (rafRef.current) cancelAnimationFrame(rafRef.current)
      ro.disconnect()
      io.disconnect()
    }
  }, [])

  return (
    <canvas
      ref={canvasRef}
      className={className}
      style={{ width: '100%', height: '100%', display: 'block', background: BG_COLOR }}
    />
  )
}
