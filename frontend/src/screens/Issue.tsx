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
  const [dragActive, setDragActive] = useState(false)

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
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="text-xs font-bold uppercase tracking-[0.22em] text-sky-400">Credential issuance</p>
          <h2 className="mt-2 text-3xl font-black tracking-tight text-slate-100">Create an official record</h2>
          <p className="mt-2 max-w-2xl text-sm leading-6 text-slate-400">Attach a citizen, identify the document, and upload the original PDF. The registry will stamp and anchor the exact issued copy.</p>
        </div>
        <div className="flex items-center gap-2 rounded-xl border border-emerald-500/20 bg-emerald-500/5 px-3 py-2 text-xs text-emerald-300"><span className="h-2 w-2 rounded-full bg-emerald-400" /> Department signer ready</div>
      </div>

      <div className="grid gap-6 xl:grid-cols-[minmax(0,1.35fr)_minmax(320px,.65fr)]">
        <form onSubmit={submit} className="overflow-hidden rounded-2xl border border-slate-800 bg-slate-950/50">
          <div className="border-b border-slate-800 bg-slate-900/50 px-6 py-5">
            <div className="flex items-center gap-4">
              <div className="flex h-12 w-12 items-center justify-center rounded-xl border border-sky-500/30 bg-sky-500/10 text-xl font-black text-sky-300">BC</div>
              <div className="min-w-0"><p className="font-bold text-slate-100">{selected?.name ?? 'Loading department…'}</p><p className="text-sm text-slate-500">Authorized for {selected ? docTypeLabel(selected.docTypeName).toLowerCase() : 'credential issuance'}</p></div>
              <span className="ml-auto rounded-full border border-slate-700 px-2.5 py-1 text-[10px] font-bold uppercase tracking-wider text-slate-500">Locked authority</span>
            </div>
          </div>

          <div className="space-y-6 p-6 sm:p-8">
            <div className="grid gap-6 md:grid-cols-2">
              <div>
                <StepLabel number="01" title="Credential holder" />
                <select value={citizenAccountId} onChange={(e) => setCitizenAccountId(e.target.value)} required className="mt-3 w-full rounded-xl border border-slate-700 bg-slate-900 px-4 py-3.5 text-slate-100 outline-none transition focus:border-sky-500">
                  <option value="">Select a provisioned citizen</option>
                  {citizens.map((account) => <option key={account.id} value={account.id}>{account.displayName} · {account.email}</option>)}
                </select>
                <p className="mt-2 text-xs text-slate-600">Identity is resolved from the controlled citizen registry.</p>
              </div>

              <div>
                <StepLabel number="02" title="Document identity" />
                <div className="mt-3 flex gap-2">
                  <Input value={docId} onChange={setDocId} placeholder="BC-2026-000001" required mono />
                  <button type="button" onClick={generateDocId} className="shrink-0 rounded-xl border border-sky-500/30 bg-sky-500/10 px-4 text-sm font-bold text-sky-300 transition hover:bg-sky-500/20">Generate</button>
                </div>
                <p className="mt-2 text-xs text-slate-600">Unique and permanent once anchored.</p>
              </div>
            </div>

            <div>
              <StepLabel number="03" title="Original credential PDF" />
              <label
                onDragEnter={() => setDragActive(true)}
                onDragLeave={() => setDragActive(false)}
                onDragOver={(event) => event.preventDefault()}
                onDrop={(event) => { event.preventDefault(); setDragActive(false); setFile(event.dataTransfer.files?.[0] ?? null) }}
                className={`mt-3 flex cursor-pointer flex-col items-center justify-center rounded-2xl border border-dashed px-6 py-10 text-center transition ${dragActive ? 'border-sky-400 bg-sky-500/10' : file ? 'border-emerald-500/40 bg-emerald-500/5' : 'border-slate-700 bg-slate-900/40 hover:border-sky-500/50 hover:bg-slate-900'}`}
              >
                <input type="file" accept="application/pdf" required onChange={(e) => setFile(e.target.files?.[0] ?? null)} className="sr-only" />
                <span className={`flex h-12 w-12 items-center justify-center rounded-2xl text-2xl ${file ? 'bg-emerald-500/15 text-emerald-300' : 'bg-slate-800 text-slate-400'}`}>{file ? '✓' : '↑'}</span>
                <p className="mt-3 font-semibold text-slate-200">{file ? file.name : 'Drop the original PDF here'}</p>
                <p className="mt-1 text-xs text-slate-500">{file ? `${(file.size / 1024).toFixed(1)} KB ready to stamp` : 'or click to browse · PDF only'}</p>
              </label>
            </div>
          </div>

          <div className="flex flex-wrap items-center justify-between gap-4 border-t border-slate-800 bg-slate-900/30 px-6 py-5 sm:px-8">
            <p className="max-w-md text-xs leading-5 text-slate-500">Submitting stamps the PDF first, anchors its SHA-256 hash, and stores the exact issued bytes.</p>
            <button type="submit" disabled={busy || !file || !citizenAccountId || !docId.trim()} className="rounded-xl bg-sky-500 px-6 py-3.5 font-bold text-slate-950 shadow-lg shadow-sky-950/30 transition hover:bg-sky-400 disabled:cursor-not-allowed disabled:bg-slate-700 disabled:text-slate-400">{busy ? 'Anchoring on chain…' : 'Issue & anchor credential →'}</button>
          </div>
        </form>

        <div className="space-y-4">
        {error && (
          <div className="rounded-2xl border border-red-500/40 bg-red-500/10 p-6">
            <p className="text-xs font-bold uppercase tracking-widest text-red-300">Operation rejected</p>
            <p className="mt-3 break-words text-red-200">{error}</p>
          </div>
        )}

        {result && (
          <div className="rounded-2xl border border-emerald-500/40 bg-emerald-500/10 p-6">
            <div className="flex h-12 w-12 items-center justify-center rounded-full bg-emerald-400 text-xl font-black text-emerald-950">✓</div>
            <p className="mt-4 text-xs font-bold uppercase tracking-widest text-emerald-300">Registry receipt</p>
            <h3 className="mt-1 text-2xl font-black text-slate-100">Issued and anchored</h3>
            <dl className="mt-5 space-y-4">
              <CopyRow label="Document id" value={result.docId} display={result.docId} copied={copied} onCopy={copy} />
              <Row label="Holder" value={result.citizen} />
              <Row label="Issuer" value={result.issuer} />
              <CopyRow label="Anchored hash" value={result.docHash} display={<Hash value={result.docHash} />} copied={copied} onCopy={copy} />
              <CopyRow label="Transaction" value={result.txHash} display={<Hash value={result.txHash} />} copied={copied} onCopy={copy} />
            </dl>
            <div className="mt-6 rounded-xl border border-emerald-500/20 bg-slate-950/40 p-4">
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
        {!error && !result && <div className="rounded-2xl border border-slate-800 bg-slate-900/30 p-6"><p className="text-xs font-bold uppercase tracking-[0.2em] text-slate-500">Issuance protocol</p><ol className="mt-5 space-y-5">{['Registry marker and QR are added', 'Stamped bytes are hashed with SHA-256', 'Hash is anchored by the department signer', 'Exact stamped PDF is stored for download'].map((text, index) => <li key={text} className="flex gap-3"><span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg border border-slate-700 bg-slate-950 font-mono text-xs text-sky-300">{index + 1}</span><span className="pt-1 text-sm leading-5 text-slate-400">{text}</span></li>)}</ol></div>}
        </div>
      </div>
    </div>
  )
}

const StepLabel = ({ number, title }: { number: string; title: string }) => <div className="flex items-center gap-2"><span className="font-mono text-xs font-bold text-sky-400">{number}</span><span className="text-xs font-bold uppercase tracking-widest text-slate-400">{title}</span></div>

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
