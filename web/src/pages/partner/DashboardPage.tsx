import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Link2, Play, TrendingUp, Search, Filter, BarChart3, Copy, Pencil } from 'lucide-react'
import { CalendarDays, Boxes, Layers3 } from 'lucide-react'
import { useAllSources, useOffers, useSummary } from '../../api/queries'
import { AdvBanner, HeroNeonArt } from '../../components/AdvBanner'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { EmptyState } from '../../components/EmptyState'
import { Select } from '../../components/Select'
import { StatsBanner } from '../../components/StatsBanner'
import { TopSourcesCard } from '../../components/TopSourcesCard'
import type { TopSourceItem } from '../../components/TopSourcesCard'
import { MultiLineChart } from '../../components/charts/MultiLineChart'
import type { MultiLineSeries } from '../../components/charts/MultiLineChart'
import { SymmetricFunnel } from '../../components/charts/SymmetricFunnel'
import type { SymmetricFunnelItem } from '../../components/charts/SymmetricFunnel'
import { useToast } from '../../components/Toast'
import { formatNumber, formatRubles } from '../../lib/format'
import { buildIncomeChart, buildPeriodStats, INCOME_PERIODS } from '../../lib/stats'
import type { IncomePeriod } from '../../lib/stats'
import type { components } from '../../api/schema'

type SummaryResponse = components['schemas']['SummaryResponse']

const INCOME_SERIES: readonly MultiLineSeries[] = [
  { label: 'Доход', color: 'var(--cx-violet-bright)' },
  { label: 'Депозиты', color: 'var(--cx-blue)', axis: 'right' },
  { label: 'Регистрация', color: 'var(--cx-success)', axis: 'right' },
]

function formatIncomeValue(value: number, seriesIndex: number): string {
  return seriesIndex === 0 ? formatRubles(value) : formatNumber(value)
}

const FUNNEL_COLORS = {
  clicks: 'var(--cx-violet-bright)',
  registrations: 'var(--cx-blue)',
  deposits: 'var(--cx-success)',
  income: 'var(--cx-magenta)',
} as const

function conversionRate(part: number, whole: number): string | null {
  if (whole <= 0) return null
  return `${((part / whole) * 100).toLocaleString('ru-RU', { maximumFractionDigits: 1 })}%`
}

function buildFunnelItems(summary: SummaryResponse): SymmetricFunnelItem[] {
  const f = summary.funnel ?? {}
  const clicks = f.clicks ?? 0
  const registrations = f.registrations ?? 0
  const deposits = f.first_payments ?? 0
  const income = f.income_kopecks ?? 0
  const regRate = conversionRate(registrations, clicks)
  const depRate = conversionRate(deposits, registrations)
  return [
    { label: 'Переходы', value: clicks, color: FUNNEL_COLORS.clicks, formatValue: formatNumber },
    {
      label: 'Регистрации',
      value: registrations,
      color: FUNNEL_COLORS.registrations,
      formatValue: formatNumber,
      hint: regRate != null ? `${regRate} от переходов` : undefined,
    },
    {
      label: 'Депозиты',
      value: deposits,
      color: FUNNEL_COLORS.deposits,
      formatValue: formatNumber,
      hint: depRate != null ? `${depRate} от регистраций` : undefined,
    },
    { label: 'Доход', value: income, color: FUNNEL_COLORS.income, formatValue: formatRubles },
  ]
}

const STATS_ICONS: Record<string, React.ReactNode> = {
  Сегодня: <TrendingUp size={15} />,
  'За неделю': <CalendarDays size={15} />,
  'За месяц': <Boxes size={15} />,
  'За всё время': <Layers3 size={15} />,
}

function mapTitleToRef(title: string): string {
  if (title === 'Сегодня') return 'Сегодня'
  if (title === 'За неделю') return '7 дней'
  if (title === 'За месяц') return '30 дней'
  if (title === 'За всё время') return 'Всего'
  return title
}

function SourceIcon({ kind }: { kind: string }) {
  const cls = 'inline-flex h-7 w-7 items-center justify-center rounded-full text-[11px] font-bold text-white'
  if (kind === 'tg')
    return <span className={`${cls} bg-[#229ED9]`}>✈</span>
  if (kind === 'vk')
    return <span className={`${cls} bg-[#07F]`}>VK</span>
  return <span className={`${cls} bg-[#111] border border-white/10`}>M</span>
}

