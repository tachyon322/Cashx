/** Форматирование денег, ставок, дат и чисел (ru-RU). */

/** Копейки → «12 345 ₽» (минус для отрицательных — автоматически). */
export function formatRubles(kopecks: number): string {
  return `${(kopecks / 100).toLocaleString('ru-RU')} ₽`
}

/** Базисные пункты → «2,5%». */
export function formatBps(bps: number): string {
  return `${(bps / 100).toLocaleString('ru-RU', { maximumFractionDigits: 2 })}%`
}

const dateFmt = new Intl.DateTimeFormat('ru-RU', {
  day: '2-digit',
  month: '2-digit',
  year: 'numeric',
})

const dateTimeFmt = new Intl.DateTimeFormat('ru-RU', {
  day: '2-digit',
  month: '2-digit',
  year: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
})

/** ISO → «17.08.2026». */
export function formatDate(iso: string): string {
  return dateFmt.format(new Date(iso))
}

/** ISO → «17.08.2026, 14:05». */
export function formatDateTime(iso: string): string {
  return dateTimeFmt.format(new Date(iso))
}

/** Число → «1 234 567». */
export function formatNumber(value: number): string {
  return value.toLocaleString('ru-RU')
}
