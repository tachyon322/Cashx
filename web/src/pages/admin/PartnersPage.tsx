import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus, Search } from 'lucide-react'
import { useAuth } from '../../auth/AuthContext'
import { useAdminPartnerCreate, useAdminPartners } from '../../api/queries'
import type { AdminPartnersParams } from '../../api/queries'
import type { components } from '../../api/schema'
import { Badge } from '../../components/Badge'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { Field } from '../../components/Field'
import { Input } from '../../components/Input'
import { Modal } from '../../components/Modal'
import { PAGE_SIZE, Pagination } from '../../components/Pagination'
import { Select } from '../../components/Select'
import { Skeleton } from '../../components/Skeleton'
import { Table } from '../../components/Table'
import { useToast } from '../../components/Toast'
import { formatDate, formatRubles } from '../../lib/format'
import type { Tone } from '../../lib/status'

type AdminPartner = components['schemas']['AdminPartner']

type StatusFilter = 'pending' | 'active' | 'blocked'

const TOOLBAR_CLASSES = 'flex flex-wrap items-end gap-3'
const SKELETON_CLASSES = 'flex flex-col gap-2'
const MODAL_FORM_CLASSES = 'flex flex-col gap-4'
const MODAL_ACTIONS_CLASSES = 'mt-1 flex justify-end gap-3'

/** Статус партнёра: pending = !approved, blocked = is_blocked, active = approved && !blocked. */
function partnerStatus(p: AdminPartner): { label: string; tone: Tone } {
  if (p.is_blocked) return { label: 'Заблокирован', tone: 'danger' }
  if (!p.is_approved) return { label: 'На модерации', tone: 'warning' }
  return { label: 'Активен', tone: 'success' }
}

const STATUS_OPTIONS: { value: StatusFilter; label: string }[] = [
  { value: 'pending', label: 'На модерации' },
  { value: 'active', label: 'Активен' },
  { value: 'blocked', label: 'Заблокирован' },
]

