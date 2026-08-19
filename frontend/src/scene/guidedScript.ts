// The guided demo: one script, real registry calls, walked one step at a time.
//
// What "real" means depends on who is signed in — the backend's RBAC is not a
// suggestion, so the script only attempts what the signed-in role can actually
// do and is honest on screen about the rest:
//
//   not signed in     -> every step plays as a preview (a canned SceneEvent,
//                        no network call) so the page is still safe to open
//                        with no account and still tells the whole story
//   OFFICIAL           -> issue, verify and the consent flow run for real;
//                        supersede/revoke preview (RBAC reserves those for ADMIN)
//   ADMIN               -> the entire sequence runs for real
//   CONTROLLER          -> preview only (monitors, does not act)
//
// Each step emits through the ordinary api.ts -> apiBus pipeline when it runs
// for real, so it animates the floor exactly the way a judge's own actions do —
// there is no separate "guided" animation path to keep in sync.

import {
  completeVerificationRequest,
  createVerificationRequest,
  decideConsent,
  getCitizenAccounts,
  getDevelopmentConsentURL,
  issueDocument,
  revokeDocument,
  supersedeDocument,
  verifyById,
  verifyFile,
  type ApplicationUser,
} from '../api'
import type { SceneApi, SceneEvent } from './sceneApi'

export type StepMode = 'real' | 'preview'

/** Extra detail a step can hand back for the UI to render — currently just the
 *  tampered step's hash pair, so the panel can show the mismatch it just found. */
export interface StepResult {
  hashes?: { expected: string; computed: string }
}

export interface GuidedStep {
  id: string
  title: string
  blurb: string
  mode: StepMode
  /** Why this step can't run for real right now, shown next to the preview badge. */
  previewReason?: string
  run: (scene: SceneApi) => Promise<StepResult | void>
}

const DOC_TYPE_BY_NAME: Record<string, string> = {
  birth_certificate: 'BC',
  driving_licence: 'DL',
  degree_certificate: 'DEG',
}

// Any doc type this department does NOT hold, for the deliberate rejection step.
const OTHER_DOC_TYPE: Record<string, { docType: string; label: string }> = {
  birth_certificate: { docType: 'driving_licence', label: 'a driving licence' },
  driving_licence: { docType: 'degree_certificate', label: 'a degree certificate' },
  degree_certificate: { docType: 'birth_certificate', label: 'a birth certificate' },
}

function freshDocId(docType: string): string {
  const prefix = DOC_TYPE_BY_NAME[docType] ?? 'DOC'
  const random = crypto.randomUUID().replaceAll('-', '').slice(0, 6).toUpperCase()
  return `${prefix}-DEMO-${random}`
}

async function blankCredential(name: string): Promise<File> {
  const res = await fetch('/demo/blank-credential.pdf')
  const blob = await res.blob()
  return new File([blob], name, { type: 'application/pdf' })
}

/** Flip one byte in the document body — the same shape of tamper the seeder
 *  produces: genuine marker, bytes that no longer match what was anchored. */
async function tamperedCopy(original: File): Promise<File> {
  const bytes = new Uint8Array(await original.arrayBuffer())
  // Somewhere past the registry page's own bytes, so the marker survives.
  const at = Math.floor(bytes.length * 0.35)
  bytes[at] = bytes[at] ^ 0xff
  return new File([bytes], 'tampered.pdf', { type: 'application/pdf' })
}

function extractToken(consentUrl: string): string {
  const path = new URL(consentUrl, window.location.origin).pathname
  return decodeURIComponent(path.split('/').filter(Boolean).pop() ?? '')
}

async function preview(scene: SceneApi, event: SceneEvent, delayMs = 200): Promise<void> {
  scene.play(event)
  // A beat so the step bar's "running" state is visible even for an instant
  // preview — otherwise every preview step looks like it did nothing.
  await new Promise((resolve) => setTimeout(resolve, delayMs))
}

/**
 * Builds the ordered script for the signed-in profile (or `null` for an
 * anonymous visitor). Steps carry their own state via closures — each run only
 * once per generated script, so re-entering guided mode calls this again with
 * fresh document ids.
 */
