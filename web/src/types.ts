export type AuditEvent = {
  id: string
  at: string
  username: string
  action: string
  subject: string
  ip: string
  detail?: Record<string, unknown>
}

export type Role = 'admin' | 'user' | 'searcher'

export type User = {
  id: string
  username: string
  role: Role
  can_delete: boolean
  must_change_password: boolean
  disabled: boolean
  auth_provider?: 'local' | 'ldap'
  api_token?: boolean
}

export type APIToken = {
  id: string
  name: string
  prefix: string
  expires_at: string
  last_used_at: string | null
  created_at: string
  expired: boolean
  token?: string
}

export function isAdmin(user?: User | null) {
  return user?.role === 'admin'
}

export function canWriteCatalog(user?: User | null) {
  return Boolean(user && user.role !== 'searcher')
}

export function canExecuteDeletion(user?: User | null) {
  return Boolean(user?.can_delete && user.role !== 'searcher')
}

export function isDirectoryUser(user?: User | null) {
  return user?.auth_provider === 'ldap'
}

export function roleLabel(role: Role | string) {
  switch (role) {
    case 'admin':
      return 'Admin'
    case 'searcher':
      return 'Sucher'
    default:
      return 'Sachbearbeiter'
  }
}

export type Roi = {
  x: number
  y: number
  w: number
  h: number
}

export type CustomFieldType = 'text' | 'number' | 'date' | 'bool' | 'select'

export type CustomField = {
  id: string
  key: string
  label: string
  type: CustomFieldType
  options: string[]
  required: boolean
  sort_order: number
}

export type SightingItem = {
  id: string
  record_id: string
  note: string
  tags: string[]
  roi?: Roi | null
  embedding_status?: 'pending' | 'processing' | 'ready' | 'failed'
  thumb_url: string
  motif_id?: string
  motif_title?: string
  fields?: Record<string, string>
}

export type CaseKind = 'civil' | 'criminal'

export type RecordItem = {
  id: string
  record_id: string
  note: string
  tags: string[]
  lat: number | null
  lon: number | null
  captured_at: string | null
  uploaded_at: string
  uploaded_by: string
  uploaded_by_name: string
  content_type: string
  has_gps: boolean
  thumb_url: string
  original_url: string
  roi?: Roi | null
  embedding_status?: 'pending' | 'processing' | 'ready' | 'failed'
  score?: number
  motif_id?: string
  motif_title?: string
  case_id?: string
  case_title?: string
  case_kind?: CaseKind
  case_closed_at?: string | null
  location_redacted_at?: string | null
  location_due_at?: string | null
  fields?: Record<string, string>
}

export type Motif = {
  id: string
  title: string
  note: string
  created_at: string
  updated_at: string
  sighting_count: number
  first_at: string | null
  last_at: string | null
  thumb_url: string
  sightings?: RecordItem[]
}

export type DeletionRequest = {
  requested_at: string
  requested_by: string
  requested_by_name: string
  reason: string
}

export type GeoAddress = {
  street?: string
  city?: string
  postal_code?: string
  country?: string
  label?: string
}

export type PhotoDetail = {
  id: string
  lat: number | null
  lon: number | null
  captured_at: string | null
  uploaded_at: string
  uploaded_by: string
  uploaded_by_name: string
  content_type: string
  has_gps: boolean
  thumb_url: string
  original_url: string
  sightings: SightingItem[]
  deletion_request?: DeletionRequest | null
  case_id?: string
  case_title?: string
  case_kind?: CaseKind
  case_closed_at?: string | null
  location_redacted_at?: string | null
  location_due_at?: string | null
  address?: GeoAddress | null
}

export type Case = {
  id: string
  title: string
  note: string
  kind: CaseKind
  retention_years: number
  created_at: string
  updated_at: string
  closed_at: string | null
  closed_by?: string
  closed_by_name?: string
  photo_count: number
  redacted_count: number
  first_at: string | null
  last_at: string | null
  due_at: string | null
  thumb_url: string
  photos?: RecordItem[]
}

export type ImportIssue = {
  filename: string
  error: string
}

