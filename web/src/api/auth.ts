import { ApiRequestError } from './client'

const GENERIC_ERROR = 'Не удалось выполнить запрос. Попробуйте ещё раз.'

/**
 * Лимин может отдавать ошибки в разном формате — пробуем вытащить `{message}`,
 * при неудачном парсинге показываем generic-текст.
 */
async function parseError(res: Response): Promise<string> {
  try {
    const body: unknown = await res.json()
    if (body !== null && typeof body === 'object' && 'message' in body) {
      const message = body.message
      if (typeof message === 'string' && message.trim().length > 0) return message
    }
  } catch {
    // тело не JSON — ниже generic
  }
  return res.status === 401 ? 'Неверный email или пароль' : GENERIC_ERROR
}

/** POST /api/v1/auth/signin/credential (роут вне OpenAPI, Limen). */
export async function signin(credential: string, password: string): Promise<void> {
  const res = await fetch('/api/v1/auth/signin/credential', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'same-origin',
    body: JSON.stringify({ credential, password }),
  })
  if (!res.ok) {
    throw new ApiRequestError(res.status, await parseError(res))
  }
}

/** POST /api/v1/auth/signout → 204 (вне OpenAPI). */
export async function signout(): Promise<void> {
  try {
    await fetch('/api/v1/auth/signout', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'same-origin',
    })
  } catch {
    // best-effort: сессия может быть уже недействительна
  }
}

/** POST /api/v1/auth/register → 201 (заявка отправлена, ждёт модерации). */
export async function register(input: {
  name: string
  email: string
  password: string
  referral_code?: string
}): Promise<void> {
  const res = await fetch('/api/v1/auth/register', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'same-origin',
    body: JSON.stringify(input),
  })
  if (!res.ok) {
    throw new ApiRequestError(res.status, await parseError(res))
  }
}

/** Ответ POST /auth/password-reset/request (202): токен возвращается только в dev-режиме. */
export interface PasswordResetRequestResult {
  reset_token?: string | null
}

/** POST /api/v1/auth/password-reset/request → всегда 202. */
export async function requestPasswordReset(email: string): Promise<PasswordResetRequestResult> {
  const res = await fetch('/api/v1/auth/password-reset/request', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'same-origin',
    body: JSON.stringify({ email }),
  })
  if (!res.ok) {
    throw new ApiRequestError(res.status, await parseError(res))
  }
  const body = (await res.json().catch(() => null)) as PasswordResetRequestResult | null
  return body ? { reset_token: body.reset_token } : {}
}

/** POST /api/v1/auth/password-reset/confirm → 204. */
export async function confirmPasswordReset(token: string, newPassword: string): Promise<void> {
  const res = await fetch('/api/v1/auth/password-reset/confirm', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'same-origin',
    body: JSON.stringify({ token, new_password: newPassword }),
  })
  if (!res.ok) {
    throw new ApiRequestError(res.status, await parseError(res))
  }
}
