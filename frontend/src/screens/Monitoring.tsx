import { useEffect, useRef, useState } from 'react'
import { getMonitoringOverview, type MonitoringOverview, type OperationCounts } from '../api'

/** Which section to scroll to on open — the Live Floor's Controller rail links
 *  here (System activity / Integrity / Department health) even though it's
 *  all one dashboard, so a section id tells us where to land. */
export type MonitoringFocus = 'activity' | 'integrity' | 'health'

export function Monitoring({ focus }: { focus?: MonitoringFocus } = {}) {
  const [data, setData] = useState<MonitoringOverview | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const activityRef = useRef<HTMLElement>(null)
  const integrityRef = useRef<HTMLDivElement>(null)
  const healthRef = useRef<HTMLElement>(null)

  function load() {
    setBusy(true)
    setError('')
    getMonitoringOverview()
      .then(setData)
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
      .finally(() => setBusy(false))
  }

  useEffect(load, [])

  useEffect(() => {
    if (!data || !focus) return
    const target = focus === 'activity' ? activityRef.current : focus === 'integrity' ? integrityRef.current : healthRef.current
    target?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }, [data, focus])

  return (
    <div className="space-y-8">
      <ValidationHeader title="Controller monitoring" busy={busy} onRefresh={load} />
      {error && <ErrorPanel message={error} />}
      {data && (
        <>
          <section>
            <h2 className="mb-4 text-xl font-bold text-slate-100">Credential operations</h2>
            <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
              {Object.entries(data.operations).map(([name, counts]) => (
                <OperationCard key={name} name={name} counts={counts} />
              ))}
            </div>
          </section>

          <section className="grid gap-6 lg:grid-cols-2">
            <div className="rounded-2xl border border-slate-800 bg-slate-900/50 p-6">
              <h2 className="text-xl font-bold text-slate-100">Verification results</h2>
              <div className="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-3">
                {Object.entries(data.verificationResults).map(([result, count]) => (
                  <Stat key={result} label={result} value={count} />
                ))}
              </div>
            </div>
            <div ref={integrityRef} className="rounded-2xl border border-slate-800 bg-slate-900/50 p-6">
              <h2 className="text-xl font-bold text-slate-100">Audit integrity</h2>
              <p className={`mt-4 text-3xl font-black ${data.auditIntegrity.valid ? 'text-emerald-400' : 'text-red-400'}`}>
                {data.auditIntegrity.valid ? 'VALID' : 'INVALID'}
              </p>
              <p className="mt-2 text-slate-400">{data.auditIntegrity.eventCount} events checked</p>
              {data.auditIntegrity.firstInvalidEventId && (
                <p className="mt-2 text-red-300">First invalid event: {data.auditIntegrity.firstInvalidEventId}</p>
              )}
            </div>
          </section>

          <section ref={healthRef}>
            <h2 className="mb-4 text-xl font-bold text-slate-100">Departments</h2>
            <div className="overflow-x-auto rounded-2xl border border-slate-800">
              <table className="w-full text-left text-sm">
                <thead className="bg-slate-900 text-slate-400">
                  <tr>{['Department', 'Events', 'Issue', 'Verify', 'Revoke', 'Supersede', 'Failed/denied', 'Last activity'].map((h) => <th key={h} className="px-4 py-3">{h}</th>)}</tr>
                </thead>
                <tbody>
                  {data.departments.map((department) => (
                    <tr key={department.id} className="border-t border-slate-800 text-slate-300">
                      <td className="px-4 py-3 font-semibold">{department.name}</td>
                      <td className="px-4 py-3">{department.events}</td>
                      <td className="px-4 py-3">{department.issue}</td>
                      <td className="px-4 py-3">{department.verify}</td>
                      <td className="px-4 py-3">{department.revoke}</td>
                      <td className="px-4 py-3">{department.supersede}</td>
                      <td className="px-4 py-3">{department.failure + department.denied + department.partialFailure}</td>
                      <td className="px-4 py-3">{formatTime(department.lastActivityAt)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </section>

          <section ref={activityRef}>
            <h2 className="mb-4 text-xl font-bold text-slate-100">Recent activity</h2>
            <div className="space-y-3">
              {data.recentActivity.map((event) => (
                <div key={event.eventId} className="rounded-xl border border-slate-800 bg-slate-900/50 p-4 text-sm">
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <span className="font-semibold text-slate-100">{event.action} · {event.outcome}</span>
                    <span className="text-slate-500">{formatTime(event.createdAt)}</span>
                  </div>
                  <p className="mt-2 text-slate-400">{event.actor.name || event.actor.email || event.actor.id} ({event.actor.role}) · {event.department?.name ?? 'System'}</p>
                </div>
              ))}
              {!data.recentActivity.length && <p className="text-slate-500">No audit activity yet.</p>}
            </div>
          </section>
        </>
      )}
    </div>
  )
}

function OperationCard({ name, counts }: { name: string; counts: OperationCounts }) {
  return (
    <div className="rounded-2xl border border-slate-800 bg-slate-900/50 p-5">
      <div className="text-xs font-semibold uppercase tracking-widest text-slate-500">{name}</div>
      <div className="mt-2 text-4xl font-black text-slate-100">{counts.total}</div>
      <div className="mt-3 text-sm text-slate-400">{counts.success} success · {counts.failure} failed · {counts.denied} denied · {counts.partialFailure} partial</div>
    </div>
  )
}

function Stat({ label, value }: { label: string; value: number }) {
  return <div className="rounded-xl bg-slate-950 p-3"><div className="text-xs text-slate-500">{label}</div><div className="mt-1 text-2xl font-bold text-slate-200">{value}</div></div>
}

export function ValidationHeader({ title, busy, onRefresh }: { title: string; busy: boolean; onRefresh: () => void }) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-4">
      <div><h1 className="text-3xl font-black text-slate-100">{title}</h1><p className="mt-1 text-sm text-amber-300">Temporary Phase 2 validation UI</p></div>
      <button type="button" onClick={onRefresh} disabled={busy} className="rounded-lg border border-slate-700 px-4 py-2 text-slate-300 hover:bg-slate-800 disabled:opacity-50">{busy ? 'Loading…' : 'Refresh'}</button>
    </div>
  )
}

export function ErrorPanel({ message }: { message: string }) {
  return <div className="rounded-xl border border-red-500/50 bg-red-500/10 p-4 text-red-300">{message}</div>
}

function formatTime(value: string | null) {
  return value ? new Date(value).toLocaleString() : '—'
}
