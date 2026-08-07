// People, grouped by what they actually do.
//
// A single flat user list is useless for operations: "driver" covers somebody
// who may legally pull a tractor-trailer and somebody who may not, and that
// distinction decides which loads they can be offered. So drivers are split by
// the categories the licence registry actually issued them, not by their role
// string — the same rule the orders service enforces when a leg is accepted.

import { useMemo, useState } from 'react'
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
import type { Order, User, UserRole, VerificationStatus } from '../api/types'
import { CountUp } from '../hooks/useCountUp'

type Group = 'all' | 'clients' | 'drivers' | 'equipment' | 'labor' | 'staff'

const roleColor: Record<UserRole, React.ComponentProps<typeof Badge>['color']> = {
  client: 'blue',
  driver: 'orange',
  equipmentProvider: 'violet',
  laborProvider: 'teal',
  fleetAdmin: 'gray',
  admin: 'gray',
}

const verColor: Record<VerificationStatus, React.ComponentProps<typeof Badge>['color']> = {
  pending: 'amber',
  approved: 'green',
  rejected: 'red',
}

/**
 * What a driver's licence actually permits. This is the operational fact —
 * "driver" alone does not tell you whether a load can be offered to them.
 */
type Capability = 'heavy' | 'truck' | 'unqualified'

function capability(u: User): Capability {
  const cats = u.licenseCategories ?? []
  if (cats.includes('CE')) return 'heavy' // tractor-trailer
  if (cats.includes('C')) return 'truck' // rigid trucks over 3.5t
  return 'unqualified' // car licence only
}

const capabilityLabel: Record<Capability, { text: string; color: 'green' | 'blue' | 'red' }> = {
  heavy: { text: 'Tractor-trailer (CE)', color: 'green' },
  truck: { text: 'Truck (C)', color: 'blue' },
  unqualified: { text: 'Car only (B)', color: 'red' },
}

function groupOf(u: User): Group {
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

/** Workload per person, derived from the orders list rather than another API call. */
interface Activity {
  placed: number // orders this client created
  jobs: number // legs assigned to this executor
  completed: number
  value: number // UZS across those orders
}

function deriveActivity(orders: Order[]): Record<string, Activity> {
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
    for (const executor of [
      o.assignedDriverId,
      o.assignedEquipmentProviderId,
      o.assignedLaborProviderId,
    ]) {
      bump(executor, (a) => {
        a.jobs++
        if (done) a.completed++
      })
    }
  }
  return out
}

function money(v: number): string {
  return `${v.toLocaleString('ru-RU')} UZS`
}

