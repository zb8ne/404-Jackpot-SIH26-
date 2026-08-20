// The registry floor, mounted inside the authenticated app. A judge driving
// Issue/Verify/Revoke in the ordinary tabs sees the same actions animate here —
// this is "live mode": no separate page, no script, just what actually happened.
//
// Collapsible because the floor is a lot of screen real estate for someone who
// just wants to fill out a form; it opens the first time an event arrives.

import { useCallback, useEffect, useRef, useState } from 'react'
import type { ApplicationUser } from '../api'
import { AuditEvents } from '../screens/AuditEvents'
import { Citizen } from '../screens/Citizen'
import { GovernmentRequests } from '../screens/GovernmentRequests'
import { Issue } from '../screens/Issue'
import { Revoke, Supersede } from '../screens/Lifecycle'
import { Monitoring, type MonitoringFocus } from '../screens/Monitoring'
import { Verify } from '../screens/Verify'
import { onSceneEvent } from './apiBus'
import { startAuditPoll } from './auditPoll'
import { RegistryFloor } from './RegistryFloor'
import type { SceneApi, SceneEvent } from './sceneApi'

type FloorAction = {
  id: string
  label: string
  hint: string
  icon: string
  tone: string
}

const ACTIONS: Record<ApplicationUser['role'], { left: FloorAction[]; right: FloorAction[] }> = {
  CONTROLLER: {
    left: [
      { id: 'activity', label: 'System activity', hint: 'All departments', icon: '◉', tone: 'sky' },
      { id: 'audit', label: 'Audit trail', hint: 'Registry events', icon: '≡', tone: 'violet' },
    ],
    right: [
      { id: 'integrity', label: 'Integrity', hint: 'Audit chain', icon: '✓', tone: 'emerald' },
      { id: 'health', label: 'Department health', hint: 'Service overview', icon: '◈', tone: 'amber' },
    ],
  },
  ADMIN: {
    left: [
      { id: 'issue', label: 'Issue', hint: 'New credential', icon: '+', tone: 'sky' },
      { id: 'verify', label: 'Verify', hint: 'Check a PDF', icon: '✓', tone: 'emerald' },
      { id: 'credentials', label: 'Credentials', hint: 'Department records', icon: '▤', tone: 'violet' },
      { id: 'audit', label: 'Audit trail', hint: 'Department events', icon: '≡', tone: 'cyan' },
    ],
    right: [
      { id: 'supersede', label: 'Supersede', hint: 'Issue correction', icon: '↻', tone: 'amber' },
      { id: 'revoke', label: 'Revoke', hint: 'End validity', icon: '×', tone: 'rose' },
      { id: 'requests', label: 'Requests', hint: 'Citizen consent', icon: '→', tone: 'cyan' },
    ],
  },
  OFFICIAL: {
    left: [
      { id: 'issue', label: 'Issue', hint: 'New credential', icon: '+', tone: 'sky' },
      { id: 'verify', label: 'Verify', hint: 'Check a PDF', icon: '✓', tone: 'emerald' },
    ],
    right: [
      { id: 'credentials', label: 'Credentials', hint: 'Department records', icon: '▤', tone: 'violet' },
      { id: 'requests', label: 'Requests', hint: 'Citizen consent', icon: '→', tone: 'cyan' },
    ],
  },
}

// A "slot" tone: badge fill/text, a hard pixel-shadow color to match, and the
// selected-state background wash. Hard offset shadows (no blur) instead of
// Tailwind's soft shadow-xl read as a game HUD next to the pixel office art;
// blurred shadows look like ordinary web chrome sitting on top of it.
const TONES: Record<string, { badge: string; shadow: string; selected: string }> = {
  sky: { badge: 'border-[#f07868]/40 bg-[#f07868]/15 text-[#ff9a89]', shadow: 'shadow-[3px_3px_0_0_rgba(240,120,104,0.45)]', selected: 'bg-[#f07868]/10' },
  emerald: { badge: 'border-emerald-500/40 bg-emerald-500/15 text-emerald-300', shadow: 'shadow-[3px_3px_0_0_rgba(16,185,129,0.45)]', selected: 'bg-emerald-500/10' },
  violet: { badge: 'border-violet-500/40 bg-violet-500/15 text-violet-300', shadow: 'shadow-[3px_3px_0_0_rgba(139,92,246,0.45)]', selected: 'bg-violet-500/10' },
  amber: { badge: 'border-amber-500/40 bg-amber-500/15 text-amber-300', shadow: 'shadow-[3px_3px_0_0_rgba(245,158,11,0.45)]', selected: 'bg-amber-500/10' },
  rose: { badge: 'border-rose-500/40 bg-rose-500/15 text-rose-300', shadow: 'shadow-[3px_3px_0_0_rgba(244,63,94,0.45)]', selected: 'bg-rose-500/10' },
  cyan: { badge: 'border-cyan-500/40 bg-cyan-500/15 text-cyan-300', shadow: 'shadow-[3px_3px_0_0_rgba(6,182,212,0.45)]', selected: 'bg-cyan-500/10' },
}

