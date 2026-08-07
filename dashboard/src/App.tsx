import { useCallback, useEffect, useState } from 'react'
import {
  Avatar,
  Badge,
  Box,
  Button,
  Container,
  DropdownMenu,
  Flex,
  Heading,
  Spinner,
  TabNav,
  Text,
} from '@radix-ui/themes'
import { CubeIcon, DashboardIcon, ExitIcon, PersonIcon } from '@radix-ui/react-icons'
import type { AdminStats, Order, User } from './api/types'
import { ApiError, loadSession, logout as endSession } from './api/auth'
import type { Session } from './api/auth'
import { fetchOrders, fetchStats, fetchUsers } from './api/client'
import { LoginScreen } from './components/LoginScreen'
import { useToast } from './components/Toaster'
import { OverviewSection } from './sections/OverviewSection'
import { OrdersSection } from './sections/OrdersSection'
import { UsersSection } from './sections/UsersSection'

const POLL_MS = 10_000 // back-office realtime: polling is fine (plan §11)
// Opt-IN, never opt-out: a build with no flag talks to the real backend.
const USE_MOCKS = import.meta.env.VITE_USE_MOCKS === 'true'

type Tab = 'overview' | 'orders' | 'users'

const TABS: { id: Tab; label: string; icon: typeof DashboardIcon }[] = [
  { id: 'overview', label: 'Overview', icon: DashboardIcon },
  { id: 'orders', label: 'Orders', icon: CubeIcon },
  { id: 'users', label: 'Users', icon: PersonIcon },
]

export default function App() {
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

function Dashboard({ session, onSignedOut }: { session: Session | null; onSignedOut: () => void }) {
  const toast = useToast()
  const [tab, setTab] = useState<Tab>('overview')
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
        // An expired session should return you to sign-in, not leave a
        // dashboard quietly showing data that stopped updating.
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

  // Clicking an order on the map or in a list should take you to it.
  const selectOrder = (id: string) => {
    setSelectedOrderId(id)
    if (tab === 'overview') setTab('orders')
  }

  const initials = session?.user.fullName
    ? session.user.fullName
        .split(' ')
        .slice(0, 2)
        .map((p) => p[0])
        .join('')
    : 'A'

  return (
    <Box>
      {/* Sticky bar: the identity and navigation stay put while a long table
          scrolls underneath. */}
      <Box
        style={{
          position: 'sticky',
          top: 0,
          zIndex: 10,
          backdropFilter: 'blur(12px)',
          borderBottom: '1px solid var(--gray-a5)',
        }}
      >
        <Container size="4" px="4">
          <Flex justify="between" align="center" py="3" gap="3">
            <Flex align="center" gap="4">
              <Flex direction="column">
                <Heading size="5">YUK BOR</Heading>
                <Text size="1" color="gray">
                  Admin dashboard
                </Text>
              </Flex>
            </Flex>

            <Flex align="center" gap="3">
              <Flex align="center" gap="2">
                {loading && <Spinner size="1" />}
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

          <TabNav.Root>
            {TABS.map(({ id, label, icon: Icon }) => (
              <TabNav.Link key={id} active={tab === id} onClick={() => setTab(id)} href="#">
                <Flex align="center" gap="2">
                  <Icon />
                  {label}
                  {id === 'orders' && orders.length > 0 && (
                    <Badge size="1" variant="soft" color="gray">
                      {orders.length}
                    </Badge>
                  )}
                  {id === 'users' && users.length > 0 && (
                    <Badge size="1" variant="soft" color="gray">
                      {users.length}
                    </Badge>
                  )}
                </Flex>
              </TabNav.Link>
            ))}
          </TabNav.Root>
        </Container>
      </Box>

      <Container size="4" px="4" py="4">
        {/* key by tab so React remounts and the entrance animation replays */}
        <div key={tab} className="animate-in">
        {tab === 'overview' && (
          <OverviewSection
            stats={stats}
            orders={orders}
            users={users}
            selectedId={selectedOrderId}
            onSelect={selectOrder}
          />
        )}
        {tab === 'orders' && (
          <OrdersSection
            orders={orders}
            selectedId={selectedOrderId}
            onSelect={setSelectedOrderId}
          />
        )}
        {tab === 'users' && <UsersSection users={users} orders={orders} />}
        </div>
      </Container>
    </Box>
  )
}
