import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Banknote, CheckCircle2, XCircle } from 'lucide-react'
import { useAuth } from '../../auth/AuthContext'
import {
  useAdminWithdrawalDecide,
  useAdminWithdrawalPay,
  useAdminWithdrawals,
} from '../../api/queries'
import type { AdminWithdrawalsParams } from '../../api/queries'
import { Badge } from '../../components/Badge'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { Field } from '../../components/Field'
import { Input } from '../../components/Input'
import { Modal } from '../../components/Modal'
import { PAGE_SIZE, Pagination } from '../../components/Pagination'
import { Skeleton } from '../../components/Skeleton'
import { Table } from '../../components/Table'
import { Textarea } from '../../components/Textarea'
import { useToast } from '../../components/Toast'
import { formatDateTime, formatRubles } from '../../lib/format'
import { WITHDRAWAL_STATUS, withdrawalStatus } from '../../lib/status'
import { cx } from '../../lib/cx'

const METHOD_LABEL: Record<string, string> = { usdt: 'USDT', sbp: 'СБП' }

/** Табы статусов: «Все» + все статусы из карты WITHDRAWAL_STATUS. */
const TABS: { value: string; label: string }[] = [
  { value: '', label: 'Все' },
  ...Object.entries(WITHDRAWAL_STATUS).map(([value, info]) => ({ value, label: info.label })),
]

const PAGE_CLASSES = 'flex w-full flex-col gap-4 p-4'
const TABS_CLASSES = 'flex w-fit max-w-full flex-wrap gap-1 rounded-md border border-border bg-surface-0 p-1'
const TAB_BTN_CLASSES =
  'cursor-pointer rounded-md border-none bg-transparent px-3.5 py-[7px] text-[13px] font-semibold text-muted transition-colors duration-150 hover:bg-surface-hover hover:text-text'
const TAB_BTN_ACTIVE_CLASSES = 'bg-surface-2 text-text shadow-[0_0_0_1px_var(--cx-border-active)]'
const SKELETON_CLASSES = 'flex flex-col gap-2'
const MODAL_FORM_CLASSES = 'flex flex-col gap-4'
const MODAL_ACTIONS_CLASSES = 'mt-1 flex justify-end gap-3'

