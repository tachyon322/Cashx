import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { AlertTriangle, Plus, RefreshCw, ShieldOff } from 'lucide-react'
import {
  useAdminIntegrationKeyCreate,
  useAdminIntegrationKeyDeactivate,
  useAdminIntegrationKeyRotate,
  useAdminIntegrationKeys,
  useAdminProjects,
} from '../../api/queries'
import type { components } from '../../api/schema'
import { Badge } from '../../components/Badge'
import { Button } from '../../components/Button'
import { Card } from '../../components/Card'
import { CopyButton } from '../../components/CopyButton'
import { Field } from '../../components/Field'
import { Modal } from '../../components/Modal'
import { Select } from '../../components/Select'
import { Skeleton } from '../../components/Skeleton'
import { Table } from '../../components/Table'
import { useToast } from '../../components/Toast'
import { formatDateTime } from '../../lib/format'

type IntegrationKey = components['schemas']['IntegrationKey']

interface SecretState {
  key_id?: string
  secret?: string
}

const PAGE_CLASSES = 'flex w-full flex-col gap-4 p-4'
const TOOLBAR_CLASSES = 'flex flex-wrap items-end gap-3'
const SKELETON_CLASSES = 'flex flex-col gap-2'
const MODAL_FORM_CLASSES = 'flex flex-col gap-4'
const MODAL_ACTIONS_CLASSES = 'mt-1 flex justify-end gap-3'

