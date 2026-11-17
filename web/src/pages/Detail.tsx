import { type FormEvent, useEffect, useMemo, useState } from 'react'
import { Link, useMatch, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { api } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { formatCoord, formatCoords, formatDay, formatWhen, parseCoord } from '../format.ts'
import {
  canExecuteDeletion,
  canWriteCatalog,
  caseKindLabel,
  casePath,
  formatMinScore,
  motifPath,
  sightingPath,
  type Case,
  type CaseKind,
  type GeoAddress,
  type Motif,
  type PhotoDetail,
  type RecordItem,
  type SightingItem,
} from '../types.ts'
import { LocationPicker } from './LocationPicker.tsx'
import { GraffitiMap, missingSightingMessage, type MapPoint } from './MiniMap.tsx'
import { FieldInputs, useCustomFields } from './FieldInputs.tsx'
import { MinScoreField, useMinScore } from './MinScoreField.tsx'
import { RoiPicker } from './RoiPicker.tsx'
import { Split } from './Split.tsx'

function parseTags(raw: string) {
  return raw
    .split(',')
    .map((t) => t.trim())
    .filter(Boolean)
}

function sightingLabel(s: SightingItem, index: number) {
  if (s.note.trim()) return s.note
  return s.roi ? `Ausschnitt ${index + 1}` : 'Ganzes Bild'
}

function addressLabel(addr?: GeoAddress | null) {
  if (!addr) return ''
  if (addr.label?.trim()) return addr.label.trim()
  const loc =
    addr.postal_code && addr.city
      ? `${addr.postal_code} ${addr.city}`
      : addr.city || addr.postal_code || ''
  if (addr.street && loc) return `${addr.street}, ${loc}`
  return addr.street || loc
}

export function DetailPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { user } = useAuth()
  const editing = Boolean(useMatch('/datensatz/:id/bearbeiten'))
  const [params, setParams] = useSearchParams()
  const selectedParam = params.get('s')
  const [photo, setPhoto] = useState<PhotoDetail | null>(null)
  const [note, setNote] = useState('')
  const [tags, setTags] = useState('')
  const [lat, setLat] = useState('')
  const [lon, setLon] = useState('')
  const [address, setAddress] = useState<GeoAddress | null>(null)
  const [captured, setCaptured] = useState('')
  const [error, setError] = useState('')
  const [saved, setSaved] = useState('')
  const [similar, setSimilar] = useState<RecordItem[]>([])
  const [similarNote, setSimilarNote] = useState('')
  const [similarTick, setSimilarTick] = useState(0)
  const [minScore, setMinScore] = useMinScore()
  const fieldDefs = useCustomFields()
  const [fieldValues, setFieldValues] = useState<Record<string, string>>({})
  const [adding, setAdding] = useState(false)
  const [motifBusy, setMotifBusy] = useState('')
  const [motifs, setMotifs] = useState<Motif[]>([])
  const [cases, setCases] = useState<Case[]>([])
  const [caseBusy, setCaseBusy] = useState('')
  const [newCaseTitle, setNewCaseTitle] = useState('')
  const [newCaseKind, setNewCaseKind] = useState<CaseKind>('civil')

  const selected = useMemo(() => {
    if (!photo?.sightings.length) return null
    return photo.sightings.find((s) => s.id === selectedParam) ?? photo.sightings[0]
  }, [photo, selectedParam])

  function applyPhoto(next: PhotoDetail, preferId?: string) {
    setPhoto(next)
    const pick =
      next.sightings.find((s) => s.id === preferId) ??
      next.sightings.find((s) => s.id === selectedParam) ??
      next.sightings[0]
    setLat(next.lat == null ? '' : String(next.lat))
    setLon(next.lon == null ? '' : String(next.lon))
    setAddress(next.address ?? null)
    setCaptured(next.captured_at ? next.captured_at.slice(0, 16) : '')
    if (pick) {
      setNote(pick.note)
      setTags(pick.tags.join(', '))
      if (pick.id !== selectedParam) {
        setParams({ s: pick.id }, { replace: true })
      }
    }
  }

  function selectSighting(sid: string) {
    setAdding(false)
    setParams({ s: sid }, { replace: true })
  }

  useEffect(() => {
    api
      .cases()
      .then((res) => setCases(res.items))
      .catch(() => setCases([]))
    api
      .motifs()
      .then((res) => setMotifs(res.items))
      .catch(() => setMotifs([]))
  }, [])

  useEffect(() => {
    if (!id) return
    let stopped = false
    api
      .record(id)
      .then((rec) => {
        if (!stopped) applyPhoto(rec)
      })
      .catch((err: unknown) => {
        if (!stopped) setError(missingSightingMessage(err))
      })
    return () => {
      stopped = true
    }
  }, [id])

  useEffect(() => {
    setAdding(false)
    setSaved('')
    setError('')
  }, [editing])

  useEffect(() => {
    if (!selected) return
    setNote(selected.note)
    setTags(selected.tags.join(', '))
    setFieldValues(selected.fields ?? {})
  }, [selected?.id])

  useEffect(() => {
    if (!selected || editing) {
      setSimilar([])
      setSimilarNote('')
      return
    }
    let stopped = false
    let timer = 0
    setSimilar([])
    setSimilarNote('')

    function loadSimilar() {
      api
        .similar(selected!.id, minScore)
        .then((res) => {
          if (stopped) return
          setSimilar(res.items)
          const waiting = res.status === 'pending' || res.status === 'processing'
          setSimilarNote(
            waiting
              ? 'Ähnlichkeit wird noch berechnet …'
              : res.status === 'failed'
                ? 'Ähnlichkeit konnte nicht berechnet werden.'
                : '',
          )
          if (waiting) timer = window.setTimeout(loadSimilar, 4000)
        })
        .catch((err: unknown) => {
          if (stopped) return
          setSimilar([])
          setSimilarNote(err instanceof Error ? err.message : 'Ähnliche Bilder nicht verfügbar')
        })
    }

    loadSimilar()
    return () => {
      stopped = true
      if (timer) window.clearTimeout(timer)
    }
  }, [selected?.id, similarTick, editing, minScore])

  async function onDrawn(roi: { x: number; y: number; w: number; h: number }) {
    if (!editing || !id || !photo || !selected) return
    setError('')
    setSaved('')
    try {
      if (adding) {
        const known = new Set(photo.sightings.map((s) => s.id))
        const next = await api.addSighting(id, { roi, note: '', tags: [] })
        const created = next.sightings.find((s) => !known.has(s.id))
        setAdding(false)
        applyPhoto(next, created?.id)
        setSimilarTick((n) => n + 1)
        setSaved('Sichtung angelegt')
        return
      }
      const next = await api.patchSighting(selected.id, { roi })
      applyPhoto(next, selected.id)
      setSimilarTick((n) => n + 1)
      setSaved('Ausschnitt übernommen')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ausschnitt fehlgeschlagen')
    }
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    if (!editing || !id || !selected || !photo) return
    setError('')
    setSaved('')
    const latN = parseCoord(lat)
    const lonN = parseCoord(lon)
    try {
      if (!photo.location_redacted_at) {
        const body: Record<string, unknown> = {
          clear_gps: latN == null || lonN == null,
          lat: latN,
          lon: lonN,
          captured_at: captured ? new Date(captured).toISOString() : undefined,
        }
        if (latN == null || lonN == null) body.clear_address = true
        else if (address) body.address = address
        await api.patch(id, body)
      } else {
        await api.patch(id, {
          captured_at: captured ? new Date(captured).toISOString() : undefined,
        })
      }
      const next = await api.patchSighting(selected.id, {
        note,
        tags: parseTags(tags),
        fields: fieldValues,
      })
      applyPhoto(next, selected.id)
      setSaved('Gespeichert')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Speichern fehlgeschlagen')
    }
  }

  async function removeSighting() {
    if (!editing || !selected) return
    setError('')
    setSaved('')
    try {
      const next = await api.deleteSighting(selected.id)
      setAdding(false)
      applyPhoto(next)
      setSimilarTick((n) => n + 1)
      setSaved('Sichtung entfernt')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Löschen fehlgeschlagen')
    }
  }

  async function startMotif() {
    if (!selected) return
    setError('')
    setMotifBusy('create')
    try {
      const motif = await api.createMotif(selected.id, selected.note)
      const next = await api.record(photo!.id)
      applyPhoto(next, selected.id)
      const list = await api.motifs()
      setMotifs(list.items)
      setSaved(`Motiv „${motif.title}“ angelegt`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Motiv fehlgeschlagen')
    } finally {
      setMotifBusy('')
    }
  }

  async function assignExistingMotif(motifId: string) {
    if (!selected || !motifId) return
    setError('')
    setMotifBusy('assign')
    try {
      const motif = await api.assignMotif(motifId, selected.id)
      const next = await api.record(photo!.id)
      applyPhoto(next, selected.id)
      const list = await api.motifs()
      setMotifs(list.items)
      setSaved(`Motiv „${motif.title}“ zugeordnet`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Zuordnen fehlgeschlagen')
    } finally {
      setMotifBusy('')
    }
  }

  async function unlinkMotif() {
    if (!selected?.motif_id) return
    setError('')
    setMotifBusy('unlink')
    try {
      await api.unlinkMotif(selected.motif_id, selected.id)
      const next = await api.record(photo!.id)
      applyPhoto(next, selected.id)
      const list = await api.motifs()
      setMotifs(list.items)
      setSaved('Vom Motiv gelöst')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Lösen fehlgeschlagen')
    } finally {
      setMotifBusy('')
    }
  }

  async function assignExistingCase(caseId: string) {
    if (!photo || !caseId) return
    setError('')
    setCaseBusy('assign')
    try {
      applyPhoto(await api.assignCase(caseId, photo.id), selected?.id)
      const list = await api.cases()
      setCases(list.items)
      setSaved('Vorgang zugeordnet')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Zuordnen fehlgeschlagen')
    } finally {
      setCaseBusy('')
    }
  }

  async function createAndAssignCase() {
    if (!photo) return
    setError('')
    setCaseBusy('create')
    try {
      const created = await api.createCase({
        title: newCaseTitle,
        kind: newCaseKind,
        record_id: photo.id,
      })
      setNewCaseTitle('')
      setNewCaseKind('civil')
      applyPhoto(await api.record(photo.id), selected?.id)
      const list = await api.cases()
      setCases(list.items)
      setSaved(`Vorgang „${created.title}“ angelegt`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Vorgang fehlgeschlagen')
    } finally {
      setCaseBusy('')
    }
  }

  async function unlinkCase() {
    if (!photo?.case_id) return
    setError('')
    setCaseBusy('unlink')
    try {
      applyPhoto(await api.unlinkCase(photo.case_id, photo.id), selected?.id)
      setSaved('Vom Vorgang gelöst')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Lösen fehlgeschlagen')
    } finally {
      setCaseBusy('')
    }
  }

  async function assignSimilar(hit: RecordItem) {
    if (!selected?.motif_id) return
    setError('')
    setMotifBusy(hit.id)
    try {
      await api.assignMotif(selected.motif_id, hit.id)
      setSimilar((cur) =>
        cur.map((s) =>
          s.id === hit.id ? { ...s, motif_id: selected.motif_id, motif_title: selected.motif_title || 'Motiv' } : s,
        ),
      )
      setSaved('Sichtung dem Motiv zugeordnet')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Zuordnen fehlgeschlagen')
    } finally {
      setMotifBusy('')
    }
  }

  const mapPoints = useMemo(
    () => (photo ? similarMapPoints(photo, selected, similar) : []),
    [photo, selected, similar],
  )

  if (!photo && !error) return <p className="page-status">Laden …</p>
  if (!photo) return <p className="error">{error}</p>
  const title = selected?.note || 'Foto'
  const writable = canWriteCatalog(user)
  const officer = canExecuteDeletion(user)
  const canRemove = Boolean(selected && (photo.sightings.length > 1 || selected.roi))
  const viewPath = sightingPath(photo.id, selected?.id)
  const editPath = sightingPath(photo.id, selected?.id, { edit: true })
  const photoStage = (
    <div className="photo">
      <RoiPicker
        src={photo.original_url}
        boxes={photo.sightings.map((s) => ({ id: s.id, roi: s.roi ?? null }))}
        selectedId={selected?.id ?? null}
        adding={adding}
        readOnly={!editing}
        onSelect={selectSighting}
        onDrawn={(roi) => void onDrawn(roi)}
      />
      <a href={photo.original_url} target="_blank" rel="noreferrer">
        Original öffnen
      </a>
    </div>
  )

  return (
    <section className={editing ? 'detail workspace' : 'detail detail-view workspace'}>
      <div className="page-head page-head-split">
        <div>
          <h1>{title}</h1>
          <p>
            Hochgeladen {formatWhen(photo.uploaded_at)} von {photo.uploaded_by_name}
            {editing ? ' · Bearbeitung' : ''}
          </p>
        </div>
        {editing ? (
          <div className="page-head-actions">
            {error ? <p className="error">{error}</p> : null}
            {saved ? <p className="ok">{saved}</p> : null}
            <button type="submit" form="detail-edit-form" className="primary">
              Speichern
            </button>
            <Link to={viewPath} className="secondary">
              Zur Ansicht
            </Link>
          </div>
        ) : writable ? (
          <Link to={editPath} className="primary">
            Bearbeiten
          </Link>
        ) : null}
      </div>
      {editing ? (
      <Split id="sf.split.detail.cols" axis="x" defaultPct={56} minA={260} minB={260}>
        {photoStage}
          <form
            id="detail-edit-form"
            className="panel stack form-scroll"
            onSubmit={(e) => void onSubmit(e)}
          >
            <SightingTabs
              photo={photo}
              selectedId={selected?.id}
              onSelect={selectSighting}
            />
            <div className="sighting-actions">
              <button type="button" className={adding ? 'active' : ''} onClick={() => setAdding(true)}>
                Weitere Sichtung
              </button>
              {canRemove ? (
                <button type="button" className="linkish" onClick={() => void removeSighting()}>
                  Diese Sichtung entfernen
                </button>
              ) : null}
            </div>
            <label>
              Notiz
              <textarea value={note} onChange={(e) => setNote(e.target.value)} rows={3} />
            </label>
            <label>
              Tags
              <input value={tags} onChange={(e) => setTags(e.target.value)} />
            </label>
            <FieldInputs defs={fieldDefs} values={fieldValues} onChange={setFieldValues} />
            <MotifPanel
              selected={selected}
              motifs={motifs}
              writable={writable}
              busy={motifBusy}
              onAssign={(id) => void assignExistingMotif(id)}
              onStart={() => void startMotif()}
              onUnlink={() => void unlinkMotif()}
            />
            <fieldset className="gps-block">
              <legend>GPS-Koordinaten</legend>
              {photo.location_redacted_at ? (
                <p className="gps-hint">
                  Standort nach Ablauf der Frist entfernt am {formatDay(photo.location_redacted_at)}.
                  GPS und Adresse (bis zur PLZ) sind weg; das Foto bleibt.
                </p>
              ) : (
                <>
                  <p className="gps-hint">
                    {parseCoord(lat) != null && parseCoord(lon) != null
                      ? 'Marker ziehen oder auf die Karte klicken, um den Ort zu ändern. Ort gilt für das ganze Foto.'
                      : 'Adresse suchen oder auf die Karte klicken, um den Ort zu setzen.'}
                  </p>
                  <LocationPicker
                    lat={parseCoord(lat)}
                    lon={parseCoord(lon)}
                    onChange={(nextLat, nextLon) => {
                      setLat(formatCoord(nextLat))
                      setLon(formatCoord(nextLon))
                    }}
                    onAddress={setAddress}
                  />
                  <div className="split">
                    <label>
                      Breite
                      <input value={lat} onChange={(e) => setLat(e.target.value)} inputMode="decimal" />
                    </label>
                    <label>
                      Länge
                      <input value={lon} onChange={(e) => setLon(e.target.value)} inputMode="decimal" />
                    </label>
                  </div>
                </>
              )}
            </fieldset>
            <label>
              Aufnahmezeit
              <input type="datetime-local" value={captured} onChange={(e) => setCaptured(e.target.value)} />
            </label>
            <CasePanel
              photo={photo}
              cases={cases}
              writable={writable}
              busy={caseBusy}
              newTitle={newCaseTitle}
              newKind={newCaseKind}
              onNewTitle={setNewCaseTitle}
              onNewKind={setNewCaseKind}
              onAssign={assignExistingCase}
              onCreate={() => void createAndAssignCase()}
              onUnlink={() => void unlinkCase()}
            />
            <DeletionPanel
              photo={photo}
              userId={user?.id}
              canWrite={writable}
              canDelete={officer}
              onPhoto={applyPhoto}
              onDeleted={() => navigate('/', { replace: true })}
              onError={setError}
              onSaved={setSaved}
            />
            <Link to="/karte">Zur Karte</Link>
          </form>
      </Split>
      ) : (
      <Split id="sf.split.detail.cols" axis="x" defaultPct={56} minA={260} minB={260}>
        <Split id="sf.split.detail.photo" axis="y" defaultPct={64} minA={180} minB={140}>
          {photoStage}
          <section className="similar">
            <div className="similar-head">
              <h2>Ähnliche Sichtungen</h2>
              <MinScoreField value={minScore} onChange={setMinScore} />
            </div>
            {similarNote ? <p className="muted">{similarNote}</p> : null}
            {similar.length === 0 && !similarNote ? (
              <p className="muted">Keine ähnlichen Treffer über {formatMinScore(minScore)}.</p>
            ) : null}
            <ul className="similar-grid">
              {similar.map((hit) => (
                <li key={hit.id} className="suggest-card">
                  <Link to={sightingPath(hit.record_id, hit.id)} className="similar-card">
                    <img src={hit.thumb_url} alt="" />
                    <span>{hit.note || 'Ohne Notiz'}</span>
                    {hit.score != null ? <em>{Math.round(hit.score * 100)} %</em> : null}
                    {hit.motif_id ? <em>{hit.motif_title || 'Motiv'}</em> : null}
                  </Link>
                  {writable && selected?.motif_id && hit.motif_id !== selected.motif_id ? (
                    <button type="button" disabled={motifBusy === hit.id} onClick={() => void assignSimilar(hit)}>
                      Diesem Motiv zuordnen
                    </button>
                  ) : null}
                  {selected?.motif_id && hit.motif_id === selected.motif_id ? (
                    <p className="muted">bereits im Motiv</p>
                  ) : null}
                </li>
              ))}
            </ul>
          </section>
        </Split>
        <Split id="sf.split.detail.side" axis="y" defaultPct={54} minA={140} minB={160}>
          <div className="panel detail-side">
            <SightingTabs
              photo={photo}
              selectedId={selected?.id}
              onSelect={selectSighting}
            />
            <dl className="meta meta-compact">
              <div>
                <dt>Notiz</dt>
                <dd>{selected?.note.trim() || '—'}</dd>
              </div>
              <div>
                <dt>Tags</dt>
                <dd>{selected?.tags.length ? selected.tags.join(', ') : '—'}</dd>
              </div>
              {fieldDefs.map((def) => (
                <div key={def.key}>
                  <dt>{def.label}</dt>
                  <dd>{selected?.fields?.[def.key]?.trim() || '—'}</dd>
                </div>
              ))}
              <div>
                <dt>Ort</dt>
                <dd>
                  {photo.location_redacted_at
                    ? `entfernt ${formatDay(photo.location_redacted_at)}`
                    : addressLabel(photo.address) || formatCoords(photo.lat, photo.lon)}
                </dd>
              </div>
              {!photo.location_redacted_at && addressLabel(photo.address) ? (
                <div>
                  <dt>Koordinaten</dt>
                  <dd>{formatCoords(photo.lat, photo.lon)}</dd>
                </div>
              ) : null}
              <div>
                <dt>Vorgang</dt>
                <dd>
                  {photo.case_id ? (
                    <Link to={casePath(photo.case_id)}>
                      {photo.case_title || 'Vorgang'}
                      {photo.case_kind ? ` · ${caseKindLabel(photo.case_kind)}` : ''}
                    </Link>
                  ) : (
                    '—'
                  )}
                </dd>
              </div>
              <div>
                <dt>Aufnahmezeit</dt>
                <dd>{formatWhen(photo.captured_at)}</dd>
              </div>
              <div>
                <dt>Motiv</dt>
                <dd>
                  {selected?.motif_id ? (
                    <Link to={motifPath(selected.motif_id)}>{selected.motif_title || 'Motiv'}</Link>
                  ) : (
                    '—'
                  )}
                </dd>
              </div>
            </dl>
            <MotifPanel
              selected={selected}
              motifs={motifs}
              writable={writable}
              busy={motifBusy}
              onAssign={(id) => void assignExistingMotif(id)}
              onStart={() => void startMotif()}
              onUnlink={() => void unlinkMotif()}
            />
            <CasePanel
              photo={photo}
              cases={cases}
              writable={writable}
              busy={caseBusy}
              newTitle={newCaseTitle}
              newKind={newCaseKind}
              onNewTitle={setNewCaseTitle}
              onNewKind={setNewCaseKind}
              onAssign={assignExistingCase}
              onCreate={() => void createAndAssignCase()}
              onUnlink={() => void unlinkCase()}
            />
            <DeletionPanel
              photo={photo}
              userId={user?.id}
              canWrite={writable}
              canDelete={officer}
              onPhoto={applyPhoto}
              onDeleted={() => navigate('/', { replace: true })}
              onError={setError}
              onSaved={setSaved}
            />
            {error ? <p className="error">{error}</p> : null}
            {saved ? <p className="ok">{saved}</p> : null}
          </div>
          <div className="panel detail-map">
            {mapPoints.length ? (
              <>
                <GraffitiMap
                  className="map map-overview"
                  points={mapPoints}
                  focusId={selected?.id}
                  onMissingPoint={(sid) => setSimilar((prev) => prev.filter((i) => i.id !== sid))}
                />
                {mapPoints.some((p) => p.kind === 'similar') ? (
                  <p className="muted map-legend">
                    Blauer Pin: dieser Fundort. Fahnen: ähnliche Sichtungen mit GPS.
                  </p>
                ) : null}
              </>
            ) : (
              <p className="muted">Keine Koordinaten für die Karte.</p>
            )}
            <Link to="/karte">Zur Karte</Link>
          </div>
        </Split>
      </Split>
      )}
    </section>
  )
}

function similarMapPoints(photo: PhotoDetail, selected: SightingItem | null, similar: RecordItem[]): MapPoint[] {
  const points: MapPoint[] = []
  if (selected && photo.lat != null && photo.lon != null) {
    points.push({
      id: selected.id,
      recordId: photo.id,
      lat: photo.lat,
      lon: photo.lon,
      note: selected.note,
      thumb_url: selected.thumb_url || photo.thumb_url,
      kind: 'current',
    })
  }
  for (const hit of similar) {
    if (hit.lat == null || hit.lon == null || hit.id === selected?.id) continue
    points.push({
      id: hit.id,
      recordId: hit.record_id,
      lat: hit.lat,
      lon: hit.lon,
      note: hit.note,
      thumb_url: hit.thumb_url,
      kind: 'similar',
    })
  }
  return points
}

function MotifPanel({
  selected,
  motifs,
  writable,
  busy,
  onAssign,
  onStart,
  onUnlink,
}: {
  selected: SightingItem | null
  motifs: Motif[]
  writable: boolean
  busy: string
  onAssign: (id: string) => void
  onStart: () => void
  onUnlink: () => void
}) {
  if (!selected) return null
  if (selected.motif_id) {
    return (
      <div className="motif-actions">
        <Link to={motifPath(selected.motif_id)} className="secondary">
          {selected.motif_title || 'Motiv'} öffnen
        </Link>
        {writable ? (
          <button type="button" className="linkish" disabled={busy !== ''} onClick={onUnlink}>
            Vom Motiv lösen
          </button>
        ) : null}
      </div>
    )
  }
  if (!writable) return null
  return (
    <div className="stack case-assign">
      <label>
        Vorhandenes Motiv zuordnen
        <select
          defaultValue=""
          disabled={busy !== ''}
          onChange={(e) => {
            const id = e.target.value
            e.target.value = ''
            if (id) onAssign(id)
          }}
        >
          <option value="">Motiv wählen …</option>
          {motifs.map((m) => (
            <option key={m.id} value={m.id}>
              {m.title.trim() || 'Motiv'}
              {m.sighting_count === 1
                ? ' · 1 Sichtung'
                : m.sighting_count
                  ? ` · ${m.sighting_count} Sichtungen`
                  : ''}
            </option>
          ))}
        </select>
      </label>
      <button type="button" disabled={busy !== ''} onClick={onStart}>
        Motiv beginnen
      </button>
    </div>
  )
}

function CasePanel({
  photo,
  cases,
  writable,
  busy,
  newTitle,
  newKind,
  onNewTitle,
  onNewKind,
  onAssign,
  onCreate,
  onUnlink,
}: {
  photo: PhotoDetail
  cases: Case[]
  writable: boolean
  busy: string
  newTitle: string
  newKind: CaseKind
  onNewTitle: (v: string) => void
  onNewKind: (v: CaseKind) => void
  onAssign: (id: string) => void
  onCreate: () => void
  onUnlink: () => void
}) {
  return (
    <div className="case-panel">
      {photo.location_redacted_at ? (
        <div className="retention-banner">
          <strong>Standort entfernt</strong>
          <p>
            GPS und Adresse (bis zur PLZ) wurden am {formatDay(photo.location_redacted_at)} nach
            Ablauf der Frist gelöscht. Das Foto bleibt als Nachweis.
          </p>
        </div>
      ) : photo.case_id && photo.location_due_at ? (
        <p className="muted">
          Vorgang geschlossen {photo.case_closed_at ? formatDay(photo.case_closed_at) : ''}.
          Standortentfernung am {formatDay(photo.location_due_at)} (
          {`${caseKindLabel(photo.case_kind || 'civil')}, ${photo.case_kind === 'criminal' ? 5 : 3} Jahre`}
          ).
        </p>
      ) : photo.case_id ? (
        <p className="muted">
          Vorgang offen — die Standortfrist beginnt erst nach dem Schließen (
          {`${caseKindLabel(photo.case_kind || 'civil')}, ${photo.case_kind === 'criminal' ? 5 : 3} Jahre`}
          ).
        </p>
      ) : writable ? (
        <p className="muted">Kein Vorgang — ohne Zuordnung läuft keine automatische Standortfrist.</p>
      ) : null}
      {photo.case_id ? (
        <div className="motif-actions">
          <Link to={casePath(photo.case_id)} className="secondary">
            {photo.case_title || 'Vorgang'} · {caseKindLabel(photo.case_kind || 'civil')}
          </Link>
          {writable ? (
            <button type="button" className="linkish" disabled={busy !== ''} onClick={onUnlink}>
              Vom Vorgang lösen
            </button>
          ) : null}
        </div>
      ) : writable ? (
        <div className="stack case-assign">
          <label>
            Vorhandenen Vorgang zuordnen
            <select
              defaultValue=""
              disabled={busy !== ''}
              onChange={(e) => {
                const id = e.target.value
                e.target.value = ''
                if (id) onAssign(id)
              }}
            >
              <option value="">Vorgang wählen …</option>
              {cases.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.title.trim() || 'Vorgang'} · {caseKindLabel(c.kind)}
                </option>
              ))}
            </select>
          </label>
          <div className="split">
            <label>
              Neuer Vorgang
              <input
                value={newTitle}
                onChange={(e) => onNewTitle(e.target.value)}
                placeholder="Aktenzeichen oder Kurzname"
              />
            </label>
            <label>
              Art
              <select value={newKind} onChange={(e) => onNewKind(e.target.value as CaseKind)}>
                <option value="civil">Zivilrechtlich (3 Jahre)</option>
                <option value="criminal">Strafrechtlich (5 Jahre)</option>
              </select>
            </label>
          </div>
          <button type="button" disabled={busy !== '' || !newTitle.trim()} onClick={onCreate}>
            Vorgang anlegen und zuordnen
          </button>
        </div>
      ) : null}
    </div>
  )
}

