// Hashes are the evidence in this demo, so they get their own presentation:
// short enough to read from a distance, complete on hover, and character-diffed
// when two of them disagree.

export const truncate = (hash?: string, keep = 6) => {
  if (!hash) return '—'
  if (hash.length <= keep * 2 + 4) return hash
  return `${hash.slice(0, keep + 2)}…${hash.slice(-keep)}`
}

export function Hash({ value, className = '' }: { value?: string; className?: string }) {
  if (!value) return <span className="text-slate-500">—</span>
  return (
    <span title={value} className={`font-mono tabular-nums cursor-help ${className}`}>
      {truncate(value)}
    </span>
  )
}

/** Two hashes stacked, with every character that differs called out. */
export function HashDiff({ expected, computed }: { expected: string; computed: string }) {
  return (
    <div className="space-y-4">
      <HashLine label="Expected — the hash on the registry" value={expected} against={computed} tone="ok" />
      <HashLine label="Computed — the hash of this file" value={computed} against={expected} tone="bad" />
    </div>
  )
}

function HashLine({
  label,
  value,
  against,
  tone,
}: {
  label: string
  value: string
  against: string
  tone: 'ok' | 'bad'
}) {
  const chars = [...value]
  const other = [...against]

  return (
    <div>
      <div className="mb-1 text-xs font-semibold uppercase tracking-widest text-slate-400">{label}</div>
      <div className="rounded-lg bg-slate-950/60 p-3 font-mono text-lg leading-relaxed break-all sm:text-xl">
        {chars.map((c, i) => {
          const differs = other[i] !== c
          return (
            <span
              key={i}
              className={
                differs
                  ? tone === 'bad'
                    ? 'rounded bg-red-500/30 font-bold text-red-200'
                    : 'rounded bg-emerald-500/20 font-bold text-emerald-200'
                  : 'text-slate-400'
              }
            >
              {c}
            </span>
          )
        })}
      </div>
    </div>
  )
}
