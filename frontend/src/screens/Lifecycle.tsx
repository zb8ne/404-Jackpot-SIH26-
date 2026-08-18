import { useEffect, useState } from 'react'
import {
  downloadDocument,
  getMe,
  revokeDocument,
  supersedeDocument,
  type RevokeResult,
  type SupersedeResult,
} from '../api'
import { Hash } from '../components/Hash'

export function Revoke() {
  const [department, setDepartment] = useState('')
  const [docHash, setDocHash] = useState('')
  const [result, setResult] = useState<RevokeResult | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    getMe()
      .then((me) => setDepartment(me.department?.id ?? ''))
      .catch((e) => setError(errorMessage(e)))
  }, [])

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError('')
    setResult(null)
    try {
      setResult(await revokeDocument(docHash.trim(), department))
    } catch (e) {
      setError(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <ValidationPanel title="Revoke credential" description="Admin-only temporary Phase 2 validation form.">
      <form onSubmit={submit} className="space-y-5">
        <Department value={department} />
        <TextField label="Document hash" value={docHash} onChange={setDocHash} placeholder="0x…" mono />
        <Submit disabled={busy || !department}>{busy ? 'Revoking…' : 'Revoke credential'}</Submit>
      </form>
      {error && <ErrorPanel message={error} />}
      {result && (
        <ResultPanel title="Credential revoked">
          <ResultRow label="Document hash"><Hash value={result.docHash} /></ResultRow>
          <ResultRow label="Transaction"><Hash value={result.txHash} /></ResultRow>
          <ResultRow label="Status">{result.status}</ResultRow>
        </ResultPanel>
      )}
    </ValidationPanel>
  )
}

export function Supersede() {
  const [department, setDepartment] = useState('')
  const [oldHash, setOldHash] = useState('')
  const [docId, setDocId] = useState('')
  const [citizen, setCitizen] = useState('')
  const [file, setFile] = useState<File | null>(null)
  const [result, setResult] = useState<SupersedeResult | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    getMe()
      .then((me) => setDepartment(me.department?.id ?? ''))
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
      }))
    } catch (e) {
      setError(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <ValidationPanel title="Supersede credential" description="Admin-only temporary Phase 2 validation form.">
      <form onSubmit={submit} className="space-y-5">
        <Department value={department} />
        <TextField label="Old document hash" value={oldHash} onChange={setOldHash} placeholder="0x…" mono />
        <TextField label="Replacement document ID" value={docId} onChange={setDocId} placeholder="BC-2026-000001-R1" mono />
        <TextField label="Citizen" value={citizen} onChange={setCitizen} placeholder="Asha Menon" />
        <label className="block">
          <span className="mb-2 block text-xs font-semibold uppercase tracking-widest text-slate-500">Replacement PDF</span>
          <input type="file" accept="application/pdf" required onChange={(e) => setFile(e.target.files?.[0] ?? null)} className="w-full rounded-lg border border-slate-700 bg-slate-950 p-3 text-slate-300 file:mr-4 file:rounded file:border-0 file:bg-slate-800 file:px-4 file:py-2 file:text-slate-200" />
        </label>
        <Submit disabled={busy || !department || !file}>{busy ? 'Superseding…' : 'Supersede credential'}</Submit>
      </form>
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
    </ValidationPanel>
  )
}

function ValidationPanel({ title, description, children }: { title: string; description: string; children: React.ReactNode }) {
  return <div className="mx-auto max-w-3xl space-y-6"><div><h1 className="text-3xl font-black text-slate-100">{title}</h1><p className="mt-1 text-amber-300">{description}</p></div><div className="rounded-2xl border border-slate-800 bg-slate-900/50 p-8">{children}</div></div>
}

function Department({ value }: { value: string }) {
  return <div><div className="text-xs font-semibold uppercase tracking-widest text-slate-500">Authenticated department</div><div className="mt-2 rounded-lg border border-slate-800 bg-slate-950 px-4 py-3 text-slate-300">{value || 'Loading…'}</div></div>
}

function TextField({ label, value, onChange, placeholder, mono }: { label: string; value: string; onChange: (value: string) => void; placeholder: string; mono?: boolean }) {
  return <label className="block"><span className="mb-2 block text-xs font-semibold uppercase tracking-widest text-slate-500">{label}</span><input value={value} onChange={(e) => onChange(e.target.value)} placeholder={placeholder} required className={`w-full rounded-lg border border-slate-700 bg-slate-950 px-4 py-3 text-slate-100 outline-none focus:border-sky-500 ${mono ? 'font-mono' : ''}`} /></label>
}

function Submit({ disabled, children }: { disabled: boolean; children: React.ReactNode }) {
  return <button type="submit" disabled={disabled} className="w-full rounded-xl bg-sky-600 px-6 py-4 font-semibold text-white hover:bg-sky-500 disabled:bg-slate-700 disabled:text-slate-400">{children}</button>
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
