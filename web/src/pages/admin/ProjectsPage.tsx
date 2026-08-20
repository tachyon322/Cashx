import { useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { ExternalLink, FolderKanban, Pencil, Plus } from 'lucide-react'
import { useAuth } from '../../auth/AuthContext'
import { useAdminProjectCreate, useAdminProjectUpdate, useAdminProjects } from '../../api/queries'
import type { components } from '../../api/schema'
import { Badge } from '../../components/Badge'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { Field } from '../../components/Field'
import { Input } from '../../components/Input'
import { Modal } from '../../components/Modal'
import { Skeleton } from '../../components/Skeleton'
import { Table } from '../../components/Table'
import { Textarea } from '../../components/Textarea'
import { useToast } from '../../components/Toast'
import { formatDate } from '../../lib/format'

type ProjectCard = components['schemas']['ProjectCard']

interface ProjectFormState {
  slug: string
  name: string
  description: string
  destination_url: string
  is_active: boolean
}

const EMPTY_FORM: ProjectFormState = {
  slug: '',
  name: '',
  description: '',
  destination_url: '',
  is_active: true,
}

const SKELETON_CLASSES = 'flex flex-col gap-2'
const MODAL_FORM_CLASSES = 'flex flex-col gap-4'
const MODAL_ROW_CLASSES = 'grid grid-cols-2 gap-3 max-[560px]:grid-cols-1'
const MODAL_ACTIONS_CLASSES = 'mt-1 flex justify-end gap-3'

function toForm(p: ProjectCard): ProjectFormState {
  return {
    slug: p.slug ?? '',
    name: p.name ?? '',
    description: p.description ?? '',
    destination_url: p.destination_url ?? '',
    is_active: p.is_active ?? true,
  }
}

export function ProjectsPage() {
  const navigate = useNavigate()
  const toast = useToast()
  const { user } = useAuth()
  const staffRoles = user?.staff?.roles ?? []
  const canEdit = staffRoles.includes('superadmin') || staffRoles.includes('project_manager')

  const projects = useAdminProjects()
  const items = projects.data?.items ?? []

  const createMutation = useAdminProjectCreate()
  const updateMutation = useAdminProjectUpdate()

  const [createOpen, setCreateOpen] = useState(false)
  const [editId, setEditId] = useState<string | null>(null)
  const [form, setForm] = useState<ProjectFormState>(EMPTY_FORM)

  const openCreate = () => {
    setForm(EMPTY_FORM)
    setCreateOpen(true)
  }

  const openEdit = (project: ProjectCard) => {
    setForm(toForm(project))
    setEditId(project.id ?? null)
  }

  const set = <K extends keyof ProjectFormState>(key: K, value: ProjectFormState[K]) =>
    setForm((prev) => ({ ...prev, [key]: value }))

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    try {
      if (editId !== null) {
        await updateMutation.mutateAsync({
          id: editId,
          name: form.name.trim(),
          description: form.description.trim() || undefined,
          destination_url: form.destination_url.trim(),
          is_active: form.is_active,
        })
        toast.success('Проект обновлён')
      } else {
        await createMutation.mutateAsync({
          slug: form.slug.trim(),
          name: form.name.trim(),
          description: form.description.trim() || undefined,
          destination_url: form.destination_url.trim(),
          is_active: form.is_active,
        })
        toast.success('Проект создан')
      }
      setCreateOpen(false)
      setEditId(null)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Не удалось сохранить проект')
    }
  }

  return (
    <div className="flex w-full flex-col gap-4 p-4">
      <Card
        title="Проекты"
        subtitle={`Всего: ${items.length}`}
        actions={
          canEdit && (
            <Button onClick={openCreate}>
              <Plus size={16} />
              Создать проект
            </Button>
          )
        }
      >
        {projects.isLoading && items.length === 0 ? (
          <div className={SKELETON_CLASSES}>
            <Skeleton style={{ height: 36 }} />
            <Skeleton style={{ height: 36 }} />
            <Skeleton style={{ height: 36 }} />
          </div>
        ) : (
          <Table
            columns={[
              { key: 'slug', header: 'Slug', render: (p) => <span className="tabular-nums">{p.slug ?? '—'}</span> },
              { key: 'name', header: 'Название', render: (p) => p.name ?? '—' },
              { key: 'description', header: 'Описание', render: (p) => p.description ?? '—' },
              {
                key: 'destination_url',
                header: 'Ссылка',
                render: (p) =>
                  p.destination_url ? (
                    <a
                      className="break-all text-lilac"
                      href={p.destination_url}
                      target="_blank"
                      rel="noreferrer"
                      onClick={(event) => event.stopPropagation()}
                    >
                      {p.destination_url}
                      <ExternalLink size={12} style={{ marginLeft: 4, verticalAlign: -1 }} />
                    </a>
                  ) : (
                    '—'
                  ),
              },
              {
                key: 'is_active',
                header: 'Статус',
                render: (p) =>
                  p.is_active ? (
                    <Badge tone="success">Активен</Badge>
                  ) : (
                    <Badge tone="muted">Неактивен</Badge>
                  ),
              },
              {
                key: 'created_at',
                header: 'Создан',
                render: (p) => (p.created_at ? formatDate(p.created_at) : '—'),
              },
              {
                key: 'actions',
                header: '',
                render: (p) => (
                  <div className="flex items-center gap-2 whitespace-nowrap">
                    {canEdit && (
                      <Button variant="ghost" size="sm" onClick={() => openEdit(p)}>
                        <Pencil size={14} />
                        Редактировать
                      </Button>
                    )}
                    {p.id && (
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => navigate(`/admin/offers?project_id=${p.id}`)}
                      >
                        <FolderKanban size={14} />
                        Офферы проекта
                      </Button>
                    )}
                  </div>
                ),
              },
            ]}
            rows={items}
            rowKey={(p) => p.id ?? p.slug ?? ''}
            emptyTitle="Проекты не найдены"
            emptyHint="Создайте первый проект"
          />
        )}
      </Card>

      <Modal
        open={createOpen}
        onClose={() => {
          setCreateOpen(false)
          setEditId(null)
        }}
        title={editId !== null ? 'Редактировать проект' : 'Создать проект'}
        wide
      >
        <form className={MODAL_FORM_CLASSES} onSubmit={submit}>
          {editId === null && (
            <Field label="Slug" hint="Уникальный латинский идентификатор" htmlFor="pj-slug">
              <Input
                id="pj-slug"
                required
                value={form.slug}
                onChange={(event) => set('slug', event.target.value)}
              />
            </Field>
          )}
          <div className={MODAL_ROW_CLASSES}>
            <Field label="Название" htmlFor="pj-name">
              <Input
                id="pj-name"
                required
                value={form.name}
                onChange={(event) => set('name', event.target.value)}
              />
            </Field>
            <Field label="Destination URL" htmlFor="pj-dest">
              <Input
                id="pj-dest"
                type="url"
                required
                value={form.destination_url}
                onChange={(event) => set('destination_url', event.target.value)}
              />
            </Field>
          </div>
          <Field label="Описание" htmlFor="pj-desc">
            <Textarea
              id="pj-desc"
              rows={3}
              value={form.description}
              onChange={(event) => set('description', event.target.value)}
            />
          </Field>
          <label className="flex cursor-pointer items-center gap-2 text-[13.5px] text-text">
            <input
              type="checkbox"
              className="h-[15px] w-[15px] accent-violet"
              checked={form.is_active}
              onChange={(event) => set('is_active', event.target.checked)}
            />
            <span>Проект активен</span>
          </label>
          <div className={MODAL_ACTIONS_CLASSES}>
            <Button
              variant="secondary"
              onClick={() => {
                setCreateOpen(false)
                setEditId(null)
              }}
            >
              Отмена
            </Button>
            <Button type="submit" loading={createMutation.isPending || updateMutation.isPending}>
              Сохранить
            </Button>
          </div>
        </form>
      </Modal>
    </div>
  )
}