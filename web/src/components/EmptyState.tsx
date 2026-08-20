import type { ReactNode } from 'react'
import { Inbox } from 'lucide-react'

interface EmptyStateProps {
  icon?: ReactNode
  title?: ReactNode
  hint?: ReactNode
  children?: ReactNode
}

export function EmptyState({ icon, title = 'Нет данных', hint, children }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center gap-2 p-4 text-center">
      <div className="mb-1 flex h-12 w-12 items-center justify-center rounded-[14px] bg-surface-2 text-faint">
        {icon ?? <Inbox size={22} />}
      </div>
      <div className="text-[14px] font-semibold">{title}</div>
      {hint != null && <div className="max-w-[340px] text-[12.5px] text-faint">{hint}</div>}
      {children}
    </div>
  )
}