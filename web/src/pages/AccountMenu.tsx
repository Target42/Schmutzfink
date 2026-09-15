import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { useAuth } from '../auth.tsx'
import { isDirectoryUser } from '../types.ts'
import { PasswordForm } from './PasswordForm.tsx'

export function AccountMenu() {
  const { user, logout } = useAuth()
  const forced = Boolean(user?.must_change_password)
  const [open, setOpen] = useState(forced)
  const root = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (forced) setOpen(true)
  }, [forced])

  useEffect(() => {
    function onDoc(e: MouseEvent) {
      if (forced) return
      if (root.current && !root.current.contains(e.target as Node)) setOpen(false)
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape' && !forced) setOpen(false)
    }
    document.addEventListener('mousedown', onDoc)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDoc)
      document.removeEventListener('keydown', onKey)
    }
  }, [forced])

  if (!user) return null

  return (
    <div className="account" ref={root}>
      <button
        type="button"
        className={forced ? 'account-toggle forced' : 'account-toggle'}
        aria-expanded={open}
        aria-haspopup="true"
        onClick={() => setOpen((v) => (forced ? true : !v))}
      >
        {user.username}
        <span aria-hidden="true">▾</span>
      </button>
      {open ? (
        <div className="account-panel panel">
          {forced ? null : (
            <>
              <Link to="/hilfe" className="linkish" onClick={() => setOpen(false)}>
                Hilfe
              </Link>
              <Link to="/token" className="linkish" onClick={() => setOpen(false)}>
                API-Token
              </Link>
            </>
          )}
          {isDirectoryUser(user) ? (
            <p className="muted">Anmeldung über Active Directory. Das Kennwort ändern Sie dort.</p>
          ) : (
            <PasswordForm onDone={() => setOpen(false)} />
          )}
          <button type="button" className="linkish account-logout" onClick={() => void logout()}>
            Abmelden
          </button>
        </div>
      ) : null}
    </div>
  )
}
