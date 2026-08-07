import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  Badge,
  Button,
  Callout,
  Card,
  DataList,
  Flex,
  Grid,
  Heading,
  Separator,
  Table,
  Text,
} from '@radix-ui/themes'
import { ArrowLeftIcon, InfoCircledIcon } from '@radix-ui/react-icons'
import { useData } from '../data/DataProvider'
import { OrdersMap } from '../components/OrdersMap'
import { dateTime, legsOf, money, statusColor, txColor } from '../lib/format'

/**
 * One order, everything about it. This is what a URL per order buys: an
 * operator can send "look at this one" to a colleague, and support can open a
 * specific job without describing how to find it.
 */
export function OrderDetailPage() {
  const { id } = useParams<{ id: string }>()
  const { orderById, userById, transactions, initialising } = useData()
  const navigate = useNavigate()

  const order = orderById(id)

  if (initialising) return <Text size="2" color="gray">Loading…</Text>
  if (!order) {
    return (
      <Flex direction="column" gap="3" align="start">
        <Callout.Root color="amber">
          <Callout.Icon>
            <InfoCircledIcon />
          </Callout.Icon>
          <Callout.Text>No order with id {id}. It may have been removed.</Callout.Text>
        </Callout.Root>
        <Button variant="soft" onClick={() => navigate('/orders')}>
          <ArrowLeftIcon />
          Back to orders
        </Button>
      </Flex>
    )
  }

  const legs = legsOf(order)
  const orderTx = transactions.filter((t) => t.orderId === order.id)
  const executors = [
    { role: 'Driver', id: order.assignedDriverId, name: order.assignedDriverName },
    { role: 'Equipment', id: order.assignedEquipmentProviderId, name: order.assignedEquipmentProviderName },
    { role: 'Labor', id: order.assignedLaborProviderId, name: order.assignedLaborProviderName },
  ].filter((e) => e.id)

  return (
    <Flex direction="column" gap="4">
      <Flex justify="between" align="center" gap="3" wrap="wrap">
        <Flex align="center" gap="3">
          <Button size="1" variant="soft" color="gray" onClick={() => navigate('/orders')}>
            <ArrowLeftIcon />
            Orders
          </Button>
          <Heading size="4">{order.cargo?.cargoType ?? order.type}</Heading>
          {legs.map((l) => (
            <Badge key={l.label} variant="soft" color={statusColor(l.status)}>
              {l.label}: {l.status}
            </Badge>
          ))}
        </Flex>
        <Text size="4" weight="bold" style={{ fontVariantNumeric: 'tabular-nums' }}>
          {money(order.priceEstimate)} {order.currency}
        </Text>
      </Flex>

      <Grid columns={{ initial: '1', md: '3' }} gap="3">
        <Card size="1" style={{ gridColumn: 'span 1' }}>
          <Flex direction="column" gap="3">
            <Heading size="2">Order</Heading>
            <DataList.Root size="1">
              <DataList.Item>
                <DataList.Label>Client</DataList.Label>
                <DataList.Value>
                  <Link to={`/users/${order.clientId}`} className="inline-link">
                    {order.clientName}
                  </Link>
                </DataList.Value>
              </DataList.Item>
              <DataList.Item>
                <DataList.Label>Type</DataList.Label>
                <DataList.Value>{order.type}</DataList.Value>
              </DataList.Item>
              <DataList.Item>
                <DataList.Label>Scheduled</DataList.Label>
                <DataList.Value>{dateTime(order.scheduledDate)}</DataList.Value>
              </DataList.Item>
              <DataList.Item>
                <DataList.Label>Created</DataList.Label>
                <DataList.Value>{dateTime(order.createdAt)}</DataList.Value>
              </DataList.Item>
              <DataList.Item>
                <DataList.Label>Updated</DataList.Label>
                <DataList.Value>{dateTime(order.updatedAt)}</DataList.Value>
              </DataList.Item>
            </DataList.Root>

            {order.cargo && (
              <>
                <Separator size="4" />
                <Heading size="2">Cargo</Heading>
                <DataList.Root size="1">
                  <DataList.Item>
                    <DataList.Label>Weight</DataList.Label>
                    <DataList.Value>{order.cargo.weightTons} t</DataList.Value>
                  </DataList.Item>
                  <DataList.Item>
                    <DataList.Label>Vehicle</DataList.Label>
                    <DataList.Value>{order.cargo.requiredVehicleType}</DataList.Value>
                  </DataList.Item>
                  {order.cargo.requiresRefrigeration && (
                    <DataList.Item>
                      <DataList.Label>Cooling</DataList.Label>
                      <DataList.Value>
                        <Badge size="1" color="cyan" variant="soft">
                          refrigerated
                        </Badge>
                      </DataList.Value>
                    </DataList.Item>
                  )}
                  {order.cargo.specialInstructions && (
                    <DataList.Item>
                      <DataList.Label>Notes</DataList.Label>
                      <DataList.Value>{order.cargo.specialInstructions}</DataList.Value>
                    </DataList.Item>
                  )}
                </DataList.Root>
              </>
            )}

            {order.equipmentRequest && (
              <>
                <Separator size="4" />
                <Heading size="2">Equipment</Heading>
                <DataList.Root size="1">
                  <DataList.Item>
                    <DataList.Label>Machine</DataList.Label>
                    <DataList.Value>{order.equipmentRequest.equipmentType}</DataList.Value>
                  </DataList.Item>
                  <DataList.Item>
                    <DataList.Label>Hours</DataList.Label>
                    <DataList.Value>{order.equipmentRequest.durationHours}</DataList.Value>
                  </DataList.Item>
                </DataList.Root>
              </>
            )}

            {order.laborRequest && (
              <>
                <Separator size="4" />
                <Heading size="2">Labor</Heading>
                <DataList.Root size="1">
                  <DataList.Item>
                    <DataList.Label>Workers</DataList.Label>
                    <DataList.Value>{order.laborRequest.workersCount}</DataList.Value>
                  </DataList.Item>
                  <DataList.Item>
                    <DataList.Label>Hours</DataList.Label>
                    <DataList.Value>{order.laborRequest.durationHours}</DataList.Value>
                  </DataList.Item>
                </DataList.Root>
              </>
            )}
          </Flex>
        </Card>

        <Card data-static size="1" style={{ gridColumn: 'span 2' }}>
          <Flex direction="column" gap="2" style={{ height: '100%' }}>
            <Flex justify="between" align="baseline" px="1">
              <Heading size="2">Route</Heading>
              <Text size="1" color="gray" truncate>
                {order.pickupAddress} → {order.dropoffAddress}
              </Text>
            </Flex>
            <div style={{ height: 380 }}>
              <OrdersMap orders={[order]} selectedId={order.id} onSelect={() => {}} />
            </div>
          </Flex>
        </Card>
      </Grid>

      <Grid columns={{ initial: '1', md: '2' }} gap="3">
        <Card size="1">
          <Flex direction="column" gap="3">
            <Heading size="2">Executors</Heading>
            {executors.length === 0 && (
              <Text size="2" color="gray">
                Nobody has taken this order yet
              </Text>
            )}
            {executors.map((e) => {
              const u = userById(e.id)
              return (
                <Flex key={e.role} justify="between" align="center" gap="3">
                  <Flex direction="column" style={{ minWidth: 0 }}>
                    <Link to={`/users/${e.id}`} className="inline-link">
                      <Text size="2">{e.name}</Text>
                    </Link>
                    <Text size="1" color="gray">
                      {e.role}
                      {u?.vehiclePlate ? ` · ${u.vehiclePlate}` : ''}
                    </Text>
                  </Flex>
                  {u?.licenseCategories?.length ? (
                    <Badge size="1" variant="soft">
                      {u.licenseCategories.join('/')}
                    </Badge>
                  ) : null}
                </Flex>
              )
            })}
          </Flex>
        </Card>

        <Card size="1">
          <Flex direction="column" gap="3">
            <Heading size="2">Escrow</Heading>
            {orderTx.length === 0 ? (
              <Text size="2" color="gray">
                No money held for this order yet
              </Text>
            ) : (
              <Table.Root size="1" variant="ghost">
                <Table.Header>
                  <Table.Row>
                    <Table.ColumnHeaderCell>Payee</Table.ColumnHeaderCell>
                    <Table.ColumnHeaderCell align="right">Amount</Table.ColumnHeaderCell>
                    <Table.ColumnHeaderCell align="right">Fee</Table.ColumnHeaderCell>
                    <Table.ColumnHeaderCell>State</Table.ColumnHeaderCell>
                  </Table.Row>
                </Table.Header>
                <Table.Body>
                  {orderTx.map((t) => (
                    <Table.Row key={t.id}>
                      <Table.Cell>
                        <Link to={`/users/${t.payeeId}`} className="inline-link">
                          <Text size="1">{userById(t.payeeId)?.fullName ?? t.payeeId.slice(0, 8)}</Text>
                        </Link>
                      </Table.Cell>
                      <Table.Cell align="right">
                        <Text size="1" style={{ fontVariantNumeric: 'tabular-nums' }}>
                          {money(t.amount)}
                        </Text>
                      </Table.Cell>
                      <Table.Cell align="right">
                        <Text size="1" color="gray" style={{ fontVariantNumeric: 'tabular-nums' }}>
                          {money(t.platformCommission)}
                        </Text>
                      </Table.Cell>
                      <Table.Cell>
                        <Badge size="1" variant="soft" color={txColor[t.status]}>
                          {t.status}
                        </Badge>
                      </Table.Cell>
                    </Table.Row>
                  ))}
                </Table.Body>
              </Table.Root>
            )}
          </Flex>
        </Card>
      </Grid>
    </Flex>
  )
}
