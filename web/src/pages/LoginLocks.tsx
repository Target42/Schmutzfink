import { useEffect, useState } from 'react'
import { api, type LoginLock } from '../api.ts'
import { formatWhen } from '../format.ts'

export function LoginLocksPage() {
  const [items, setItems] = useState<LoginLock[]>([])
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [ready, setReady] = useState(false)
  const [busyIP, setBusyIP] = useState('')

  function reload() {
    api
      .loginLocks()
      .then((res) => setItems(res.items))
      .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Fehler'))
      .finally(() => setReady(true))
  }

  useEffect(() => {
    reload()
  }, [])

  async function onUnlock(ip: string) {
    if (!window.confirm(`Sperre für ${ip} aufheben?`)) return
    setError('')
    setNotice('')
    setBusyIP(ip)
    try {
      await api.unlockLoginIP(ip)
      setNotice(`${ip} ist wieder freigegeben.`)
      reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Sperre konnte nicht aufgehoben werden')
    } finally {
      setBusyIP('')
    }
  }

  return (
    <section className="locks-page">
      <div className="page-head">
        <h1>Gesperrte IPs</h1>
        <p>
          Nach fünf fehlgeschlagenen Anmeldungen sperrt der Server die Adresse für fünf Minuten.
          Hier heben Sie diese Adress-Sperre sofort auf. Eine gleichzeitige Sperre des
          Benutzernamens bleibt bestehen.
        </p>
      </div>
      {notice ? <p className="ok">{notice}</p> : null}
      {error ? <p className="error">{error}</p> : null}
      <table className="data-table">
        <thead>
          <tr>
            <th>IP</th>
            <th>Fehlversuche</th>
            <th>Gesperrt bis</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {items.map((item) => (
            <tr key={item.ip}>
              <td className="mono">{item.ip}</td>
              <td>{item.fails}</td>
              <td>{formatWhen(item.locked_until)}</td>
              <td>
                <button
                  type="button"
                  className="secondary"
                  disabled={busyIP !== ''}
                  onClick={() => void onUnlock(item.ip)}
                >
                  {busyIP === item.ip ? 'Wird aufgehoben …' : 'Sperre aufheben'}
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {!ready ? <p className="muted">Laden …</p> : null}
      {ready && items.length === 0 && !error ? <p className="muted">Keine IP ist gesperrt.</p> : null}
    </section>
  )
}
