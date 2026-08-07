import { useCallback, useEffect, useState } from 'react'
import {
  Avatar,
  Badge,
  Box,
  Button,
  Card,
  DropdownMenu,
  Flex,
  Grid,
  Heading,
  Tabs,
  Text,
} from '@radix-ui/themes'
import { ExitIcon, ReloadIcon } from '@radix-ui/react-icons'
import type { AdminStats, Order, User } from './api/types'
import { ApiError, loadSession, logout as endSession } from './api/auth'
import type { Session } from './api/auth'
import { fetchOrders, fetchStats, fetchUsers } from './api/client'
import { LoginScreen } from './components/LoginScreen'
import { useToast } from './components/Toaster'
import { StatsCards } from './components/StatsCards'
import { OrdersTable } from './components/OrdersTable'
import { UsersTable } from './components/UsersTable'
import { OrdersMap } from './components/OrdersMap'

const POLL_MS = 10_000 // back-office realtime: polling is fine (plan §11)
const USE_MOCKS = import.meta.env.VITE_USE_MOCKS !== 'false'

export default function App() {
  // Mock mode has no backend to sign in against, so it skips the gate.
  const [session, setSession] = useState<Session | null>(() => (USE_MOCKS ? null : loadSession()))
  const [authed, setAuthed] = useState(USE_MOCKS || Boolean(loadSession()))

  if (!authed) {
    return (
      <LoginScreen
        onSignedIn={(s) => {
          setSession(s)
          setAuthed(true)
        }}
      />
    )
  }
  return (
    <Dashboard
      session={session}
      onSignedOut={() => {
        endSession(session)
        setSession(null)
        setAuthed(false)
      }}
    />
  )
}

function Dashboard({
  session,
  onSignedOut,
}: {
  session: Session | null
  onSignedOut: () => void
}) {
  const toast = useToast()
  const [stats, setStats] = useState<AdminStats | null>(null)
  const [orders, setOrders] = useState<Order[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [selectedOrderId, setSelectedOrderId] = useState<string | null>(null)
  const [lastSync, setLastSync] = useState<Date | null>(null)
  const [loading, setLoading] = useState(true)

  const load = useCallback(
    async (alive: () => boolean) => {
      try {
        const [s, o, u] = await Promise.all([fetchStats(), fetchOrders(), fetchUsers()])
        if (!alive()) return
        setStats(s)
        setOrders(o)
        setUsers(u)
        setLastSync(new Date())
      } catch (e) {
        if (!alive()) return
        const err = e as ApiError
        // An expired session should return you to the sign-in screen, not
        // leave a dashboard quietly showing data that stopped updating.
        if (err.status === 401 || err.status === 403) {
          toast.error('Signed out', err.message)
          onSignedOut()
          return
        }
        toast.error('Could not refresh', err.message)
      } finally {
        if (alive()) setLoading(false)
      }
    },
    [toast, onSignedOut],
  )

  useEffect(() => {
    let mounted = true
    const alive = () => mounted
    void load(alive)
    const t = setInterval(() => void load(alive), POLL_MS)
    return () => {
      mounted = false
      clearInterval(t)
    }
  }, [load])

  const initials = session?.user.fullName
    ? session.user.fullName
        .split(' ')
        .slice(0, 2)
        .map((p) => p[0])
        .join('')
    : 'A'

  return (
    <Box p="4" style={{ maxWidth: 1400, margin: '0 auto' }}>
      <Flex justify="between" align="center" mb="4" gap="3">
        <Flex direction="column">
          <Heading size="6">YUK BOR</Heading>
          <Text size="1" color="gray">
            Admin dashboard
          </Text>
        </Flex>

        <Flex align="center" gap="3">
          <Flex align="center" gap="2">
            {loading ? (
              <Text size="1" color="gray">
                <ReloadIcon />
              </Text>
            ) : null}
            <Text size="1" color="gray">
              {USE_MOCKS
                ? 'mock data'
                : lastSync
                  ? `updated ${lastSync.toLocaleTimeString()}`
                  : 'connecting'}
            </Text>
          </Flex>

          {session && (
            <DropdownMenu.Root>
              <DropdownMenu.Trigger>
                <Button variant="soft" color="gray">
                  <Avatar size="1" fallback={initials} radius="full" />
                  {session.user.fullName}
                  <DropdownMenu.TriggerIcon />
                </Button>
              </DropdownMenu.Trigger>
              <DropdownMenu.Content>
                <DropdownMenu.Label>
                  <Flex direction="column" gap="1">
                    <Text size="1">{session.user.phoneNumber}</Text>
                    <Badge size="1" variant="soft">
                      {session.user.role}
                    </Badge>
                  </Flex>
                </DropdownMenu.Label>
                <DropdownMenu.Separator />
                <DropdownMenu.Item color="red" onSelect={onSignedOut}>
                  <ExitIcon />
                  Sign out
                </DropdownMenu.Item>
              </DropdownMenu.Content>
            </DropdownMenu.Root>
          )}
        </Flex>
      </Flex>

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
