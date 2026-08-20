import { useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { AlertCircle } from 'lucide-react'
import { signin } from '../../api/auth'
import { ApiRequestError } from '../../api/client'
import { useMe } from '../../api/queries'
import { Button } from '../../components/Button'
import { Field } from '../../components/Field'
import { Input } from '../../components/Input'

/** Вход для партнёров и сотрудников. Pending/blocked состояния отрисует AppGate. */
export function LoginPage() {
  const navigate = useNavigate()
  const me = useMe()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (loading) return
    setError(null)
    if (email.trim().length === 0 || password.length === 0) {
      setError('Заполните email и пароль')
      return
    }
    setLoading(true)
    try {
      await signin(email.trim(), password)
      // Свежий /auth/me: роль определяет стартовый роут (сессия есть даже у pending).
      const result = await me.refetch()
      navigate(result.data?.role === 'staff' ? '/admin/partners' : '/cabinet')
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : 'Неверный email или пароль')
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-bg bg-hero-glow p-4">
      <div className="flex w-full max-w-[420px] flex-col gap-4 rounded-lg border border-border bg-surface-1 p-4 shadow-card">
        <div className="flex items-center justify-center gap-3 font-display text-[24px] font-bold">
          CashX
          <span className="h-2 w-2 rounded-full bg-violet shadow-[0_0_14px_rgba(168,85,247,0.9)]" />
        </div>
        <h1 className="text-center text-[19px] font-bold">Вход для партнёров</h1>

        {error && (
          <div
            className="flex items-start gap-2 rounded-md border border-danger/35 bg-danger/10 px-3 py-2.5 text-[13px] leading-[1.45] text-danger [&>svg]:mt-px [&>svg]:shrink-0"
            role="alert"
          >
            <AlertCircle size={16} />
            <span>{error}</span>
          </div>
        )}

        <form className="flex flex-col gap-4" onSubmit={(e) => void onSubmit(e)} noValidate>
          <Field label="Email" htmlFor="login-email">
            <Input
              id="login-email"
              type="email"
              autoComplete="email"
              placeholder="you@example.com"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </Field>
          <Field label="Пароль" htmlFor="login-password">
            <Input
              id="login-password"
              type="password"
              autoComplete="current-password"
              placeholder="••••••••"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </Field>

          <Button type="submit" size="lg" loading={loading} className="mt-0.5 w-full">
            Войти
          </Button>
        </form>

        <div className="mt-0.5 flex flex-col items-center gap-3">
          <Link to="/forgot" className="text-[13.5px] text-muted transition-colors duration-150 hover:text-violet-bright">
            Забыли пароль?
          </Link>
          <Link to="/register" className="text-[13.5px] text-muted transition-colors duration-150 hover:text-violet-bright">
            Регистрация
          </Link>
        </div>
      </div>
    </div>
  )
}
