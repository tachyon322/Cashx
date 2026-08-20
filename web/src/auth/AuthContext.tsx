import { createContext, useCallback, useContext, useEffect, useMemo } from 'react'
import type { ReactNode } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { signout } from '../api/auth'
import { setAuthSignOut, useMe } from '../api/queries'
import type { components } from '../api/schema'

type UserMe = components['schemas']['UserMe']

export interface AuthState {
  user: UserMe | null
  /** Первичная загрузка /auth/me без данных и без ошибки (splash-гейт). */
  isPending: boolean
  isLoading: boolean
  isPartner: boolean
  isStaff: boolean
  signOut: () => Promise<void>
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const me = useMe()

  const signOut = useCallback(async () => {
    try {
      await signout()
    } finally {
      queryClient.clear()
      navigate('/login')
    }
  }, [queryClient, navigate])

  // Регистрируем signOut для обработки 401 на защищённых запросах (без цикл. импорта).
  useEffect(() => {
    setAuthSignOut(signOut)
    return () => setAuthSignOut(null)
  }, [signOut])

  const value = useMemo<AuthState>(
    () => ({
      user: me.data ?? null,
      isPending: me.isPending,
      isLoading: me.isLoading,
      isPartner: me.data?.partner != null,
      isStaff: me.data?.role === 'staff',
      signOut,
    }),
    [me.data, me.isPending, me.isLoading, signOut],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) {
    throw new Error('useAuth должен использоваться внутри AuthProvider')
  }
  return ctx
}
