import { useMemo, useState } from 'react'
import { Search, Download, CalendarDays, Users, Wallet } from 'lucide-react'
import { useB2CReferrals } from '../../api/queries'
import { Card } from '../../components/Card'
import { EmptyState } from '../../components/EmptyState'
import { Skeleton } from '../../components/Skeleton'
import { Table } from '../../components/Table'
import type { TableColumn } from '../../components/Table'
import { addDays, formatDate, formatRub, toInputDate } from '../../lib/format'
import type { B2CReferralItem } from '../../api/queries'

function toCsv(items: B2CReferralItem[]) {
  const header = ['user_id', 'name', 'kind', 'source', 'deposits_count', 'deposits_sum', 'income', 'created_at']
  const rows = items.map((r) =>
    [r.user_id, r.name, r.kind, r.source_name, String(r.deposits_count), String(r.deposits_sum), String(r.income), r.created_at]
      .map((v) => `"${String(v).replace(/"/g, '""')}"`)
      .join(','),
  )
  return [header.join(','), ...rows].join('\n')
}

function downloadCsv(filename: string, csv: string) {
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

export function B2CReferralsPage() {
  const [from, setFrom] = useState(() => toInputDate(addDays(new Date(), -29)))
  const [to, setTo] = useState(() => toInputDate(new Date()))
  const [search, setSearch] = useState('')
  const [qSearch, setQSearch] = useState('')

  const query = useB2CReferrals({ from, to, search: qSearch || undefined, limit: 100, offset: 0 })

  const items = query.data?.items ?? []
  const total = query.data?.total ?? 0
  const sum = query.data?.sum ?? 0

  const columns: readonly TableColumn<B2CReferralItem>[] = useMemo(
    () => [
      {
        key: 'name',
        header: 'Игрок',
        render: (row) => (
          <div className="leading-tight">
            <p className="text-[13px] font-semibold">{row.name || row.user_id.slice(0, 8)}</p>
            <p className="font-mono text-[11px] text-faint">{row.email ?? row.user_id.slice(0, 12)}</p>
          </div>
        ),
      },
      {
        key: 'kind',
        header: 'Тип',
        render: (row) =>
          row.kind === 'promo' ? (
            <span className="inline-flex rounded-full border border-violet/30 bg-violet/12 px-2 py-0.5 text-[11px] font-semibold text-violet-bright">промо</span>
          ) : (
            <span className="inline-flex rounded-full border border-white/10 bg-white/[0.06] px-2 py-0.5 text-[11px] font-semibold text-muted">ссылка</span>
          ),
      },
      {
        key: 'source',
        header: 'Источник',
        render: (row) => <span className="text-[12px]">{row.source_name || '—'}</span>,
      },
      {
        key: 'deposits',
        header: 'Депозиты',
        align: 'right',
        render: (row) => (
          <div className="text-right tabular-nums">
            <span className="font-semibold">{formatRub(row.deposits_sum)}</span>
            <span className="ml-1 text-[11px] text-faint">×{row.deposits_count}</span>
          </div>
        ),
      },
      {
        key: 'income',
        header: 'Доход',
        align: 'right',
        render: (row) => <span className="font-bold tabular-nums text-violet-bright">{formatRub(row.income)}</span>,
      },
      {
        key: 'date',
        header: 'Дата',
        render: (row) => <span className="text-[12px]">{formatDate(row.created_at)}</span>,
      },
    ],
    [],
  )

  if (query.isLoading) {
    return (
      <div className="flex flex-col gap-4">
        <div className="grid gap-4 md:grid-cols-2">
          <Skeleton style={{ height: 120 }} />
          <Skeleton style={{ height: 120 }} />
        </div>
        <Skeleton style={{ height: 360 }} />
      </div>
    )
  }

  if (query.isError) {
    return <EmptyState title="Не удалось загрузить игроков" hint="Попробуйте обновить страницу" />
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="grid gap-4 md:grid-cols-2">
        <div className="relative flex flex-col justify-between overflow-hidden rounded-xl border border-[rgba(168,85,247,0.22)] bg-[#0d0c1c] p-4 card-neon">
          <div className="pointer-events-none absolute -right-8 -top-8 h-28 w-28 rounded-full bg-violet/10 blur-[36px]" />
          <div className="flex items-start justify-between">
            <div>
              <p className="text-[10.5px] font-semibold uppercase tracking-[0.06em] text-muted">Игроков</p>
              <p className="mt-1 font-display text-[22px] font-bold leading-none">{total}</p>
              <p className="mt-1 text-[11px] text-faint">
                за период {from} — {to}
              </p>
            </div>
            <span className="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-[rgba(168,85,247,0.28)] bg-violet/10 text-violet-bright">
              <Users size={16} />
            </span>
          </div>
        </div>
        <div className="relative flex flex-col justify-between overflow-hidden rounded-xl border border-[rgba(168,85,247,0.22)] bg-[#0d0c1c] p-4 card-neon">
          <div className="pointer-events-none absolute -right-8 -top-8 h-28 w-28 rounded-full bg-violet/10 blur-[36px]" />
          <div className="flex items-start justify-between">
            <div>
              <p className="text-[10.5px] font-semibold uppercase tracking-[0.06em] text-muted">Доход</p>
              <p className="mt-1 font-display text-[20px] font-bold leading-none">{formatRub(sum)}</p>
              <p className="mt-1 text-[11px] text-faint">Суммарный доход с игроков</p>
            </div>
            <span className="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-[rgba(168,85,247,0.28)] bg-violet/10 text-violet-bright">
              <Wallet size={16} />
            </span>
          </div>
        </div>
      </div>

      <Card
        neon
        className="overflow-hidden p-0"
        title={<span className="px-4 pt-4 text-[12px] font-bold uppercase tracking-[0.08em]">Игроки</span>}
        actions={
          <div className="hidden items-center gap-2 pr-4 pt-4 md:flex">
            <div className="flex items-center gap-2">
              <span className="inline-flex items-center gap-1 text-[11px] text-faint">
                <CalendarDays size={14} /> Период
              </span>
              <input
                type="date"
                value={from}
                onChange={(e) => setFrom(e.target.value)}
                className="h-8 rounded-md border border-[rgba(168,85,247,0.22)] bg-surface-0 px-2 text-[12px]"
              />
              <span className="text-faint">—</span>
              <input
                type="date"
                value={to}
                onChange={(e) => setTo(e.target.value)}
                className="h-8 rounded-md border border-[rgba(168,85,247,0.22)] bg-surface-0 px-2 text-[12px]"
              />
            </div>
            <button
              className="inline-flex h-8 items-center gap-1.5 rounded-md border border-[rgba(168,85,247,0.22)] bg-surface-0 px-3 text-[12px] text-muted hover:text-text"
              onClick={() => downloadCsv(`b2c-${from}_${to}.csv`, toCsv(items))}
            >
              <Download size={14} /> CSV
            </button>
          </div>
        }
      >
        <div className="flex flex-wrap items-center gap-2 border-b border-[rgba(168,85,247,0.12)] px-4 py-3">
          <div className="relative flex-1 min-w-[200px]">
            <Search size={14} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
            <input
              placeholder="Поиск игрока / источника"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') setQSearch(search.trim())
              }}
              className="h-8 w-full rounded-md border border-[rgba(168,85,247,0.22)] bg-surface-0 py-1 pl-8 pr-3 text-[12px] placeholder:text-faint focus:border-[rgba(168,85,247,0.45)] focus:outline-none"
            />
          </div>
          <button
            className="h-8 rounded-md border border-[rgba(168,85,247,0.22)] bg-surface-0 px-3 text-[12px]"
            onClick={() => setQSearch(search.trim())}
          >
            Найти
          </button>
          <a
            href={`/api/v1/cabinet/b2c-referrals.csv?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`}
            className="inline-flex h-8 items-center gap-1.5 rounded-md border border-[rgba(168,85,247,0.22)] bg-surface-0 px-3 text-[12px] text-muted hover:text-text"
          >
            <Download size={14} /> Скачать CSV (бэк)
          </a>
        </div>

        <div className="overflow-x-auto">
          <Table
            columns={columns}
            rows={items}
            rowKey={(r) => r.user_id}
            emptyTitle={total === 0 ? 'Игроков пока нет' : 'Ничего не найдено'}
            emptyHint={total === 0 ? 'Привлекайте игроков через ссылки и промокоды — они появятся здесь' : `По запросу «${qSearch}» игроков нет`}
          />
        </div>
      </Card>
    </div>
  )
}
