// The demo page: the registry floor, on its own route, with no login.
//
// Phase 1 renders the floor and the cast. The guided step bar and the live event
// feed land on top of this in the next phases.

import { RegistryFloor } from '../scene/RegistryFloor'
import { ACTOR_SEATS } from '../scene/registryTheme'

const ACTOR_COLOR: Record<string, string> = {
  birth: 'bg-sky-500',
  transport: 'bg-emerald-500',
  education: 'bg-violet-500',
  citizen: 'bg-amber-500',
  verifier: 'bg-rose-500',
}

export function Demo() {
  return (
    <div className="min-h-screen bg-slate-950">
      <header className="border-b border-slate-800 bg-slate-900/60">
        <div className="mx-auto max-w-6xl px-6 py-5">
          <h1 className="text-2xl font-black tracking-tight text-slate-100">
            Government Credential Registry
          </h1>
          <p className="text-sm text-slate-500">
            A live view of the registry floor — departments issue, citizens hold, verifiers check.
          </p>
        </div>
      </header>

      <main className="mx-auto max-w-6xl space-y-6 px-6 py-8">
        <RegistryFloor />

        <div className="flex flex-wrap gap-x-6 gap-y-3">
          {ACTOR_SEATS.map((seat) => (
            <div key={seat.id} className="flex items-center gap-2 text-sm text-slate-300">
              <span className={`h-3 w-3 rounded-full ${ACTOR_COLOR[seat.id] ?? 'bg-slate-500'}`} />
              {seat.label}
            </div>
          ))}
        </div>
      </main>
    </div>
  )
}
