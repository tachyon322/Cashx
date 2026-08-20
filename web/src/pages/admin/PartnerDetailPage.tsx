import { useState } from 'react'
import type { FormEvent } from 'react'
import { useParams } from 'react-router-dom'
import { Ban, CheckCircle2, Pencil, RotateCcw, ShieldCheck, Wallet } from 'lucide-react'
import { useAuth } from '../../auth/AuthContext'
import {
  useAdminOffers,
  useAdminPartner,
  useAdminPartnerRate,
  useAdminPartnerUpdate,
  useAdminWithdrawalDecide,
} from '../../api/queries'
import type { components } from '../../api/schema'
import { Badge } from '../../components/Badge'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { Field } from '../../components/Field'
import { Input } from '../../components/Input'
import { Modal } from '../../components/Modal'
import { Select } from '../../components/Select'
import { Skeleton } from '../../components/Skeleton'
import { StatCard } from '../../components/StatCard'
import { Table } from '../../components/Table'
import { Textarea } from '../../components/Textarea'
import { useToast } from '../../components/Toast'
import { formatBps, formatDateTime, formatRubles } from '../../lib/format'
import { offerStatus, withdrawalStatus } from '../../lib/status'

type AdminPartner = components['schemas']['AdminPartner']

const PAGE_CLASSES = 'flex w-full flex-col gap-4 p-4'
const GRID_STATS_CLASSES = 'grid grid-cols-1 gap-4 min-[768px]:grid-cols-2 min-[1200px]:grid-cols-4'
const MODAL_FORM_CLASSES = 'flex flex-col gap-4'
const MODAL_ACTIONS_CLASSES = 'mt-1 flex justify-end gap-3'

/** Статус партнёра: pending = !approved, blocked = is_blocked, active = approved && !blocked. */
function partnerStatus(p: AdminPartner): { label: string; tone: 'danger' | 'warning' | 'success' } {
  if (p.is_blocked) return { label: 'Заблокирован', tone: 'danger' }
  if (!p.is_approved) return { label: 'На модерации', tone: 'warning' }
  return { label: 'Активен', tone: 'success' }
}

const METHOD_LABEL: Record<string, string> = { usdt: 'USDT', sbp: 'СБП' }

