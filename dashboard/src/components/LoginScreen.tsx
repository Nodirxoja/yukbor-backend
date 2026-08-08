// Sign-in for the dashboard.
//
// Username and password, NOT the phone/SMS flow the mobile app uses. The two
// audiences are different: an operator opening the back office is not a
// customer, may not hold the account's handset, and should not wait on an SMS
// to read a chart.
//
// Built from Radix Themes components; icons from @radix-ui/react-icons;
// failures surface as toasts rather than a block that shifts the layout.

import { useEffect, useRef, useState } from 'react'
import { Box, Button, Card, Flex, Heading, IconButton, Text, TextField } from '@radix-ui/themes'
import {
  ArrowRightIcon,
  EyeClosedIcon,
  EyeOpenIcon,
  LockClosedIcon,
  PersonIcon,
} from '@radix-ui/react-icons'
import { ApiError, adminLogin, saveSession } from '../api/auth'
import type { Session } from '../api/auth'
import { useToast } from './Toaster'

export function LoginScreen({ onSignedIn }: { onSignedIn: (s: Session) => void }) {
  const toast = useToast()

  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [busy, setBusy] = useState(false)

  const usernameRef = useRef<HTMLInputElement>(null)
  const passwordRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    const t = setTimeout(() => usernameRef.current?.focus(), 50)
    return () => clearTimeout(t)
  }, [])

  const canSubmit = username.trim().length > 0 && password.length > 0

  async function submit() {
    if (!canSubmit || busy) return
    setBusy(true)
    try {
      const session = await adminLogin(username.trim(), password)
      saveSession(session)
      toast.success('Signed in', `Welcome back, ${session.user.fullName}`)
      onSignedIn(session)
    } catch (e) {
      const err = e as ApiError
      toast.error(
        err.code === 'INVALID_CREDENTIALS' ? 'Wrong username or password' : 'Sign-in failed',
        err.message,
      )
      // Clear only the password: retyping a correct username is pure friction.
      setPassword('')
      passwordRef.current?.focus()
    } finally {
      setBusy(false)
    }
  }

  return (
    <Flex align="center" justify="center" style={{ minHeight: '100vh' }} p="4">
      <Box style={{ width: '100%', maxWidth: 400 }}>
        <Flex direction="column" align="center" gap="3" mb="5">
          {/* The mark carries the wordmark already, so there is no heading
              here — repeating "YUK BOR" underneath would just say it twice. */}
          <img
            src="/logo.png"
            alt="YUK BOR"
            width={84}
            height={84}
            className="brand-mark brand-mark-lg"
          />
          <Text size="2" color="gray">
            Admin dashboard
          </Text>
        </Flex>

        <Card size="4">
          <Flex direction="column" gap="4">
            <Flex direction="column" gap="1">
              <Heading size="4">Sign in</Heading>
              <Text size="2" color="gray">
                Enter your administrator credentials.
              </Text>
            </Flex>

            <Flex direction="column" gap="2">
              <Text as="label" size="2" weight="medium" htmlFor="username">
                Username
              </Text>
              <TextField.Root
                id="username"
                ref={usernameRef}
                size="3"
                placeholder="admin"
                value={username}
                autoComplete="username"
                autoCapitalize="none"
                spellCheck={false}
                onChange={(e) => setUsername(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && passwordRef.current?.focus()}
              >
                <TextField.Slot>
                  <PersonIcon height="16" width="16" />
                </TextField.Slot>
              </TextField.Root>
            </Flex>

            <Flex direction="column" gap="2">
              <Text as="label" size="2" weight="medium" htmlFor="password">
                Password
              </Text>
              <TextField.Root
                id="password"
                ref={passwordRef}
                size="3"
                placeholder="••••••••"
                type={showPassword ? 'text' : 'password'}
                value={password}
                autoComplete="current-password"
                onChange={(e) => setPassword(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && submit()}
              >
                <TextField.Slot>
                  <LockClosedIcon height="16" width="16" />
                </TextField.Slot>
                <TextField.Slot>
                  <IconButton
                    size="1"
                    variant="ghost"
                    color="gray"
                    type="button"
                    tabIndex={-1}
                    aria-label={showPassword ? 'Hide password' : 'Show password'}
                    onClick={() => setShowPassword((v) => !v)}
                  >
                    {showPassword ? <EyeOpenIcon /> : <EyeClosedIcon />}
                  </IconButton>
                </TextField.Slot>
              </TextField.Root>
            </Flex>

            <Button size="3" disabled={!canSubmit} loading={busy} onClick={submit}>
              Sign in
              <ArrowRightIcon />
            </Button>
          </Flex>
        </Card>

        <Flex direction="column" align="center" gap="1" mt="4">
          <Text size="1" color="gray">
            Administrator access only
          </Text>
          <Text size="1" color="gray">
            Drivers and clients sign in through the mobile app.
          </Text>
        </Flex>
      </Box>
    </Flex>
  )
}
