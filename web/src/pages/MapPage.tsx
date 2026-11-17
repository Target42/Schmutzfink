import { useEffect, useState } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { ApiError, api } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { distanceMeters, formatRadius } from '../format.ts'
import { hasRadiusFilter, type RecordItem } from '../types.ts'
import { ExportButtons } from './ExportButtons.tsx'
import { FilterBar, useSessionFilters } from './FilterBar.tsx'
import { GraffitiMap } from './MiniMap.tsx'
import { Split } from './Split.tsx'

export function MapPage() {
  const { user, ready } = useAuth()
  if (!ready) return <div className="page-status">Laden …</div>
  if (!user) return <PublicMap />
  return <SignedInMap />
}

function PublicMap() {
  const [items, setItems] = useState<{ id: string; lat: number; lon: number; thumb_url: string }[]>([])
  const [error, setError] = useState('')
  const [denied, setDenied] = useState(false)

  useEffect(() => {
    api
      .publicMap()
      .then((res) => {
        setItems(res.items ?? [])
        setError('')
      })
      .catch((err: unknown) => {
        if (err instanceof ApiError && err.status === 404) {
          setDenied(true)
          return
        }
        setError(err instanceof Error ? err.message : 'Fehler')
      })
  }, [])

  if (denied) return <Navigate to="/login" replace />

  const points = items.map((i) => ({
    id: i.id,
    lat: i.lat,
    lon: i.lon,
    thumb_url: i.thumb_url,
  }))

  return (
    <section className="workspace map-page">
      <div className="page-head">
        <h1>Karte</h1>
        <p>
          {points.length} Punkt{points.length === 1 ? '' : 'e'} mit Koordinaten. Ohne Anmeldung nur die
          kleinen Vorschaubilder.
        </p>
      </div>
      {error ? <p className="error">{error}</p> : null}
      <GraffitiMap className="map map-fill" points={points} thumbOnly />
    </section>
  )
}

function SignedInMap() {
  const location = useLocation()
  const focusId = (location.state as { focusId?: string } | null)?.focusId
  const [filters, setFilters] = useSessionFilters()
  const [items, setItems] = useState<RecordItem[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    const handle = window.setTimeout(() => {
      const req = filters.queryFile
        ? api.searchByImage(filters.queryFile, { ...filters, nearLat: null, nearLon: null }, { forMap: true })
        : api.map({ ...filters, nearLat: null, nearLon: null })
      req
        .then((res) => {
          setItems(res.items)
          setError('')
        })
        .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Fehler'))
    }, 200)
    return () => window.clearTimeout(handle)
  }, [
    filters.q,
    filters.mode,
    filters.from,
    filters.to,
    filters.queryFile,
    filters.custom,
    filters.motifId,
    filters.caseId,
    filters.edited,
    filters.minScore,
  ])

  const points = items
    .filter((i) => i.lat != null && i.lon != null)
    .map((i) => ({
      id: i.id,
      recordId: i.record_id,
      lat: i.lat as number,
      lon: i.lon as number,
      note: i.note,
      thumb_url: i.thumb_url,
    }))
  const searchArea =
    filters.nearLat != null && filters.nearLon != null
      ? { lat: filters.nearLat, lon: filters.nearLon, radiusM: filters.radiusM }
      : null
  const hitCount = searchArea
    ? points.filter((p) => distanceMeters(p.lat, p.lon, searchArea.lat, searchArea.lon) <= searchArea.radiusM).length
    : points.length
  const around = hasRadiusFilter(filters) ? ` im Umkreis von ${formatRadius(filters.radiusM)}` : ''

  return (
    <section className="workspace map-page">
      <div className="page-head page-head-split">
        <div>
          <h1>Karte</h1>
          <p>
            {hitCount} Punkt{hitCount === 1 ? '' : 'e'} mit Koordinaten{around}
          </p>
        </div>
        <ExportButtons filters={filters} gpsOnly />
      </div>
      <Split id="sf.split.map.main" axis="y" defaultPct={32} minA={120} minB={220}>
        <div className="filters-pane form-scroll">
          <FilterBar value={filters} onChange={setFilters} hideGps pickOnMap />
        </div>
        <div className="map-pane">
          {error ? <p className="error">{error}</p> : null}
          <GraffitiMap
            className="map map-fill"
            points={points}
            focusId={focusId}
            searchArea={searchArea}
            onPickCenter={(lat, lon) => setFilters((prev) => ({ ...prev, nearLat: lat, nearLon: lon }))}
            onMissingPoint={(id) => setItems((prev) => prev.filter((i) => i.id !== id))}
          />
        </div>
      </Split>
    </section>
  )
}