export function PartnerDetailPage() {
  const { id = '' } = useParams<{ id: string }>()
  const toast = useToast()
  const { user } = useAuth()
  const staffRoles = user?.staff?.roles ?? []
  const canEdit = staffRoles.includes('superadmin') || staffRoles.includes('project_manager')

  const detail = useAdminPartner(id)
  const partner = detail.data?.partner
  const balance = detail.data?.balance
  const accesses = detail.data?.accesses ?? []
  const ledger = detail.data?.ledger ?? []
  const withdrawals = detail.data?.withdrawals ?? []

  const updateMutation = useAdminPartnerUpdate()
  const rateMutation = useAdminPartnerRate()
  const decideMutation = useAdminWithdrawalDecide()

  // Редактирование партнёра.
  const [editOpen, setEditOpen] = useState(false)
  const [editName, setEditName] = useState('')
  const [editEmail, setEditEmail] = useState('')
  const [editPassword, setEditPassword] = useState('')

  const openEdit = () => {
    setEditName(partner?.name ?? '')
    setEditEmail(partner?.email ?? '')
    setEditPassword('')
    setEditOpen(true)
  }

  const submitEdit = async (event: FormEvent) => {
    event.preventDefault()
    try {
      await updateMutation.mutateAsync({
        id,
        name: editName.trim(),
        email: editEmail.trim(),
        ...(editPassword ? { password: editPassword } : {}),
      })
      toast.success('Партнёр обновлён')
      setEditOpen(false)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Не удалось обновить партнёра')
    }
  }

  const toggleApproval = async (isApproved: boolean) => {
    try {
      await updateMutation.mutateAsync({ id, is_approved: isApproved })
      toast.success(isApproved ? 'Партнёр одобрен' : 'Одобрение отозвано')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Не удалось изменить статус')
    }
  }

  const toggleBlock = async (isBlocked: boolean) => {
    try {
      await updateMutation.mutateAsync({ id, is_blocked: isBlocked })
      toast.success(isBlocked ? 'Партнёр заблокирован' : 'Партнёр разблокирован')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Не удалось изменить статус')
    }
  }

  // Задание ставки по офферу.
  const [rateOpen, setRateOpen] = useState(false)
  const [rateOfferId, setRateOfferId] = useState('')
  const [rateBps, setRateBps] = useState('')
  const offersQuery = useAdminOffers()
  const offers = offersQuery.data?.items ?? []

  const openRate = () => {
    setRateOfferId('')
    setRateBps('')
    setRateOpen(true)
  }

  const submitRate = async (event: FormEvent) => {
    event.preventDefault()
    if (!rateOfferId) return
    try {
      await rateMutation.mutateAsync({ id, offer_id: rateOfferId, rate_bps: Number(rateBps) })
      toast.success('Ставка задана')
      setRateOpen(false)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Не удалось задать ставку')
    }
  }

  // Рассмотрение заявки на вывод.
  const [decideId, setDecideId] = useState<string | null>(null)
  const [decideComment, setDecideComment] = useState('')

  const submitDecide = async (decision: 'approved' | 'rejected') => {
    if (!decideId) return
    try {
      await decideMutation.mutateAsync({ id: decideId, decision, comment: decideComment || undefined })
      toast.success(decision === 'approved' ? 'Заявка одобрена' : 'Заявка отклонена')
      setDecideId(null)
      setDecideComment('')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Не удалось рассмотреть заявку')
    }
  }

  if (detail.isLoading && !partner) {
    return (
      <div className={PAGE_CLASSES}>
        <Skeleton style={{ height: 64 }} />
        <div className={GRID_STATS_CLASSES}>
          <Skeleton style={{ height: 96 }} />
          <Skeleton style={{ height: 96 }} />
        </div>
        <Skeleton style={{ height: 180 }} />
        <Skeleton style={{ height: 180 }} />
      </div>
    )
  }

  if (!partner) {
    return (
      <div className={PAGE_CLASSES}>
        <Card title="Партнёр не найден" subtitle="Возможно, он был удалён или ссылка неверна." />
      </div>
    )
  }

  const statusInfo = partnerStatus(partner)

  return (
    <div className={PAGE_CLASSES}>
      <Card>
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="flex flex-col gap-1">
            <div className="flex flex-wrap items-center gap-3 font-display text-[20px] font-semibold">
              {partner.name ?? '—'}
              <Badge tone={statusInfo.tone}>{statusInfo.label}</Badge>
            </div>
            <div className="text-[13.5px] text-muted">{partner.email ?? '—'}</div>
          </div>
          {canEdit && (
            <div className="flex flex-wrap items-center gap-3">
              {!partner.is_approved && (
                <Button onClick={() => void toggleApproval(true)} loading={updateMutation.isPending}>
                  <CheckCircle2 size={16} />
                  Одобрить
                </Button>
              )}
              {partner.is_blocked ? (
                <Button variant="secondary" onClick={() => void toggleBlock(false)} loading={updateMutation.isPending}>
                  <RotateCcw size={16} />
                  Разблокировать
                </Button>
              ) : (
                <Button variant="danger" onClick={() => void toggleBlock(true)} loading={updateMutation.isPending}>
                  <Ban size={16} />
                  Заблокировать
                </Button>
              )}
              <Button variant="secondary" onClick={openEdit}>
                <Pencil size={16} />
                Редактировать
              </Button>
            </div>
          )}
        </div>
      </Card>

      <div className={GRID_STATS_CLASSES}>
        <StatCard
          label="Доступно к выводу"
          value={balance ? formatRubles(balance.available_kopecks ?? 0) : '—'}
          display
          icon={<Wallet size={16} />}
        />
        <StatCard
          label="Зарезервировано"
          value={balance ? formatRubles(balance.reserved_kopecks ?? 0) : '—'}
          icon={<ShieldCheck size={16} />}
        />
      </div>

      <Card
        title="Ставки по офферам"
        actions={
          canEdit && (
            <Button variant="secondary" size="sm" onClick={openRate}>
              Задать ставку
            </Button>
          )
        }
      >
        <Table
          columns={[
            { key: 'offer_name', header: 'Оффер', render: (a) => a.offer_name ?? '—' },
            {
              key: 'rate_bps',
              header: 'Ставка',
              align: 'right',
              render: (a) => formatBps(a.rate_bps ?? 0),
            },
            {
              key: 'status',
              header: 'Статус',
              render: (a) => {
                const s = offerStatus(a.status)
                return <Badge tone={s.tone}>{s.label}</Badge>
              },
            },
          ]}
          rows={accesses}
          rowKey={(a) => a.offer_id ?? ''}
          emptyTitle="Ставки не заданы"
          emptyHint="Нажмите «Задать ставку», чтобы назначить партнёру индивидуальную ставку"
        />
      </Card>

      <Card title="История операций">
        <Table
          columns={[
            { key: 'type', header: 'Тип', render: (l) => l.type ?? '—' },
            {
              key: 'amount',
              header: 'Сумма',
              align: 'right',
              render: (l) => (
                <span className="tabular-nums">{formatRubles(l.amount_kopecks ?? 0)}</span>
              ),
            },
            {
              key: 'balance_after',
              header: 'Баланс после',
              align: 'right',
              render: (l) => (
                <span className="tabular-nums">
                  {formatRubles(l.balance_after_kopecks ?? 0)}
                </span>
              ),
            },
            {
              key: 'created_at',
              header: 'Дата',
              render: (l) => (l.created_at ? formatDateTime(l.created_at) : '—'),
            },
          ]}
          rows={ledger}
          rowKey={(l) => l.id ?? `${l.type}-${l.created_at}`}
          emptyTitle="Операций нет"
        />
      </Card>

      <Card title="Заявки на вывод">
        <Table
          columns={[
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
              header: 'Дата',
              render: (w) => (w.created_at ? formatDateTime(w.created_at) : '—'),
            },
            {
              key: 'actions',
              header: '',
              render: (w) =>
                canEdit && w.status === 'pending' ? (
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
                ) : null,
            },
          ]}
          rows={withdrawals}
          rowKey={(w) => w.id ?? ''}
          emptyTitle="Заявок на вывод нет"
        />
      </Card>

      <Modal open={editOpen} onClose={() => setEditOpen(false)} title="Редактировать партнёра">
        <form className={MODAL_FORM_CLASSES} onSubmit={submitEdit}>
          <Field label="Имя" htmlFor="pd-name">
            <Input
              id="pd-name"
              required
              value={editName}
              onChange={(event) => setEditName(event.target.value)}
            />
          </Field>
          <Field label="Email" htmlFor="pd-email">
            <Input
              id="pd-email"
              type="email"
              required
              value={editEmail}
              onChange={(event) => setEditEmail(event.target.value)}
            />
          </Field>
          <Field label="Новый пароль" hint="Оставьте пустым, чтобы не менять" htmlFor="pd-password">
            <Input
              id="pd-password"
              type="password"
              minLength={8}
              value={editPassword}
              onChange={(event) => setEditPassword(event.target.value)}
            />
          </Field>
          <div className={MODAL_ACTIONS_CLASSES}>
            <Button variant="secondary" onClick={() => setEditOpen(false)}>
              Отмена
            </Button>
            <Button type="submit" loading={updateMutation.isPending}>
              Сохранить
            </Button>
          </div>
        </form>
      </Modal>

      <Modal open={rateOpen} onClose={() => setRateOpen(false)} title="Задать ставку по офферу">
        <form className={MODAL_FORM_CLASSES} onSubmit={submitRate}>
          <Field label="Оффер" htmlFor="pd-offer">
            <Select
              id="pd-offer"
              required
              value={rateOfferId}
              onChange={(event) => setRateOfferId(event.target.value)}
            >
              <option value="" disabled>
                Выберите оффер
              </option>
              {offers.map((offer) => (
                <option key={offer.id} value={offer.id}>
                  {offer.project_name ? `${offer.project_name} — ` : ''}
                  {offer.name}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Ставка, бпс" hint="1% = 100 бпс" htmlFor="pd-rate">
            <Input
              id="pd-rate"
              type="number"
              required
              min={0}
              value={rateBps}
              onChange={(event) => setRateBps(event.target.value)}
            />
          </Field>
          <div className={MODAL_ACTIONS_CLASSES}>
            <Button variant="secondary" onClick={() => setRateOpen(false)}>
              Отмена
            </Button>
            <Button type="submit" loading={rateMutation.isPending}>
              Задать
            </Button>
          </div>
        </form>
      </Modal>

      <Modal
        open={decideId !== null}
        onClose={() => setDecideId(null)}
        title="Рассмотреть заявку на вывод"
      >
        <div className={MODAL_FORM_CLASSES}>
          <Field label="Комментарий" htmlFor="pd-comment">
            <Textarea
              id="pd-comment"
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
              Отклонить
            </Button>
            <Button
              onClick={() => void submitDecide('approved')}
              loading={decideMutation.isPending}
            >
              Одобрить
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  )
}