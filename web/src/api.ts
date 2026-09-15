import { defaultMinScore, listPageSize, type APIToken, type AuditEvent, type Case, type CaseKind, type CustomField, type Filters, type ImportResult, type Motif, type PhotoDetail, type RecordItem, type Summary, type User } from './types.ts'

export class ApiError extends Error {
  status: number
  recordId?: string
  code?: string

  constructor(message: string, status: number, recordId?: string, code?: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.recordId = recordId
    this.code = code
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body && !(init.body instanceof FormData) && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  const res = await fetch(path, { ...init, headers, credentials: 'include' })
  if (res.status === 401 && path !== '/api/auth/login' && path !== '/api/auth/me') {
    throw new Error('Nicht angemeldet')
  }
  const text = await res.text()
  const data = (text ? JSON.parse(text) : {}) as { error?: string; record_id?: string; code?: string }
  if (!res.ok) {
    throw new ApiError(data.error || 'Anfrage fehlgeschlagen', res.status, data.record_id, data.code)
  }
  return data as T
}

function filenameFromDisposition(header: string | null, fallback: string) {
  const match = /filename="([^"]+)"/.exec(header ?? '')
  return match?.[1] || fallback
}

function filterParams(filters: Filters) {
  const q = new URLSearchParams()
  if (filters.q) q.set('q', filters.q)
  if (filters.mode === 'semantic' && filters.q) q.set('semantic', 'true')
  if (filters.from) q.set('from', filters.from)
  if (filters.to) q.set('to', filters.to)
  if (filters.hasGps) q.set('has_gps', filters.hasGps)
  if (filters.motifId) q.set('motif_id', filters.motifId)
  if (filters.caseId) q.set('case_id', filters.caseId)
  if (filters.mode === 'semantic' || filters.queryFile) {
    q.set('min_score', String(filters.minScore ?? defaultMinScore))
  }
  if (filters.nearLat != null && filters.nearLon != null) {
    q.set('near_lat', String(filters.nearLat))
    q.set('near_lon', String(filters.nearLon))
    q.set('radius_m', String(filters.radiusM))
  }
  for (const [key, val] of Object.entries(filters.custom ?? {})) {
    if (val.trim()) q.set(`cf.${key}`, val.trim())
  }
  return q
}

