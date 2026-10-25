import { type FormEvent, useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api } from '../api.ts'
import { formatCoords, formatDay, formatWhen } from '../format.ts'
import { useAuth } from '../auth.tsx'
import { canWriteCatalog, caseKindLabel, sightingPath, type Case, type CaseKind } from '../types.ts'
import { GraffitiMap } from './MiniMap.tsx'
import { Split } from './Split.tsx'

export function CasePage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { user } = useAuth()
  const writable = canWriteCatalog(user)
  const [item, setItem] = useState<Case | null>(null)
  const [title, setTitle] = useState('')
  const [note, setNote] = useState('')
  const [kind, setKind] = useState<CaseKind>('civil')
  const [error, setError] = useState('')
  const [saved, setSaved] = useState('')
  const [busy, setBusy] = useState('')

  function apply(next: Case) {
    setItem(next)
    setTitle(next.title)
    setNote(next.note)
    setKind(next.kind)
  }

  useEffect(() => {
    if (!id) return
    api
      .case(id)
      .then((c) => {
        apply(c)
        setError('')
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Fehler'))
  }, [id])

  const points = useMemo(
    () =>
      (item?.photos ?? [])
        .filter((s) => s.lat != null && s.lon != null)
        .map((s) => ({
          id: s.id,
          recordId: s.record_id,
          lat: s.lat as number,
          lon: s.lon as number,
          note: s.note,
          thumb_url: s.thumb_url,
        })),
    [item],
  )

  async function onSave(e: FormEvent) {
    e.preventDefault()
    if (!id) return
    setError('')
    setSaved('')
    try {
      apply(await api.patchCase(id, { title, note, kind }))
      setSaved('Gespeichert')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Speichern fehlgeschlagen')
    }
  }

  async function unlink(recordId: string) {
    if (!id) return
    setBusy(recordId)
    setError('')
    try {
      await api.unlinkCase(id, recordId)
      apply(await api.case(id))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Lösen fehlgeschlagen')
    } finally {
      setBusy('')
    }
  }

  async function toggleClosed() {
    if (!id || !item) return
    if (!item.closed_at) {
      const years = item.retention_years
      if (
        !window.confirm(
          `Vorgang schließen? Die Standortfrist von ${years} Jahren beginnt jetzt.`,
        )
      ) {
        return
      }
    }
    setBusy('close')
    setError('')
    setSaved('')
    try {
      apply(item.closed_at ? await api.reopenCase(id) : await api.closeCase(id))
      setSaved(item.closed_at ? 'Wieder geöffnet' : 'Geschlossen. Die Frist läuft.')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Status fehlgeschlagen')
    } finally {
      setBusy('')
    }
  }

  async function removeCase() {
    if (!id || !window.confirm('Vorgang auflösen? Die Fotos bleiben, die Zuordnung und Frist fallen weg.')) return
    try {
      await api.deleteCase(id)
      navigate('/vorgaenge')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Löschen fehlgeschlagen')
    }
  }

  const uniquePhotos = useMemo(() => {
    const seen = new Set<string>()
    return (item?.photos ?? []).filter((s) => {
      if (seen.has(s.record_id)) return false
      seen.add(s.record_id)
      return true
    })
  }, [item])

  if (!item && !error) return <p className="page-status">Laden …</p>
  if (!item) return <p className="error">{error}</p>
  const photos = uniquePhotos

  return (
    <section className="workspace">
      <div className="page-head page-head-split">
        <div>
          <h1>{item.title || 'Vorgang'}</h1>
          <p>
            {caseKindLabel(item.kind)} · {item.retention_years} Jahre
            {item.photo_count === 1 ? ' · 1 Foto' : ` · ${item.photo_count} Fotos`}
            {item.first_at || item.last_at ? ` · ${formatWhen(item.first_at)} – ${formatWhen(item.last_at)}` : ''}
          </p>
          <p className="muted">
            {item.closed_at
              ? [
                  `Geschlossen ${formatDay(item.closed_at)}`,
                  item.closed_by_name ? `von ${item.closed_by_name}` : '',
                  item.due_at ? `· Standortentfernung ab ${formatDay(item.due_at)}` : '',
                  item.redacted_count
                    ? `· ${item.redacted_count === 1 ? '1 Standort' : `${item.redacted_count} Standorte`} entfernt`
                    : '',
                ]
                  .filter(Boolean)
                  .join(' ')
              : 'Offen — die Standortfrist beginnt erst, wenn Sie den Vorgang schließen.'}
          </p>
        </div>
        <Link to="/vorgaenge" className="secondary">
          Alle Vorgänge
        </Link>
      </div>
      {error ? <p className="error">{error}</p> : null}
      <Split id="sf.split.case.cols" axis="x" defaultPct={64} minA={260} minB={220}>
        <Split id="sf.split.case.map" axis="y" defaultPct={56} minA={140} minB={140}>
          <div className="panel detail-map">
            {points.length ? (
              <GraffitiMap
                className="map map-overview"
                points={points}
                onMissingPoint={(sid) =>
                  setItem((prev) =>
                    prev
                      ? { ...prev, photos: (prev.photos ?? []).filter((p) => p.id !== sid) }
                      : prev,
                  )
                }
              />
            ) : (
              <p className="muted">Keine Koordinaten für die Karte.</p>
            )}
          </div>
          <div className="panel workspace-list">
            <ol className="motif-timeline">
              {photos.map((s) => (
                <li key={s.id}>
                  <Link to={sightingPath(s.record_id, s.id)} className="record-row">
                    <img src={s.thumb_url} alt="" />
                    <div>
                      <strong>{s.note.trim() || 'Ohne Notiz'}</strong>
                      <p>
                        {formatWhen(s.captured_at || s.uploaded_at)} · {formatCoords(s.lat, s.lon)}
                        {s.location_redacted_at ? ` · Standort entfernt ${formatDay(s.location_redacted_at)}` : ''}
                        {s.location_due_at && !s.location_redacted_at
                          ? ` · Frist ${formatDay(s.location_due_at)}`
                          : ''}
                      </p>
                    </div>
                  </Link>
                  {writable ? (
                    <button
                      type="button"
                      className="linkish"
                      disabled={busy === s.record_id}
                      onClick={() => void unlink(s.record_id)}
                    >
                      Vom Vorgang lösen
                    </button>
                  ) : null}
                </li>
              ))}
            </ol>
          </div>
        </Split>
        {writable ? (
          <form className="panel stack form-scroll" onSubmit={(e) => void onSave(e)}>
            <label>
              Titel / Aktenzeichen
              <input value={title} onChange={(e) => setTitle(e.target.value)} required />
            </label>
            <label>
              Art
              <select value={kind} onChange={(e) => setKind(e.target.value as CaseKind)}>
                <option value="civil">Zivilrechtlich (3 Jahre)</option>
                <option value="criminal">Strafrechtlich (5 Jahre)</option>
              </select>
            </label>
            <label>
              Notiz
              <textarea value={note} onChange={(e) => setNote(e.target.value)} rows={3} />
            </label>
            {saved ? <p className="ok">{saved}</p> : null}
            <button type="submit">Speichern</button>
            {item.redacted_count ? null : (
              <button type="button" className="secondary" disabled={busy === 'close'} onClick={() => void toggleClosed()}>
                {item.closed_at ? 'Wieder öffnen' : 'Vorgang schließen'}
              </button>
            )}
            <button type="button" className="linkish" onClick={() => void removeCase()}>
              Vorgang auflösen
            </button>
          </form>
        ) : (
          <div className="panel stack form-scroll">
            {item.note ? <p>{item.note}</p> : <p className="muted">Keine Notiz.</p>}
          </div>
        )}
      </Split>
    </section>
  )
}
