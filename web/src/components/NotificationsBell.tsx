import { useEffect, useRef, useState } from 'react'
import { Bell, CheckCheck } from 'lucide-react'
import { useNotificationRead, useNotifications, useNotificationsReadAll } from '../api/queries'
import { formatDateTime } from '../lib/format'
import { EmptyState } from './EmptyState'
import { cx } from '../lib/cx'

export function NotificationsBell() {
  const { data } = useNotifications()
  const readOne = useNotificationRead()
  const readAll = useNotificationsReadAll()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const onDocClick = (event: MouseEvent) => {
      if (ref.current && !ref.current.contains(event.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDocClick)
    return () => document.removeEventListener('mousedown', onDocClick)
  }, [open])

  const unread = data?.unread_count ?? 0
  const items = data?.items ?? []

  return (
    <div className="relative" ref={ref}>
      <button
        type="button"
        className="relative inline-flex h-9 w-9 cursor-pointer items-center justify-center rounded-md border border-border bg-surface-1 text-muted transition-[background-color,color] duration-150 hover:bg-surface-hover hover:text-text"
        onClick={() => setOpen((value) => !value)}
        aria-label="Уведомления"
        aria-expanded={open}
      >
        <Bell size={18} />
        {unread > 0 && (
          <span className="absolute -top-[5px] -right-[5px] flex h-[18px] min-w-[18px] items-center justify-center rounded-full bg-danger px-1 text-[10px] font-bold leading-none text-white shadow-[0_0_8px_rgba(255,100,124,0.5)]">
            {unread > 99 ? '99+' : unread}
          </span>
        )}
      </button>

      {open && (
        <div className="absolute right-0 top-[calc(100%+10px)] z-50 w-[360px] max-w-[calc(100vw-32px)] overflow-hidden rounded-lg border border-border bg-surface-2 shadow-card">
          <div className="flex items-center justify-between gap-3 border-b border-border p-4">
            <span className="text-[13.5px] font-semibold">Уведомления</span>
            {unread > 0 && (
              <button
                type="button"
                className="inline-flex cursor-pointer items-center gap-2 border-none bg-none p-1 text-[12px] font-medium text-lilac transition-colors duration-150 hover:text-violet-bright"
                onClick={() => void readAll.mutate()}
              >
                <CheckCheck size={14} />
                Прочитать все
              </button>
            )}
          </div>
          <div className="max-h-[480px] overflow-y-auto">
            {items.length === 0 ? (
              <EmptyState title="Нет уведомлений" hint="Здесь появятся анонсы и личные сообщения" />
            ) : (
              items.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  className={cx(
                    'flex w-full cursor-pointer flex-col gap-1 border-b border-border bg-transparent p-4 text-left text-text transition-colors duration-150 last:border-b-0 hover:bg-surface-hover',
                    !item.read && 'bg-surface-hover',
                  )}
                  onClick={() => {
                    if (!item.read) {
                      void readOne.mutate({ type: item.type ?? 'announcement', id: item.id ?? '' })
                    }
                  }}
                >
                  <span className="flex items-center gap-2">
                    {!item.read && <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-violet-bright shadow-[0_0_6px_rgba(168,85,247,0.8)]" aria-hidden />}
                    <span className="text-[13px] font-semibold">{item.title}</span>
                  </span>
                  <span className="line-clamp-2 text-[12.5px] text-muted">{item.body}</span>
                  <span className="text-[11.5px] text-faint">
                    {item.created_at ? formatDateTime(item.created_at) : ''}
                  </span>
                </button>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  )
}