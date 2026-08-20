import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Users, UserCheck, Wallet, Coins, Gift, Link2, Copy, Share2, Eye, Search, CalendarDays, Download } from 'lucide-react'
import { usePayouts, useReferrals } from '../../api/queries'
import { Card } from '../../components/Card'
import { Button } from '../../components/Button'
import { EmptyState } from '../../components/EmptyState'
import { Skeleton } from '../../components/Skeleton'
import { Table } from '../../components/Table'
import type { TableColumn } from '../../components/Table'
import { formatDate, formatRubles } from '../../lib/format'
import type { components } from '../../api/schema'
import { useToast } from '../../components/Toast'

type ReferralItem = components['schemas']['ReferralItem']

function ReferralsSkeleton() {
  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
        {Array.from({ length: 4 }, (_, i) => (
          <Skeleton key={i} style={{ height: 128 }} />
        ))}
      </div>
      <Skeleton style={{ height: 180 }} />
      <Skeleton style={{ height: 320 }} />
    </div>
  )
}

function ReferralArt() {
  return (
    <div className="relative hidden h-[160px] w-[240px] select-none md:block" aria-hidden>
      <div className="absolute left-1/2 top-[58%] h-[90px] w-[180px] -translate-x-1/2 rounded-full bg-[radial-gradient(ellipse_at_center,rgba(168,85,247,0.45),transparent_70%)] blur-[14px]" />
      <div className="absolute bottom-[18%] left-1/2 h-[56px] w-[180px] -translate-x-1/2 rounded-[12px] border border-[rgba(168,85,247,0.5)] bg-[linear-gradient(180deg,rgba(30,16,60,0.95),rgba(10,10,26,0.98))] shadow-[0_0_22px_rgba(121,40,255,0.35)]" style={{ transform: 'perspective(320px) rotateX(18deg)' }} />
      {/* 3 persons */}
      <div className="absolute bottom-[34%] left-1/2 flex -translate-x-1/2 items-end gap-3">
        <div className="flex flex-col items-center">
          <span className="inline-flex h-9 w-9 items-center justify-center rounded-full border border-[rgba(168,85,247,0.4)] bg-[rgba(168,85,247,0.15)] text-violet-bright">◉</span>
          <span className="mt-1 h-6 w-12 rounded bg-[linear-gradient(180deg,#4a1a8a,#1a0f2e)] border border-violet/20" />
        </div>
        <div className="flex flex-col items-center -translate-y-2">
          <span className="inline-flex h-11 w-11 items-center justify-center rounded-full border-2 border-violet-bright bg-violet text-white shadow-[0_0_16px_rgba(168,85,247,0.6)]">◉</span>
          <span className="mt-1 h-7 w-14 rounded bg-violet border border-white/20" />
        </div>
        <div className="flex flex-col items-center">
          <span className="inline-flex h-9 w-9 items-center justify-center rounded-full border border-[rgba(168,85,247,0.4)] bg-[rgba(168,85,247,0.15)] text-violet-bright">◉</span>
          <span className="mt-1 h-6 w-12 rounded bg-[linear-gradient(180deg,#4a1a8a,#1a0f2e)] border border-violet/20" />
        </div>
      </div>
    </div>
  )
}

