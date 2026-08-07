import { Badge, Table, Text } from '@radix-ui/themes'
import type { Order, OrderStatus } from '../api/types'

const statusColor: Record<OrderStatus, React.ComponentProps<typeof Badge>['color']> = {
  draft: 'gray',
  published: 'blue',
  matched: 'cyan',
  accepted: 'indigo',
  inProgress: 'violet',
  loadingInProgress: 'violet',
  inTransit: 'orange',
  delivered: 'teal',
  completed: 'green',
  cancelled: 'red',
  disputed: 'crimson',
}

function LegBadges({ order }: { order: Order }) {
  // Single-leg orders carry their leg's status in `status`; only combo
  // orders (transportWithOptions) have extra per-leg statuses.
  const legs: { label: string; status: OrderStatus }[] = []
  if (order.type === 'equipmentOnly') legs.push({ label: 'equipment', status: order.status })
  else if (order.type === 'laborOnly') legs.push({ label: 'labor', status: order.status })
  else legs.push({ label: 'transport', status: order.status })
  if (order.type === 'transportWithOptions') {
    if (order.equipmentStatus) legs.push({ label: 'equipment', status: order.equipmentStatus })
    if (order.laborStatus) legs.push({ label: 'labor', status: order.laborStatus })
  }
  return (
    <>
      {legs.map((leg) => (
        <Badge key={leg.label} color={statusColor[leg.status]} variant="soft" mr="1">
          {leg.label}: {leg.status}
        </Badge>
      ))}
    </>
  )
}

export function OrdersTable({
  orders,
  selectedId,
  onSelect,
}: {
  orders: Order[]
  selectedId: string | null
  onSelect: (id: string) => void
}) {
  return (
    <Table.Root size="1" variant="surface">
      <Table.Header>
        <Table.Row>
          <Table.ColumnHeaderCell>Client</Table.ColumnHeaderCell>
          <Table.ColumnHeaderCell>Type</Table.ColumnHeaderCell>
          <Table.ColumnHeaderCell>Legs / status</Table.ColumnHeaderCell>
          <Table.ColumnHeaderCell>Route</Table.ColumnHeaderCell>
          <Table.ColumnHeaderCell align="right">Price</Table.ColumnHeaderCell>
        </Table.Row>
      </Table.Header>
      <Table.Body>
        {orders.map((o) => (
          <Table.Row
            key={o.id}
            onClick={() => onSelect(o.id)}
            style={{
              cursor: 'pointer',
              background: o.id === selectedId ? 'var(--accent-a3)' : undefined,
            }}
          >
            <Table.Cell>{o.clientName}</Table.Cell>
            <Table.Cell>
              <Badge variant="outline">{o.type}</Badge>
            </Table.Cell>
            <Table.Cell>
              <LegBadges order={o} />
            </Table.Cell>
            <Table.Cell>
              <Text size="1" color="gray">
                {o.pickupAddress.split(',')[0]} → {o.dropoffAddress.split(',')[0]}
              </Text>
            </Table.Cell>
            <Table.Cell align="right">
              {Number(o.priceEstimate).toLocaleString('ru-RU')} {o.currency}
            </Table.Cell>
          </Table.Row>
        ))}
      </Table.Body>
    </Table.Root>
  )
}