export type ImportResult = {
  imported: number
  skipped: number
  errors: ImportIssue[]
  items: PhotoDetail[]
}

export type NamedCount = {
  key: string
  label: string
  count: number
}

export type SummaryHotspot = {
  lat: number
  lon: number
  count: number
}

export type FieldCounts = {
  key: string
  label: string
  values: NamedCount[]
}

export type Summary = {
  sightings: number
  photos: number
  with_gps: number
  without_gps: number
  with_motif: number
  total: number
  truncated: boolean
  months: NamedCount[]
  tags: NamedCount[]
  motifs: NamedCount[]
  fields: FieldCounts[]
  hotspots: SummaryHotspot[]
}

export type SearchMode = 'text' | 'semantic'

export type Filters = {
  q: string
  mode: SearchMode
  queryFile: File | null
  from: string
  to: string
  hasGps: '' | 'true' | 'false'
  edited: '' | 'true' | 'false'
  nearLat: number | null
  nearLon: number | null
  radiusM: number
  minScore: number
  motifId: string
  caseId: string
  custom: Record<string, string>
}

export const radiusPresets = [
  { m: 50, label: '50 m' },
  { m: 100, label: '100 m' },
  { m: 250, label: '250 m' },
  { m: 500, label: '500 m' },
  { m: 1000, label: '1 km' },
  { m: 2000, label: '2 km' },
  { m: 5000, label: '5 km' },
  { m: 10000, label: '10 km' },
] as const

export const defaultRadiusM = 500

export const defaultMinScore = 0.5

export const minScorePresets = [
  { score: 0.3, label: '30 % — breit' },
  { score: 0.5, label: '50 % — Standard' },
  { score: 0.7, label: '70 % — streng' },
  { score: 0.85, label: '85 % — sehr streng' },
] as const

const minScoreStorageKey = 'sf.minScore'

export function loadMinScore(): number {
  try {
    const n = Number(localStorage.getItem(minScoreStorageKey))
    if (minScorePresets.some((p) => p.score === n)) return n
  } catch {
    /* private mode */
  }
  return defaultMinScore
}

export function saveMinScore(score: number) {
  try {
    localStorage.setItem(minScoreStorageKey, String(score))
  } catch {
    /* private mode */
  }
}

export function formatMinScore(score: number) {
  return `${Math.round(score * 100)}\u00a0%`
}

export const tileSizes = ['l', 'm', 's', 'xs'] as const
export type TileSize = (typeof tileSizes)[number]

export const tileSizeLabels: Record<TileSize, string> = {
  l: 'Groß',
  m: 'Mittel',
  s: 'Klein',
  xs: 'Mini',
}

export const defaultTileSize: TileSize = 'l'

const tileSizeStorageKey = 'sf.tileSize'

export function loadTileSize(): TileSize {
  try {
    const v = localStorage.getItem(tileSizeStorageKey)
    if (v && (tileSizes as readonly string[]).includes(v)) return v as TileSize
  } catch {
    /* private mode */
  }
  return defaultTileSize
}

export function saveTileSize(size: TileSize) {
  try {
    localStorage.setItem(tileSizeStorageKey, size)
  } catch {
    /* private mode */
  }
}

export const listPageSize = 12

/** More tiles per page when the grid is denser, so rows fill better. */
export const listPageSizeByTile: Record<TileSize, number> = {
  l: 12,
  m: 24,
  s: 36,
  xs: 48,
}

export function listPageSizeFor(size: TileSize) {
  return listPageSizeByTile[size]
}

export const emptyFilters = (): Filters => ({
  q: '',
  mode: 'text',
  queryFile: null,
  from: '',
  to: '',
  hasGps: '',
  edited: '',
  nearLat: null,
  nearLon: null,
  radiusM: defaultRadiusM,
  minScore: loadMinScore(),
  motifId: '',
  caseId: '',
  custom: {},
})

const sessionFiltersKey = 'sf.filters.session'

function isStringRecord(v: unknown): v is Record<string, string> {
  if (!v || typeof v !== 'object' || Array.isArray(v)) return false
  return Object.values(v).every((x) => typeof x === 'string')
}

