import { supabase } from './lib/supabase'

// The one place that knows where the backend lives.
export const API = import.meta.env.VITE_API_URL ?? 'http://localhost:8088'

export type Verdict = 'VALID' | 'SUPERSEDED' | 'REVOKED' | 'TAMPERED' | 'NOT_ISSUED' | 'UNKNOWN'

export type VerifyResult = {
  status: Verdict
  message: string
  docId: string
  expectedHash: string
  computedHash?: string
  docType?: string
  issuer?: string
  citizen?: string
  timestamp?: number
  filename?: string
  originalFilename?: string
  prevHash?: string
  resolvedDocId?: string
  supersededBy?: { docId: string; hash: string }
}

export type Department = {
	id?: string
  slug: string
  name: string
  docType: number
  docTypeName: string
  address: string
}

export type ApplicationUser = {
  id: string
  email: string
  name: string
  role: 'CONTROLLER' | 'ADMIN' | 'OFFICIAL'
  active: boolean
  department: null | {
    id: string
    name: string
    docType: number
    docTypeName: string
  }
}

export type Credential = {
  docHash: string
  docId: string
  docType: string
  citizen: string
  issuer: string
  filename: string
  txHash: string
  issuedAt: string
  sizeBytes: number
  status: Verdict
}

export type IssueResult = {
  docHash: string
  txHash: string
  docId: string
  docType: string
  citizen: string
  issuer: string
  downloadUrl: string
  stamped: boolean
  citizenAccountId: string
}

export type CitizenAccountOption = { id: string; displayName: string; email: string }
export type VerificationRequestState = 'PENDING' | 'APPROVED' | 'DENIED' | 'EXPIRED' | 'COMPLETED'
export type VerificationRequest = {
  id: string
  documentId: string
  requesterUserId: string
  requesterEmail: string
  requesterName: string
  requesterRole: string
  departmentId: string
  departmentName: string
  documentType: string
  state: VerificationRequestState
  purpose: string
  createdAt: string
  expiresAt: string
  decisionAt: string | null
  completedAt: string | null
  decisionChannel: string | null
  decisionReference: string | null
  completedResult: VerifyResult | null
  version: number
  notificationStatus: string
  notificationDestination: string
}
export type ConsentDetails = {
  requestId: string
  state: VerificationRequestState
  requester: { name: string; role: string }
  department: { id: string; name: string }
  documentType: string
  purpose: string
  expiresAt: string
}

export type RevokeResult = {
  docHash: string
  txHash: string
  status: 'REVOKED'
}

export type SupersedeResult = {
  docHash: string
  oldHash: string
  txHash: string
  docId: string
  citizen: string
  downloadUrl: string
  stamped: boolean
}

export type AuditOutcome = 'SUCCESS' | 'FAILURE' | 'DENIED' | 'PARTIAL_FAILURE'
export type AuditAction =
  | 'ISSUE'
  | 'VERIFY_FILE'
  | 'VERIFY_ID'
  | 'REVOKE'
  | 'SUPERSEDE'
  | 'USER_PROFILE_CREATE'
  | 'USER_PROFILE_UPDATE'
  | 'VERIFICATION_REQUEST_CREATED'
  | 'CONSENT_NOTIFICATION'
  | 'CONSENT_APPROVED'
  | 'CONSENT_DENIED'
  | 'VERIFICATION_REQUEST_EXPIRED'
  | 'VERIFICATION_REQUEST_COMPLETED'
  | 'CONSENT_TOKEN_REJECTED'

export type AuditEvent = {
  id: number
  eventId: string
  createdAt: string
  actor: { id: string; email: string; name: string; role: string }
  department: null | { id: string; name: string }
  action: AuditAction
  outcome: AuditOutcome
  result: string
  credential: {
    docId: string | null
    docHash: string | null
    citizen: string | null
    transactionHash: string | null
    referenceHash: string | null
  }
  requestId: string
  httpStatus: number
  error: string | null
  details: Record<string, unknown>
  previousHash: string
  entryHash: string
}

export type OperationCounts = {
  total: number
  success: number
  failure: number
  denied: number
  partialFailure: number
}

export type MonitoringOverview = {
  generatedAt: string
  auditIntegrity: { valid: boolean; eventCount: number; firstInvalidEventId: number | null }
  totals: OperationCounts
  operations: Record<'issue' | 'verify' | 'revoke' | 'supersede', OperationCounts>
  verificationResults: Record<Verdict, number>
  departments: Array<{
    id: string
    name: string
    active: boolean
    events: number
    success: number
    failure: number
    denied: number
    partialFailure: number
    issue: number
    verify: number
    revoke: number
    supersede: number
    lastActivityAt: string | null
  }>
  recentActivity: AuditEvent[]
}

async function json<T>(res: Response): Promise<T> {
  const raw = await res.text()
  let body: unknown = {}
  if (raw) {
    try {
      body = JSON.parse(raw)
    } catch {
      body = raw
    }
  }
  if (!res.ok) {
    const message =
      typeof body === 'string'
        ? body.trim()
        : (body as { error?: string }).error
    throw new Error(message || `${res.status} ${res.statusText}`)
  }
  return body as T
}

