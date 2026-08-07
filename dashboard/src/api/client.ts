// API client with a mock mode (default ON) so the dashboard renders whole
// without the backend. Set VITE_USE_MOCKS=false to hit the real gateway via
// the /api dev proxy (see vite.config.ts).
import type { AdminStats, Order, User } from './types'
import { ApiError, loadSession } from './auth'
import { computeStats, mockOrders, mockUsers } from '../mocks/data'

const USE_MOCKS = import.meta.env.VITE_USE_MOCKS !== 'false'

/**
 * Every admin request carries the token of the signed-in administrator — the
 * one obtained by them logging in, held in sessionStorage for this tab only.
 *
 * Nothing is baked in at build time: Vite inlines VITE_* values into a bundle
 * that is served publicly, so a token embedded there is a token given away.
 */
async function get<T>(path: string): Promise<T> {
  const session = loadSession()
  if (!session) {
    throw new ApiError('UNAUTHORIZED', 'Your session has ended. Sign in again.', 401)
  }

  let res: Response
  try {
    res = await fetch(`/api${path}`, {
      headers: { Authorization: `Bearer ${session.accessToken}` },
    })
  } catch {
    throw new ApiError('NETWORK', 'Cannot reach the server.', 0)
  }

  if (res.status === 401 || res.status === 403) {
    throw new ApiError(
      'UNAUTHORIZED',
      res.status === 403
        ? 'This account is not an administrator.'
        : 'Your session has expired. Sign in again.',
      res.status,
    )
  }
  if (!res.ok) {
    throw new ApiError('REQUEST_FAILED', `Could not load data (${res.status}).`, res.status)
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
