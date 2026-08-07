import { Card, Flex, Grid, Text } from '@radix-ui/themes'
import type { AdminStats } from '../api/types'

function formatUZS(v: string): string {
  return `${Number(v).toLocaleString('ru-RU')} UZS`
}

export function StatsCards({ stats }: { stats: AdminStats }) {
  const items: { label: string; value: string; accent?: boolean }[] = [
    { label: 'Total orders', value: String(stats.totalOrders) },
    { label: 'Active orders', value: String(stats.activeOrders) },
    { label: 'Completed', value: String(stats.completedOrders) },
    { label: 'Registered users', value: String(stats.registeredUsers) },
    { label: 'Credited to executors', value: formatUZS(stats.creditedToExecutors), accent: true },
    { label: 'Service fees charged', value: formatUZS(stats.serviceFeesCharged), accent: true },
    { label: 'Held in escrow', value: formatUZS(stats.heldInEscrow) },
  ]
  return (
    <Grid columns={{ initial: '2', sm: '4', lg: '7' }} gap="3">
      {items.map((it) => (
        <Card key={it.label}>
          <Flex direction="column" gap="1">
            <Text size="1" color="gray">
              {it.label}
            </Text>
            <Text size="5" weight="bold" color={it.accent ? 'green' : undefined}>
              {it.value}
            </Text>
          </Flex>
        </Card>
      ))}
    </Grid>
  )
}
