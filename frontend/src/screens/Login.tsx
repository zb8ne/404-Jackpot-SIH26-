import { useState } from 'react'
import { supabase } from '../lib/supabase'

export function Login() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function submit(e: React.FormEvent) {
    e.preventDefault()

    setBusy(true)
    setError('')

    const { error } = await supabase.auth.signInWithPassword({
      email,
      password,
    })

    if (error) {
      setError(error.message)
    }

    setBusy(false)
  }

  return (
    <div className="flex min-h-screen items-center justify-center px-6">
      <form
        onSubmit={submit}
        className="w-full max-w-md space-y-6 rounded-2xl border border-slate-800 bg-slate-900/50 p-8"
      >
        <div>
          <h1 className="text-3xl font-black text-slate-100">
            Government Credential Registry
          </h1>

          <p className="mt-2 text-slate-500">
            Sign in to continue
          </p>
        </div>

        {error && (
          <div className="rounded-lg border border-red-500/50 bg-red-500/10 p-4 text-sm text-red-300">
            {error}
          </div>
        )}

        <div>
          <label className="mb-2 block text-sm text-slate-400">
            Email
          </label>

          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            className="w-full rounded-lg border border-slate-700 bg-slate-950 px-4 py-3 text-slate-100 outline-none focus:border-sky-500"
          />
        </div>

        <div>
          <label className="mb-2 block text-sm text-slate-400">
            Password
          </label>

          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            className="w-full rounded-lg border border-slate-700 bg-slate-950 px-4 py-3 text-slate-100 outline-none focus:border-sky-500"
          />
        </div>

        <button
          type="submit"
          disabled={busy}
          className="w-full rounded-xl bg-sky-600 px-6 py-3 font-semibold text-white hover:bg-sky-500 disabled:bg-slate-700"
        >
          {busy ? 'Signing in...' : 'Sign in'}
        </button>
      </form>
    </div>
  )
}
