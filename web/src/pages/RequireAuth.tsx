import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { useAuth } from '../auth.tsx'
import { canExecuteDeletion, canWriteCatalog, isAdmin } from '../types.ts'

export function RequireAuth() {
  const { user, ready } = useAuth()
  const location = useLocation()
  if (!ready) return <div className="page-status">Laden …</div>
  if (!user) return <Navigate to="/login" replace state={{ from: location.pathname }} />
  return <Outlet />
}

export function RequireAdmin() {
  const { user } = useAuth()
  if (!isAdmin(user)) return <Navigate to="/" replace />
  return <Outlet />
}

export function RequireWriter() {
  const { user } = useAuth()
  if (!canWriteCatalog(user)) return <Navigate to="/" replace />
  return <Outlet />
}

export function RequireDeletionOfficer() {
  const { user } = useAuth()
  if (!canExecuteDeletion(user)) return <Navigate to="/" replace />
  return <Outlet />
}
