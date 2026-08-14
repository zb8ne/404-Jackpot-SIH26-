import { useCallback, useRef, useState } from 'react'
import { docTypeLabel, verifyById, verifyFile, type VerifyResult } from '../api'
import { Hash, HashDiff } from '../components/Hash'
import { verdictOf } from '../verdicts'

export function Verify() {
  const [result, setResult] = useState<VerifyResult | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [dragging, setDragging] = useState(false)
  const [name, setName] = useState('')
  const input = useRef<HTMLInputElement>(null)

  const check = useCallback(async (file: File) => {
    setBusy(true)
    setError('')
    setResult(null)
    setName(file.name)
    try {
      setResult(await verifyFile(file))
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }, [])

  const checkId = useCallback(async (docId: string) => {
    setBusy(true)
    setError('')
    setResult(null)
    setName(docId)
    try {
      setResult(await verifyById(docId))
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }, [])

  return (
    <div className="space-y-8">
      <div
        onDragOver={(e) => {
          e.preventDefault()
          setDragging(true)
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={(e) => {
          e.preventDefault()
          setDragging(false)
          const file = e.dataTransfer.files[0]
          if (file) void check(file)
        }}
        onClick={() => input.current?.click()}
        className={`cursor-pointer rounded-2xl border-2 border-dashed p-12 text-center transition ${
          dragging
            ? 'border-sky-400 bg-sky-500/10'
            : 'border-slate-700 bg-slate-900/50 hover:border-slate-500 hover:bg-slate-900'
        }`}
      >
        <input
          ref={input}
          type="file"
          accept="application/pdf"
          className="hidden"
          onChange={(e) => {
            const file = e.target.files?.[0]
            if (file) void check(file)
          }}
        />
        <p className="text-2xl font-semibold text-slate-200">Drop a document here</p>
        <p className="mt-2 text-slate-400">or click to choose a PDF</p>
        {name && <p className="mt-4 font-mono text-sm text-slate-500">{name}</p>}
      </div>

      {busy && <p className="text-center text-xl text-slate-400">Checking the registry…</p>}

      {error && (
        <div className="rounded-xl border border-red-500/50 bg-red-500/10 p-4 text-red-300">{error}</div>
      )}

      {result && <Verdict result={result} onVerifyId={checkId} />}
    </div>
  )
}

function Verdict({
  result,
  onVerifyId,
}: {
  result: VerifyResult
  onVerifyId: (docId: string) => void
}) {
  const v = verdictOf(result.status)
  const tampered = result.status === 'TAMPERED'

  return (
    <div className={`rounded-2xl border-2 p-8 ${v.panel}`}>
      <div className="flex items-baseline gap-5">
        <span className={`text-6xl leading-none ${v.accent}`}>{v.icon}</span>
        <div>
          <h2 className={`text-6xl font-black tracking-tight ${v.accent}`}>{v.label.toUpperCase()}</h2>
          <p className="mt-2 text-xl text-slate-300">{v.headline}</p>
        </div>
      </div>

      <p className="mt-6 text-lg text-slate-400">{result.message}</p>

      {result.supersededBy && (
        <div className="mt-6 rounded-xl border border-amber-500/40 bg-amber-500/10 p-5">
          <p className="text-lg text-amber-200">
            Replaced by <span className="font-mono font-bold">{result.supersededBy.docId}</span>
          </p>
          <p className="mt-1 text-sm text-amber-200/70">
            Current hash <Hash value={result.supersededBy.hash} />
          </p>
          <button
            onClick={() => onVerifyId(result.supersededBy!.docId)}
            className="mt-4 rounded-lg bg-amber-500/20 px-4 py-2 font-semibold text-amber-100 transition hover:bg-amber-500/30"
          >
            Verify the current version →
          </button>
        </div>
      )}

      {result.resolvedDocId && (
        <p className="mt-4 text-amber-200/80">
          That id now resolves to <span className="font-mono font-bold">{result.resolvedDocId}</span>.
        </p>
      )}

      <dl className="mt-8 grid gap-x-8 gap-y-5 sm:grid-cols-2 lg:grid-cols-4">
        <Field label="Document id" value={result.docId || '—'} mono />
        <Field label="Type" value={docTypeLabel(result.docType)} />
        <Field label="Holder" value={result.citizen ?? '—'} />
        <Field label="Issued" value={formatTime(result.timestamp)} />
        <Field label="Issuing department" value={result.issuer ?? '—'} mono span />
      </dl>

      <div className="mt-8">
        {tampered ? (
          <HashDiff expected={result.expectedHash} computed={result.computedHash ?? ''} />
        ) : (
          <dl className="grid gap-x-8 gap-y-5 sm:grid-cols-2">
            <Field label="Expected hash" value={<Hash value={result.expectedHash} />} />
            {result.computedHash && (
              <Field label="Computed hash" value={<Hash value={result.computedHash} />} />
            )}
          </dl>
        )}
      </div>
    </div>
  )
}

function Field({
  label,
  value,
  mono,
  span,
}: {
  label: string
  value: React.ReactNode
  mono?: boolean
  span?: boolean
}) {
  return (
    <div className={span ? 'sm:col-span-2' : undefined}>
      <dt className="text-xs font-semibold uppercase tracking-widest text-slate-500">{label}</dt>
      <dd className={`mt-1 text-lg text-slate-200 ${mono ? 'font-mono break-all' : ''}`}>{value}</dd>
    </div>
  )
}

function formatTime(seconds?: number) {
  if (!seconds) return '—'
  return new Date(seconds * 1000).toLocaleString()
}
