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
}) => {
  const form = new FormData()
  form.append('file', fields.file)
  form.append('dept', fields.dept)
  form.append('doc_type', fields.docType)
  form.append('doc_id', fields.docId)
  form.append('citizen', fields.citizen)
  return request<IssueResult>('/issue', { method: 'POST', body: form })
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
