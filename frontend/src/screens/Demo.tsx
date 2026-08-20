// The demo page: the registry floor on its own route, with no login required to
// open it — a judge can watch here even without an account.
//
// Two ways to drive it:
//   - Guided:  a scripted walkthrough. If someone is signed in, each step makes
//     a real call against the live backend (scoped to what their role can
//     actually do); if not, every step plays a preview so the story still
//     tells itself.
//   - Preview: fire any single event by hand with sample data, no backend at
//     all — useful for rehearsing how the floor reads.

import { useCallback, useEffect, useRef, useState } from 'react'
import { getMe, type ApplicationUser } from '../api'
import { HashDiff } from '../components/Hash'
import { supabase } from '../lib/supabase'
import { onSceneEvent } from '../scene/apiBus'
import { buildGuidedSteps, type GuidedStep, type StepResult } from '../scene/guidedScript'
import { ACTOR_SEATS } from '../scene/registryTheme'
import { RegistryFloor } from '../scene/RegistryFloor'
import type { SceneApi, SceneEvent } from '../scene/sceneApi'

const ACTOR_COLOR: Record<string, string> = {
  birth: 'bg-sky-500',
  transport: 'bg-emerald-500',
  education: 'bg-violet-500',
  citizen: 'bg-amber-500',
  verifier: 'bg-rose-500',
}

/** Sample events for the rehearsal panel — fixed data, no relation to whatever
 *  the guided script has actually done against the backend. */
const SAMPLES: Array<{ label: string; tone: 'good' | 'bad' | 'neutral'; event: SceneEvent }> = [
  { label: 'Issue a birth certificate', tone: 'good', event: { kind: 'ISSUE', dept: 'birth', docId: 'BC-2019-004471', ok: true } },
  { label: 'Transport tries a birth certificate', tone: 'bad', event: { kind: 'ISSUE', dept: 'transport', docId: 'BC-FAKE-0001', ok: false, reason: 'outside my department' } },
  { label: 'Verify → VALID', tone: 'good', event: { kind: 'VERIFY', docId: 'DEG-2021-1174', verdict: 'VALID' } },
  { label: 'Verify → TAMPERED', tone: 'bad', event: { kind: 'VERIFY', docId: 'DEG-2019-0455', verdict: 'TAMPERED' } },
  { label: 'Verify → REVOKED', tone: 'bad', event: { kind: 'VERIFY', docId: 'DL-GA-2016-33017', verdict: 'REVOKED' } },
  { label: 'Verify → NOT_ISSUED', tone: 'bad', event: { kind: 'VERIFY', docId: 'DL-GA-2024-99999', verdict: 'NOT_ISSUED' } },
  { label: 'Correct the birth certificate', tone: 'neutral', event: { kind: 'SUPERSEDE', dept: 'birth', docId: 'BC-2019-004471', newDocId: 'BC-2019-004471-R1' } },
  { label: 'Verify the original → SUPERSEDED', tone: 'neutral', event: { kind: 'VERIFY', docId: 'BC-2019-004471', verdict: 'SUPERSEDED' } },
  { label: 'Revoke a licence', tone: 'bad', event: { kind: 'REVOKE', dept: 'transport', docId: 'DL-GA-2016-33017' } },
  { label: 'Ask the citizen for consent', tone: 'neutral', event: { kind: 'CONSENT_REQUEST', docId: 'BC-2019-004471-R1' } },
  { label: 'Citizen approves', tone: 'good', event: { kind: 'CONSENT_APPROVED', docId: 'BC-2019-004471-R1' } },
  { label: 'Citizen refuses', tone: 'bad', event: { kind: 'CONSENT_DENIED', docId: 'BC-2019-004471-R1' } },
]

const TONE_CLASS: Record<'good' | 'bad' | 'neutral', string> = {
  good: 'border-emerald-500/40 hover:border-emerald-400 hover:bg-emerald-500/10',
  bad: 'border-rose-500/40 hover:border-rose-400 hover:bg-rose-500/10',
  neutral: 'border-slate-700 hover:border-slate-500 hover:bg-slate-800/60',
}

type StepStatus = 'pending' | 'running' | 'done' | 'error'