export function buildGuidedSteps(profile: ApplicationUser | null): GuidedStep[] {
  const dept = profile?.department
  const canIssueVerify = profile?.role === 'OFFICIAL' || profile?.role === 'ADMIN'
  const canLifecycle = profile?.role === 'ADMIN'
  const deptId = (dept?.id ?? 'birth') as 'birth' | 'transport' | 'education'
  const deptDocType = dept?.docTypeName ?? 'birth_certificate'
  const other = OTHER_DOC_TYPE[deptDocType]

  const notSignedIn = !profile ? 'sign in to run this step for real' : undefined
  const notAdmin = !canLifecycle ? 'requires an ADMIN session' : undefined

  // Threaded across steps.
  const state: {
    citizenAccountId: string
    citizenName: string
    docId: string
    docHash: string
    correctedDocId: string
    correctedHash: string
    issuedFile: File | null
    verificationRequestId: string
    consentToken: string
  } = {
    citizenAccountId: '',
    citizenName: '',
    docId: freshDocId(deptDocType),
    docHash: '',
    correctedDocId: '',
    correctedHash: '',
    issuedFile: null,
    verificationRequestId: '',
    consentToken: '',
  }

  const steps: GuidedStep[] = []

  steps.push({
    id: 'issue',
    title: `${dept?.name ?? 'A department'} issues ${state.docId}`,
    blurb: 'A genuine document, anchored on chain and handed to a citizen.',
    mode: canIssueVerify ? 'real' : 'preview',
    previewReason: notSignedIn,
    run: async (scene) => {
      if (!canIssueVerify) {
        await preview(scene, { kind: 'ISSUE', dept: deptId, docId: state.docId, ok: true })
        return
      }
      const accounts = await getCitizenAccounts()
      const account = accounts[0]
      if (!account) throw new Error('no citizen accounts to issue against — run the seeder first')
      state.citizenAccountId = account.id
      state.citizenName = account.displayName
      state.issuedFile = await blankCredential(`${state.docId}.pdf`)
      const result = await issueDocument({
        file: state.issuedFile,
        dept: deptId,
        docType: deptDocType,
        docId: state.docId,
        citizen: state.citizenName,
        citizenAccountId: state.citizenAccountId,
      })
      state.docHash = result.docHash
    },
  })

  const deniedDocId = freshDocId(other.docType)
  steps.push({
    id: 'issue-denied',
    title: `${dept?.name ?? 'The same department'} is refused for ${other.label}`,
    blurb: 'Outside its role — the backend rejects it before the chain is ever touched.',
    mode: canIssueVerify ? 'real' : 'preview',
    previewReason: notSignedIn,
    run: async (scene) => {
      if (!canIssueVerify) {
        await preview(scene, {
          kind: 'ISSUE',
          dept: deptId,
          docId: deniedDocId,
          ok: false,
          reason: 'document type is outside the authenticated department',
        })
        return
      }
      const file = await blankCredential('denied.pdf')
      const accounts = await getCitizenAccounts()
      const account = accounts[0]
      try {
        await issueDocument({
          file,
          dept: deptId,
          docType: other.docType,
          docId: deniedDocId,
          citizen: account?.displayName ?? 'Demo Citizen',
          citizenAccountId: account?.id ?? '',
        })
        // If this somehow succeeds the RBAC model has a hole — surface it loudly.
        throw new Error('expected this issue to be refused, but it succeeded')
      } catch (error) {
        // The rejection IS the point of this step; api.ts already emitted the
        // ISSUE/ok:false event for us. Only a genuinely unexpected failure
        // (not the 403 we're demonstrating) should propagate.
        if (error instanceof Error && /outside the authenticated department/i.test(error.message)) return
        throw error
      }
    },
  })

  steps.push({
    id: 'verify-valid',
    title: `Verify ${state.docId} — should read VALID`,
    blurb: 'Hashes match what the registry anchored.',
    mode: canIssueVerify ? 'real' : 'preview',
    previewReason: notSignedIn,
    run: async (scene) => {
      if (!canIssueVerify) {
        await preview(scene, { kind: 'VERIFY', docId: state.docId, verdict: 'VALID' })
        return
      }
      await verifyById(state.docId)
    },
  })

  steps.push({
    id: 'supersede',
    title: `Correct ${state.docId}`,
    blurb: 'A fresh version replaces it; the original stays on record as SUPERSEDED.',
    mode: canLifecycle ? 'real' : 'preview',
    previewReason: notAdmin,
    run: async (scene) => {
      state.correctedDocId = `${state.docId}-R1`
      if (!canLifecycle) {
        await preview(scene, {
          kind: 'SUPERSEDE',
          dept: deptId,
          docId: state.docId,
          newDocId: state.correctedDocId,
        })
        return
      }
      const file = await blankCredential(`${state.correctedDocId}.pdf`)
      const result = await supersedeDocument({
        file,
        dept: deptId,
        oldHash: state.docHash,
        oldDocId: state.docId,
        docId: state.correctedDocId,
        citizen: state.citizenName,
        citizenAccountId: state.citizenAccountId,
      })
      state.correctedHash = result.docHash
    },
  })

  steps.push({
    id: 'verify-superseded',
    title: `Verify the original ${state.docId} — should read SUPERSEDED`,
    blurb: 'Still genuine; the registry points on to the version that replaced it.',
    mode: canIssueVerify ? 'real' : 'preview',
    previewReason: notSignedIn,
    run: async (scene) => {
      if (!canIssueVerify) {
        await preview(scene, { kind: 'VERIFY', docId: state.docId, verdict: 'SUPERSEDED' })
        return
      }
      await verifyById(state.docId)
    },
  })

  steps.push({
    id: 'tampered',
    title: 'Verify a tampered copy',
    blurb: 'Same marker, edited bytes — the hashes will not match.',
    mode: canIssueVerify && state.issuedFile ? 'real' : 'preview',
    previewReason: notSignedIn,
    run: async (scene) => {
      if (!canIssueVerify || !state.issuedFile) {
        await preview(scene, { kind: 'VERIFY', docId: state.docId, verdict: 'TAMPERED' })
        return
      }
      const tampered = await tamperedCopy(state.issuedFile)
      const result = await verifyFile(tampered)
      return { hashes: { expected: result.expectedHash, computed: result.computedHash ?? '' } }
    },
  })

  steps.push({
    id: 'not-issued',
    title: 'Verify a document that was never issued',
    blurb: 'A well-formed id the registry has never heard of.',
    mode: canIssueVerify ? 'real' : 'preview',
    run: async (scene) => {
      if (!canIssueVerify) {
        await preview(scene, { kind: 'VERIFY', docId: 'NEVER-ISSUED-0000', verdict: 'NOT_ISSUED' })
        return
      }
      await verifyById('NEVER-ISSUED-0000')
    },
  })

  steps.push({
    id: 'revoke',
    title: `Revoke ${state.docId}`,
    blurb: 'Genuine bytes, withdrawn by the department that issued them.',
    mode: canLifecycle ? 'real' : 'preview',
    previewReason: notAdmin,
    run: async (scene) => {
      if (!canLifecycle) {
        await preview(scene, { kind: 'REVOKE', dept: deptId, docId: state.docId })
        return
      }
      await revokeDocument(state.docHash, deptId, state.docId)
    },
  })

  const canRequestConsent = profile?.role === 'OFFICIAL' || profile?.role === 'ADMIN'
  steps.push({
    id: 'consent',
    // Not interpolated with a docId here: by the time this step displays, the
    // supersede step above may or may not have run yet, and this title is
    // computed once when the script is built — state.correctedDocId is still
    // empty at that point even though it will be set before this step runs.
    // The specific id shows up correctly once the step actually plays, via the
    // floor's caption (which resolves targetDocId at run time, not build time).
    title: "Ask the citizen's consent to check the current document",
    blurb: 'The verifier waits; the citizen approves before any verdict is revealed.',
    mode: canRequestConsent ? 'real' : 'preview',
    previewReason: notSignedIn,
    run: async (scene) => {
      const targetDocId = state.correctedDocId || state.docId
      if (!canRequestConsent) {
        await preview(scene, { kind: 'CONSENT_REQUEST', docId: targetDocId })
        await preview(scene, { kind: 'CONSENT_APPROVED', docId: targetDocId })
        return
      }
      const request = await createVerificationRequest(targetDocId, 'guided demo walkthrough')
      state.verificationRequestId = request.id
      const notification = await getDevelopmentConsentURL(request.id)
      state.consentToken = extractToken(notification.consentUrl)
      await decideConsent(state.consentToken, 'approve', targetDocId)
      await completeVerificationRequest(state.verificationRequestId)
    },
  })

  return steps
}