function DeletionPanel({
  photo,
  userId,
  canWrite,
  canDelete,
  onPhoto,
  onDeleted,
  onError,
  onSaved,
}: {
  photo: PhotoDetail
  userId?: string
  canWrite?: boolean
  canDelete?: boolean
  onPhoto: (next: PhotoDetail) => void
  onDeleted: () => void
  onError: (msg: string) => void
  onSaved: (msg: string) => void
}) {
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  const req = photo.deletion_request
  const ownRequest = Boolean(req && req.requested_by === userId)

  async function requestDelete() {
    setBusy(true)
    onError('')
    onSaved('')
    try {
      onPhoto(await api.requestDeletion(photo.id, reason))
      setReason('')
      onSaved('Löschung beantragt. Das Foto bleibt, bis ein Löschbeauftragter entscheidet.')
    } catch (err) {
      onError(err instanceof Error ? err.message : 'Antrag fehlgeschlagen')
    } finally {
      setBusy(false)
    }
  }

  async function cancel() {
    setBusy(true)
    onError('')
    onSaved('')
    try {
      onPhoto(await api.cancelDeletion(photo.id))
      onSaved(canDelete && !ownRequest ? 'Antrag abgelehnt.' : 'Antrag zurückgezogen.')
    } catch (err) {
      onError(err instanceof Error ? err.message : 'Zurückziehen fehlgeschlagen')
    } finally {
      setBusy(false)
    }
  }

  async function remove() {
    if (
      !window.confirm(
        'Foto und alle Sichtungen unwiderruflich löschen? Das kann nicht rückgängig gemacht werden.',
      )
    ) {
      return
    }
    setBusy(true)
    onError('')
    onSaved('')
    try {
      await api.deleteRecord(photo.id)
      onDeleted()
    } catch (err) {
      onError(err instanceof Error ? err.message : 'Löschen fehlgeschlagen')
      setBusy(false)
    }
  }

  if (!canWrite && !req) return null

  return (
    <div className="deletion-panel">
      {req ? (
        <div className="deletion-banner">
          <strong>Löschung beantragt</strong>
          <p>
            {req.requested_by_name} am {formatWhen(req.requested_at)}
            {req.reason.trim() ? ` · ${req.reason}` : ''}
          </p>
          {canWrite ? (
            <div className="user-actions">
              {ownRequest || canDelete ? (
                <button type="button" className="secondary" disabled={busy} onClick={() => void cancel()}>
                  {canDelete && !ownRequest ? 'Antrag ablehnen' : 'Antrag zurückziehen'}
                </button>
              ) : null}
              {canDelete ? (
                <button
                  type="button"
                  className="danger"
                  disabled={busy || ownRequest}
                  title={ownRequest ? 'Eigene Anträge dürfen nicht selbst ausgeführt werden' : undefined}
                  onClick={() => void remove()}
                >
                  Foto löschen
                </button>
              ) : null}
            </div>
          ) : null}
        </div>
      ) : (
        <>
          <p className="muted">
            Fotos bleiben als möglicher Nachweis erhalten. Sachbearbeiter beantragen die Löschung,
            nur Löschbeauftragte löschen wirklich.
          </p>
          {canWrite ? (
            <div className="deletion-request">
              <label>
                Grund (optional)
                <textarea
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                  rows={2}
                  maxLength={500}
                />
              </label>
              <button type="button" className="secondary" disabled={busy} onClick={() => void requestDelete()}>
                Löschung beantragen
              </button>
            </div>
          ) : null}
          {canDelete ? (
            <button type="button" className="danger" disabled={busy} onClick={() => void remove()}>
              Foto löschen
            </button>
          ) : null}
        </>
      )}
    </div>
  )
}

function SightingTabs({
  photo,
  selectedId,
  onSelect,
}: {
  photo: PhotoDetail
  selectedId?: string
  onSelect: (id: string) => void
}) {
  return (
    <div className="sighting-list">
      <p className="sighting-label">Sichtungen</p>
      <ul>
        {photo.sightings.map((s, i) => (
          <li key={s.id}>
            <button
              type="button"
              className={s.id === selectedId ? 'active' : ''}
              onClick={() => onSelect(s.id)}
            >
              {sightingLabel(s, i)}
            </button>
          </li>
        ))}
      </ul>
    </div>
  )
}
