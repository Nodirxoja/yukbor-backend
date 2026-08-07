import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Badge,
  Box,
  Card,
  Flex,
  Grid,
  SegmentedControl,
  Table,
  Text,
  TextField,
} from '@radix-ui/themes'
import { MagnifyingGlassIcon } from '@radix-ui/react-icons'
import { useData } from '../data/DataProvider'
import { Metric } from '../components/Metric'
import { TableSkeleton } from './OrdersPage'
import { roleColor, verificationColor } from '../lib/format'
import type { Order, User } from '../api/types'

type Group = 'all' | 'clients' | 'drivers' | 'equipment' | 'labor' | 'staff'

/**
 * What a driver's licence actually permits — the operational fact. "Driver"
 * alone does not tell you whether a load can be offered to them, and this is
 * the same rule the orders service enforces when a leg is accepted.
 */
export type Capability = 'heavy' | 'truck' | 'unqualified'

export function capability(u: User): Capability {
  const cats = u.licenseCategories ?? []
  if (cats.includes('CE')) return 'heavy'
  if (cats.includes('C')) return 'truck'
  return 'unqualified'
}

export const capabilityLabel: Record<Capability, { text: string; color: 'green' | 'blue' | 'red' }> = {
  heavy: { text: 'Tractor-trailer', color: 'green' },
  truck: { text: 'Rigid truck', color: 'blue' },
  unqualified: { text: 'Car only', color: 'red' },
}

export function groupOf(u: User): Group {
  switch (u.role) {
    case 'client':
      return 'clients'
    case 'driver':
      return 'drivers'
    case 'equipmentProvider':
      return 'equipment'
    case 'laborProvider':
      return 'labor'
    default:
      return 'staff'
  }
}

export interface Activity {
  placed: number
  jobs: number
  completed: number
  value: number
}

export function deriveActivity(orders: Order[]): Record<string, Activity> {
  const out: Record<string, Activity> = {}
  const bump = (id: string | null | undefined, f: (a: Activity) => void) => {
    if (!id) return
    out[id] ??= { placed: 0, jobs: 0, completed: 0, value: 0 }
    f(out[id])
  }
  for (const o of orders) {
    const price = Number(o.priceEstimate) || 0
    const done = o.status === 'completed'
    bump(o.clientId, (a) => {
      a.placed++
      a.value += price
      if (done) a.completed++
    })
    for (const ex of [o.assignedDriverId, o.assignedEquipmentProviderId, o.assignedLaborProviderId]) {
      bump(ex, (a) => {
        a.jobs++
        if (done) a.completed++
      })
    }
  }
  return out
}

