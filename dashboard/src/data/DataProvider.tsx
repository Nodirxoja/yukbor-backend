// One poll for the whole app.
//
// Every page needs some slice of the same four collections, and each page
// fetching for itself would mean four requests per page per ten seconds, plus a
// flash of empty state on every navigation. Loading once here means moving
// between pages is instant — the data is already in memory — and the server
// sees a constant, predictable load no matter how much someone clicks.

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react'
import type { AdminStats, Order, Transaction, User } from '../api/types'
import { ApiError } from '../api/auth'
import { fetchOrders, fetchStats, fetchTransactions, fetchUsers } from '../api/client'
import { useToast } from '../components/Toaster'

const POLL_MS = 10_000

interface DataState {
  stats: AdminStats | null
  orders: Order[]
  users: User[]
  transactions: Transaction[]
  /** True only for the very first load, so pages can show skeletons once. */
  initialising: boolean
  refreshing: boolean
  lastSync: Date | null
  refresh: () => void
  userById: (id: string | null | undefined) => User | undefined
  orderById: (id: string | null | undefined) => Order | undefined
}

const DataContext = createContext<DataState | null>(null)

export function useData(): DataState {
  const ctx = useContext(DataContext)
  if (!ctx) throw new Error('useData must be used inside <DataProvider>')
  return ctx
}

export function DataProvider({
  children,
  onUnauthorized,
}: {
  children: React.ReactNode
  onUnauthorized: (message: string) => void
}) {
  const toast = useToast()
  const [stats, setStats] = useState<AdminStats | null>(null)
  const [orders, setOrders] = useState<Order[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [transactions, setTransactions] = useState<Transaction[]>([])
  const [initialising, setInitialising] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [lastSync, setLastSync] = useState<Date | null>(null)
  const [nonce, setNonce] = useState(0)

  const refresh = useCallback(() => setNonce((n) => n + 1), [])

  useEffect(() => {
    let mounted = true

    const load = async () => {
      if (mounted) setRefreshing(true)
      try {
        const [s, o, u, t] = await Promise.all([
          fetchStats(),
          fetchOrders(),
          fetchUsers(),
          fetchTransactions(),
        ])
        if (!mounted) return
        setStats(s)
        setOrders(o)
        setUsers(u)
        setTransactions(t)
        setLastSync(new Date())
      } catch (e) {
        if (!mounted) return
        const err = e as ApiError
        // A dead session must return to sign-in, not leave a dashboard quietly
        // showing numbers that stopped moving.
        if (err.status === 401 || err.status === 403) {
          onUnauthorized(err.message)
          return
        }
        toast.error('Could not refresh', err.message)
      } finally {
        if (mounted) {
          setRefreshing(false)
          setInitialising(false)
        }
      }
    }

    void load()
    const timer = setInterval(load, POLL_MS)
    return () => {
      mounted = false
      clearInterval(timer)
    }
  }, [nonce, toast, onUnauthorized])

  // Indexes, so a detail page is a lookup rather than a scan.
  const userIndex = useMemo(() => new Map(users.map((u) => [u.id, u])), [users])
  const orderIndex = useMemo(() => new Map(orders.map((o) => [o.id, o])), [orders])

  const value = useMemo<DataState>(
    () => ({
      stats,
      orders,
      users,
      transactions,
      initialising,
      refreshing,
      lastSync,
      refresh,
      userById: (id) => (id ? userIndex.get(id) : undefined),
      orderById: (id) => (id ? orderIndex.get(id) : undefined),
    }),
    [stats, orders, users, transactions, initialising, refreshing, lastSync, refresh, userIndex, orderIndex],
  )

  return <DataContext.Provider value={value}>{children}</DataContext.Provider>
}
