import { MapContainer, Marker, Polyline, Popup, TileLayer } from 'react-leaflet'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import type { Order } from '../api/types'

// Leaflet's default marker images don't resolve under bundlers — point them
// at the CDN once here.
const icon = L.icon({
  iconUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon.png',
  iconRetinaUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon-2x.png',
  shadowUrl: 'https://unpkg.com/leaflet@1.9.4/dist/images/marker-shadow.png',
  iconSize: [25, 41],
  iconAnchor: [12, 41],
  popupAnchor: [1, -34],
})

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
              icon={icon}
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
