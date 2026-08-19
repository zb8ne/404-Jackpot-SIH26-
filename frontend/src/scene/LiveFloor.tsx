// The registry floor, mounted inside the authenticated app. A judge driving
// Issue/Verify/Revoke in the ordinary tabs sees the same actions animate here —
// this is "live mode": no separate page, no script, just what actually happened.
//
// Collapsible because the floor is a lot of screen real estate for someone who
// just wants to fill out a form; it opens the first time an event arrives.

import { useEffect, useRef, useState } from 'react'
import { getMe } from '../api'
import { onSceneEvent } from './apiBus'
import { startAuditPoll } from './auditPoll'
import { RegistryFloor } from './RegistryFloor'
import type { SceneApi, SceneEvent } from './sceneApi'

export function LiveFloor() {
  const apiRef = useRef<SceneApi | null>(null)
  const [open, setOpen] = useState(false)
  const [caption, setCaption] = useState('')
  const [activity, setActivity] = useState(0)

  // Queue events that arrive before the scene has finished booting.
  const pendingRef = useRef<SceneEvent[]>([])

  useEffect(() => {
    const unsubscribe = onSceneEvent((event) => {
      setActivity((n) => n + 1)
      setOpen(true)
      if (apiRef.current) apiRef.current.play(event)
      else pendingRef.current.push(event)
    })
    return unsubscribe
  }, [])

  // The audit poll is a secondary source: only roles that can read
  // /audit-events benefit from it, and it stops itself permanently on 403.
  useEffect(() => {
    let stop: (() => void) | undefined
    getMe()
      .then((me) => {
        if (me.role !== 'OFFICIAL') stop = startAuditPoll()
      })
      .catch(() => {
        /* not signed in yet, or /me failed — no secondary source, api-bus still works */
      })
    return () => stop?.()
  }, [])

  return (
    <div className="mb-8 overflow-hidden rounded-2xl border border-slate-800 bg-slate-900/40">
      <button
        onClick={() => setOpen((o) => !o)}
        className="flex w-full items-center justify-between px-5 py-3 text-left"
      >
        <span className="flex items-center gap-2 text-sm font-semibold text-slate-300">
          Live floor
          {activity > 0 && (
            <span className="rounded-full bg-sky-500/20 px-2 py-0.5 text-xs text-sky-300">{activity}</span>
          )}
        </span>
        <span className="text-xs text-slate-500">{open ? 'hide' : 'show'}</span>
      </button>

      {open && (
        <div className="border-t border-slate-800 px-5 py-5">
          <RegistryFloor
            onReady={(api) => {
              apiRef.current = api
              for (const event of pendingRef.current.splice(0)) api.play(event)
            }}
            onCaption={setCaption}
          />
          <p className="mt-3 min-h-[1.5rem] text-sm text-slate-400">{caption}</p>
        </div>
      )}
    </div>
  )
}
