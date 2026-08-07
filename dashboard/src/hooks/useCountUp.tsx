import { useEffect, useRef, useState } from 'react'

const prefersReducedMotion = () =>
  typeof window !== 'undefined' &&
  window.matchMedia('(prefers-reduced-motion: reduce)').matches

/**
 * Animates a number towards its target.
 *
 * On a polling dashboard this does real work rather than decoration: when
 * "held in escrow" jumps because an order was accepted, a counter that TRAVELS
 * shows both that something changed and roughly how much. A number that simply
 * swaps is easy to miss entirely between two ten-second refreshes.
 *
 * Animates from the PREVIOUS value, not from zero — after the first render, a
 * refresh should show the delta, not replay the whole count.
 */
export function useCountUp(target: number, duration = 650): number {
  const [value, setValue] = useState(target)
  const fromRef = useRef(target)
  const frameRef = useRef(0)

  useEffect(() => {
    const from = fromRef.current
    if (from === target || prefersReducedMotion()) {
      fromRef.current = target
      setValue(target)
      return
    }

    const start = performance.now()
    const tick = (now: number) => {
      const t = Math.min(1, (now - start) / duration)
      // Same decelerating curve as the CSS, so motion feels of a piece.
      const eased = 1 - Math.pow(1 - t, 4)
      setValue(from + (target - from) * eased)
      if (t < 1) {
        frameRef.current = requestAnimationFrame(tick)
      } else {
        fromRef.current = target
      }
    }

    frameRef.current = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(frameRef.current)
  }, [target, duration])

  return value
}

/** Whole number, animated. */
export function CountUp({ value }: { value: number }) {
  const n = useCountUp(value)
  return <>{Math.round(n).toLocaleString('ru-RU')}</>
}

/** Money, animated, in the UZS format used across the dashboard. */
export function CountUpMoney({ value, suffix = 'UZS' }: { value: number; suffix?: string }) {
  const n = useCountUp(value)
  return <>{`${Math.round(n).toLocaleString('ru-RU')} ${suffix}`}</>
}
