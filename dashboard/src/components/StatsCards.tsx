import { Card, Flex, Grid, Text } from '@radix-ui/themes'
import type { AdminStats } from '../api/types'
import { CountUp, CountUpMoney } from '../hooks/useCountUp'

export function StatsCards({ stats }: { stats: AdminStats }) {
  const items: {
    label: string
    value: number
    money?: boolean
    accent?: 'green' | 'blue' | 'orange'
  }[] = [
    { label: 'Total orders', value: stats.totalOrders },
    { label: 'Active orders', value: stats.activeOrders, accent: 'orange' },
    { label: 'Completed', value: stats.completedOrders },
    { label: 'Registered users', value: stats.registeredUsers },
    { label: 'Credited to executors', value: Number(stats.creditedToExecutors), money: true, accent: 'green' },
    { label: 'Service fees charged', value: Number(stats.serviceFeesCharged), money: true, accent: 'green' },
    { label: 'Held in escrow', value: Number(stats.heldInEscrow), money: true, accent: 'blue' },
  ]

  return (
    <Grid columns={{ initial: '2', sm: '4', lg: '7' }} gap="3" className="stagger">
      {items.map((it) => (
        <Card key={it.label}>
          <Flex direction="column" gap="1">
            <Text size="1" color="gray">
              {it.label}
            </Text>
            {/* Tabular figures stop the number jittering as digits change. */}
            <Text
              size={it.money ? '4' : '6'}
              weight="bold"
              color={it.accent}
              style={{ fontVariantNumeric: 'tabular-nums' }}
            >
              {it.money ? <CountUpMoney value={it.value} /> : <CountUp value={it.value} />}
            </Text>
          </Flex>
        </Card>
      ))}
    </Grid>
  )
}
