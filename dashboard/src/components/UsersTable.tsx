import { Badge, Table } from '@radix-ui/themes'
import type { User, UserRole, VerificationStatus } from '../api/types'

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

export function UsersTable({ users }: { users: User[] }) {
  return (
    <Table.Root size="1" variant="surface">
      <Table.Header>
        <Table.Row>
          <Table.ColumnHeaderCell>Name</Table.ColumnHeaderCell>
          <Table.ColumnHeaderCell>Phone</Table.ColumnHeaderCell>
          <Table.ColumnHeaderCell>Role</Table.ColumnHeaderCell>
          <Table.ColumnHeaderCell>Verification</Table.ColumnHeaderCell>
          <Table.ColumnHeaderCell align="right">Rating</Table.ColumnHeaderCell>
        </Table.Row>
      </Table.Header>
      <Table.Body>
        {users.map((u) => (
          <Table.Row key={u.id}>
            <Table.Cell>{u.fullName}</Table.Cell>
            <Table.Cell>{u.phoneNumber}</Table.Cell>
            <Table.Cell>
              <Badge color={roleColor[u.role]} variant="soft">
                {u.role}
              </Badge>
            </Table.Cell>
            <Table.Cell>
              <Badge color={verColor[u.verificationStatus]} variant="soft">
                {u.verificationStatus}
              </Badge>
            </Table.Cell>
            <Table.Cell align="right">
              {u.ratingsCount > 0 ? `★ ${u.rating.toFixed(1)} (${u.ratingsCount})` : '—'}
            </Table.Cell>
          </Table.Row>
        ))}
      </Table.Body>
    </Table.Root>
  )
}