function ActionRail({ actions, side, active, onSelect, floating = false }: { actions: FloorAction[]; side: 'left' | 'right'; active: string; onSelect: (action: FloorAction) => void; floating?: boolean }) {
  return (
    <div className={floating ? 'flex flex-col gap-2' : 'grid grid-cols-2 gap-2 sm:grid-cols-3'} aria-label={`${side} floor controls`}>
      {actions.map((action) => {
        const tone = TONES[action.tone]
        const isActive = active === action.id
        return (
          <button
            key={action.label}
            type="button"
            onClick={() => onSelect(action)}
            aria-pressed={isActive}
            title={`${action.label} — open workspace`}
            className={`group flex items-center rounded-lg border-2 bg-slate-950/90 p-3 backdrop-blur transition-all duration-100 hover:-translate-x-0.5 hover:-translate-y-0.5 active:translate-x-0 active:translate-y-0 active:shadow-none ${tone.shadow} ${floating ? 'h-[5.5rem] w-28 flex-col justify-center gap-2 text-center xl:w-32' : 'min-h-20 gap-3 text-left'} ${isActive ? `border-slate-300 ${tone.selected}` : 'border-slate-700'}`}
          >
            <span className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-md border-2 font-mono text-xl font-black shadow-[inset_0_1px_0_0_rgba(255,255,255,0.2)] transition group-hover:brightness-125 ${tone.badge}`}>
              {action.icon}
            </span>
            <span className={floating ? 'w-full' : ''}>
              <span className={`block font-bold uppercase tracking-wider text-slate-200 ${floating ? 'whitespace-nowrap text-[11px] leading-none' : 'text-xs'}`}>{action.label}</span>
              {!floating && <span className="block text-[11px] normal-case leading-tight text-slate-500">{action.hint}</span>}
            </span>
          </button>
        )
      })}
    </div>
  )
}

function ConnectedPanel({ action, onClose, onComplete }: { action: FloorAction; onClose: () => void; onComplete: () => void }) {
  const content = action.id === 'issue' ? <Issue onSuccess={onComplete} />
    : action.id === 'verify' ? <Verify />
      : action.id === 'credentials' ? <Citizen />
        : action.id === 'supersede' ? <Supersede />
          : action.id === 'revoke' ? <Revoke />
            : action.id === 'requests' ? <GovernmentRequests />
              : action.id === 'audit' ? <AuditEvents />
                : <Monitoring focus={action.id as MonitoringFocus} />
  return (
    <section className="overflow-hidden rounded-2xl border border-slate-700 bg-slate-900/70 shadow-2xl shadow-black/30">
      <div className="flex items-center justify-between border-b border-slate-800 px-4 py-3">
        <div><p className="font-bold text-slate-100">{action.label}</p><p className="text-[11px] text-slate-500">Connected workspace</p></div>
        <button type="button" onClick={onClose} className="rounded-lg px-3 py-1.5 text-slate-500 hover:bg-slate-800 hover:text-slate-200" aria-label="Close workspace">×</button>
      </div>
      <div className="p-4 sm:p-6">{content}</div>
    </section>
  )
}

export function LiveFloor({ profile }: { profile: ApplicationUser }) {
  const apiRef = useRef<SceneApi | null>(null)
  const [open, setOpen] = useState(true)
  const [caption, setCaption] = useState('')
  const [activity, setActivity] = useState(0)
  const [selectedAction, setSelectedAction] = useState<FloorAction | null>(null)
  const floorRef = useRef<HTMLDivElement | null>(null)
  const panelRef = useRef<HTMLDivElement | null>(null)

  // Queue events that arrive before the scene has finished booting.
  const pendingRef = useRef<SceneEvent[]>([])
  const actions = ACTIONS[profile.role]
  const roleAccent = profile.role === 'ADMIN' ? 'text-[#f07868]' : profile.role === 'OFFICIAL' ? 'text-[#65c8a3]' : 'text-[#e8b45b]'
  const toggleAction = (action: FloorAction) => {
    setSelectedAction((current) => current?.id === action.id ? null : action)
  }

  const returnToFloor = useCallback(() => {
    // Let the connected form commit its success result before moving the
    // viewport; otherwise browser scroll anchoring can cancel the smooth jump.
    window.setTimeout(() => {
      floorRef.current?.scrollIntoView({ behavior: 'smooth', block: 'center' })
    }, 80)
  }, [])

  useEffect(() => {
    if (!selectedAction) return
    const frame = requestAnimationFrame(() => panelRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' }))
    return () => cancelAnimationFrame(frame)
  }, [selectedAction])

  useEffect(() => {
    const unsubscribe = onSceneEvent((event) => {
      setActivity((n) => n + 1)
      setOpen(true)
      if (apiRef.current) apiRef.current.play(event)
      else pendingRef.current.push(event)
      returnToFloor()
    })
    return unsubscribe
  }, [returnToFloor])

  // The audit poll is a secondary source: only roles that can read
  // /audit-events benefit from it, and it stops itself permanently on 403.
  useEffect(() => {
    const stop = profile.role !== 'OFFICIAL' ? startAuditPoll() : undefined
    return () => stop?.()
  }, [profile.role])

  return (
    <div className="civic-theme mb-8 overflow-hidden rounded-3xl border border-[#493f39] bg-[#181b1b] shadow-2xl shadow-black/30">
      <button
        onClick={() => setOpen((o) => !o)}
        className="flex w-full items-center justify-between border-b border-[#3a322e] bg-[#201d1b] px-5 py-3 text-left"
      >
        <span className="flex items-center gap-2 text-sm font-semibold text-slate-300">
          Live floor
          {activity > 0 && (
            <span className="rounded-full bg-[#f07868]/20 px-2 py-0.5 text-xs text-[#ff9a89]">{activity}</span>
          )}
        </span>
        <span className="text-xs text-slate-500">{open ? 'hide' : 'show'}</span>
      </button>

      {open && (
        <div className="bg-[radial-gradient(circle_at_50%_0%,rgba(240,120,104,.07),transparent_36%),linear-gradient(180deg,#181b1b,#121516)] px-4 py-5 sm:px-5">
          <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
            <div>
              <p className={`text-xs font-semibold uppercase tracking-[0.2em] ${roleAccent}`}>{profile.role} operations floor</p>
              <p className="text-sm text-[#aaa198]">{profile.department?.name ?? 'System-wide registry oversight'}</p>
            </div>
            <span className="rounded-full border border-emerald-500/20 bg-emerald-500/10 px-3 py-1 text-[10px] font-semibold uppercase tracking-wider text-emerald-300">
              ● Live workspace
            </span>
          </div>

          <div className="space-y-3 lg:space-y-0">
            <div className="lg:hidden">
              <ActionRail actions={actions.left} side="left" active={selectedAction?.id ?? ''} onSelect={toggleAction} />
            </div>

            <div ref={floorRef} className="relative scroll-mt-6">
              <RegistryFloor
                onReady={(api) => {
                  apiRef.current = api
                  for (const event of pendingRef.current.splice(0)) api.play(event)
                }}
                onCaption={setCaption}
              />

              <div className="absolute inset-y-0 left-3 z-10 hidden items-center lg:flex">
                <ActionRail actions={actions.left} side="left" active={selectedAction?.id ?? ''} onSelect={toggleAction} floating />
              </div>
              <div className="absolute inset-y-0 right-3 z-10 hidden items-center lg:flex">
                <ActionRail actions={actions.right} side="right" active={selectedAction?.id ?? ''} onSelect={toggleAction} floating />
              </div>
            </div>

            <div className="lg:hidden">
              <ActionRail actions={actions.right} side="right" active={selectedAction?.id ?? ''} onSelect={toggleAction} />
            </div>
            {selectedAction && (
              <div ref={panelRef} className="mx-auto w-full max-w-5xl scroll-mt-6 pt-3">
                <ConnectedPanel action={selectedAction} onClose={() => setSelectedAction(null)} onComplete={returnToFloor} />
              </div>
            )}
          </div>
          <p className="mt-3 min-h-[1.5rem] text-sm text-slate-400">{caption}</p>
        </div>
      )}
    </div>
  )
}
