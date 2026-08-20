import { useEffect, useState } from 'react'
import { getAccount, getHealth, type ApplicationUser, type AuthenticatedAccount } from './api'
import { LOGIN_INTENT_KEY, Login, type LoginIntent } from './screens/Login'
import { Consent } from './screens/Consent'
import { VerificationRequestFlow } from './screens/VerificationRequest'
import { supabase } from './lib/supabase'
import { lazy, Suspense } from 'react'
import { AppShell, type ShellTab } from './components/AppShell'
import { CitizenDashboard } from './screens/CitizenDashboard'
import { CitizenInbox } from './screens/CitizenInbox'

// Pixi + the scene engine are a lot of bytes for a form-filling app to load up
// front; only the account holders who actually see live activity pay for it.
const LiveFloor = lazy(() => import('./scene/LiveFloor').then((m) => ({ default: m.LiveFloor })))
// Same reasoning as LiveFloor: /demo pulls in the whole Pixi scene, and most
// visitors to the authenticated app never go near it.
const Demo = lazy(() => import('./screens/Demo').then((m) => ({ default: m.Demo })))

function defaultTabFor(account: AuthenticatedAccount): ShellTab {
  if (account.accountType === 'CITIZEN') return 'my-credentials'
  return account.governmentProfile.role === 'CONTROLLER' ? 'monitoring' : 'verify'
}

export default function App() {
	const consentMatch = window.location.pathname.match(/^\/consent\/([^/]+)$/)
	const consentToken = consentMatch ? decodeURIComponent(consentMatch[1]) : ''
	const qrDocumentId = window.location.pathname === '/verify' ? new URLSearchParams(window.location.search).get('docId') ?? '' : ''
	// The demo floor is a standalone page: no login, so it can be projected or
	// handed to a judge without an account.
	const isDemo = window.location.pathname === '/demo'
  const [session, setSession] = useState<any>(null)
  const [tab, setTab] = useState<ShellTab>('verify')
  const [contract, setContract] = useState('')
  const [account, setAccount] = useState<AuthenticatedAccount | null>(null)
  const [profile, setProfile] = useState<ApplicationUser | null>(null)
  const [profileError, setProfileError] = useState('')
  const [loginError, setLoginError] = useState('')
  const [roleMismatchNotice, setRoleMismatchNotice] = useState('')

  useEffect(() => {
    supabase.auth.getSession().then(({ data }) => {
      setSession(data.session)
    })

    const {
      data: { subscription },
    } = supabase.auth.onAuthStateChange((event, session) => {
      if (event === 'SIGNED_OUT') {
        setAccount(null)
        setProfile(null)
      }
      setSession(session)
    })

    return () => subscription.unsubscribe()
  }, [])

  useEffect(() => {
    getHealth()
      .then((health) => setContract(health.contract))
      .catch(() => setContract(''))
  }, [])

  useEffect(() => {
    if (!session) {
      setAccount(null)
      setProfile(null)
      setProfileError('')
      return
    }

    getAccount()
      .then((resolvedAccount) => {
        const intent = sessionStorage.getItem(LOGIN_INTENT_KEY) as LoginIntent | null
        const actualRole = resolvedAccount.accountType === 'GOVERNMENT'
          ? resolvedAccount.governmentProfile.role
          : 'CITIZEN'
        // The selected card controls only the sign-in presentation. The
        // backend-resolved account remains authoritative for the workspace and
        // permissions, so a stale or mistaken card must never sign a valid
        // application account back out — but the user should still be told
        // their card choice didn't match, instead of this happening silently.
        if (intent !== actualRole) {
          sessionStorage.setItem(LOGIN_INTENT_KEY, actualRole)
          if (intent) setRoleMismatchNotice(`Signed in as ${actualRole.toLowerCase()}, not ${intent.toLowerCase()} as selected.`)
        } else {
          setRoleMismatchNotice('')
        }
        setAccount(resolvedAccount)
        setProfile(resolvedAccount.governmentProfile)
        setTab(defaultTabFor(resolvedAccount))
        setProfileError('')
        setLoginError('')
      })
      .catch((error) => {
        setAccount(null)
        setProfile(null)
        setProfileError(error instanceof Error ? error.message : String(error))
      })
  }, [session])

  if (isDemo) {
    return (
      <Suspense fallback={null}>
        <Demo />
      </Suspense>
    )
  }

  if (consentToken) {
    return <Consent token={consentToken} />
  }

  if (!session) {
    return <Login initialError={loginError} />
  }


  async function logout() {
    // Clear this before Supabase emits SIGNED_OUT; Login may mount in response
    // to that event and must not initialize itself with the previous role.
    sessionStorage.removeItem(LOGIN_INTENT_KEY)
    setAccount(null)
    setProfile(null)
    await supabase.auth.signOut()
  }

  if (!account) {
    return (
      <main className="flex min-h-screen items-center justify-center px-6">
        {profileError ? (
          <div className="max-w-lg rounded-xl border border-red-500/50 bg-red-500/10 p-5 text-red-300">{profileError}</div>
        ) : (
          <p className="text-slate-400">Loading application account…</p>
        )}
      </main>
    )
  }

  return (
    <AppShell account={account} contract={contract} activeTab={tab} onTabChange={setTab} onSignOut={logout}>
      {roleMismatchNotice && (
        <div className="mb-4 flex items-center justify-between gap-4 rounded-xl border border-amber-500/50 bg-amber-500/10 px-4 py-3 text-sm text-amber-200">
          <span>{roleMismatchNotice}</span>
          <button type="button" onClick={() => setRoleMismatchNotice('')} className="text-amber-300 hover:text-amber-100">Dismiss</button>
        </div>
      )}

      {profile && (
        <Suspense fallback={null}>
          <LiveFloor profile={profile} />
        </Suspense>
      )}

      {account.accountType === 'CITIZEN' ? (
        tab === 'my-credentials' ? <CitizenDashboard /> : <CitizenInbox />
      ) : profile ? (
          qrDocumentId && profile.role !== 'CONTROLLER' ? <VerificationRequestFlow documentId={qrDocumentId} /> : null
        ) : null}
    </AppShell>
  )
}
