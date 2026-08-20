import { ApiRequestError } from '../api/client'

const CODE_MESSAGES: Record<string, string> = {
  code_taken: 'Этот код уже занят другим источником',
  invalid_code: 'Код может содержать только A–Z и 0–9 (до 32 символов)',
  has_clicks: 'У источника уже есть переходы — его можно только отключить',
  last_source: 'Нельзя удалить или отключить последний активный источник',
  group_not_empty: 'Поток не пуст — сначала перенесите из него источники',
  group_name_taken: 'Поток с таким названием уже существует',
  source_not_found: 'Источник не найден',
  group_not_found: 'Поток не найден',
  offer_not_joined: 'Оффер не подключён',
}

/** Превращает ошибку API источников/потоков в понятное сообщение. */
export function sourceErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiRequestError) {
    return CODE_MESSAGES[error.message] ?? error.message ?? fallback
  }
  if (error instanceof Error && error.message) {
    return CODE_MESSAGES[error.message] ?? error.message
  }
  return fallback
}
