/** Карты статусов домена → русский лейбл + тон бейджа. */

export type Tone = 'success' | 'violet' | 'warning' | 'blue' | 'danger' | 'muted'

export interface StatusInfo {
  label: string
  tone: Tone
}

export const OFFER_STATUS: Record<string, StatusInfo> = {
  active: { label: 'Активен', tone: 'success' },
  available: { label: 'Доступен', tone: 'violet' },
  pending: { label: 'На модерации', tone: 'warning' },
  coming_soon: { label: 'Скоро', tone: 'muted' },
}

export const WITHDRAWAL_STATUS: Record<string, StatusInfo> = {
  pending: { label: 'Ожидает', tone: 'warning' },
  approved: { label: 'Одобрен', tone: 'blue' },
  paid: { label: 'Выплачен', tone: 'success' },
  rejected: { label: 'Отклонён', tone: 'danger' },
  cancelled: { label: 'Отменён', tone: 'muted' },
}

/** Статус оффера с фолбэком на сырое значение. */
export function offerStatus(status?: string | null): StatusInfo {
  return OFFER_STATUS[status ?? ''] ?? { label: status ?? '—', tone: 'muted' }
}

/** Статус заявки на вывод с фолбэком на сырое значение. */
export function withdrawalStatus(status?: string | null): StatusInfo {
  return WITHDRAWAL_STATUS[status ?? ''] ?? { label: status ?? '—', tone: 'muted' }
}