export function Demo() {
  const apiRef = useRef<SceneApi | null>(null)
  const [caption, setCaption] = useState('')
  const [ready, setReady] = useState(false)

  // Who, if anyone, is signed in — decides what the guided script can do for real.
  const [profile, setProfile] = useState<ApplicationUser | null>(null)
  const [profileChecked, setProfileChecked] = useState(false)

  useEffect(() => {
    supabase.auth.getSession().then(({ data }) => {
      if (!data.session) {
        setProfileChecked(true)
        return
      }
      getMe()
        .then(setProfile)
        .catch(() => setProfile(null))
        .finally(() => setProfileChecked(true))
    })
  }, [])

  // Real calls (guided mode, or anything else running in this tab) come through
  // the same bus a judge's own actions do — one animation pipeline for both.
  useEffect(() => {
    return onSceneEvent((event) => apiRef.current?.play(event))
  }, [])

  const onReady = useCallback((api: SceneApi) => {
    apiRef.current = api
    setReady(true)
  }, [])

  const fire = (event: SceneEvent) => apiRef.current?.play(event)

  // --- guided mode -----------------------------------------------------------

  const [steps, setSteps] = useState<GuidedStep[]>([])
  const [current, setCurrent] = useState(0)
  const [statuses, setStatuses] = useState<StepStatus[]>([])
  const [stepError, setStepError] = useState('')
  const [results, setResults] = useState<Record<number, StepResult>>({})
  const [running, setRunning] = useState(false)

  const startGuided = useCallback(() => {
    const built = buildGuidedSteps(profile)
    setSteps(built)
    setCurrent(0)
    setStatuses(built.map(() => 'pending'))
    setResults({})
    setStepError('')
    apiRef.current?.reset()
  }, [profile])

  useEffect(() => {
    if (profileChecked) startGuided()
  }, [profileChecked, startGuided])

  const advance = useCallback(async () => {
    const api = apiRef.current
    const step = steps[current]
    if (!api || !step || running) return
    setRunning(true)
    setStepError('')
    setStatuses((s) => s.map((status, i) => (i === current ? 'running' : status)))
    try {
      const result = await step.run(api)
      if (result?.hashes) setResults((r) => ({ ...r, [current]: result }))
      setStatuses((s) => s.map((status, i) => (i === current ? 'done' : status)))
      setCurrent((c) => Math.min(c + 1, steps.length - 1))
    } catch (error) {
      setStatuses((s) => s.map((status, i) => (i === current ? 'error' : status)))
      setStepError(error instanceof Error ? error.message : String(error))
    } finally {
      setRunning(false)
    }
  }, [steps, current, running])

  const step = steps[current]
  const atEnd = current === steps.length - 1 && statuses[current] === 'done'

  return (
    <div className="civic-theme civic-workspace min-h-screen">
      <header className="border-b border-slate-800 bg-slate-900/60">
        <div className="mx-auto max-w-6xl px-6 py-5">
          <h1 className="text-2xl font-black tracking-tight text-slate-100">
            Government Credential Registry
          </h1>
          <p className="text-sm text-slate-500">
            {!profileChecked
              ? 'checking session…'
              : profile
                ? <>signed in as <span className="text-slate-300">{profile.name}</span> ({profile.role}
                    {profile.department ? ` · ${profile.department.name}` : ''}) — guided steps run for real</>
                : 'not signed in — guided steps play as a preview, no backend calls'}
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
            Reset floor
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
          <div className="mb-4 flex items-center justify-between">
            <h2 className="text-xs font-semibold uppercase tracking-widest text-slate-500">
              Guided walkthrough
            </h2>
            <button
              onClick={startGuided}
              className="rounded-lg border border-slate-700 px-3 py-1.5 text-xs font-semibold text-slate-300 transition hover:border-slate-500 hover:text-slate-100"
            >
              Restart script
            </button>
          </div>

          <div className="mb-4 flex flex-wrap gap-1.5">
            {steps.map((s, i) => (
              <span
                key={s.id}
                title={s.title}
                className={`h-2 flex-1 min-w-[1.5rem] rounded-full ${
                  statuses[i] === 'done'
                    ? 'bg-emerald-500'
                    : statuses[i] === 'error'
                      ? 'bg-rose-500'
                      : statuses[i] === 'running'
                        ? 'bg-sky-500 animate-pulse'
                        : i === current
                          ? 'bg-slate-500'
                          : 'bg-slate-800'
                }`}
              />
            ))}
          </div>

          {step && (
            <div>
              <div className="flex items-baseline gap-2">
                <h3 className="text-lg font-semibold text-slate-100">{step.title}</h3>
                <span
                  className={`rounded-full px-2 py-0.5 text-xs font-semibold ${
                    step.mode === 'real' ? 'bg-emerald-500/15 text-emerald-300' : 'bg-slate-700 text-slate-400'
                  }`}
                >
                  {step.mode === 'real' ? 'live' : 'preview'}
                </span>
              </div>
              <p className="mt-1 text-sm text-slate-400">{step.blurb}</p>
              {step.mode === 'preview' && step.previewReason && (
                <p className="mt-1 text-xs text-slate-500">preview only — {step.previewReason}</p>
              )}

              {results[current]?.hashes && (
                <div className="mt-4">
                  <HashDiff expected={results[current].hashes!.expected} computed={results[current].hashes!.computed} />
                </div>
              )}

              {stepError && (
                <div className="mt-4 rounded-lg border border-rose-500/40 bg-rose-500/10 p-3 text-sm text-rose-300">
                  {stepError}
                </div>
              )}

              <button
                onClick={advance}
                disabled={running || atEnd}
                className="mt-4 rounded-xl bg-sky-600 px-6 py-3 text-sm font-semibold text-white transition hover:bg-sky-500 disabled:cursor-not-allowed disabled:bg-slate-700 disabled:text-slate-400"
              >
                {atEnd ? 'Done — restart to run again' : running ? 'Running…' : statuses[current] === 'error' ? 'Retry' : 'Next'}
              </button>
            </div>
          )}
        </section>

        <section className="rounded-2xl border border-slate-800 bg-slate-900/40 p-5">
          <h2 className="mb-1 text-xs font-semibold uppercase tracking-widest text-slate-500">
            Preview — sample events, no backend
          </h2>
          <p className="mb-4 text-sm text-slate-500">
            Fire any single event by hand, for rehearsing how the floor reads.
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
