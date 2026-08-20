import { useState } from 'react'
import type { FormEvent } from 'react'
import { useSearchParams } from 'react-router-dom'
import { BadgePercent, Pencil, Plus } from 'lucide-react'
import { useAuth } from '../../auth/AuthContext'
import {
  useAdminOfferCreate,
  useAdminOffers,
  useAdminOfferTerms,
  useAdminOfferUpdate,
  useAdminProjects,
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
import { Table } from '../../components/Table'
import { Textarea } from '../../components/Textarea'
import { useToast } from '../../components/Toast'
import { formatBps, formatDate } from '../../lib/format'
import { OFFER_STATUS, offerStatus } from '../../lib/status'

type OfferCard = components['schemas']['OfferCard']
type OfferStatus = 'active' | 'available' | 'pending' | 'coming_soon'

interface OfferFormState {
  project_id: string
  name: string
  category: string
  description: string
  destination_url: string
  status: OfferStatus
}

const EMPTY_FORM: OfferFormState = {
  project_id: '',
  name: '',
  category: '',
  description: '',
  destination_url: '',
  status: 'pending',
}

const STATUS_OPTIONS: OfferStatus[] = ['active', 'available', 'pending', 'coming_soon']

type ModalState =
  | { kind: 'closed' }
  | { kind: 'create' }
  | { kind: 'edit'; id: string }
  | { kind: 'rate'; id: string }

const PAGE_CLASSES = 'flex w-full flex-col gap-4 p-4'
const TOOLBAR_CLASSES = 'flex flex-wrap items-end gap-3'
const SKELETON_CLASSES = 'flex flex-col gap-2'
const MODAL_FORM_CLASSES = 'flex flex-col gap-4'
const MODAL_ROW_CLASSES = 'grid grid-cols-2 gap-3 max-[560px]:grid-cols-1'
const MODAL_ACTIONS_CLASSES = 'mt-1 flex justify-end gap-3'

export function OffersAdminPage() {
  const toast = useToast()
  const { user } = useAuth()
  const staffRoles = user?.staff?.roles ?? []
  const canEdit = staffRoles.includes('superadmin') || staffRoles.includes('project_manager')

  const [searchParams, setSearchParams] = useSearchParams()
  const projectId = searchParams.get('project_id') ?? ''
  const setProjectId = (value: string) => {
    setSearchParams(value ? { project_id: value } : {}, { replace: true })
  }

  const projectsQuery = useAdminProjects()
  const projects = projectsQuery.data?.items ?? []

  const offers = useAdminOffers(projectId || undefined)
  const items = offers.data?.items ?? []

  const createMutation = useAdminOfferCreate()
  const updateMutation = useAdminOfferUpdate()
  const termsMutation = useAdminOfferTerms()

  const [modal, setModal] = useState<ModalState>({ kind: 'closed' })
  const [form, setForm] = useState<OfferFormState>(EMPTY_FORM)
  const [rateBps, setRateBps] = useState('')
  const [rateOfferName, setRateOfferName] = useState('')

  const openCreate = () => {
    setForm({ ...EMPTY_FORM, project_id: projectId })
    setRateBps('')
    setModal({ kind: 'create' })
  }

  const openEdit = (offer: OfferCard) => {
    setForm({
      project_id: offer.project_id ?? '',
      name: offer.name ?? '',
      category: offer.category ?? '',
      description: offer.description ?? '',
      destination_url: offer.destination_url ?? '',
      status: (offer.status as OfferStatus) ?? 'pending',
    })
    setModal({ kind: 'edit', id: offer.id ?? '' })
  }

  const openRate = (offer: OfferCard) => {
    setRateBps(String(offer.current_rate_bps ?? ''))
    setRateOfferName(offer.name ?? '')
    setModal({ kind: 'rate', id: offer.id ?? '' })
  }

  const closeModal = () => setModal({ kind: 'closed' })

  const set = <K extends keyof OfferFormState>(key: K, value: OfferFormState[K]) =>
    setForm((prev) => ({ ...prev, [key]: value }))

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    try {
      if (modal.kind === 'edit') {
        await updateMutation.mutateAsync({
          id: modal.id,
          name: form.name.trim(),
          category: form.category.trim() || undefined,
          description: form.description.trim() || undefined,
          destination_url: form.destination_url.trim() || undefined,
          status: form.status,
        })
        toast.success('Оффер обновлён')
      } else {
        await createMutation.mutateAsync({
          project_id: form.project_id,
          name: form.name.trim(),
          category: form.category.trim() || undefined,
          description: form.description.trim() || undefined,
          destination_url: form.destination_url.trim() || undefined,
          status: form.status,
          rate_bps: Number(rateBps),
        })
        toast.success('Оффер создан')
      }
      closeModal()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Не удалось сохранить оффер')
    }
  }

  const submitTerms = async (event: FormEvent) => {
    event.preventDefault()
    if (modal.kind !== 'rate') return
    try {
      const result = await termsMutation.mutateAsync({ id: modal.id, rate_bps: Number(rateBps) })
      if (result?.rate_bps != null && result.effective_from) {
        toast.success(`Ставка ${formatBps(result.rate_bps)} с ${formatDate(result.effective_from)}`)
      } else {
        toast.success('Ставка обновлена')
      }
      closeModal()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Не удалось обновить ставку')
    }
  }

  return (
    <div className={PAGE_CLASSES}>
      <Card
        title="Офферы"
        subtitle={`Всего: ${items.length}`}
        actions={
          canEdit && (
            <Button onClick={openCreate}>
              <Plus size={16} />
              Создать оффер
            </Button>
          )
        }
      >
        <div className={TOOLBAR_CLASSES}>
          <Field label="Проект" className="min-w-[240px]">
            <Select value={projectId} onChange={(event) => setProjectId(event.target.value)}>
              <option value="">Все проекты</option>
              {projects.map((project) => (
                <option key={project.id} value={project.id}>
                  {project.name}
                </option>
              ))}
            </Select>
          </Field>
        </div>

        {offers.isLoading && items.length === 0 ? (
          <div className={SKELETON_CLASSES}>
            <Skeleton style={{ height: 36 }} />
            <Skeleton style={{ height: 36 }} />
            <Skeleton style={{ height: 36 }} />
          </div>
        ) : (
          <Table
            columns={[
              { key: 'project_name', header: 'Проект', render: (o) => o.project_name ?? '—' },
              { key: 'name', header: 'Название', render: (o) => o.name ?? '—' },
              { key: 'category', header: 'Категория', render: (o) => o.category ?? '—' },
              {
                key: 'status',
                header: 'Статус',
                render: (o) => {
                  const s = offerStatus(o.status)
                  return <Badge tone={s.tone}>{s.label}</Badge>
                },
              },
              {
                key: 'current_rate_bps',
                header: 'Ставка',
                align: 'right',
                render: (o) => formatBps(o.current_rate_bps ?? 0),
              },
              {
                key: 'version',
                header: 'Версия',
                align: 'right',
                render: (o) => (o.version != null ? `v${o.version}` : '—'),
              },
              {
                key: 'actions',
                header: '',
                render: (o) =>
                  canEdit ? (
                    <div className="flex items-center gap-2 whitespace-nowrap">
                      <Button variant="ghost" size="sm" onClick={() => openEdit(o)}>
                        <Pencil size={14} />
                        Редактировать
                      </Button>
                      <Button variant="ghost" size="sm" onClick={() => openRate(o)}>
                        <BadgePercent size={14} />
                        Новая ставка
                      </Button>
                    </div>
                  ) : null,
              },
            ]}
            rows={items}
            rowKey={(o) => o.id ?? ''}
            emptyTitle="Офферы не найдены"
            emptyHint="Измените фильтр по проекту или создайте оффер"
          />
        )}
      </Card>

      {/* Модалка создания/редактирования оффера */}
      <Modal
        open={modal.kind === 'create' || modal.kind === 'edit'}
        onClose={closeModal}
        title={modal.kind === 'edit' ? 'Редактировать оффер' : 'Создать оффер'}
        wide
      >
        <form className={MODAL_FORM_CLASSES} onSubmit={submit}>
          <Field label="Проект" htmlFor="oa-project">
            <Select
              id="oa-project"
              required={modal.kind === 'create'}
              disabled={modal.kind === 'edit'}
              value={form.project_id}
              onChange={(event) => set('project_id', event.target.value)}
            >
              <option value="" disabled>
                Выберите проект
              </option>
              {projects.map((project) => (
                <option key={project.id} value={project.id}>
                  {project.name}
                </option>
              ))}
            </Select>
          </Field>
          <div className={MODAL_ROW_CLASSES}>
            <Field label="Название" htmlFor="oa-name">
              <Input
                id="oa-name"
                required
                value={form.name}
                onChange={(event) => set('name', event.target.value)}
              />
            </Field>
            <Field label="Категория" htmlFor="oa-category">
              <Input
                id="oa-category"
                value={form.category}
                onChange={(event) => set('category', event.target.value)}
              />
            </Field>
          </div>
          <div className={MODAL_ROW_CLASSES}>
            <Field label="Destination URL" htmlFor="oa-dest">
              <Input
                id="oa-dest"
                type="url"
                value={form.destination_url}
                onChange={(event) => set('destination_url', event.target.value)}
              />
            </Field>
            <Field label="Статус" htmlFor="oa-status">
              <Select
                id="oa-status"
                value={form.status}
                onChange={(event) => set('status', event.target.value as OfferStatus)}
              >
                {STATUS_OPTIONS.map((status) => (
                  <option key={status} value={status}>
                    {OFFER_STATUS[status].label}
                  </option>
                ))}
              </Select>
            </Field>
          </div>
          <Field label="Описание" htmlFor="oa-desc">
            <Textarea
              id="oa-desc"
              rows={3}
              value={form.description}
              onChange={(event) => set('description', event.target.value)}
            />
          </Field>
          {modal.kind === 'create' && (
            <Field label="Ставка, бпс" hint="1% = 100 бпс" htmlFor="oa-rate">
              <Input
                id="oa-rate"
                type="number"
                required
                min={0}
                value={rateBps}
                onChange={(event) => setRateBps(event.target.value)}
              />
            </Field>
          )}
          <div className={MODAL_ACTIONS_CLASSES}>
            <Button variant="secondary" onClick={closeModal}>
              Отмена
            </Button>
            <Button type="submit" loading={createMutation.isPending || updateMutation.isPending}>
              Сохранить
            </Button>
          </div>
        </form>
      </Modal>

      {/* Модалка «Новая ставка» */}
      <Modal open={modal.kind === 'rate'} onClose={closeModal} title="Новая ставка">
        <form className={MODAL_FORM_CLASSES} onSubmit={submitTerms}>
          <p className="text-[13px] text-muted">
            {rateOfferName}
          </p>
          <Field label="Ставка, бпс" hint="1% = 100 бпс" htmlFor="oa-terms-rate">
            <Input
              id="oa-terms-rate"
              type="number"
              required
              min={0}
              value={rateBps}
              onChange={(event) => setRateBps(event.target.value)}
            />
          </Field>
          <div className={MODAL_ACTIONS_CLASSES}>
            <Button variant="secondary" onClick={closeModal}>
              Отмена
            </Button>
            <Button type="submit" loading={termsMutation.isPending}>
              Применить
            </Button>
          </div>
        </form>
      </Modal>
    </div>
  )
}