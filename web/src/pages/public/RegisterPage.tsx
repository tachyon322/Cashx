import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { AlertCircle, CheckCircle2, Gift, UserRound } from 'lucide-react'
import { register } from '../../api/auth'
import { ApiRequestError } from '../../api/client'
import { useRegistrationBonus } from '../../api/queries'
import { Button } from '../../components/Button'
import { Field } from '../../components/Field'
import { Input } from '../../components/Input'
import { formatRubles } from '../../lib/format'

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

interface FieldErrors {
  name?: string
  email?: string
  password?: string
  confirm?: string
}

/** Регистрация партнёра; /invite/:code — с предзаполненным referral_code. */
export function RegisterPage() {
  const navigate = useNavigate()
  const { code } = useParams<{ code?: string }>()
  const [search] = useSearchParams()
  const refParam = code ?? search.get('ref')
  const bonusQ = useRegistrationBonus(refParam || undefined)
  const bonus = bonusQ.data?.bonus

  useEffect(() => {
    if (refParam) {
      try {
        localStorage.setItem('aff_ref', refParam)
        document.cookie = `aff_ref=${encodeURIComponent(refParam)}; path=/; max-age=${90 * 86400}; samesite=lax`
      } catch {
        // ignore
      }
    }
  }, [refParam])
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [done, setDone] = useState(false)

  const validate = (): boolean => {
    const next: FieldErrors = {}
    if (name.trim().length === 0) next.name = 'Укажите имя'
    if (!EMAIL_RE.test(email.trim())) next.email = 'Введите корректный email'
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
      await register({
        name: name.trim(),
        email: email.trim(),
        password,
        referral_code: code,
      })
      setDone(true)
    } catch (err) {
      setFormError(
        err instanceof ApiRequestError ? err.message : 'Не удалось отправить заявку. Попробуйте ещё раз.',
      )
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
          <h1 className="text-center text-[19px] font-bold">Заявка отправлена</h1>
          <p className="text-center text-[13.5px] leading-[1.55] text-muted">
            Заявка отправлена, войдите после подтверждения регистрации администратором.
          </p>
          <Button
            size="lg"
            className="mt-0.5 w-full"
            onClick={() => navigate('/login')}
          >
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
        <h1 className="text-center text-[19px] font-bold">Регистрация партнёра</h1>

        {(code || refParam) && (
          <div className="flex items-center gap-3 rounded-md border border-border bg-surface-2 p-4 text-lilac">
            <UserRound size={18} />
            <div className="flex min-w-0 flex-col gap-1">
              <span className="text-[12.5px] font-medium text-muted">Вас пригласил партнёр CashX</span>
              <code className="break-all font-mono text-[14px] font-semibold tracking-[0.04em] text-text">{code ?? refParam}</code>
              {bonus != null && bonus > 0 && (
                <span className="inline-flex items-center gap-1 text-[12px] font-semibold text-violet-bright">
                  <Gift size={14} /> Бонус {formatRubles(bonus * 100)} по промокоду
                </span>
              )}
            </div>
          </div>
        )}

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
          <Field label="Имя" htmlFor="register-name" error={fieldErrors.name}>
            <Input
              id="register-name"
              type="text"
              autoComplete="name"
              placeholder="Иван Петров"
              value={name}
              onChange={(e) => {
                setName(e.target.value)
                if (fieldErrors.name) setFieldErrors((prev) => ({ ...prev, name: undefined }))
              }}
            />
          </Field>
          <Field label="Email" htmlFor="register-email" error={fieldErrors.email}>
            <Input
              id="register-email"
              type="email"
              autoComplete="email"
              placeholder="you@example.com"
              value={email}
              onChange={(e) => {
                setEmail(e.target.value)
                if (fieldErrors.email) setFieldErrors((prev) => ({ ...prev, email: undefined }))
              }}
            />
          </Field>
          <Field
            label="Пароль"
            htmlFor="register-password"
            error={fieldErrors.password}
            hint="Минимум 8 символов"
          >
            <Input
              id="register-password"
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
          <Field label="Повтор пароля" htmlFor="register-confirm" error={fieldErrors.confirm}>
            <Input
              id="register-confirm"
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
            Зарегистрироваться
          </Button>
        </form>

        <div className="mt-0.5 flex flex-col items-center gap-3">
          <span className="text-faint">Уже есть аккаунт?</span>
          <Link to="/login" className="text-[13.5px] text-muted transition-colors duration-150 hover:text-violet-bright">
            Войти
          </Link>
        </div>
      </div>
    </div>
  )
}
