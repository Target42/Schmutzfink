import { useEffect, useId, useState } from 'react'
import { NavLink, Outlet, useLocation } from 'react-router-dom'
import { useAuth } from '../auth.tsx'
import { canExecuteDeletion, canWriteCatalog, isAdmin } from '../types.ts'
import { AccountMenu } from './AccountMenu.tsx'

export function Layout() {
  const { user } = useAuth()
  const location = useLocation()
  const [navOpen, setNavOpen] = useState(false)
  const navId = useId()

  useEffect(() => {
    setNavOpen(false)
  }, [location.pathname])

  useEffect(() => {
    if (!navOpen) return
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') setNavOpen(false)
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [navOpen])

  return (
    <div className="app-shell">
      <header className="topbar">
        <div className="brand">
          <img className="brand-logo" src="/logo-mark.png" width="28" height="28" alt="" />
          Schmutzfink
        </div>
        <button
          type="button"
          className="nav-toggle"
          aria-expanded={navOpen}
          aria-controls={navId}
          onClick={() => setNavOpen((v) => !v)}
        >
          {navOpen ? 'Menü schließen' : 'Menü'}
        </button>
        <nav id={navId} className={navOpen ? 'is-open' : undefined}>
          <NavLink to="/" end>
            Bestand
          </NavLink>
          <NavLink to="/karte">Karte</NavLink>
          <NavLink to="/auswertungen">Auswertungen</NavLink>
          {canWriteCatalog(user) ? <NavLink to="/hochladen">Hochladen</NavLink> : null}
          <NavLink to="/motive">Motive</NavLink>
          <NavLink to="/vorgaenge">Vorgänge</NavLink>
          {isAdmin(user) ? <NavLink to="/benutzer">Benutzer</NavLink> : null}
          {isAdmin(user) ? <NavLink to="/felder">Felder</NavLink> : null}
          {canExecuteDeletion(user) ? <NavLink to="/loeschungen">Löschungen</NavLink> : null}
          {isAdmin(user) ? <NavLink to="/protokoll">Protokoll</NavLink> : null}
          <NavLink to="/hilfe">Hilfe</NavLink>
        </nav>
        <AccountMenu />
      </header>
      <main>
        {user?.must_change_password ? (
          <section className="narrow">
            <div className="page-head">
              <h1>Passwort ändern</h1>
              <p>
                Das Formular steht oben rechts unter <strong>{user.username}</strong>.
              </p>
            </div>
          </section>
        ) : (
          <Outlet />
        )}
      </main>
    </div>
  )
}
