// The floor's primary event source.
//
// `frontend/src/api.ts` calls straight through to fetch — no event stream, no
// websocket. This is the seam: a tiny publish/subscribe bus that the six
// mutating API calls push through on success *and* on failure, so the scene
// hears about every action a judge takes in the ordinary UI without polling.
//
// Deliberately not routed through `/audit-events`: that endpoint requires
// `monitor_all` or `view_department_audit`, which an OFFICIAL — the role that
// actually issues and verifies — holds neither of. Audit polling is a
// secondary source for other roles (see auditPoll.ts), not the primary one.

import type { SceneEvent } from './sceneApi'

type Listener = (event: SceneEvent) => void

const listeners = new Set<Listener>()

export function onSceneEvent(listener: Listener): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

export function emitSceneEvent(event: SceneEvent): void {
  remember(event)
  for (const listener of listeners) listener(event)
}

// --- recent-event guard, shared with auditPoll.ts -------------------------
//
// The audit log is a secondary source: it exists to catch actions taken
// outside this tab (a citizen approving consent on their phone), not to
// re-narrate what the api-bus already announced instantly. Without this, a
// judge's own click would animate twice — once the moment the request
// resolves, again a few seconds later when the poller notices the same row.

const RECENT_WINDOW_MS = 8000
const recent = new Map<string, number>()

function keyOf(event: SceneEvent): string {
  switch (event.kind) {
    case 'ISSUE':
    case 'VERIFY':
    case 'SUPERSEDE':
    case 'REVOKE':
      return `${event.kind}:${event.docId}`
    case 'CONSENT_REQUEST':
    case 'CONSENT_APPROVED':
    case 'CONSENT_DENIED':
      return `${event.kind}:${event.docId}`
  }
}

function remember(event: SceneEvent): void {
  const now = Date.now()
  recent.set(keyOf(event), now)
  // Prune occasionally rather than on every call.
  if (recent.size > 64) {
    for (const [key, at] of recent) if (now - at > RECENT_WINDOW_MS) recent.delete(key)
  }
}

/** True if an event with this same kind+docId was emitted very recently — the
 *  audit poller uses this to skip re-announcing what the api-bus already said. */
export function wasRecentlyEmitted(event: SceneEvent): boolean {
  const at = recent.get(keyOf(event))
  return at !== undefined && Date.now() - at < RECENT_WINDOW_MS
}
