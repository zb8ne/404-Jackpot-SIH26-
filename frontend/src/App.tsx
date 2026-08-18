import { useEffect, useState } from 'react'
import { getHealth, getMe, type ApplicationUser } from './api'
import { Citizen } from './screens/Citizen'
import { Issue } from './screens/Issue'
import { Verify } from './screens/Verify'
import { Login } from './screens/Login'
import { supabase } from './lib/supabase'

const TABS = [
  { id: 'verify', label: 'Verify', hint: 'check a document' },
  { id: 'issue', label: 'Issue', hint: 'anchor a new one' },
  { id: 'citizen', label: 'Citizen', hint: 'what someone holds' },
] as const

type Tab = (typeof TABS)[number]['id']

export default function App() {
  const [session, setSession] = useState<any>(null)
  const [tab, setTab] = useState<Tab>('verify')
  const [contract, setContract] = useState('')
  const [profile, setProfile] = useState<ApplicationUser | null>(null)
  const [profileError, setProfileError] = useState('')

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
      setProfile(null)
      setProfileError('')
      return
    }

    getMe()
      .then((me) => {
        setProfile(me)
        setProfileError('')
      })
      .catch((error) => {
        setProfile(null)
        setProfileError(error instanceof Error ? error.message : String(error))
      })
  }, [session])

  if (!session) {
    return <Login />
  }


  async function logout() {
    await supabase.auth.signOut()
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
            {profile?.role !== 'CONTROLLER' && <nav className="flex gap-2">
              {TABS.map((t) => (
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
            </nav>}
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-6xl px-6 py-10">
        {profileError && (
          <div className="rounded-xl border border-red-500/50 bg-red-500/10 p-4 text-red-300">
            {profileError}
          </div>
        )}
        {profile?.role === 'CONTROLLER' ? (
          <div className="rounded-2xl border border-slate-800 bg-slate-900/50 p-8 text-slate-300">
            Controller monitoring will be added in the audit phase. This role cannot perform credential operations.
          </div>
        ) : profile ? (
          <>
            {tab === 'verify' && <Verify />}
            {tab === 'issue' && <Issue />}
            {tab === 'citizen' && <Citizen />}
          </>
        ) : !profileError ? (
          <p className="text-slate-400">Loading application profile…</p>
        ) : null}
      </main>
    </div>
  )
}
