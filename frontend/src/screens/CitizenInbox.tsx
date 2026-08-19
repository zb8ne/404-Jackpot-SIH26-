import { useEffect, useState } from 'react'
import { docTypeLabel, listCitizenVerificationRequests, type VerificationRequest } from '../api'

export function CitizenInbox() {
  const [requests, setRequests] = useState<VerificationRequest[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    listCitizenVerificationRequests().then(setRequests).catch((e) => setError(String(e))).finally(() => setLoading(false))
  }, [])

  if (loading) return <p className="text-slate-400">Loading your inbox…</p>
  return <section className="space-y-6">
    <div><p className="text-xs font-semibold uppercase tracking-widest text-sky-400">Citizen inbox</p><h2 className="mt-2 text-3xl font-black">Verification requests</h2><p className="mt-2 text-slate-400">Requests expire 24 hours after they are created.</p></div>
    {error && <div className="rounded-xl border border-red-500/50 bg-red-500/10 p-4 text-red-300">{error}</div>}
    {!error && requests.length === 0 && <div className="rounded-2xl border border-slate-800 bg-slate-900/50 p-8 text-slate-400">Your inbox is empty.</div>}
    <div className="space-y-4">{requests.map((request) => <article key={request.id} className="rounded-2xl border border-slate-800 bg-slate-900/60 p-6">
      <div className="flex flex-wrap items-start justify-between gap-4"><div><h3 className="text-xl font-bold">{docTypeLabel(request.documentType)}</h3><p className="mt-1 font-mono text-sm text-slate-400">{request.documentId}</p></div><StateBadge state={request.state} /></div>
      <dl className="mt-6 grid gap-4 sm:grid-cols-2"><Row label="Requested by" value={`${request.requesterName} · ${request.departmentName}`} /><Row label="Expires" value={new Date(request.expiresAt).toLocaleString()} /><Row label="Purpose" value={request.purpose} /></dl>
      {request.state === 'PENDING' && <p className="mt-5 text-sm text-sky-300">Decision controls are added in the next phase.</p>}
    </article>)}</div>
  </section>
}

function StateBadge({ state }: { state: string }) { return <span className="rounded-full border border-slate-700 bg-slate-800 px-3 py-1 text-xs font-bold text-slate-200">{state}</span> }
function Row({ label, value }: { label: string; value: string }) { return <div><dt className="text-xs font-semibold uppercase tracking-widest text-slate-500">{label}</dt><dd className="mt-1 text-slate-200">{value}</dd></div> }
