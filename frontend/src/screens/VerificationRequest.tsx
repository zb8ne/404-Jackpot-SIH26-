import { useEffect, useState } from 'react'
import { completeVerificationRequest, createVerificationRequest, getVerificationRequest, type VerificationRequest, type VerifyResult } from '../api'
import { VerdictPanel } from './Verify'

export function VerificationRequestFlow({ documentId }: { documentId: string }) {
  const [purpose, setPurpose] = useState('')
  const [request, setRequest] = useState<VerificationRequest | null>(null)
  const [requestId, setRequestId] = useState('')
  const [verification, setVerification] = useState<VerifyResult | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function create(e: React.FormEvent) {
    e.preventDefault(); setBusy(true); setError('')
    try {
      const created = await createVerificationRequest(documentId, purpose.trim())
      setRequestId(created.id)
      setRequest(await getVerificationRequest(created.id))
    } catch (e) { setError(message(e)) } finally { setBusy(false) }
  }

  async function refresh() {
    if (!requestId) return
    setBusy(true); setError('')
    try { setRequest(await getVerificationRequest(requestId)) } catch (e) { setError(message(e)) } finally { setBusy(false) }
  }

  async function complete() {
    if (!requestId) return
    setBusy(true); setError('')
    try { const result = await completeVerificationRequest(requestId); setRequest(result.request); setVerification(result.verification) } catch (e) { setError(message(e)) } finally { setBusy(false) }
  }

  useEffect(() => { if (requestId) void refresh() }, [requestId])

  return <div className="mx-auto max-w-3xl space-y-6">
    <div><p className="text-xs font-semibold uppercase tracking-widest text-sky-400">QR verification</p><h1 className="mt-2 text-3xl font-black text-slate-100">Request citizen consent</h1><p className="mt-2 text-slate-400">Document <span className="font-mono text-slate-200">{documentId}</span> requires one-time citizen approval before its registry result is shown.</p></div>
    {error && <div className="rounded-xl border border-red-500/50 bg-red-500/10 p-4 text-red-300">{error}</div>}
    {!request && <form onSubmit={create} className="rounded-2xl border border-slate-800 bg-slate-900/50 p-8"><label className="text-sm text-slate-400">Verification purpose<textarea value={purpose} onChange={(e) => setPurpose(e.target.value)} required maxLength={500} className="mt-2 min-h-28 w-full rounded-md border-2 border-slate-700 bg-slate-950 p-4 text-slate-100" placeholder="Why is this credential being verified?" /></label><button disabled={busy || !purpose.trim()} className="mt-5 w-full rounded-md border-2 border-sky-900 bg-sky-600 px-6 py-4 font-bold uppercase tracking-wide text-white shadow-[4px_4px_0_0_rgba(14,165,233,0.5)] transition-all duration-100 hover:-translate-x-0.5 hover:-translate-y-0.5 active:translate-x-0 active:translate-y-0 active:shadow-none disabled:opacity-50 disabled:shadow-none">{busy ? 'Creating…' : 'Send consent request'}</button></form>}
    {request && <section className="rounded-2xl border border-slate-800 bg-slate-900/50 p-8"><div className="flex flex-wrap items-center justify-between gap-3"><h2 className="text-xl font-bold text-slate-100">Request status</h2><span className="rounded-full bg-slate-800 px-3 py-1 font-bold text-sky-300">{request.state}</span></div><dl className="mt-5 grid gap-4 sm:grid-cols-2"><Row label="Request ID" value={request.id} mono /><Row label="Department" value={request.departmentName} /><Row label="Requester" value={request.requesterName || request.requesterEmail} /><Row label="Purpose" value={request.purpose} /><Row label="Expires" value={new Date(request.expiresAt).toLocaleString()} /><Row label="Delivery" value="Citizen web inbox" /></dl><div className="mt-6 flex gap-3"><button disabled={busy} onClick={() => void refresh()} className="rounded-md border-2 border-slate-700 px-5 py-3 font-bold uppercase tracking-wide text-slate-200 shadow-[3px_3px_0_0_rgba(100,116,139,0.3)] transition-all duration-100 hover:-translate-x-0.5 hover:-translate-y-0.5 hover:border-slate-500 active:translate-x-0 active:translate-y-0 active:shadow-none disabled:shadow-none">Refresh</button>{request.state === 'APPROVED' && <button disabled={busy} onClick={() => void complete()} className="rounded-md border-2 border-emerald-900 bg-emerald-600 px-5 py-3 font-bold uppercase tracking-wide text-white shadow-[3px_3px_0_0_rgba(16,185,129,0.5)] transition-all duration-100 hover:-translate-x-0.5 hover:-translate-y-0.5 active:translate-x-0 active:translate-y-0 active:shadow-none disabled:shadow-none">Complete verification</button>}</div>{request.state === 'DENIED' && <p className="mt-5 text-red-300">The citizen denied this request. Verification cannot proceed.</p>}{request.state === 'EXPIRED' && <p className="mt-5 text-amber-300">This request expired. Create a new request if verification is still needed.</p>}</section>}
    {verification && <VerdictPanel result={verification} />}
  </div>
}

function Row({ label, value, mono }: { label: string; value: string; mono?: boolean }) { return <div><dt className="text-xs uppercase tracking-wider text-slate-500">{label}</dt><dd className={`mt-1 break-all text-slate-200 ${mono ? 'font-mono' : ''}`}>{value || '—'}</dd></div> }
function message(error: unknown) { return error instanceof Error ? error.message : String(error) }
