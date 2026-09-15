import { useEffect, useState } from 'react'
import { useLocation } from 'react-router-dom'
import { api } from '../api.ts'
import { distanceMeters, formatRadius } from '../format.ts'
import { emptyFilters, hasRadiusFilter, type RecordItem } from '../types.ts'
import { ExportButtons } from './ExportButtons.tsx'
import { FilterBar } from './FilterBar.tsx'
import { GraffitiMap } from './MiniMap.tsx'
import { Split } from './Split.tsx'

export function MapPage() {
  const location = useLocation()
  const focusId = (location.state as { focusId?: string } | null)?.focusId
  const [filters, setFilters] = useState(emptyFilters)
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
  }, [filters.q, filters.mode, filters.from, filters.to, filters.queryFile, filters.custom, filters.motifId, filters.minScore])

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
          />
        </div>
      </Split>
    </section>
  )
}
