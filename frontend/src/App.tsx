import { useEffect, useState } from 'react'
import { getAccount, getHealth, type ApplicationUser, type AuthenticatedAccount } from './api'
import { Citizen } from './screens/Citizen'
import { Issue } from './screens/Issue'
import { Verify } from './screens/Verify'
import { LOGIN_INTENT_KEY, Login, type LoginIntent } from './screens/Login'
import { AuditEvents } from './screens/AuditEvents'
import { Monitoring } from './screens/Monitoring'
import { Revoke, Supersede } from './screens/Lifecycle'
import { Consent } from './screens/Consent'
import { VerificationRequestFlow } from './screens/VerificationRequest'
import { supabase } from './lib/supabase'
import { lazy, Suspense } from 'react'
import { AppShell, type ShellTab } from './components/AppShell'

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

  useEffect(() => {
    supabase.auth.getSession().then(({ data }) => {
      setSession(data.session)
    })

    const {
      data: { subscription },
    } = supabase.auth.onAuthStateChange((_event, session) => {
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
      .then(async (resolvedAccount) => {
        const intent = sessionStorage.getItem(LOGIN_INTENT_KEY) as LoginIntent | null
        const actualRole = resolvedAccount.accountType === 'GOVERNMENT'
          ? resolvedAccount.governmentProfile.role
          : 'CITIZEN'
        if (!intent || intent !== actualRole) {
          setLoginError(!intent
            ? 'Choose an account type before signing in.'
            : `This account is registered as ${actualRole.toLowerCase()}, not ${intent.toLowerCase()}.`)
          await supabase.auth.signOut()
          return
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
    await supabase.auth.signOut()
    sessionStorage.removeItem(LOGIN_INTENT_KEY)
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
      {profile && (
        <Suspense fallback={null}>
          <LiveFloor />
        </Suspense>
      )}

      {account.accountType === 'CITIZEN' ? (
        <section className="rounded-2xl border border-slate-800 bg-slate-900/50 p-8">
          <p className="text-xs font-semibold uppercase tracking-widest text-sky-400">{tab === 'inbox' ? 'Citizen inbox' : 'My credentials'}</p>
          <h2 className="mt-2 text-3xl font-black text-slate-100">
            {tab === 'inbox' ? 'Verification requests' : `Welcome, ${account.citizenProfile.displayName}`}
          </h2>
          <p className="mt-3 text-slate-400">
            {tab === 'inbox' ? 'Your secure request inbox will appear here.' : 'Your issued credential dashboard will appear here.'}
          </p>
        </section>
      ) : profile ? (
          <>
            {qrDocumentId && profile.role !== 'CONTROLLER' ? <VerificationRequestFlow documentId={qrDocumentId} /> : tab === 'verify' && <Verify />}
            {tab === 'issue' && <Issue />}
            {tab === 'revoke' && profile.role === 'ADMIN' && <Revoke />}
            {tab === 'supersede' && profile.role === 'ADMIN' && <Supersede />}
            {tab === 'citizen' && <Citizen />}
            {tab === 'audit' && profile.role !== 'OFFICIAL' && <AuditEvents />}
            {tab === 'monitoring' && profile.role === 'CONTROLLER' && <Monitoring />}
          </>
        ) : null}
    </AppShell>
  )
}