export const api = {
  me: () => request<{ user: User }>('/api/auth/me'),
  login: (username: string, password: string) =>
    request<{ user: User }>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  logout: () => request('/api/auth/logout', { method: 'POST' }),
  changePassword: (oldPassword: string, newPassword: string) =>
    request<{ user: User }>('/api/auth/password', {
      method: 'POST',
      body: JSON.stringify({ old_password: oldPassword, new_password: newPassword }),
    }),
  tokens: () => request<{ items: APIToken[] }>('/api/tokens'),
  createToken: (name: string, expiresInDays: number) =>
    request<APIToken>('/api/tokens', {
      method: 'POST',
      body: JSON.stringify({ name, expires_in_days: expiresInDays }),
    }),
  revokeToken: (id: string) => request<{ ok: boolean }>(`/api/tokens/${id}`, { method: 'DELETE' }),
  users: () => request<{ items: User[]; ldap?: boolean }>('/api/users'),
  createUser: (
    username: string,
    password: string,
    role: User['role'],
    canDelete: boolean,
    authProvider: User['auth_provider'] = 'local',
  ) =>
    request<{ user: User }>('/api/users', {
      method: 'POST',
      body: JSON.stringify({
        username,
        password: authProvider === 'ldap' ? undefined : password,
        role,
        can_delete: canDelete,
        auth_provider: authProvider,
      }),
    }),
  patchUser: (id: string, body: { disabled?: boolean; role?: User['role']; can_delete?: boolean }) =>
    request<{ user: User }>(`/api/users/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(body),
    }),
  setUserDisabled: (id: string, disabled: boolean) =>
    request<{ user: User }>(`/api/users/${id}`, {
      method: 'PATCH',
      body: JSON.stringify({ disabled }),
    }),
  resetUserPassword: (id: string, password: string) =>
    request<{ user: User }>(`/api/users/${id}/password`, {
      method: 'POST',
      body: JSON.stringify({ password }),
    }),
  audit: (offset = 0) =>
    request<{ items: AuditEvent[]; total: number }>(`/api/audit?offset=${offset}`),
  records: (filters: Filters, offset = 0, limit = listPageSize) => {
    const q = filterParams(filters)
    q.set('offset', String(offset))
    q.set('limit', String(limit))
    return request<{ items: RecordItem[]; total: number }>(`/api/records?${q}`)
  },
  exportRecords: async (filters: Filters, format: 'csv' | 'json') => {
    const q = filterParams(filters)
    q.set('format', format)
    const init: RequestInit = { credentials: 'include' }
    if (filters.queryFile) {
      const form = new FormData()
      form.append('file', filters.queryFile)
      init.method = 'POST'
      init.body = form
    }
    const res = await fetch(`/api/records/export?${q}`, init)
    if (res.status === 401) {
      throw new Error('Nicht angemeldet')
    }
    if (!res.ok) {
      const text = await res.text()
      let message = 'Export fehlgeschlagen'
      try {
        const data = JSON.parse(text) as { error?: string }
        if (data.error) message = data.error
      } catch {
        if (text.trim()) message = text
      }
      throw new ApiError(message, res.status)
    }
    const blob = await res.blob()
    const total = Number(res.headers.get('X-Export-Total') || '0')
    const count = Number(res.headers.get('X-Export-Count') || '0')
    const truncated = res.headers.get('X-Export-Truncated') === 'true'
    const name = filenameFromDisposition(res.headers.get('Content-Disposition'), `schmutzfink-sichtungen.${format}`)
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = name
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
    return { total, count, truncated }
  },
  summary: (filters: Filters) => {
    const q = filterParams(filters)
    if (filters.queryFile) {
      const form = new FormData()
      form.append('file', filters.queryFile)
      return request<Summary>(`/api/records/summary?${q}`, { method: 'POST', body: form })
    }
    return request<Summary>(`/api/records/summary?${q}`)
  },
  map: (filters: Filters) => {
    const q = filterParams(filters)
    q.delete('has_gps')
    return request<{ items: RecordItem[] }>(`/api/records/map?${q}`)
  },
  searchByImage: (file: File, filters: Filters, opts?: { offset?: number; limit?: number; forMap?: boolean }) => {
    const q = filterParams({ ...filters, q: '', mode: 'text' })
    q.set('min_score', String(filters.minScore ?? defaultMinScore))
    if (opts?.offset != null) q.set('offset', String(opts.offset))
    if (opts?.limit != null) q.set('limit', String(opts.limit))
    if (opts?.forMap) {
      q.set('for_map', 'true')
      q.delete('has_gps')
    }
    const form = new FormData()
    form.append('file', file)
    return request<{ items: RecordItem[]; total?: number }>(`/api/search/image?${q}`, {
      method: 'POST',
      body: form,
    })
  },
  geocode: (q: string) =>
    request<{ lat: number; lon: number; label: string }>(`/api/geocode?q=${encodeURIComponent(q)}`),
  embeddings: () =>
    request<{ state: string; error?: string; pending?: number; ready?: number; failed?: number }>(
      '/api/embeddings',
    ),
  similar: (id: string, minScore = defaultMinScore) =>
    request<{ items: RecordItem[]; status: string }>(
      `/api/sightings/${id}/similar?min_score=${encodeURIComponent(String(minScore))}`,
    ),
  record: (id: string) => request<PhotoDetail>(`/api/records/${id}`),
  upload: (form: FormData) =>
    request<PhotoDetail>('/api/records', { method: 'POST', body: form }),
  importZip: (form: FormData) =>
    request<ImportResult>('/api/records/import', { method: 'POST', body: form }),
  patch: (id: string, body: Record<string, unknown>) =>
    request<PhotoDetail>(`/api/records/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(body),
    }),
  addSighting: (recordId: string, body: Record<string, unknown>) =>
    request<PhotoDetail>(`/api/records/${recordId}/sightings`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  patchSighting: (id: string, body: Record<string, unknown>) =>
    request<PhotoDetail>(`/api/sightings/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(body),
    }),
  deleteSighting: (id: string) =>
    request<PhotoDetail>(`/api/sightings/${id}`, { method: 'DELETE' }),
  requestDeletion: (id: string, reason: string) =>
    request<PhotoDetail>(`/api/records/${id}/deletion-request`, {
      method: 'POST',
      body: JSON.stringify({ reason }),
    }),
  cancelDeletion: (id: string) =>
    request<PhotoDetail>(`/api/records/${id}/deletion-request`, { method: 'DELETE' }),
  deleteRecord: (id: string) =>
    request<{ ok: boolean }>(`/api/records/${id}`, { method: 'DELETE' }),
  deletionRequests: () => request<{ items: PhotoDetail[] }>('/api/deletion-requests'),
  motifs: () => request<{ items: Motif[] }>('/api/motifs'),
  motif: (id: string) => request<Motif>(`/api/motifs/${id}`),
  createMotif: (sightingId: string, title = '', note = '') =>
    request<Motif>('/api/motifs', {
      method: 'POST',
      body: JSON.stringify({ sighting_id: sightingId, title, note }),
    }),
  patchMotif: (id: string, body: { title?: string; note?: string }) =>
    request<Motif>(`/api/motifs/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(body),
    }),
  deleteMotif: (id: string) =>
    request<{ ok: boolean }>(`/api/motifs/${id}`, { method: 'DELETE' }),
  assignMotif: (motifId: string, sightingId: string) =>
    request<Motif>(`/api/motifs/${motifId}/sightings`, {
      method: 'POST',
      body: JSON.stringify({ sighting_id: sightingId }),
    }),
  unlinkMotif: (motifId: string, sightingId: string) =>
    request<Motif | { deleted: true }>(`/api/motifs/${motifId}/sightings/${sightingId}`, {
      method: 'DELETE',
    }),
  motifSuggestions: (id: string, minScore = defaultMinScore) =>
    request<{ items: RecordItem[] }>(
      `/api/motifs/${id}/suggestions?min_score=${encodeURIComponent(String(minScore))}`,
    ),
  cases: () => request<{ items: Case[] }>('/api/cases'),
  case: (id: string) => request<Case>(`/api/cases/${id}`),
  createCase: (body: { title: string; note?: string; kind: CaseKind; record_id?: string }) =>
    request<Case>('/api/cases', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  patchCase: (id: string, body: { title: string; note?: string; kind: CaseKind }) =>
    request<Case>(`/api/cases/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(body),
    }),
  deleteCase: (id: string) =>
    request<{ ok: boolean }>(`/api/cases/${id}`, { method: 'DELETE' }),
  assignCase: (caseId: string, recordId: string) =>
    request<PhotoDetail>(`/api/cases/${caseId}/records`, {
      method: 'POST',
      body: JSON.stringify({ record_id: recordId }),
    }),
  unlinkCase: (caseId: string, recordId: string) =>
    request<PhotoDetail>(`/api/cases/${caseId}/records/${recordId}`, { method: 'DELETE' }),
  closeCase: (id: string) =>
    request<Case>(`/api/cases/${id}/close`, { method: 'POST' }),
  reopenCase: (id: string) =>
    request<Case>(`/api/cases/${id}/reopen`, { method: 'POST' }),
  fields: () => request<{ items: CustomField[] }>('/api/fields'),
  createField: (body: Partial<CustomField> & { label: string; type: CustomField['type'] }) =>
    request<CustomField>('/api/fields', { method: 'POST', body: JSON.stringify(body) }),
  patchField: (id: string, body: Partial<CustomField>) =>
    request<CustomField>(`/api/fields/${id}`, { method: 'PATCH', body: JSON.stringify(body) }),
  deleteField: (id: string) =>
    request<{ ok: boolean }>(`/api/fields/${id}`, { method: 'DELETE' }),
}
