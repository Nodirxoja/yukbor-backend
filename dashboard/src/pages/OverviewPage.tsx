import { useNavigate } from 'react-router-dom'
import { Badge, Card, Flex, Grid, Heading, Skeleton, Text } from '@radix-ui/themes'
import { useData } from '../data/DataProvider'
import { Metric, MoneyMetric } from '../components/Metric'
import { OrdersMap } from '../components/OrdersMap'
import { statusColor } from '../lib/format'

export function OverviewPage() {
  const { stats, orders, users, initialising } = useData()
  const navigate = useNavigate()

  const onTheRoad = orders.filter((o) =>
    ['loadingInProgress', 'inTransit', 'inProgress'].includes(o.status),
  ).length
  const unclaimed = orders.filter((o) => o.status === 'published').length
  const rejected = users.filter((u) => u.verificationStatus === 'rejected').length

  if (initialising) return <OverviewSkeleton />

  return (
    <Flex direction="column" gap="4">
      <Grid columns={{ initial: '2', sm: '4', lg: '7' }} gap="2" className="stagger">
        <Metric label="Orders" value={stats?.totalOrders ?? 0} hint="all time" />
        <Metric label="Active" value={stats?.activeOrders ?? 0} accent="orange" hint="in flight" />
        <Metric label="On the road" value={onTheRoad} accent="orange" hint="carrying now" />
        <Metric label="Unclaimed" value={unclaimed} accent="blue" hint="need an executor" />
        <Metric label="People" value={stats?.registeredUsers ?? 0} hint={`${rejected} rejected`} />
        <MoneyMetric
          label="Paid out"
          value={Number(stats?.creditedToExecutors ?? 0)}
          accent="green"
          hint="to executors"
        />
        <MoneyMetric
          label="In escrow"
          value={Number(stats?.heldInEscrow ?? 0)}
          accent="blue"
          hint="held"
        />
      </Grid>

      <Card data-static size="1">
        <Flex direction="column" gap="2">
          <Flex justify="between" align="baseline" px="1">
            <Heading size="2">Live map</Heading>
            <Text size="1" color="gray">
              Solid lines are road routes; dashed are straight-line estimates
            </Text>
          </Flex>
          <div style={{ height: 460 }}>
            <OrdersMap
              orders={orders}
              selectedId={null}
              onSelect={(id) => navigate(`/orders/${id}`)}
            />
          </div>
        </Flex>
      </Card>

      <Grid columns={{ initial: '1', md: '2' }} gap="3" className="stagger">
        <Card size="1">
          <Flex direction="column" gap="2">
            <Heading size="2" mb="1">
              Latest orders
            </Heading>
            {orders.slice(0, 7).map((o) => (
              <Flex
                key={o.id}
                justify="between"
                align="center"
                gap="3"
                className="row-link"
                onClick={() => navigate(`/orders/${o.id}`)}
              >
                <Flex direction="column" style={{ minWidth: 0 }}>
                  <Text size="2" truncate>
                    {o.cargo?.cargoType ?? o.type}
                  </Text>
                  <Text size="1" color="gray" truncate>
                    {o.pickupAddress.split(',')[0]} → {o.dropoffAddress.split(',')[0]}
                  </Text>
                </Flex>
                <Badge size="1" variant="soft" color={statusColor(o.status)}>
                  {o.status}
                </Badge>
              </Flex>
            ))}
            {orders.length === 0 && (
              <Text size="2" color="gray">
                No orders yet
              </Text>
            )}
          </Flex>
        </Card>

        <Card size="1">
          <Flex direction="column" gap="2">
            <Heading size="2" mb="1">
              Newest people
            </Heading>
            {users.slice(0, 7).map((u) => (
              <Flex
                key={u.id}
                justify="between"
                align="center"
                gap="3"
                className="row-link"
                onClick={() => navigate(`/users/${u.id}`)}
              >
                <Flex direction="column" style={{ minWidth: 0 }}>
                  <Text size="2" truncate>
                    {u.fullName}
                  </Text>
                  <Text size="1" color="gray">
                    {u.phoneNumber}
                  </Text>
                </Flex>
                <Flex gap="2" align="center">
                  <Badge size="1" variant="soft">
                    {u.role}
                  </Badge>
                  {u.verificationStatus === 'rejected' && (
                    <Badge size="1" variant="soft" color="red">
                      rejected
                    </Badge>
                  )}
                </Flex>
              </Flex>
            ))}
            {users.length === 0 && (
              <Text size="2" color="gray">
                Nobody registered yet
              </Text>
            )}
          </Flex>
        </Card>
      </Grid>
    </Flex>
  )
}

/* Skeletons rather than a spinner: the page keeps its shape while it loads, so
   nothing jumps when the data lands. */
function OverviewSkeleton() {
  return (
    <Flex direction="column" gap="4">
      <Grid columns={{ initial: '2', sm: '4', lg: '7' }} gap="2">
        {Array.from({ length: 7 }, (_, i) => (
          <Card size="1" key={i}>
            <Flex direction="column" gap="2">
              <Skeleton height="10px" width="60%" />
              <Skeleton height="22px" width="80%" />
              <Skeleton height="10px" width="50%" />
            </Flex>
          </Card>
        ))}
      </Grid>
      <Skeleton height="460px" />
      <Grid columns={{ initial: '1', md: '2' }} gap="3">
        <Skeleton height="220px" />
        <Skeleton height="220px" />
      </Grid>
    </Flex>
  )
}