export function WithdrawalsPage() {
  const toast = useToast()
  const { user } = useAuth()
  const staffRoles = user?.staff?.roles ?? []
  const canEdit = staffRoles.includes('superadmin') || staffRoles.includes('finance')

  // Статус — query-параметр; смена таба сбрасывает offset.
  const [searchParams, setSearchParams] = useSearchParams()
  const status = searchParams.get('status') ?? ''
  const [offset, setOffset] = useState(0)

  useEffect(() => {
    setOffset(0)
  }, [status])

  const setTab = (value: string) => {
    setSearchParams(value ? { status: value } : {}, { replace: true })
  }

  const params: AdminWithdrawalsParams = {
    limit: PAGE_SIZE,
    offset,
    ...(status ? { status } : {}),
  }
  const withdrawals = useAdminWithdrawals(params)
  const items = withdrawals.data?.items ?? []
  const total = withdrawals.data?.total ?? 0

  const decideMutation = useAdminWithdrawalDecide()
  const payMutation = useAdminWithdrawalPay()

  // Рассмотрение заявки (pending).
  const [decideId, setDecideId] = useState<string | null>(null)
  const [decideComment, setDecideComment] = useState('')

  const submitDecide = async (decision: 'approved' | 'rejected') => {
    if (!decideId) return
    try {
      await decideMutation.mutateAsync({
        id: decideId,
        decision,
        comment: decideComment || undefined,
      })
      toast.success(decision === 'approved' ? 'Заявка одобрена' : 'Заявка отклонена')
      setDecideId(null)
      setDecideComment('')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Не удалось рассмотреть заявку')
    }
  }

  // Отметка выплаченным (approved).
  const [payId, setPayId] = useState<string | null>(null)
  const [externalTxId, setExternalTxId] = useState('')
  const [payComment, setPayComment] = useState('')

  const submitPay = async () => {
    if (!payId) return
    try {
      await payMutation.mutateAsync({
        id: payId,
        external_tx_id: externalTxId.trim() || undefined,
        comment: payComment.trim() || undefined,
      })
      toast.success('Заявка отмечена выплаченной')
      setPayId(null)
      setExternalTxId('')
      setPayComment('')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Не удалось отметить выплату')
    }
  }

  return (
    <div className={PAGE_CLASSES}>
      <Card title="Заявки на вывод" subtitle={`Всего: ${total}`}>
        <div className={TABS_CLASSES} role="tablist">
          {TABS.map((tab) => (
            <button
              key={tab.value}
              type="button"
              role="tab"
              aria-selected={status === tab.value}
              className={cx(TAB_BTN_CLASSES, status === tab.value && TAB_BTN_ACTIVE_CLASSES)}
              onClick={() => setTab(tab.value)}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {withdrawals.isLoading && items.length === 0 ? (
          <div className={SKELETON_CLASSES}>
            <Skeleton style={{ height: 36 }} />
            <Skeleton style={{ height: 36 }} />
            <Skeleton style={{ height: 36 }} />
          </div>
        ) : (
          <Table
            columns={[
              {
                key: 'partner',
                header: 'Партнёр',
                render: (w) => (
                  <div>
                    <div>{w.partner_name ?? '—'}</div>
                    <div className="text-[12px] text-faint">
                      {w.partner_email ?? ''}
                    </div>
                  </div>
                ),
              },
              {
                key: 'amount',
                header: 'Сумма',
                align: 'right',
                render: (w) => (
                  <span className="tabular-nums">{formatRubles(w.amount_kopecks ?? 0)}</span>
                ),
              },
              {
                key: 'method',
                header: 'Метод',
                render: (w) => METHOD_LABEL[w.method ?? ''] ?? w.method ?? '—',
              },
              { key: 'requisites', header: 'Реквизиты', render: (w) => w.requisites ?? '—' },
              { key: 'bank', header: 'Банк', render: (w) => w.bank ?? '—' },
              {
                key: 'fee',
                header: 'Комиссия',
                align: 'right',
                render: (w) => formatRubles(w.fee_kopecks ?? 0),
              },
              {
                key: 'status',
                header: 'Статус',
                render: (w) => {
                  const s = withdrawalStatus(w.status)
                  return <Badge tone={s.tone}>{s.label}</Badge>
                },
              },
              {
                key: 'created_at',
                header: 'Создана',
                render: (w) => (w.created_at ? formatDateTime(w.created_at) : '—'),
              },
              {
                key: 'updated_at',
                header: 'Обновлена',
                render: (w) => (w.updated_at ? formatDateTime(w.updated_at) : '—'),
              },
              {
                key: 'actions',
                header: '',
                render: (w) => {
                  if (!canEdit) return null
                  if (w.status === 'pending') {
                    return (
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => {
                          setDecideId(w.id ?? null)
                          setDecideComment('')
                        }}
                      >
                        Рассмотреть
                      </Button>
                    )
                  }
                  if (w.status === 'approved') {
                    return (
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => {
                          setPayId(w.id ?? null)
                          setExternalTxId('')
                          setPayComment('')
                        }}
                      >
                        <Banknote size={14} />
                        Отметить выплаченным
                      </Button>
                    )
                  }
                  return null
                },
              },
            ]}
            rows={items}
            rowKey={(w) => w.id ?? ''}
            emptyTitle="Заявок на вывод нет"
            emptyHint="Заявки появятся здесь после создания партнёрами"
          />
        )}

        <Pagination total={total} offset={offset} limit={PAGE_SIZE} onChange={setOffset} />
      </Card>

      {/* Рассмотрение заявки */}
      <Modal
        open={decideId !== null}
        onClose={() => setDecideId(null)}
        title="Рассмотреть заявку"
      >
        <div className={MODAL_FORM_CLASSES}>
          <Field label="Комментарий" htmlFor="wd-comment">
            <Textarea
              id="wd-comment"
              rows={3}
              value={decideComment}
              onChange={(event) => setDecideComment(event.target.value)}
            />
          </Field>
          <div className={MODAL_ACTIONS_CLASSES}>
            <Button variant="secondary" onClick={() => setDecideId(null)}>
              Отмена
            </Button>
            <Button
              variant="danger"
              onClick={() => void submitDecide('rejected')}
              loading={decideMutation.isPending}
            >
              <XCircle size={16} />
              Отклонить
            </Button>
            <Button
              onClick={() => void submitDecide('approved')}
              loading={decideMutation.isPending}
            >
              <CheckCircle2 size={16} />
              Одобрить
            </Button>
          </div>
        </div>
      </Modal>

      {/* Отметка выплаченным */}
      <Modal open={payId !== null} onClose={() => setPayId(null)} title="Отметить выплаченным">
        <div className={MODAL_FORM_CLASSES}>
          <Field label="ID внешней транзакции" htmlFor="wd-txid">
            <Input
              id="wd-txid"
              value={externalTxId}
              onChange={(event) => setExternalTxId(event.target.value)}
            />
          </Field>
          <Field label="Комментарий" htmlFor="wd-pay-comment">
            <Textarea
              id="wd-pay-comment"
              rows={3}
              value={payComment}
              onChange={(event) => setPayComment(event.target.value)}
            />
          </Field>
          <div className={MODAL_ACTIONS_CLASSES}>
            <Button variant="secondary" onClick={() => setPayId(null)}>
              Отмена
            </Button>
            <Button onClick={() => void submitPay()} loading={payMutation.isPending}>
              <Banknote size={16} />
              Подтвердить выплату
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  )
}