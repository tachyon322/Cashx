import { useState } from 'react'
import type { FormEvent } from 'react'
import { useSearchParams } from 'react-router-dom'
import { ExternalLink, Pencil, Plus, Trash2 } from 'lucide-react'
import { useAuth } from '../../auth/AuthContext'
import {
  useAdminOfferCreate,
  useAdminOffers,
  useAdminOfferUpdate,
  useAdminOfferDomains,
  useAdminProjectCreate,
  useAdminProjects,
  useAdminProjectUpdate,
  useCreateOfferDomain,
  useDeleteOfferDomain,
  useUpdateOfferDomain,
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
import { cx } from '../../lib/cx'
import { offerStatus, OFFER_STATUS } from '../../lib/status'

type OfferCard = components['schemas']['OfferCard']
type ProjectCard = components['schemas']['ProjectCard']
type OfferStatus = 'active' | 'available' | 'pending' | 'coming_soon'

interface OfferFormState {
  project_id: string
  name: string
  category: string
  description: string
  destination_url: string
  status: OfferStatus
}

interface ProjectFormState {
  slug: string
  name: string
  description: string
  destination_url: string
  is_active: boolean
}

const EMPTY_OFFER_FORM: OfferFormState = {
  project_id: '',
  name: '',
  category: '',
  description: '',
  destination_url: '',
  status: 'pending',
}

const EMPTY_PROJECT_FORM: ProjectFormState = {
  slug: '',
  name: '',
  description: '',
  destination_url: '',
  is_active: true,
}

const STATUS_OPTIONS: OfferStatus[] = ['active', 'available', 'pending', 'coming_soon']

type OfferModalState =
  | { kind: 'closed' }
  | { kind: 'create' }
  | { kind: 'edit'; id: string }

const MODAL_FORM_CLASSES = 'flex flex-col gap-4'
const MODAL_ROW_CLASSES = 'grid grid-cols-2 gap-3 max-[560px]:grid-cols-1'
const MODAL_ACTIONS_CLASSES = 'mt-1 flex justify-end gap-3'

function toProjectForm(p: ProjectCard): ProjectFormState {
  return {
    slug: p.slug ?? '',
    name: p.name ?? '',
    description: p.description ?? '',
    destination_url: p.destination_url ?? '',
    is_active: p.is_active ?? true,
  }
}

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

  const createOfferMutation = useAdminOfferCreate()
  const updateOfferMutation = useAdminOfferUpdate()
  const createProjectMutation = useAdminProjectCreate()
  const updateProjectMutation = useAdminProjectUpdate()

  // offer modal
  const [offerModal, setOfferModal] = useState<OfferModalState>({ kind: 'closed' })
  const [offerForm, setOfferForm] = useState<OfferFormState>(EMPTY_OFFER_FORM)

  // project modal
  const [projectModalOpen, setProjectModalOpen] = useState(false)
  const [editingProjectId, setEditingProjectId] = useState<string | null>(null)
  const [projectForm, setProjectForm] = useState<ProjectFormState>(EMPTY_PROJECT_FORM)

  const openCreateOffer = () => {
    setOfferForm({ ...EMPTY_OFFER_FORM, project_id: projectId })
    setOfferModal({ kind: 'create' })
  }

  const openEditOffer = (offer: OfferCard) => {
    setOfferForm({
      project_id: offer.project_id ?? '',
      name: offer.name ?? '',
      category: offer.category ?? '',
      description: offer.description ?? '',
      destination_url: offer.destination_url ?? '',
      status: (offer.status as OfferStatus) ?? 'pending',
    })
    setOfferModal({ kind: 'edit', id: offer.id ?? '' })
  }

  const closeOfferModal = () => setOfferModal({ kind: 'closed' })

  const setOffer = <K extends keyof OfferFormState>(key: K, value: OfferFormState[K]) =>
    setOfferForm((prev) => ({ ...prev, [key]: value }))

  const openCreateProject = () => {
    setProjectForm(EMPTY_PROJECT_FORM)
    setEditingProjectId(null)
    setProjectModalOpen(true)
  }

  const openEditProject = (p: ProjectCard) => {
    setProjectForm(toProjectForm(p))
    setEditingProjectId(p.id ?? null)
    setProjectModalOpen(true)
  }

  const setProject = <K extends keyof ProjectFormState>(key: K, value: ProjectFormState[K]) =>
    setProjectForm((prev) => ({ ...prev, [key]: value }))

  const submitOffer = async (event: FormEvent) => {
    event.preventDefault()
    const trimmedUrl = offerForm.destination_url.trim()
    if (!trimmedUrl) {
      toast.error('Укажите Destination URL оффера — обязательное поле')
      return
    }
    try {
      if (offerModal.kind === 'edit') {
        await updateOfferMutation.mutateAsync({
          id: offerModal.id,
          name: offerForm.name.trim(),
          category: offerForm.category.trim() || undefined,
          description: offerForm.description.trim() || undefined,
          destination_url: trimmedUrl,
          status: offerForm.status,
        })
        toast.success('Оффер обновлён')
      } else {
        await createOfferMutation.mutateAsync({
          project_id: offerForm.project_id,
          name: offerForm.name.trim(),
          category: offerForm.category.trim() || undefined,
          description: offerForm.description.trim() || undefined,
          destination_url: trimmedUrl,
          status: offerForm.status,
        })
        toast.success('Оффер создан')
      }
      closeOfferModal()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Не удалось сохранить оффер')
    }
  }

  const submitProject = async (event: FormEvent) => {
    event.preventDefault()
    try {
      if (editingProjectId !== null) {
        await updateProjectMutation.mutateAsync({
          id: editingProjectId,
          name: projectForm.name.trim(),
          description: projectForm.description.trim() || undefined,
          destination_url: projectForm.destination_url.trim(),
          is_active: projectForm.is_active,
        })
        toast.success('Проект обновлён')
      } else {
        await createProjectMutation.mutateAsync({
          slug: projectForm.slug.trim(),
          name: projectForm.name.trim(),
          description: projectForm.description.trim() || undefined,
          destination_url: projectForm.destination_url.trim(),
          is_active: projectForm.is_active,
        })
        toast.success('Проект создан')
      }
      setProjectModalOpen(false)
      setEditingProjectId(null)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Не удалось сохранить проект')
    }
  }

  const selectedProject = projects.find((p) => p.id === projectId) ?? null

  return (
    <div className="flex w-full flex-col gap-4 p-4">
      <div className="grid items-start gap-4 xl:grid-cols-[320px_minmax(0,1fr)]">
        {/* Левая панель — проекты */}
        <Card
          title="Проекты"
          subtitle={`Всего: ${projects.length}`}
          actions={
            canEdit && (
              <Button size="sm" onClick={openCreateProject}>
                <Plus size={14} />
                Создать
              </Button>
            )
          }
          className="overflow-hidden"
        >
          {projectsQuery.isLoading && projects.length === 0 ? (
            <div className="flex flex-col gap-2">
              <Skeleton style={{ height: 56 }} />
              <Skeleton style={{ height: 56 }} />
              <Skeleton style={{ height: 56 }} />
            </div>
          ) : (
            <div className="flex flex-col gap-1">
              <button
                type="button"
                onClick={() => setProjectId('')}
                className={cx(
                  'flex w-full items-center justify-between rounded-lg border px-3 py-2.5 text-left transition-colors',
                  projectId === ''
                    ? 'border-[rgba(168,85,247,0.35)] bg-[rgba(168,85,247,0.12)]'
                    : 'border-transparent hover:bg-white/[0.04]',
                )}
              >
                <div className="min-w-0">
                  <p className="text-[13px] font-semibold leading-none">Все проекты</p>
                  <p className="mt-1 text-[11px] text-faint">{items.length} офферов</p>
                </div>
                {projectId === '' && <span className="h-2 w-2 shrink-0 rounded-full bg-violet shadow-[0_0_8px_rgba(168,85,247,0.8)]" />}
              </button>

              <div className="my-1 h-px bg-border" />

              {projects.length === 0 ? (
                <p className="px-2 py-6 text-center text-[13px] text-faint">Проектов пока нет</p>
              ) : (
                projects.map((project) => {
                  const isActive = projectId === project.id
                  return (
                    <div
                      key={project.id}
                      className={cx(
                        'group flex items-center gap-2 rounded-lg border px-3 py-2.5 transition-colors',
                        isActive
                          ? 'border-[rgba(168,85,247,0.35)] bg-[rgba(168,85,247,0.12)]'
                          : 'border-transparent hover:bg-white/[0.04]',
                      )}
                    >
                      <button
                        type="button"
                        onClick={() => setProjectId(project.id ?? '')}
                        className="min-w-0 flex-1 text-left"
                      >
                        <div className="flex items-center gap-2">
                          <span className="truncate text-[13px] font-semibold leading-none">{project.name ?? '—'}</span>
                          {project.is_active ? (
                            <Badge tone="success" className="shrink-0 text-[10px]">Активен</Badge>
                          ) : (
                            <Badge tone="muted" className="shrink-0 text-[10px]">Неактивен</Badge>
                          )}
                        </div>
                        <p className="mt-1 truncate font-mono text-[11px] text-faint">{project.slug ?? '—'}</p>
                        {project.destination_url && (
                          <p className="mt-0.5 flex items-center gap-1 truncate text-[11px] text-lilac">
                            <span className="truncate">{project.destination_url}</span>
                            <ExternalLink size={10} className="shrink-0" />
                          </p>
                        )}
                      </button>
                      {canEdit && (
                        <button
                          type="button"
                          onClick={(e) => {
                            e.stopPropagation()
                            openEditProject(project)
                          }}
                          className="hidden h-7 w-7 shrink-0 items-center justify-center rounded-md text-faint hover:bg-surface-hover hover:text-text group-hover:inline-flex"
                          aria-label="Редактировать проект"
                        >
                          <Pencil size={13} />
                        </button>
                      )}
                    </div>
                  )
                })
              )}
            </div>
          )}
        </Card>

        {/* Правая панель — офферы */}
        <Card
          title={
            <span className="inline-flex items-center gap-2">
              Офферы
              {selectedProject && (
                <span className="rounded-md border border-[rgba(168,85,247,0.2)] bg-[rgba(168,85,247,0.08)] px-2 py-0.5 text-[11px] font-semibold text-violet-bright">
                  {selectedProject.name}
                </span>
              )}
            </span>
          }
          subtitle={
            selectedProject
              ? `Проекта · Всего: ${items.length} · Destination URL оффера обязателен (fallback на URL проекта если пусто)`
              : `Всего: ${items.length} · Destination URL оффера обязателен`
          }
          actions={
            canEdit && (
              <Button onClick={openCreateOffer} disabled={projects.length === 0 && !projectId}>
                <Plus size={16} />
                Создать оффер
              </Button>
            )
          }
        >
          {offers.isLoading && items.length === 0 ? (
            <div className="flex flex-col gap-2">
              <Skeleton style={{ height: 36 }} />
              <Skeleton style={{ height: 36 }} />
              <Skeleton style={{ height: 36 }} />
            </div>
          ) : (
            <Table
              columns={[
                { key: 'project_name', header: 'Проект', render: (o) => o.project_name ?? '—' },
                { key: 'name', header: 'Название', render: (o) => o.name ?? '—' },
                {
                  key: 'destination_url',
                  header: 'URL',
                  render: (o) =>
                    o.destination_url ? (
                      <span className="max-w-[220px] truncate break-all text-[12px] text-lilac" title={o.destination_url}>
                        {o.destination_url}
                      </span>
                    ) : (
                      <span className="text-[12px] text-faint" title="Наследует URL проекта">
                        — наследует
                      </span>
                    ),
                },
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
                  key: 'actions',
                  header: '',
                  render: (o) =>
                    canEdit ? (
                      <Button variant="ghost" size="sm" onClick={() => openEditOffer(o)}>
                        <Pencil size={14} />
                        Редактировать
                      </Button>
                    ) : null,
                },
              ]}
              rows={items}
              rowKey={(o) => o.id ?? ''}
              emptyTitle="Офферы не найдены"
              emptyHint={
                projects.length === 0
                  ? 'Сначала создайте проект слева'
                  : projectId
                    ? 'В этом проекте пока нет офферов — создайте первый'
                    : 'Создайте оффер или выберите проект слева'
              }
            />
          )}
        </Card>
      </div>

      {/* Модалка проекта */}
      <Modal
        open={projectModalOpen}
        onClose={() => {
          setProjectModalOpen(false)
          setEditingProjectId(null)
        }}
        title={editingProjectId !== null ? 'Редактировать проект' : 'Создать проект'}
        wide
      >
        <form className={MODAL_FORM_CLASSES} onSubmit={submitProject}>
          {editingProjectId === null && (
            <Field label="Slug" hint="Уникальный латинский идентификатор" htmlFor="pj-slug">
              <Input
                id="pj-slug"
                required
                value={projectForm.slug}
                onChange={(event) => setProject('slug', event.target.value)}
              />
            </Field>
          )}
          <div className={MODAL_ROW_CLASSES}>
            <Field label="Название" htmlFor="pj-name">
              <Input
                id="pj-name"
                required
                value={projectForm.name}
                onChange={(event) => setProject('name', event.target.value)}
              />
            </Field>
            <Field
              label="Destination URL"
              hint="Базовый URL проекта — обязателен. Если у оффера не задан свой URL, используется этот"
              htmlFor="pj-dest"
            >
              <Input
                id="pj-dest"
                type="url"
                required
                placeholder="https://example.com"
                value={projectForm.destination_url}
                onChange={(event) => setProject('destination_url', event.target.value)}
              />
            </Field>
          </div>
          <Field label="Описание" htmlFor="pj-desc">
            <Textarea
              id="pj-desc"
              rows={3}
              value={projectForm.description}
              onChange={(event) => setProject('description', event.target.value)}
            />
          </Field>
          <label className="flex cursor-pointer items-center gap-2 text-[13.5px] text-text">
            <input
              type="checkbox"
              className="h-[15px] w-[15px] accent-violet"
              checked={projectForm.is_active}
              onChange={(event) => setProject('is_active', event.target.checked)}
            />
            <span>Проект активен</span>
          </label>
          <div className={MODAL_ACTIONS_CLASSES}>
            <Button
              variant="secondary"
              onClick={() => {
                setProjectModalOpen(false)
                setEditingProjectId(null)
              }}
            >
              Отмена
            </Button>
            <Button type="submit" loading={createProjectMutation.isPending || updateProjectMutation.isPending}>
              Сохранить
            </Button>
          </div>
        </form>
      </Modal>

      {/* Модалка создания/редактирования оффера */}
      <Modal
        open={offerModal.kind === 'create' || offerModal.kind === 'edit'}
        onClose={closeOfferModal}
        title={offerModal.kind === 'edit' ? 'Редактировать оффер' : 'Создать оффер'}
        wide
      >
        <form className={MODAL_FORM_CLASSES} onSubmit={submitOffer}>
          <Field label="Проект" htmlFor="oa-project">
            <Select
              id="oa-project"
              required={offerModal.kind === 'create'}
              disabled={offerModal.kind === 'edit'}
              value={offerForm.project_id}
              onChange={(event) => setOffer('project_id', event.target.value)}
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
                value={offerForm.name}
                onChange={(event) => setOffer('name', event.target.value)}
              />
            </Field>
            <Field label="Категория" htmlFor="oa-category">
              <Input
                id="oa-category"
                value={offerForm.category}
                onChange={(event) => setOffer('category', event.target.value)}
              />
            </Field>
          </div>
          <div className={MODAL_ROW_CLASSES}>
            <Field
              label="Destination URL *"
              hint="Обязательно. Куда уходит трафик по /c/{code}. Должен быть на том же домене, что и URL проекта"
              htmlFor="oa-dest"
            >
              <Input
                id="oa-dest"
                type="url"
                required
                placeholder="https://example.com/landing"
                value={offerForm.destination_url}
                onChange={(event) => setOffer('destination_url', event.target.value)}
              />
            </Field>
            <Field label="Статус" htmlFor="oa-status">
              <Select
                id="oa-status"
                value={offerForm.status}
                onChange={(event) => setOffer('status', event.target.value as OfferStatus)}
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
              value={offerForm.description}
              onChange={(event) => setOffer('description', event.target.value)}
            />
          </Field>
          {offerModal.kind === 'edit' && <OfferDomainsSection offerId={offerModal.id} />}
          <div className={MODAL_ACTIONS_CLASSES}>
            <Button variant="secondary" onClick={closeOfferModal}>
              Отмена
            </Button>
            <Button type="submit" loading={createOfferMutation.isPending || updateOfferMutation.isPending}>
              Сохранить
            </Button>
          </div>
        </form>
      </Modal>
    </div>
  )
}

