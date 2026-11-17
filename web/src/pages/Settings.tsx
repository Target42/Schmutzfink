import { useEffect, useState } from 'react'
import { api } from '../api.ts'

export function SettingsPage() {
  const [publicMap, setPublicMap] = useState(false)
  const [loaded, setLoaded] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    api
      .settings()
      .then((res) => {
        setPublicMap(res.public_map)
        setLoaded(true)
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Fehler'))
  }, [])

  async function onToggle(next: boolean) {
    if (
      next &&
      !window.confirm(
        'Ohne Anmeldung werden Fundorte und die kleinen Vorschaubilder sichtbar. Notizen, Originale und der Bestand bleiben gesperrt. Einschalten?',
      )
    ) {
      return
    }
    setError('')
    setNotice('')
    setBusy(true)
    try {
      const res = await api.setPublicMap(next)
      setPublicMap(res.public_map)
      setNotice(res.public_map ? 'Die Karte ist ohne Anmeldung sichtbar.' : 'Die Karte ist wieder nur nach Anmeldung sichtbar.')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Speichern fehlgeschlagen')
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="settings-page">
      <div className="page-head">
        <h1>Einstellungen</h1>
        <p>Hier steuern Sie, was jemand ohne Konto von Schmutzfink sehen kann.</p>
      </div>
      {notice ? <p className="ok">{notice}</p> : null}
      {error ? <p className="error">{error}</p> : null}
      <form className="panel stack" onSubmit={(e) => e.preventDefault()}>
        <h2>Öffentliche Karte</h2>
        <p className="muted">
          Die Anmeldeseite verlinkt dann auf die Karte. Besucher sehen Marker und beim Draufzeigen
          oder Antippen nur das kleine Vorschaubild. Notizen, Tags, Originale, Filter, Export und
          der Weg zum Datensatz bleiben zu.
        </p>
        {loaded ? (
          <label className="role-flag">
            <input
              type="checkbox"
              checked={publicMap}
              disabled={busy}
              onChange={(e) => void onToggle(e.target.checked)}
            />
            Karte ohne Anmeldung anzeigen
          </label>
        ) : error ? null : (
          <p className="muted">Laden …</p>
        )}
      </form>
    </section>
  )
}
