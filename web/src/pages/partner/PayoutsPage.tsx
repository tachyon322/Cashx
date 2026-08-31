import { useMemo, useState } from 'react'
import type { FormEvent } from 'react'
import { Clock, Wallet, ArrowUpRight, Copy, Filter, CalendarDays, Download } from 'lucide-react'
import { ApiRequestError } from '../../api/client'
import { useCabinetTransactions, useCancelPayout, usePayoutConfig, usePayouts, useRequestPayout } from '../../api/queries'
import type { PayoutRequestInput } from '../../api/queries'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { EmptyState } from '../../components/EmptyState'
import { Field } from '../../components/Field'
import { Input } from '../../components/Input'
import { Select } from '../../components/Select'
import { Skeleton } from '../../components/Skeleton'
import { Modal } from '../../components/Modal'
import { Table } from '../../components/Table'
import type { TableColumn } from '../../components/Table'
import { useToast } from '../../components/Toast'
import { formatDateTime, formatRubles } from '../../lib/format'
import { withdrawalStatus } from '../../lib/status'
import type { components } from '../../api/schema'

type WithdrawalRequest = components['schemas']['WithdrawalRequest']
type LedgerEntry = components['schemas']['LedgerEntry']

const LEDGER_LABELS: Record<string, string> = {
  commission: 'Комиссия',
  referral_reward: 'Реферальная награда',
  reversal: 'Отмена',
  withdrawal: 'Вывод',
}

function PayoutsSkeleton() {
  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
        {Array.from({ length: 4 }, (_, i) => (
          <Skeleton key={i} style={{ height: 140 }} />
        ))}
      </div>
      <Skeleton style={{ height: 420 }} />
    </div>
  )
}

function UsdtIcon() {
  return (
    <span className="inline-flex h-9 w-9 items-center justify-center rounded-full bg-[#1BA27A] text-white font-bold text-[14px]">₮</span>
  )
}