/* --- Домены оффера: один основной + запасные зеркала --- */

function OfferDomainsSection({ offerId }: { offerId: string }) {
  const toast = useToast()
  const domainsQuery = useAdminOfferDomains(offerId)
  const createDomain = useCreateOfferDomain(offerId)
  const updateDomain = useUpdateOfferDomain(offerId)
  const deleteDomain = useDeleteOfferDomain(offerId)

  const [url, setUrl] = useState('')
  const [isMain, setIsMain] = useState(false)

  const items = domainsQuery.data?.items ?? []
  const busy = createDomain.isPending || updateDomain.isPending || deleteDomain.isPending

  const addDomain = async () => {
    const trimmed = url.trim()
    if (!trimmed) {
      toast.error('Укажите домен, например https://litgmplay.fun')
      return
    }
    try {
      // Первый домен оффера автоматически становится основным.
      await createDomain.mutateAsync({ url: trimmed, is_main: isMain || items.length === 0 })
      setUrl('')
      setIsMain(false)
      toast.success('Домен добавлен')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Не удалось добавить домен')
    }
  }

  const makeMain = async (id: string) => {
    try {
      await updateDomain.mutateAsync({ id, is_main: true, is_active: true })
      toast.success('Основной домен обновлён')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Не удалось назначить основной домен')
    }
  }

  const toggleActive = async (id: string, is_active: boolean) => {
    try {
      await updateDomain.mutateAsync({ id, is_active })
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Не удалось изменить домен')
    }
  }

  const removeDomain = async (id: string) => {
    try {
      await deleteDomain.mutateAsync(id)
      toast.success('Домен удалён')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Не удалось удалить домен')
    }
  }

  return (
    <div className="flex flex-col gap-2.5 rounded-lg border border-border bg-surface-0 p-3.5">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <span className="text-[12px] font-bold uppercase tracking-[0.08em] text-faint">Домены оффера</span>
        <span className="text-[11px] text-faint">
          Основной — адрес ссылок по умолчанию и посадка кликов; запасные — на случай блокировки
        </span>
      </div>

      {items.length === 0 ? (
        <p className="py-1.5 text-[12.5px] text-faint">Доменов пока нет — ссылки строятся на трекере CashX</p>
      ) : (
        <div className="flex flex-col gap-1.5">
          {items.map((d) => (
            <div key={d.id} className="flex items-center gap-2.5 rounded-md border border-border bg-surface-1 px-3 py-2">
              <span className="min-w-0 flex-1 truncate font-mono text-[12px] text-violet-bright" title={d.url}>
                {d.url}
              </span>
              {d.is_main ? (
                <Badge tone="violet" className="shrink-0 text-[10px]">Основной</Badge>
              ) : (
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => void makeMain(d.id)}
                  className="shrink-0 rounded-full border border-border px-2 py-0.5 text-[10.5px] font-semibold text-muted transition-colors hover:bg-surface-hover hover:text-text disabled:opacity-50"
                >
                  Сделать основным
                </button>
              )}
              <label className="flex shrink-0 cursor-pointer items-center gap-1.5 text-[11.5px] text-muted">
                <input
                  type="checkbox"
                  className="h-3.5 w-3.5 accent-violet"
                  checked={d.is_active}
                  disabled={busy}
                  onChange={(event) => void toggleActive(d.id, event.target.checked)}
                />
                активен
              </label>
              <button
                type="button"
                disabled={busy}
                onClick={() => void removeDomain(d.id)}
                className="shrink-0 rounded-md px-1.5 py-1 text-[12px] text-muted transition-colors hover:bg-danger/10 hover:text-danger disabled:opacity-50"
                aria-label="Удалить домен"
              >
                <Trash2 size={13} />
              </button>
            </div>
          ))}
        </div>
      )}

      <div className="flex items-center gap-2 pt-0.5">
        <Input
          placeholder="https://litgmplay.fun"
          value={url}
          onChange={(event) => setUrl(event.target.value)}
          className="flex-1"
        />
        <label className="flex shrink-0 cursor-pointer items-center gap-1.5 text-[11.5px] text-muted">
          <input
            type="checkbox"
            className="h-3.5 w-3.5 accent-violet"
            checked={isMain}
            onChange={(event) => setIsMain(event.target.checked)}
          />
          основной
        </label>
        <Button type="button" size="sm" loading={createDomain.isPending} onClick={() => void addDomain()}>
          <Plus size={13} />
          Добавить
        </Button>
      </div>
    </div>
  )
}
