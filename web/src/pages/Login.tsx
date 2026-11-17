import { type FormEvent, useEffect, useState } from 'react'
import { Link, Navigate, useLocation, useNavigate } from 'react-router-dom'
import { api } from '../api.ts'
import { useAuth } from '../auth.tsx'

export function LoginPage() {
  const { login, user, ready } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const from = (location.state as { from?: string } | null)?.from || '/'
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState('')
  const [publicMap, setPublicMap] = useState(false)

  useEffect(() => {
    api
      .publicConfig()
      .then((res) => setPublicMap(res.public_map))
      .catch(() => setPublicMap(false))
  }, [])

  if (!ready) return <div className="page-status">Laden …</div>
  if (user) return <Navigate to={from} replace />

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    try {
      await login(username, password)
      navigate(from, { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Anmeldung fehlgeschlagen')
    }
  }

  return (
    <div className="login-wrap">
      <div className="login-stack">
        <form className="panel login-card" onSubmit={(e) => void onSubmit(e)}>
          <img className="login-logo" src="/logo.png" width="168" height="168" alt="Schmutzfink" />
          <p className="eyebrow">Inventar</p>
          <h1>Schmutzfink</h1>
          <p className="lead">
            {publicMap
              ? 'Der Bestand bleibt intern. Die Karte mit Vorschaubildern ist ohne Anmeldung sichtbar.'
              : 'Zugang nur mit Konto. Der Bestand ist nicht öffentlich.'}
          </p>
          <label>
            Benutzername
            <input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" required />
          </label>
          <div className="field">
            <label htmlFor="login-password">Passwort</label>
            <span className="password-field">
              <input
                id="login-password"
                type={showPassword ? 'text' : 'password'}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="current-password"
                required
              />
              <button
                type="button"
                className="password-toggle"
                aria-pressed={showPassword}
                aria-label={showPassword ? 'Passwort verbergen' : 'Passwort anzeigen'}
                title={showPassword ? 'Passwort verbergen' : 'Passwort anzeigen'}
                onClick={() => setShowPassword((visible) => !visible)}
              >
                {showPassword ? <EyeOffIcon /> : <EyeIcon />}
              </button>
            </span>
          </div>
          {error ? <p className="error">{error}</p> : null}
          <button type="submit">Anmelden</button>
          {publicMap ? (
            <Link to="/karte" className="login-map-link">
              Karte ohne Anmeldung ansehen
            </Link>
          ) : null}
        </form>
        <p className="login-sources muted">
          Quellcode:{' '}
          <a href="https://github.com/Target42/Schmutzfink" target="_blank" rel="noopener noreferrer">
            github.com/Target42/Schmutzfink
          </a>
        </p>
      </div>
    </div>
  )
}

function EyeIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true">
      <path
        d="M2.062 12.348a1 1 0 0 1 0-.696 10.75 10.75 0 0 1 19.876 0 1 1 0 0 1 0 .696 10.75 10.75 0 0 1-19.876 0"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <circle cx="12" cy="12" r="3" fill="none" stroke="currentColor" strokeWidth="2" />
    </svg>
  )
}

function EyeOffIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true">
      <path
        d="M10.733 5.076a10.744 10.744 0 0 1 11.205 6.575 1 1 0 0 1 0 .696 10.747 10.747 0 0 1-1.444 2.49"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M14.084 14.158a3 3 0 0 1-4.242-4.242"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M17.479 17.499a10.75 10.75 0 0 1-15.417-5.151 1 1 0 0 1 0-.696 10.75 10.75 0 0 1 4.446-5.143"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="m2 2 20 20"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  )
}
