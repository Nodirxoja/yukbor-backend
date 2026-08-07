// Compact metric tiles.
//
// The previous version used full Cards with size-6 figures, which ate a third
// of the viewport before showing a single order. These are deliberately thin:
// a label, a number, and at most one line of context. Density is the point —
// an operations screen should fit the operation on it.

import { Card, Flex, Text } from '@radix-ui/themes'
import { CountUp } from '../hooks/useCountUp'

export type MetricAccent = 'green' | 'blue' | 'orange' | 'red' | undefined

export function Metric({
  label,
  value,
  hint,
  accent,
  suffix,
}: {
  label: string
  value: number
  hint?: string
  accent?: MetricAccent
  suffix?: string
}) {
  return (
    <Card size="1" className="metric">
      <Flex direction="column" gap="1">
        <Text size="1" color="gray" truncate>
          {label}
        </Text>
        <Flex align="baseline" gap="1">
          {/* Tabular figures keep the number from jittering as digits change
              while it counts. */}
          <Text size="5" weight="bold" color={accent} style={{ fontVariantNumeric: 'tabular-nums' }}>
            <CountUp value={value} />
          </Text>
          {suffix && (
            <Text size="1" color="gray">
              {suffix}
            </Text>
          )}
        </Flex>
        {hint && (
          <Text size="1" color="gray" truncate>
            {hint}
          </Text>
        )}
      </Flex>
    </Card>
  )
}

/** Money is long; it gets a smaller figure so tiles stay the same height. */
export function MoneyMetric({
  label,
  value,
  hint,
  accent,
}: {
  label: string
  value: number
  hint?: string
  accent?: MetricAccent
}) {
  return (
    <Card size="1" className="metric">
      <Flex direction="column" gap="1">
        <Text size="1" color="gray" truncate>
          {label}
        </Text>
        <Flex align="baseline" gap="1">
          <Text size="4" weight="bold" color={accent} style={{ fontVariantNumeric: 'tabular-nums' }}>
            <CountUp value={value} />
          </Text>
          <Text size="1" color="gray">
            UZS
          </Text>
        </Flex>
        {hint && (
          <Text size="1" color="gray" truncate>
            {hint}
          </Text>
        )}
      </Flex>
    </Card>
  )
}
