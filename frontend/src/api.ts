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
  slug: string
  name: string
  docType: number
  docTypeName: string
  address: string
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
  const body = await res.json().catch(() => ({}))
  if (!res.ok) throw new Error((body as { error?: string }).error ?? `${res.status} ${res.statusText}`)
  return body as T
}

export const getDepartments = () => fetch(`${API}/departments`).then(json<Department[]>)
export const getCitizens = () => fetch(`${API}/citizens`).then(json<string[]>)

export const getCredentials = (citizen: string) =>
  fetch(`${API}/credentials/${encodeURIComponent(citizen)}`)
    .then(json<{ citizen: string; documents: Credential[] }>)
    .then((r) => r.documents)

export const verifyFile = (file: File) => {
  const form = new FormData()
  form.append('file', file)
  return fetch(`${API}/verify`, { method: 'POST', body: form }).then(json<VerifyResult>)
}

export const verifyById = (docId: string) =>
  fetch(`${API}/verify/${encodeURIComponent(docId)}`).then(json<VerifyResult>)

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
  return fetch(`${API}/issue`, { method: 'POST', body: form }).then(json<IssueResult>)
}

export const docTypeLabel = (t?: string) =>
  ({
    birth_certificate: 'Birth certificate',
    driving_licence: 'Driving licence',
    degree_certificate: 'Degree certificate',
  })[t ?? ''] ?? t ?? '—'
