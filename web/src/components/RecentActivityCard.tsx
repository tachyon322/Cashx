import { useMemo, useState } from 'react'
import {
  Download,
  History,
  MousePointerClick,
  RotateCcw,
  Search,
  TrendingUp,
  UserPlus,
  Wallet,
  type LucideIcon,
} from 'lucide-react'
import { Card } from './Card'
import { EmptyState } from './EmptyState'
import type { ActivityFeedKind, ActivityKind } from '../api/queries'
import { useRecentActivity } from '../api/queries'
import { formatDateTime, formatRubles, todayStr } from '../lib/format'
import { cx } from '../lib/cx'

type ActivityFilter = 'all' | ActivityFeedKind

const KIND_META: Record<ActivityKind, { label: string; icon: LucideIcon; tile: string; iconClass: string }> = {
  click: { label: 'Переход', icon: MousePointerClick, tile: 'bg-amber-400/15', iconClass: 'text-amber-300' },
  registration: { label: 'Регистрация', icon: UserPlus, tile: 'bg-blue-500/15', iconClass: 'text-blue-300' },
  payment: { label: 'Депозит', icon: TrendingUp, tile: 'bg-emerald-500/15', iconClass: 'text-emerald-300' },
  earning: { label: 'Начисление', icon: Wallet, tile: 'bg-violet-500/15', iconClass: 'text-violet-300' },
  reversal: { label: 'Сторно', icon: RotateCcw, tile: 'bg-red-500/15', iconClass: 'text-red-300' },
}

const FILTERS: ReadonlyArray<{ label: string; value: ActivityFilter }> = [
  { label: 'Все', value: 'all' },
  { label: 'Доход', value: 'income' },
  { label: 'Регистрации', value: 'signups' },
  { label: 'Переходы', value: 'clicks' },
]

function hasAmount(kind: ActivityKind): boolean {
  return kind === 'payment' || kind === 'earning' || kind === 'reversal'
}

function formatAmount(kopecks: number): string {
  if (kopecks < 0) return formatRubles(kopecks)
  return `+${formatRubles(kopecks)}`
}

function filterToKind(filter: ActivityFilter): ActivityFeedKind | undefined {
  return filter === 'all' ? undefined : filter
}

interface RecentActivityCardProps {
  offerId?: string
  limit?: number
  className?: string
}

