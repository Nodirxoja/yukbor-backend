// Orders, filterable by what an operator actually asks: what is stuck, what is
// unclaimed, what is running.

import { useMemo, useState } from 'react'
import { Box, Card, Flex, Grid, SegmentedControl, Text, TextField } from '@radix-ui/themes'
import { MagnifyingGlassIcon } from '@radix-ui/react-icons'
import type { Order, OrderStatus } from '../api/types'
import { OrdersTable } from '../components/OrdersTable'

type Filter = 'all' | 'open' | 'running' | 'completed' | 'cancelled'

/** Legs an order actually has, so a combo order is judged on all of them. */
function legStatuses(o: Order): OrderStatus[] {
  const out: OrderStatus[] = [o.status]
  if (o.equipmentStatus) out.push(o.equipmentStatus)
  if (o.laborStatus) out.push(o.laborStatus)
  return out
}

function matches(o: Order, f: Filter): boolean {
  const legs = legStatuses(o)
  switch (f) {
    case 'open':
      // Anything still waiting for somebody to take it.
      return legs.some((s) => s === 'published' || s === 'draft' || s === 'matched')
    case 'running':
      return legs.some((s) =>
        ['accepted', 'inProgress', 'loadingInProgress', 'inTransit', 'delivered'].includes(s),
      )
    case 'completed':
      return legs.every((s) => s === 'completed')
    case 'cancelled':
      return legs.every((s) => s === 'cancelled')
    default:
      return true
  }
}

export function OrdersSection({
  orders,
  selectedId,
  onSelect,
}: {
  orders: Order[]
  selectedId: string | null
  onSelect: (id: string) => void
}) {
  const [filter, setFilter] = useState<Filter>('all')
  const [query, setQuery] = useState('')

  const counts = useMemo(() => {
    const c: Record<Filter, number> = { all: orders.length, open: 0, running: 0, completed: 0, cancelled: 0 }
    for (const o of orders) {
      for (const f of ['open', 'running', 'completed', 'cancelled'] as Filter[]) {
        if (matches(o, f)) c[f]++
      }
    }
    return c
  }, [orders])

  const totals = useMemo(() => {
    let value = 0
    let combo = 0
    for (const o of orders) {
      value += Number(o.priceEstimate) || 0
      if (o.type === 'transportWithOptions') combo++
    }
    return { value, combo }
  }, [orders])

  const shown = useMemo(() => {
    const q = query.trim().toLowerCase()
    return orders
      .filter((o) => matches(o, filter))
      .filter(
        (o) =>
          !q ||
          o.clientName.toLowerCase().includes(q) ||
          o.pickupAddress.toLowerCase().includes(q) ||
          o.dropoffAddress.toLowerCase().includes(q) ||
          (o.assignedDriverName ?? '').toLowerCase().includes(q),
      )
  }, [orders, filter, query])

  return (
    <Flex direction="column" gap="4">
      <Grid columns={{ initial: '2', sm: '4' }} gap="3">
        <Stat label="Total orders" value={String(counts.all)} />
        <Stat label="Awaiting an executor" value={String(counts.open)} accent="blue" />
        <Stat label="In progress" value={String(counts.running)} accent="orange" />
        <Stat
          label="Combined orders"
          value={String(totals.combo)}
          hint="transport + equipment/labor"
        />
      </Grid>

      <Card>
        <Flex direction="column" gap="3">
          <Flex justify="between" align="center" gap="3" wrap="wrap">
            <SegmentedControl.Root value={filter} onValueChange={(v) => setFilter(v as Filter)} size="1">
              <SegmentedControl.Item value="all">All ({counts.all})</SegmentedControl.Item>
              <SegmentedControl.Item value="open">Open ({counts.open})</SegmentedControl.Item>
              <SegmentedControl.Item value="running">Running ({counts.running})</SegmentedControl.Item>
              <SegmentedControl.Item value="completed">Done ({counts.completed})</SegmentedControl.Item>
              <SegmentedControl.Item value="cancelled">Cancelled ({counts.cancelled})</SegmentedControl.Item>
            </SegmentedControl.Root>

            <TextField.Root
              size="1"
              placeholder="Client, route, driver"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              style={{ minWidth: 240 }}
            >
              <TextField.Slot>
                <MagnifyingGlassIcon height="14" width="14" />
              </TextField.Slot>
            </TextField.Root>
          </Flex>

          <Box style={{ overflowX: 'auto' }}>
            <OrdersTable orders={shown} selectedId={selectedId} onSelect={onSelect} />
          </Box>

          {shown.length === 0 && (
            <Flex justify="center" py="6">
              <Text size="2" color="gray">
                {query ? `No orders match "${query}"` : 'Nothing in this state'}
              </Text>
            </Flex>
          )}
        </Flex>
      </Card>
    </Flex>
  )
}

function Stat({
  label,
  value,
  hint,
  accent,
}: {
  label: string
  value: string
  hint?: string
  accent?: 'blue' | 'orange'
}) {
  return (
    <Card>
      <Flex direction="column" gap="1">
        <Text size="1" color="gray">
          {label}
        </Text>
        <Text size="6" weight="bold" color={accent}>
          {value}
        </Text>
        {hint && (
          <Text size="1" color="gray">
            {hint}
          </Text>
        )}
      </Flex>
    </Card>
  )
}
