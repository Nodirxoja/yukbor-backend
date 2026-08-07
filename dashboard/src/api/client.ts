// API client with a mock mode (default ON) so the dashboard renders whole
// without the backend. Set VITE_USE_MOCKS=false to hit the real gateway via
// the /api dev proxy (see vite.config.ts).
import type { AdminStats, Order, User } from './types'
import { computeStats, mockOrders, mockUsers } from '../mocks/data'

const USE_MOCKS = import.meta.env.VITE_USE_MOCKS !== 'false'

// The admin endpoints require an admin-role JWT. A back office does not need
// its own login flow for a hackathon: scripts/seed.sh registers the admin and
// writes the token into dashboard/.env.local, which Vite exposes here.
const ADMIN_TOKEN = import.meta.env.VITE_ADMIN_TOKEN ?? ''

async function get<T>(path: string): Promise<T> {
  if (!ADMIN_TOKEN) {
    throw new Error(
      'VITE_ADMIN_TOKEN is not set — run ./scripts/seed.sh, or start with VITE_USE_MOCKS=true',
    )
  }
  const res = await fetch(`/api${path}`, {
    headers: { Authorization: `Bearer ${ADMIN_TOKEN}` },
  })
  if (res.status === 401 || res.status === 403) {
    throw new Error(`GET ${path} → ${res.status}: admin token missing, expired or not an admin`)
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
