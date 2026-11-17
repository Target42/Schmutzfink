import { type FormEvent, useEffect, useState } from 'react'
import { api } from '../api.ts'
import { formatWhen } from '../format.ts'
import type { APIToken } from '../types.ts'

const expiryOptions = [
  { days: 30, label: '30 Tage' },
  { days: 90, label: '90 Tage' },
  { days: 365, label: '1 Jahr' },
]

export function TokensPage() {
  const [items, setItems] = useState<APIToken[]>([])
  const [name, setName] = useState('')
  const [days, setDays] = useState(90)
  const [created, setCreated] = useState('')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)
  const [copied, setCopied] = useState(false)

  function reload() {
    api
      .tokens()
      .then((res) => setItems(res.items))
      .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Fehler'))
  }

  useEffect(() => {
    reload()
  }, [])

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setNotice('')
    setCopied(false)
    setBusy(true)
    try {
      const tok = await api.createToken(name.trim(), days)
      setName('')
      setCreated(tok.token || '')
      setNotice('Token angelegt. Den Wert nur jetzt kopieren — er wird nicht erneut angezeigt.')
      reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Anlegen fehlgeschlagen')
    } finally {
      setBusy(false)
    }
  }

  async function onRevoke(tok: APIToken) {
    if (!window.confirm(`Token „${tok.name}“ widerrufen? Skripte mit diesem Token verlieren den Zugriff.`)) {
      return
    }
    setError('')
    setNotice('')
    setBusy(true)
    try {
      await api.revokeToken(tok.id)
      if (created.startsWith(tok.prefix)) setCreated('')
      setNotice(`Token „${tok.name}“ ist widerrufen.`)
      reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Widerruf fehlgeschlagen')
    } finally {
      setBusy(false)
    }
  }

  async function copyToken() {
    if (!created) return
    try {
      await navigator.clipboard.writeText(created)
      setCopied(true)
    } catch {
      setError('Kopieren nicht möglich — Token bitte markieren.')
    }
  }

  return (
    <section className="tokens-page">
      <div className="page-head">
        <h1>API-Token</h1>
        <p>
          Maschinen-Zugang nur zum Lesen: Suche, Karte, Export. Keine Uploads, keine Originale.
          Header <code>Authorization: Bearer …</code>. Beim Passwortändern werden alle Token ungültig.
        </p>
      </div>
      {error ? <p className="error">{error}</p> : null}
      {notice ? <p className="ok">{notice}</p> : null}
      {created ? (
        <div className="panel token-secret">
          <p className="eyebrow">Nur jetzt sichtbar</p>
          <code>{created}</code>
          <button type="button" className="secondary" onClick={() => void copyToken()}>
            {copied ? 'Kopiert' : 'Kopieren'}
          </button>
        </div>
      ) : null}
      <form className="panel stack" onSubmit={(e) => void onSubmit(e)}>
        <h2>Neues Token</h2>
        <label>
          Name
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            maxLength={80}
            required
            placeholder="z. B. Auswertungsskript"
          />
        </label>
        <label>
          Gültig
          <select value={days} onChange={(e) => setDays(Number(e.target.value))}>
            {expiryOptions.map((opt) => (
              <option key={opt.days} value={opt.days}>
                {opt.label}
              </option>
            ))}
          </select>
        </label>
        <button type="submit" disabled={busy}>
          Token anlegen
        </button>
      </form>
      <table className="data-table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Kennung</th>
            <th>Angelegt</th>
            <th>Gültig bis</th>
            <th>Zuletzt genutzt</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {items.map((tok) => (
            <tr key={tok.id}>
              <td>
                {tok.name}
                {tok.expired ? <div className="muted">abgelaufen</div> : null}
              </td>
              <td className="mono">{tok.prefix}…</td>
              <td>{formatWhen(tok.created_at)}</td>
              <td>{formatWhen(tok.expires_at)}</td>
              <td>{tok.last_used_at ? formatWhen(tok.last_used_at) : '—'}</td>
              <td>
                <button type="button" className="secondary" disabled={busy} onClick={() => void onRevoke(tok)}>
                  Widerrufen
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {items.length === 0 ? <p className="muted">Noch keine Token.</p> : null}
    </section>
  )
}
