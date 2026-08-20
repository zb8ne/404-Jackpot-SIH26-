// Secondary event source: polls the audit log for actions the api-bus never
// saw, because they didn't happen in this tab — a citizen approving consent on
// their own phone is the case that actually matters here.
//
// Not usable as the primary source: `/audit-events` requires `monitor_all` or
// `view_department_audit`, and OFFICIAL — the role that actually issues and
// verifies — holds neither. Only ADMIN (own department) and CONTROLLER
// (everything) can even read it, so this is best-effort supplementary
// coverage, not the backbone.

import { getAuditEvents, type AuditEvent } from '../api'
import { emitSceneEvent, wasRecentlyEmitted } from './apiBus'
import type { DeptId, SceneEvent, Verdict } from './sceneApi'

const POLL_MS = 4000
const PAGE_SIZE = 25
const VERDICTS = new Set<Verdict>(['VALID', 'SUPERSEDED', 'REVOKED', 'TAMPERED', 'NOT_ISSUED'])
const DEPT_IDS = new Set<DeptId>(['birth', 'transport', 'education'])

function toSceneEvent(row: AuditEvent): SceneEvent | null {
  const dept = row.department?.id
  const docId = row.credential.docId ?? ''

  switch (row.action) {
    case 'ISSUE':
      if (!dept || !DEPT_IDS.has(dept as DeptId) || !docId) return null
      return { kind: 'ISSUE', dept: dept as DeptId, docId, ok: row.outcome === 'SUCCESS', reason: row.error ?? undefined }

    case 'VERIFY_FILE':
    case 'VERIFY_ID':
      if (!docId || !VERDICTS.has(row.result as Verdict)) return null
      return { kind: 'VERIFY', docId, verdict: row.result as Verdict }

    case 'REVOKE':
      if (!dept || !DEPT_IDS.has(dept as DeptId) || !docId) return null
      return { kind: 'REVOKE', dept: dept as DeptId, docId }

    case 'SUPERSEDE':
      // The row only carries the new document's id; the choreography treats
      // old and new the same here rather than guessing at the original.
      if (!dept || !DEPT_IDS.has(dept as DeptId) || !docId) return null
      return { kind: 'SUPERSEDE', dept: dept as DeptId, docId, newDocId: docId }

    case 'VERIFICATION_REQUEST_CREATED':
      return docId ? { kind: 'CONSENT_REQUEST', docId } : null

    case 'CONSENT_APPROVED':
      return docId ? { kind: 'CONSENT_APPROVED', docId } : null

    case 'CONSENT_DENIED':
      return docId ? { kind: 'CONSENT_DENIED', docId } : null

    default:
      return null
  }
}

/** Starts polling and returns a stop function. Silently gives up for good on
 *  the first 403 — that role simply cannot read this endpoint, and retrying
 *  would just spam it. */
export function startAuditPoll(): () => void {
  let lastSeenId = -1
  let stopped = false
  let seeded = false
  let timer: ReturnType<typeof setTimeout> | undefined

  async function tick() {
    if (stopped) return
    try {
      const response = await getAuditEvents({ limit: PAGE_SIZE })
      // Newest-first. On the very first successful poll, just record the
      // high-water mark — a floor that just mounted shouldn't replay history.
      if (!seeded) {
        seeded = true
        lastSeenId = response.events[0]?.id ?? 0
      } else {
        const fresh = response.events.filter((row) => row.id > lastSeenId).reverse()
        for (const row of fresh) {
          const event = toSceneEvent(row)
          if (event && !wasRecentlyEmitted(event)) emitSceneEvent(event)
        }
        if (response.events[0]) lastSeenId = Math.max(lastSeenId, response.events[0].id)
      }
    } catch (error) {
      // A 403 here means the role cannot read the audit log at all — that's
      // permanent for the session, not worth retrying.
      if (error instanceof Error && /403|forbidden/i.test(error.message)) {
        stopped = true
        return
      }
      // Anything else (a dropped connection) is worth trying again.
    }
    if (!stopped) timer = setTimeout(tick, POLL_MS)
  }

  void tick()
  return () => {
    stopped = true
    if (timer) clearTimeout(timer)
  }
}
