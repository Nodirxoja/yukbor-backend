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

/**
 * Signs the back office in with a username and password.
 *
 * Deliberately not the phone/SMS flow the mobile app uses: an operator is not a
 * customer, may not hold the account's handset, and should not wait on an SMS
 * to open a dashboard. The server returns the same kind of token either way, so
 * everything downstream is identical — only the way you prove who you are
 * differs.
 */
export function adminLogin(username: string, password: string): Promise<Session> {
  return post<Session>('/auth/admin/login', { username, password })
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

// Phone formatting/validation helpers lived here while the dashboard used the
// SMS flow. They are gone with it — the mobile app owns that flow now.