export function ReferralsPage() {
  const { data: referrals, isLoading } = useReferrals()
  const { data: payouts } = usePayouts()
  const toast = useToast()
  const navigate = useNavigate()
  const [search, setSearch] = useState('')
  const [levelFilter, setLevelFilter] = useState<'all' | '1' | '2'>('all')
  const [statusFilter, setStatusFilter] = useState<'all' | 'active' | 'inactive'>('all')

  const items = referrals?.items ?? []
  const filteredItems = useMemo(() => {
    let out = items
    const q = search.trim().toLowerCase()
    if (q) {
      out = out.filter(
        (r) =>
          (r.name ?? '').toLowerCase().includes(q) ||
          (r.email ?? '').toLowerCase().includes(q) ||
          (r.partner_id ?? '').toLowerCase().includes(q),
      )
    }
    if (statusFilter !== 'all') {
      const wantActive = statusFilter === 'active'
      out = out.filter((r) => ((r.reward_kopecks ?? 0) > 0) === wantActive)
    }
    if (levelFilter === '2') {
      // второго уровня пока нет — отдельный пул в будущем
      out = []
    }
    return out
  }, [items, search, statusFilter, levelFilter])

  if (isLoading) return <ReferralsSkeleton />
  if (!referrals)
    return <EmptyState title="Не удалось загрузить рефералов" hint="Попробуйте обновить страницу через несколько секунд" />

  const total = referrals.total_invited ?? items.length
  const totalReward = referrals.total_reward_kopecks ?? 0
  const activeCount = items.filter((it) => (it.reward_kopecks ?? 0) > 0).length
  const available = payouts?.balance?.available_kopecks ?? 0
  const avgReward = total > 0 ? Math.round(totalReward / total) : 0

  const isSearching = search.trim().length > 0 || statusFilter !== 'all' || levelFilter !== 'all'
  const emptyTitle = items.length === 0 ? 'Рефералов пока нет' : isSearching ? 'Ничего не найдено' : 'Рефералов пока нет'
  const emptyHint =
    items.length === 0
      ? 'Поделитесь инвайт-ссылкой — вы будете получать вознаграждение с их дохода'
      : isSearching
        ? `По запросу «${search.trim() || statusFilter}» ничего не найдено`
        : 'Поделитесь инвайт-ссылкой — вы будете получать вознаграждение с их дохода'

  const columns: readonly TableColumn<ReferralItem>[] = [
    {
      key: 'referral',
      header: 'Реферал',
      render: (row) => (
        <div className="flex items-center gap-2.5">
          <span className="inline-flex h-7 w-7 items-center justify-center rounded-full bg-[#2a1a4a] border border-white/10 text-[11px] font-bold text-violet-bright">
            {(row.name ?? '?').slice(0, 2).toUpperCase()}
          </span>
          <div className="leading-tight">
            <p className="text-[13px] font-semibold">{row.name ?? '—'}</p>
            <p className="font-mono text-[11px] text-faint">ID: {(row.partner_id ?? '').slice(0, 6).toUpperCase() || '—'}</p>
          </div>
        </div>
      ),
    },
    {
      key: 'level',
      header: 'Уровень',
      render: () => (
        <div className="flex justify-center">
          <span className="inline-flex h-6 w-6 items-center justify-center rounded-full bg-violet/12 text-[12px] font-bold text-violet-bright">1</span>
        </div>
      ),
    },
    {
      key: 'joined',
      header: 'Дата регистрации',
      render: (row) => (row.joined_at ? <span className="text-[12px]">{formatDate(row.joined_at)}</span> : '—'),
    },
    {
      key: 'status',
      header: 'Статус',
      render: (row) =>
        (row.reward_kopecks ?? 0) > 0 ? (
          <span className="inline-flex rounded-full border border-success/30 bg-success/12 px-2.5 py-1 text-[11px] font-semibold text-success">Активен</span>
        ) : (
          <span className="inline-flex rounded-full border border-white/10 bg-white/[0.06] px-2.5 py-1 text-[11px] font-semibold text-faint">Неактивен</span>
        ),
    },
    {
      key: 'partner_income',
      header: 'Доход партнёра',
      align: 'right',
      render: () => <span className="tabular-nums text-faint">—</span>,
    },
    {
      key: 'reward',
      header: 'Ваш доход',
      align: 'right',
      render: (row) => <span className="font-bold tabular-nums">{formatRubles(row.reward_kopecks ?? 0)}</span>,
    },
    {
      key: 'actions',
      header: '',
      align: 'right',
      render: () => (
        <button className="inline-flex h-7 w-7 items-center justify-center rounded-md border border-transparent text-faint hover:bg-surface-hover hover:text-text">
          <Eye size={14} />
        </button>
      ),
    },
  ]

  return (
    <div className="flex flex-col gap-4">
      {/* top 4 */}
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <div className="relative flex flex-col justify-between overflow-hidden rounded-xl border border-[rgba(168,85,247,0.22)] bg-[#0d0c1c] p-4 card-neon">
          <div className="pointer-events-none absolute -right-8 -top-8 h-28 w-28 rounded-full bg-violet/10 blur-[36px]" />
          <div className="flex items-start justify-between">
            <div>
              <p className="text-[10.5px] font-semibold uppercase tracking-[0.06em] text-muted">Всего рефералов</p>
              <p className="mt-1 font-display text-[22px] font-bold leading-none">{total}</p>
              <p className="mt-1 text-[11px] text-faint">
                Код: <span className="font-mono font-semibold text-muted">{referrals.referral_code ?? '—'}</span>
              </p>
            </div>
            <span className="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-[rgba(168,85,247,0.28)] bg-violet/10 text-violet-bright">
              <Users size={16} />
            </span>
          </div>
        </div>

        <div className="relative flex flex-col justify-between overflow-hidden rounded-xl border border-[rgba(168,85,247,0.22)] bg-[#0d0c1c] p-4 card-neon">
          <div className="flex items-start justify-between">
            <div>
              <p className="text-[10.5px] font-semibold uppercase tracking-[0.06em] text-muted">Активных рефералов</p>
              <p className="mt-1 font-display text-[22px] font-bold leading-none">{activeCount}</p>
              <p className="mt-1 text-[11px] text-faint">{total > 0 ? Math.round((activeCount / total) * 100) : 0}% от общего числа</p>
            </div>
            <span className="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-[rgba(47,227,154,0.28)] bg-success/10 text-success">
              <UserCheck size={16} />
            </span>
          </div>
        </div>

        <div className="relative flex flex-col justify-between overflow-hidden rounded-xl border border-[rgba(168,85,247,0.22)] bg-[#0d0c1c] p-4 card-neon">
          <div className="pointer-events-none absolute -right-6 -top-6 h-24 w-24 rounded-full bg-violet/10 blur-[36px]" />
          <div className="flex items-start justify-between">
            <div>
              <p className="text-[10.5px] font-semibold uppercase tracking-[0.06em] text-muted">Доход с рефералов</p>
              <p className="mt-1 font-display text-[20px] font-bold leading-none">{formatRubles(totalReward)}</p>
              <p className="mt-1 text-[11px] text-faint">
                Средний: <span className="font-semibold text-muted">{total > 0 ? formatRubles(avgReward) : '—'}</span>
              </p>
            </div>
            <span className="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-[rgba(168,85,247,0.28)] bg-violet/10 text-violet-bright">
              <Wallet size={16} />
            </span>
          </div>
        </div>

        <div className="relative flex flex-col justify-between overflow-hidden rounded-xl border border-[rgba(168,85,247,0.22)] bg-[#0d0c1c] p-4 card-neon">
          <div className="flex items-start justify-between gap-3">
            <div>
              <p className="text-[10.5px] font-semibold uppercase tracking-[0.06em] text-muted">Доступно к выплате</p>
              <p className="mt-1 font-display text-[20px] font-bold leading-none">{formatRubles(available)}</p>
              <p className="mt-1 text-[11px] text-faint">Баланс кошелька</p>
            </div>
            <span className="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-[rgba(168,85,247,0.28)] bg-violet/10 text-violet-bright">
              <Coins size={16} />
            </span>
          </div>
          <Button
            size="sm"
            className="mt-3 w-full  "
            onClick={() => navigate('/cabinet/payouts')}
          >
            Запросить выплату
          </Button>
        </div>
      </div>

      {/* middle row */}
      <div className="grid gap-4 xl:grid-cols-[1.6fr_1fr]">
        <Card
          neon
          className="overflow-hidden"
          title={<span className="text-[12px] font-bold">Ваша реферальная ссылка</span>}
          subtitle="Приглашайте новых партнёров и получайте процент с их дохода"
        >
          <div className="flex flex-col gap-3">
            <div className="flex items-center gap-2">
              <div className="flex flex-1 items-center gap-2 rounded-md border border-[rgba(168,85,247,0.28)] bg-surface-0 px-3 py-2">
                <Link2 size={14} className="shrink-0 text-faint" />
                <input
                  readOnly
                  value={referrals.invite_url ?? ''}
                  className="min-w-0 flex-1 bg-transparent font-mono text-[12px] text-text focus:outline-none"
                />
                <button
                  type="button"
                  className="inline-flex h-7 w-7 items-center justify-center rounded-md bg-surface-2 text-faint hover:text-text"
                  onClick={() => {
                    void navigator.clipboard.writeText(referrals.invite_url ?? '')
                    toast.success('Скопировано')
                  }}
                >
                  <Copy size={14} />
                </button>
              </div>
              <ReferralArt />
            </div>
            <div className="flex flex-wrap gap-2">
              <Button
                size="sm"
                className="rounded-md bg-violet px-4 text-white "
                onClick={() => {
                  void navigator.clipboard.writeText(referrals.invite_url ?? '')
                  toast.success('Ссылка скопирована')
                }}
              >
                <Copy size={14} />
                Скопировать ссылку
              </Button>
              <Button
                size="sm"
                variant="secondary"
                className="rounded-md border-[rgba(168,85,247,0.22)]"
                onClick={() => {
                  if (navigator.share) void navigator.share({ title: 'CashX', url: referrals.invite_url ?? '' })
                  else {
                    void navigator.clipboard.writeText(referrals.invite_url ?? '')
                    toast.success('Ссылка скопирована')
                  }
                }}
              >
                <Share2 size={14} />
                Поделиться
              </Button>
            </div>
          </div>
        </Card>

        <Card
          neon
          title={<span className="text-[12px] font-bold">Уровни реферальной программы</span>}
          actions={
            <button className="rounded-md border border-[rgba(168,85,247,0.22)] bg-white/[0.04] px-3 py-1 text-[11px] font-semibold text-violet-bright hover:bg-violet/10">
              Подробнее о программе
            </button>
          }
          className="overflow-hidden"
        >
          <div className="grid grid-cols-2 gap-4">
            <div className="flex flex-col gap-2 rounded-xl border border-[rgba(168,85,247,0.22)] bg-surface-0 p-3">
              <div className="flex items-center gap-2">
                <span className="inline-flex h-7 w-7 items-center justify-center rounded-lg border border-violet/30 bg-violet/10 text-violet-bright">
                  <Gift size={14} />
                </span>
                <span className="text-[11px] font-semibold text-faint">1 уровень</span>
              </div>
              <p className="font-display text-[22px] font-black leading-none">5%</p>
              <p className="text-[11px] leading-tight text-faint">с дохода партнёра</p>
              <div className="pointer-events-none mt-1 h-[36px] rounded bg-[radial-gradient(ellipse_at_center,rgba(168,85,247,0.18),transparent_70%)]" />
            </div>
            <div className="flex flex-col gap-2 rounded-xl border border-[rgba(168,85,247,0.22)] bg-surface-0 p-3 opacity-60">
              <div className="flex items-center gap-2">
                <span className="inline-flex h-7 w-7 items-center justify-center rounded-lg border border-white/10 bg-white/[0.06] text-faint">
                  <Users size={14} />
                </span>
                <span className="text-[11px] font-semibold text-faint">2 уровень</span>
              </div>
              <p className="font-display text-[22px] font-black leading-none">—</p>
              <p className="text-[11px] leading-tight text-faint">скоро</p>
              <div className="pointer-events-none mt-1 h-[36px] rounded bg-[radial-gradient(ellipse_at_center,rgba(86,183,255,0.08),transparent_70%)]" />
            </div>
          </div>
        </Card>
      </div>

      {/* table */}
      <Card
        neon
        className="overflow-hidden p-0"
        title={
          <span className="px-4 pt-4 text-[12px] font-bold uppercase tracking-[0.08em] md:hidden">Рефералы</span>
        }
      >
        <div className="flex flex-wrap items-center gap-2 border-b border-[rgba(168,85,247,0.12)] px-4 py-3">
          <div className="relative flex-1 min-w-[200px]">
            <Search size={14} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
            <input
              placeholder="Поиск реферала"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="h-8 w-full rounded-md border border-[rgba(168,85,247,0.22)] bg-surface-0 py-1 pl-8 pr-3 text-[12px] placeholder:text-faint focus:border-[rgba(168,85,247,0.45)] focus:outline-none"
            />
          </div>
          <select
            value={levelFilter}
            onChange={(e) => setLevelFilter(e.target.value as typeof levelFilter)}
            className="h-8 rounded-md border border-[rgba(168,85,247,0.22)] bg-surface-0 px-3 text-[12px]"
          >
            <option value="all">Все уровни</option>
            <option value="1">1 уровень</option>
            <option value="2">2 уровень</option>
          </select>
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value as typeof statusFilter)}
            className="h-8 rounded-md border border-[rgba(168,85,247,0.22)] bg-surface-0 px-3 text-[12px]"
          >
            <option value="all">Все статусы</option>
            <option value="active">Активен</option>
            <option value="inactive">Неактивен</option>
          </select>
          <button className="hidden h-8 items-center gap-1.5 rounded-md border border-[rgba(168,85,247,0.22)] bg-surface-0 px-3 text-[12px] text-muted hover:text-text md:inline-flex">
            <CalendarDays size={14} />
            Выбрать период
          </button>
          <button className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-[rgba(168,85,247,0.22)] bg-surface-0 text-faint">
            <Download size={14} />
          </button>
        </div>

        <div className="overflow-x-auto">
          <Table
            columns={columns}
            rows={filteredItems}
            rowKey={(r) => r.partner_id ?? r.email ?? r.joined_at ?? 'row'}
            emptyTitle={emptyTitle}
            emptyHint={emptyHint}
          />
        </div>
      </Card>
    </div>
  )
}
