import { useEffect, useId, useRef, useState } from 'react'
import { NavLink, Outlet, useLocation } from 'react-router-dom'
import { useAuth } from '../auth.tsx'
import { canExecuteDeletion, canWriteCatalog, isAdmin } from '../types.ts'
import { AccountMenu } from './AccountMenu.tsx'

type NavEntry = {
  to: string
  label: string
  end?: boolean
  also?: string[]
}

function entryCurrent(pathname: string, entry: NavEntry) {
  if (entry.end) return pathname === entry.to
  if (pathname === entry.to || pathname.startsWith(`${entry.to}/`)) return true
  return (entry.also ?? []).some(
    (prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`),
  )
}

function NavEntryLink({ entry, onPick }: { entry: NavEntry; onPick: () => void }) {
  const { pathname } = useLocation()
  const current = entryCurrent(pathname, entry)
  return (
    <NavLink
      to={entry.to}
      end={entry.end}
      className={() => (current ? 'active' : '')}
      aria-current={current ? 'page' : undefined}
      onClick={onPick}
    >
      {entry.label}
    </NavLink>
  )
}

function NavGroup({
  label,
  entries,
  open,
  onToggle,
  onPick,
}: {
  label: string
  entries: NavEntry[]
  open: boolean
  onToggle: () => void
  onPick: () => void
}) {
  const panelId = useId()
  const { pathname } = useLocation()
  if (entries.length === 0) return null
  if (entries.length === 1) return <NavEntryLink entry={entries[0]} onPick={onPick} />

  const current = entries.some((entry) => entryCurrent(pathname, entry))
  return (
    <div className="nav-group">
      <button
        type="button"
        className={current ? 'nav-group-toggle is-current' : 'nav-group-toggle'}
        aria-expanded={open}
        aria-controls={panelId}
        aria-current={current ? 'true' : undefined}
        onClick={onToggle}
      >
        {label}
        <span className="nav-caret" aria-hidden="true">
          ▾
        </span>
      </button>
      {open ? (
        <div id={panelId} className="nav-group-panel" role="group" aria-label={label}>
          {entries.map((entry) => (
            <NavEntryLink key={entry.to} entry={entry} onPick={onPick} />
          ))}
        </div>
      ) : null}
    </div>
  )
}

export function Layout() {
  const { user, ready } = useAuth()
  const location = useLocation()
  const [navOpen, setNavOpen] = useState(false)
  const [openGroup, setOpenGroup] = useState<string | null>(null)
  const navId = useId()
  const navRef = useRef<HTMLElement>(null)

  const catalog: NavEntry[] = user
    ? [
        { to: '/motive', label: 'Motive', also: ['/motiv'] },
        { to: '/vorgaenge', label: 'Vorgänge', also: ['/vorgang'] },
      ]
    : []

  const adminEntries: NavEntry[] = []
  if (isAdmin(user)) {
    adminEntries.push(
      { to: '/benutzer', label: 'Benutzer' },
      { to: '/felder', label: 'Felder' },
      { to: '/einstellungen', label: 'Einstellungen' },
    )
  }
  if (canExecuteDeletion(user)) {
    adminEntries.push({ to: '/loeschungen', label: 'Löschungen' })
  }
  if (isAdmin(user)) {
    adminEntries.push({ to: '/protokoll', label: 'Protokoll' })
  }

  function closeNav() {
    setNavOpen(false)
    setOpenGroup(null)
  }

  useEffect(() => {
    setNavOpen(false)
    setOpenGroup(null)
  }, [location.pathname])

  useEffect(() => {
    if (!navOpen && !openGroup) return
    function onKey(e: KeyboardEvent) {
      if (e.key !== 'Escape') return
      if (openGroup) setOpenGroup(null)
      else setNavOpen(false)
    }
    function onPointer(e: MouseEvent) {
      if (!openGroup) return
      if (navRef.current && !navRef.current.contains(e.target as Node)) setOpenGroup(null)
    }
    document.addEventListener('keydown', onKey)
    document.addEventListener('mousedown', onPointer)
    return () => {
      document.removeEventListener('keydown', onKey)
      document.removeEventListener('mousedown', onPointer)
    }
  }, [navOpen, openGroup])

  if (!ready) return <div className="page-status">Laden …</div>
  if (!user && location.pathname !== '/karte') return <Outlet />

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
        <nav id={navId} ref={navRef} className={navOpen ? 'is-open' : undefined}>
          {user ? (
            <NavEntryLink entry={{ to: '/', label: 'Bestand', end: true }} onPick={closeNav} />
          ) : null}
          <NavEntryLink entry={{ to: '/karte', label: 'Karte' }} onPick={closeNav} />
          {user ? (
            <NavEntryLink entry={{ to: '/auswertungen', label: 'Auswertungen' }} onPick={closeNav} />
          ) : null}
          {user && canWriteCatalog(user) ? (
            <NavEntryLink entry={{ to: '/hochladen', label: 'Hochladen' }} onPick={closeNav} />
          ) : null}
          <NavGroup
            label="Katalog"
            entries={catalog}
            open={openGroup === 'katalog'}
            onToggle={() => setOpenGroup((current) => (current === 'katalog' ? null : 'katalog'))}
            onPick={closeNav}
          />
          <NavGroup
            label="Verwaltung"
            entries={adminEntries}
            open={openGroup === 'verwaltung'}
            onToggle={() =>
              setOpenGroup((current) => (current === 'verwaltung' ? null : 'verwaltung'))
            }
            onPick={closeNav}
          />
          {user ? <NavEntryLink entry={{ to: '/hilfe', label: 'Hilfe' }} onPick={closeNav} /> : null}
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
