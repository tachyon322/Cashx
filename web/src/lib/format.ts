/** Форматирование денег, ставок, дат и чисел (ru-RU). */

/** Копейки → «12 345 ₽» (минус для отрицательных — автоматически). */
export function formatRubles(kopecks: number): string {
  return `${(kopecks / 100).toLocaleString('ru-RU')} ₽`
}

/** Формат как kazik formatRub — для B2C/Settings. */
export function formatRub(kopecks: number): string {
  return formatRubles(kopecks)
}

/** Базисные пункты → «2,5%». */
export function formatBps(bps: number): string {
  return `${(bps / 100).toLocaleString('ru-RU', { maximumFractionDigits: 2 })}%`
}

const dateFmt = new Intl.DateTimeFormat('ru-RU', {
  day: '2-digit',
  month: '2-digit',
  year: 'numeric',
  timeZone: 'Europe/Moscow',
})

const dateTimeFmt = new Intl.DateTimeFormat('ru-RU', {
  day: '2-digit',
  month: '2-digit',
  year: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
  timeZone: 'Europe/Moscow',
})

/** ISO → «17.08.2026». MSK */
export function formatDate(iso: string): string {
  return dateFmt.format(new Date(iso))
}

/** ISO → «17.08.2026, 14:05». MSK */
export function formatDateTime(iso: string): string {
  return dateTimeFmt.format(new Date(iso))
}

/** Число → «1 234 567». */
export function formatNumber(value: number): string {
  return value.toLocaleString('ru-RU')
}

/* ── MSK helpers как front/components/partner/format.ts ── */

export function toInputDate(d: Date): string {
  // YYYY-MM-DD в Europe/Moscow
  const parts = new Intl.DateTimeFormat('ru-RU', {
    timeZone: 'Europe/Moscow',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).formatToParts(d)
  const y = parts.find((p) => p.type === 'year')?.value ?? '1970'
  const m = parts.find((p) => p.type === 'month')?.value ?? '01'
  const day = parts.find((p) => p.type === 'day')?.value ?? '01'
  return `${y}-${m}-${day}`
}

export function todayStr(): string {
  return toInputDate(new Date())
}

export function addDays(d: Date, n: number): Date {
  const nd = new Date(d)
  nd.setDate(nd.getDate() + n)
  return nd
}

export function formatDayShort(iso: string): string {
  const dt = new Date(iso)
  return new Intl.DateTimeFormat('ru-RU', {
    timeZone: 'Europe/Moscow',
    day: '2-digit',
    month: '2-digit',
  }).format(dt)
}

export function parseRangeMSK(fromStr: string, toStr: string): { from?: string; to?: string } {
  return { from: fromStr || undefined, to: toStr || undefined }
}
