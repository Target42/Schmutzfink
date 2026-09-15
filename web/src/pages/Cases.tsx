import { type FormEvent, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api.ts'
import { formatDay, formatWhen } from '../format.ts'
import { useAuth } from '../auth.tsx'
import { canWriteCatalog, caseKindLabel, casePath, type Case, type CaseKind } from '../types.ts'

export function CasesPage() {
  const { user } = useAuth()
  const writable = canWriteCatalog(user)
  const [items, setItems] = useState<Case[]>([])
  const [error, setError] = useState('')
  const [title, setTitle] = useState('')
  const [kind, setKind] = useState<CaseKind>('civil')
  const [busy, setBusy] = useState(false)

  function reload() {
    api
      .cases()
      .then((res) => {
        setItems(res.items)
        setError('')
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Fehler'))
  }

  useEffect(() => {
    reload()
  }, [])

  async function onCreate(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      const created = await api.createCase({ title, kind })
      setTitle('')
      setKind('civil')
      setItems((cur) => [created, ...cur.filter((c) => c.id !== created.id)])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Anlegen fehlgeschlagen')
    } finally {
      setBusy(false)
    }
  }

  const total = items.length
  const countLabel = total === 1 ? '1 Vorgang' : `${total} Vorgänge`

  return (
    <section>
      <div className="page-head">
        <h1>Vorgänge</h1>
        <p>
          {countLabel}. Die Frist (3 bzw. 5 Jahre) läuft erst, wenn ein Bearbeiter den Vorgang
          schließt. Danach fallen GPS und Adresse (bis zur PLZ) weg. Die Fotos bleiben.
        </p>
      </div>
      {error ? <p className="error">{error}</p> : null}
      {writable ? (
        <form className="panel stack case-create" onSubmit={(e) => void onCreate(e)}>
          <div className="split">
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
          </div>
          <button type="submit" disabled={busy}>
            {busy ? 'Anlegen …' : 'Vorgang anlegen'}
          </button>
        </form>
      ) : null}
      {items.length === 0 && !error ? (
        <p className="empty">
          {writable ? 'Noch keine Vorgänge. Legen Sie einen an und ordnen Sie Fotos zu.' : 'Noch keine Vorgänge.'}
        </p>
      ) : null}
      <ul className="record-list">
        {items.map((c) => (
          <li key={c.id}>
            <Link to={casePath(c.id)} className="record-row">
              {c.thumb_url ? <img src={c.thumb_url} alt="" /> : <div className="thumb-placeholder" />}
              <div>
                <strong>{c.title || 'Vorgang'}</strong>
                <p>
                  {caseKindLabel(c.kind)} · {c.retention_years} Jahre
                  {c.photo_count === 1 ? ' · 1 Foto' : ` · ${c.photo_count} Fotos`}
                  {c.first_at || c.last_at ? ` · ${formatWhen(c.first_at)} – ${formatWhen(c.last_at)}` : ''}
                </p>
                <p className="muted">
                  {c.closed_at
                    ? c.due_at
                      ? `Geschlossen ${formatDay(c.closed_at)} · Standortentfernung ab ${formatDay(c.due_at)}`
                      : c.redacted_count
                        ? `Geschlossen ${formatDay(c.closed_at)} · Standort entfernt`
                        : `Geschlossen ${formatDay(c.closed_at)}`
                    : 'Offen — Frist beginnt erst nach dem Schließen'}
                </p>
                {c.note ? <p className="muted">{c.note}</p> : null}
              </div>
            </Link>
          </li>
        ))}
      </ul>
    </section>
  )
}
