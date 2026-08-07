import { useEffect, useState } from 'react'
import { MapContainer, Marker, Polyline, Popup, TileLayer } from 'react-leaflet'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import type { Order } from '../api/types'
import { fetchRoute, isSamePlace } from '../api/routing'
import type { Route } from '../api/routing'

// Leaflet's default marker images don't resolve under bundlers. Rather than
// pointing them at a CDN, the pin is an inline SVG: one less thing that can
// fail on venue wifi mid-demo, and it lets markers carry the order's status
// colour. The tile layer still needs the network; the markers no longer do.
function pin(color: string, hollow = false) {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="26" height="38" viewBox="0 0 26 38">
    <path d="M13 0C5.8 0 0 5.8 0 13c0 9.4 13 25 13 25s13-15.6 13-25c0-7.2-5.8-13-13-13z"
          fill="${hollow ? '#ffffff' : color}" stroke="${color}" stroke-width="${hollow ? 3 : 0}"/>
    <circle cx="13" cy="13" r="5" fill="${hollow ? color : '#fff'}"/>
  </svg>`
  return L.icon({
    iconUrl: `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`,
    iconSize: [26, 38],
    iconAnchor: [13, 38],
    popupAnchor: [0, -32],
  })
}

// Markers are colour-coded by status (plan §11).
const markerColor: Record<string, string> = {
  draft: '#94a3b8',
  published: '#3b82f6',
  matched: '#06b6d4',
  accepted: '#6366f1',
  inProgress: '#8b5cf6',
  loadingInProgress: '#8b5cf6',
  inTransit: '#f97316',
  delivered: '#14b8a6',
  completed: '#22c55e',
  cancelled: '#ef4444',
  disputed: '#e11d48',
}

const CENTER: [number, number] = [40.5, 68.0]

/** Resolves real driving routes for every order that has two distinct points. */
function useRoutes(orders: Order[]) {
  const [routes, setRoutes] = useState<Record<string, Route | null>>({})

  useEffect(() => {
    let alive = true
    for (const o of orders) {
      if (isSamePlace(o.pickupLocation, o.dropoffLocation)) continue
      if (routes[o.id] !== undefined) continue
      void fetchRoute(o.pickupLocation, o.dropoffLocation).then((r) => {
        if (alive) setRoutes((prev) => ({ ...prev, [o.id]: r }))
      })
    }
    return () => {
      alive = false
    }
    // Keyed on the set of orders, not the routes map, or resolving one route
    // would re-trigger the effect for all of them.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [orders.map((o) => o.id).join(',')])

  return routes
}

export function OrdersMap({
  orders,
  selectedId,
  onSelect,
}: {
  orders: Order[]
  selectedId: string | null
  onSelect: (id: string) => void
}) {
  const routes = useRoutes(orders)

  return (
    <MapContainer
      center={CENTER}
      zoom={6}
      style={{ height: '100%', minHeight: 420, borderRadius: 'var(--radius-3)' }}
    >
      <TileLayer
        attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
        url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
      />
      {orders.map((o) => {
        const pickup: [number, number] = [o.pickupLocation.latitude, o.pickupLocation.longitude]
        const dropoff: [number, number] = [o.dropoffLocation.latitude, o.dropoffLocation.longitude]
        const onSite = isSamePlace(o.pickupLocation, o.dropoffLocation)
        const route = routes[o.id]
        const selected = o.id === selectedId
        const colour = markerColor[o.status] ?? '#94a3b8'

        return (
          <span key={o.id}>
            <Marker
              position={pickup}
              icon={pin(colour)}
              eventHandlers={{ click: () => onSelect(o.id) }}
            >
              <Popup>
                <b>{o.clientName}</b>
                <br />
                {o.type} · {o.status}
                <br />
                {o.pickupAddress}
                {!onSite && (
                  <>
                    <br />→ {o.dropoffAddress}
                  </>
                )}
                <br />
                {Number(o.priceEstimate).toLocaleString('ru-RU')} {o.currency}
                {route && (
                  <>
                    <br />
                    {route.distanceKm.toFixed(0)} km · {(route.durationMin / 60).toFixed(1)} h by road
                  </>
                )}
              </Popup>
            </Marker>

            {/* Drop-off gets a hollow pin, so a route reads directionally. */}
            {!onSite && (
              <Marker
                position={dropoff}
                icon={pin(colour, true)}
                eventHandlers={{ click: () => onSelect(o.id) }}
              >
                <Popup>
                  Drop-off — {o.dropoffAddress}
                  <br />
                  <b>{o.clientName}</b>
                </Popup>
              </Marker>
            )}

            {!onSite && (
              <Polyline
                positions={route ? route.points : [pickup, dropoff]}
                pathOptions={{
                  color: selected ? '#3b82f6' : colour,
                  weight: selected ? 5 : 3,
                  opacity: route ? (selected ? 0.95 : 0.65) : 0.35,
                  // Dashed means "we could not get a real route, this is a
                  // straight-line placeholder" — never mistake one for the other.
                  dashArray: route ? undefined : '6 8',
                }}
                eventHandlers={{ click: () => onSelect(o.id) }}
              />
            )}
          </span>
        )
      })}
    </MapContainer>
  )
}
