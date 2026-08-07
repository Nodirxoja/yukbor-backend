import { MapContainer, Marker, Polyline, Popup, TileLayer } from 'react-leaflet'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import type { Order } from '../api/types'

// Leaflet's default marker images don't resolve under bundlers. Rather than
// pointing them at a CDN, the pin is an inline SVG: one less thing that can
// fail on venue wifi mid-demo, and it lets markers carry the order's status
// colour. The tile layer still needs the network; the markers no longer do.
function pin(color: string) {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="26" height="38" viewBox="0 0 26 38">
    <path d="M13 0C5.8 0 0 5.8 0 13c0 9.4 13 25 13 25s13-15.6 13-25c0-7.2-5.8-13-13-13z" fill="${color}"/>
    <circle cx="13" cy="13" r="5" fill="#fff"/>
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

// Centered between Tashkent and Samarkand so demo orders are all visible.
const CENTER: [number, number] = [40.5, 68.0]

export function OrdersMap({
  orders,
  selectedId,
  onSelect,
}: {
  orders: Order[]
  selectedId: string | null
  onSelect: (id: string) => void
}) {
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
        const isTransport = pickup[0] !== dropoff[0] || pickup[1] !== dropoff[1]
        return (
          <span key={o.id}>
            <Marker
              position={pickup}
              icon={pin(markerColor[o.status] ?? '#94a3b8')}
              eventHandlers={{ click: () => onSelect(o.id) }}
            >
              <Popup>
                <b>{o.clientName}</b>
                <br />
                {o.type} · {o.status}
                <br />
                {o.pickupAddress}
                <br />
                {Number(o.priceEstimate).toLocaleString('ru-RU')} {o.currency}
              </Popup>
            </Marker>
            {isTransport && (
              <Polyline
                positions={[pickup, dropoff]}
                pathOptions={{
                  color: o.id === selectedId ? '#3b82f6' : '#94a3b8',
                  weight: o.id === selectedId ? 4 : 2,
                  dashArray: o.status === 'completed' ? undefined : '6 6',
                }}
              />
            )}
          </span>
        )
      })}
    </MapContainer>
  )
}
