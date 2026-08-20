import { useState } from 'react'
import { FolderCog, Pencil, Plus, Trash2 } from 'lucide-react'
import type { components } from '../api/schema'
import { useDeleteSource, useSourceGroups, useSources } from '../api/queries'
import { Badge } from './Badge'
import { Button } from './Button'
import { Card } from './Card'
import { CopyButton } from './CopyButton'
import { GroupsModal } from './GroupsModal'
import { Skeleton } from './Skeleton'
import { SourceModal } from './SourceModal'
import { Table } from './Table'
import type { TableColumn } from './Table'
import { useToast } from './Toast'
import { formatNumber, formatRubles } from '../lib/format'
import { sourceErrorMessage } from '../lib/sourceErrors'

type Source = components['schemas']['Source']

const ICON_BUTTON =
  'inline-flex h-[30px] w-[30px] cursor-pointer items-center justify-center rounded-md border-none bg-transparent text-muted transition-colors duration-150 hover:bg-surface-hover hover:text-text'

export function SourcesCard({ offerId }: { offerId: string }) {
  const toast = useToast()
  const sourcesQuery = useSources(offerId)
  const groupsQuery = useSourceGroups()
  const removeSource = useDeleteSource(offerId)

  const [sourceModalOpen, setSourceModalOpen] = useState(false)
  const [editingSource, setEditingSource] = useState<Source | null>(null)
  const [groupsOpen, setGroupsOpen] = useState(false)

  const sources = sourcesQuery.data?.items ?? []
  const groups = groupsQuery.data?.items ?? []

  const openCreate = () => {
    setEditingSource(null)
    setSourceModalOpen(true)
  }

  const openEdit = (source: Source) => {
    setEditingSource(source)
    setSourceModalOpen(true)
  }

  const handleDelete = (source: Source) => {
    if (!source.id) return
    removeSource.mutate(source.id, {
      onSuccess: () => toast.success('Источник удалён'),
      onError: (error) => toast.error(sourceErrorMessage(error, 'Не удалось удалить источник')),
    })
  }

  const columns: readonly TableColumn<Source>[] = [
    {
      key: 'name',
      header: 'Источник',
      render: (row) => (
        <div className="flex min-w-0 flex-col gap-1">
          <div className="flex items-center gap-2">
            <span className="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-[#229ED9] text-[10px] font-bold text-white">
              {(row.name ?? '?').slice(0, 2).toUpperCase()}
            </span>
            <span className="truncate font-semibold text-text">{row.name ?? '—'}</span>
            {row.is_default && <Badge tone="violet">Основной</Badge>}
          </div>
          {row.url && (
            <div className="flex items-center gap-2 pl-8">
              <span className="min-w-0 truncate font-mono text-[11px] text-violet-bright" title={row.url}>
                {row.url}
              </span>
              <CopyButton value={row.url} />
            </div>
          )}
          {row.code && <span className="pl-8 text-[11px] text-faint">Промокод: {row.code}</span>}
        </div>
      ),
    },
    {
      key: 'clicks',
      header: 'Переходы',
      align: 'right',
      render: (row) => <span className="font-semibold tabular-nums">{formatNumber(row.totals?.clicks ?? 0)}</span>,
    },
    {
      key: 'registrations',
      header: 'Регистрации',
      align: 'right',
      render: (row) => <span className="tabular-nums">{formatNumber(row.totals?.registrations ?? 0)}</span>,
    },
    {
      key: 'deposits',
      header: 'Депозиты',
      align: 'right',
      render: (row) => <span className="tabular-nums">{formatNumber((row.totals as any)?.first_payments ?? 0)}</span>,
    },
    {
      key: 'income',
      header: 'Доход',
      align: 'right',
      render: (row) => <span className="font-bold tabular-nums">{formatRubles(row.totals?.income_kopecks ?? 0)}</span>,
    },
    {
      key: 'cr',
      header: 'CR',
      align: 'right',
      render: (row) => {
        const clicks = row.totals?.clicks ?? 0
        const regs = row.totals?.registrations ?? 0
        if (clicks <= 0) return <span className="text-faint">—</span>
        const cr = (regs / clicks) * 100
        return <span className="tabular-nums text-muted">{cr.toLocaleString('ru-RU', { maximumFractionDigits: 1 })}%</span>
      },
    },
    {
      key: 'actions',
      header: '',
      align: 'right',
      render: (row) => (
        <div className="flex justify-end gap-1">
          <button type="button" className={ICON_BUTTON} onClick={() => openEdit(row)} aria-label="Редактировать">
            <Pencil size={15} />
          </button>
          <button
            type="button"
            className={`${ICON_BUTTON} hover:bg-danger/10 hover:text-danger`}
            onClick={() => handleDelete(row)}
            aria-label="Удалить"
          >
            <Trash2 size={15} />
          </button>
        </div>
      ),
    },
  ]

  return (
    <Card
      neon
      title={<span className="text-[12px] font-bold uppercase tracking-[0.08em]">Источники трафика</span>}
      actions={
        <>
          <Button
            variant="secondary"
            size="sm"
            className="rounded-md border-[rgba(168,85,247,0.22)]"
            onClick={() => setGroupsOpen(true)}
          >
            <FolderCog size={14} />
            Потоки
          </Button>
          <Button size="sm" className=" " onClick={openCreate}>
            <Plus size={14} />
            Создать ссылку
          </Button>
        </>
      }
    >
      {sourcesQuery.isLoading ? (
        <Skeleton style={{ height: 160 }} />
      ) : (
        <Table
          columns={columns}
          rows={sources}
          rowKey={(row) => row.id ?? ''}
          emptyTitle="Источников пока нет"
          emptyHint="Создайте отдельные ссылки под каждый канал — Telegram, YouTube, рассылки"
        />
      )}

      <SourceModal
        open={sourceModalOpen}
        offerId={offerId}
        initial={editingSource}
        groups={groups}
        onClose={() => setSourceModalOpen(false)}
      />
      <GroupsModal open={groupsOpen} onClose={() => setGroupsOpen(false)} />
    </Card>
  )
}
