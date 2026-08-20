import { useEffect, useState } from 'react'
import { useAdminAudit } from '../../api/queries'
import type { AdminAuditParams } from '../../api/queries'
import { Badge } from '../../components/Badge'
import { Card } from '../../components/Card'
import { Field } from '../../components/Field'
import { Input } from '../../components/Input'
import { PAGE_SIZE, Pagination } from '../../components/Pagination'
import { Select } from '../../components/Select'
import { Skeleton } from '../../components/Skeleton'
import { Table } from '../../components/Table'
import { formatDateTime } from '../../lib/format'

const PAGE_CLASSES = 'flex w-full flex-col gap-4 p-4'
const TOOLBAR_CLASSES = 'flex flex-wrap items-end gap-3'
const TOOLBAR_FIELD_CLASSES = 'min-w-[240px] max-w-[420px] flex-1'
const SKELETON_CLASSES = 'flex flex-col gap-2'

export function AuditPage() {
  // Фильтры: отложенное применение (300 мс) со сбросом offset.
  const [entityType, setEntityType] = useState('')
  const [entityId, setEntityId] = useState('')
  const [applied, setApplied] = useState<{ entity_type?: string; entity_id?: string }>({})
  const [offset, setOffset] = useState(0)

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setApplied({
        entity_type: entityType || undefined,
        entity_id: entityId.trim() || undefined,
      })
      setOffset(0)
    }, 300)
    return () => window.clearTimeout(timer)
  }, [entityType, entityId])

  const params: AdminAuditParams = {
    limit: PAGE_SIZE,
    offset,
    ...(applied.entity_type ? { entity_type: applied.entity_type } : {}),
    ...(applied.entity_id ? { entity_id: applied.entity_id } : {}),
  }
  const audit = useAdminAudit(params)
  const items = audit.data?.items ?? []
  const total = audit.data?.total ?? 0

  // Варианты entity_type собираем из данных по мере загрузки страниц.
  const [knownTypes, setKnownTypes] = useState<string[]>([])
  useEffect(() => {
    if (!audit.data?.items) return
    const fresh = new Set<string>()
    for (const entry of audit.data.items) {
      if (entry.entity_type) fresh.add(entry.entity_type)
    }
    if (fresh.size > 0) {
      setKnownTypes((prev) => [...new Set([...prev, ...fresh])])
    }
  }, [audit.data])

  return (
    <div className={PAGE_CLASSES}>
      <Card title="Аудит" subtitle={`Всего записей: ${total}`}>
        <div className={TOOLBAR_CLASSES}>
          <Field label="Тип сущности" className={TOOLBAR_FIELD_CLASSES}>
            <Select
              value={entityType}
              onChange={(event) => setEntityType(event.target.value)}
            >
              <option value="">Все типы</option>
              {knownTypes.map((type) => (
                <option key={type} value={type}>
                  {type}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="ID сущности" className={TOOLBAR_FIELD_CLASSES}>
            <Input
              type="search"
              placeholder="Например, id партнёра или оффера"
              value={entityId}
              onChange={(event) => setEntityId(event.target.value)}
            />
          </Field>
        </div>

        {audit.isLoading && items.length === 0 ? (
          <div className={SKELETON_CLASSES}>
            <Skeleton style={{ height: 36 }} />
            <Skeleton style={{ height: 36 }} />
            <Skeleton style={{ height: 36 }} />
          </div>
        ) : (
          <Table
            columns={[
              { key: 'actor_email', header: 'Кто', render: (a) => a.actor_email ?? '—' },
              { key: 'action', header: 'Действие', render: (a) => a.action ?? '—' },
              {
                key: 'entity_type',
                header: 'Тип сущности',
                render: (a) =>
                  a.entity_type ? <Badge tone="violet">{a.entity_type}</Badge> : '—',
              },
              {
                key: 'entity_id',
                header: 'ID сущности',
                render: (a) => <span className="tabular-nums">{a.entity_id ?? '—'}</span>,
              },
              {
                key: 'created_at',
                header: 'Когда',
                render: (a) => (a.created_at ? formatDateTime(a.created_at) : '—'),
              },
              {
                key: 'changes',
                header: 'Изменения',
                render: (a) =>
                  a.changes ? (
                    <details>
                      <summary className="cursor-pointer select-none text-[13px] text-muted transition-colors duration-150 hover:text-text">
                        Показать JSON
                      </summary>
                      <pre className="mt-2 max-h-[240px] max-w-[640px] overflow-y-auto whitespace-pre-wrap break-words rounded-md border border-border bg-surface-0 p-2.5 px-3 text-[12.5px] leading-[1.5]">
                        {JSON.stringify(a.changes, null, 2)}
                      </pre>
                    </details>
                  ) : (
                    '—'
                  ),
              },
            ]}
            rows={items}
            rowKey={(a) => a.id ?? `${a.created_at}-${a.action}`}
            emptyTitle="Записей аудита нет"
            emptyHint="Измените фильтры"
          />
        )}

        <Pagination total={total} offset={offset} limit={PAGE_SIZE} onChange={setOffset} />
      </Card>
    </div>
  )
}