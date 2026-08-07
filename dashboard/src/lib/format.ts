// Shared formatting and colour mapping, so a status is the same colour on the
// map, in a table and on a detail page. Three copies of this drift within days.

import type { OrderStatus, TransactionStatus, UserRole, VerificationStatus } from '../api/types'

type Color = 'gray' | 'blue' | 'cyan' | 'indigo' | 'violet' | 'orange' | 'teal' | 'green' | 'red' | 'crimson' | 'amber'

export function statusColor(s: OrderStatus): Color {
  switch (s) {
    case 'draft':
      return 'gray'
    case 'published':
      return 'blue'
    case 'matched':
      return 'cyan'
    case 'accepted':
      return 'indigo'
    case 'inProgress':
    case 'loadingInProgress':
      return 'violet'
    case 'inTransit':
      return 'orange'
    case 'delivered':
      return 'teal'
    case 'completed':
      return 'green'
    case 'cancelled':
      return 'red'
    case 'disputed':
      return 'crimson'
  }
}

export const roleColor: Record<UserRole, Color> = {
  client: 'blue',
  driver: 'orange',
  equipmentProvider: 'violet',
  laborProvider: 'teal',
  fleetAdmin: 'gray',
  admin: 'gray',
}

export const verificationColor: Record<VerificationStatus, Color> = {
  pending: 'amber',
  approved: 'green',
  rejected: 'red',
}

export const txColor: Record<TransactionStatus, Color> = {
  held: 'blue',
  released: 'green',
  refunded: 'orange',
}

/** Money as read in Uzbekistan: grouped, no decimals — sums have no subunit. */
export function money(v: string | number): string {
  return Number(v).toLocaleString('ru-RU')
}

export function dateTime(iso?: string | null): string {
  if (!iso) return '—'
  const d = new Date(iso)
  return d.toLocaleString('ru-RU', {
    day: '2-digit',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export function dateOnly(iso?: string | null): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleDateString('ru-RU', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
  })
}

/** "3 hours ago" — the useful form when scanning a feed. */
export function relative(iso?: string | null): string {
  if (!iso) return '—'
  const diff = Date.now() - new Date(iso).getTime()
  const mins = Math.round(diff / 60000)
  if (mins < 1) return 'just now'
  if (mins < 60) return `${mins} min ago`
  const hours = Math.round(mins / 60)
  if (hours < 24) return `${hours} h ago`
  const days = Math.round(hours / 24)
  return `${days} d ago`
}

/** The legs an order actually has, with their statuses. */
export function legsOf(o: {
  type: string
  status: OrderStatus
  equipmentStatus?: OrderStatus | null
  laborStatus?: OrderStatus | null
}): { label: string; status: OrderStatus }[] {
  const legs: { label: string; status: OrderStatus }[] = []
  if (o.type === 'equipmentOnly') legs.push({ label: 'equipment', status: o.status })
  else if (o.type === 'laborOnly') legs.push({ label: 'labor', status: o.status })
  else legs.push({ label: 'transport', status: o.status })

  if (o.type === 'transportWithOptions') {
    if (o.equipmentStatus) legs.push({ label: 'equipment', status: o.equipmentStatus })
    if (o.laborStatus) legs.push({ label: 'labor', status: o.laborStatus })
  }
  return legs
}