async function request<T>(path: string, init: RequestInit = {}, authenticated = true): Promise<T> {
  const headers = new Headers(init.headers)
  if (authenticated) {
    const { data } = await supabase.auth.getSession()
    const token = data.session?.access_token
    if (!token) throw new Error('Authentication required')
    headers.set('Authorization', `Bearer ${token}`)
  }
  return fetch(`${API}${path}`, { ...init, headers }).then(json<T>)
}

export const getHealth = () => request<{ ok: boolean; contract: string }>('/health', {}, false)
export const getDepartments = () => request<Department[]>('/departments', {}, false)
export const getMe = () => request<ApplicationUser>('/me')
export const getCitizens = () => request<string[]>('/citizens')
export const getCitizenAccounts = () => request<CitizenAccountOption[]>('/citizen-accounts')

export const getAuditEvents = (query: {
  limit?: number
  before?: number
  action?: AuditAction
  outcome?: AuditOutcome
  department?: string
  documentId?: string
  actorUserId?: string
} = {}) => {
  const params = new URLSearchParams()
  Object.entries(query).forEach(([key, value]) => {
    if (value !== undefined && value !== '') params.set(key, String(value))
  })
  const suffix = params.size ? `?${params.toString()}` : ''
  return request<{
    events: AuditEvent[]
    page: { limit: number; nextBefore: number | null; hasMore: boolean }
  }>(`/audit-events${suffix}`)
}

export const getMonitoringOverview = () =>
  request<MonitoringOverview>('/monitoring/overview')

export const getCredentials = (citizen: string) =>
  request<{ citizen: string; documents: Credential[] }>(`/credentials/${encodeURIComponent(citizen)}`)
    .then((r) => r.documents)

export const verifyFile = (file: File) => {
  const form = new FormData()
  form.append('file', file)
  return request<VerifyResult>('/verify', { method: 'POST', body: form })
}

export const verifyById = (docId: string) =>
  request<VerifyResult>(`/verify/${encodeURIComponent(docId)}`)

export const issueDocument = (fields: {
  file: File
  dept: string
  docType: string
  docId: string
  citizen: string
  citizenAccountId: string
}) => {
  const form = new FormData()
  form.append('file', fields.file)
  form.append('dept', fields.dept)
  form.append('doc_type', fields.docType)
  form.append('doc_id', fields.docId)
  form.append('citizen', fields.citizen)
  form.append('citizen_account_id', fields.citizenAccountId)
  return request<IssueResult>('/issue', { method: 'POST', body: form })
}

export const createVerificationRequest = (documentId: string, purpose: string) =>
  request<{ id: string; state: VerificationRequestState; expiresAt: string; notification: { channel: string; destination: string; status: string } }>('/verification-requests', {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ documentId, purpose }),
  })

export const getVerificationRequest = (id: string) => request<VerificationRequest>(`/verification-requests/${encodeURIComponent(id)}`)
export const listVerificationRequests = (state?: VerificationRequestState) => request<{ requests: VerificationRequest[]; page: { limit: number; offset: number; hasMore: boolean } }>(`/verification-requests${state ? `?state=${state}` : ''}`)
export const completeVerificationRequest = (id: string) => request<{ request: VerificationRequest; verification: VerifyResult }>(`/verification-requests/${encodeURIComponent(id)}/complete`, { method: 'POST' })
export const getDevelopmentConsentURL = (id: string) => request<{ requestId: string; consentUrl: string; developmentOnly: boolean }>(`/development/notifications/${encodeURIComponent(id)}`)
export const getConsentDetails = (token: string) => request<ConsentDetails>(`/consent/${encodeURIComponent(token)}`, {}, false)
export const decideConsent = (token: string, decision: 'approve' | 'deny') => request<{ id: string; state: VerificationRequestState; decisionAt: string }>(`/consent/${encodeURIComponent(token)}/${decision}`, { method: 'POST' }, false)

export const revokeDocument = (docHash: string, dept: string) =>
  request<RevokeResult>('/revoke', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ docHash, dept }),
  })

export const supersedeDocument = (fields: {
  file: File
  dept: string
  oldHash: string
  docId: string
  citizen: string
  citizenAccountId: string
}) => {
  const form = new FormData()
  form.append('file', fields.file)
  form.append('dept', fields.dept)
  form.append('old_hash', fields.oldHash)
  form.append('doc_id', fields.docId)
  form.append('citizen', fields.citizen)
  form.append('citizen_account_id', fields.citizenAccountId)
  return request<SupersedeResult>('/supersede', { method: 'POST', body: form })
}

export async function downloadDocument(path: string, filename: string) {
  const { data } = await supabase.auth.getSession()
  const token = data.session?.access_token
  if (!token) throw new Error('Authentication required')
  const response = await fetch(`${API}${path}`, {
    headers: { Authorization: `Bearer ${token}` },
  })
  if (!response.ok) {
    const raw = await response.text()
    let message = raw.trim()
    if (raw) {
      try {
        message = (JSON.parse(raw) as { error?: string }).error ?? message
      } catch {
        // Plain-text auth/RBAC errors are already the message to display.
      }
    }
    throw new Error(message || `${response.status} ${response.statusText}`)
  }
  const url = URL.createObjectURL(await response.blob())
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  anchor.click()
  URL.revokeObjectURL(url)
}

export const docTypeLabel = (t?: string) =>
  ({
    birth_certificate: 'Birth certificate',
    driving_licence: 'Driving licence',
    degree_certificate: 'Degree certificate',
  })[t ?? ''] ?? t ?? '—'
