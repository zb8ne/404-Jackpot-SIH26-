import { useEffect, useState } from 'react'
import { getAuditEvents, type AuditAction, type AuditEvent, type AuditOutcome } from '../api'
import { ErrorPanel, ValidationHeader } from './Monitoring'

const ACTIONS: Array<AuditAction | ''> = ['', 'ISSUE', 'VERIFY_FILE', 'VERIFY_ID', 'REVOKE', 'SUPERSEDE', 'USER_PROFILE_CREATE', 'USER_PROFILE_UPDATE', 'VERIFICATION_REQUEST_CREATED', 'CONSENT_NOTIFICATION', 'CONSENT_APPROVED', 'CONSENT_DENIED', 'VERIFICATION_REQUEST_EXPIRED', 'VERIFICATION_REQUEST_COMPLETED', 'CONSENT_TOKEN_REJECTED']
const OUTCOMES: Array<AuditOutcome | ''> = ['', 'SUCCESS', 'FAILURE', 'DENIED', 'PARTIAL_FAILURE']

export function AuditEvents() {
  const [events, setEvents] = useState<AuditEvent[]>([])
  const [action, setAction] = useState<AuditAction | ''>('')
  const [outcome, setOutcome] = useState<AuditOutcome | ''>('')
  const [nextBefore, setNextBefore] = useState<number | null>(null)
  const [hasMore, setHasMore] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  function load(before?: number, append = false) {
    setBusy(true)
    setError('')
    getAuditEvents({ limit: 25, before, action: action || undefined, outcome: outcome || undefined })
      .then((response) => {
        setEvents((current) => append ? [...current, ...response.events] : response.events)
        setNextBefore(response.page.nextBefore)
        setHasMore(response.page.hasMore)
      })
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
      .finally(() => setBusy(false))
  }

  useEffect(() => { load() }, [action, outcome])

  return (
    <div className="space-y-6">
      <ValidationHeader title="Audit events" busy={busy} onRefresh={() => load()} />
      <div className="flex flex-wrap gap-3 rounded-xl border border-slate-800 bg-slate-900/50 p-4">
        <Filter label="Action" value={action} options={ACTIONS} onChange={(value) => setAction(value as AuditAction | '')} />
        <Filter label="Outcome" value={outcome} options={OUTCOMES} onChange={(value) => setOutcome(value as AuditOutcome | '')} />
      </div>
      {error && <ErrorPanel message={error} />}
      <div className="space-y-4">
        {events.map((event) => <EventCard key={event.eventId} event={event} />)}
        {!busy && !events.length && !error && <p className="text-slate-500">No matching audit events.</p>}
      </div>
      {hasMore && nextBefore && (
        <button type="button" disabled={busy} onClick={() => load(nextBefore, true)} className="w-full rounded-md border-2 border-slate-700 py-3 font-bold uppercase tracking-wide text-slate-300 shadow-[3px_3px_0_0_rgba(100,116,139,0.3)] transition-all duration-100 hover:-translate-x-0.5 hover:-translate-y-0.5 hover:border-slate-500 active:translate-x-0 active:translate-y-0 active:shadow-none disabled:opacity-50 disabled:shadow-none">Load older events</button>
      )}
    </div>
  )
}

function EventCard({ event }: { event: AuditEvent }) {
  const credential = event.credential
  return (
    <article className="rounded-2xl border border-slate-800 bg-slate-900/50 p-5">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-bold text-slate-100">{event.action}</span>
            <span className={`rounded-full border px-2 py-1 text-xs font-bold ${outcomeClass(event.outcome)}`}>{event.outcome}</span>
            {event.result && <span className="text-sm text-slate-400">{event.result}</span>}
          </div>
          <p className="mt-2 text-sm text-slate-400">{event.actor.name || event.actor.email || event.actor.id} · {event.actor.role} · {event.department?.name ?? 'System'}</p>
        </div>
        <time className="text-sm text-slate-500">{new Date(event.createdAt).toLocaleString()}</time>
      </div>
      {(credential.docId || credential.docHash || credential.citizen || credential.transactionHash || credential.referenceHash) && (
        <dl className="mt-4 grid gap-3 border-t border-slate-800 pt-4 text-sm sm:grid-cols-2 lg:grid-cols-3">
          <Field label="Document ID" value={credential.docId} mono />
          <Field label="Citizen" value={credential.citizen} />
          <Field label="Document hash" value={credential.docHash} mono />
          <Field label="Transaction" value={credential.transactionHash} mono />
          <Field label="Reference hash" value={credential.referenceHash} mono />
        </dl>
      )}
      {event.error && <p className="mt-4 rounded-lg bg-red-500/10 p-3 text-sm text-red-300">{event.error}</p>}
    </article>
  )
}

function Filter({ label, value, options, onChange }: { label: string; value: string; options: string[]; onChange: (value: string) => void }) {
  return <label className="text-sm text-slate-400">{label}<select value={value} onChange={(e) => onChange(e.target.value)} className="ml-2 rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-slate-200"><option value="">All</option>{options.filter(Boolean).map((option) => <option key={option} value={option}>{option}</option>)}</select></label>
}

function Field({ label, value, mono }: { label: string; value: string | null; mono?: boolean }) {
  if (!value) return null
  return <div><dt className="text-xs uppercase tracking-wider text-slate-600">{label}</dt><dd className={`mt-1 break-all text-slate-300 ${mono ? 'font-mono' : ''}`}>{value}</dd></div>
}

function outcomeClass(outcome: AuditOutcome) {
  if (outcome === 'SUCCESS') return 'border-emerald-500/40 bg-emerald-500/10 text-emerald-300'
  if (outcome === 'DENIED') return 'border-amber-500/40 bg-amber-500/10 text-amber-300'
  return 'border-red-500/40 bg-red-500/10 text-red-300'
}
