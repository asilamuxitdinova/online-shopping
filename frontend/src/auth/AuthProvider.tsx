import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { api } from '../api/api'
import { ApiError, setSessionExpiredHandler, tokens } from '../api/client'
import type { AuthResponse, User } from '../api/types'

interface Auth {
  user: User | null
  /** False until the stored session has been checked on page load. */
  ready: boolean
  /** True after the session ran out without the user logging out. */
  expired: boolean
  login: (email: string, password: string) => Promise<void>
  register: (name: string, email: string, password: string) => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<Auth | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [ready, setReady] = useState(false)
  const [expired, setExpired] = useState(false)

  const loadUser = useCallback(async () => {
    if (!tokens.access && !tokens.refresh) {
      setUser(null)
      return
    }
    try {
      setUser(await api.me())
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) tokens.clear()
      setUser(null)
    }
  }, [])

  useEffect(() => {
    setSessionExpiredHandler(() => {
      setUser(null)
      setExpired(true)
    })
    loadUser().finally(() => setReady(true))

    // Follow logins and logouts that happen in other tabs.
    const onStorage = (e: StorageEvent) => {
      if (e.key !== tokens.storageKey) return
      if (e.newValue === null) setUser(null)
      else if (e.oldValue === null) loadUser()
    }
    window.addEventListener('storage', onStorage)
    return () => window.removeEventListener('storage', onStorage)
  }, [loadUser])

  const start = useCallback((auth: AuthResponse) => {
    tokens.set(auth)
    setUser(auth.user)
    setExpired(false)
  }, [])

  const value = useMemo<Auth>(
    () => ({
      user,
      ready,
      expired,
      login: async (email, password) => start(await api.login(email, password)),
      register: async (name, email, password) => start(await api.register(name, email, password)),
      logout: async () => {
        try {
          if (tokens.refresh) await api.logout()
        } catch {
          // the server session expires on its own; log out locally regardless
        } finally {
          tokens.clear()
          setUser(null)
        }
      },
    }),
    [user, ready, expired, start],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): Auth {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider')
  return ctx
}
