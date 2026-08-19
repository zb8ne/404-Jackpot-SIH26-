import { useState } from 'react'
import { supabase } from '../lib/supabase'

export type LoginIntent = 'CONTROLLER' | 'ADMIN' | 'OFFICIAL' | 'CITIZEN'

export const LOGIN_INTENT_KEY = 'credreg.loginIntent'

const INTENT_COPY: Record<LoginIntent, { title: string; description: string }> = {
  CONTROLLER: { title: 'Controller', description: 'System monitoring and audit oversight' },
  ADMIN: { title: 'Department Admin', description: 'Issue credentials and manage their lifecycle' },
  OFFICIAL: { title: 'Department Official', description: 'Issue and verify department credentials' },
  CITIZEN: { title: 'Citizen', description: 'View credentials and respond to access requests' },
}

export function Login({ initialError = '' }: { initialError?: string }) {
  const [intent, setIntent] = useState<LoginIntent | null>(() => {
    const saved = sessionStorage.getItem(LOGIN_INTENT_KEY)
    return saved && saved in INTENT_COPY ? (saved as LoginIntent) : null
  })
  const [authorityOpen, setAuthorityOpen] = useState(false)
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState(initialError)

  function choose(next: LoginIntent) {
    sessionStorage.setItem(LOGIN_INTENT_KEY, next)
    setIntent(next)
    setError('')
  }

  function back() {
    sessionStorage.removeItem(LOGIN_INTENT_KEY)
    setIntent(null)
    setAuthorityOpen(false)
    setPassword('')
    setError('')
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    if (!intent) return

    setBusy(true)
    setError('')
    const { error: signInError } = await supabase.auth.signInWithPassword({ email, password })
    if (signInError) setError(signInError.message)
    setBusy(false)
  }

  return (
    <div className="flex min-h-screen items-center justify-center px-6 py-12">
      <div className="w-full max-w-5xl">
        <div className="mb-8 text-center">
          <p className="text-xs font-semibold uppercase tracking-[0.3em] text-sky-400">Secure credential registry</p>
          <h1 className="mt-3 text-4xl font-black tracking-tight text-slate-100">Government Credential Registry</h1>
          <p className="mx-auto mt-3 max-w-2xl text-slate-400">
            Choose how you use the registry. Your permissions are always confirmed by the backend after sign-in.
          </p>
        </div>

        {intent ? (
          <LoginForm
            intent={intent}
            email={email}
            password={password}
            busy={busy}
            error={error}
            onEmail={setEmail}
            onPassword={setPassword}
            onBack={back}
            onSubmit={submit}
          />
        ) : (
          <div className="grid gap-4 md:grid-cols-3">
            <RoleCard
              eyebrow="Oversight"
              title="Controller"
              description="Monitor registry activity and inspect system-wide audit history."
              onClick={() => choose('CONTROLLER')}
            />

            <div className="rounded-2xl border border-slate-800 bg-slate-900/50 p-6">
              <button type="button" onClick={() => setAuthorityOpen((open) => !open)} className="w-full text-left">
                <span className="text-xs font-semibold uppercase tracking-widest text-emerald-400">Government</span>
                <span className="mt-3 block text-2xl font-bold text-slate-100">Government Authority</span>
                <span className="mt-2 block text-sm leading-6 text-slate-400">Department staff who issue and verify credentials.</span>
                <span className="mt-5 block text-sm font-semibold text-emerald-300">{authorityOpen ? 'Hide account types ↑' : 'Choose account type ↓'}</span>
              </button>

              {authorityOpen && (
                <div className="mt-5 grid gap-3 border-t border-slate-800 pt-5">
                  <AuthorityChoice title="Admin" description="Issue, revoke, and supersede" onClick={() => choose('ADMIN')} />
                  <AuthorityChoice title="Official" description="Issue and verify credentials" onClick={() => choose('OFFICIAL')} />
                </div>
              )}
            </div>

            <RoleCard
              eyebrow="Personal"
              title="Citizen"
              description="Access your issued credentials and respond to verification requests."
              onClick={() => choose('CITIZEN')}
            />
          </div>
        )}
      </div>
    </div>
  )
}

function LoginForm({
  intent,
  email,
  password,
  busy,
  error,
  onEmail,
  onPassword,
  onBack,
  onSubmit,
}: {
  intent: LoginIntent
  email: string
  password: string
  busy: boolean
  error: string
  onEmail: (value: string) => void
  onPassword: (value: string) => void
  onBack: () => void
  onSubmit: (event: React.FormEvent) => void
}) {
  const copy = INTENT_COPY[intent]
  return (
    <form onSubmit={onSubmit} className="mx-auto w-full max-w-md space-y-6 rounded-2xl border border-slate-800 bg-slate-900/60 p-8 shadow-2xl shadow-slate-950/40">
      <div>
        <button type="button" onClick={onBack} className="mb-5 text-sm font-semibold text-slate-400 hover:text-slate-200">← Change account type</button>
        <p className="text-xs font-semibold uppercase tracking-widest text-sky-400">{copy.description}</p>
        <h2 className="mt-2 text-3xl font-black text-slate-100">{copy.title} sign in</h2>
      </div>

      {error && <div role="alert" className="rounded-lg border border-red-500/50 bg-red-500/10 p-4 text-sm text-red-300">{error}</div>}

      <label className="block">
        <span className="mb-2 block text-sm text-slate-400">Email</span>
        <input
          type="email"
          autoComplete="email"
          value={email}
          onChange={(event) => onEmail(event.target.value)}
          required
          className="w-full rounded-lg border border-slate-700 bg-slate-950 px-4 py-3 text-slate-100 outline-none focus:border-sky-500"
        />
      </label>

      <label className="block">
        <span className="mb-2 block text-sm text-slate-400">Password</span>
        <input
          type="password"
          autoComplete="current-password"
          value={password}
          onChange={(event) => onPassword(event.target.value)}
          required
          className="w-full rounded-lg border border-slate-700 bg-slate-950 px-4 py-3 text-slate-100 outline-none focus:border-sky-500"
        />
      </label>

      <button type="submit" disabled={busy} className="w-full rounded-xl bg-sky-600 px-6 py-3 font-semibold text-white hover:bg-sky-500 disabled:bg-slate-700">
        {busy ? 'Signing in…' : `Sign in as ${copy.title}`}
      </button>
    </form>
  )
}

function RoleCard({ eyebrow, title, description, onClick }: { eyebrow: string; title: string; description: string; onClick: () => void }) {
  return (
    <button type="button" onClick={onClick} className="rounded-2xl border border-slate-800 bg-slate-900/50 p-6 text-left transition hover:-translate-y-1 hover:border-sky-500/60 hover:bg-slate-900">
      <span className="text-xs font-semibold uppercase tracking-widest text-sky-400">{eyebrow}</span>
      <span className="mt-3 block text-2xl font-bold text-slate-100">{title}</span>
      <span className="mt-2 block text-sm leading-6 text-slate-400">{description}</span>
      <span className="mt-5 block text-sm font-semibold text-sky-300">Continue →</span>
    </button>
  )
}

function AuthorityChoice({ title, description, onClick }: { title: string; description: string; onClick: () => void }) {
  return (
    <button type="button" onClick={onClick} className="rounded-xl border border-slate-700 bg-slate-950/60 px-4 py-3 text-left transition hover:border-emerald-500/60">
      <span className="font-semibold text-slate-100">{title}</span>
      <span className="mt-1 block text-xs text-slate-500">{description}</span>
    </button>
  )
}