export function RecentActivityCard({ offerId, limit = 50, className }: RecentActivityCardProps) {
  const [search, setSearch] = useState('')
  const [filter, setFilter] = useState<ActivityFilter>('all')
  // Each tab has its own backend-filtered feed (queryKey includes kind, so
  // switching tabs is instant from cache; placeholderData keeps the previous
  // tab visible while the new one loads).
  const { data, isLoading } = useRecentActivity(limit, offerId, filterToKind(filter))
  const items = data?.items ?? []

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase()
    return items.filter((it) => {
      if (q && !it.source_name.toLowerCase().includes(q) && !it.offer_name.toLowerCase().includes(q)) return false
      return true
    })
  }, [items, search])

  const exportCsv = () => {
    const header = ['Дата', 'Операция', 'Источник', 'Оффер', 'Сумма, руб']
    const rows = filtered.map((it) => [
      formatDateTime(it.occurred_at),
      KIND_META[it.kind].label,
      it.source_name,
      it.offer_name,
      it.amount_kopecks != null ? (it.amount_kopecks / 100).toString().replace('.', ',') : '',
    ])
    const csv = [header, ...rows]
      .map((r) => r.map((c) => `"${String(c ?? '').replace(/"/g, '""')}"`).join(';'))
      .join('\n')
    const blob = new Blob(['﻿' + csv], { type: 'text/csv;charset=utf-8;' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `affiliate_history_${todayStr()}.csv`
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <Card
      neon
      className={cx('flex flex-col overflow-hidden p-0', className)}
      title={<span className="px-4 pt-4 text-[12px] font-bold uppercase tracking-[0.08em]">Последние действия</span>}
    >
      <div className="flex flex-col gap-2 px-4 pb-3">
        <div className="relative">
          <Search size={14} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
          <input
            placeholder="Поиск по источнику"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="h-8 w-full rounded-md border border-[rgba(168,85,247,0.22)] bg-surface-0 py-1 pl-8 pr-3 text-[12px] placeholder:text-faint focus:border-[rgba(168,85,247,0.45)] focus:outline-none"
          />
        </div>
        <div className="flex items-center justify-between gap-2">
          <div className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto">
            {FILTERS.map((f) => (
              <button
                key={f.value}
                type="button"
                onClick={() => setFilter(f.value)}
                className={cx(
                  'shrink-0 rounded-md px-2.5 py-1 text-[11px] font-semibold transition-colors',
                  filter === f.value
                    ? 'bg-violet/15 text-violet-bright'
                    : 'text-faint hover:bg-white/[0.04] hover:text-text',
                )}
              >
                {f.label}
              </button>
            ))}
          </div>
          <button
            type="button"
            onClick={exportCsv}
            title="Экспорт в CSV"
            className="inline-flex h-7 shrink-0 items-center gap-1.5 rounded-md border border-[rgba(168,85,247,0.22)] px-2 text-[11px] font-semibold text-faint hover:text-text"
          >
            <Download size={13} />
            CSV
          </button>
        </div>
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-2 px-4 pb-4">
          {Array.from({ length: 6 }, (_, i) => (
            <div key={i} className="flex items-center gap-3">
              <span className="h-9 w-9 shrink-0 animate-pulse rounded-md bg-white/10" />
              <div className="min-w-0 flex-1 space-y-1.5">
                <div className="h-3 w-2/3 animate-pulse rounded bg-white/10" />
                <div className="h-2.5 w-1/2 animate-pulse rounded bg-white/5" />
              </div>
            </div>
          ))}
        </div>
      ) : filtered.length === 0 ? (
        filter === 'all' ? (
          <div className="px-2 pb-2">
            <EmptyState
              icon={<History size={22} />}
              title="Пока нет операций"
              hint="Переходы, регистрации и доход появятся здесь после первой активности"
            />
          </div>
        ) : (
          <p className="px-4 pb-6 pt-2 text-center text-[13px] text-faint">Ничего не найдено</p>
        )
      ) : (
        <ul className="mx-2 mb-2 max-h-[420px] flex-1 overflow-y-auto p-1 [&::-webkit-scrollbar-thumb]:rounded-full [&::-webkit-scrollbar-thumb]:bg-white/15 [&::-webkit-scrollbar-track]:bg-transparent [&::-webkit-scrollbar]:w-1.5">
          {filtered.map((it) => {
            const meta = KIND_META[it.kind]
            const Icon = meta.icon
            return (
              <li
                key={it.id}
                className="flex items-center gap-3 rounded-md px-2 py-2 transition-colors hover:bg-white/[0.02]"
              >
                <span
                  className={cx(
                    'flex h-9 w-9 shrink-0 items-center justify-center rounded-md',
                    meta.tile,
                  )}
                >
                  <Icon size={16} className={meta.iconClass} />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-[13px] font-semibold">{it.source_name}</span>
                  <span className="block truncate text-[11px] text-faint">
                    {meta.label} · {it.offer_name} · {formatDateTime(it.occurred_at)}
                  </span>
                </span>
                {hasAmount(it.kind) ? (
                  <span
                    className={cx(
                      'shrink-0 text-[13px] font-bold tabular-nums',
                      (it.amount_kopecks ?? 0) < 0 ? 'text-danger' : 'text-success',
                    )}
                  >
                    {formatAmount(it.amount_kopecks ?? 0)}
                  </span>
                ) : (
                  <span className="shrink-0 text-[13px] text-faint">—</span>
                )}
              </li>
            )
          })}
        </ul>
      )}
    </Card>
  )
}
