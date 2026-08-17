import { useEffect, useState } from 'react'
import {
  API,
  docTypeLabel,
  getDepartments,
  issueDocument,
  type Department,
  type IssueResult,
} from '../api'
import { Hash } from '../components/Hash'

export function Issue() {
  const [departments, setDepartments] = useState<Department[]>([])
  const [dept, setDept] = useState('')
  const [citizen, setCitizen] = useState('')
  const [docId, setDocId] = useState('')
  const [file, setFile] = useState<File | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [result, setResult] = useState<IssueResult | null>(null)

  useEffect(() => {
    getDepartments()
      .then((d) => {
        setDepartments(d)
        setDept(d[0]?.slug ?? '')
      })
      .catch((e) => setError(String(e)))
  }, [])

  // A department may only issue its own kind of document — the contract enforces
  // it, so the form simply follows the picker rather than offering a free choice.
  const selected = departments.find((d) => d.slug === dept)



// adding generate random docid function



function generateDocId() {
  if (!selected) return

  const prefixes: Record<string, string> = {
    birth_certificate: 'BC',
    driving_licence: 'DL',
    degree_certificate: 'DEG',
  }

  const prefix = prefixes[selected.docTypeName] ?? 'DOC'
  const random = crypto.randomUUID().replaceAll('-', '').slice(0, 8).toUpperCase()

  setDocId(`${prefix}-${new Date().getFullYear()}-${random}`)
}

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    if (!file || !selected) return
    setBusy(true)
    setError('')
    setResult(null)
    try {
      setResult(
        await issueDocument({
          file,
          dept,
          docType: selected.docTypeName,
          docId: docId.trim(),
          citizen: citizen.trim(),
        }),
      )
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="grid gap-8 lg:grid-cols-2">
      <form onSubmit={submit} className="space-y-6 rounded-2xl border border-slate-800 bg-slate-900/50 p-8">
        <div>
          <Label>Issuing department</Label>
          <div className="grid gap-3">
            {departments.map((d) => (
              <button
                type="button"
                key={d.slug}
                // if the dept is changed the placeholder docid is cleared so that the user can generate a new one for the new dept
                onClick={() => {
                  setDept(d.slug)
                  setDocId('')
                }}
                className={`rounded-xl border p-4 text-left transition ${
                  d.slug === dept
                    ? 'border-sky-500 bg-sky-500/10'
                    : 'border-slate-700 bg-slate-900 hover:border-slate-500'
                }`}
              >
                <div className="font-semibold text-slate-100">{d.name}</div>
                <div className="text-sm text-slate-400">
                  may issue {docTypeLabel(d.docTypeName).toLowerCase()}
                </div>
                <div className="mt-1 font-mono text-xs text-slate-600">{d.address}</div>
              </button>
            ))}
          </div>
        </div>

        <div>
          <Label>Citizen</Label>
          <Input value={citizen} onChange={setCitizen} placeholder="Asha Menon" required />
        </div>

{/* changed the button and added a genrate docid button */}
        <div>
          <Label>Document id</Label>

              <div className="flex gap-3">
                <Input
                  value={docId}
                  onChange={setDocId}
                  placeholder="BC-2026-000001"
                  required
                  mono
                />

                <button
                  type="button"
                  onClick={generateDocId}
                  className="shrink-0 rounded-lg border border-slate-700 bg-slate-800 px-4 py-3 font-semibold text-slate-200 transition hover:bg-slate-700"
                >
                  Generate
                </button>
              </div>
        </div>

        <div>
          <Label>Document (PDF)</Label>
          <input
            type="file"
            accept="application/pdf"
            required
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
            className="w-full rounded-lg border border-slate-700 bg-slate-950 p-3 text-slate-300 file:mr-4 file:rounded file:border-0 file:bg-slate-800 file:px-4 file:py-2 file:text-slate-200"
          />
          <p className="mt-2 text-sm text-slate-500">
            Upload the unstamped original. Issuing adds the QR and registry marker, then anchors the
            hash of the stamped file.
          </p>
        </div>

        <button
          type="submit"
          disabled={busy || !file}
          className="w-full rounded-xl bg-sky-600 px-6 py-4 text-lg font-semibold text-white transition hover:bg-sky-500 disabled:cursor-not-allowed disabled:bg-slate-700 disabled:text-slate-400"
        >
          {busy ? 'Anchoring on chain…' : 'Issue document'}
        </button>
      </form>

      <div className="space-y-6">
        {error && (
          <div className="rounded-2xl border-2 border-red-500/60 bg-red-500/10 p-6">
            <h3 className="text-2xl font-bold text-red-400">Rejected</h3>
            <p className="mt-3 break-words text-red-200">{error}</p>
            <p className="mt-4 text-sm text-red-200/70">
              The contract refuses any document a department does not hold the role for.
            </p>
          </div>
        )}

        {result && (
          <div className="rounded-2xl border-2 border-emerald-500/60 bg-emerald-500/10 p-6">
            <h3 className="text-2xl font-bold text-emerald-400">Issued and anchored</h3>
            <dl className="mt-5 space-y-4">
              <Row label="Document id" value={<span className="font-mono">{result.docId}</span>} />
              <Row label="Holder" value={result.citizen} />
              <Row label="Issuer" value={result.issuer} />
              <Row label="Anchored hash" value={<Hash value={result.docHash} />} />
              <Row label="Transaction" value={<Hash value={result.txHash} />} />
            </dl>
            <a
              href={`${API}${result.downloadUrl}`}
              className="mt-6 inline-block rounded-lg bg-emerald-500/20 px-5 py-3 font-semibold text-emerald-100 transition hover:bg-emerald-500/30"
            >
              Download the stamped PDF ↓
            </a>
            {!result.stamped && (
              <p className="mt-4 text-sm text-amber-300">
                This PDF's structure could not be extended, so the marker was appended as a comment
                instead of a registry page. It still verifies.
              </p>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

const Label = ({ children }: { children: React.ReactNode }) => (
  <div className="mb-2 text-xs font-semibold uppercase tracking-widest text-slate-500">{children}</div>
)

function Input({
  value,
  onChange,
  placeholder,
  required,
  mono,
}: {
  value: string
  onChange: (v: string) => void
  placeholder?: string
  required?: boolean
  mono?: boolean
}) {
  return (
    <input
      value={value}
      required={required}
      placeholder={placeholder}
      onChange={(e) => onChange(e.target.value)}
      className={`w-full rounded-lg border border-slate-700 bg-slate-950 px-4 py-3 text-slate-100 outline-none transition placeholder:text-slate-600 focus:border-sky-500 ${
        mono ? 'font-mono' : ''
      }`}
    />
  )
}

const Row = ({ label, value }: { label: string; value: React.ReactNode }) => (
  <div className="flex items-baseline justify-between gap-4">
    <dt className="text-sm text-emerald-200/60">{label}</dt>
    <dd className="text-right text-slate-100">{value}</dd>
  </div>
)
