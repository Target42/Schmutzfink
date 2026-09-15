import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { api } from './api.ts'
import type { User } from './types.ts'

type AuthState = {
  user: User | null
  ready: boolean
  login: (username: string, password: string) => Promise<User>
  logout: () => Promise<void>
  changePassword: (oldPassword: string, newPassword: string) => Promise<void>
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [ready, setReady] = useState(false)

  useEffect(() => {
    api
      .me()
      .then((res) => setUser(res.user))
      .catch(() => setUser(null))
      .finally(() => setReady(true))
  }, [])

  const value: AuthState = {
    user,
    ready,
    login: async (username, password) => {
      const res = await api.login(username, password)
      setUser(res.user)
      return res.user
    },
    logout: async () => {
      await api.logout()
      setUser(null)
    },
    changePassword: async (oldPassword, newPassword) => {
      const res = await api.changePassword(oldPassword, newPassword)
      setUser(res.user)
    },
  }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('AuthProvider fehlt')
  return ctx
}
