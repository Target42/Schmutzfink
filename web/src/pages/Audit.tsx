import { useEffect, useState } from 'react'
import { api } from '../api.ts'
import { formatWhen } from '../format.ts'
import type { AuditEvent } from '../types.ts'

const labels: Record<string, string> = {
  login: 'Anmeldung',
  login_failed: 'Anmeldung fehlgeschlagen',
  login_disabled: 'Anmeldung deaktiviert',
  login_locked: 'Anmeldung gesperrt',
  logout: 'Abmeldung',
  password_change: 'Passwort geändert',
  password_reset: 'Passwort gesetzt',
  user_create: 'Benutzer angelegt',
  user_disable: 'Benutzer deaktiviert',
  user_enable: 'Benutzer aktiviert',
  user_patch: 'Benutzer geändert',
  record_upload: 'Upload',
  record_import: 'Import',
  record_patch: 'Foto geändert',
  record_delete: 'Foto gelöscht',
  deletion_request: 'Löschung beantragt',
  deletion_cancel: 'Löschantrag zurückgezogen',
  deletion_reject: 'Löschantrag abgelehnt',
  sighting_add: 'Sichtung angelegt',
  sighting_patch: 'Sichtung geändert',
  sighting_delete: 'Sichtung entfernt',
  motif_create: 'Motiv angelegt',
  motif_patch: 'Motiv geändert',
  motif_delete: 'Motiv aufgelöst',
  motif_assign: 'Sichtung zugeordnet',
  motif_unlink: 'Sichtung gelöst',
  field_create: 'Feld angelegt',
  field_patch: 'Feld geändert',
  field_delete: 'Feld gelöscht',
  record_export: 'Bestand exportiert',
  token_create: 'API-Token angelegt',
  token_revoke: 'API-Token widerrufen',
  case_create: 'Vorgang angelegt',
  case_patch: 'Vorgang geändert',
  case_delete: 'Vorgang aufgelöst',
  case_assign: 'Foto einem Vorgang zugeordnet',
  case_unlink: 'Foto vom Vorgang gelöst',
  case_close: 'Vorgang geschlossen',
  case_reopen: 'Vorgang wieder geöffnet',
  location_redact: 'Standort nach Frist entfernt',
}

function actionLabel(action: string) {
  return labels[action] || action
}

function detailText(ev: AuditEvent) {
  if (!ev.detail || Object.keys(ev.detail).length === 0) return ''
  return Object.entries(ev.detail)
    .map(([k, v]) => `${k}=${String(v)}`)
    .join(', ')
}

export function AuditPage() {
  const [items, setItems] = useState<AuditEvent[]>([])
  const [total, setTotal] = useState(0)
  const [error, setError] = useState('')
  const [ready, setReady] = useState(false)

  function reload() {
    api
      .audit()
      .then((res) => {
        setItems(res.items)
        setTotal(res.total)
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Fehler'))
      .finally(() => setReady(true))
  }

  useEffect(() => {
    reload()
  }, [])

  async function loadMore() {
    setError('')
    try {
      const res = await api.audit(items.length)
      setItems((cur) => cur.concat(res.items))
      setTotal(res.total)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Fehler')
    }
  }

  return (
    <section>
      <div className="page-head">
        <h1>Protokoll</h1>
        <p>Anmeldungen, Benutzer und Änderungen am Bestand. Nur für Admins.</p>
      </div>
      {error ? <p className="error">{error}</p> : null}
      <table className="data-table">
        <thead>
          <tr>
            <th>Zeit</th>
            <th>Aktion</th>
            <th>Benutzer</th>
            <th>Objekt</th>
            <th>IP</th>
          </tr>
        </thead>
        <tbody>
          {items.map((ev) => (
            <tr key={ev.id}>
              <td>{formatWhen(ev.at)}</td>
              <td>
                {actionLabel(ev.action)}
                {detailText(ev) ? <div className="muted">{detailText(ev)}</div> : null}
              </td>
              <td>{ev.username || '—'}</td>
              <td className="mono">{ev.subject || '—'}</td>
              <td className="mono">{ev.ip || '—'}</td>
            </tr>
          ))}
        </tbody>
      </table>
      {!ready ? <p className="muted">Laden …</p> : null}
      {ready && items.length === 0 && !error ? <p className="muted">Noch keine Einträge.</p> : null}
      {items.length < total ? (
        <p>
          <button type="button" className="secondary" onClick={() => void loadMore()}>
            Weitere ({items.length} von {total})
          </button>
        </p>
      ) : null}
    </section>
  )
}
