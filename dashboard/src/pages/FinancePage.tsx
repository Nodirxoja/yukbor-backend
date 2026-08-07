import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
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
import { Metric, MoneyMetric } from '../components/Metric'
import { TableSkeleton } from './OrdersPage'
import { dateTime, money, txColor } from '../lib/format'
import type { TransactionStatus } from '../api/types'

type Filter = 'all' | TransactionStatus

/**
 * The ledger, not just its totals. A back office needs to answer "who was paid
 * what, for which order, and has it cleared" — a headline figure cannot.
 */
export function FinancePage() {
  const { transactions, stats, userById, initialising } = useData()
  const [filter, setFilter] = useState<Filter>('all')
  const [query, setQuery] = useState('')

  const counts = useMemo(() => {
    const c = { all: transactions.length, held: 0, released: 0, refunded: 0 }
    for (const t of transactions) c[t.status]++
    return c
  }, [transactions])

  const shown = useMemo(() => {
    const q = query.trim().toLowerCase()
    return transactions
      .filter((t) => filter === 'all' || t.status === filter)
      .filter((t) => {
        if (!q) return true
        const payee = userById(t.payeeId)?.fullName ?? ''
        return (
          t.orderTitle.toLowerCase().includes(q) ||
          payee.toLowerCase().includes(q) ||
          (t.providerRef ?? '').toLowerCase().includes(q) ||
          t.paymentMethod.includes(q)
        )
      })
  }, [transactions, filter, query, userById])

  // The commission take rate — the number that says whether the business works.
  const releasedGross = transactions
    .filter((t) => t.status === 'released')
    .reduce((s, t) => s + Number(t.amount), 0)
  const fees = Number(stats?.serviceFeesCharged ?? 0)
  const takeRate = releasedGross > 0 ? (fees / releasedGross) * 100 : 0

  if (initialising) return <TableSkeleton />

  return (
    <Flex direction="column" gap="4">
      <Grid columns={{ initial: '2', sm: '5' }} gap="2" className="stagger">
        <MoneyMetric
          label="Paid to executors"
          value={Number(stats?.creditedToExecutors ?? 0)}
          accent="green"
          hint="net of commission"
        />
        <MoneyMetric label="Platform fees" value={fees} accent="green" hint="revenue" />
        <MoneyMetric
          label="Held in escrow"
          value={Number(stats?.heldInEscrow ?? 0)}
          accent="blue"
          hint="not yet released"
        />
        <Metric label="Take rate" value={Math.round(takeRate)} suffix="%" hint="of released" />
        <Metric label="Transactions" value={counts.all} hint={`${counts.held} still held`} />
      </Grid>

      <Card size="1">
        <Flex direction="column" gap="3">
          <Flex justify="between" align="center" gap="3" wrap="wrap">
            <SegmentedControl.Root value={filter} onValueChange={(v) => setFilter(v as Filter)} size="1">
              <SegmentedControl.Item value="all">All ({counts.all})</SegmentedControl.Item>
              <SegmentedControl.Item value="held">Held ({counts.held})</SegmentedControl.Item>
              <SegmentedControl.Item value="released">Released ({counts.released})</SegmentedControl.Item>
              <SegmentedControl.Item value="refunded">Refunded ({counts.refunded})</SegmentedControl.Item>
            </SegmentedControl.Root>

            <TextField.Root
              size="1"
              placeholder="Order, payee, provider reference"
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
                  <Table.ColumnHeaderCell>Order</Table.ColumnHeaderCell>
                  <Table.ColumnHeaderCell>Payee</Table.ColumnHeaderCell>
                  <Table.ColumnHeaderCell>Method</Table.ColumnHeaderCell>
                  <Table.ColumnHeaderCell align="right">Amount</Table.ColumnHeaderCell>
                  <Table.ColumnHeaderCell align="right">Fee</Table.ColumnHeaderCell>
                  <Table.ColumnHeaderCell align="right">Payout</Table.ColumnHeaderCell>
                  <Table.ColumnHeaderCell>State</Table.ColumnHeaderCell>
                  <Table.ColumnHeaderCell align="right">When</Table.ColumnHeaderCell>
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {shown.map((t) => {
                  const payout = Number(t.amount) - Number(t.platformCommission)
                  return (
                    <Table.Row key={t.id}>
                      <Table.Cell>
                        <Link to={`/orders/${t.orderId}`} className="inline-link">
                          <Text size="1">{t.orderTitle}</Text>
                        </Link>
                      </Table.Cell>
                      <Table.Cell>
                        <Link to={`/users/${t.payeeId}`} className="inline-link">
                          <Text size="1">
                            {userById(t.payeeId)?.fullName ?? t.payeeId.slice(0, 8)}
                          </Text>
                        </Link>
                      </Table.Cell>
                      <Table.Cell>
                        <Badge size="1" variant="outline">
                          {t.paymentMethod}
                        </Badge>
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
                      <Table.Cell align="right">
                        <Text
                          size="1"
                          weight="medium"
                          style={{ fontVariantNumeric: 'tabular-nums' }}
                        >
                          {money(payout)}
                        </Text>
                      </Table.Cell>
                      <Table.Cell>
                        <Badge size="1" variant="soft" color={txColor[t.status]}>
                          {t.status}
                        </Badge>
                      </Table.Cell>
                      <Table.Cell align="right">
                        <Text size="1" color="gray">
                          {dateTime(t.releasedAt ?? t.refundedAt ?? t.createdAt)}
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
                {query ? `Nothing matches "${query}"` : 'No transactions in this state'}
              </Text>
            </Flex>
          )}
        </Flex>
      </Card>
    </Flex>
  )
}
