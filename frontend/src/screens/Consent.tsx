import { useEffect, useState } from 'react'
import { decideConsent, docTypeLabel, getConsentDetails, type ConsentDetails } from '../api'

export function Consent({ token }: { token: string }) {
  const [details, setDetails] = useState<ConsentDetails | null>(null)
  const [result, setResult] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    getConsentDetails(token).then(setDetails).catch((e) => setError(message(e)))
  }, [token])

  async function decide(decision: 'approve' | 'deny') {
    setBusy(true); setError('')
    try {
      const response = await decideConsent(token, decision, details?.requestId)
      setResult(response.state)
      setDetails((current) => current ? { ...current, state: response.state } : current)
    } catch (e) { setError(message(e)) } finally { setBusy(false) }
  }

  return <main className="flex min-h-screen items-center justify-center px-6 py-12">
    <section className="w-full max-w-xl rounded-2xl border border-slate-800 bg-slate-900/60 p-8">
      <p className="text-xs font-semibold uppercase tracking-widest text-sky-400">Citizen consent</p>
      <h1 className="mt-2 text-3xl font-black text-slate-100">Credential verification request</h1>
      {error && <div className="mt-6 rounded-xl border border-red-500/50 bg-red-500/10 p-4 text-red-300">{error}</div>}
      {!details && !error && <p className="mt-6 text-slate-400">Loading request…</p>}
      {details && <>
        <dl className="mt-6 space-y-4 text-sm">
          <Row label="Department" value={details.department.name} />
          <Row label="Requesting official" value={`${details.requester.name || 'Government user'} (${details.requester.role})`} />
          <Row label="Document type" value={docTypeLabel(details.documentType)} />
          <Row label="Purpose" value={details.purpose} />
          <Row label="Expires" value={new Date(details.expiresAt).toLocaleString()} />
        </dl>
        {result ? <div className={`mt-8 rounded-xl p-5 font-bold ${result === 'APPROVED' ? 'bg-emerald-500/10 text-emerald-300' : 'bg-amber-500/10 text-amber-300'}`}>Request {result.toLowerCase()}. This one-time link cannot be used to change the decision.</div> :
          <div className="mt-8 grid gap-3 sm:grid-cols-2">
            <button disabled={busy} onClick={() => void decide('approve')} className="rounded-xl bg-emerald-600 px-6 py-4 font-bold text-white disabled:opacity-50">Approve</button>
            <button disabled={busy} onClick={() => void decide('deny')} className="rounded-xl bg-red-600 px-6 py-4 font-bold text-white disabled:opacity-50">Deny</button>
          </div>}
        <p className="mt-5 text-xs text-slate-500">Consent permits this request only. Credential authenticity is checked separately against the blockchain.</p>
      </>}
    </section>
  </main>
}

function Row({ label, value }: { label: string; value: string }) { return <div><dt className="text-slate-500">{label}</dt><dd className="mt-1 text-slate-200">{value}</dd></div> }
function message(error: unknown) { return error instanceof Error ? error.message : String(error) }
