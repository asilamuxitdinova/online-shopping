import { Navigate, Outlet, useLocation } from 'react-router'
import { useAuth } from '../auth/AuthProvider'
import { NotFoundPage } from '../pages/NotFoundPage'
import { Loading } from './ui'

/** Renders child routes for logged-in users (admins only with `admin`), else sends to login. */
export function RequireAuth({ admin = false }: { admin?: boolean }) {
  const { user, ready } = useAuth()
  const location = useLocation()

  if (!ready) return <Loading />
  if (!user) return <Navigate to="/login" replace state={{ from: location }} />
  if (admin && user.role !== 'admin') return <NotFoundPage />
  return <Outlet />
}
