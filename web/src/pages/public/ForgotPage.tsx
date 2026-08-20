import { useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { AlertCircle, KeyRound } from 'lucide-react'
import { requestPasswordReset } from '../../api/auth'
import { ApiRequestError } from '../../api/client'
import { Button } from '../../components/Button'
import { CopyButton } from '../../components/CopyButton'
import { Field } from '../../components/Field'
import { Input } from '../../components/Input'

/** Запрос сброса пароля; в dev-режиме API возвращает reset_token в ответе. */
export function ForgotPage() {
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [sent, setSent] = useState(false)
  const [token, setToken] = useState<string | null>(null)

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (loading) return
    setError(null)
    if (email.trim().length === 0) {
      setError('Укажите email')
      return
    }
    setLoading(true)
    try {
      const res = await requestPasswordReset(email.trim())
      setToken(res.reset_token ?? null)
      setSent(true)
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : 'Не удалось отправить запрос. Попробуйте ещё раз.')
    } finally {
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
        <h1 className="text-center text-[19px] font-bold">Восстановление пароля</h1>

        {!sent ? (
          <>
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
              <Field label="Email" htmlFor="forgot-email">
                <Input
                  id="forgot-email"
                  type="email"
                  autoComplete="email"
                  placeholder="you@example.com"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                />
              </Field>

              <Button type="submit" size="lg" loading={loading} className="mt-0.5 w-full">
                Отправить
              </Button>
            </form>

            <div className="mt-0.5 flex flex-col items-center gap-3">
              <Link to="/login" className="text-[13.5px] text-muted transition-colors duration-150 hover:text-violet-bright">
                Назад к входу
              </Link>
            </div>
          </>
        ) : token ? (
          <>
            <p className="text-center text-[13.5px] leading-[1.55] text-muted">
              Dev-режим: API вернул одноразовый токен сброса.
            </p>

            <div className="flex flex-col items-stretch gap-3 rounded-md border border-border bg-surface-2 p-4">
              <span className="text-[12.5px] font-medium text-muted">Токен сброса</span>
              <code className="break-all font-mono text-[13px] leading-[1.5] text-lilac">{token}</code>
              <CopyButton value={token} label="Копировать токен" />
            </div>

            <Link
              to={`/reset?token=${encodeURIComponent(token)}`}
              className="mt-0.5 inline-flex h-[46px] w-full select-none items-center justify-center gap-2 whitespace-nowrap rounded-md border border-transparent bg-violet px-[26px] text-[15px] font-semibold text-white transition-[background-color,border-color,color,filter,opacity] duration-150 "
            >
              Перейти к сбросу пароля
            </Link>

            <div className="mt-0.5 flex flex-col items-center gap-3">
              <Button
                variant="secondary"
                size="lg"
                className="mt-0.5 w-full"
                onClick={() => navigate('/login')}
              >
                Назад к входу
              </Button>
            </div>
          </>
        ) : (
          <>
            <div className="flex flex-col items-center gap-3 py-2.5 text-lilac">
              <KeyRound size={20} />
              <p className="text-center text-[13.5px] leading-[1.55] text-muted">
                Если email зарегистрирован, письмо отправлено
              </p>
            </div>

            <div className="mt-0.5 flex flex-col items-center gap-3">
              <Button
                variant="secondary"
                size="lg"
                className="mt-0.5 w-full"
                onClick={() => navigate('/login')}
              >
                Назад к входу
              </Button>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
