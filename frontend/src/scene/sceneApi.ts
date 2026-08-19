// The contract between the registry and the floor.
//
// Everything the scene can show is one of these events. Both demo modes speak
// this vocabulary — the guided script emits them as it walks its steps, and live
// mode emits them from real API calls — so there is a single set of animations
// to build and debug rather than one per mode.

export type DeptId = 'birth' | 'transport' | 'education';

export type Verdict = 'VALID' | 'SUPERSEDED' | 'REVOKED' | 'TAMPERED' | 'NOT_ISSUED';

export type SceneEvent =
  /** A department issued a document — or was refused. `ok: false` is the
   *  overreach: a department reaching for a type it does not hold. */
  | { kind: 'ISSUE'; dept: DeptId; docId: string; ok: boolean; reason?: string }
  /** Someone checked a document and got a verdict back. */
  | { kind: 'VERIFY'; docId: string; verdict: Verdict }
  /** A document was replaced by a corrected version; both stay on record. */
  | { kind: 'SUPERSEDE'; dept: DeptId; docId: string; newDocId: string }
  /** A genuine document was withdrawn by its issuing department. */
  | { kind: 'REVOKE'; dept: DeptId; docId: string }
  /** A verifier asked the citizen's permission before seeing a verdict. */
  | { kind: 'CONSENT_REQUEST'; docId: string }
  | { kind: 'CONSENT_APPROVED'; docId: string }
  | { kind: 'CONSENT_DENIED'; docId: string };

/** The imperative handle the scene hands back once it is running. */
export interface SceneApi {
  /** Queue an event. Events play one at a time, in order. */
  play(event: SceneEvent): void;
  /** Drop anything queued and return the cast to their desks. */
  reset(): void;
  /** How many events are waiting, including the one playing. */
  pending(): number;
}

/** Short caption for an event — shown under the floor while it plays. */
export function describe(event: SceneEvent): string {
  switch (event.kind) {
    case 'ISSUE':
      return event.ok
        ? `${deptName(event.dept)} issued ${event.docId} to the citizen`
        : `${deptName(event.dept)} was refused: ${event.reason ?? 'not its document type'}`;
    case 'VERIFY':
      return `${event.docId} checked — ${VERDICT_COPY[event.verdict]}`;
    case 'SUPERSEDE':
      return `${event.docId} corrected — ${event.newDocId} is now current`;
    case 'REVOKE':
      return `${deptName(event.dept)} revoked ${event.docId}`;
    case 'CONSENT_REQUEST':
      return `Consent requested from the citizen for ${event.docId}`;
    case 'CONSENT_APPROVED':
      return `Citizen approved the check on ${event.docId}`;
    case 'CONSENT_DENIED':
      return `Citizen refused the check on ${event.docId}`;
  }
}

export const VERDICT_COPY: Record<Verdict, string> = {
  VALID: 'authentic and current',
  SUPERSEDED: 'genuine, but replaced',
  REVOKED: 'genuine, but withdrawn',
  TAMPERED: 'the bytes do not match the record',
  NOT_ISSUED: 'never issued',
};

export function deptName(dept: DeptId): string {
  return { birth: 'Birth Dept', transport: 'Transport Dept', education: 'Education Dept' }[dept];
}
