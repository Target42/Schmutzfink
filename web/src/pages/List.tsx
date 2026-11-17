import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api.ts'
import { formatCoords, formatRadius, formatWhen } from '../format.ts'
import { useAuth } from '../auth.tsx'
import {
  canWriteCatalog,
  caseKindLabel,
  formatMinScore,
  hasRadiusFilter,
  listPageSizeFor,
  sightingPath,
  type Filters,
  type RecordItem,
  type TileSize,
} from '../types.ts'
import { ExportButtons } from './ExportButtons.tsx'
import { FilterBar, useSessionFilters } from './FilterBar.tsx'
import { formatFieldValues, useCustomFields } from './FieldInputs.tsx'
import { Split } from './Split.tsx'
import { TileSizeControl, useTileSize } from './TileSizeControl.tsx'

function mapPins(items: RecordItem[]) {
  return items
    .filter((i) => i.lat != null && i.lon != null)
    .map((i) => ({
      id: i.id,
      recordId: i.record_id,
      lat: i.lat as number,
      lon: i.lon as number,
      note: i.note,
      thumb_url: i.thumb_url,
    }))
}

function noteLabel(item: RecordItem) {
  const note = item.note?.trim()
  return note || 'Ohne Notiz'
}

export function ListPage() {
  const { user } = useAuth()
  const fieldDefs = useCustomFields()
  const [filters, setFilters] = useSessionFilters()
  const [page, setPage] = useState(0)
  const [items, setItems] = useState<RecordItem[]>([])
  const [pins, setPins] = useState<ReturnType<typeof mapPins>>([])
  const [total, setTotal] = useState(0)
  const [error, setError] = useState('')
  const [tileSize, setTileSize] = useTileSize()
  const fetchNow = useRef(false)
  const listPaneRef = useRef<HTMLDivElement>(null)
  const pageSize = listPageSizeFor(tileSize)

  function changeFilters(next: Filters) {
    setFilters(next)
    setPage(0)
  }

  function changeTileSize(next: TileSize) {
    setTileSize(next)
    setPage(0)
    fetchNow.current = true
  }

  useEffect(() => {
    const delay = fetchNow.current ? 0 : 200
    fetchNow.current = false
    const handle = window.setTimeout(() => {
      const req = filters.queryFile
        ? api.searchByImage(filters.queryFile, filters, { offset: page * pageSize, limit: pageSize })
        : api.records(filters, page * pageSize, pageSize)
      req
        .then((res) => {
          setItems(res.items)
          setTotal(res.total ?? res.items.length)
          setError('')
        })
        .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Fehler'))
    }, delay)
    return () => window.clearTimeout(handle)
  }, [filters, page, pageSize])

  useEffect(() => {
    const handle = window.setTimeout(() => {
      const req = filters.queryFile
        ? api.searchByImage(filters.queryFile, { ...filters, nearLat: null, nearLon: null }, { forMap: true })
        : api.map({ ...filters, nearLat: null, nearLon: null })
      req
        .then((res) => setPins(mapPins(res.items)))
        .catch(() => setPins([]))
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
    filters.hasGps,
    filters.minScore,
  ])

  const pageCount = Math.max(1, Math.ceil(total / pageSize))
  const from = total === 0 ? 0 : page * pageSize + 1
  const to = Math.min(total, (page + 1) * pageSize)
  const around = hasRadiusFilter(filters) ? ` im Umkreis von ${formatRadius(filters.radiusM)}` : ''
  const countLabel =
    total === 1
      ? '1 Foto'
      : pageCount > 1
        ? `${from}–${to} von ${total} Fotos`
        : `${total} Fotos`
  const compactTiles = tileSize === 's' || tileSize === 'xs'

  function goToPage(next: number) {
    fetchNow.current = true
    setPage(next)
    listPaneRef.current?.scrollTo({ top: 0, behavior: 'smooth' })
  }

  return (
    <section className="workspace">
      <div className="page-head page-head-split">
        <div>
          <h1>Bestand</h1>
          <p>
            {countLabel}
            {around}
          </p>
        </div>
        <div className="page-head-actions">
          <TileSizeControl value={tileSize} onChange={changeTileSize} />
          <ExportButtons filters={filters} />
        </div>
      </div>
      <Split id="sf.split.list.main" axis="y" defaultPct={36} minA={120} minB={220}>
        <div className="filters-pane form-scroll">
          <FilterBar value={filters} onChange={changeFilters} points={pins} />
        </div>
        <div className="list-results">
          <div className="list-results-body form-scroll" ref={listPaneRef}>
            {error ? <p className="error">{error}</p> : null}
            {items.length === 0 && !error ? (
              <p className="empty">
                {filters.mode === 'semantic'
                  ? `Keine Treffer über ${formatMinScore(filters.minScore)}.`
                  : canWriteCatalog(user)
                    ? 'Noch keine Treffer. Laden Sie ein Foto hoch.'
                    : 'Noch keine Treffer.'}
              </p>
            ) : null}
            <ul className="record-list" data-tile-size={tileSize}>
              {items.map((item) => {
                const label = noteLabel(item)
                const hasNote = Boolean(item.note?.trim())
                const imageOnly = tileSize === 'xs' || (compactTiles && !hasNote)
                return (
                  <li key={item.record_id}>
                    <Link
                      to={sightingPath(item.record_id)}
                      className={imageOnly ? 'record-row record-row-image' : 'record-row'}
                      title={label}
                    >
                      <img src={`/api/records/${item.record_id}/thumb`} alt={label} />
                      {imageOnly ? null : (
                        <div>
                          <strong title={label}>{hasNote ? label : 'Ohne Notiz'}</strong>
                          <p>
                            {formatWhen(item.captured_at || item.uploaded_at)} · {formatCoords(item.lat, item.lon)}
                          </p>
                          <p className="muted">
                            {item.tags.join(', ') || 'keine Tags'} · {item.uploaded_by_name}
                            {item.score != null ? ` · ${Math.round(item.score * 100)} % ähnlich` : ''}
                            {item.motif_id ? (
                              <>
                                {' · '}
                                <span className="motif-chip">{item.motif_title || 'Motiv'}</span>
                              </>
                            ) : null}
                            {item.case_id ? (
                              <>
                                {' · '}
                                <span className="case-chip">
                                  {item.case_title || 'Vorgang'}
                                  {item.case_kind ? ` · ${caseKindLabel(item.case_kind)}` : ''}
                                </span>
                              </>
                            ) : null}
                            {item.location_redacted_at ? ' · Standort entfernt' : ''}
                            {formatFieldValues(fieldDefs, item.fields).map((line) => (
                              <span key={line}> · {line}</span>
                            ))}
                          </p>
                        </div>
                      )}
                    </Link>
                  </li>
                )
              })}
            </ul>
          </div>
          {pageCount > 1 ? (
            <nav className="pager" aria-label="Seiten">
              <button type="button" className="secondary" disabled={page <= 0} onClick={() => goToPage(page - 1)}>
                Zurück
              </button>
              <p>
                Seite {page + 1} von {pageCount}
              </p>
              <button type="button" className="secondary" disabled={page + 1 >= pageCount} onClick={() => goToPage(page + 1)}>
                Weiter
              </button>
            </nav>
          ) : null}
        </div>
      </Split>
    </section>
  )
}