export function IntegrationKeysPage() {
  const toast = useToast()
  const [searchParams, setSearchParams] = useSearchParams()
  const projectId = searchParams.get('project_id') ?? ''

  const projectsQuery = useAdminProjects()
  const projects = projectsQuery.data?.items ?? []
  const currentProject = projects.find((p) => p.id === projectId)

  const keys = useAdminIntegrationKeys(projectId)
  const items = keys.data?.items ?? []

  const createMutation = useAdminIntegrationKeyCreate()
  const rotateMutation = useAdminIntegrationKeyRotate()
  const deactivateMutation = useAdminIntegrationKeyDeactivate()

  // Одноразовый секрет (после создания/ротации).
  const [secret, setSecret] = useState<SecretState | null>(null)
  // Подтверждение ротации/деактивации.
  const [confirm, setConfirm] = useState<{ action: 'rotate' | 'deactivate'; key: IntegrationKey } | null>(null)

  const runCreate = async () => {
    try {
      const result = await createMutation.mutateAsync(projectId)
      setSecret(result ?? {})
      toast.success('Ключ создан')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Не удалось создать ключ')
    }
  }

  const runRotate = async () => {
    if (!confirm) return
    try {
      const result = await rotateMutation.mutateAsync(confirm.key.key_id ?? '')
      setConfirm(null)
      setSecret(result ?? {})
      toast.success('Ключ ротирован')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Не удалось ротировать ключ')
    }
  }

  const runDeactivate = async () => {
    if (!confirm) return
    try {
      await deactivateMutation.mutateAsync(confirm.key.key_id ?? '')
      toast.success('Ключ деактивирован')
      setConfirm(null)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Не удалось деактивировать ключ')
    }
  }

  // Экран выбора проекта: ключи привязаны к проекту.
  if (!projectId) {
    return (
      <div className={PAGE_CLASSES}>
        <Card
          title="Интеграционные ключи"
          subtitle="Ключи привязаны к проекту — выберите проект"
        >
          <div className={TOOLBAR_CLASSES}>
            <Field label="Проект" htmlFor="ik-project" className="min-w-[280px] max-w-[480px] flex-1">
              <Select
                id="ik-project"
                value=""
                onChange={(event) => setSearchParams({ project_id: event.target.value })}
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
          </div>
          {projectsQuery.isLoading && <Skeleton style={{ height: 40 }} />}
        </Card>
      </div>
    )
  }

  return (
    <div className={PAGE_CLASSES}>
      <Card
        title="Интеграционные ключи"
        subtitle={currentProject ? `Проект: ${currentProject.name}` : `Проект: ${projectId}`}
        actions={
          <Button onClick={() => void runCreate()} loading={createMutation.isPending}>
            <Plus size={16} />
            Создать ключ
          </Button>
        }
      >
        {keys.isLoading && items.length === 0 ? (
          <div className={SKELETON_CLASSES}>
            <Skeleton style={{ height: 36 }} />
            <Skeleton style={{ height: 36 }} />
          </div>
        ) : (
          <Table
            columns={[
              {
                key: 'key_id',
                header: 'Ключ',
                render: (k) => <span className="tabular-nums">{k.key_id ?? '—'}</span>,
              },
              {
                key: 'is_active',
                header: 'Статус',
                render: (k) =>
                  k.is_active ? (
                    <Badge tone="success">Активен</Badge>
                  ) : (
                    <Badge tone="muted">Неактивен</Badge>
                  ),
              },
              {
                key: 'created_at',
                header: 'Создан',
                render: (k) => (k.created_at ? formatDateTime(k.created_at) : '—'),
              },
              {
                key: 'last_used_at',
                header: 'Последнее использование',
                render: (k) => (k.last_used_at ? formatDateTime(k.last_used_at) : '—'),
              },
              {
                key: 'secret_hint',
                header: 'Секрет',
                render: (k) => <span className="tabular-nums">{k.secret_hint ?? '—'}</span>,
              },
              {
                key: 'actions',
                header: '',
                render: (k) => (
                  <div className="flex items-center gap-2 whitespace-nowrap">
                    <Button
                      variant="ghost"
                      size="sm"
                      disabled={!k.is_active}
                      onClick={() => setConfirm({ action: 'rotate', key: k })}
                    >
                      <RefreshCw size={14} />
                      Ротация
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      disabled={!k.is_active}
                      onClick={() => setConfirm({ action: 'deactivate', key: k })}
                    >
                      <ShieldOff size={14} />
                      Деактивировать
                    </Button>
                  </div>
                ),
              },
            ]}
            rows={items}
            rowKey={(k) => k.key_id ?? ''}
            emptyTitle="Ключей нет"
            emptyHint="Создайте первый интеграционный ключ для проекта"
          />
        )}
      </Card>

      {/* Одноразовый секрет */}
      <Modal
        open={secret !== null}
        onClose={() => setSecret(null)}
        title="Секрет интеграционного ключа"
      >
        <div className={MODAL_FORM_CLASSES}>
          <div className="flex items-start gap-2 rounded-md border border-warning/25 bg-warning/8 px-3 py-2.5 text-[13px] text-warning [&>svg]:mt-px [&>svg]:shrink-0">
            <AlertTriangle size={16} />
            <span>Секрет показывается один раз — скопируйте его сейчас.</span>
          </div>
          {secret?.key_id && (
            <Field label="Идентификатор ключа">
              <div className="tabular-nums">{secret.key_id}</div>
            </Field>
          )}
          {secret?.secret && (
            <Field label="Секрет">
              <div className="flex flex-wrap items-center gap-3">
                <code className="min-w-[160px] flex-1 break-all rounded-md border border-border bg-surface-0 px-2.5 py-2 text-[13px]">
                  {secret.secret}
                </code>
                <CopyButton value={secret.secret} label="Копировать" />
              </div>
            </Field>
          )}
          <div className={MODAL_ACTIONS_CLASSES}>
            <Button onClick={() => setSecret(null)}>Понятно</Button>
          </div>
        </div>
      </Modal>

      {/* Подтверждение ротации/деактивации */}
      <Modal
        open={confirm !== null}
        onClose={() => setConfirm(null)}
        title={confirm?.action === 'rotate' ? 'Ротировать ключ?' : 'Деактивировать ключ?'}
      >
        <div className={MODAL_FORM_CLASSES}>
          <p className="text-[13.5px] text-muted">
            {confirm?.action === 'rotate'
              ? `Секрет ключа ${confirm?.key.key_id ?? ''} будет заменён, старый перестанет работать.`
              : `Ключ ${confirm?.key.key_id ?? ''} перестанет принимать запросы.`}
          </p>
          <div className={MODAL_ACTIONS_CLASSES}>
            <Button variant="secondary" onClick={() => setConfirm(null)}>
              Отмена
            </Button>
            {confirm?.action === 'rotate' ? (
              <Button onClick={() => void runRotate()} loading={rotateMutation.isPending}>
                <RefreshCw size={16} />
                Ротировать
              </Button>
            ) : (
              <Button variant="danger" onClick={() => void runDeactivate()} loading={deactivateMutation.isPending}>
                <ShieldOff size={16} />
                Деактивировать
              </Button>
            )}
          </div>
        </div>
      </Modal>
    </div>
  )
}