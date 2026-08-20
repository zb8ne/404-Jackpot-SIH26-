import { useCallback, useEffect, useState } from 'react'
import { completeVerificationRequest, docTypeLabel, listVerificationRequests, type VerificationRequest, type VerificationRequestState } from '../api'

const FILTERS: Array<{ label: string; value?: VerificationRequestState }> = [
  { label: 'All' }, { label: 'Pending', value: 'PENDING' }, { label: 'Approved', value: 'APPROVED' },
  { label: 'Denied', value: 'DENIED' }, { label: 'Completed', value: 'COMPLETED' }, { label: 'Expired', value: 'EXPIRED' },
]

export function GovernmentRequests() {
  const [requests, setRequests] = useState<VerificationRequest[]>([])
  const [filter, setFilter] = useState<VerificationRequestState | undefined>()
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    setLoading(true); setError('')
    try { setRequests((await listVerificationRequests(filter)).requests) }
    catch (e) { setError(String(e)) }
    finally { setLoading(false) }
  }, [filter])

  useEffect(() => { void load() }, [load])

  async function complete(id: string) {
    setError('')
    try {
      const result = await completeVerificationRequest(id)
      setRequests((current) => current.map((request) => request.id === id ? result.request : request))
    } catch (e) { setError(String(e)) }
  }

  return <section className="space-y-6">
    <div className="flex flex-wrap items-end justify-between gap-4"><div><p className="text-xs font-semibold uppercase tracking-widest text-sky-400">Consent workflow</p><h2 className="mt-2 text-3xl font-black">Verification requests</h2><p className="mt-2 text-slate-400">Track citizen decisions and complete approved checks.</p></div><button type="button" onClick={() => void load()} className="rounded-xl border border-slate-700 px-4 py-3 text-sm font-semibold text-slate-200 hover:bg-slate-800">Refresh</button></div>
    <div className="flex gap-2 overflow-x-auto">{FILTERS.map((item) => <button key={item.label} type="button" onClick={() => setFilter(item.value)} className={`shrink-0 rounded-lg px-4 py-2 text-sm font-semibold ${filter === item.value ? 'bg-sky-600 text-white' : 'border border-slate-800 text-slate-400'}`}>{item.label}</button>)}</div>
    {error && <div className="rounded-xl border border-red-500/50 bg-red-500/10 p-4 text-red-300">{error}</div>}
    {loading ? <p className="text-slate-400">Loading requests…</p> : requests.length === 0 ? <div className="rounded-2xl border border-slate-800 bg-slate-900/50 p-8 text-slate-400">No requests match this filter.</div> : <div className="space-y-4">{requests.map((request) => <article key={request.id} className="rounded-2xl border border-slate-800 bg-slate-900/60 p-6">
      <div className="flex flex-wrap items-start justify-between gap-4"><div><h3 className="text-xl font-bold">{docTypeLabel(request.documentType)}</h3><p className="mt-1 font-mono text-sm text-slate-400">{request.documentId}</p></div><span className="rounded-full bg-slate-800 px-3 py-1 text-xs font-bold text-sky-300">{request.state}</span></div>
      <dl className="mt-5 grid gap-4 sm:grid-cols-3"><Row label="Purpose" value={request.purpose} /><Row label="Requester" value={request.requesterName || request.requesterEmail} /><Row label="Expires" value={new Date(request.expiresAt).toLocaleString()} /></dl>
      {request.state === 'APPROVED' && <button type="button" onClick={() => void complete(request.id)} className="mt-6 rounded-xl bg-emerald-600 px-5 py-3 font-semibold text-white hover:bg-emerald-500">Complete verification</button>}
      {request.completedResult && <p className="mt-5 text-sm text-emerald-300">Completed result: {request.completedResult.status}</p>}
    </article>)}</div>}
  </section>
}

function Row({ label, value }: { label: string; value: string }) { return <div><dt className="text-xs font-semibold uppercase tracking-widest text-slate-500">{label}</dt><dd className="mt-1 text-slate-200">{value}</dd></div> }
