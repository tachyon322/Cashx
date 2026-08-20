import { useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { AlertCircle, CheckCircle2 } from 'lucide-react'
import { confirmPasswordReset } from '../../api/auth'
import { ApiRequestError } from '../../api/client'
import { Button } from '../../components/Button'
import { Field } from '../../components/Field'
import { Input } from '../../components/Input'

interface FieldErrors {
  token?: string
  password?: string
  confirm?: string
}

/** Установка нового пароля по токену из ?token= (или из поля ввода). */
export function ResetPage() {
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const urlToken = params.get('token') ?? ''
  const [token, setToken] = useState(urlToken)
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [done, setDone] = useState(false)

  const validate = (): boolean => {
    const next: FieldErrors = {}
    if (token.trim().length === 0) next.token = 'Укажите токен сброса'
    if (password.length < 8) next.password = 'Пароль должен быть не короче 8 символов'
    if (confirm !== password) next.confirm = 'Пароли не совпадают'
    setFieldErrors(next)
    return Object.keys(next).length === 0
  }

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (loading) return
    setFormError(null)
    if (!validate()) return
    setLoading(true)
    try {
      await confirmPasswordReset(token.trim(), password)
      setDone(true)
    } catch (err) {
      setFormError(err instanceof ApiRequestError ? err.message : 'Не удалось сменить пароль. Попробуйте ещё раз.')
      setLoading(false)
    }
  }

  if (done) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-bg bg-hero-glow p-4">
        <div className="flex w-full max-w-[420px] flex-col items-center gap-4 rounded-lg border border-border bg-surface-1 p-4 text-center shadow-card">
          <div className="mx-auto mt-1 flex h-14 w-14 items-center justify-center rounded-[16px] bg-success/12 text-success">
            <CheckCircle2 size={28} />
          </div>
          <h1 className="text-center text-[19px] font-bold">Пароль изменён</h1>
          <p className="text-center text-[13.5px] leading-[1.55] text-muted">
            Теперь вы можете войти с новым паролем.
          </p>
          <Button size="lg" className="mt-0.5 w-full" onClick={() => navigate('/login')}>
            Войти
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-bg bg-hero-glow p-4">
      <div className="flex w-full max-w-[420px] flex-col gap-4 rounded-lg border border-border bg-surface-1 p-4 shadow-card">
        <div className="flex items-center justify-center gap-3 font-display text-[24px] font-bold">
          CashX
          <span className="h-2 w-2 rounded-full bg-violet shadow-[0_0_14px_rgba(168,85,247,0.9)]" />
        </div>
        <h1 className="text-center text-[19px] font-bold">Новый пароль</h1>

        {formError && (
          <div
            className="flex items-start gap-2 rounded-md border border-danger/35 bg-danger/10 px-3 py-2.5 text-[13px] leading-[1.45] text-danger [&>svg]:mt-px [&>svg]:shrink-0"
            role="alert"
          >
            <AlertCircle size={16} />
            <span>{formError}</span>
          </div>
        )}

        <form className="flex flex-col gap-4" onSubmit={(e) => void onSubmit(e)} noValidate>
          {!urlToken && (
            <Field label="Токен сброса" htmlFor="reset-token" error={fieldErrors.token}>
              <Input
                id="reset-token"
                type="text"
                autoComplete="one-time-code"
                placeholder="Токен из письма"
                value={token}
                onChange={(e) => {
                  setToken(e.target.value)
                  if (fieldErrors.token) setFieldErrors((prev) => ({ ...prev, token: undefined }))
                }}
              />
            </Field>
          )}
          <Field
            label="Новый пароль"
            htmlFor="reset-password"
            error={fieldErrors.password}
            hint="Минимум 8 символов"
          >
            <Input
              id="reset-password"
              type="password"
              autoComplete="new-password"
              placeholder="••••••••"
              value={password}
              onChange={(e) => {
                setPassword(e.target.value)
                if (fieldErrors.password) setFieldErrors((prev) => ({ ...prev, password: undefined }))
              }}
            />
          </Field>
          <Field label="Повтор пароля" htmlFor="reset-confirm" error={fieldErrors.confirm}>
            <Input
              id="reset-confirm"
              type="password"
              autoComplete="new-password"
              placeholder="••••••••"
              value={confirm}
              onChange={(e) => {
                setConfirm(e.target.value)
                if (fieldErrors.confirm) setFieldErrors((prev) => ({ ...prev, confirm: undefined }))
              }}
            />
          </Field>

          <Button type="submit" size="lg" loading={loading} className="mt-0.5 w-full">
            Сохранить
          </Button>
        </form>

        <div className="mt-0.5 flex flex-col items-center gap-3">
          <Link to="/login" className="text-[13.5px] text-muted transition-colors duration-150 hover:text-violet-bright">
            Назад к входу
          </Link>
        </div>
      </div>
    </div>
  )
}
