import { useEffect } from 'react'
import gsap from 'gsap'
import SmoothScroller from './components/SmoothScroller'
import Navigation from './components/Navigation'
import HeroSection from './components/HeroSection'
import IntroSection from './components/IntroSection'
import CapabilitiesSection from './components/CapabilitiesSection'
import BrandRevealSection from './components/BrandRevealSection'
import GettingStartedSection from './components/GettingStartedSection'
import PrivacySection from './components/PrivacySection'
import ToolsReferenceSection from './components/ToolsReferenceSection'
import ConfigReferenceSection from './components/ConfigReferenceSection'
import PreFooterCTA from './components/PreFooterCTA'
import Footer from './components/Footer'

export default function App() {

  /* ── Idle CPU: sleep GSAP ticker after 1.5s inactivity ── */
  useEffect(() => {
    let idleTimer: ReturnType<typeof setTimeout>

    const wake = () => {
      gsap.ticker.wake()
      clearTimeout(idleTimer)
      idleTimer = setTimeout(() => {
        gsap.ticker.sleep()
      }, 1500)
    }

    const events = ['scroll', 'mousemove', 'touchmove', 'keydown', 'click'] as const
    events.forEach((evt) => window.addEventListener(evt, wake, { passive: true }))

    // Start the idle timer
    idleTimer = setTimeout(() => {
      gsap.ticker.sleep()
    }, 1500)

    return () => {
      clearTimeout(idleTimer)
      events.forEach((evt) => window.removeEventListener(evt, wake))
      gsap.ticker.wake()
    }
  }, [])

  return (
    <SmoothScroller>
      <Navigation />
      <main>
        <HeroSection />
        <IntroSection />
        <CapabilitiesSection />
        <BrandRevealSection />
        <GettingStartedSection />
        <PrivacySection />
        <ToolsReferenceSection />
        <ConfigReferenceSection />
        <PreFooterCTA />
      </main>
      <Footer />
    </SmoothScroller>
  )
}
