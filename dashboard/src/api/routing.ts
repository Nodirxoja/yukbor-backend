// Road routing for the map.
//
// A straight line between pickup and drop-off is a lie: Tashkent to Samarkand
// is 270 km of road across a 240 km straight line, and the line runs through
// terrain no truck crosses. For a logistics back office the shape of the actual
// route is the information.
//
// Uses the public OSRM demo server — free, no API key, no account. That is also
// its weakness: it is a shared best-effort service with no uptime promise, so
// every failure falls back to a straight line rendered DIFFERENTLY (dashed), so
// nobody mistakes an estimate for a real route.
//
// Swapping in a paid router later means changing only fetchRoute.

import type { Location } from './types'

/** [lat, lng] — Leaflet's order, not GeoJSON's. */
export type LatLng = [number, number]

export interface Route {
  points: LatLng[]
  distanceKm: number
  durationMin: number
}

const cache = new Map<string, Route | null>()
const inflight = new Map<string, Promise<Route | null>>()

// The demo server is shared infrastructure; two at a time is neighbourly and
// still fast enough for a screen showing a handful of orders.
const MAX_CONCURRENT = 2
let active = 0
const queue: (() => void)[] = []

function acquire(): Promise<void> {
  if (active < MAX_CONCURRENT) {
    active++
    return Promise.resolve()
  }
  return new Promise((resolve) => queue.push(() => { active++; resolve() }))
}

function release() {
  active--
  queue.shift()?.()
}

function key(from: Location, to: Location): string {
  return `${from.latitude},${from.longitude};${to.latitude},${to.longitude}`
}

/** True when pickup and drop-off are effectively the same spot (on-site jobs). */
export function isSamePlace(from: Location, to: Location): boolean {
  return (
    Math.abs(from.latitude - to.latitude) < 1e-4 &&
    Math.abs(from.longitude - to.longitude) < 1e-4
  )
}

/**
 * Resolves the driving route, or null if it cannot be determined. Results are
 * cached for the session — the same city pair is asked for by many orders and
 * the answer does not change.
 */
export function fetchRoute(from: Location, to: Location): Promise<Route | null> {
  if (isSamePlace(from, to)) return Promise.resolve(null)

  const k = key(from, to)
  const cached = cache.get(k)
  if (cached !== undefined) return Promise.resolve(cached)
  const pending = inflight.get(k)
  if (pending) return pending

  const request = (async (): Promise<Route | null> => {
    await acquire()
    try {
      // OSRM takes lon,lat — the opposite of Leaflet.
      const coords = `${from.longitude},${from.latitude};${to.longitude},${to.latitude}`
      const url = `https://router.project-osrm.org/route/v1/driving/${coords}?overview=full&geometries=geojson`

      const controller = new AbortController()
      const timeout = setTimeout(() => controller.abort(), 12_000)
      const res = await fetch(url, { signal: controller.signal })
      clearTimeout(timeout)
      if (!res.ok) return null

      const data = (await res.json()) as {
        code?: string
        routes?: { distance: number; duration: number; geometry: { coordinates: [number, number][] } }[]
      }
      const route = data.routes?.[0]
      if (data.code !== 'Ok' || !route) return null

      return {
        points: route.geometry.coordinates.map(([lng, lat]) => [lat, lng] as LatLng),
        distanceKm: route.distance / 1000,
        durationMin: route.duration / 60,
      }
    } catch {
      // Offline, blocked, timed out, rate-limited — the caller draws the
      // straight-line fallback and marks it as such.
      return null
    } finally {
      release()
      inflight.delete(k)
    }
  })()

  inflight.set(k, request)
  return request.then((r) => {
    cache.set(k, r)
    return r
  })
}