function iconKindFromName(name: string): string {
  const lower = name.toLowerCase()
  if (lower.includes('tg') || lower.includes('telegram') || lower.includes('тг')) return 'tg'
  if (lower.includes('vk') || lower.includes('вк')) return 'vk'
  return 'mm'
}

export function DashboardPage() {
  const navigate = useNavigate()
  const toast = useToast()
  const { data: summary } = useSummary()
  const { data: offersData } = useOffers()
  const { data: allSources, isLoading: sourcesLoading, isError: sourcesError } = useAllSources()
  const [period, setPeriod] = useState<IncomePeriod>('day')
  const [trafficSearch, setTrafficSearch] = useState('')

  const chartData = useMemo(() => (summary ? buildIncomeChart(summary.chart ?? [], period) : []), [summary, period])
  const funnelItems = useMemo(() => (summary ? buildFunnelItems(summary) : []), [summary])
  const periodStats = useMemo(() => (summary ? buildPeriodStats(summary) : []), [summary])

  const hasOffers = (offersData?.items?.length ?? 0) > 0

  const trafficRows = useMemo(() => {
    if (!allSources) return []
    return allSources
      .map((s) => {
        const t = s.totals ?? {}
        const clicks = (t as { clicks?: number }).clicks ?? 0
        const regs = (t as { registrations?: number }).registrations ?? 0
        const deps = (t as { first_payments?: number }).first_payments ?? 0
        const income = (t as { income_kopecks?: number }).income_kopecks ?? 0
        const cr = clicks > 0 ? `${((regs / clicks) * 100).toFixed(1)}%` : '—'
        const name = s.name ?? s.code ?? '—'
        return {
          id: s.id ?? name,
          kind: iconKindFromName(name),
          name,
          url: s.url ?? '',
          promo: s.code ?? '—',
          clicks,
          regs,
          deps,
          income,
          cr,
        }
      })
      .sort((a, b) => b.income - a.income || b.clicks - a.clicks)
  }, [allSources])

  const filteredRows = useMemo(() => {
    const q = trafficSearch.trim().toLowerCase()
    if (!q) return trafficRows
    return trafficRows.filter(
      (r) => r.name.toLowerCase().includes(q) || r.promo.toLowerCase().includes(q) || r.url.toLowerCase().includes(q),
    )
  }, [trafficRows, trafficSearch])

  const topSources: TopSourceItem[] = useMemo(
    () =>
      trafficRows.slice(0, 5).map((r, i) => ({
        rank: i + 1,
        name: r.name,
        sub: `${formatNumber(r.clicks)} переходов · ${r.deps} депозитов`,
        incomeKopecks: r.income,
      })),
    [trafficRows],
  )

  const topSourcesLoading = summary == null || (hasOffers && sourcesLoading)

  return (
    <div className="flex flex-col gap-4">
      <div className="grid items-stretch gap-4">
        <AdvBanner
          title={
            <>
              Зарабатывайте больше
              <br />
              с <span className="text-violet-bright">Cashx</span>Pay
            </>
          }
          description="Получайте до 40% комиссии с каждого привлечённого игрока пожизненно. Прозрачная статистика и быстрые выплаты."
          actions={
            <>
              <Button
                size="md"
                className="h-[38px] rounded-md bg-violet px-5 text-white shadow-[0_0_18px_rgba(121,40,255,0.35)]  btn-bevel-light"
                onClick={() => navigate('/cabinet/offers')}
              >
                <Link2 size={16} />
                Создать ссылку
              </Button>
              <Button
                size="md"
                variant="secondary"
                className="h-[38px] rounded-md border-white/10 bg-white/[0.06] text-white hover:bg-white/10"
                onClick={() => navigate('/cabinet/offers')}
              >
                <Play size={14} className="fill-white" />
                Как это работает?
              </Button>
            </>
          }
          media={<HeroNeonArt />}
        />
        {/* <RevShareCard revsharePercentBps={summary?.revshare_percent_bps} onDetails={() => navigate('/cabinet/offers')} /> */}
      </div>

      <div className="grid items-stretch gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {summary == null
          ? Array.from({ length: 4 }, (_, i) => (
              <div key={i} className="h-[172px] animate-pulse rounded-xl border border-border bg-surface-1" />
            ))
          : periodStats.map((stats) => {
              const refTitle = mapTitleToRef(stats.title)
              const icon = STATS_ICONS[stats.title]
              return <StatsBanner key={stats.title} stats={{ ...stats, title: refTitle.toUpperCase(), subtitle: '' }} icon={icon} />
            })}
      </div>

      <div className="grid items-stretch gap-4 xl:grid-cols-[1.25fr_0.7fr_0.7fr]">
        {summary == null ? (
          <>
            <div className="h-[340px] animate-pulse rounded-xl border border-border bg-surface-1" />
            <div className="h-[340px] animate-pulse rounded-xl border border-border bg-surface-1" />
            <div className="h-[340px] animate-pulse rounded-xl border border-border bg-surface-1" />
          </>
        ) : (
          <>
            <Card
              neon
              title={<span className="text-[12px] font-bold uppercase tracking-[0.08em]">Динамика дохода</span>}
              subtitle={
                <span className="hidden items-center gap-2 text-[11px] md:inline-flex">
                  <span className="inline-flex items-center gap-1.5">
                    <span className="h-1.5 w-1.5 rounded-full bg-violet-bright" /> Доход
                  </span>
                  <span className="inline-flex items-center gap-1.5">
                    <span className="h-1.5 w-1.5 rounded-full bg-blue" /> Депозиты
                  </span>
                  <span className="inline-flex items-center gap-1.5">
                    <span className="h-1.5 w-1.5 rounded-full bg-success" /> Регистрации
                  </span>
                </span>
              }
              actions={
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
              }
              className="overflow-hidden"
            >
              <MultiLineChart data={chartData} series={INCOME_SERIES} formatValue={formatIncomeValue} />
            </Card>

            <Card
              neon
              title={<span className="text-[12px] font-bold uppercase tracking-[0.08em]">Воронка конверсии</span>}
              className="overflow-hidden"
            >
              <SymmetricFunnel items={funnelItems} />
            </Card>

            <TopSourcesCard items={topSources} isLoading={topSourcesLoading} onViewAll={() => navigate('/cabinet/offers')} />
          </>
        )}
      </div>

      <Card
        neon
        className="overflow-hidden p-0"
        title={
          <span className="flex items-center gap-3 text-[12px] font-bold uppercase tracking-[0.08em]">
            Источники трафика
            <span
              onClick={() => navigate('/cabinet/offers')}
              className="inline-flex cursor-pointer items-center gap-1.5 rounded-md bg-violet px-3 py-1 text-[11px] font-bold normal-case tracking-normal text-white "
            >
              + Создать ссылку
            </span>
          </span>
        }
        actions={
          <div className="hidden items-center gap-2 md:flex">
            <div className="relative">
              <Search size={14} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
              <input
                placeholder="Поиск источника"
                value={trafficSearch}
                onChange={(e) => setTrafficSearch(e.target.value)}
                className="h-8 w-[200px] rounded-md border border-[rgba(168,85,247,0.22)] bg-surface-0 py-1 pl-8 pr-3 text-[12px] placeholder:text-faint focus:border-[rgba(168,85,247,0.45)] focus:outline-none"
              />
            </div>
            <button className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-[rgba(168,85,247,0.22)] bg-surface-0 text-faint hover:text-text">
              <Filter size={14} />
            </button>
          </div>
        }
      >
        <div className="overflow-x-auto">
          <table className="w-full min-w-[760px] border-collapse text-left">
            <thead>
              <tr className="border-b border-[rgba(168,85,247,0.14)] text-[10px] font-semibold uppercase tracking-[0.06em] text-faint">
                <th className="px-4 py-3 font-semibold">Источник</th>
                <th className="px-4 py-3 font-semibold">Ссылка / Промокод</th>
                <th className="px-4 py-3 text-right font-semibold">Переходы</th>
                <th className="px-4 py-3 text-right font-semibold">Регистрации</th>
                <th className="px-4 py-3 text-right font-semibold">Депозиты</th>
                <th className="px-4 py-3 text-right font-semibold">Доход</th>
                <th className="px-4 py-3 text-right font-semibold">CR</th>
                <th className="px-4 py-3 text-right font-semibold">Действия</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[rgba(168,85,247,0.08)]">
              {sourcesLoading ? (
                Array.from({ length: 3 }, (_, i) => (
                  <tr key={i} className="text-[13px]">
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2.5">
                        <span className="h-7 w-7 animate-pulse rounded-full bg-white/10" />
                        <span className="h-3 w-20 animate-pulse rounded bg-white/10" />
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="h-3 w-40 animate-pulse rounded bg-white/10" />
                    </td>
                    <td colSpan={6} className="px-4 py-3">
                      <div className="h-3 w-full animate-pulse rounded bg-white/5" />
                    </td>
                  </tr>
                ))
              ) : sourcesError ? (
                <tr>
                  <td colSpan={8} className="px-4 py-8 text-center text-[13px] text-danger">
                    Не удалось загрузить источники
                  </td>
                </tr>
              ) : filteredRows.length === 0 ? (
                <tr>
                  <td colSpan={8} className="px-4 py-6">
                    {trafficRows.length === 0 ? (
                      <EmptyState
                        title={hasOffers ? 'Нет источников' : 'Нет подключённых офферов'}
                        hint={
                          hasOffers
                            ? 'Создайте источник в офферах — статистика появится здесь'
                            : 'Подключите оффер, чтобы создать первую ссылку'
                        }
                      />
                    ) : (
                      <EmptyState title="Ничего не найдено" hint={`По запросу «${trafficSearch}» источников нет`} />
                    )}
                  </td>
                </tr>
              ) : (
                filteredRows.map((row) => (
                  <tr key={row.id} className="text-[13px] hover:bg-white/[0.02]">
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2.5">
                        <SourceIcon kind={row.kind} />
                        <span className="font-semibold">{row.name}</span>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex flex-col leading-tight">
                        <span className="truncate font-mono text-[12px] text-violet-bright" title={row.url}>
                          {row.url || '—'}
                        </span>
                        <span className="text-[11px] text-faint">Промокод: {row.promo}</span>
                      </div>
                    </td>
                    <td className="px-4 py-3 text-right tabular-nums">{formatNumber(row.clicks)}</td>
                    <td className="px-4 py-3 text-right tabular-nums">{formatNumber(row.regs)}</td>
                    <td className="px-4 py-3 text-right tabular-nums">{formatNumber(row.deps)}</td>
                    <td className="px-4 py-3 text-right font-bold tabular-nums">{formatRubles(row.income)}</td>
                    <td className="px-4 py-3 text-right tabular-nums text-muted">{row.cr}</td>
                    <td className="px-4 py-3">
                      <div className="flex justify-end gap-1">
                        <button
                          title="Статистика"
                          className="inline-flex h-7 w-7 items-center justify-center rounded-md border border-transparent text-faint hover:bg-surface-hover hover:text-text"
                          onClick={() => navigate('/cabinet/offers')}
                        >
                          <BarChart3 size={14} />
                        </button>
                        <button
                          title="Копировать ссылку"
                          className="inline-flex h-7 w-7 items-center justify-center rounded-md border border-transparent text-faint hover:bg-surface-hover hover:text-text"
                          onClick={() => {
                            if (row.url) {
                              void navigator.clipboard.writeText(row.url)
                              toast.success('Ссылка скопирована')
                            }
                          }}
                        >
                          <Copy size={14} />
                        </button>
                        <button
                          title="К офферам"
                          className="inline-flex h-7 w-7 items-center justify-center rounded-md border border-transparent text-faint hover:bg-surface-hover hover:text-text"
                          onClick={() => navigate('/cabinet/offers')}
                        >
                          <Pencil size={14} />
                        </button>
                        <button className="inline-flex h-7 w-7 items-center justify-center rounded-md border border-transparent text-faint hover:text-text">
                          ⋯
                        </button>
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </Card>
    </div>
  )
}
