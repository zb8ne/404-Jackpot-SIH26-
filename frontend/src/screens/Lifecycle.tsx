import { useEffect, useState } from 'react'
import {
  downloadDocument,
  getCitizenAccounts,
  getMe,
  getDepartmentCredentials,
  revokeDocument,
  supersedeDocument,
  type RevokeResult,
  type SupersedeResult,
  type Credential,
} from '../api'
import { Hash } from '../components/Hash'

export function Revoke() {
  const [department, setDepartment] = useState('')
  const [docHash, setDocHash] = useState('')
  const [result, setResult] = useState<RevokeResult | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [credentials, setCredentials] = useState<Credential[]>([])
  const [query, setQuery] = useState('')

  useEffect(() => {
    Promise.all([getMe(), getDepartmentCredentials()])
      .then(([me, documents]) => { setDepartment(me.department?.id ?? ''); setCredentials(documents.filter((doc) => doc.status === 'VALID')) })
      .catch((e) => setError(errorMessage(e)))
  }, [])

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError('')
    setResult(null)
    try {
      setResult(await revokeDocument(docHash.trim(), department, credentials.find((item) => item.docHash === docHash)?.docId))
    } catch (e) {
      setError(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  const filtered = credentials.filter((item) => `${item.docId} ${item.citizen}`.toLowerCase().includes(query.toLowerCase()))
  const selected = credentials.find((item) => item.docHash === docHash)

  return <LifecycleWorkspace eyebrow="Lifecycle control" title="Revoke a credential" description="End the validity of an existing department credential. Its issuance history remains permanently verifiable." tone="rose">
    <div className="grid gap-6 lg:grid-cols-[minmax(0,1.25fr)_minmax(300px,.75fr)]">
      <form onSubmit={submit} className="overflow-hidden rounded-2xl border border-slate-800 bg-slate-950/50">
        <div className="border-b border-slate-800 bg-slate-900/40 p-6"><StepTitle number="01" title="Find an existing credential" /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search by document ID or citizen name…" className="mt-4 w-full rounded-xl border border-slate-700 bg-slate-950 px-4 py-3 text-slate-100 outline-none placeholder:text-slate-600 focus:border-rose-500" /></div>
        <div className="max-h-80 space-y-2 overflow-y-auto p-4">
          {filtered.map((credential) => <button key={credential.docHash} type="button" onClick={() => setDocHash(credential.docHash)} className={`w-full rounded-xl border p-4 text-left transition ${docHash === credential.docHash ? 'border-rose-500/60 bg-rose-500/10' : 'border-slate-800 bg-slate-900/40 hover:border-slate-600'}`}><div className="flex items-start justify-between gap-3"><div><p className="font-mono text-sm font-bold text-slate-100">{credential.docId}</p><p className="mt-1 text-sm text-slate-400">{credential.citizen}</p></div><span className="rounded-full bg-emerald-500/10 px-2 py-1 text-[10px] font-bold text-emerald-300">VALID</span></div></button>)}
          {filtered.length === 0 && <div className="py-8 text-center text-sm text-slate-500">No matching valid credentials.</div>}
        </div>
        <div className="border-t border-slate-800 p-6"><StepTitle number="02" title="Confirm revocation" />{selected ? <div className="mt-4 rounded-xl border border-rose-500/20 bg-rose-500/5 p-4"><p className="text-sm text-slate-300">You are revoking <span className="font-mono font-bold text-rose-300">{selected.docId}</span>.</p><p className="mt-2 text-xs leading-5 text-slate-500">This changes its on-chain status to REVOKED. The credential will remain discoverable and its history will not be deleted.</p></div> : <p className="mt-4 text-sm text-slate-500">Select a credential above to continue.</p>}<button type="submit" disabled={busy || !department || !docHash} className="mt-5 w-full rounded-xl bg-rose-500 px-6 py-4 font-bold text-white transition hover:bg-rose-400 disabled:cursor-not-allowed disabled:bg-slate-700 disabled:text-slate-400">{busy ? 'Submitting revocation…' : 'Revoke selected credential'}</button></div>
      </form>
      <div><AuthorityCard department={department} action="Revocation authority" tone="rose" />{error && <ErrorPanel message={error} />}{result && <ResultPanel title="Credential revoked"><ResultRow label="Document hash"><Hash value={result.docHash} /></ResultRow><ResultRow label="Transaction"><Hash value={result.txHash} /></ResultRow><ResultRow label="Status">{result.status}</ResultRow></ResultPanel>}</div>
    </div>
  </LifecycleWorkspace>
}

export function Supersede() {
  const [department, setDepartment] = useState('')
  const [oldHash, setOldHash] = useState('')
  const [docId, setDocId] = useState('')
  const [citizen, setCitizen] = useState('')
  const [citizenAccountId, setCitizenAccountId] = useState('')
  const [citizenOptions, setCitizenOptions] = useState<Array<{ id: string; displayName: string; email: string }>>([])
  const [file, setFile] = useState<File | null>(null)
  const [result, setResult] = useState<SupersedeResult | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [credentials, setCredentials] = useState<Credential[]>([])

  useEffect(() => {
    Promise.all([getMe(), getCitizenAccounts(), getDepartmentCredentials()])
      .then(([me, accounts, documents]) => {
        setDepartment(me.department?.id ?? '')
        setCitizenOptions(accounts)
        setCitizenAccountId(accounts[0]?.id ?? '')
        setCitizen(accounts[0]?.displayName ?? '')
        setCredentials(documents.filter((doc) => doc.status === 'VALID'))
      })
      .catch((e) => setError(errorMessage(e)))
  }, [])

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    if (!file) return
    setBusy(true)
    setError('')
    setResult(null)
    try {
      setResult(await supersedeDocument({
        file,
        dept: department,
        oldHash: oldHash.trim(),
        docId: docId.trim(),
        citizen: citizen.trim(),
        citizenAccountId,
      }))
    } catch (e) {
      setError(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  function generateReplacementId() {
    const current = credentials.find((item) => item.docHash === oldHash)
    if (!current) return
    const revision = crypto.randomUUID().replaceAll('-', '').slice(0, 4).toUpperCase()
    setDocId(`${current.docId}-R${revision}`)
  }

  return (
    <LifecycleWorkspace eyebrow="Credential correction" title="Supersede a credential" description="Anchor a corrected PDF while preserving the original credential as part of its permanent history." tone="amber">
      <div className="grid gap-6 lg:grid-cols-[minmax(0,1.25fr)_minmax(300px,.75fr)]">
      <form onSubmit={submit} className="space-y-6 rounded-2xl border border-slate-800 bg-slate-950/50 p-6 sm:p-8">
        <div><StepTitle number="01" title="Original credential" /><div className="mt-3"><CredentialSelect credentials={credentials} value={oldHash} onChange={(value) => { setOldHash(value); setDocId('') }} /></div></div>
        <div><StepTitle number="02" title="Replacement identity" /><div className="mt-3 flex gap-2"><div className="min-w-0 flex-1"><TextField label="New document ID" value={docId} onChange={setDocId} placeholder="BC-2026-000001-R1" mono /></div><button type="button" onClick={generateReplacementId} disabled={!oldHash} className="mt-6 shrink-0 rounded-xl border border-amber-500/30 bg-amber-500/10 px-4 text-sm font-bold text-amber-300 hover:bg-amber-500/20 disabled:opacity-40">Generate</button></div></div>
        <div><StepTitle number="03" title="Replacement holder and PDF" /></div>
        <label className="block">
          <span className="mb-2 block text-xs font-semibold uppercase tracking-widest text-slate-500">Citizen account</span>
          <select value={citizenAccountId} onChange={(e) => { const id = e.target.value; setCitizenAccountId(id); setCitizen(citizenOptions.find((item) => item.id === id)?.displayName ?? '') }} className="w-full rounded-lg border border-slate-700 bg-slate-950 px-4 py-3 text-slate-100">
            <option value="">Inherit the old credential linkage</option>
            {citizenOptions.map((item) => <option key={item.id} value={item.id}>{item.displayName} · {item.email}</option>)}
          </select>
        </label>
        <label className="block">
          <span className="mb-2 block text-xs font-semibold uppercase tracking-widest text-slate-500">Replacement PDF</span>
          <input type="file" accept="application/pdf" required onChange={(e) => setFile(e.target.files?.[0] ?? null)} className="w-full rounded-lg border border-slate-700 bg-slate-950 p-3 text-slate-300 file:mr-4 file:rounded file:border-0 file:bg-slate-800 file:px-4 file:py-2 file:text-slate-200" />
        </label>
        <button type="submit" disabled={busy || !department || !file || !oldHash || !docId.trim()} className="w-full rounded-xl bg-amber-400 px-6 py-4 font-bold text-amber-950 hover:bg-amber-300 disabled:bg-slate-700 disabled:text-slate-400">{busy ? 'Anchoring replacement…' : 'Supersede & anchor replacement'}</button>
      </form>
      <div><AuthorityCard department={department} action="Supersede authority" tone="amber" />
      {error && <ErrorPanel message={error} />}
      {result && (
        <ResultPanel title="Replacement anchored">
          <ResultRow label="Document ID">{result.docId}</ResultRow>
          <ResultRow label="Old hash"><Hash value={result.oldHash} /></ResultRow>
          <ResultRow label="New hash"><Hash value={result.docHash} /></ResultRow>
          <ResultRow label="Transaction"><Hash value={result.txHash} /></ResultRow>
          <button type="button" onClick={() => void downloadDocument(result.downloadUrl, `${result.docId}.pdf`).catch((e) => setError(errorMessage(e)))} className="mt-5 rounded-lg bg-emerald-500/20 px-5 py-3 font-semibold text-emerald-100 hover:bg-emerald-500/30">Download replacement PDF ↓</button>
        </ResultPanel>
      )}
      </div></div>
    </LifecycleWorkspace>
  )
}

function LifecycleWorkspace({ eyebrow, title, description, children, tone }: { eyebrow: string; title: string; description: string; children: React.ReactNode; tone: 'rose' | 'amber' }) {
  return <div className="space-y-6"><div><p className={`text-xs font-bold uppercase tracking-[0.22em] ${tone === 'rose' ? 'text-rose-400' : 'text-amber-400'}`}>{eyebrow}</p><h1 className="mt-2 text-3xl font-black tracking-tight text-slate-100">{title}</h1><p className="mt-2 max-w-2xl text-sm leading-6 text-slate-400">{description}</p></div>{children}</div>
}

function StepTitle({ number, title }: { number: string; title: string }) {
  return <div className="flex items-center gap-2"><span className="font-mono text-xs font-bold text-sky-400">{number}</span><span className="text-xs font-bold uppercase tracking-widest text-slate-400">{title}</span></div>
}

function AuthorityCard({ department, action, tone }: { department: string; action: string; tone: 'rose' | 'amber' }) {
  const accent = tone === 'rose' ? 'border-rose-500/20 bg-rose-500/5 text-rose-300' : 'border-amber-500/20 bg-amber-500/5 text-amber-300'
  return <div className={`rounded-2xl border p-6 ${accent}`}><p className="text-xs font-bold uppercase tracking-widest opacity-70">Authenticated authority</p><p className="mt-3 text-xl font-black capitalize">{department || 'Loading…'} Department</p><p className="mt-1 text-sm opacity-70">{action} · Admin only</p><div className="mt-5 flex items-center gap-2 text-xs"><span className="h-2 w-2 rounded-full bg-current" /> Signer available</div></div>
}

function CredentialSelect({ credentials, value, onChange }: { credentials: Credential[]; value: string; onChange: (value: string) => void }) {
  return <label className="block"><span className="mb-2 block text-xs font-semibold uppercase tracking-widest text-slate-500">Credential</span><select value={value} onChange={(event) => onChange(event.target.value)} required className="w-full rounded-lg border border-slate-700 bg-slate-950 px-4 py-3 text-slate-100"><option value="">Select a valid credential</option>{credentials.map((credential) => <option key={credential.docHash} value={credential.docHash}>{credential.docId} · {credential.citizen} · {credential.filename}</option>)}</select>{credentials.length === 0 && <p className="mt-2 text-sm text-amber-300">No valid department credentials are available.</p>}</label>
}

function TextField({ label, value, onChange, placeholder, mono }: { label: string; value: string; onChange: (value: string) => void; placeholder: string; mono?: boolean }) {
  return <label className="block"><span className="mb-2 block text-xs font-semibold uppercase tracking-widest text-slate-500">{label}</span><input value={value} onChange={(e) => onChange(e.target.value)} placeholder={placeholder} required className={`w-full rounded-lg border border-slate-700 bg-slate-950 px-4 py-3 text-slate-100 outline-none focus:border-sky-500 ${mono ? 'font-mono' : ''}`} /></label>
}

function ErrorPanel({ message }: { message: string }) {
  return <div className="mt-6 rounded-xl border border-red-500/50 bg-red-500/10 p-4 text-red-300">{message}</div>
}

function ResultPanel({ title, children }: { title: string; children: React.ReactNode }) {
  return <div className="mt-6 rounded-xl border border-emerald-500/50 bg-emerald-500/10 p-5"><h2 className="text-xl font-bold text-emerald-300">{title}</h2><dl className="mt-4 space-y-3">{children}</dl></div>
}

function ResultRow({ label, children }: { label: string; children: React.ReactNode }) {
  return <div><dt className="text-xs uppercase tracking-wider text-emerald-200/60">{label}</dt><dd className="mt-1 break-all text-slate-100">{children}</dd></div>
}

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error)
}
