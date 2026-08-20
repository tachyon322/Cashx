import type { components } from '../api/schema'

type DayStats = components['schemas']['DayStats']
type Summary = components['schemas']['SummaryResponse']

/** Готовые данные для одного баннера периода. */
export interface PeriodStats {
  title: string
  subtitle: string
  amountKopecks: number
  /** Рост к предыдущему периоду в %; null — сравнить не с чем (например, «всё время»). */
  growthPercent: number | null
  clicks: number
  registrations: number
  deposits: number
}

function sum(days: DayStats[], key: 'clicks' | 'registrations' | 'first_payments' | 'income_kopecks'): number {
  return days.reduce((acc, d) => acc + (d[key] ?? 0), 0)
}

function growthPercent(cur: number, prev: number): number | null {
  if (prev <= 0) return cur > 0 ? null : 0
  return Math.round(((cur - prev) / prev) * 100)
}

/* ============================================================
   График «Динамика дохода»: агрегация дневных данных
   в периоды по дням / неделям / месяцам.
   ============================================================ */

export type IncomePeriod = 'day' | 'week' | 'month'

export const INCOME_PERIODS: readonly { key: IncomePeriod; label: string }[] = [
  { key: 'day', label: 'По дням' },
  { key: 'week', label: 'По неделям' },
  { key: 'month', label: 'По месяцам' },
]

/** Одна точка графика: короткая подпись для оси X, полная — для тултипа. */
export interface IncomeChartPoint {
  label: string
  tooltipLabel: string
  /** [доход (коп.), депозиты, регистрации] — в порядке серий графика. */
  values: readonly [number, number, number]
}

const pad2 = (n: number): string => String(n).padStart(2, '0')
const fmtDay = (d: Date): string => `${pad2(d.getDate())}.${pad2(d.getMonth() + 1)}`
const isoDay = (d: Date): string => `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`

const MONTHS_SHORT = ['янв', 'фев', 'мар', 'апр', 'май', 'июн', 'июл', 'авг', 'сен', 'окт', 'ноя', 'дек']
const MONTHS_FULL = [
  'Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь',
  'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь',
]

function parseDay(iso: string | undefined): Date | null {
  if (!iso) return null
  const d = new Date(`${iso}T00:00:00`)
  return Number.isNaN(d.getTime()) ? null : d
}

/** Понедельник как начало недели. */
function startOfWeek(d: Date): Date {
  const out = new Date(d)
  out.setDate(d.getDate() - ((d.getDay() + 6) % 7))
  return out
}

function bucketKey(d: Date, period: IncomePeriod): string {
  if (period === 'day') return isoDay(d)
  if (period === 'week') return isoDay(startOfWeek(d))
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}`
}

function labelFor(start: Date, period: IncomePeriod): string {
  if (period === 'day') return fmtDay(start)
  if (period === 'week') return fmtDay(start)
  return `${MONTHS_SHORT[start.getMonth()]} ${String(start.getFullYear()).slice(2)}`
}

function tooltipFor(start: Date, period: IncomePeriod): string {
  if (period === 'day') return `${fmtDay(start)}.${String(start.getFullYear()).slice(2)}`
  if (period === 'week') {
    const end = new Date(start)
    end.setDate(end.getDate() + 6)
    return `${fmtDay(start)} – ${fmtDay(end)}`
  }
  return `${MONTHS_FULL[start.getMonth()]} ${start.getFullYear()}`
}

/**
 * Суммирует дневную статистику по выбранному периоду.
 * Возвращает точки в порядке следования дней (без пропусков).
 */
export function buildIncomeChart(days: readonly DayStats[], period: IncomePeriod): IncomeChartPoint[] {
  const out: IncomeChartPoint[] = []
  let key = ''
  let start: Date | null = null
  let income = 0
  let deposits = 0
  let registrations = 0

  const flush = (): void => {
    if (!start) return
    out.push({
      label: labelFor(start, period),
      tooltipLabel: tooltipFor(start, period),
      values: [income, deposits, registrations],
    })
  }

  for (const day of days) {
    const d = parseDay(day.date)
    if (!d) continue
    const k = bucketKey(d, period)
    if (k !== key) {
      flush()
      key = k
      start = d
      income = 0
      deposits = 0
      registrations = 0
    }
    income += day.income_kopecks ?? 0
    deposits += day.first_payments ?? 0
    registrations += day.registrations ?? 0
  }
  flush()
  return out
}

/** 4 баннера: сегодня / неделя / месяц / всё время. */
export function buildPeriodStats(summary: Summary): PeriodStats[] {
  const chart = summary.chart ?? []
  const income = summary.income ?? {}
  const funnel = summary.funnel ?? {}
  const today = chart[chart.length - 1]
  const yesterday = chart[chart.length - 2]
  const week = chart.slice(-7)
  const prevWeek = chart.slice(-14, -7)
  const month = chart.slice(-30)
  const prevMonth = chart.slice(-60, -30)

  return [
    {
      title: 'Сегодня',
      subtitle: 'заработок за сегодня',
      amountKopecks: income.today_kopecks ?? 0,
      growthPercent: growthPercent(today?.income_kopecks ?? 0, yesterday?.income_kopecks ?? 0),
      clicks: today?.clicks ?? 0,
      registrations: today?.registrations ?? 0,
      deposits: today?.first_payments ?? 0,
    },
    {
      title: 'За неделю',
      subtitle: 'заработок за неделю',
      amountKopecks: income.week_kopecks ?? 0,
      growthPercent: growthPercent(sum(week, 'income_kopecks'), sum(prevWeek, 'income_kopecks')),
      clicks: sum(week, 'clicks'),
      registrations: sum(week, 'registrations'),
      deposits: sum(week, 'first_payments'),
    },
    {
      title: 'За месяц',
      subtitle: 'заработок за месяц',
      amountKopecks: income.month_kopecks ?? 0,
      growthPercent: growthPercent(sum(month, 'income_kopecks'), sum(prevMonth, 'income_kopecks')),
      clicks: sum(month, 'clicks'),
      registrations: sum(month, 'registrations'),
      deposits: sum(month, 'first_payments'),
    },
    {
      title: 'За всё время',
      subtitle: 'заработок за всё время',
      amountKopecks: income.all_kopecks ?? 0,
      growthPercent: null,
      clicks: funnel.clicks ?? 0,
      registrations: funnel.registrations ?? 0,
      deposits: funnel.first_payments ?? 0,
    },
  ]
}
