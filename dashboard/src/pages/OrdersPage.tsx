import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Badge,
  Box,
  Card,
  Flex,
  Grid,
  SegmentedControl,
  Skeleton,
  Table,
  Text,
  TextField,
} from '@radix-ui/themes'
import { MagnifyingGlassIcon } from '@radix-ui/react-icons'
import { useData } from '../data/DataProvider'
import { Metric } from '../components/Metric'
import { legsOf, money, relative, statusColor } from '../lib/format'
import type { Order } from '../api/types'

type Filter = 'all' | 'open' | 'running' | 'done' | 'cancelled'

function inFilter(o: Order, f: Filter): boolean {
  const legs = legsOf(o).map((l) => l.status)
  switch (f) {
    case 'open':
      return legs.some((s) => ['published', 'draft', 'matched'].includes(s))
    case 'running':
      return legs.some((s) =>
        ['accepted', 'inProgress', 'loadingInProgress', 'inTransit', 'delivered'].includes(s),
      )
    case 'done':
      return legs.every((s) => s === 'completed')
    case 'cancelled':
      return legs.every((s) => s === 'cancelled')
    default:
      return true
  }
}

export function OrdersPage() {
  const { orders, initialising } = useData()
  const navigate = useNavigate()
  const [filter, setFilter] = useState<Filter>('all')
  const [query, setQuery] = useState('')

  const counts = useMemo(() => {
    const c: Record<Filter, number> = { all: orders.length, open: 0, running: 0, done: 0, cancelled: 0 }
    for (const o of orders) {
      for (const f of ['open', 'running', 'done', 'cancelled'] as Filter[]) {
        if (inFilter(o, f)) c[f]++
      }
    }
    return c
  }, [orders])

  const value = useMemo(
    () => orders.reduce((sum, o) => sum + (Number(o.priceEstimate) || 0), 0),
    [orders],
  )

  const shown = useMemo(() => {
    const q = query.trim().toLowerCase()
    return orders
      .filter((o) => inFilter(o, filter))
      .filter(
        (o) =>
          !q ||
          o.clientName.toLowerCase().includes(q) ||
          o.pickupAddress.toLowerCase().includes(q) ||
          o.dropoffAddress.toLowerCase().includes(q) ||
          (o.cargo?.cargoType ?? '').toLowerCase().includes(q) ||
          (o.assignedDriverName ?? '').toLowerCase().includes(q),
      )
  }, [orders, filter, query])

  if (initialising) return <TableSkeleton />

  return (
    <Flex direction="column" gap="4">
      <Grid columns={{ initial: '2', sm: '4' }} gap="2" className="stagger">
        <Metric label="Orders" value={counts.all} hint="all time" />
        <Metric label="Unclaimed" value={counts.open} accent="blue" hint="need an executor" />
        <Metric label="Running" value={counts.running} accent="orange" hint="in flight" />
        <Metric label="Completed" value={counts.done} accent="green" hint={`${money(value)} UZS booked`} />
      </Grid>

      <Card size="1">
        <Flex direction="column" gap="3">
          <Flex justify="between" align="center" gap="3" wrap="wrap">
            <SegmentedControl.Root value={filter} onValueChange={(v) => setFilter(v as Filter)} size="1">
              <SegmentedControl.Item value="all">All</SegmentedControl.Item>
              <SegmentedControl.Item value="open">Open ({counts.open})</SegmentedControl.Item>
              <SegmentedControl.Item value="running">Running ({counts.running})</SegmentedControl.Item>
              <SegmentedControl.Item value="done">Done ({counts.done})</SegmentedControl.Item>
              <SegmentedControl.Item value="cancelled">Cancelled ({counts.cancelled})</SegmentedControl.Item>
            </SegmentedControl.Root>

            <TextField.Root
              size="1"
              placeholder="Client, cargo, route, driver"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              style={{ minWidth: 260 }}
            >
              <TextField.Slot>
                <MagnifyingGlassIcon height="14" width="14" />
              </TextField.Slot>
            </TextField.Root>
          </Flex>

          <Box style={{ overflowX: 'auto' }}>
            <Table.Root size="1" variant="surface">
              <Table.Header>
                <Table.Row>
                  <Table.ColumnHeaderCell>Cargo</Table.ColumnHeaderCell>
                  <Table.ColumnHeaderCell>Client</Table.ColumnHeaderCell>
                  <Table.ColumnHeaderCell>Route</Table.ColumnHeaderCell>
                  <Table.ColumnHeaderCell>Legs</Table.ColumnHeaderCell>
                  <Table.ColumnHeaderCell align="right">Price</Table.ColumnHeaderCell>
                  <Table.ColumnHeaderCell align="right">Created</Table.ColumnHeaderCell>
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {shown.map((o) => (
                  <Table.Row
                    key={o.id}
                    className="row-link"
                    onClick={() => navigate(`/orders/${o.id}`)}
                  >
                    <Table.Cell>
                      <Text size="2">{o.cargo?.cargoType ?? o.type}</Text>
                    </Table.Cell>
                    <Table.Cell>
                      <Text size="2">{o.clientName}</Text>
                    </Table.Cell>
                    <Table.Cell>
                      <Text size="1" color="gray">
                        {o.pickupAddress.split(',')[0]} → {o.dropoffAddress.split(',')[0]}
                      </Text>
                    </Table.Cell>
                    <Table.Cell>
                      <Flex gap="1" wrap="wrap">
                        {legsOf(o).map((l) => (
                          <Badge key={l.label} size="1" variant="soft" color={statusColor(l.status)}>
                            {l.label}: {l.status}
                          </Badge>
                        ))}
                      </Flex>
                    </Table.Cell>
                    <Table.Cell align="right">
                      <Text size="2" style={{ fontVariantNumeric: 'tabular-nums' }}>
                        {money(o.priceEstimate)}
                      </Text>
                    </Table.Cell>
                    <Table.Cell align="right">
                      <Text size="1" color="gray">
                        {relative(o.createdAt)}
                      </Text>
                    </Table.Cell>
                  </Table.Row>
                ))}
              </Table.Body>
            </Table.Root>
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

export function TableSkeleton() {
  return (
    <Flex direction="column" gap="4">
      <Grid columns={{ initial: '2', sm: '4' }} gap="2">
        {Array.from({ length: 4 }, (_, i) => (
          <Skeleton key={i} height="72px" />
        ))}
      </Grid>
      <Skeleton height="420px" />
    </Flex>
  )
}
