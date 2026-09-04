import { useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { Download, Lock, TrendingUp } from 'lucide-react'
import { ApiRequestError } from '../../api/client'
import { useOfferStats, useSources } from '../../api/queries'
import { Badge } from '../../components/Badge'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { EmptyState } from '../../components/EmptyState'
import { RecentActivityCard } from '../../components/RecentActivityCard'
import { Skeleton } from '../../components/Skeleton'
import { StatCard } from '../../components/StatCard'
import { SourcesCard } from '../../components/SourcesCard'
import { MultiLineChart } from '../../components/charts/MultiLineChart'
import type { MultiLineSeries } from '../../components/charts/MultiLineChart'
import { formatBps, formatNumber, formatRubles } from '../../lib/format'
import { buildIncomeChart, INCOME_PERIODS } from '../../lib/stats'
import type { IncomePeriod } from '../../lib/stats'
import { offerStatus } from '../../lib/status'
import { Select } from '../../components/Select'
import type { components } from '../../api/schema'
import { cx } from '../../lib/cx'

type OfferAgg = components['schemas']['OfferAgg']
type OfferStats = components['schemas']['OfferStatsResponse']
type Source = components['schemas']['Source']

const SUMMARY_PERIODS: readonly { key: 'today' | 'week' | 'month' | 'all'; label: string }[] = [
  { key: 'today', label: 'ДОХОД ЗА СЕГОДНЯ' },
  { key: 'week', label: 'ДОХОД ЗА НЕДЕЛЮ' },
  { key: 'month', label: 'ДОХОД ЗА МЕСЯЦ' },
  { key: 'all', label: 'ДОХОД ЗА ВСЕ ВРЕМЯ' },
]

const CHART_SERIES: readonly MultiLineSeries[] = [
  { label: 'Доход', color: 'var(--cx-violet-bright)' },
  { label: 'Депозиты', color: 'var(--cx-blue)', axis: 'right' },
  { label: 'Регистрация', color: 'var(--cx-success)', axis: 'right' },
]

function formatChartValue(value: number, seriesIndex: number): string {
  return seriesIndex === 0 ? formatRubles(value) : formatNumber(value)
}

type ChartMetric = 'all' | 'income' | 'deposits' | 'regs'

const CHART_METRICS: readonly { key: ChartMetric; label: string; indexes: readonly number[] }[] = [
  { key: 'all', label: 'Все', indexes: [0, 1, 2] },
  { key: 'income', label: 'Доход', indexes: [0] },
  { key: 'deposits', label: 'Депозиты', indexes: [1] },
  { key: 'regs', label: 'Регистрации', indexes: [2] },
]

type TopMetric = 'income' | 'clicks' | 'regs'

const TOP_METRICS: readonly { key: TopMetric; label: string }[] = [
  { key: 'income', label: 'Доход' },
  { key: 'clicks', label: 'Перех.' },
  { key: 'regs', label: 'Рег.' },
]

function percent(part: number, whole: number): string {
  if (whole <= 0) return '—'
  return `${((part / whole) * 100).toLocaleString('ru-RU', { maximumFractionDigits: 1 })}%`
}

function DetailSkeleton() {
  return (
    <div className="flex flex-col gap-4 xl:flex-row xl:items-start">
      <div className="min-w-0 flex-1 space-y-4">
        <Skeleton style={{ height: 96 }} />
        <div className="grid grid-cols-1 gap-4 min-[768px]:grid-cols-2 min-[1200px]:grid-cols-4">
          {Array.from({ length: 4 }, (_, i) => (
            <Skeleton key={i} style={{ height: 96 }} />
          ))}
        </div>
        <Skeleton style={{ height: 280 }} />
        <Skeleton style={{ height: 240 }} />
      </div>
      <div className="w-full xl:w-[340px] xl:shrink-0">
        <Skeleton style={{ height: 480 }} />
      </div>
    </div>
  )
}

export function OfferDetailPage() {
  const { offerId = '' } = useParams()
  const navigate = useNavigate()
  const query = useOfferStats(offerId)
  const { data: sourcesData } = useSources(offerId)
  const [period, setPeriod] = useState<IncomePeriod>('day')
  const [chartMetric, setChartMetric] = useState<ChartMetric>('all')
  const [topMetric, setTopMetric] = useState<TopMetric>('income')

  const error = query.error
  if (error instanceof ApiRequestError && (error.status === 403 || error.status === 404)) {
    return (
      <EmptyState
        icon={<Lock size={22} />}
        title="Подключите оффер, чтобы видеть статистику"
        hint="Статистика доступна только по подключённым офферам — найдите его в каталоге и нажмите «Подключиться»"
      >
        <Button variant="primary" onClick={() => navigate('/cabinet/offers')}>
          К списку офферов
        </Button>
      </EmptyState>
    )
  }

  if (query.isLoading) return <DetailSkeleton />

  if (!query.data) {
    return (
      <EmptyState
        title="Не удалось загрузить статистику"
        hint="Попробуйте обновить страницу через несколько секунд"
      />
    )
  }

  return (
    <OfferDetail
      offerId={offerId}
      data={query.data}
      sources={sourcesData?.items ?? []}
      period={period}
      setPeriod={setPeriod}
      chartMetric={chartMetric}
      setChartMetric={setChartMetric}
      topMetric={topMetric}
      setTopMetric={setTopMetric}
    />
  )
}

interface OfferDetailProps {
  offerId: string
  data: OfferStats
  sources: Source[]
  period: IncomePeriod
  setPeriod: (p: IncomePeriod) => void
  chartMetric: ChartMetric
  setChartMetric: (m: ChartMetric) => void
  topMetric: TopMetric
  setTopMetric: (m: TopMetric) => void
}

function OfferDetail({
  offerId,
  data,
  sources,
  period,
  setPeriod,
  chartMetric,
  setChartMetric,
  topMetric,
  setTopMetric,
}: OfferDetailProps) {
  const offer = data.offer
  const summary = data.summary

  const chartPoints = useMemo(() => buildIncomeChart(data.chart ?? [], period), [data.chart, period])
  const metricIndexes = CHART_METRICS.find((m) => m.key === chartMetric)?.indexes ?? [0, 1, 2]
  const series = useMemo(
    () => CHART_SERIES.filter((_, i) => metricIndexes.includes(i)),
    [metricIndexes],
  )
  const points = useMemo(
    () => chartPoints.map((p) => ({ ...p, values: p.values.filter((_, i) => metricIndexes.includes(i)) })),
    [chartPoints, metricIndexes],
  )

  const month = summary?.month
  const monthClicks = month?.clicks ?? 0
  const monthRegs = month?.registrations ?? 0
  const monthDeps = month?.first_payments ?? 0
  const monthIncome = month?.income_kopecks ?? 0
  const conversions = [
    {
      label: 'Конверсия (рег/клики)',
      value: percent(monthRegs, monthClicks),
      hint: `${formatNumber(monthRegs)} рег из ${formatNumber(monthClicks)} переходов`,
    },
    {
      label: 'Доход с регистрации',
      value: monthRegs > 0 ? formatRubles(Math.floor(monthIncome / monthRegs)) : '—',
      hint: `${formatRubles(monthIncome)} / ${formatNumber(monthRegs)} рег`,
    },
    {
      label: 'Платёжная конверсия (депозиты/рег)',
      value: percent(monthDeps, monthRegs),
      hint: `${formatNumber(monthDeps)} депозитов из ${formatNumber(monthRegs)} рег`,
    },
  ]

  const topSources = useMemo(() => {
    const valueOf = (s: Source): number => {
      const t = (s.totals ?? {}) as { clicks?: number; registrations?: number; income_kopecks?: number }
      if (topMetric === 'clicks') return t.clicks ?? 0
      if (topMetric === 'regs') return t.registrations ?? 0
      return t.income_kopecks ?? 0
    }
    return [...sources]
      .sort((a, b) => valueOf(b) - valueOf(a))
      .slice(0, 5)
      .map((s) => ({ id: s.id ?? s.name ?? s.code ?? '—', name: s.name ?? s.code ?? '—', value: valueOf(s) }))
  }, [sources, topMetric])

  const status = offerStatus(offer?.status)

  const subCaption = (agg: OfferAgg | undefined): string => {
    const clicks = formatNumber(agg?.clicks ?? 0)
    const unique = formatNumber(agg?.unique_clicks ?? 0)
    const registrations = formatNumber(agg?.registrations ?? 0)
    return `${clicks} кликов · ${unique} уник. · ${registrations} регистраций`
  }

  return (
    <div className="flex flex-col gap-4 xl:flex-row xl:items-start">
      <div className="min-w-0 flex-1 space-y-4">
        <div className="flex flex-wrap items-center gap-4 rounded-lg border border-border bg-surface-1 p-4 shadow-card">
          <div className="flex min-w-0 flex-1 flex-col gap-1">
            {offer?.project_name && (
              <span className="text-[10.5px] font-semibold uppercase leading-[1.4] tracking-[0.08em] text-muted">
                {offer.project_name}
              </span>
            )}
            <h2 className="truncate font-display text-[20px] font-semibold leading-[1.25]">
              {offer?.name ?? 'Оффер'}
            </h2>
          </div>
          <div className="flex items-center gap-3">
            <Badge tone={status.tone}>{status.label}</Badge>
            {offer?.current_rate_bps != null && (
              <span className="text-[15px] font-bold text-violet-bright">
                {formatBps(offer.current_rate_bps)}
              </span>
            )}
          </div>
          <a
            className="inline-flex items-center gap-2 rounded-md border border-border px-3.5 py-2 text-[13px] font-semibold text-text transition-colors duration-150 hover:border-border-active hover:bg-surface-hover"
            href={`/api/v1/cabinet/offers/${offerId}/history.csv`}
            download
          >
            <Download size={14} />
            Скачать CSV
          </a>
        </div>

        <div className="grid grid-cols-1 gap-4 min-[768px]:grid-cols-2 min-[1200px]:grid-cols-4">
          {SUMMARY_PERIODS.map((p) => (
            <StatCard
              key={p.key}
              label={p.label}
              value={formatRubles(summary?.[p.key]?.income_kopecks ?? 0)}
              hint={subCaption(summary?.[p.key])}
            />
          ))}
        </div>

        <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
          {conversions.map((c) => (
            <StatCard
              key={c.label}
              label={c.label}
              value={c.value}
              hint={c.hint}
              icon={<TrendingUp size={15} />}
            />
          ))}
        </div>

        <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
          <Card
            neon
            className="overflow-hidden lg:col-span-2"
            title={<span className="text-[12px] font-bold uppercase tracking-[0.08em]">Статистика</span>}
            actions={
              <div className="flex flex-wrap items-center gap-2">
                <div className="flex items-center gap-1">
                  {CHART_METRICS.map((m) => (
                    <button
                      key={m.key}
                      type="button"
                      onClick={() => setChartMetric(m.key)}
                      className={cx(
                        'rounded-md px-2.5 py-1 text-[11px] font-semibold transition-colors',
                        chartMetric === m.key
                          ? 'bg-violet/15 text-violet-bright'
                          : 'text-faint hover:bg-white/[0.04] hover:text-text',
                      )}
                    >
                      {m.label}
                    </button>
                  ))}
                </div>
                <Select
                  value={period}
                  onChange={(event) => setPeriod(event.target.value as IncomePeriod)}
                  className="h-8 w-[124px] rounded-md border-[rgba(168,85,247,0.28)] bg-surface-0 text-[12px]"
                  aria-label="Период графика"
                >
                  {INCOME_PERIODS.map((option) => (
                    <option key={option.key} value={option.key}>
                      {option.label}
                    </option>
                  ))}
                </Select>
              </div>
            }
          >
            <MultiLineChart data={points} series={series} formatValue={formatChartValue} />
          </Card>

          <Card
            neon
            className="flex flex-col overflow-hidden p-0"
            title={<span className="px-4 pt-4 text-[12px] font-bold uppercase tracking-[0.08em]">Топ источников</span>}
            actions={
              <div className="flex items-center gap-1">
                {TOP_METRICS.map((m) => (
                  <button
                    key={m.key}
                    type="button"
                    onClick={() => setTopMetric(m.key)}
                    className={cx(
                      'rounded-md px-2 py-1 text-[11px] font-semibold transition-colors',
                      topMetric === m.key
                        ? 'bg-violet/15 text-violet-bright'
                        : 'text-faint hover:bg-white/[0.04] hover:text-text',
                    )}
                  >
                    {m.label}
                  </button>
                ))}
              </div>
            }
          >
            {topSources.length === 0 ? (
              <div className="px-4 pb-4">
                <EmptyState title="Активности пока нет" />
              </div>
            ) : (
              <div className="flex flex-col divide-y divide-[rgba(168,85,247,0.12)]">
                {topSources.map((s, i) => (
                  <div key={s.id} className="flex items-center gap-2.5 px-4 py-3">
                    <span
                      className={cx(
                        'inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-full border text-[12px] font-bold',
                        i === 0
                          ? 'border-amber-400/40 bg-amber-400/15 text-amber-300'
                          : 'border-white/10 bg-white/[0.06] text-muted',
                      )}
                    >
                      {i + 1}
                    </span>
                    <span className="min-w-0 flex-1 truncate text-[13px] font-semibold">{s.name}</span>
                    <span className="shrink-0 text-[13px] font-bold tabular-nums">
                      {topMetric === 'income' ? formatRubles(s.value) : formatNumber(s.value)}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </Card>
        </div>

        <SourcesCard offerId={offerId} />
      </div>

      <RecentActivityCard
        className="w-full xl:w-[340px] xl:shrink-0"
        offerId={offerId}
      />
    </div>
  )
}
