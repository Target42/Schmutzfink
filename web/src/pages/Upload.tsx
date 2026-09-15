import { type DragEvent, type FormEvent, useEffect, useId, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import exifr from 'exifr'
import { ApiError, api } from '../api.ts'
import { formatCoord, parseCoord } from '../format.ts'
import { caseKindLabel, type Case, type ImportResult, type Roi } from '../types.ts'
import { FieldInputs, useCustomFields } from './FieldInputs.tsx'
import { LocationPicker } from './LocationPicker.tsx'
import { RoiPicker } from './RoiPicker.tsx'
import { Split } from './Split.tsx'

type GpsSource = 'exif' | 'map' | 'search' | 'edit' | null

type QueueItem = {
  id: string
  file: File
}

const imageAccept = 'image/jpeg,image/png,image/webp,image/gif'

function isImageFile(file: File) {
  if (file.type.startsWith('image/')) return true
  return /\.(jpe?g|png|webp|gif)$/i.test(file.name)
}

function isZipFile(file: File) {
  return file.type === 'application/zip' || file.type === 'application/x-zip-compressed' || /\.zip$/i.test(file.name)
}

function filesFromList(list: FileList | File[] | null | undefined) {
  const images: File[] = []
  const zips: File[] = []
  if (!list) return { images, zips }
  for (const file of Array.from(list)) {
    if (isZipFile(file)) zips.push(file)
    else if (isImageFile(file)) images.push(file)
  }
  return { images, zips }
}

export function UploadPage() {
  const navigate = useNavigate()
  const fileInputId = useId()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const zipInputRef = useRef<HTMLInputElement>(null)
  const [queue, setQueue] = useState<QueueItem[]>([])
  const [current, setCurrent] = useState<QueueItem | null>(null)
  const [preview, setPreview] = useState('')
  const [lat, setLat] = useState('')
  const [lon, setLon] = useState('')
  const [gpsSource, setGpsSource] = useState<GpsSource>(null)
  const [gpsCleared, setGpsCleared] = useState(false)
  const [note, setNote] = useState('')
  const [tags, setTags] = useState('')
  const [error, setError] = useState('')
  const [duplicateId, setDuplicateId] = useState('')
  const [busy, setBusy] = useState(false)
  const [roi, setRoi] = useState<Roi | null>(null)
  const [dragOver, setDragOver] = useState(false)
  const [zipDragOver, setZipDragOver] = useState(false)
  const [zipFile, setZipFile] = useState<File | null>(null)
  const [zipBusy, setZipBusy] = useState(false)
  const [zipError, setZipError] = useState('')
  const [zipResult, setZipResult] = useState<ImportResult | null>(null)
  const fieldDefs = useCustomFields()
  const [fieldValues, setFieldValues] = useState<Record<string, string>>({})
  const [cases, setCases] = useState<Case[]>([])
  const [caseId, setCaseId] = useState('')
  const [savedCount, setSavedCount] = useState(0)

  useEffect(() => {
    api
      .cases()
      .then((res) => setCases(res.items))
      .catch(() => setCases([]))
  }, [])

  useEffect(() => {
    return () => {
      if (preview) URL.revokeObjectURL(preview)
    }
  }, [preview])

  async function loadItem(item: QueueItem | null) {
    setCurrent(item)
    setPreview((prev) => {
      if (prev) URL.revokeObjectURL(prev)
      return item ? URL.createObjectURL(item.file) : ''
    })
    setLat('')
    setLon('')
    setGpsSource(null)
    setGpsCleared(false)
    setRoi(null)
    setNote('')
    setTags('')
    setFieldValues({})
    setError('')
    setDuplicateId('')
    if (fileInputRef.current) fileInputRef.current.value = ''
    if (!item) return
    try {
      const gps = await exifr.gps(item.file)
      if (gps?.latitude != null && gps?.longitude != null) {
        setLat(formatCoord(gps.latitude))
        setLon(formatCoord(gps.longitude))
        setGpsSource('exif')
      }
    } catch {
      /* kein oder unlesbares EXIF */
    }
  }

  function enqueueImages(files: File[], replace = false) {
    if (!files.length) return
    const items = files.map((file) => ({
      id: `${file.name}-${file.size}-${file.lastModified}-${Math.random().toString(36).slice(2, 8)}`,
      file,
    }))
    if (replace || !current) {
      setQueue(items.slice(1))
      setSavedCount(0)
      void loadItem(items[0])
      return
    }
    setQueue((prev) => [...prev, ...items])
  }

  function takeZip(file: File) {
    setZipFile(file)
    setZipError('')
    setZipResult(null)
    if (zipInputRef.current) zipInputRef.current.value = ''
  }

  function acceptFiles(list: FileList | File[] | null | undefined, replaceImages = false) {
    const { images, zips } = filesFromList(list)
    if (images.length) enqueueImages(images, replaceImages)
    if (zips[0]) takeZip(zips[0])
    if (!images.length && !zips.length && list && Array.from(list).length) {
      setError('Nur JPEG, PNG, WebP, GIF oder ZIP')
    }
  }

  function setFromMap(nextLat: number, nextLon: number, via: 'map' | 'search' = 'map') {
    setLat(formatCoord(nextLat))
    setLon(formatCoord(nextLon))
    setGpsSource(via)
    setGpsCleared(false)
  }

  function clearGps() {
    setLat('')
    setLon('')
    setGpsSource(null)
    setGpsCleared(true)
  }

  function skipCurrent() {
    const [next, ...rest] = queue
    setQueue(rest)
    void loadItem(next ?? null)
  }

  function clearQueue() {
    setQueue([])
    setSavedCount(0)
    void loadItem(null)
  }

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!current) {
      setError('Bitte ein Bild wählen oder ablegen')
      return
    }
    const data = new FormData()
    data.set('file', current.file)
    data.set('note', note)
    data.set('tags', tags)
    const latN = parseCoord(lat)
    const lonN = parseCoord(lon)
    if ((latN == null) !== (lonN == null)) {
      setError('Bitte Breite und Länge angeben oder beide Felder leeren')
      return
    }
    if (latN != null && lonN != null) {
      data.set('lat', String(latN))
      data.set('lon', String(lonN))
    }
    if (gpsCleared) data.set('clear_gps', 'true')
    if (roi) data.set('roi', JSON.stringify(roi))
    if (Object.keys(fieldValues).length) data.set('fields', JSON.stringify(fieldValues))
    if (caseId) data.set('case_id', caseId)
    setBusy(true)
    setError('')
    setDuplicateId('')
    try {
      const rec = await api.upload(data)
      const focusId = rec.sightings[0]?.id
      const [next, ...rest] = queue
      if (next) {
        setSavedCount((n) => n + 1)
        setQueue(rest)
        void loadItem(next)
      } else {
        navigate('/karte', { replace: true, state: { focusId } })
      }
    } catch (err) {
      if (err instanceof ApiError && err.recordId) {
        setDuplicateId(err.recordId)
      }
      setError(err instanceof Error ? err.message : 'Upload fehlgeschlagen')
    } finally {
      setBusy(false)
    }
  }

  async function onZipSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!zipFile) {
      setZipError('Bitte eine ZIP-Datei wählen oder ablegen')
      return
    }
    const data = new FormData()
    data.set('file', zipFile)
    setZipBusy(true)
    setZipError('')
    setZipResult(null)
    try {
      const result = await api.importZip(data)
      setZipResult(result)
      setZipFile(null)
      if (zipInputRef.current) zipInputRef.current.value = ''
    } catch (err) {
      setZipError(err instanceof Error ? err.message : 'Import fehlgeschlagen')
    } finally {
      setZipBusy(false)
    }
  }

  function onDragOver(e: DragEvent, kind: 'image' | 'zip') {
    e.preventDefault()
    e.stopPropagation()
    if (kind === 'image') setDragOver(true)
    else setZipDragOver(true)
  }

  function onDragLeave(e: DragEvent, kind: 'image' | 'zip') {
    e.preventDefault()
    e.stopPropagation()
    const related = e.relatedTarget as Node | null
    if (related && e.currentTarget.contains(related)) return
    if (kind === 'image') setDragOver(false)
    else setZipDragOver(false)
  }

  function onDropImages(e: DragEvent) {
    e.preventDefault()
    e.stopPropagation()
    setDragOver(false)
    acceptFiles(e.dataTransfer.files, !current)
  }

  function onDropZip(e: DragEvent) {
    e.preventDefault()
    e.stopPropagation()
    setZipDragOver(false)
    const { zips, images } = filesFromList(e.dataTransfer.files)
    if (zips[0]) takeZip(zips[0])
    else if (images.length) {
      setZipError('Für den ZIP-Import bitte eine .zip-Datei ablegen')
    } else {
      setZipError('Bitte eine ZIP-Datei ablegen')
    }
  }

  const latN = parseCoord(lat)
  const lonN = parseCoord(lon)
  const hasGps = latN != null && lonN != null
  const pending = (current ? 1 : 0) + queue.length
  const batchTotal = savedCount + pending

  return (
    <section className="workspace">
      <div className="page-head">
        <h1>Hochladen</h1>
        <p>Bilder per Drag & Drop oder Dateiwahl — einzeln oder mehrere nacheinander. ZIP mit index.json wie unter Beispiele/import.</p>
      </div>
      <Split id="sf.split.upload.main" axis="y" defaultPct={78} minA={280} minB={140}>
      <form className="workspace-form" onSubmit={(e) => void onSubmit(e)}>
        <Split id="sf.split.upload.cols" axis="x" defaultPct={56} minA={240} minB={260}>
          <div
            className={`photo photo-upload${dragOver ? ' is-drop-target' : ''}`}
            onDragEnter={(e) => onDragOver(e, 'image')}
            onDragOver={(e) => onDragOver(e, 'image')}
            onDragLeave={(e) => onDragLeave(e, 'image')}
            onDrop={onDropImages}
          >
            <div className="upload-file-row">
              <label htmlFor={fileInputId}>
                Bild
                <input
                  id={fileInputId}
                  ref={fileInputRef}
                  type="file"
                  accept={imageAccept}
                  multiple
                  onChange={(e) => {
                    acceptFiles(e.target.files, !current)
                    e.target.value = ''
                  }}
                />
              </label>
              {current ? (
                <p className="muted upload-queue-hint">
                  {current.file.name}
                  {batchTotal > 1 ? ` · ${savedCount + 1} von ${batchTotal}` : ''}
                  {queue.length ? ` · ${queue.length} in der Warteschlange` : ''}
                </p>
              ) : (
                <p className="muted upload-queue-hint">Ziehen Sie Bilder hierher oder wählen Sie Dateien.</p>
              )}
            </div>
            {queue.length || current ? (
              <div className="upload-queue-actions">
                {queue.length ? (
                  <button type="button" className="linkish" onClick={skipCurrent} disabled={busy}>
                    Dieses überspringen
                  </button>
                ) : null}
                <button type="button" className="linkish" onClick={clearQueue} disabled={busy}>
                  Auswahl leeren
                </button>
              </div>
            ) : null}
            {preview ? (
              <RoiPicker src={preview} value={roi} onChange={setRoi} alt="Vorschau" />
            ) : (
              <label htmlFor={fileInputId} className={`photo-empty drop-zone${dragOver ? ' is-drop-target' : ''}`}>
                Bilder hier ablegen oder klicken zum Auswählen
              </label>
            )}
          </div>
          <div className="panel stack form-scroll">
            <fieldset className="gps-block">
              <legend>GPS-Koordinaten</legend>
              <p className="gps-hint">{gpsHint(gpsSource, hasGps)}</p>
              <LocationPicker lat={latN} lon={lonN} onChange={setFromMap} />
              <div className="split">
                <label>
                  Breite
                  <input
                    name="lat"
                    value={lat}
                    onChange={(e) => {
                      setLat(e.target.value)
                      setGpsSource('edit')
                      setGpsCleared(false)
                    }}
                    inputMode="decimal"
                    placeholder="z. B. 52.52"
                    autoComplete="off"
                  />
                </label>
                <label>
                  Länge
                  <input
                    name="lon"
                    value={lon}
                    onChange={(e) => {
                      setLon(e.target.value)
                      setGpsSource('edit')
                      setGpsCleared(false)
                    }}
                    inputMode="decimal"
                    placeholder="z. B. 13.40"
                    autoComplete="off"
                  />
                </label>
              </div>
              {hasGps || gpsCleared ? (
                <button type="button" className="linkish gps-clear" onClick={clearGps}>
                  Koordinaten entfernen
                </button>
              ) : null}
            </fieldset>
            <label>
              Notiz
              <textarea
                name="note"
                rows={3}
                placeholder="Ort, Motiv, Hinweise"
                value={note}
                onChange={(e) => setNote(e.target.value)}
              />
            </label>
            <label>
              Tags
              <input
                name="tags"
                placeholder="Brücke, Schriftzug, … (kommagetrennt)"
                value={tags}
                onChange={(e) => setTags(e.target.value)}
              />
            </label>
            <FieldInputs defs={fieldDefs} values={fieldValues} onChange={setFieldValues} />
            <label>
              Vorgang
              <select value={caseId} onChange={(e) => setCaseId(e.target.value)}>
                <option value="">keiner — später zuordnen</option>
                {cases.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.title.trim() || 'Vorgang'} · {caseKindLabel(c.kind)}
                  </option>
                ))}
              </select>
            </label>
            {error ? (
              <p className="error">
                {error}
                {duplicateId ? (
                  <>
                    {' '}
                    <Link to={`/datensatz/${duplicateId}`}>Zum vorhandenen Datensatz</Link>
                    {queue.length ? (
                      <>
                        {' · '}
                        <button type="button" className="linkish" onClick={skipCurrent}>
                          Trotzdem nächstes Bild
                        </button>
                      </>
                    ) : null}
                  </>
                ) : null}
              </p>
            ) : null}
            <button type="submit" disabled={busy || !current}>
              {busy
                ? 'Speichern …'
                : queue.length
                  ? `Speichern und weiter (${queue.length} offen)`
                  : 'Speichern'}
            </button>
          </div>
        </Split>
      </form>
      <form
        className={`panel stack import-panel form-scroll${zipDragOver ? ' is-drop-target' : ''}`}
        onDragEnter={(e) => onDragOver(e, 'zip')}
        onDragOver={(e) => onDragOver(e, 'zip')}
        onDragLeave={(e) => onDragLeave(e, 'zip')}
        onDrop={onDropZip}
        onSubmit={(e) => void onZipSubmit(e)}
      >
        <h2>ZIP importieren</h2>
        <p className="muted">
          Archiv mit Bildern und <code>index.json</code> (Dateiname, GPS, Adresse, Notiz). Bereits vorhandene Dateien
          werden anhand ihres Hash übersprungen. ZIP hier ablegen oder wählen.
        </p>
        <label>
          ZIP-Datei
          <input
            ref={zipInputRef}
            type="file"
            name="file"
            accept=".zip,application/zip"
            onChange={(e) => {
              const file = e.target.files?.[0]
              if (file) takeZip(file)
              else setZipFile(null)
            }}
          />
        </label>
        {zipFile ? <p className="muted">{zipFile.name}</p> : null}
        {zipError ? <p className="error">{zipError}</p> : null}
        {zipResult ? (
          <div className="import-result">
            <p className="ok">
              {zipResult.imported} übernommen, {zipResult.skipped} bereits vorhanden
              {zipResult.errors.length ? `, ${zipResult.errors.length} Hinweise` : ''}
            </p>
            {zipResult.errors.length ? (
              <ul className="plain-list">
                {zipResult.errors.map((item) => (
                  <li key={`${item.filename}-${item.error}`}>
                    {item.filename ? `${item.filename}: ` : ''}
                    {item.error}
                  </li>
                ))}
              </ul>
            ) : null}
            {zipResult.imported > 0 ? (
              <Link className="secondary" to="/">
                Zum Bestand
              </Link>
            ) : null}
          </div>
        ) : null}
        <button type="submit" disabled={zipBusy || !zipFile}>
          {zipBusy ? 'Importiere …' : 'Importieren'}
        </button>
      </form>
      </Split>
    </section>
  )
}

function gpsHint(source: GpsSource, hasGps: boolean) {
  if (source === 'exif') return 'Koordinaten aus dem Bild gelesen. Du kannst sie auf der Karte anpassen.'
  if (source === 'search') return 'Adresse gefunden. Marker auf die genaue Stelle ziehen oder die Karte klicken.'
  if (hasGps) return 'Ort gesetzt. Marker ziehen oder erneut auf die Karte klicken.'
  return 'Adresse suchen oder auf die Karte klicken, um den Ort zu setzen.'
}
