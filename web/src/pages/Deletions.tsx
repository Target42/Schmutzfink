import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api.ts'
import { formatWhen } from '../format.ts'
import { useAuth } from '../auth.tsx'
import { type PhotoDetail } from '../types.ts'

export function DeletionsPage() {
  const { user } = useAuth()
  const [items, setItems] = useState<PhotoDetail[]>([])
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [ready, setReady] = useState(false)
  const [busy, setBusy] = useState('')

  function reload() {
    api
      .deletionRequests()
      .then((res) => setItems(res.items))
      .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Fehler'))
      .finally(() => setReady(true))
  }

  useEffect(() => {
    reload()
  }, [])

  async function reject(photo: PhotoDetail) {
    setError('')
    setNotice('')
    setBusy(photo.id)
    try {
      await api.cancelDeletion(photo.id)
      setItems((cur) => cur.filter((p) => p.id !== photo.id))
      setNotice('Antrag abgelehnt. Das Foto bleibt erhalten.')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ablehnen fehlgeschlagen')
    } finally {
      setBusy('')
    }
  }

  async function remove(photo: PhotoDetail) {
    if (
      !window.confirm(
        'Foto und alle Sichtungen unwiderruflich löschen? Das kann nicht rückgängig gemacht werden.',
      )
    ) {
      return
    }
    setError('')
    setNotice('')
    setBusy(photo.id)
    try {
      await api.deleteRecord(photo.id)
      setItems((cur) => cur.filter((p) => p.id !== photo.id))
      setNotice('Foto gelöscht.')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Löschen fehlgeschlagen')
    } finally {
      setBusy('')
    }
  }

  return (
    <section className="deletions-page">
      <div className="page-head">
        <h1>Löschungen</h1>
        <p>
          Offene Anträge auf vollständige Löschung. Abgelaufene Vorgangsfristen entfernen nur
          GPS und Adresse — das Foto bleibt. Fotos bleiben sichtbar, bis ein Löschbeauftragter sie hier oder in der
          Detailansicht wirklich löscht. Eigene Anträge können nicht selbst ausgeführt werden.
        </p>
      </div>
      {notice ? <p className="ok">{notice}</p> : null}
      {error ? <p className="error">{error}</p> : null}
      {!ready ? <p className="muted">Laden …</p> : null}
      {ready && items.length === 0 && !error ? <p className="muted">Keine offenen Anträge.</p> : null}
      <ul className="deletion-list">
        {items.map((photo) => {
          const req = photo.deletion_request
          const own = req?.requested_by === user?.id
          return (
            <li key={photo.id} className="panel deletion-card">
              <Link to={`/datensatz/${photo.id}`} className="deletion-thumb">
                <img src={photo.thumb_url} alt="" />
              </Link>
              <div className="deletion-meta">
                <p>
                  Beantragt von <strong>{req?.requested_by_name || '—'}</strong> am{' '}
                  {formatWhen(req?.requested_at)}
                </p>
                <p className="muted">Hochgeladen von {photo.uploaded_by_name}</p>
                <p>{req?.reason.trim() ? req.reason : 'Kein Grund angegeben.'}</p>
                <div className="user-actions">
                  <Link to={`/datensatz/${photo.id}`} className="secondary">
                    Foto öffnen
                  </Link>
                  <button
                    type="button"
                    className="secondary"
                    disabled={busy === photo.id}
                    onClick={() => void reject(photo)}
                  >
                    Ablehnen
                  </button>
                  <button
                    type="button"
                    className="danger"
                    disabled={busy === photo.id || own}
                    title={own ? 'Eigene Anträge dürfen nicht selbst ausgeführt werden' : undefined}
                    onClick={() => void remove(photo)}
                  >
                    Löschen
                  </button>
                </div>
              </div>
            </li>
          )
        })}
      </ul>
    </section>
  )
}
