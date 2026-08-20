import createClient from 'openapi-fetch'
import type { paths } from './schema'

/** Базовый клиент: все запросы идут через Vite-прокси на :8080 (same-origin, cookie сессии). */
export const api = createClient<paths>({ baseUrl: '/api/v1' })

/** Ошибка API с HTTP-статусом и текстом из ответа (или generic). */
export class ApiRequestError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiRequestError'
    this.status = status
  }
}

interface FetchResult<T> {
  data?: T
  error?: unknown
  response: Response
}

/**
 * Разворачивает ответ openapi-fetch: кидает ApiRequestError при не-2xx,
 * иначе возвращает data (для 204 — undefined).
 */
export async function unwrap<T>(request: Promise<FetchResult<T>>): Promise<T> {
  const res = await request
  if (!res.response.ok) {
    const err: unknown = res.error
    let message = `Ошибка запроса (${res.response.status})`
    if (err !== null && typeof err === 'object' && 'message' in err) {
      const raw = err.message
      if (typeof raw === 'string' && raw.trim().length > 0) message = raw
    }
    throw new ApiRequestError(res.response.status, message)
  }
  return res.data as T
}
