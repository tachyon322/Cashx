import { useState } from 'react'
import { Pencil, Plus, Trash2 } from 'lucide-react'
import type { components } from '../api/schema'
import { useDeleteGroup, useSourceGroups } from '../api/queries'
import { Button } from './Button'
import { EmptyState } from './EmptyState'
import { GroupModal } from './GroupModal'
import { Modal } from './Modal'
import { useToast } from './Toast'
import { sourceErrorMessage } from '../lib/sourceErrors'

type SourceGroup = components['schemas']['SourceGroup']

const ICON_BUTTON =
  'inline-flex h-[30px] w-[30px] cursor-pointer items-center justify-center rounded-md border-none bg-transparent text-muted transition-colors duration-150 hover:bg-surface-hover hover:text-text'

interface GroupsModalProps {
  open: boolean
  onClose: () => void
}

export function GroupsModal({ open, onClose }: GroupsModalProps) {
  const toast = useToast()
  const groupsQuery = useSourceGroups()
  const remove = useDeleteGroup()

  const [creating, setCreating] = useState(false)
  const [editing, setEditing] = useState<SourceGroup | null>(null)

  const groups = groupsQuery.data?.items ?? []

  const handleDelete = (group: SourceGroup) => {
    if (!group.id) return
    remove.mutate(group.id, {
      onSuccess: () => toast.success('Поток удалён'),
      onError: (error) => toast.error(sourceErrorMessage(error, 'Не удалось удалить поток')),
    })
  }

  return (
    <>
      <Modal open={open} onClose={onClose} title="Потоки" wide>
        <div className="flex flex-col gap-3">
          <Button variant="secondary" onClick={() => setCreating(true)}>
            <Plus size={14} />
            Создать поток
          </Button>

          {groups.length === 0 ? (
            <EmptyState
              title="Потоков пока нет"
              hint="Потоки помогают группировать источники — например, «Telegram» или «YouTube»"
            />
          ) : (
            <div className="flex flex-col gap-2">
              {groups.map((group) => (
                <div
                  key={group.id ?? ''}
                  className="flex items-center justify-between gap-3 rounded-md border border-border bg-surface-1 p-3"
                >
                  <div className="min-w-0">
                    <div className="truncate text-[13.5px] font-semibold">{group.name ?? ''}</div>
                    {group.comment && (
                      <div className="truncate text-[12px] text-muted">{group.comment}</div>
                    )}
                  </div>
                  <div className="flex shrink-0 gap-1">
                    <button
                      type="button"
                      className={ICON_BUTTON}
                      onClick={() => setEditing(group)}
                      aria-label="Редактировать"
                    >
                      <Pencil size={15} />
                    </button>
                    <button
                      type="button"
                      className={`${ICON_BUTTON} hover:bg-danger/10 hover:text-danger`}
                      onClick={() => handleDelete(group)}
                      aria-label="Удалить"
                    >
                      <Trash2 size={15} />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </Modal>

      <GroupModal open={creating} initial={null} onClose={() => setCreating(false)} />
      <GroupModal open={editing != null} initial={editing} onClose={() => setEditing(null)} />
    </>
  )
}
