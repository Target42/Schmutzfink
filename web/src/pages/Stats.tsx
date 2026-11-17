import { useEffect, useState } from 'react'
import { api } from '../api.ts'
import { formatCoords } from '../format.ts'
import { hasRadiusFilter, type NamedCount, type Summary } from '../types.ts'
import { ExportButtons } from './ExportButtons.tsx'
import { FilterBar, useSessionFilters } from './FilterBar.tsx'
import { GraffitiMap } from './MiniMap.tsx'

function monthLabel(key: string) {
  const [y, m] = key.split('-').map(Number)
  if (!y || !m) return key
  return new Date(y, m - 1, 1).toLocaleDateString('de-DE', { month: 'short', year: 'numeric' })
}

function CountList({ items, empty }: { items: NamedCount[]; empty: string }) {
  if (!items.length) return <p className="muted">{empty}</p>
  const max = Math.max(...items.map((i) => i.count), 1)
  return (
    <ul className="stat-bars">
      {items.map((item) => (
        <li key={item.key || item.label}>
          <span className="stat-bar-label" title={item.label}>
            {item.label}
          </span>
          <span className="stat-bar-track">
            <span className="stat-bar-fill" style={{ width: `${(item.count / max) * 100}%` }} />
          </span>
          <span className="stat-bar-n">{item.count}</span>
        </li>
      ))}
    </ul>
  )
}

export function StatsPage() {
  const [filters, setFilters] = useSessionFilters()
  const [data, setData] = useState<Summary | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    const handle = window.setTimeout(() => {
      api
        .summary(filters)
        .then((res) => {
          setData(res)
          setError('')
        })
        .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Fehler'))
    }, 200)
    return () => window.clearTimeout(handle)
  }, [filters])

  const around = hasRadiusFilter(filters) ? ' im gewählten Umkreis' : ''
  const points =
    data?.hotspots.map((h, i) => ({
      id: `h-${i}`,
      lat: h.lat,
      lon: h.lon,
      note: `${h.count} Sichtung${h.count === 1 ? '' : 'en'} · ${formatCoords(h.lat, h.lon)}`,
    })) ?? []

  return (
    <section className="stats-page">
      <div className="page-head page-head-split">
        <div>
          <h1>Auswertungen</h1>
          <p>
            Zählungen zu den aktuellen Filtern — ohne Bilder. Geeignet für Hotspots, Serien und
            Zeitverläufe.
            {around}
          </p>
        </div>
        <ExportButtons filters={filters} />
      </div>
      <FilterBar value={filters} onChange={setFilters} points={points} />
      {error ? <p className="error">{error}</p> : null}
      {!data && !error ? <p className="page-status">Laden …</p> : null}
      {data ? (
        <>
          {data.truncated ? (
            <p className="muted">
              Nur die ersten {data.sightings} von {data.total} Treffern sind einbezogen.
            </p>
          ) : null}
          <dl className="stat-cards">
            <div>
              <dt>Sichtungen</dt>
              <dd>{data.sightings}</dd>
            </div>
            <div>
              <dt>Fotos</dt>
              <dd>{data.photos}</dd>
            </div>
            <div>
              <dt>mit GPS</dt>
              <dd>{data.with_gps}</dd>
            </div>
            <div>
              <dt>ohne GPS</dt>
              <dd>{data.without_gps}</dd>
            </div>
            <div>
              <dt>mit Motiv</dt>
              <dd>{data.with_motif}</dd>
            </div>
          </dl>
          <div className="stats-grid">
            <div className="panel stack">
              <h2>Zeitverlauf</h2>
              <CountList
                items={data.months.map((m) => ({ ...m, label: monthLabel(m.key) }))}
                empty="Keine Sichtungen im Filter."
              />
            </div>
            <div className="panel stack">
              <h2>Hotspots</h2>
              {points.length ? (
                <>
                  <GraffitiMap className="map map-overview" points={points} popups />
                  <p className="muted">Raster rund 100 m. Die {points.length} häufigsten Zellen.</p>
                </>
              ) : (
                <p className="muted">Keine Koordinaten in der Treffermenge.</p>
              )}
            </div>
            <div className="panel stack">
              <h2>Tags</h2>
              <CountList items={data.tags} empty="Keine Tags." />
            </div>
            <div className="panel stack">
              <h2>Motive</h2>
              <CountList items={data.motifs} empty="Keine Motive." />
            </div>
            {data.fields.map((field) => (
              <div className="panel stack" key={field.key}>
                <h2>{field.label}</h2>
                <CountList items={field.values} empty="Keine Werte." />
              </div>
            ))}
          </div>
        </>
      ) : null}
    </section>
  )
}
