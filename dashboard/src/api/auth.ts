// Dashboard authentication.
//
// The dashboard signs in as a real user through the same endpoints the iOS app
// uses — phone, SMS code, token. It deliberately does NOT have a token handed
// to it: a build-time VITE_* value ends up inlined in a public bundle, and a
// reverse proxy injecting one means the browser can never tell who it is or
// show a sensible session.
//
// The token lives in sessionStorage, so it dies with the tab and is never
// written to disk. That is XSS-reachable in principle; for a back office behind
// an admin-role check it is the right trade, and the alternative (a token baked
// into JavaScript served to everyone) is strictly worse.

import type { User } from './types'

const SESSION_KEY = 'yukbor.dashboard.session'

export interface Session {
  accessToken: string
  refreshToken: string
  user: User
}

/** ApiError carries the contract's error code so callers can branch on it. */
export class ApiError extends Error {
  constructor(
    readonly code: string,
    message: string,
    readonly status: number,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

async function post<T>(path: string, body: unknown): Promise<T> {
  let res: Response
  try {
    res = await fetch(`/api${path}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
  } catch {
    throw new ApiError('NETWORK', 'Cannot reach the server. Check your connection.', 0)
  }

  if (!res.ok) {
    // Every error in the contract is { error: { code, message } }.
    const payload = await res.json().catch(() => null)
    const code = payload?.error?.code ?? 'UNKNOWN'
    const message = payload?.error?.message ?? `Request failed (${res.status})`
    throw new ApiError(code, message, res.status)
  }
  return res.json() as Promise<T>
}

export interface OtpChallenge {
  verificationId: string
  expiresInSeconds: number
  /** Present outside production only — lets local dev skip reading an SMS. */
  devCode?: string
}

export function requestOtp(phoneNumber: string): Promise<OtpChallenge> {
  return post<OtpChallenge>('/auth/otp/request', { phoneNumber })
}

export function verifyOtp(verificationId: string, code: string): Promise<{ verified: boolean }> {
  return post('/auth/otp/verify', { verificationId, code })
}

/**
 * Exchanges a confirmed phone for tokens. The contract sends only phoneNumber;
 * the server checks that this number completed an OTP recently.
 */
export function login(phoneNumber: string): Promise<Session> {
  return post<Session>('/auth/login', { phoneNumber })
}

export function logout(session: Session | null): void {
  if (session) {
    // Best effort — the local session is cleared regardless.
    void post('/auth/logout', { refreshToken: session.refreshToken }).catch(() => {})
  }
  sessionStorage.removeItem(SESSION_KEY)
}

export function loadSession(): Session | null {
  const raw = sessionStorage.getItem(SESSION_KEY)
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as Session
    return parsed?.accessToken && parsed?.user ? parsed : null
  } catch {
    sessionStorage.removeItem(SESSION_KEY)
    return null
  }
}

export function saveSession(session: Session): void {
  sessionStorage.setItem(SESSION_KEY, JSON.stringify(session))
}

/** Formats +998901234567 as +998 90 123 45 67 for display. */
export function formatPhone(phone: string): string {
  const d = phone.replace(/\D/g, '')
  if (d.length !== 12) return phone
  return `+${d.slice(0, 3)} ${d.slice(3, 5)} ${d.slice(5, 8)} ${d.slice(8, 10)} ${d.slice(10)}`
}

/** Normalises whatever the user typed into the +998XXXXXXXXX the API wants. */
export function normalisePhone(input: string): string {
  const digits = input.replace(/\D/g, '')
  const national = digits.startsWith('998') ? digits.slice(3) : digits
  return `+998${national}`.slice(0, 13)
}

export function isValidPhone(input: string): boolean {
  return /^\+998\d{9}$/.test(normalisePhone(input))
}