export function PayoutsPage() {
  const toast = useToast()
  const payoutsQuery = usePayouts()
  const configQuery = usePayoutConfig()
  const txQuery = useCabinetTransactions()
  const request = useRequestPayout()
  const cancel = useCancelPayout()

  const [method, setMethod] = useState<PayoutRequestInput['method']>('usdt')
  const [amount, setAmount] = useState('')
  const [requisites, setRequisites] = useState('')
  const [bank, setBank] = useState('')
  const [amountError, setAmountError] = useState<string | null>(null)
  const [requisitesError, setRequisitesError] = useState<string | null>(null)
  const [formError, setFormError] = useState<string | null>(null)
  const [cancellingId, setCancellingId] = useState<string | null>(null)
  const [modalOpen, setModalOpen] = useState(false)

  if (payoutsQuery.isLoading || configQuery.isLoading) return <PayoutsSkeleton />
  if (payoutsQuery.error || configQuery.error)
    return <EmptyState title="Не удалось загрузить данные" hint="Попробуйте обновить страницу через несколько секунд" />

  const txItems = txQuery.data?.items ?? []

  const balance = payoutsQuery.data?.balance
  const requests = payoutsQuery.data?.requests ?? []
  const history = payoutsQuery.data?.history ?? []
  const rules = configQuery.data
  const minKopecks = rules?.min_withdraw_kopecks ?? 500000

  // derived stats for top cards
  const available = balance?.available_kopecks ?? 0
  const reserved = balance?.reserved_kopecks ?? 0
  const totalPaid = history.filter((h) => h.type === 'withdrawal' && (h.amount_kopecks ?? 0) < 0).reduce((a, h) => a + Math.abs(h.amount_kopecks ?? 0), 0)
  const totalOps = requests.filter((r) => r.status === 'paid').length || history.filter((h) => h.type === 'withdrawal').length

  const handleSubmit = (event: FormEvent) => {
    event.preventDefault()
    setFormError(null)
    setAmountError(null)
    setRequisitesError(null)
    const raw = amount.trim().replace(',', '.')
    const rubles = Number.parseFloat(raw)
    const kopecks = Number.isFinite(rubles) ? Math.round(rubles * 100) : NaN
    let invalid = false
    if (!Number.isFinite(kopecks) || kopecks <= 0) {
      setAmountError('Укажите сумму')
      invalid = true
    } else if (kopecks < minKopecks) {
      setAmountError(`Минимальная сумма — ${formatRubles(minKopecks)}`)
      invalid = true
    }
    if (!requisites.trim()) {
      setRequisitesError('Укажите реквизиты')
      invalid = true
    }
    if (invalid) return
    request.mutate(
      {
        method,
        amount_kopecks: kopecks,
        requisites: requisites.trim(),
        bank: method === 'sbp' && bank.trim() ? bank.trim() : undefined,
      },
      {
        onSuccess: () => {
          toast.success('Заявка на вывод создана')
          setAmount('')
          setRequisites('')
          setBank('')
          setModalOpen(false)
        },
        onError: (error) => {
          if (error instanceof ApiRequestError && error.status === 402) setFormError('Недостаточно средств')
          else setFormError(error instanceof Error ? error.message : 'Не удалось создать заявку')
        },
      },
    )
  }

  const handleCancel = (id: string) => {
    if (cancel.isPending) return
    setCancellingId(id)
    cancel.mutate(id, {
      onSuccess: () => toast.success('Заявка отменена'),
      onError: (error) => {
        if (error instanceof ApiRequestError && error.status === 409) {
          toast.error('Заявку уже нельзя отменить')
          void payoutsQuery.refetch()
        } else toast.error(error instanceof Error ? error.message : 'Не удалось отменить заявку')
      },
      onSettled: () => setCancellingId(null),
    })
  }

  const requestColumns: readonly TableColumn<WithdrawalRequest>[] = [
    {
      key: 'id',
      header: 'ID',
      render: (row) => <span className="font-mono text-[12px] text-muted">#{String(row.id ?? '').slice(0, 4).toUpperCase()}</span>,
    },
    {
      key: 'created',
      header: 'Дата запроса',
      render: (row) => (row.created_at ? <span className="text-[12px]">{formatDateTime(row.created_at)}</span> : '—'),
    },
    {
      key: 'method',
      header: 'Способ',
      render: (row) => (
        <span className="inline-flex items-center gap-1.5 text-[12px]">
          <span className="inline-flex h-5 w-5 items-center justify-center rounded-full bg-[#1BA27A] text-[10px] font-bold text-white">₮</span>
          {row.method === 'usdt' ? 'USDT (TRC20)' : 'СБП'}
        </span>
      ),
    },
    {
      key: 'amount',
      header: 'Сумма',
      align: 'right',
      render: (row) => <span className="font-bold tabular-nums">{formatRubles(row.amount_kopecks ?? 0)}</span>,
    },
    {
      key: 'fee',
      header: 'Комиссия',
      align: 'right',
      render: (row) => <span className="tabular-nums text-faint">{formatRubles((row as any).fee_kopecks ?? 0)}</span>,
    },
    {
      key: 'status',
      header: 'Статус',
      render: (row) => {
        const st = withdrawalStatus(row.status)
        const toneCls =
          row.status === 'paid'
            ? 'border-success/30 bg-success/12 text-success'
            : row.status === 'pending'
              ? 'border-warning/30 bg-warning/12 text-warning'
              : row.status === 'approved'
                ? 'border-blue/30 bg-blue/12 text-blue'
                : 'border-danger/30 bg-danger/12 text-danger'
        return (
          <span className={`inline-flex rounded-full border px-2 py-0.5 text-[11px] font-semibold ${toneCls}`}>{st.label}</span>
        )
      },
    },
    {
      key: 'paid_at',
      header: 'Дата выплаты',
      render: (row) => ((row as any).paid_at ? formatDateTime((row as any).paid_at) : <span className="text-faint">—</span>),
    },
    {
      key: 'tx',
      header: 'TxID',
      align: 'right',
      render: (row) => {
        const tx = (row as any).tx_id ?? (row as any).external_tx_id ?? ''
        if (!tx) return <span className="text-faint">—</span>
        const short = String(tx).slice(0, 6) + '...' + String(tx).slice(-4)
        return (
          <span className="inline-flex items-center gap-1 font-mono text-[11px] text-muted">
            {short}
            <button
              className="inline-flex h-6 w-6 items-center justify-center rounded-md border border-transparent text-faint hover:bg-surface-hover hover:text-text"
              onClick={() => {
                void navigator.clipboard.writeText(String(tx))
                toast.success('Скопировано')
              }}
            >
              <Copy size={12} />
            </button>
          </span>
        )
      },
    },
    {
      key: 'actions',
      header: '',
      align: 'right',
      render: (row) =>
        row.status === 'pending' ? (
          <Button variant="danger" size="sm" loading={cancellingId === row.id} onClick={() => handleCancel(row.id ?? '')}>
            Отменить
          </Button>
        ) : null,
    },
  ]

  const historyColumns: readonly TableColumn<LedgerEntry>[] = [
    { key: 'type', header: 'Тип', render: (row) => (row.type ? (LEDGER_LABELS[row.type] ?? row.type) : '—') },
    { key: 'amount', header: 'Сумма', align: 'right', render: (row) => formatRubles(row.amount_kopecks ?? 0) },
    { key: 'balance', header: 'Баланс после', align: 'right', render: (row) => formatRubles(row.balance_after_kopecks ?? 0) },
    { key: 'created', header: 'Дата', render: (row) => (row.created_at ? formatDateTime(row.created_at) : '—') },
  ]

  const feePreview = useMemo(() => {
    const raw = amount.trim().replace(',', '.')
    const rub = Number.parseFloat(raw)
    const k = Number.isFinite(rub) ? Math.round(rub * 100) : 0
    if (k <= 0 || !rules) return null
    if (method === 'sbp') {
      const flat = rules.sbp_fee_flat_kopecks ?? 0
      const pct = rules.sbp_fee_percent_bps ?? 0
      const fee = flat + Math.round((k * pct) / 10000)
      return fee
    }
    return 0
  }, [amount, method, rules])

  return (
    <div className="flex flex-col gap-4">
      {/* top 4 cards */}
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        {/* Available - neon strong */}
        <div className="relative flex flex-col justify-between overflow-hidden rounded-xl border border-[rgba(168,85,247,0.45)] bg-[#0d0c1c] p-4 card-neon-strong">
          <div className="pointer-events-none absolute -right-10 -top-10 h-32 w-32 rounded-full bg-violet/20 blur-[40px]" />
          <div className="pointer-events-none absolute inset-0 hero-grid opacity-[0.14]" />
          <div className="relative z-[1] flex items-start justify-between">
            <div>
              <p className="text-[10.5px] font-semibold uppercase tracking-[0.06em] text-muted">Доступно к выплате</p>
              <p className="mt-1 font-display text-[22px] font-bold leading-none tracking-[-0.02em]">{formatRubles(available)}</p>
              <p className="mt-1 text-[11px] text-faint">Минимальная сумма: {formatRubles(minKopecks)}</p>
            </div>
            <span className="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-[rgba(168,85,247,0.35)] bg-violet/10 text-violet-bright">
              <Wallet size={16} />
            </span>
          </div>
          <Button
            className="relative z-[1] mt-4 w-full"
            onClick={() => setModalOpen(true)}
          >
            Запросить выплату
          </Button>
        </div>

        <div className="relative flex flex-col justify-between overflow-hidden rounded-xl border border-[rgba(168,85,247,0.22)] bg-[#0d0c1c] p-4 card-neon">
          <div className="pointer-events-none absolute -right-8 -top-8 h-28 w-28 rounded-full bg-violet/10 blur-[36px]" />
          <div className="flex items-start justify-between">
            <div>
              <p className="text-[10.5px] font-semibold uppercase tracking-[0.06em] text-muted">Ожидается</p>
              <p className="mt-1 font-display text-[20px] font-bold leading-none">{formatRubles(reserved)}</p>
              <p className="mt-1 text-[11px] text-faint">Будет доступно: —</p>
            </div>
            <span className="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-[rgba(168,85,247,0.22)] bg-white/[0.04] text-faint">
              <Clock size={16} />
            </span>
          </div>
          <div className="mt-3 h-1 rounded-full bg-white/10">
            <div className="h-1 w-[42%] rounded-full bg-violet/60" />
          </div>
        </div>

        <div className="relative flex flex-col justify-between overflow-hidden rounded-xl border border-[rgba(168,85,247,0.22)] bg-[#0d0c1c] p-4 card-neon">
          <div className="pointer-events-none absolute -right-8 -top-8 h-28 w-28 rounded-full bg-violet/10 blur-[36px]" />
          <div className="flex items-start justify-between">
            <div>
              <p className="text-[10.5px] font-semibold uppercase tracking-[0.06em] text-muted">Всего выплачено</p>
              <p className="mt-1 font-display text-[20px] font-bold leading-none">{formatRubles(totalPaid)}</p>
              <p className="mt-1 text-[11px] text-faint">Всего операций: {totalOps}</p>
            </div>
            <span className="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-[rgba(168,85,247,0.22)] bg-violet/10 text-violet-bright">
              <ArrowUpRight size={16} />
            </span>
          </div>
        </div>

        <div className="relative flex flex-col justify-between overflow-hidden rounded-xl border border-[rgba(168,85,247,0.22)] bg-[#0d0c1c] p-4 card-neon">
          <div className="flex items-start justify-between gap-3">
            <div>
              <p className="text-[10.5px] font-semibold uppercase tracking-[0.06em] text-muted">Способ выплаты</p>
              <div className="mt-2 flex items-center gap-2.5">
                <UsdtIcon />
                <div className="leading-tight">
                  <p className="text-[13px] font-bold">USDT (TRC20)</p>
                  <p className="font-mono text-[11px] text-faint">{requests[0]?.requisites ? String(requests[0].requisites).slice(0, 12) + '...aWhk' : 'TKoi...aWhk'}</p>
                </div>
              </div>
            </div>
          </div>
          <button
            type="button"
            onClick={() => setModalOpen(true)}
            className="relative isolate mt-4 inline-flex w-full items-center justify-center overflow-hidden rounded-md border border-[rgba(168,85,247,0.28)] bg-white/[0.04] px-3 py-2 text-[12px] font-semibold text-violet-bright transition-[background-color,border-color,color,box-shadow,transform] duration-150 hover:bg-violet/10 active:btn-volume-pressed active:translate-y-px btn-volume-ghost btn-side-gradient"
          >
            <span className="relative z-[1]">Изменить</span>
          </button>
        </div>
      </div>

      {/* History */}
      <Card
        neon
        className="overflow-hidden p-0"
        title={<span className="px-4 pt-4 text-[12px] font-bold uppercase tracking-[0.08em]">История выплат</span>}
        actions={
          <div className="hidden items-center gap-2 pr-4 pt-4 md:flex">
            <select className="h-8 rounded-md border border-[rgba(168,85,247,0.22)] bg-surface-0 px-3 text-[12px]">
              <option>Все статусы</option>
              <option>Выплачено</option>
              <option>Ожидает</option>
            </select>
            <button className="inline-flex h-8 items-center gap-1.5 rounded-md border border-[rgba(168,85,247,0.22)] bg-surface-0 px-3 text-[12px] text-muted hover:text-text">
              <CalendarDays size={14} />
              Выбрать период
            </button>
            <button className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-[rgba(168,85,247,0.22)] bg-surface-0 text-faint">
              <Filter size={14} />
            </button>
          </div>
        }
      >
        {requests.length === 0 ? (
          <div className="p-4">
            <EmptyState title="Заявок пока нет" hint="Создайте первую заявку на выплату — она появится здесь" />
          </div>
        ) : (
          <div className="overflow-x-auto">
            <Table
              columns={requestColumns}
              rows={requests}
              rowKey={(r) => r.id ?? `${r.created_at}-${r.amount_kopecks}`}
              compact
            />
          </div>
        )}
      </Card>

      {/* Ledger as second table like before */}
      <Card
        neon
        title={<span className="text-[12px] font-bold uppercase tracking-[0.08em]">История операций</span>}
        subtitle="Движения по балансу"
        actions={
          <a
            href="/api/v1/cabinet/transactions?format=csv"
            className="inline-flex h-8 items-center gap-1.5 rounded-md border border-[rgba(168,85,247,0.22)] bg-surface-0 px-3 text-[12px] text-muted hover:text-text"
          >
            <Download size={14} /> CSV
          </a>
        }
      >
        {history.length === 0 ? (
          <EmptyState title="Операций пока нет" hint="Здесь появятся начисления и списания" />
        ) : (
          <Table columns={historyColumns} rows={history} rowKey={(r) => r.id ?? `${r.created_at}-${r.type}`} compact />
        )}
      </Card>

      {txItems.length > 0 && (
        <Card
          neon
          title={<span className="text-[12px] font-bold uppercase tracking-[0.08em]">Транзакции (200)</span>}
          subtitle="Последние 200 движений кошелька"
          actions={
            <a
              href="/api/v1/cabinet/transactions?format=csv"
              className="inline-flex h-8 items-center gap-1.5 rounded-md border border-[rgba(168,85,247,0.22)] bg-surface-0 px-3 text-[12px] text-muted hover:text-text"
            >
              <Download size={14} /> CSV (200)
            </a>
          }
        >
          <Table
            columns={
              [
                { key: 'type', header: 'Тип', render: (row: any) => LEDGER_LABELS[row.type] ?? row.type },
                { key: 'amount', header: 'Сумма', align: 'right', render: (row: any) => formatRubles(row.amount_kopecks) },
                { key: 'created', header: 'Дата', render: (row: any) => (row.created_at ? formatDateTime(row.created_at) : '—') },
              ] as any
            }
            rows={txItems as any}
            rowKey={(r: any) => r.id}
            compact
          />
        </Card>
      )}

      {/* Request modal */}
      <Modal open={modalOpen} onClose={() => setModalOpen(false)} title="Запросить выплату">
        <form className="flex flex-col gap-4" onSubmit={handleSubmit} noValidate>
          <div className="rounded-lg border border-[rgba(168,85,247,0.18)] bg-surface-1 p-3 text-[12px] leading-relaxed text-faint">
            Доступно: <span className="font-bold text-text">{formatRubles(available)}</span> · Мин. {formatRubles(minKopecks)}
            {rules?.usdt_rate != null && <span> · Курс USDT 1 = {rules.usdt_rate.toLocaleString('ru-RU', { maximumFractionDigits: 2 })} ₽</span>}
          </div>
          <Field label="Метод вывода" htmlFor="payout-method-modal">
            <Select
              id="payout-method-modal"
              value={method}
              onChange={(e) => setMethod(e.target.value as PayoutRequestInput['method'])}
            >
              <option value="usdt">USDT (TRC20)</option>
              <option value="sbp">СБП</option>
            </Select>
          </Field>
          <Field label="Сумма, ₽" htmlFor="payout-amount-modal" error={amountError}>
            <Input
              id="payout-amount-modal"
              type="number"
              inputMode="decimal"
              min="0"
              step="0.01"
              placeholder="Например, 1000"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
            />
          </Field>
          <Field label="Реквизиты" htmlFor="payout-req-modal" error={requisitesError}>
            <Input
              id="payout-req-modal"
              placeholder={method === 'usdt' ? 'Адрес USDT-кошелька' : 'Номер телефона или счёт'}
              value={requisites}
              onChange={(e) => setRequisites(e.target.value)}
            />
          </Field>
          {feePreview != null && (
            <div className="rounded-md border border-white/10 bg-white/[0.04] px-3 py-2 text-[12px] text-muted">
              Комиссия: <span className="font-semibold text-text">{formatRubles(feePreview)}</span> · К выплате:{' '}
              <span className="font-semibold text-text">
                {formatRubles(
                  (() => {
                    const raw = amount.trim().replace(',', '.')
                    const rub = Number.parseFloat(raw)
                    const k = Number.isFinite(rub) ? Math.round(rub * 100) : 0
                    return Math.max(0, k - feePreview)
                  })(),
                )}
              </span>
              {rules?.usdt_rate != null && method === 'usdt' && (
                <span>
                  {' '}
                  · ~{((Math.max(0, (Number.parseFloat(amount.trim().replace(',', '.')) || 0) * 100 - feePreview) / 100 / (rules.usdt_rate as number)).toFixed(2))} USDT
                </span>
              )}
            </div>
          )}
          {method === 'sbp' && (
            <Field label="Банк (необязательно)" htmlFor="payout-bank-modal">
              <Input id="payout-bank-modal" placeholder="Название банка" value={bank} onChange={(e) => setBank(e.target.value)} />
            </Field>
          )}
          {formError && (
            <div className="rounded-md border border-danger/40 bg-danger/8 px-3 py-2.5 text-[13px] text-danger" role="alert">
              {formError}
            </div>
          )}
          <div className="flex gap-2">
            <Button type="button" variant="secondary" className="flex-1 rounded-md" onClick={() => setModalOpen(false)}>
              Отмена
            </Button>
            <Button type="submit" className="flex-1 rounded-md" loading={request.isPending}>
              Отправить заявку
            </Button>
          </div>
        </form>
      </Modal>
    </div>
  )
}
