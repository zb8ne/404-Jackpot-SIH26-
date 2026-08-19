// The demo page: the registry floor on its own route, with no login.
//
// Phase 2 adds a driver panel that fires each event with sample data and no
// backend at all — this is the surface for tuning how the story reads before any
// API wiring exists.

import { useCallback, useRef, useState } from 'react'
import { RegistryFloor } from '../scene/RegistryFloor'
import { ACTOR_SEATS } from '../scene/registryTheme'
import type { SceneApi, SceneEvent } from '../scene/sceneApi'

const ACTOR_COLOR: Record<string, string> = {
  birth: 'bg-sky-500',
  transport: 'bg-emerald-500',
  education: 'bg-violet-500',
  citizen: 'bg-amber-500',
  verifier: 'bg-rose-500',
}

/** The seeded demo scenario, told as scene events. */
const SAMPLES: Array<{ label: string; tone: 'good' | 'bad' | 'neutral'; event: SceneEvent }> = [
  {
    label: 'Issue a birth certificate',
    tone: 'good',
    event: { kind: 'ISSUE', dept: 'birth', docId: 'BC-2019-004471', ok: true },
  },
  {
    label: 'Transport tries a birth certificate',
    tone: 'bad',
    event: {
      kind: 'ISSUE',
      dept: 'transport',
      docId: 'BC-FAKE-0001',
      ok: false,
      reason: 'outside my department',
    },
  },
  {
    label: 'Verify → VALID',
    tone: 'good',
    event: { kind: 'VERIFY', docId: 'DEG-2021-1174', verdict: 'VALID' },
  },
  {
    label: 'Verify → TAMPERED',
    tone: 'bad',
    event: { kind: 'VERIFY', docId: 'DEG-2019-0455', verdict: 'TAMPERED' },
  },
  {
    label: 'Verify → REVOKED',
    tone: 'bad',
    event: { kind: 'VERIFY', docId: 'DL-GA-2016-33017', verdict: 'REVOKED' },
  },
  {
    label: 'Verify → NOT_ISSUED',
    tone: 'bad',
    event: { kind: 'VERIFY', docId: 'DL-GA-2024-99999', verdict: 'NOT_ISSUED' },
  },
  {
    label: 'Correct the birth certificate',
    tone: 'neutral',
    event: {
      kind: 'SUPERSEDE',
      dept: 'birth',
      docId: 'BC-2019-004471',
      newDocId: 'BC-2019-004471-R1',
    },
  },
  {
    label: 'Verify the original → SUPERSEDED',
    tone: 'neutral',
    event: { kind: 'VERIFY', docId: 'BC-2019-004471', verdict: 'SUPERSEDED' },
  },
  {
    label: 'Revoke a licence',
    tone: 'bad',
    event: { kind: 'REVOKE', dept: 'transport', docId: 'DL-GA-2016-33017' },
  },
  {
    label: 'Ask the citizen for consent',
    tone: 'neutral',
    event: { kind: 'CONSENT_REQUEST', docId: 'BC-2019-004471-R1' },
  },
  {
    label: 'Citizen approves',
    tone: 'good',
    event: { kind: 'CONSENT_APPROVED', docId: 'BC-2019-004471-R1' },
  },
  {
    label: 'Citizen refuses',
    tone: 'bad',
    event: { kind: 'CONSENT_DENIED', docId: 'BC-2019-004471-R1' },
  },
]

const TONE_CLASS: Record<'good' | 'bad' | 'neutral', string> = {
  good: 'border-emerald-500/40 hover:border-emerald-400 hover:bg-emerald-500/10',
  bad: 'border-rose-500/40 hover:border-rose-400 hover:bg-rose-500/10',
  neutral: 'border-slate-700 hover:border-slate-500 hover:bg-slate-800/60',
}

export function Demo() {
  const apiRef = useRef<SceneApi | null>(null)
  const [caption, setCaption] = useState('')
  const [ready, setReady] = useState(false)

  const onReady = useCallback((api: SceneApi) => {
    apiRef.current = api
    setReady(true)
  }, [])

  const fire = (event: SceneEvent) => apiRef.current?.play(event)

  return (
    <div className="min-h-screen bg-slate-950">
      <header className="border-b border-slate-800 bg-slate-900/60">
        <div className="mx-auto max-w-6xl px-6 py-5">
          <h1 className="text-2xl font-black tracking-tight text-slate-100">
            Government Credential Registry
          </h1>
          <p className="text-sm text-slate-500">
            The registry floor — departments issue, citizens hold, verifiers check.
          </p>
        </div>
      </header>

      <main className="mx-auto max-w-6xl space-y-6 px-6 py-8">
        <RegistryFloor onReady={onReady} onCaption={setCaption} />

        <div className="flex min-h-[2rem] items-center justify-between gap-4">
          <p className="text-lg text-slate-200">{caption || <span className="text-slate-600">idle</span>}</p>
          <button
            onClick={() => apiRef.current?.reset()}
            className="rounded-lg border border-slate-700 px-4 py-2 text-sm font-semibold text-slate-300 transition hover:border-slate-500 hover:text-slate-100"
          >
            Reset
          </button>
        </div>

        <div className="flex flex-wrap gap-x-6 gap-y-3">
          {ACTOR_SEATS.map((seat) => (
            <div key={seat.id} className="flex items-center gap-2 text-sm text-slate-300">
              <span className={`h-3 w-3 rounded-full ${ACTOR_COLOR[seat.id] ?? 'bg-slate-500'}`} />
              {seat.label}
            </div>
          ))}
        </div>

        <section className="rounded-2xl border border-slate-800 bg-slate-900/40 p-5">
          <h2 className="mb-1 text-xs font-semibold uppercase tracking-widest text-slate-500">
            Driver — sample events, no backend
          </h2>
          <p className="mb-4 text-sm text-slate-500">
            Events queue and play one at a time. This panel goes away once the guided script and
            live feed are wired.
          </p>
          <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
            {SAMPLES.map((sample) => (
              <button
                key={sample.label}
                disabled={!ready}
                onClick={() => fire(sample.event)}
                className={`rounded-lg border px-4 py-3 text-left text-sm text-slate-200 transition disabled:cursor-not-allowed disabled:opacity-40 ${TONE_CLASS[sample.tone]}`}
              >
                {sample.label}
              </button>
            ))}
          </div>
        </section>
      </main>
    </div>
  )
}
