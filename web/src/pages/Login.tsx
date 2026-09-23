import { type FormEvent, useState } from 'react'
import { Navigate, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth.tsx'

export function LoginPage() {
  const { login, user, ready } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const from = (location.state as { from?: string } | null)?.from || '/'
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')

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
      <form className="panel login-card" onSubmit={(e) => void onSubmit(e)}>
        <img className="login-logo" src="/logo.png" width="168" height="168" alt="Schmutzfink" />
        <p className="eyebrow">Inventar</p>
        <h1>Schmutzfink</h1>
        <p className="lead">Zugang nur mit Konto. Der Bestand ist nicht öffentlich.</p>
        <label>
          Benutzername
          <input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" required />
        </label>
        <label>
          Passwort
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
            required
          />
        </label>
        {error ? <p className="error">{error}</p> : null}
        <button type="submit">Anmelden</button>
      </form>
    </div>
  )
}
