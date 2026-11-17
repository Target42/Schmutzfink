import { type FormEvent, useEffect, useState } from 'react'
import { api } from '../api.ts'
import { useAuth } from '../auth.tsx'
import { canExecuteDeletion, isDirectoryUser, roleLabel, type Role, type User } from '../types.ts'

export function UsersPage() {
  const { user: me } = useAuth()
  const [items, setItems] = useState<User[]>([])
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [provider, setProvider] = useState<'local' | 'ldap'>('local')
  const [role, setRole] = useState<Role>('user')
  const [canDelete, setCanDelete] = useState(false)
  const [resetFor, setResetFor] = useState<string | null>(null)
  const [resetPassword, setResetPassword] = useState('')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)

  function reload() {
    api
      .users()
      .then((res) => setItems(res.items))
      .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Fehler'))
  }

  useEffect(() => {
    reload()
  }, [])

  const enabledAdmins = items.filter((u) => u.role === 'admin' && !u.disabled).length

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setNotice('')
    setBusy(true)
    try {
      await api.createUser(username, password, role, role === 'searcher' ? false : canDelete, provider)
      setUsername('')
      setPassword('')
      setProvider('local')
      setRole('user')
      setCanDelete(false)
      setNotice('Benutzer angelegt.')
      reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Anlegen fehlgeschlagen')
    } finally {
      setBusy(false)
    }
  }

  async function onReset(e: FormEvent, user: User) {
    e.preventDefault()
    setError('')
    setNotice('')
    setBusy(true)
    try {
      await api.resetUserPassword(user.id, resetPassword)
      setResetFor(null)
      setResetPassword('')
      setNotice(`Neues Startpasswort für ${user.username} gesetzt.`)
      reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Passwort konnte nicht gesetzt werden')
    } finally {
      setBusy(false)
    }
  }

  async function onToggle(user: User) {
    const next = !user.disabled
    if (next && !window.confirm(`Konto „${user.username}“ deaktivieren? Die Person kann sich nicht mehr anmelden.`)) {
      return
    }
    setError('')
    setNotice('')
    setBusy(true)
    try {
      await api.setUserDisabled(user.id, next)
      setNotice(next ? `${user.username} ist deaktiviert.` : `${user.username} ist wieder aktiv.`)
      reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Änderung fehlgeschlagen')
    } finally {
      setBusy(false)
    }
  }

  async function onRole(user: User, next: Role) {
    setError('')
    setNotice('')
    setBusy(true)
    try {
      await api.patchUser(user.id, { role: next, can_delete: next === 'searcher' ? false : user.can_delete })
      setNotice(`${user.username}: ${roleLabel(next)}`)
      reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Rolle konnte nicht geändert werden')
    } finally {
      setBusy(false)
    }
  }

  async function onCanDelete(user: User, next: boolean) {
    setError('')
    setNotice('')
    setBusy(true)
    try {
      await api.patchUser(user.id, { can_delete: next })
      setNotice(next ? `${user.username} ist Löschbeauftragter.` : `${user.username} ist nicht mehr Löschbeauftragter.`)
      reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Änderung fehlgeschlagen')
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="users-page">
      <div className="page-head">
        <h1>Benutzer</h1>
        <p>
          Admins legen Konten an und setzen Rollen. Sachbearbeiter pflegen den Bestand.
          Löschbeauftragte sind eine Zusatzrolle und führen beantragte Löschungen aus. Sucher
          dürfen nur suchen — auch mit einem Vergleichsbild, das nicht im Bestand landet.
          AD-Konten bekommen kein Passwort in Schmutzfink; die Anmeldung prüft das Kennwort
          gegen das Verzeichnis.
        </p>
      </div>
      {notice ? <p className="ok">{notice}</p> : null}
      {error ? <p className="error">{error}</p> : null}
      <table className="data-table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Rolle</th>
            <th>Status</th>
            <th>Aktionen</th>
          </tr>
        </thead>
        <tbody>
          {items.map((u) => {
            const self = u.id === me?.id
            const lastAdmin = u.role === 'admin' && !u.disabled && enabledAdmins < 2
            return (
              <tr key={u.id} className={u.disabled ? 'user-disabled' : undefined}>
                <td>
                  {u.username}
                  {isDirectoryUser(u) ? <span className="muted"> · AD</span> : null}
                </td>
                <td>
                  <div className="role-edit">
                    <select
                      value={u.role}
                      disabled={busy || (lastAdmin && u.role === 'admin')}
                      title={lastAdmin && u.role === 'admin' ? 'Der letzte Admin kann nicht herabgestuft werden' : undefined}
                      onChange={(e) => void onRole(u, e.target.value as Role)}
                    >
                      <option value="user">Sachbearbeiter</option>
                      <option value="searcher">Sucher</option>
                      <option value="admin">Admin</option>
                    </select>
                    {u.role === 'searcher' ? null : (
                      <label className="role-flag">
                        <input
                          type="checkbox"
                          checked={canExecuteDeletion(u)}
                          disabled={busy}
                          onChange={(e) => void onCanDelete(u, e.target.checked)}
                        />
                        Löschbeauftragter
                      </label>
                    )}
                  </div>
                </td>
                <td>
                  {u.disabled
                    ? 'Deaktiviert'
                    : u.must_change_password && !isDirectoryUser(u)
                      ? 'Passwortänderung ausstehend'
                      : 'Aktiv'}
                </td>
                <td>
                  <div className="user-actions">
                    {!self && !isDirectoryUser(u) ? (
                      <button
                        type="button"
                        className="secondary"
                        disabled={busy}
                        onClick={() => {
                          setResetFor(resetFor === u.id ? null : u.id)
                          setResetPassword('')
                          setError('')
                          setNotice('')
                        }}
                      >
                        Passwort setzen
                      </button>
                    ) : null}
                    {!self ? (
                      <button
                        type="button"
                        className="secondary"
                        disabled={busy || (!u.disabled && lastAdmin)}
                        title={!u.disabled && lastAdmin ? 'Der letzte Admin kann nicht deaktiviert werden' : undefined}
                        onClick={() => void onToggle(u)}
                      >
                        {u.disabled ? 'Aktivieren' : 'Deaktivieren'}
                      </button>
                    ) : (
                      <span className="muted">eigenes Konto</span>
                    )}
                  </div>
                  {resetFor === u.id ? (
                    <form className="user-reset" onSubmit={(e) => void onReset(e, u)}>
                      <label>
                        Neues Startpasswort
                        <input
                          type="password"
                          value={resetPassword}
                          onChange={(e) => setResetPassword(e.target.value)}
                          minLength={8}
                          required
                          autoComplete="new-password"
                        />
                      </label>
                      <div className="user-actions">
                        <button type="submit" disabled={busy}>
                          Setzen
                        </button>
                        <button
                          type="button"
                          className="secondary"
                          onClick={() => {
                            setResetFor(null)
                            setResetPassword('')
                          }}
                        >
                          Abbrechen
                        </button>
                      </div>
                    </form>
                  ) : null}
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
      {items.length === 0 ? <p className="muted">Noch keine Benutzer geladen.</p> : null}
      <form className="panel stack" onSubmit={(e) => void onSubmit(e)}>
        <h2>Neu anlegen</h2>
        <label>
          Benutzername
          <input value={username} onChange={(e) => setUsername(e.target.value)} required />
        </label>
        <label>
          Konto
          <select
            value={provider}
            onChange={(e) => setProvider(e.target.value as 'local' | 'ldap')}
          >
            <option value="local">Lokales Passwort</option>
            <option value="ldap">Active Directory</option>
          </select>
        </label>
        {provider === 'ldap' ? (
          <p className="muted">
            Kein Startpasswort. Der Name muss zum Konto im Verzeichnis passen (meist
            sAMAccountName).
          </p>
        ) : (
          <label>
            Passwort
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              minLength={8}
              required
            />
          </label>
        )}
        <label>
          Rolle
          <select
            value={role}
            onChange={(e) => {
              const next = e.target.value as Role
              setRole(next)
              if (next === 'searcher') setCanDelete(false)
            }}
          >
            <option value="user">Sachbearbeiter</option>
            <option value="searcher">Sucher</option>
            <option value="admin">Admin</option>
          </select>
        </label>
        {role === 'searcher' ? null : (
          <label className="role-flag">
            <input type="checkbox" checked={canDelete} onChange={(e) => setCanDelete(e.target.checked)} />
            Zusätzlich Löschbeauftragter
          </label>
        )}
        <button type="submit" disabled={busy}>
          Anlegen
        </button>
      </form>
    </section>
  )
}
