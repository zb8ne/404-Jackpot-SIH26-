import { useEffect, useState } from 'react'
import { docTypeLabel, downloadDocument, getCitizens, getCredentials, type Credential } from '../api'
import { Hash } from '../components/Hash'
import { verdictOf } from '../verdicts'

export function Citizen() {
  const [citizens, setCitizens] = useState<string[]>([])
  const [selected, setSelected] = useState('')
  const [documents, setDocuments] = useState<Credential[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    getCitizens()
      .then((names) => {
        setCitizens(names)
        setSelected(names[0] ?? '')
      })
      .catch((e) => setError(String(e)))
  }, [])

  useEffect(() => {
    if (!selected) return
    getCredentials(selected).then(setDocuments).catch((e) => setError(String(e)))
  }, [selected])

  return (
    <div className="space-y-8">
      <div className="flex flex-wrap gap-3">
        {citizens.map((name) => (
          <button
            key={name}
            onClick={() => setSelected(name)}
            className={`rounded-xl border px-6 py-3 text-lg font-semibold transition ${
              name === selected
                ? 'border-sky-500 bg-sky-500/10 text-sky-200'
                : 'border-slate-700 bg-slate-900 text-slate-300 hover:border-slate-500'
            }`}
          >
            {name}
          </button>
        ))}
      </div>

      {error && <div className="rounded-xl border border-red-500/50 bg-red-500/10 p-4 text-red-300">{error}</div>}

      <div className="space-y-4">
        {documents.map((doc) => {
          const v = verdictOf(doc.status)
          return (
            <div
              key={doc.docHash}
              className={`rounded-2xl border p-6 ${
                doc.status === 'SUPERSEDED' ? 'border-amber-500/30 bg-amber-500/5' : 'border-slate-800 bg-slate-900/50'
              }`}
            >
              <div className="flex flex-wrap items-start justify-between gap-4">
                <div>
                  <div className="flex items-center gap-3">
                    <h3 className="text-2xl font-bold text-slate-100">{docTypeLabel(doc.docType)}</h3>
                    <span className={`rounded-full border px-3 py-1 text-sm font-bold ${v.badge}`}>
                      {v.icon} {v.label}
                    </span>
                  </div>
                  <p className="mt-1 font-mono text-slate-400">{doc.docId}</p>
                </div>
                <button
                  onClick={() => void downloadDocument(`/documents/${doc.docHash}/download`, doc.filename).catch((e) => setError(String(e)))}
                  className="rounded-lg border border-slate-700 px-4 py-2 text-sm font-semibold text-slate-300 transition hover:border-slate-500 hover:text-slate-100"
                >
                  Download ↓
                </button>
              </div>

              <dl className="mt-5 grid gap-x-8 gap-y-3 text-sm sm:grid-cols-3">
                <Row label="Issued" value={doc.issuedAt} />
                <Row label="Anchored hash" value={<Hash value={doc.docHash} />} />
                <Row label="Transaction" value={<Hash value={doc.txHash} />} />
              </dl>
            </div>
          )
        })}

        {selected && documents.length === 0 && !error && (
          <p className="text-slate-500">No documents on record for {selected}.</p>
        )}
      </div>
    </div>
  )
}

const Row = ({ label, value }: { label: string; value: React.ReactNode }) => (
  <div>
    <dt className="text-xs font-semibold uppercase tracking-widest text-slate-500">{label}</dt>
    <dd className="mt-1 text-slate-300">{value}</dd>
  </div>
)
