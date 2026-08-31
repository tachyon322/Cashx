import { useState } from 'react'
import { Trophy, TrendingUp, Users, MousePointer } from 'lucide-react'
import { useLeaderboard } from '../../api/queries'
import { Card } from '../../components/Card'
import { EmptyState } from '../../components/EmptyState'
import { Skeleton } from '../../components/Skeleton'
import { Table } from '../../components/Table'
import type { TableColumn } from '../../components/Table'
import { formatRubles } from '../../lib/format'
import type { LeaderboardEntry } from '../../api/queries'

const PERIODS: readonly { key: string; label: string }[] = [
  { key: 'all', label: 'За всё время' },
  { key: 'month', label: 'Месяц' },
  { key: 'week', label: 'Неделя' },
]

const METRICS: readonly { key: string; label: string }[] = [
  { key: 'income', label: 'Доход' },
  { key: 'deposits', label: 'Депозиты' },
  { key: 'signups', label: 'Регистрации' },
  { key: 'clicks', label: 'Клики' },
]

export function LeaderboardPage() {
  const [period, setPeriod] = useState('all')
  const [metric, setMetric] = useState('income')
  const query = useLeaderboard(period, metric)
  const items = query.data?.items ?? []

  const columns: readonly TableColumn<LeaderboardEntry>[] = [
    {
      key: 'rank',
      header: '#',
      render: (row: LeaderboardEntry) => (
        <span className="inline-flex h-6 w-6 items-center justify-center rounded-full bg-violet/12 text-[11px] font-bold text-violet-bright">
          {items.indexOf(row) + 1}
        </span>
      ),
    },
    {
      key: 'name',
      header: 'Партнёр',
      render: (row) => (
        <div className="leading-tight">
          <p className="text-[13px] font-semibold">{row.name}</p>
          <p className="font-mono text-[11px] text-faint">{row.email}</p>
        </div>
      ),
    },
    {
      key: 'clicks',
      header: 'Клики',
      align: 'right',
      render: (row) => <span className="tabular-nums">{row.clicks}</span>,
    },
    {
      key: 'signups',
      header: 'Реги',
      align: 'right',
      render: (row) => <span className="tabular-nums">{row.signups}</span>,
    },
    {
      key: 'deposits',
      header: 'Депозиты',
      align: 'right',
      render: (row) => <span className="tabular-nums">{formatRubles(row.deposits_sum)}</span>,
    },
    {
      key: 'income',
      header: 'Доход',
      align: 'right',
      render: (row) => <span className="font-bold tabular-nums text-violet-bright">{formatRubles(row.income)}</span>,
    },
    {
      key: 'cr',
      header: 'CR',
      align: 'right',
      render: (row) => (row.cr != null ? `${row.cr.toFixed(2)}%` : '—'),
    },
  ]

  if (query.isLoading) {
    return (
      <div className="flex flex-col gap-4">
        <Skeleton style={{ height: 80 }} />
        <Skeleton style={{ height: 420 }} />
      </div>
    )
  }

  if (query.isError) {
    return <EmptyState title="Не удалось загрузить лидерборд" />
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Trophy size={18} className="text-violet-bright" />
          <h2 className="font-display text-[18px] font-bold">Лидерборд</h2>
        </div>
        <div className="flex flex-wrap gap-2">
          <div className="inline-flex rounded-lg border border-[rgba(168,85,247,0.22)] bg-surface-0 p-1">
            {PERIODS.map((p) => (
              <button
                key={p.key}
                onClick={() => setPeriod(p.key)}
                className={`rounded-md px-3 py-1 text-[12px] font-semibold ${period === p.key ? 'bg-violet text-white' : 'text-muted hover:text-text'}`}
              >
                {p.label}
              </button>
            ))}
          </div>
          <div className="inline-flex rounded-lg border border-[rgba(168,85,247,0.22)] bg-surface-0 p-1">
            {METRICS.map((m) => (
              <button
                key={m.key}
                onClick={() => setMetric(m.key)}
                className={`rounded-md px-3 py-1 text-[12px] font-semibold ${metric === m.key ? 'bg-violet text-white' : 'text-muted hover:text-text'}`}
              >
                {m.label}
              </button>
            ))}
          </div>
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-4">
        <div className="flex items-center gap-2 rounded-xl border border-[rgba(168,85,247,0.22)] bg-[#0d0c1c] px-3 py-2">
          <Trophy size={14} className="text-violet-bright" /> <span className="text-[11px] font-semibold uppercase tracking-[0.06em] text-muted">Доход</span>
        </div>
        <div className="flex items-center gap-2 rounded-xl border border-white/10 bg-[#0d0c1c] px-3 py-2">
          <TrendingUp size={14} className="text-success" /> <span className="text-[11px] font-semibold uppercase tracking-[0.06em] text-muted">Депозиты</span>
        </div>
        <div className="flex items-center gap-2 rounded-xl border border-white/10 bg-[#0d0c1c] px-3 py-2">
          <Users size={14} className="text-blue" /> <span className="text-[11px] font-semibold uppercase tracking-[0.06em] text-muted">Регистрации</span>
        </div>
        <div className="flex items-center gap-2 rounded-xl border border-white/10 bg-[#0d0c1c] px-3 py-2">
          <MousePointer size={14} className="text-faint" /> <span className="text-[11px] font-semibold uppercase tracking-[0.06em] text-muted">Клики</span>
        </div>
      </div>

      <Card neon className="overflow-hidden p-0">
        {items.length === 0 ? (
          <div className="p-4">
            <EmptyState title="Пока нет данных" hint="Лидерборд появится после первых депозитов" />
          </div>
        ) : (
          <div className="overflow-x-auto">
            <Table columns={columns} rows={items} rowKey={(r) => r.partner_id} compact />
          </div>
        )}
      </Card>
    </div>
  )
}
