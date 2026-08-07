// Mission control: the money, the map, and what changed most recently.

import { Badge, Card, Flex, Grid, Heading, Text } from '@radix-ui/themes'
import type { AdminStats, Order, User } from '../api/types'
import { StatsCards } from '../components/StatsCards'
import { OrdersMap } from '../components/OrdersMap'

export function OverviewSection({
  stats,
  orders,
  users,
  selectedId,
  onSelect,
}: {
  stats: AdminStats | null
  orders: Order[]
  users: User[]
  selectedId: string | null
  onSelect: (id: string) => void
}) {
  // Executors currently carrying something — the number an operator watches.
  const onTheRoad = orders.filter((o) =>
    ['loadingInProgress', 'inTransit', 'inProgress'].includes(o.status),
  ).length
  const unclaimed = orders.filter((o) => o.status === 'published').length
  const pendingVerification = users.filter((u) => u.verificationStatus === 'pending').length
  const rejected = users.filter((u) => u.verificationStatus === 'rejected').length

  return (
    <Flex direction="column" gap="4">
      {stats && <StatsCards stats={stats} />}

      <Flex gap="2" wrap="wrap" className="animate-in">
        <Badge size="2" color="orange" variant="soft">
          {onTheRoad} on the road
        </Badge>
        <Badge size="2" color="blue" variant="soft">
          {unclaimed} waiting for an executor
        </Badge>
        {pendingVerification > 0 && (
          <Badge size="2" color="amber" variant="soft">
            {pendingVerification} awaiting verification
          </Badge>
        )}
        {rejected > 0 && (
          <Badge size="2" color="red" variant="soft">
            {rejected} rejected applicant{rejected === 1 ? '' : 's'}
          </Badge>
        )}
      </Flex>

      <Card data-static>
        <Flex direction="column" gap="2">
          <Flex justify="between" align="baseline">
            <Heading size="3">Live map</Heading>
            <Text size="1" color="gray">
              Solid lines are real road routes; dashed lines are straight-line estimates
            </Text>
          </Flex>
          <div style={{ height: 520 }}>
            <OrdersMap orders={orders} selectedId={selectedId} onSelect={onSelect} />
          </div>
        </Flex>
      </Card>

      <Grid columns={{ initial: '1', md: '2' }} gap="4" className="stagger">
        <RecentOrders orders={orders} onSelect={onSelect} />
        <RecentUsers users={users} />
      </Grid>
    </Flex>
  )
}

function RecentOrders({ orders, onSelect }: { orders: Order[]; onSelect: (id: string) => void }) {
  const recent = [...orders]
    .sort((a, b) => (b.createdAt ?? '').localeCompare(a.createdAt ?? ''))
    .slice(0, 6)

  return (
    <Card>
      <Flex direction="column" gap="3">
        <Heading size="3">Latest orders</Heading>
        {recent.map((o) => (
          <Flex
            key={o.id}
            justify="between"
            align="center"
            gap="3"
            onClick={() => onSelect(o.id)}
            style={{ cursor: 'pointer' }}
          >
            <Flex direction="column" style={{ minWidth: 0 }}>
              <Text size="2" truncate>
                {o.cargo?.cargoType ?? o.type}
              </Text>
              <Text size="1" color="gray" truncate>
                {o.pickupAddress.split(',')[0]} → {o.dropoffAddress.split(',')[0]}
              </Text>
            </Flex>
            <Flex align="center" gap="2">
              <Text size="1" color="gray">
                {Number(o.priceEstimate).toLocaleString('ru-RU')}
              </Text>
              <Badge variant="soft" size="1">
                {o.status}
              </Badge>
            </Flex>
          </Flex>
        ))}
        {recent.length === 0 && (
          <Text size="2" color="gray">
            No orders yet
          </Text>
        )}
      </Flex>
    </Card>
  )
}

function RecentUsers({ users }: { users: User[] }) {
  const recent = [...users]
    .sort((a, b) => (b.createdAt ?? '').localeCompare(a.createdAt ?? ''))
    .slice(0, 6)

  return (
    <Card>
      <Flex direction="column" gap="3">
        <Heading size="3">Newest sign-ups</Heading>
        {recent.map((u) => (
          <Flex key={u.id} justify="between" align="center" gap="3">
            <Flex direction="column" style={{ minWidth: 0 }}>
              <Text size="2" truncate>
                {u.fullName}
              </Text>
              <Text size="1" color="gray">
                {u.phoneNumber}
              </Text>
            </Flex>
            <Flex align="center" gap="2">
              <Badge variant="soft" size="1">
                {u.role}
              </Badge>
              {u.verificationStatus !== 'approved' && (
                <Badge
                  size="1"
                  variant="soft"
                  color={u.verificationStatus === 'rejected' ? 'red' : 'amber'}
                >
                  {u.verificationStatus}
                </Badge>
              )}
            </Flex>
          </Flex>
        ))}
        {recent.length === 0 && (
          <Text size="2" color="gray">
            Nobody registered yet
          </Text>
        )}
      </Flex>
    </Card>
  )
}
