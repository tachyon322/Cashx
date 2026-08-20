import { useState } from 'react'
import type { FormEvent } from 'react'
import { Megaphone, Pencil, Plus, Trash2 } from 'lucide-react'
import {
  useAdminAnnouncementCreate,
  useAdminAnnouncementDelete,
  useAdminAnnouncements,
  useAdminAnnouncementUpdate,
  useAdminPartners,
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
import { Textarea } from '../../components/Textarea'
import { useToast } from '../../components/Toast'
import { formatDateTime } from '../../lib/format'
import { cx } from '../../lib/cx'

type Announcement = components['schemas']['Announcement']

type Audience = 'all' | 'partners' | 'staff' | 'specific_partner'

const AUDIENCE_LABEL: Record<string, string> = {
  all: 'Все',
  partners: 'Партнёры',
  staff: 'Сотрудники',
  specific_partner: 'Конкретный партнёр',
}

const AUDIENCE_OPTIONS: Audience[] = ['all', 'partners', 'staff', 'specific_partner']

interface FormState {
  title: string
  body: string
  audience: Audience
  partner_ids: string[]
}

const EMPTY_FORM: FormState = { title: '', body: '', audience: 'all', partner_ids: [] }

const PAGE_CLASSES = 'flex w-full flex-col gap-4 p-4'
const SKELETON_CLASSES = 'flex flex-col gap-2'
const MODAL_FORM_CLASSES = 'flex flex-col gap-4'
const MODAL_ACTIONS_CLASSES = 'mt-1 flex justify-end gap-3'

export function AnnouncementsPage() {
  const toast = useToast()

  const announcements = useAdminAnnouncements()
  const items = announcements.data?.items ?? []

  const partnersQuery = useAdminPartners({ limit: 200 })
  const partnerOptions = partnersQuery.data?.items ?? []

  const createMutation = useAdminAnnouncementCreate()
  const updateMutation = useAdminAnnouncementUpdate()
  const deleteMutation = useAdminAnnouncementDelete()

  const [modal, setModal] = useState<{ kind: 'closed' } | { kind: 'create' } | { kind: 'edit'; id: string }>({
    kind: 'closed',
  })
  const [deleteId, setDeleteId] = useState<string | null>(null)
  const [form, setForm] = useState<FormState>(EMPTY_FORM)

  const openCreate = () => {
    setForm(EMPTY_FORM)
    setModal({ kind: 'create' })
  }

  const openEdit = (announcement: Announcement) => {
    setForm({
      title: announcement.title ?? '',
      body: announcement.body ?? '',
      audience: (announcement.audience as Audience) ?? 'all',
      partner_ids: [],
    })
    setModal({ kind: 'edit', id: announcement.id ?? '' })
  }

  const closeModal = () => setModal({ kind: 'closed' })

  const set = <K extends keyof FormState>(key: K, value: FormState[K]) =>
    setForm((prev) => ({ ...prev, [key]: value }))

  const togglePartner = (id: string) => {
    setForm((prev) => ({
      ...prev,
      partner_ids: prev.partner_ids.includes(id)
        ? prev.partner_ids.filter((pid) => pid !== id)
        : [...prev.partner_ids, id],
    }))
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    const input = {
      title: form.title.trim() || undefined,
      body: form.body.trim() || undefined,
      audience: form.audience,
      partner_ids: form.audience === 'specific_partner' ? form.partner_ids : undefined,
    }
    try {
      if (modal.kind === 'edit') {
        await updateMutation.mutateAsync({ id: modal.id, ...input })
        toast.success('Анонс обновлён')
      } else {
        await createMutation.mutateAsync(input)
        toast.success('Анонс создан')
      }
      closeModal()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Не удалось сохранить анонс')
    }
  }

  const runDelete = async () => {
    if (!deleteId) return
    try {
      await deleteMutation.mutateAsync(deleteId)
      toast.success('Анонс удалён')
      setDeleteId(null)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Не удалось удалить анонс')
    }
  }

  return (
    <div className={PAGE_CLASSES}>
      <Card
        title="Анонсы"
        subtitle={`Всего: ${items.length}`}
        actions={
          <Button onClick={openCreate}>
            <Plus size={16} />
            Создать анонс
          </Button>
        }
      >
        {announcements.isLoading && items.length === 0 ? (
          <div className={SKELETON_CLASSES}>
            <Skeleton style={{ height: 84 }} />
            <Skeleton style={{ height: 84 }} />
          </div>
        ) : (
          <div className="flex flex-col gap-4">
            {items.map((announcement) => {
              const deleted = Boolean(announcement.deleted_at)
              return (
                <Card key={announcement.id}>
                  <div className="flex flex-col gap-2">
                    <div className={cx('text-[15px] font-semibold', deleted && 'opacity-55 line-through')}>
                      {announcement.title ?? 'Без заголовка'}
                    </div>
                    {announcement.body && (
                      <div className={cx('text-muted', deleted && 'opacity-55 line-through')}>{announcement.body}</div>
                    )}
                    <div className="flex flex-wrap items-center gap-3 text-[12.5px]">
                      <Badge tone="violet">
                        <Megaphone size={11} />
                        {AUDIENCE_LABEL[announcement.audience ?? ''] ?? announcement.audience ?? '—'}
                      </Badge>
                      {announcement.is_published ? (
                        <Badge tone="success">Опубликован</Badge>
                      ) : (
                        <Badge tone="muted">Черновик</Badge>
                      )}
                      {deleted && <Badge tone="danger">Удалён</Badge>}
                      <span className="text-faint">
                        {announcement.created_at ? `Создан ${formatDateTime(announcement.created_at)}` : ''}
                        {announcement.updated_at && announcement.updated_at !== announcement.created_at
                          ? ` · изменён ${formatDateTime(announcement.updated_at)}`
                          : ''}
                      </span>
                    </div>
                    <div className="mt-1 flex gap-2">
                      <Button variant="secondary" size="sm" onClick={() => openEdit(announcement)}>
                        <Pencil size={14} />
                        Редактировать
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        disabled={deleted}
                        onClick={() => setDeleteId(announcement.id ?? null)}
                      >
                        <Trash2 size={14} />
                        Удалить
                      </Button>
                    </div>
                  </div>
                </Card>
              )
            })}
          </div>
        )}
      </Card>

      <Modal
        open={modal.kind === 'create' || modal.kind === 'edit'}
        onClose={closeModal}
        title={modal.kind === 'edit' ? 'Редактировать анонс' : 'Создать анонс'}
        wide
      >
        <form className={MODAL_FORM_CLASSES} onSubmit={submit}>
          <Field label="Заголовок" htmlFor="an-title">
            <Input
              id="an-title"
              value={form.title}
              onChange={(event) => set('title', event.target.value)}
            />
          </Field>
          <Field label="Текст" htmlFor="an-body">
            <Textarea
              id="an-body"
              rows={4}
              value={form.body}
              onChange={(event) => set('body', event.target.value)}
            />
          </Field>
          <Field label="Аудитория" htmlFor="an-audience">
            <Select
              id="an-audience"
              value={form.audience}
              onChange={(event) => set('audience', event.target.value as Audience)}
            >
              {AUDIENCE_OPTIONS.map((audience) => (
                <option key={audience} value={audience}>
                  {AUDIENCE_LABEL[audience]}
                </option>
              ))}
            </Select>
          </Field>
          {form.audience === 'specific_partner' && (
            <Field label="Партнёры" hint={`Выбрано: ${form.partner_ids.length}`}>
              {partnersQuery.isLoading ? (
                <Skeleton style={{ height: 80 }} />
              ) : (
                <div className="flex max-h-[220px] flex-col gap-1 overflow-y-auto rounded-md border border-border bg-surface-1 p-1.5">
                  {partnerOptions.map((partner) => (
                    <label
                      key={partner.id}
                      className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-[13px] transition-colors duration-150 hover:bg-surface-hover"
                    >
                      <input
                        type="checkbox"
                        className="h-[14px] w-[14px] shrink-0 accent-violet"
                        checked={form.partner_ids.includes(partner.id ?? '')}
                        onChange={() => partner.id && togglePartner(partner.id)}
                      />
                      <span>
                        {partner.name ?? '—'}
                        <span className="text-faint"> · {partner.email ?? '—'}</span>
                      </span>
                    </label>
                  ))}
                  {partnerOptions.length === 0 && (
                    <div className="p-2 text-faint">
                      Партнёров пока нет
                    </div>
                  )}
                </div>
              )}
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

      <Modal open={deleteId !== null} onClose={() => setDeleteId(null)} title="Удалить анонс?">
        <div className={MODAL_FORM_CLASSES}>
          <p className="text-[13.5px] text-muted">
            Анонс будет удалён без возможности восстановления.
          </p>
          <div className={MODAL_ACTIONS_CLASSES}>
            <Button variant="secondary" onClick={() => setDeleteId(null)}>
              Отмена
            </Button>
            <Button variant="danger" onClick={() => void runDelete()} loading={deleteMutation.isPending}>
              <Trash2 size={16} />
              Удалить
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  )
}