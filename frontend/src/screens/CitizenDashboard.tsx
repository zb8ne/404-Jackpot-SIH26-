import { useEffect, useState } from 'react'
import { docTypeLabel, downloadDocument, getMyCredentials, type Credential } from '../api'
import { Hash } from '../components/Hash'
import { verdictOf } from '../verdicts'

export function CitizenDashboard() {
  const [documents, setDocuments] = useState<Credential[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    getMyCredentials().then(setDocuments).catch((e) => setError(String(e))).finally(() => setLoading(false))
  }, [])

  if (loading) return <p className="text-slate-400">Loading your credentials…</p>
  return (
    <section className="space-y-6">
      <div>
        <p className="text-xs font-semibold uppercase tracking-widest text-sky-400">Citizen wallet</p>
        <h2 className="mt-2 text-3xl font-black text-slate-100">My credentials</h2>
        <p className="mt-2 text-slate-400">Stamped documents issued directly to your account.</p>
      </div>
      {error && <div className="rounded-xl border border-red-500/50 bg-red-500/10 p-4 text-red-300">{error}</div>}
      {!error && documents.length === 0 && <div className="rounded-2xl border border-slate-800 bg-slate-900/50 p-8 text-slate-400">No credentials have been issued to this account.</div>}
      <div className="grid gap-5 lg:grid-cols-2">
        {documents.map((doc) => {
          const verdict = verdictOf(doc.status)
          return <article key={doc.docHash} className="rounded-2xl border border-slate-800 bg-slate-900/60 p-6">
            <div className="flex items-start justify-between gap-4">
              <div><h3 className="text-xl font-bold">{docTypeLabel(doc.docType)}</h3><p className="mt-1 font-mono text-sm text-slate-400">{doc.docId}</p></div>
              <span className={`rounded-full border px-3 py-1 text-xs font-bold ${verdict.badge}`}>{verdict.icon} {verdict.label}</span>
            </div>
            <dl className="mt-6 space-y-3 text-sm">
              <Row label="Issued" value={doc.issuedAt} />
              <Row label="Document hash" value={<Hash value={doc.docHash} />} />
            </dl>
            <button type="button" onClick={() => void downloadDocument(`/citizen/documents/${doc.docHash}/download`, doc.filename).catch((e) => setError(String(e)))} className="mt-6 w-full rounded-xl bg-sky-600 px-4 py-3 font-semibold text-white hover:bg-sky-500">Download stamped PDF</button>
          </article>
        })}
      </div>
    </section>
  )
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return <div><dt className="text-xs font-semibold uppercase tracking-widest text-slate-500">{label}</dt><dd className="mt-1 text-slate-300">{value}</dd></div>
}
