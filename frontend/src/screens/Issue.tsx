import { useEffect, useState } from 'react'
import {
  downloadDocument,
  docTypeLabel,
  getDepartments,
  getCitizenAccounts,
  getMe,
  issueDocument,
  type Department,
  type CitizenAccountOption,
  type IssueResult,
} from '../api'
import { Hash } from '../components/Hash'

export function Issue({ onSuccess }: { onSuccess?: (result: IssueResult) => void } = {}) {
  const [departments, setDepartments] = useState<Department[]>([])
  const [dept, setDept] = useState('')
  const [citizens, setCitizens] = useState<CitizenAccountOption[]>([])
  const [citizenAccountId, setCitizenAccountId] = useState('')
  const [docId, setDocId] = useState('')
  const [file, setFile] = useState<File | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [result, setResult] = useState<IssueResult | null>(null)
  const [copied, setCopied] = useState('')

  useEffect(() => {
    Promise.all([getDepartments(), getMe(), getCitizenAccounts()])
      .then(([allDepartments, me, citizenAccounts]) => {
        const d = allDepartments.filter((department) => department.slug === me.department?.id)
        setDepartments(d)
        setDept(d[0]?.slug ?? '')
        setCitizens(citizenAccounts)
        setCitizenAccountId(citizenAccounts[0]?.id ?? '')
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
      const issued = await issueDocument({
        file,
        dept,
        docType: selected.docTypeName,
        docId: docId.trim(),
        citizen: citizens.find((candidate) => candidate.id === citizenAccountId)?.displayName ?? '',
        citizenAccountId,
      })
      setResult(issued)
      onSuccess?.(issued)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  async function copy(label: string, value: string) {
    try {
      await navigator.clipboard.writeText(value)
      setCopied(label)
      window.setTimeout(() => setCopied(''), 1800)
    } catch {
      setError(`Could not copy ${label.toLowerCase()}`)
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
          <select value={citizenAccountId} onChange={(e) => setCitizenAccountId(e.target.value)} required className="w-full rounded-lg border border-slate-700 bg-slate-950 px-4 py-3 text-slate-100">
            <option value="">Select a provisioned citizen</option>
            {citizens.map((account) => <option key={account.id} value={account.id}>{account.displayName} · {account.email}</option>)}
          </select>
          <p className="mt-2 text-sm text-slate-500">Citizen accounts, including their required email, are provisioned by the controlled backend CLI.</p>
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
          disabled={busy || !file || !citizenAccountId}
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
              <CopyRow label="Document id" value={result.docId} display={result.docId} copied={copied} onCopy={copy} />
              <Row label="Holder" value={result.citizen} />
              <Row label="Issuer" value={result.issuer} />
              <CopyRow label="Anchored hash" value={result.docHash} display={<Hash value={result.docHash} />} copied={copied} onCopy={copy} />
              <CopyRow label="Transaction" value={result.txHash} display={<Hash value={result.txHash} />} copied={copied} onCopy={copy} />
            </dl>
            <div className="mt-6 rounded-xl border border-emerald-500/30 bg-slate-950/30 p-4">
              <p className="text-sm text-emerald-100">The downloaded PDF is the exact stamped byte sequence anchored by the hash above.</p>
              <button
                type="button"
                onClick={() => void downloadDocument(result.downloadUrl, `${result.docId}-stamped.pdf`).catch((e) => setError(String(e)))}
                className="mt-4 w-full rounded-lg bg-emerald-500/20 px-5 py-3 font-semibold text-emerald-100 transition hover:bg-emerald-500/30"
              >
                Download stamped PDF
              </button>
            </div>
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

function CopyRow({ label, value, display, copied, onCopy }: { label: string; value: string; display: React.ReactNode; copied: string; onCopy: (label: string, value: string) => Promise<void> }) {
  return <div className="flex items-center justify-between gap-4"><dt className="text-sm text-emerald-200/60">{label}</dt><dd className="flex items-center gap-3 text-right text-slate-100"><span className="font-mono">{display}</span><button type="button" onClick={() => void onCopy(label, value)} className="rounded border border-emerald-500/30 px-2 py-1 text-xs font-semibold text-emerald-200 hover:bg-emerald-500/10">{copied === label ? 'Copied' : 'Copy'}</button></dd></div>
}