export function UsersSection({ users, orders }: { users: User[]; orders: Order[] }) {
  const [group, setGroup] = useState<Group>('all')
  const [query, setQuery] = useState('')

  const activity = useMemo(() => deriveActivity(orders), [orders])

  const counts = useMemo(() => {
    const c: Record<Group, number> = { all: users.length, clients: 0, drivers: 0, equipment: 0, labor: 0, staff: 0 }
    for (const u of users) c[groupOf(u)]++
    return c
  }, [users])

  const driverBreakdown = useMemo(() => {
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

  return (
    <Flex direction="column" gap="4">
      {/* Who is on the platform, at a glance. */}
      <Grid columns={{ initial: '2', sm: '3', lg: '6' }} gap="3" className="stagger">
        <Tile label="Clients" value={counts.clients} hint="place orders" />
        <Tile label="Drivers" value={counts.drivers} hint="transport" />
        <Tile
          label="Tractor-trailer"
          value={driverBreakdown.heavy}
          hint="licensed CE"
          accent="green"
        />
        <Tile label="Equipment" value={counts.equipment} hint="cranes, excavators" />
        <Tile label="Labor teams" value={counts.labor} hint="loading crews" />
        <Tile label="Staff" value={counts.staff} hint="admin" />
      </Grid>

      <Card>
        <Flex direction="column" gap="3">
          <Flex justify="between" align="center" gap="3" wrap="wrap">
            <SegmentedControl.Root
              value={group}
              onValueChange={(v) => setGroup(v as Group)}
              size="1"
            >
              <SegmentedControl.Item value="all">All ({counts.all})</SegmentedControl.Item>
              <SegmentedControl.Item value="clients">Clients ({counts.clients})</SegmentedControl.Item>
              <SegmentedControl.Item value="drivers">Drivers ({counts.drivers})</SegmentedControl.Item>
              <SegmentedControl.Item value="equipment">Equipment ({counts.equipment})</SegmentedControl.Item>
              <SegmentedControl.Item value="labor">Labor ({counts.labor})</SegmentedControl.Item>
              <SegmentedControl.Item value="staff">Staff ({counts.staff})</SegmentedControl.Item>
            </SegmentedControl.Root>

            <TextField.Root
              size="1"
              placeholder="Name, phone, licence, plate"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              style={{ minWidth: 240 }}
            >
              <TextField.Slot>
                <MagnifyingGlassIcon height="14" width="14" />
              </TextField.Slot>
            </TextField.Root>
          </Flex>

          {group === 'drivers' && (
            <Flex gap="2" wrap="wrap">
              <Badge color="green" variant="soft">
                {driverBreakdown.heavy} can pull a tractor-trailer
              </Badge>
              <Badge color="blue" variant="soft">
                {driverBreakdown.truck} rigid trucks only
              </Badge>
              {driverBreakdown.unqualified > 0 && (
                <Badge color="red" variant="soft">
                  {driverBreakdown.unqualified} not qualified for freight
                </Badge>
              )}
            </Flex>
          )}

          <UsersTable users={shown} group={group} activity={activity} />

          {shown.length === 0 && (
            <Flex justify="center" py="6">
              <Text size="2" color="gray">
                {query ? `Nobody matches "${query}"` : 'No users in this group yet'}
              </Text>
            </Flex>
          )}
        </Flex>
      </Card>
    </Flex>
  )
}

function Tile({
  label,
  value,
  hint,
  accent,
}: {
  label: string
  value: number
  hint: string
  accent?: 'green'
}) {
  return (
    <Card>
      <Flex direction="column" gap="1">
        <Text size="1" color="gray">
          {label}
        </Text>
        <Text size="6" weight="bold" color={accent} style={{ fontVariantNumeric: 'tabular-nums' }}>
          <CountUp value={value} />
        </Text>
        <Text size="1" color="gray">
          {hint}
        </Text>
      </Flex>
    </Card>
  )
}

/** Columns follow the group: an operator looking at drivers wants licences. */
function UsersTable({
  users,
  group,
  activity,
}: {
  users: User[]
  group: Group
  activity: Record<string, Activity>
}) {
  const showLicence = group === 'drivers' || group === 'equipment' || group === 'all'
  const showClientStats = group === 'clients'

  return (
    <Box style={{ overflowX: 'auto' }}>
      <Table.Root size="1" variant="surface">
        <Table.Header>
          <Table.Row>
            <Table.ColumnHeaderCell>Name</Table.ColumnHeaderCell>
            <Table.ColumnHeaderCell>Phone</Table.ColumnHeaderCell>
            {group === 'all' && <Table.ColumnHeaderCell>Role</Table.ColumnHeaderCell>}
            {group === 'drivers' && <Table.ColumnHeaderCell>Can carry</Table.ColumnHeaderCell>}
            {showLicence && <Table.ColumnHeaderCell>Licence / vehicle</Table.ColumnHeaderCell>}
            <Table.ColumnHeaderCell>Status</Table.ColumnHeaderCell>
            <Table.ColumnHeaderCell align="right">
              {showClientStats ? 'Orders placed' : 'Jobs'}
            </Table.ColumnHeaderCell>
            <Table.ColumnHeaderCell align="right">Rating</Table.ColumnHeaderCell>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {users.map((u) => {
            const a = activity[u.id]
            const cap = capabilityLabel[capability(u)]
            return (
              <Table.Row key={u.id}>
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
                    <Badge color={roleColor[u.role]} variant="soft">
                      {u.role}
                    </Badge>
                  </Table.Cell>
                )}

                {group === 'drivers' && (
                  <Table.Cell>
                    <Badge color={cap.color} variant="soft">
                      {cap.text}
                    </Badge>
                  </Table.Cell>
                )}

                {showLicence && (
                  <Table.Cell>
                    {u.licenseNumber ? (
                      <Flex direction="column">
                        <Text size="1">
                          {u.licenseNumber}
                          {u.licenseCategories?.length ? ` · ${u.licenseCategories.join('/')}` : ''}
                        </Text>
                        {u.vehiclePlate && (
                          <Text size="1" color="gray">
                            {u.vehiclePlate}
                          </Text>
                        )}
                      </Flex>
                    ) : (
                      <Text size="1" color="gray">
                        —
                      </Text>
                    )}
                  </Table.Cell>
                )}

                <Table.Cell>
                  <Flex direction="column" gap="1" align="start">
                    <Badge color={verColor[u.verificationStatus]} variant="soft">
                      {u.verificationStatus}
                    </Badge>
                    {u.rejectionReason && (
                      <Text size="1" color="red">
                        {u.rejectionReason}
                      </Text>
                    )}
                  </Flex>
                </Table.Cell>

                <Table.Cell align="right">
                  {showClientStats ? (
                    a ? (
                      <Flex direction="column" align="end">
                        <Text size="2">{a.placed}</Text>
                        <Text size="1" color="gray">
                          {money(a.value)}
                        </Text>
                      </Flex>
                    ) : (
                      <Text size="1" color="gray">
                        —
                      </Text>
                    )
                  ) : a?.jobs ? (
                    <Flex direction="column" align="end">
                      <Text size="2">{a.jobs}</Text>
                      <Text size="1" color="gray">
                        {a.completed} done
                      </Text>
                    </Flex>
                  ) : (
                    <Text size="1" color="gray">
                      —
                    </Text>
                  )}
                </Table.Cell>

                <Table.Cell align="right">
                  {u.ratingsCount > 0 ? (
                    <Flex direction="column" align="end">
                      <Text size="2">{u.rating.toFixed(1)}</Text>
                      <Text size="1" color="gray">
                        {u.ratingsCount} review{u.ratingsCount === 1 ? '' : 's'}
                      </Text>
                    </Flex>
                  ) : (
                    <Text size="1" color="gray">
                      —
                    </Text>
                  )}
                </Table.Cell>
              </Table.Row>
            )
          })}
        </Table.Body>
      </Table.Root>
    </Box>
  )
}