export function PartnersPage() {
  const navigate = useNavigate()
  const toast = useToast()
  const { user } = useAuth()
  const staffRoles = user?.staff?.roles ?? []
  const canEdit = staffRoles.includes('superadmin') || staffRoles.includes('project_manager')

  // Фильтры: локальное состояние + отложенное применение (300 мс) сбросом offset.
  const [search, setSearch] = useState('')
  const [status, setStatus] = useState<StatusFilter | ''>('')
  const [applied, setApplied] = useState<{ search: string; status?: StatusFilter }>({
    search: '',
    status: undefined,
  })
  const [offset, setOffset] = useState(0)

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setApplied({ search: search.trim(), status: status || undefined })
      setOffset(0)
    }, 300)
    return () => window.clearTimeout(timer)
  }, [search, status])

  const params: AdminPartnersParams = {
    limit: PAGE_SIZE,
    offset,
    ...(applied.search ? { search: applied.search } : {}),
    ...(applied.status ? { status: applied.status } : {}),
  }
  const partners = useAdminPartners(params)
  const items = partners.data?.items ?? []
  const total = partners.data?.total ?? 0

  // Модалка создания.
  const [createOpen, setCreateOpen] = useState(false)
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [commissionBps, setCommissionBps] = useState('')
  const createMutation = useAdminPartnerCreate()

  const submitCreate = async (event: FormEvent) => {
    event.preventDefault()
    try {
      await createMutation.mutateAsync({
        name: name.trim(),
        email: email.trim(),
        password,
        ...(commissionBps.trim() ? { commission_bps: Number(commissionBps) } : {}),
      })
      toast.success('Партнёр создан')
      setCreateOpen(false)
      setName('')
      setEmail('')
      setPassword('')
      setCommissionBps('')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Не удалось создать партнёра')
    }
  }

  return (
    <div className="flex w-full flex-col gap-4 p-4">
      <Card
        title="Партнёры"
        subtitle={`Всего: ${total}`}
        actions={
          canEdit && (
            <Button onClick={() => setCreateOpen(true)}>
              <Plus size={16} />
              Создать партнёра
            </Button>
          )
        }
      >
        <div className={TOOLBAR_CLASSES}>
          <div className="min-w-[240px] max-w-[420px] flex-1">
            <Field label="Поиск">
              <div className="relative">
                <Search size={15} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
                <Input
                  type="search"
                  placeholder="Имя или email"
                  className="pl-[34px]"
                  value={search}
                  onChange={(event) => setSearch(event.target.value)}
                />
              </div>
            </Field>
          </div>
          <Field label="Статус" className="min-w-[180px] max-w-[320px] flex-1">
            <Select
              value={status}
              onChange={(event) => setStatus(event.target.value as StatusFilter | '')}
            >
              <option value="">Все</option>
              {STATUS_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </Select>
          </Field>
        </div>

        {partners.isLoading && items.length === 0 ? (
          <div className={SKELETON_CLASSES}>
            <Skeleton style={{ height: 36 }} />
            <Skeleton style={{ height: 36 }} />
            <Skeleton style={{ height: 36 }} />
            <Skeleton style={{ height: 36 }} />
          </div>
        ) : (
          <Table
            columns={[
              { key: 'name', header: 'Имя', render: (p) => p.name ?? '—' },
              { key: 'email', header: 'Email', render: (p) => p.email ?? '—' },
              {
                key: 'status',
                header: 'Статус',
                render: (p) => {
                  const statusInfo = partnerStatus(p)
                  return <Badge tone={statusInfo.tone}>{statusInfo.label}</Badge>
                },
              },
              {
                key: 'revshare',
                header: 'RevShare',
                align: 'right',
                render: (p) => {
                  const bps = (p as any).revshare_percent_bps ?? 4000
                  return `${(bps / 100).toLocaleString('ru-RU', { maximumFractionDigits: 1 })}%`
                },
              },
              {
                key: 'balance',
                header: 'Баланс',
                align: 'right',
                render: (p) => formatRubles(p.balance_kopecks ?? 0),
              },
              {
                key: 'created_at',
                header: 'Регистрация',
                render: (p) => (p.created_at ? formatDate(p.created_at) : '—'),
              },
            ]}
            rows={items}
            rowKey={(p) => p.id ?? ''}
            onRowClick={(p) => p.id && navigate(`/admin/partners/${p.id}`)}
            emptyTitle="Партнёры не найдены"
            emptyHint="Измените фильтры или создайте первого партнёра"
          />
        )}

        <Pagination total={total} offset={offset} limit={PAGE_SIZE} onChange={setOffset} />
      </Card>

      <Modal open={createOpen} onClose={() => setCreateOpen(false)} title="Создать партнёра">
        <form className={MODAL_FORM_CLASSES} onSubmit={submitCreate}>
          <Field label="Имя" htmlFor="pp-name">
            <Input
              id="pp-name"
              required
              autoFocus
              value={name}
              onChange={(event) => setName(event.target.value)}
            />
          </Field>
          <Field label="Email" htmlFor="pp-email">
            <Input
              id="pp-email"
              type="email"
              required
              value={email}
              onChange={(event) => setEmail(event.target.value)}
            />
          </Field>
          <Field label="Пароль" htmlFor="pp-password">
            <Input
              id="pp-password"
              type="password"
              required
              minLength={8}
              value={password}
              onChange={(event) => setPassword(event.target.value)}
            />
          </Field>
          <Field label="RevShare, бпс" hint="Глобально для юзера на всех офферах; 1% = 100 бпс · 4000 = 40%. Пусто = 4000 по умолчанию" htmlFor="pp-commission">
            <Input
              id="pp-commission"
              type="number"
              min={0}
              value={commissionBps}
              onChange={(event) => setCommissionBps(event.target.value)}
            />
          </Field>
          <div className={MODAL_ACTIONS_CLASSES}>
            <Button variant="secondary" onClick={() => setCreateOpen(false)}>
              Отмена
            </Button>
            <Button type="submit" loading={createMutation.isPending}>
              Создать
            </Button>
          </div>
        </form>
      </Modal>
    </div>
  )
}