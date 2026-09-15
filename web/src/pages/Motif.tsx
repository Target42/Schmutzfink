import { type FormEvent, useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api } from '../api.ts'
import { formatCoords, formatWhen } from '../format.ts'
import { useAuth } from '../auth.tsx'
import { canWriteCatalog, formatMinScore, sightingPath, type Motif, type RecordItem } from '../types.ts'
import { GraffitiMap } from './MiniMap.tsx'
import { MinScoreField, useMinScore } from './MinScoreField.tsx'
import { Split } from './Split.tsx'

export function MotifPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { user } = useAuth()
  const writable = canWriteCatalog(user)
  const [motif, setMotif] = useState<Motif | null>(null)
  const [title, setTitle] = useState('')
  const [note, setNote] = useState('')
  const [suggestions, setSuggestions] = useState<RecordItem[]>([])
  const [error, setError] = useState('')
  const [saved, setSaved] = useState('')
  const [busy, setBusy] = useState('')
  const [minScore, setMinScore] = useMinScore()

  function apply(next: Motif) {
    setMotif(next)
    setTitle(next.title)
    setNote(next.note)
  }

  function reload() {
    if (!id) return
    api
      .motif(id)
      .then((m) => {
        apply(m)
        setError('')
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Fehler'))
    api
      .motifSuggestions(id, minScore)
      .then((res) => setSuggestions(res.items))
      .catch(() => setSuggestions([]))
  }

  useEffect(() => {
    reload()
  }, [id, minScore])

  const points = useMemo(
    () =>
      (motif?.sightings ?? [])
        .filter((s) => s.lat != null && s.lon != null)
        .map((s) => ({
          id: s.id,
          recordId: s.record_id,
          lat: s.lat as number,
          lon: s.lon as number,
          note: s.note,
          thumb_url: s.thumb_url,
        })),
    [motif],
  )

  async function onSave(e: FormEvent) {
    e.preventDefault()
    if (!id) return
    setError('')
    setSaved('')
    try {
      apply(await api.patchMotif(id, { title, note }))
      setSaved('Gespeichert')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Speichern fehlgeschlagen')
    }
  }

  async function unlink(sightingId: string) {
    if (!id) return
    setBusy(sightingId)
    setError('')
    try {
      const res = await api.unlinkMotif(id, sightingId)
      if ('deleted' in res && res.deleted) {
        navigate('/motive')
        return
      }
      apply(res as Motif)
      const sug = await api.motifSuggestions(id, minScore)
      setSuggestions(sug.items)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Lösen fehlgeschlagen')
    } finally {
      setBusy('')
    }
  }

  async function assign(sightingId: string) {
    if (!id) return
    setBusy(sightingId)
    setError('')
    try {
      apply(await api.assignMotif(id, sightingId))
      setSuggestions((cur) => cur.filter((s) => s.id !== sightingId))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Zuordnen fehlgeschlagen')
    } finally {
      setBusy('')
    }
  }

  async function removeMotif() {
    if (!id || !window.confirm('Motiv auflösen? Die Sichtungen bleiben, die Zuordnung fällt weg.')) return
    try {
      await api.deleteMotif(id)
      navigate('/motive')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Löschen fehlgeschlagen')
    }
  }

  if (!motif && !error) return <p className="page-status">Laden …</p>
  if (!motif) return <p className="error">{error}</p>
  const items = motif.sightings ?? []

  return (
    <section className="workspace">
      <div className="page-head page-head-split">
        <div>
          <h1>{motif.title || 'Motiv'}</h1>
          <p>
            {motif.sighting_count === 1 ? '1 Sichtung' : `${motif.sighting_count} Sichtungen`}
            {motif.first_at || motif.last_at ? ` · ${formatWhen(motif.first_at)} – ${formatWhen(motif.last_at)}` : ''}
          </p>
        </div>
        <Link to="/motive" className="secondary">
          Alle Motive
        </Link>
      </div>
      {error ? <p className="error">{error}</p> : null}
      <Split id="sf.split.motif.main" axis="y" defaultPct={64} minA={260} minB={160}>
      <Split id="sf.split.motif.cols" axis="x" defaultPct={64} minA={260} minB={220}>
        <Split id="sf.split.motif.map" axis="y" defaultPct={56} minA={140} minB={140}>
          <div className="panel detail-map">
            {points.length ? (
              <GraffitiMap className="map map-overview" points={points} />
            ) : (
              <p className="muted">Keine Koordinaten für die Karte.</p>
            )}
          </div>
          <div className="panel workspace-list">
            <ol className="motif-timeline">
              {items.map((s) => (
                <li key={s.id}>
                  <Link to={sightingPath(s.record_id, s.id)} className="record-row">
                    <img src={s.thumb_url} alt="" />
                    <div>
                      <strong>{s.note || 'Ohne Notiz'}</strong>
                      <p>
                        {formatWhen(s.captured_at || s.uploaded_at)} · {formatCoords(s.lat, s.lon)}
                      </p>
                      <p className="muted">{s.tags.join(', ') || 'keine Tags'}</p>
                    </div>
                  </Link>
                  {writable ? (
                    <button
                      type="button"
                      className="linkish"
                      disabled={busy === s.id}
                      onClick={() => void unlink(s.id)}
                    >
                      Lösen
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
              Titel
              <input value={title} onChange={(e) => setTitle(e.target.value)} />
            </label>
            <label>
              Notiz
              <textarea value={note} onChange={(e) => setNote(e.target.value)} rows={3} />
            </label>
            {saved ? <p className="ok">{saved}</p> : null}
            <button type="submit">Speichern</button>
            <button type="button" className="linkish" onClick={() => void removeMotif()}>
              Motiv auflösen
            </button>
          </form>
        ) : (
          <div className="panel stack form-scroll">
            <p>
              <strong>{motif.title || 'Motiv'}</strong>
            </p>
            <p className="muted">{motif.note.trim() || 'Keine Notiz'}</p>
          </div>
        )}
      </Split>
      <section className="similar">
        <div className="similar-head">
          <h2>Ähnliche Sichtungen</h2>
          <MinScoreField value={minScore} onChange={setMinScore} />
        </div>
        {writable ? (
          <p className="muted">Vorschläge aus CLIP. Zuordnen nur nach Ihrer Bestätigung.</p>
        ) : (
          <p className="muted">Ähnliche Sichtungen aus CLIP.</p>
        )}
        {suggestions.length === 0 ? (
          <p className="muted">Keine Vorschläge über {formatMinScore(minScore)}.</p>
        ) : null}
        <ul className="similar-grid">
          {suggestions.map((hit) => (
            <li key={hit.id} className="suggest-card">
              <Link to={sightingPath(hit.record_id, hit.id)} className="similar-card">
                <img src={hit.thumb_url} alt="" />
                <span>{hit.note || 'Ohne Notiz'}</span>
                {hit.score != null ? <em>{Math.round(hit.score * 100)} %</em> : null}
                {hit.motif_id && hit.motif_id !== motif.id ? (
                  <em>jetzt: {hit.motif_title || 'anderes Motiv'}</em>
                ) : null}
              </Link>
              {writable ? (
                <button type="button" disabled={busy === hit.id} onClick={() => void assign(hit.id)}>
                  Diesem Motiv zuordnen
                </button>
              ) : null}
            </li>
          ))}
        </ul>
      </section>
      </Split>
    </section>
  )
}
