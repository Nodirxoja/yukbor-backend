// Sign-in for the dashboard.
//
// Replaces the browser's basic-auth dialog, which could not show what the app
// is, why it wants a number, whether a code was sent, or what went wrong — it
// could only ask twice and fail silently.
//
// Two steps, one decision each: the phone, then the code. Everything built from
// Radix Themes components; icons from @radix-ui/react-icons; failures surface
// as toasts rather than a red block that shifts the layout (plan §11).

import { useEffect, useRef, useState } from 'react'
import { Badge, Box, Button, Callout, Card, Flex, Heading, Separator, Text, TextField } from '@radix-ui/themes'
import {
  ArrowLeftIcon,
  ArrowRightIcon,
  CheckCircledIcon,
  InfoCircledIcon,
  LockClosedIcon,
  MobileIcon,
  ReloadIcon,
} from '@radix-ui/react-icons'
import { ApiError, formatPhone, isValidPhone, login, normalisePhone, requestOtp, saveSession, verifyOtp } from '../api/auth'
import type { Session } from '../api/auth'
import { useToast } from './Toaster'

const CODE_LENGTH = 4

export function LoginScreen({ onSignedIn }: { onSignedIn: (s: Session) => void }) {
  const toast = useToast()

  const [step, setStep] = useState<'phone' | 'code'>('phone')
  const [phone, setPhone] = useState('+998 ')
  const [code, setCode] = useState('')
  const [verificationId, setVerificationId] = useState('')
  const [busy, setBusy] = useState(false)
  const [secondsLeft, setSecondsLeft] = useState(0)
  const [devCode, setDevCode] = useState<string | undefined>()

  const phoneRef = useRef<HTMLInputElement>(null)
  const codeRef = useRef<HTMLInputElement>(null)

  // Move focus with the step, so the keyboard never has to be reached for.
  useEffect(() => {
    const t = setTimeout(() => (step === 'phone' ? phoneRef : codeRef).current?.focus(), 50)
    return () => clearTimeout(t)
  }, [step])

  // Countdown until a new code may be requested.
  useEffect(() => {
    if (secondsLeft <= 0) return
    const t = setInterval(() => setSecondsLeft((s) => Math.max(0, s - 1)), 1000)
    return () => clearInterval(t)
  }, [secondsLeft])

  const phoneValid = isValidPhone(phone)

  async function sendCode(resend = false) {
    if (!phoneValid || busy) return
    setBusy(true)
    try {
      const challenge = await requestOtp(normalisePhone(phone))
      setVerificationId(challenge.verificationId)
      setSecondsLeft(challenge.expiresInSeconds)
      setDevCode(challenge.devCode)
      setCode('')
      setStep('code')
      toast.success(
        resend ? 'New code sent' : 'Code sent',
        `Check the SMS on ${formatPhone(normalisePhone(phone))}`,
      )
    } catch (e) {
      const err = e as ApiError
      toast.error(
        err.code === 'OTP_RATE_LIMITED' ? 'Too many attempts' : 'Could not send the code',
        err.message,
      )
    } finally {
      setBusy(false)
    }
  }

  async function submitCode() {
    if (code.length !== CODE_LENGTH || busy) return
    setBusy(true)
    try {
      await verifyOtp(verificationId, code)
      const session = await login(normalisePhone(phone))

      // The dashboard is the back office: anyone else has nothing to see here,
      // and every /admin endpoint would reject them anyway.
      if (session.user.role !== 'admin') {
        toast.error(
          'Not an administrator',
          `${session.user.fullName} is signed up as ${session.user.role}. Use an admin account.`,
        )
        return
      }

      saveSession(session)
      toast.success('Signed in', `Welcome back, ${session.user.fullName}`)
      onSignedIn(session)
    } catch (e) {
      const err = e as ApiError
      const title =
        err.code === 'OTP_INVALID'
          ? 'Wrong code'
          : err.code === 'OTP_EXPIRED'
            ? 'Code expired'
            : err.code === 'USER_NOT_FOUND'
              ? 'No account for this number'
              : 'Sign-in failed'
      toast.error(title, err.message)
      if (err.code === 'OTP_EXPIRED') setSecondsLeft(0)
      setCode('')
      codeRef.current?.focus()
    } finally {
      setBusy(false)
    }
  }

  return (
    <Flex align="center" justify="center" style={{ minHeight: '100vh' }} p="4">
      <Box style={{ width: '100%', maxWidth: 420 }}>
        <Flex direction="column" align="center" gap="1" mb="5">
          <Heading size="7" weight="bold">
            YUK BOR
          </Heading>
          <Text size="2" color="gray">
            Admin dashboard
          </Text>
        </Flex>

        <Card size="4">
          {step === 'phone' ? (
            <Flex direction="column" gap="4">
              <Flex direction="column" gap="1">
                <Heading size="4">Sign in</Heading>
                <Text size="2" color="gray">
                  We will text a confirmation code to your number.
                </Text>
              </Flex>

              <Flex direction="column" gap="2">
                <Text as="label" size="2" weight="medium" htmlFor="phone">
                  Phone number
                </Text>
                <TextField.Root
                  id="phone"
                  ref={phoneRef}
                  size="3"
                  placeholder="+998 90 123 45 67"
                  value={phone}
                  inputMode="tel"
                  autoComplete="tel"
                  onChange={(e) => setPhone(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && sendCode()}
                >
                  <TextField.Slot>
                    <MobileIcon height="16" width="16" />
                  </TextField.Slot>
                  {phoneValid && (
                    <TextField.Slot>
                      <Text color="green" style={{ display: 'flex' }}>
                        <CheckCircledIcon height="16" width="16" />
                      </Text>
                    </TextField.Slot>
                  )}
                </TextField.Root>
                <Text size="1" color="gray">
                  Uzbek mobile number, nine digits after +998.
                </Text>
              </Flex>

              <Button size="3" disabled={!phoneValid} loading={busy} onClick={() => sendCode()}>
                Send code
                <ArrowRightIcon />
              </Button>
            </Flex>
          ) : (
            <Flex direction="column" gap="4">
              <Flex direction="column" gap="1">
                <Heading size="4">Enter the code</Heading>
                <Text size="2" color="gray">
                  Sent to {formatPhone(normalisePhone(phone))}
                </Text>
              </Flex>

              <Flex direction="column" gap="2">
                <Text as="label" size="2" weight="medium" htmlFor="code">
                  Confirmation code
                </Text>
                <TextField.Root
                  id="code"
                  ref={codeRef}
                  size="3"
                  placeholder="0000"
                  value={code}
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  maxLength={CODE_LENGTH}
                  // The only typographic override in the app: a code field is
                  // unreadable without spacing, and Themes has no variant for it.
                  style={{ letterSpacing: '0.5em', fontVariantNumeric: 'tabular-nums' }}
                  onChange={(e) => setCode(e.target.value.replace(/\D/g, '').slice(0, CODE_LENGTH))}
                  onKeyDown={(e) => e.key === 'Enter' && submitCode()}
                >
                  <TextField.Slot>
                    <LockClosedIcon height="16" width="16" />
                  </TextField.Slot>
                </TextField.Root>

                <Flex justify="between" align="center">
                  <Text size="1" color="gray">
                    {secondsLeft > 0 ? `Expires in ${secondsLeft}s` : 'Code expired'}
                  </Text>
                  <Button
                    size="1"
                    variant="ghost"
                    disabled={secondsLeft > 0 || busy}
                    onClick={() => sendCode(true)}
                  >
                    <ReloadIcon />
                    Resend
                  </Button>
                </Flex>
              </Flex>

              {devCode && (
                <Callout.Root size="1" color="amber" variant="surface">
                  <Callout.Icon>
                    <InfoCircledIcon />
                  </Callout.Icon>
                  <Callout.Text>
                    Development mode — your code is <Badge variant="soft">{devCode}</Badge>
                  </Callout.Text>
                </Callout.Root>
              )}

              <Button
                size="3"
                disabled={code.length !== CODE_LENGTH}
                loading={busy}
                onClick={submitCode}
              >
                Sign in
                <ArrowRightIcon />
              </Button>

              <Separator size="4" />

              <Button
                size="2"
                variant="ghost"
                color="gray"
                onClick={() => {
                  setStep('phone')
                  setCode('')
                }}
              >
                <ArrowLeftIcon />
                Use a different number
              </Button>
            </Flex>
          )}
        </Card>

        <Flex justify="center" mt="4">
          <Text size="1" color="gray">
            Administrator access only
          </Text>
        </Flex>
      </Box>
    </Flex>
  )
}
