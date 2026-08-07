import { useEffect, useState } from 'react'
import { Box, Card, Flex, Grid, Heading, Tabs, Text } from '@radix-ui/themes'
import type { AdminStats, Order, User } from './api/types'
import { fetchOrders, fetchStats, fetchUsers } from './api/client'
import { StatsCards } from './components/StatsCards'
import { OrdersTable } from './components/OrdersTable'
import { UsersTable } from './components/UsersTable'
import { OrdersMap } from './components/OrdersMap'

const POLL_MS = 10_000 // back-office realtime: polling is fine (plan §11)

export default function App() {
  const [stats, setStats] = useState<AdminStats | null>(null)
  const [orders, setOrders] = useState<Order[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [selectedOrderId, setSelectedOrderId] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let alive = true
    const load = () =>
      Promise.all([fetchStats(), fetchOrders(), fetchUsers()])
        .then(([s, o, u]) => {
          if (!alive) return
          setStats(s)
          setOrders(o)
          setUsers(u)
          setError(null)
        })
        .catch((e: unknown) => alive && setError(String(e)))
    load()
    const t = setInterval(load, POLL_MS)
    return () => {
      alive = false
      clearInterval(t)
    }
  }, [])

  return (
    <Box p="4" style={{ maxWidth: 1400, margin: '0 auto' }}>
      <Flex justify="between" align="center" mb="4">
        <Heading size="6">YUK BOR — Admin Dashboard</Heading>
        <Text size="1" color="gray">
          {import.meta.env.VITE_USE_MOCKS !== 'false' ? 'mock data' : 'live'} · refresh {POLL_MS / 1000}s
        </Text>
      </Flex>

      {error && (
        <Text color="red" size="2">
          {error}
        </Text>
      )}

      {stats && (
        <Box mb="4">
          <StatsCards stats={stats} />
        </Box>
      )}

      <Grid columns={{ initial: '1', md: '2' }} gap="4">
        <Card style={{ height: 480 }}>
          <OrdersMap orders={orders} selectedId={selectedOrderId} onSelect={setSelectedOrderId} />
        </Card>
        <Card>
          <Tabs.Root defaultValue="orders">
            <Tabs.List>
              <Tabs.Trigger value="orders">Orders ({orders.length})</Tabs.Trigger>
              <Tabs.Trigger value="users">Users ({users.length})</Tabs.Trigger>
            </Tabs.List>
            <Box pt="3">
              <Tabs.Content value="orders">
                <OrdersTable
                  orders={orders}
                  selectedId={selectedOrderId}
                  onSelect={setSelectedOrderId}
                />
              </Tabs.Content>
              <Tabs.Content value="users">
                <UsersTable users={users} />
              </Tabs.Content>
            </Box>
          </Tabs.Root>
        </Card>
      </Grid>
    </Box>
  )
}
