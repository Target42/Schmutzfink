import { useEffect, useState } from 'react'
import { api, type AndroidMobileInfo } from '../api.ts'

function formatBytes(n: number) {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / (1024 * 1024)).toFixed(1)} MB`
}

export function MobileAppPage() {
  const [info, setInfo] = useState<AndroidMobileInfo | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    api
      .mobileAndroid()
      .then(setInfo)
      .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Fehler'))
  }, [])

  return (
    <div className="mobile-app-page">
      <div className="page-head">
        <h1>Android-App</h1>
        <p>
          Feld-Client für Kamera und GPS. Liste, Karte und Verwaltung bleiben in dieser Web-Oberfläche.
        </p>
      </div>

      <section className="panel">
        {error ? <p className="error">{error}</p> : null}
        {!error && !info ? <p className="muted">Lade…</p> : null}
        {info && !info.available ? (
          <p className="muted">
            Derzeit ist keine App-Datei auf dem Server hinterlegt. Bitte die Administration bitten, eine
            signierte Release-APK unter dem konfigurierten Pfad abzulegen.
          </p>
        ) : null}
        {info?.available ? (
          <>
            <dl className="mobile-app-meta">
              {info.version ? (
                <>
                  <dt>Version</dt>
                  <dd>
                    {info.version}
                    {info.version_code ? ` (${info.version_code})` : ''}
                  </dd>
                </>
              ) : null}
              {info.released_at ? (
                <>
                  <dt>Stand</dt>
                  <dd>{new Date(info.released_at).toLocaleString('de-DE')}</dd>
                </>
              ) : null}
              {info.size_bytes != null ? (
                <>
                  <dt>Größe</dt>
                  <dd>{formatBytes(info.size_bytes)}</dd>
                </>
              ) : null}
            </dl>
            {info.notes ? <p>{info.notes}</p> : null}
            <p>
              <a className="primary" href="/api/mobile/android.apk" download={info.filename || 'schmutzfink-android.apk'}>
                APK herunterladen
              </a>
            </p>
            <h2>Installation</h2>
            <ol>
              <li>Datei auf dem Android-Gerät öffnen (Download oder per USB/Dateifreigabe).</li>
              <li>
                Falls nötig: Installation aus dieser Quelle / unbekannte Apps einmalig erlauben
                (oder über das Geräte-Management der Behörde).
              </li>
              <li>In der App unter „Server einstellen“ die URL dieses Portals eintragen und anmelden.</li>
            </ol>
            <p className="muted">
              Updates erscheinen hier als neue Version. Die App muss dieselbe Signatur behalten —
              sonst ist eine Neuinstallation nötig.
            </p>
          </>
        ) : null}
      </section>
    </div>
  )
}
