import { useNavigate, useParams } from 'react-router-dom'
import { Download, Lock } from 'lucide-react'
import { ApiRequestError } from '../../api/client'
import { useOfferStats } from '../../api/queries'
import { Badge } from '../../components/Badge'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { EmptyState } from '../../components/EmptyState'
import { Skeleton } from '../../components/Skeleton'
import { StatCard } from '../../components/StatCard'
import { Table } from '../../components/Table'
import type { TableColumn } from '../../components/Table'
import { AreaChart } from '../../components/charts/AreaChart'
import { SourcesCard } from '../../components/SourcesCard'
import { formatBps, formatDateTime, formatNumber, formatRubles } from '../../lib/format'
import { offerStatus } from '../../lib/status'
import type { components } from '../../api/schema'

type HistoryItem = components['schemas']['HistoryItem']
type OfferAgg = components['schemas']['OfferAgg']

/** kind → русская подпись события. */
const KIND_LABELS: Record<string, string> = {
  click: 'Клик',
  registration: 'Регистрация',
  payment: 'Платеж',
  earning: 'Начисление',
  reversal: 'Отмена',
}

const SUMMARY_PERIODS: readonly { key: 'today' | 'week' | 'month' | 'all'; label: string }[] = [
  { key: 'today', label: 'ДОХОД ЗА СЕГОДНЯ' },
  { key: 'week', label: 'ДОХОД ЗА НЕДЕЛЮ' },
  { key: 'month', label: 'ДОХОД ЗА МЕСЯЦ' },
  { key: 'all', label: 'ДОХОД ЗА ВСЕ ВРЕМЯ' },
]

function DetailSkeleton() {
  return (
    <>
      <Skeleton style={{ height: 96 }} />
      <div className="grid grid-cols-1 gap-4 min-[768px]:grid-cols-2 min-[1200px]:grid-cols-4 min-[1920px]:grid-cols-6">
        {Array.from({ length: 6 }, (_, i) => (
          <Skeleton key={i} style={{ height: 96 }} />
        ))}
      </div>
      <Skeleton style={{ height: 280 }} />
      <Skeleton style={{ height: 240 }} />
    </>
  )
}

export function OfferDetailPage() {
  const { offerId = '' } = useParams()
  const navigate = useNavigate()
  const query = useOfferStats(offerId)

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

  const offer = query.data.offer
  const summary = query.data.summary
  const chart = (query.data.chart ?? [])
    .map((point) => ({ date: point.date ?? '', value: point.income_kopecks ?? 0 }))
    .filter((point) => point.date !== '')
  const history = query.data.history ?? []

  const status = offerStatus(offer?.status)

  const historyColumns: readonly TableColumn<HistoryItem>[] = [
    {
      key: 'date',
      header: 'Дата',
      render: (row) => (row.occurred_at ? formatDateTime(row.occurred_at) : '—'),
    },
    {
      key: 'kind',
      header: 'Событие',
      render: (row) => (row.kind ? (KIND_LABELS[row.kind] ?? row.kind) : '—'),
    },
    {
      key: 'amount',
      header: 'Сумма',
      align: 'right',
      render: (row) => (row.amount_kopecks != null ? formatRubles(row.amount_kopecks) : '—'),
    },
  ]

  const subCaption = (agg: OfferAgg | undefined): string => {
    const clicks = formatNumber(agg?.clicks ?? 0)
    const unique = formatNumber(agg?.unique_clicks ?? 0)
    const registrations = formatNumber(agg?.registrations ?? 0)
    return `${clicks} кликов · ${unique} уник. · ${registrations} регистраций`
  }

  return (
    <>
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

      <SourcesCard offerId={offerId} />

      <div className="grid grid-cols-1 gap-4 min-[768px]:grid-cols-2 min-[1200px]:grid-cols-4 min-[1920px]:grid-cols-6">
        {SUMMARY_PERIODS.map((period) => (
          <StatCard
            key={period.key}
            label={period.label}
            value={formatRubles(summary?.[period.key]?.income_kopecks ?? 0)}
            hint={subCaption(summary?.[period.key])}
          />
        ))}
      </div>

      <Card title="Динамика дохода" subtitle="Доход по дням за последний период">
        <AreaChart
          data={chart}
          color="var(--cx-violet-bright)"
          formatValue={(value) => formatRubles(value)}
        />
      </Card>

      <Card title="История" subtitle="Последние события по офферу">
        <Table
          columns={historyColumns}
          rows={history}
          rowKey={(row) => `${row.id ?? ''}${row.kind ?? ''}${row.occurred_at ?? ''}`}
          emptyTitle="История пока пуста"
          emptyHint="События появятся после первых переходов по трекинг-ссылке"
        />
      </Card>
    </>
  )
}