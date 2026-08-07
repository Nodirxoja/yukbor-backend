// API client with a mock mode (default ON) so the dashboard renders whole
// without the backend. Set VITE_USE_MOCKS=false to hit the real gateway via
// the /api dev proxy (see vite.config.ts).
import type { AdminStats, Order, User } from './types'
import { computeStats, mockOrders, mockUsers } from '../mocks/data'

const USE_MOCKS = import.meta.env.VITE_USE_MOCKS !== 'false'

// The admin endpoints require an admin-role JWT, and there are two ways it
// gets attached — deliberately, because they have different threat models:
//
//   local dev  — scripts/seed.sh writes the token into dashboard/.env.local
//                and Vite exposes it here.
//   production — NO token is built in. Vite inlines every VITE_* value into
//                the JavaScript bundle, which is served publicly, so a token
//                baked in at build time is a token published to the world.
//                Caddy holds it instead and injects the Authorization header
//                server-side, behind basic auth (see docs/DEPLOY.md).
//
// So an empty token is normal in production: send no header and let the proxy
// add it. A 401 then means the proxy is misconfigured, not the browser.
const ADMIN_TOKEN = import.meta.env.VITE_ADMIN_TOKEN ?? ''

async function get<T>(path: string): Promise<T> {
  const res = await fetch(`/api${path}`, {
    headers: ADMIN_TOKEN ? { Authorization: `Bearer ${ADMIN_TOKEN}` } : {},
  })
  if (res.status === 401 || res.status === 403) {
    throw new Error(
      ADMIN_TOKEN
        ? `GET ${path} → ${res.status}: admin token expired or not an admin role`
        : `GET ${path} → ${res.status}: no admin token reached the API — the reverse proxy should be injecting one`,
    )
  }
  if (!res.ok) {
    throw new Error(`GET ${path} → ${res.status}`)
  }
  return res.json() as Promise<T>
}

export async function fetchStats(): Promise<AdminStats> {
  if (USE_MOCKS) return computeStats()
  return get<AdminStats>('/admin/stats')
}

export async function fetchOrders(): Promise<Order[]> {
  if (USE_MOCKS) return mockOrders
  return get<Order[]>('/admin/orders')
}

export async function fetchUsers(): Promise<User[]> {
  if (USE_MOCKS) return mockUsers
  return get<User[]>('/admin/users')
}
