import { useEffect, useRef, useState } from 'react'
import { api } from '../api.ts'
import { formatCoords, formatRadius } from '../format.ts'
import { hasRadiusFilter, radiusPresets, type Case, type Filters, type Motif } from '../types.ts'
import { useCustomFields } from './FieldInputs.tsx'
import { MinScoreField } from './MinScoreField.tsx'
import { GraffitiMap } from './MiniMap.tsx'

type MapPoint = {
  id: string
  recordId?: string
  lat: number
  lon: number
  note?: string
  thumb_url?: string
}

type Props = {
  value: Filters
  onChange: (next: Filters) => void
  hideGps?: boolean
  pickOnMap?: boolean
  points?: MapPoint[]
}

export function FilterBar({ value, onChange, hideGps, pickOnMap, points = [] }: Props) {
  const [place, setPlace] = useState('')
  const [searching, setSearching] = useState(false)
  const [searchError, setSearchError] = useState('')
  const [showPicker, setShowPicker] = useState(false)
  const valueRef = useRef(value)
  valueRef.current = value
  const patch = (partial: Partial<Filters>) => onChange({ ...valueRef.current, ...partial })
  const customDefs = useCustomFields()
  const [motifs, setMotifs] = useState<Motif[]>([])
  const [cases, setCases] = useState<Case[]>([])
  const [fileKey, setFileKey] = useState(0)
  const [preview, setPreview] = useState('')
  const active = hasRadiusFilter(value)

  useEffect(() => {
    api
      .motifs()
      .then((res) => setMotifs(res.items))
      .catch(() => setMotifs([]))
    api
      .cases()
      .then((res) => setCases(res.items))
      .catch(() => setCases([]))
  }, [])

  useEffect(() => {
    if (!value.queryFile) {
      setPreview('')
      return
    }
    const url = URL.createObjectURL(value.queryFile)
    setPreview(url)
    return () => URL.revokeObjectURL(url)
  }, [value.queryFile])
  const searchArea =
    value.nearLat != null && value.nearLon != null
      ? { lat: value.nearLat, lon: value.nearLon, radiusM: value.radiusM }
      : null

  async function searchPlace() {
    const q = place.trim()
    if (!q) {
      setSearchError('Bitte einen Ort eingeben')
      return
    }
    setSearching(true)
    setSearchError('')
    try {
      const hit = await api.geocode(q)
      patch({ nearLat: hit.lat, nearLon: hit.lon })
      setPlace(hit.label)
    } catch (err) {
      setSearchError(err instanceof Error ? err.message : 'Ort nicht gefunden')
    } finally {
      setSearching(false)
    }
  }

  function clearRadius() {
    setPlace('')
    setSearchError('')
    patch({ nearLat: null, nearLon: null })
  }

  return (
    <div className="filters-wrap">
      <div className="filters">
        <label className="filter-mode">
          Suche in
          <select
            value={value.mode}
            onChange={(e) => {
              const mode = e.target.value as Filters['mode']
              patch({ mode, queryFile: mode === 'semantic' ? value.queryFile : null })
              if (mode !== 'semantic') setFileKey((k) => k + 1)
            }}
          >
            <option value="text">Text (Notiz, Tags, Felder, Motiv, Vorgang)</option>
            <option value="semantic">Bildinhalt</option>
          </select>
        </label>
        <label className="filter-search">
          Suche
          <input
            value={value.q}
            placeholder={
              value.mode === 'semantic' ? 'z. B. rotes Tag an einer Wand' : 'Notiz, Tag, Feld, Motiv oder Vorgang'
            }
            onChange={(e) => patch({ q: e.target.value })}
          />
        </label>
        {value.mode === 'semantic' ? (
          <div className="query-image-field">
            <span>Suchbild</span>
            <div className="query-image">
              {preview ? <img src={preview} alt="" className="query-image-preview" /> : null}
              <input
                key={fileKey}
                type="file"
                accept="image/jpeg,image/png,image/webp,image/gif"
                onChange={(e) => {
                  const file = e.target.files?.[0] ?? null
                  patch({ queryFile: file, mode: 'semantic' })
                }}
              />
              {value.queryFile ? (
                <button
                  type="button"
                  className="secondary"
                  onClick={() => {
                    setFileKey((k) => k + 1)
                    patch({ queryFile: null })
                  }}
                >
                  Bild entfernen
                </button>
              ) : null}
            </div>
            <span className="muted query-image-hint">
              Nur zum Vergleichen, nicht im Bestand gespeichert.
            </span>
          </div>
        ) : null}
        <MinScoreField value={value.minScore} onChange={(minScore) => patch({ minScore })} />
        <div className="filter-dates">
          <label>
            Von
            <input type="date" value={value.from} onChange={(e) => patch({ from: e.target.value })} />
          </label>
          <label>
            Bis
            <input type="date" value={value.to} onChange={(e) => patch({ to: e.target.value })} />
          </label>
        </div>
        {hideGps ? null : (
          <label>
            GPS
            <select
              value={value.hasGps}
              onChange={(e) => patch({ hasGps: e.target.value as Filters['hasGps'] })}
            >
              <option value="">alle</option>
              <option value="true">mit Koordinaten</option>
              <option value="false">ohne Koordinaten</option>
            </select>
          </label>
        )}
        <label>
          Motiv
          <select value={value.motifId} onChange={(e) => patch({ motifId: e.target.value })}>
            <option value="">alle</option>
            <option value="none">ohne Motiv</option>
            {motifs.map((m) => (
              <option key={m.id} value={m.id}>
                {m.title.trim() || 'Motiv'}
              </option>
            ))}
          </select>
        </label>
        <label>
          Vorgang
          <select value={value.caseId} onChange={(e) => patch({ caseId: e.target.value })}>
            <option value="">alle</option>
            <option value="none">ohne Vorgang</option>
            {cases.map((c) => (
              <option key={c.id} value={c.id}>
                {c.title.trim() || 'Vorgang'}
              </option>
            ))}
          </select>
        </label>
        {value.mode === 'text'
          ? customDefs.map((d) => (
              <label key={d.id}>
                {d.label}
                {d.type === 'select' ? (
                  <select
                    value={value.custom[d.key] ?? ''}
                    onChange={(e) => patch({ custom: { ...value.custom, [d.key]: e.target.value } })}
                  >
                    <option value="">alle</option>
                    {d.options.map((opt) => (
                      <option key={opt} value={opt}>
                        {opt}
                      </option>
                    ))}
                  </select>
                ) : d.type === 'bool' ? (
                  <select
                    value={value.custom[d.key] ?? ''}
                    onChange={(e) => patch({ custom: { ...value.custom, [d.key]: e.target.value } })}
                  >
                    <option value="">alle</option>
                    <option value="ja">ja</option>
                    <option value="nein">nein</option>
                  </select>
                ) : (
                  <input
                    type={d.type === 'date' ? 'date' : d.type === 'number' ? 'number' : 'text'}
                    value={value.custom[d.key] ?? ''}
                    onChange={(e) => patch({ custom: { ...value.custom, [d.key]: e.target.value } })}
                  />
                )}
              </label>
            ))
          : null}
        <label>
          Umkreis
          <select
            value={value.radiusM}
            onChange={(e) => patch({ radiusM: Number(e.target.value) })}
          >
            {radiusPresets.map((opt) => (
              <option key={opt.m} value={opt.m}>
                {opt.label}
              </option>
            ))}
          </select>
        </label>
      </div>
      <div className="radius-filter">
        <div className="geo-search">
          <input
            value={place}
            onChange={(e) => setPlace(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault()
                void searchPlace()
              }
            }}
            placeholder="Ort für Umkreis, z. B. Hannover, Lister Meile"
            autoComplete="off"
          />
          <button type="button" className="secondary" disabled={searching} onClick={() => void searchPlace()}>
            {searching ? 'Suche …' : 'Ort suchen'}
          </button>
          {pickOnMap ? null : (
            <button
              type="button"
              className="secondary"
              onClick={() => setShowPicker((open) => !open)}
              aria-expanded={showPicker}
            >
              {showPicker ? 'Karte ausblenden' : 'Auf Karte wählen'}
            </button>
          )}
          {active ? (
            <button type="button" className="secondary" onClick={clearRadius}>
              Umkreis aufheben
            </button>
          ) : null}
        </div>
        {searchError ? <p className="error">{searchError}</p> : null}
        {active ? (
          <p className="radius-hint">
            Suche im Umkreis von {formatRadius(value.radiusM)} um {formatCoords(value.nearLat, value.nearLon)}.
          </p>
        ) : (
          <p className="radius-hint">
            {pickOnMap
              ? 'Auf die Karte klicken, um den Mittelpunkt zu setzen. Anschließend den Umkreis anpassen.'
              : 'Ort suchen oder auf der Karte einen Punkt wählen, dann den Umkreis festlegen.'}
          </p>
        )}
        {pickOnMap || !showPicker ? null : (
          <GraffitiMap
            className="map map-picker radius-picker-map"
            popups={false}
            points={points}
            searchArea={searchArea}
            onPickCenter={(lat, lon) => patch({ nearLat: lat, nearLon: lon })}
          />
        )}
      </div>
    </div>
  )
}
