import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  Avatar,
  Badge,
  Button,
  Callout,
  Card,
  DataList,
  Flex,
  Grid,
  Heading,
  Table,
  Text,
} from '@radix-ui/themes'
import { ArrowLeftIcon, InfoCircledIcon } from '@radix-ui/react-icons'
import { useData } from '../data/DataProvider'
import { Metric, MoneyMetric } from '../components/Metric'
import { capability, capabilityLabel, deriveActivity } from './UsersPage'
import { dateOnly, money, roleColor, statusColor, txColor, verificationColor } from '../lib/format'

export function UserDetailPage() {
  const { id } = useParams<{ id: string }>()
  const { userById, orders, transactions, initialising } = useData()
  const navigate = useNavigate()

  const user = userById(id)

  if (initialising) return <Text size="2" color="gray">Loading…</Text>
  if (!user) {
    return (
      <Flex direction="column" gap="3" align="start">
        <Callout.Root color="amber">
          <Callout.Icon>
            <InfoCircledIcon />
          </Callout.Icon>
          <Callout.Text>No person with id {id}.</Callout.Text>
        </Callout.Root>
        <Button variant="soft" onClick={() => navigate('/users')}>
          <ArrowLeftIcon />
          Back to people
        </Button>
      </Flex>
    )
  }

  const activity = deriveActivity(orders)[user.id]
  const isExecutor = user.role !== 'client' && user.role !== 'admin' && user.role !== 'fleetAdmin'

  // Everything this person touched, from whichever side.
  const theirOrders = orders.filter(
    (o) =>
      o.clientId === user.id ||
      o.assignedDriverId === user.id ||
      o.assignedEquipmentProviderId === user.id ||
      o.assignedLaborProviderId === user.id,
  )
  const theirTx = transactions.filter((t) => t.payeeId === user.id || t.payerId === user.id)
  const earned = theirTx
    .filter((t) => t.payeeId === user.id && t.status === 'released')
    .reduce((s, t) => s + (Number(t.amount) - Number(t.platformCommission)), 0)
  const pending = theirTx
    .filter((t) => t.payeeId === user.id && t.status === 'held')
    .reduce((s, t) => s + Number(t.amount), 0)

  const initials = user.fullName.split(' ').slice(0, 2).map((p) => p[0]).join('')
  const cap = capabilityLabel[capability(user)]

  return (
    <Flex direction="column" gap="4">
      <Flex align="center" gap="3" wrap="wrap">
        <Button size="1" variant="soft" color="gray" onClick={() => navigate('/users')}>
          <ArrowLeftIcon />
          People
        </Button>
        <Avatar size="3" fallback={initials} radius="full" />
        <Flex direction="column">
          <Heading size="4">{user.fullName}</Heading>
          <Text size="1" color="gray">
            {user.phoneNumber}
          </Text>
        </Flex>
        <Badge color={roleColor[user.role]} variant="soft">
          {user.role}
        </Badge>
        <Badge color={verificationColor[user.verificationStatus]} variant="soft">
          {user.verificationStatus}
        </Badge>
        {user.role === 'driver' && (
          <Badge color={cap.color} variant="soft">
            {cap.text}
          </Badge>
        )}
      </Flex>

      {user.rejectionReason && (
        <Callout.Root color="red" size="1">
          <Callout.Icon>
            <InfoCircledIcon />
          </Callout.Icon>
          <Callout.Text>
            Registration was refused: <strong>{user.rejectionReason}</strong>
          </Callout.Text>
        </Callout.Root>
      )}

      <Grid columns={{ initial: '2', sm: '4' }} gap="2" className="stagger">
        {user.role === 'client' ? (
          <>
            <Metric label="Orders placed" value={activity?.placed ?? 0} />
            <Metric label="Completed" value={activity?.completed ?? 0} accent="green" />
            <MoneyMetric label="Total ordered" value={activity?.value ?? 0} />
            <Metric label="Rating" value={user.rating} hint={`${user.ratingsCount} reviews`} />
          </>
        ) : (
          <>
            <Metric label="Jobs taken" value={activity?.jobs ?? 0} />
            <Metric label="Completed" value={activity?.completed ?? 0} accent="green" />
            <MoneyMetric label="Earned" value={earned} accent="green" hint="after commission" />
            <MoneyMetric label="In escrow" value={pending} accent="blue" hint="not yet released" />
          </>
        )}
      </Grid>

      <Grid columns={{ initial: '1', md: '3' }} gap="3">
        <Card size="1">
          <Flex direction="column" gap="3">
            <Heading size="2">Profile</Heading>
            <DataList.Root size="1">
              <DataList.Item>
                <DataList.Label>Joined</DataList.Label>
                <DataList.Value>{dateOnly(user.createdAt)}</DataList.Value>
              </DataList.Item>
              <DataList.Item>
                <DataList.Label>Verified</DataList.Label>
                <DataList.Value>{user.isVerified ? 'yes' : 'no'}</DataList.Value>
              </DataList.Item>
              <DataList.Item>
                <DataList.Label>Rating</DataList.Label>
                <DataList.Value>
                  {user.ratingsCount ? `${user.rating.toFixed(1)} of 5 · ${user.ratingsCount}` : '—'}
                </DataList.Value>
              </DataList.Item>
              {isExecutor && user.licenseNumber && (
                <>
                  <DataList.Item>
                    <DataList.Label>Licence</DataList.Label>
                    <DataList.Value>{user.licenseNumber}</DataList.Value>
                  </DataList.Item>
                  <DataList.Item>
                    <DataList.Label>Categories</DataList.Label>
                    <DataList.Value>
                      <Flex gap="1">
                        {(user.licenseCategories ?? []).map((c) => (
                          <Badge key={c} size="1" variant="soft">
                            {c}
                          </Badge>
                        ))}
                      </Flex>
                    </DataList.Value>
                  </DataList.Item>
                  {user.vehiclePlate && (
                    <DataList.Item>
                      <DataList.Label>Plate</DataList.Label>
                      <DataList.Value>{user.vehiclePlate}</DataList.Value>
                    </DataList.Item>
                  )}
                </>
              )}
            </DataList.Root>
          </Flex>
        </Card>

        <Card size="1" style={{ gridColumn: 'span 2' }}>
          <Flex direction="column" gap="3">
            <Heading size="2">Orders ({theirOrders.length})</Heading>
            {theirOrders.length === 0 ? (
              <Text size="2" color="gray">
                Nothing yet
              </Text>
            ) : (
              <Table.Root size="1" variant="ghost">
                <Table.Header>
                  <Table.Row>
                    <Table.ColumnHeaderCell>Cargo</Table.ColumnHeaderCell>
                    <Table.ColumnHeaderCell>Route</Table.ColumnHeaderCell>
                    <Table.ColumnHeaderCell>Role</Table.ColumnHeaderCell>
                    <Table.ColumnHeaderCell>Status</Table.ColumnHeaderCell>
                    <Table.ColumnHeaderCell align="right">Price</Table.ColumnHeaderCell>
                  </Table.Row>
                </Table.Header>
                <Table.Body>
                  {theirOrders.slice(0, 12).map((o) => (
                    <Table.Row key={o.id}>
                      <Table.Cell>
                        <Link to={`/orders/${o.id}`} className="inline-link">
                          <Text size="1">{o.cargo?.cargoType ?? o.type}</Text>
                        </Link>
                      </Table.Cell>
                      <Table.Cell>
                        <Text size="1" color="gray">
                          {o.pickupAddress.split(',')[0]} → {o.dropoffAddress.split(',')[0]}
                        </Text>
                      </Table.Cell>
                      <Table.Cell>
                        <Text size="1" color="gray">
                          {o.clientId === user.id ? 'client' : 'executor'}
                        </Text>
                      </Table.Cell>
                      <Table.Cell>
                        <Badge size="1" variant="soft" color={statusColor(o.status)}>
                          {o.status}
                        </Badge>
                      </Table.Cell>
                      <Table.Cell align="right">
                        <Text size="1" style={{ fontVariantNumeric: 'tabular-nums' }}>
                          {money(o.priceEstimate)}
                        </Text>
                      </Table.Cell>
                    </Table.Row>
                  ))}
                </Table.Body>
              </Table.Root>
            )}
          </Flex>
        </Card>
      </Grid>

      {theirTx.length > 0 && (
        <Card size="1">
          <Flex direction="column" gap="3">
            <Heading size="2">Money ({theirTx.length})</Heading>
            <Table.Root size="1" variant="ghost">
              <Table.Header>
                <Table.Row>
                  <Table.ColumnHeaderCell>Order</Table.ColumnHeaderCell>
                  <Table.ColumnHeaderCell>Direction</Table.ColumnHeaderCell>
                  <Table.ColumnHeaderCell align="right">Amount</Table.ColumnHeaderCell>
                  <Table.ColumnHeaderCell align="right">Fee</Table.ColumnHeaderCell>
                  <Table.ColumnHeaderCell>State</Table.ColumnHeaderCell>
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {theirTx.map((t) => (
                  <Table.Row key={t.id}>
                    <Table.Cell>
                      <Link to={`/orders/${t.orderId}`} className="inline-link">
                        <Text size="1">{t.orderTitle}</Text>
                      </Link>
                    </Table.Cell>
                    <Table.Cell>
                      <Text size="1" color="gray">
                        {t.payeeId === user.id ? 'incoming' : 'outgoing'}
                      </Text>
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
          </Flex>
        </Card>
      )}
    </Flex>
  )
}