export function UsersPage() {
  const { users, orders, initialising } = useData()
  const navigate = useNavigate()
  const [group, setGroup] = useState<Group>('all')
  const [query, setQuery] = useState('')

  const activity = useMemo(() => deriveActivity(orders), [orders])

  const counts = useMemo(() => {
    const c: Record<Group, number> = { all: users.length, clients: 0, drivers: 0, equipment: 0, labor: 0, staff: 0 }
    for (const u of users) c[groupOf(u)]++
    return c
  }, [users])

  const fleet = useMemo(() => {
    const d = { heavy: 0, truck: 0, unqualified: 0 }
    for (const u of users) if (u.role === 'driver') d[capability(u)]++
    return d
  }, [users])

  const shown = useMemo(() => {
    const q = query.trim().toLowerCase()
    return users
      .filter((u) => group === 'all' || groupOf(u) === group)
      .filter(
        (u) =>
          !q ||
          u.fullName.toLowerCase().includes(q) ||
          u.phoneNumber.includes(q) ||
          (u.licenseNumber ?? '').toLowerCase().includes(q) ||
          (u.vehiclePlate ?? '').toLowerCase().includes(q),
      )
  }, [users, group, query])

  if (initialising) return <TableSkeleton />

  const showLicence = group === 'drivers' || group === 'equipment' || group === 'all'

  return (
    <Flex direction="column" gap="4">
      <Grid columns={{ initial: '2', sm: '3', lg: '6' }} gap="2" className="stagger">
        <Metric label="Clients" value={counts.clients} hint="place orders" />
        <Metric label="Drivers" value={counts.drivers} hint="transport" />
        <Metric label="Tractor-trailer" value={fleet.heavy} accent="green" hint="licensed CE" />
        <Metric label="Equipment" value={counts.equipment} hint="cranes, excavators" />
        <Metric label="Labor teams" value={counts.labor} hint="loading crews" />
        <Metric label="Staff" value={counts.staff} hint="admin" />
      </Grid>

      <Card size="1">
        <Flex direction="column" gap="3">
          <Flex justify="between" align="center" gap="3" wrap="wrap">
            <SegmentedControl.Root value={group} onValueChange={(v) => setGroup(v as Group)} size="1">
              <SegmentedControl.Item value="all">All ({counts.all})</SegmentedControl.Item>
              <SegmentedControl.Item value="clients">Clients</SegmentedControl.Item>
              <SegmentedControl.Item value="drivers">Drivers</SegmentedControl.Item>
              <SegmentedControl.Item value="equipment">Equipment</SegmentedControl.Item>
              <SegmentedControl.Item value="labor">Labor</SegmentedControl.Item>
              <SegmentedControl.Item value="staff">Staff</SegmentedControl.Item>
            </SegmentedControl.Root>

            <TextField.Root
              size="1"
              placeholder="Name, phone, licence, plate"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              style={{ minWidth: 260 }}
            >
              <TextField.Slot>
                <MagnifyingGlassIcon height="14" width="14" />
              </TextField.Slot>
            </TextField.Root>
          </Flex>

          {group === 'drivers' && (
            <Flex gap="2" wrap="wrap">
              <Badge color="green" variant="soft" size="1">
                {fleet.heavy} can pull a tractor-trailer
              </Badge>
              <Badge color="blue" variant="soft" size="1">
                {fleet.truck} rigid trucks only
              </Badge>
              {fleet.unqualified > 0 && (
                <Badge color="red" variant="soft" size="1">
                  {fleet.unqualified} not qualified for freight
                </Badge>
              )}
            </Flex>
          )}

          <Box style={{ overflowX: 'auto' }}>
            <Table.Root size="1" variant="surface">
              <Table.Header>
                <Table.Row>
                  <Table.ColumnHeaderCell>Name</Table.ColumnHeaderCell>
                  <Table.ColumnHeaderCell>Phone</Table.ColumnHeaderCell>
                  {group === 'all' && <Table.ColumnHeaderCell>Role</Table.ColumnHeaderCell>}
                  {group === 'drivers' && <Table.ColumnHeaderCell>Can carry</Table.ColumnHeaderCell>}
                  {showLicence && <Table.ColumnHeaderCell>Licence</Table.ColumnHeaderCell>}
                  <Table.ColumnHeaderCell>Status</Table.ColumnHeaderCell>
                  <Table.ColumnHeaderCell align="right">
                    {group === 'clients' ? 'Ordered' : 'Jobs'}
                  </Table.ColumnHeaderCell>
                  <Table.ColumnHeaderCell align="right">Rating</Table.ColumnHeaderCell>
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {shown.map((u) => {
                  const a = activity[u.id]
                  const cap = capabilityLabel[capability(u)]
                  return (
                    <Table.Row
                      key={u.id}
                      className="row-link"
                      onClick={() => navigate(`/users/${u.id}`)}
                    >
                      <Table.Cell>
                        <Text size="2">{u.fullName}</Text>
                      </Table.Cell>
                      <Table.Cell>
                        <Text size="1" color="gray">
                          {u.phoneNumber}
                        </Text>
                      </Table.Cell>
                      {group === 'all' && (
                        <Table.Cell>
                          <Badge size="1" color={roleColor[u.role]} variant="soft">
                            {u.role}
                          </Badge>
                        </Table.Cell>
                      )}
                      {group === 'drivers' && (
                        <Table.Cell>
                          <Badge size="1" color={cap.color} variant="soft">
                            {cap.text}
                          </Badge>
                        </Table.Cell>
                      )}
                      {showLicence && (
                        <Table.Cell>
                          {u.licenseNumber ? (
                            <Text size="1">
                              {u.licenseNumber}
                              {u.vehiclePlate ? ` · ${u.vehiclePlate}` : ''}
                            </Text>
                          ) : (
                            <Text size="1" color="gray">
                              —
                            </Text>
                          )}
                        </Table.Cell>
                      )}
                      <Table.Cell>
                        <Badge size="1" color={verificationColor[u.verificationStatus]} variant="soft">
                          {u.verificationStatus}
                        </Badge>
                      </Table.Cell>
                      <Table.Cell align="right">
                        <Text size="2" style={{ fontVariantNumeric: 'tabular-nums' }}>
                          {group === 'clients' ? (a?.placed ?? 0) : (a?.jobs ?? 0)}
                        </Text>
                      </Table.Cell>
                      <Table.Cell align="right">
                        <Text size="1" color={u.ratingsCount ? undefined : 'gray'}>
                          {u.ratingsCount ? `${u.rating.toFixed(1)} (${u.ratingsCount})` : '—'}
                        </Text>
                      </Table.Cell>
                    </Table.Row>
                  )
                })}
              </Table.Body>
            </Table.Root>
          </Box>

          {shown.length === 0 && (
            <Flex justify="center" py="6">
              <Text size="2" color="gray">
                {query ? `Nobody matches "${query}"` : 'No one in this group yet'}
              </Text>
            </Flex>
          )}
        </Flex>
      </Card>
    </Flex>
  )
}
