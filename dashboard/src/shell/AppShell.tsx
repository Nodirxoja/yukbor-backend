// The application frame: a fixed sidebar, a compact topbar, and a content area
// that animates between routes.
//
// A sidebar rather than tabs because tabs stop working around six items — and a
// logistics back office grows sections (fleet, disputes, tariffs, settlements)
// rather than shrinking. The rail also gives navigation a permanent home, so
// where you are is never a question.

import { NavLink, useLocation } from 'react-router-dom'
import {
  Avatar,
  Badge,
  Box,
  Button,
  DropdownMenu,
  Flex,
  IconButton,
  Separator,
  Text,
  Tooltip,
} from '@radix-ui/themes'
import {
  ArchiveIcon,
  BarChartIcon,
  CubeIcon,
  DashboardIcon,
  ExitIcon,
  HamburgerMenuIcon,
  PersonIcon,
  ReloadIcon,
} from '@radix-ui/react-icons'
import { useState } from 'react'
import type { Session } from '../api/auth'
import { useData } from '../data/DataProvider'

interface NavItem {
  to: string
  label: string
  icon: typeof DashboardIcon
  count?: number
}

export function AppShell({
  session,
  onSignOut,
  children,
}: {
  session: Session | null
  onSignOut: () => void
  children: React.ReactNode
}) {
  const { orders, users, transactions, refreshing, lastSync, refresh } = useData()
  const location = useLocation()
  const [collapsed, setCollapsed] = useState(false)

  const nav: NavItem[] = [
    { to: '/', label: 'Overview', icon: DashboardIcon },
    { to: '/orders', label: 'Orders', icon: CubeIcon, count: orders.length },
    { to: '/users', label: 'People', icon: PersonIcon, count: users.length },
    { to: '/finance', label: 'Finance', icon: BarChartIcon, count: transactions.length },
  ]

  const initials = session?.user.fullName
    ? session.user.fullName.split(' ').slice(0, 2).map((p) => p[0]).join('')
    : 'A'

  return (
    <Flex style={{ minHeight: '100vh' }}>
      <Box
        className="app-sidebar"
        data-collapsed={collapsed || undefined}
        style={{ width: collapsed ? 64 : 232 }}
      >
        <Flex direction="column" gap="1" p="3" style={{ height: '100%' }}>
          <Flex align="center" justify="between" mb="4" px="1">
            {!collapsed && (
              <Flex direction="column">
                <Text size="3" weight="bold">
                  YUK BOR
                </Text>
                <Text size="1" color="gray">
                  Operations
                </Text>
              </Flex>
            )}
            <IconButton
              size="1"
              variant="ghost"
              color="gray"
              onClick={() => setCollapsed((c) => !c)}
              aria-label={collapsed ? 'Expand navigation' : 'Collapse navigation'}
            >
              <HamburgerMenuIcon />
            </IconButton>
          </Flex>

          {nav.map((item) => (
            <NavItemLink key={item.to} item={item} collapsed={collapsed} />
          ))}

          <Box style={{ flex: 1 }} />

          <Separator size="4" my="2" />

          {session && (
            <DropdownMenu.Root>
              <DropdownMenu.Trigger>
                <Button variant="ghost" color="gray" size="2" style={{ justifyContent: 'flex-start' }}>
                  <Avatar size="1" fallback={initials} radius="full" />
                  {!collapsed && (
                    <Text size="2" truncate>
                      {session.user.fullName}
                    </Text>
                  )}
                </Button>
              </DropdownMenu.Trigger>
              <DropdownMenu.Content side="top">
                <DropdownMenu.Label>
                  <Flex direction="column" gap="1">
                    <Text size="1">{session.user.phoneNumber}</Text>
                    <Badge size="1" variant="soft">
                      {session.user.role}
                    </Badge>
                  </Flex>
                </DropdownMenu.Label>
                <DropdownMenu.Separator />
                <DropdownMenu.Item color="red" onSelect={onSignOut}>
                  <ExitIcon />
                  Sign out
                </DropdownMenu.Item>
              </DropdownMenu.Content>
            </DropdownMenu.Root>
          )}
        </Flex>
      </Box>

      <Box style={{ flex: 1, minWidth: 0 }}>
        <Flex className="app-topbar" align="center" justify="between" px="4" gap="3">
          <Breadcrumb />
          <Flex align="center" gap="3">
            <Text size="1" color="gray">
              {lastSync ? `synced ${lastSync.toLocaleTimeString()}` : 'connecting'}
            </Text>
            <Tooltip content="Refresh now">
              <IconButton
                size="1"
                variant="ghost"
                color="gray"
                onClick={refresh}
                aria-label="Refresh"
              >
                <ReloadIcon className={refreshing ? 'spin' : undefined} />
              </IconButton>
            </Tooltip>
          </Flex>
        </Flex>

        {/* Keyed on the path so each navigation replays the entrance: a page
            should arrive, not blink into existence. */}
        <Box key={location.pathname} className="page-enter" p="4">
          {children}
        </Box>
      </Box>
    </Flex>
  )
}

function NavItemLink({ item, collapsed }: { item: NavItem; collapsed: boolean }) {
  const { to, label, icon: Icon, count } = item
  const body = (
    <NavLink
      to={to}
      end={to === '/'}
      className={({ isActive }) => (isActive ? 'nav-item active' : 'nav-item')}
    >
      {({ isActive }) => (
        <>
          {/* The active marker is a separate element so it can slide rather
              than appear — the eye follows movement, not a repaint. */}
          <span className="nav-item-marker" data-active={isActive || undefined} />
          <Icon />
          {!collapsed && (
            <>
              <Text size="2" style={{ flex: 1 }}>
                {label}
              </Text>
              {count !== undefined && count > 0 && (
                <Text size="1" color="gray">
                  {count}
                </Text>
              )}
            </>
          )}
        </>
      )}
    </NavLink>
  )

  return collapsed ? (
    <Tooltip content={label} side="right">
      {body}
    </Tooltip>
  ) : (
    body
  )
}

function Breadcrumb() {
  const { pathname } = useLocation()
  const parts = pathname.split('/').filter(Boolean)
  const title =
    parts.length === 0
      ? 'Overview'
      : parts[0] === 'users'
        ? 'People'
        : parts[0][0].toUpperCase() + parts[0].slice(1)

  return (
    <Flex align="center" gap="2">
      <Text size="2" weight="medium">
        {title}
      </Text>
      {parts.length > 1 && (
        <>
          <Text size="2" color="gray">
            /
          </Text>
          <Flex align="center" gap="1">
            <ArchiveIcon />
            <Text size="1" color="gray">
              {parts[1].slice(0, 8)}
            </Text>
          </Flex>
        </>
      )}
    </Flex>
  )
}
