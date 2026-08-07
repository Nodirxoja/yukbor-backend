import { useCallback, useState } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { loadSession, logout as endSession } from './api/auth'
import type { Session } from './api/auth'
import { LoginScreen } from './components/LoginScreen'
import { useToast } from './components/Toaster'
import { DataProvider } from './data/DataProvider'
import { AppShell } from './shell/AppShell'
import { OverviewPage } from './pages/OverviewPage'
import { OrdersPage } from './pages/OrdersPage'
import { OrderDetailPage } from './pages/OrderDetailPage'
import { UsersPage } from './pages/UsersPage'
import { UserDetailPage } from './pages/UserDetailPage'
import { FinancePage } from './pages/FinancePage'

// Opt-IN, never opt-out: a build with no flag talks to the real backend.
const USE_MOCKS = import.meta.env.VITE_USE_MOCKS === 'true'

export default function App() {
  const toast = useToast()
  const [session, setSession] = useState<Session | null>(() => (USE_MOCKS ? null : loadSession()))
  const [authed, setAuthed] = useState(USE_MOCKS || Boolean(loadSession()))

  const signOut = useCallback(() => {
    endSession(session)
    setSession(null)
    setAuthed(false)
  }, [session])

  // A dead token should return to sign-in with an explanation, not leave a
  // dashboard quietly showing numbers that stopped moving.
  const handleUnauthorized = useCallback(
    (message: string) => {
      toast.error('Signed out', message)
      signOut()
    },
    [toast, signOut],
  )

  if (!authed) {
    return (
      <LoginScreen
        onSignedIn={(s) => {
          setSession(s)
          setAuthed(true)
        }}
      />
    )
  }

  return (
    <DataProvider onUnauthorized={handleUnauthorized}>
      <AppShell session={session} onSignOut={signOut}>
        <Routes>
          <Route path="/" element={<OverviewPage />} />
          <Route path="/orders" element={<OrdersPage />} />
          <Route path="/orders/:id" element={<OrderDetailPage />} />
          <Route path="/users" element={<UsersPage />} />
          <Route path="/users/:id" element={<UserDetailPage />} />
          <Route path="/finance" element={<FinancePage />} />
          {/* An unknown path is a typo or a stale link, not an error page. */}
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </AppShell>
    </DataProvider>
  )
}
