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

// Pixi + the scene engine are a lot of bytes for a form-filling app to load up
// front; only the account holders who actually see live activity pay for it.
const LiveFloor = lazy(() => import('./scene/LiveFloor').then((m) => ({ default: m.LiveFloor })))
// Same reasoning as LiveFloor: /demo pulls in the whole Pixi scene, and most
// visitors to the authenticated app never go near it.
const Demo = lazy(() => import('./screens/Demo').then((m) => ({ default: m.Demo })))

const TABS = [
  { id: 'verify', label: 'Verify', hint: 'check a document' },
  { id: 'issue', label: 'Issue', hint: 'anchor a new one' },
  { id: 'revoke', label: 'Revoke', hint: 'Admin lifecycle action' },
  { id: 'supersede', label: 'Supersede', hint: 'Admin replacement' },
  { id: 'citizen', label: 'Citizen', hint: 'what someone holds' },
  { id: 'audit', label: 'Audit', hint: 'Phase 2 activity' },
  { id: 'monitoring', label: 'Monitoring', hint: 'system statistics' },
] as const

type Tab = (typeof TABS)[number]['id']

export default function App() {
	const consentMatch = window.location.pathname.match(/^\/consent\/([^/]+)$/)
	const consentToken = consentMatch ? decodeURIComponent(consentMatch[1]) : ''
	const qrDocumentId = window.location.pathname === '/verify' ? new URLSearchParams(window.location.search).get('docId') ?? '' : ''
	// The demo floor is a standalone page: no login, so it can be projected or
	// handed to a judge without an account.
	const isDemo = window.location.pathname === '/demo'
  const [session, setSession] = useState<any>(null)
  const [tab, setTab] = useState<Tab>('verify')
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
        setProfileError('')
        setLoginError('')
      })
      .catch((error) => {
        setAccount(null)
        setProfile(null)
        setProfileError(error instanceof Error ? error.message : String(error))
      })
  }, [session])

  useEffect(() => {
    if (!profile) return
    setTab(profile.role === 'CONTROLLER' ? 'monitoring' : 'verify')
  }, [profile])

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

  return (
    <div className="min-h-screen">
      <header className="border-b border-slate-800 bg-slate-900/60">
        <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-4 px-6 py-5">
          <div>
            <h1 className="text-2xl font-black tracking-tight text-slate-100">
              Government Credential Registry
            </h1>

            <p className="text-sm text-slate-500">
              {contract ? (
                <>
                  contract <span className="font-mono">{contract}</span>
                </>
              ) : (
                'backend unreachable — is `make demo` running?'
              )}
            </p>
          </div>

          <div className="flex items-center gap-4">
            <span className="text-sm text-slate-400">
              {session.user.email}
            </span>

            <button
              onClick={logout}
              className="rounded-lg border border-slate-700 px-4 py-2 text-sm text-slate-300 hover:bg-slate-800"
            >
              Sign out
            </button>
            <nav className="flex flex-wrap gap-2">
              {TABS.filter((candidate) => {
                if (!profile) return false
                if (profile.role === 'CONTROLLER') return candidate.id === 'monitoring' || candidate.id === 'audit'
                if (profile.role === 'ADMIN') return candidate.id !== 'monitoring'
                return candidate.id === 'verify' || candidate.id === 'issue' || candidate.id === 'citizen'
              }).map((t) => (
                <button
                  key={t.id}
                  onClick={() => setTab(t.id)}
                  className={`rounded-xl px-5 py-3 text-left transition ${
                    tab === t.id
                      ? 'bg-sky-600 text-white'
                      : 'text-slate-400 hover:bg-slate-800 hover:text-slate-200'
                  }`}
                >
                  <div className="font-semibold">{t.label}</div>
                  <div className="text-xs opacity-70">{t.hint}</div>
                </button>
              ))}
            </nav>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-6xl px-6 py-10">
        <Suspense fallback={null}>
          <LiveFloor />
        </Suspense>

        {profileError && (
          <div className="rounded-xl border border-red-500/50 bg-red-500/10 p-4 text-red-300">
            {profileError}
          </div>
        )}
        {account?.accountType === 'CITIZEN' ? (
          <section className="rounded-2xl border border-slate-800 bg-slate-900/50 p-8">
            <p className="text-xs font-semibold uppercase tracking-widest text-sky-400">Citizen account</p>
            <h2 className="mt-2 text-3xl font-black text-slate-100">Welcome, {account.citizenProfile.displayName}</h2>
            <p className="mt-3 text-slate-400">Your credential dashboard will be added in the next phase.</p>
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
        ) : !profileError ? (
          <p className="text-slate-400">Loading application profile…</p>
        ) : null}
      </main>
    </div>
  )
}
