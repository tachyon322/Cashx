import { useState } from 'react'
import { useAuth } from '../../auth/AuthContext'
import { useSourceGroups, useRedirectPools } from '../../api/queries'
import { Card } from '../../components/Card'
import { EmptyState } from '../../components/EmptyState'
import { Skeleton } from '../../components/Skeleton'
import { Table } from '../../components/Table'
import type { TableColumn } from '../../components/Table'
import { GroupModal } from '../../components/GroupModal'
import { formatDate } from '../../lib/format'
import type { components } from '../../api/schema'

type SourceGroup = components['schemas']['SourceGroup']

function GroupsTab() {
  const groupsQ = useSourceGroups()
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<SourceGroup | null>(null)

  if (groupsQ.isLoading) return <Skeleton style={{ height: 200 }} />
  if (groupsQ.isError) return <EmptyState title="Не удалось загрузить потоки" />

  const items = groupsQ.data?.items ?? []
  const columns: readonly TableColumn<SourceGroup>[] = [
    { key: 'name', header: 'Поток', render: (r) => <span className="font-semibold">{r.name ?? '—'}</span> },
    { key: 'comment', header: 'Комментарий', render: (r) => <span className="text-[12px] text-muted">{r.comment ?? '—'}</span> },
    { key: 'created', header: 'Создан', render: (r) => (r.created_at ? formatDate(r.created_at) : '—') },
    {
      key: 'actions',
      header: '',
      align: 'right',
      render: (row) => (
        <button
          className="text-[12px] font-semibold text-violet-bright hover:underline"
          onClick={() => {
            setEditing(row)
            setModalOpen(true)
          }}
        >
          Изм.
        </button>
      ),
    },
  ]

  return (
    <div className="flex flex-col gap-3">
      <div className="flex justify-end">
        <button
          className="rounded-md bg-violet px-3 py-1.5 text-[12px] font-bold text-white"
          onClick={() => {
            setEditing(null)
            setModalOpen(true)
          }}
        >
          + Создать поток
        </button>
      </div>
      {items.length === 0 ? (
        <EmptyState title="Потоков пока нет" hint="Создайте поток для группировки источников" />
      ) : (
        <Table columns={columns} rows={items as SourceGroup[]} rowKey={(r) => r.id ?? ''} compact />
      )}
      <GroupModal open={modalOpen} onClose={() => setModalOpen(false)} initial={editing as any} />
    </div>
  )
}

function RedirectsTab() {
  const q = useRedirectPools()
  if (q.isLoading) return <Skeleton style={{ height: 180 }} />
  if (q.isError) return <EmptyState title="Не удалось загрузить редиректы" />
  const items = (q.data?.items ?? []) as any[]
  if (items.length === 0) return <EmptyState title="Редиректов пока нет" hint="Редиректы настраивает администратор" />
  return (
    <div className="flex flex-col gap-3">
      {items.map((pool: any) => (
        <div key={pool.id} className="rounded-lg border border-[rgba(168,85,247,0.18)] bg-surface-1 p-3">
          <div className="flex items-center justify-between">
            <span className="font-semibold">{pool.name}</span>
            <span className="text-[11px] text-faint">{pool.comment ?? ''}</span>
          </div>
          <div className="mt-2 flex flex-wrap gap-1.5">
            {(pool.urls ?? []).map((u: any) => (
              <span key={u.id} className="rounded-full border border-white/10 bg-white/[0.04] px-2 py-1 text-[11px] font-mono">
                {u.url} <span className="text-faint">×{u.weight}</span>
              </span>
            ))}
          </div>
        </div>
      ))}
      <p className="text-[11px] text-faint">Редиректы настраиваются администратором, партнёр видит список для выбора в источниках.</p>
    </div>
  )
}

export function SettingsPage() {
  const { user } = useAuth()
  const partner = user?.partner as any
  // is_owner/is_admin may come from backend partner profile; fallback to false if not present
  const canViewPartners = Boolean(partner?.is_owner || partner?.is_admin)
  const [tab, setTab] = useState<'groups' | 'redirects' | 'partners'>('groups')

  const tabs: readonly { key: typeof tab; label: string; visible: boolean }[] = [
    { key: 'groups', label: 'Потоки', visible: true },
    { key: 'redirects', label: 'Редиректы', visible: true },
    { key: 'partners', label: 'Партнёры', visible: canViewPartners },
  ]

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-2">
        <h2 className="font-display text-[18px] font-bold">Настройки партнёра</h2>
        <span className="text-[11px] text-faint">Потоки и редиректы</span>
      </div>

      <div className="inline-flex self-start rounded-lg border border-[rgba(168,85,247,0.22)] bg-surface-0 p-1">
        {tabs
          .filter((t) => t.visible)
          .map((t) => (
            <button
              key={t.key}
              onClick={() => setTab(t.key)}
              className={`rounded-md px-3 py-1.5 text-[12px] font-semibold ${tab === t.key ? 'bg-violet text-white' : 'text-muted hover:text-text'}`}
            >
              {t.label}
            </button>
          ))}
      </div>

      <Card neon>
        {tab === 'groups' && <GroupsTab />}
        {tab === 'redirects' && <RedirectsTab />}
        {tab === 'partners' && (
          <EmptyState title="Команда партнёров" hint="Управление ставками и доступами пока в админке /admin/partners (требует is_owner/is_admin)" />
        )}
      </Card>
    </div>
  )
}
