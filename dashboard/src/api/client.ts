// API client with a mock mode (default ON) so the dashboard renders whole
// without the backend. Set VITE_USE_MOCKS=false to hit the real gateway via
// the /api dev proxy (see vite.config.ts).
import type { AdminStats, Order, User } from './types'
import { computeStats, mockOrders, mockUsers } from '../mocks/data'

const USE_MOCKS = import.meta.env.VITE_USE_MOCKS !== 'false'

async function get<T>(path: string): Promise<T> {
  const res = await fetch(`/api${path}`, {
    headers: {
      // TODO(day-3): real admin login flow; for the hackathon a seeded
      // admin JWT in localStorage-free memory or an env var is enough.
      Authorization: `Bearer ${import.meta.env.VITE_ADMIN_TOKEN ?? ''}`,
    },
  })
  if (!res.ok) throw new Error(`GET ${path} → ${res.status}`)
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
