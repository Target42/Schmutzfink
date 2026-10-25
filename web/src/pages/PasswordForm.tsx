import { type FormEvent, useState } from 'react'
import { useAuth } from '../auth.tsx'

export function PasswordForm({ onDone }: { onDone?: () => void }) {
  const { user, changePassword } = useAuth()
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [repeat, setRepeat] = useState('')
  const [error, setError] = useState('')

  const forced = Boolean(user?.must_change_password)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    if (newPassword !== repeat) {
      setError('Die neuen Passwörter stimmen nicht überein')
      return
    }
    try {
      await changePassword(oldPassword, newPassword)
      setOldPassword('')
      setNewPassword('')
      setRepeat('')
      onDone?.()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Passwort konnte nicht geändert werden')
    }
  }

  return (
    <form className="account-form" onSubmit={(e) => void onSubmit(e)}>
      <p className="eyebrow">{forced ? 'Pflicht' : 'Konto'}</p>
      <h2>Passwort ändern</h2>
      <p className="lead">
        {forced
          ? 'Erst nach einem eigenen Passwort ist der Bestand erreichbar.'
          : 'Mindestens 8 Zeichen, nicht der Benutzername, kein Standardwort.'}
      </p>
      <label>
        Aktuelles Passwort
        <input
          type="password"
          value={oldPassword}
          onChange={(e) => setOldPassword(e.target.value)}
          autoComplete="current-password"
          required
        />
      </label>
      <label>
        Neues Passwort
        <input
          type="password"
          value={newPassword}
          onChange={(e) => setNewPassword(e.target.value)}
          autoComplete="new-password"
          minLength={8}
          required
        />
      </label>
      <label>
        Neues Passwort wiederholen
        <input
          type="password"
          value={repeat}
          onChange={(e) => setRepeat(e.target.value)}
          autoComplete="new-password"
          minLength={8}
          required
        />
      </label>
      {error ? <p className="error">{error}</p> : null}
      <button type="submit">Passwort speichern</button>
    </form>
  )
}