function parseSessionFilters(raw: string, base: Filters): Filters {
  const data = JSON.parse(raw) as Record<string, unknown>
  const hasGps = data.hasGps === 'true' || data.hasGps === 'false' ? data.hasGps : ''
  const edited = data.edited === 'true' || data.edited === 'false' ? data.edited : ''
  const nearLat = typeof data.nearLat === 'number' ? data.nearLat : null
  const nearLon = typeof data.nearLon === 'number' ? data.nearLon : null
  const radiusM =
    typeof data.radiusM === 'number' && data.radiusM > 0 ? data.radiusM : base.radiusM
  const minScore =
    typeof data.minScore === 'number' && minScorePresets.some((p) => p.score === data.minScore)
      ? data.minScore
      : base.minScore
  return {
    ...base,
    q: typeof data.q === 'string' ? data.q : base.q,
    mode: data.mode === 'semantic' ? 'semantic' : 'text',
    queryFile: null,
    from: typeof data.from === 'string' ? data.from : base.from,
    to: typeof data.to === 'string' ? data.to : base.to,
    hasGps,
    edited,
    nearLat,
    nearLon,
    radiusM,
    minScore,
    motifId: typeof data.motifId === 'string' ? data.motifId : '',
    caseId: typeof data.caseId === 'string' ? data.caseId : '',
    custom: isStringRecord(data.custom) ? data.custom : {},
  }
}

/** Clear tab filter state (e.g. on logout / login). */
export function clearSessionFilters() {
  try {
    sessionStorage.removeItem(sessionFiltersKey)
    const toRemove: string[] = []
    for (let i = 0; i < sessionStorage.length; i++) {
      const key = sessionStorage.key(i)
      // Also drop briefly used per-user keys from the interim fix.
      if (key?.startsWith('sf.filters.session.')) toRemove.push(key)
    }
    for (const key of toRemove) sessionStorage.removeItem(key)
  } catch {
    /* private mode */
  }
}

/** Restore filters for this browser tab. Image file is never persisted. */
export function loadSessionFilters(): Filters {
  const base = emptyFilters()
  try {
    const raw = sessionStorage.getItem(sessionFiltersKey)
    if (!raw) return base
    return parseSessionFilters(raw, base)
  } catch {
    return base
  }
}

export function saveSessionFilters(f: Filters) {
  try {
    const { queryFile: _file, ...rest } = f
    sessionStorage.setItem(sessionFiltersKey, JSON.stringify(rest))
  } catch {
    /* private mode */
  }
}

export function hasRadiusFilter(f: Filters) {
  return f.nearLat != null && f.nearLon != null
}

/** Extra filters beyond the always-visible search field (for the collapse badge). */
export function countExtraFilters(f: Filters, opts?: { hideGps?: boolean }) {
  let n = 0
  if (f.mode !== 'text') n++
  if (f.queryFile) n++
  if (f.from) n++
  if (f.to) n++
  if (!opts?.hideGps && f.hasGps) n++
  if (f.edited) n++
  if (f.motifId) n++
  if (f.caseId) n++
  if (hasRadiusFilter(f)) n++
  if (f.minScore !== defaultMinScore) n++
  for (const v of Object.values(f.custom)) {
    if (v) n++
  }
  return n
}

export function motifPath(id: string) {
  return `/motiv/${id}`
}

export function casePath(id: string) {
  return `/vorgang/${id}`
}

export function caseKindLabel(kind: CaseKind | string) {
  return kind === 'criminal' ? 'Strafrechtlich' : 'Zivilrechtlich'
}

export function retentionYears(kind: CaseKind | string) {
  return kind === 'criminal' ? 5 : 3
}

export function sightingPath(recordId: string, sightingId?: string, opts?: { edit?: boolean }) {
  const path = opts?.edit ? `/datensatz/${recordId}/bearbeiten` : `/datensatz/${recordId}`
  return sightingId ? `${path}?s=${encodeURIComponent(sightingId)}` : path
}
