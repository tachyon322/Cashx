import type { ReactNode } from 'react'
import { cx } from '../lib/cx'

interface StatCardProps {
  label: string
  value: ReactNode
  icon?: ReactNode
  /** display — Unbounded 32px для hero-сумм */
  display?: boolean
  hint?: ReactNode
}

export function StatCard({ label, value, icon, display = false, hint }: StatCardProps) {
  return (
    <div className="flex flex-col gap-2 rounded-lg border border-border bg-surface-1 p-4 shadow-card transition-colors duration-150 hover:border-border-active">
      <div className="flex items-center justify-between gap-2">
        <span className="text-[10.5px] font-semibold uppercase leading-[1.4] tracking-[0.08em] text-muted">
          {label}
        </span>
        {icon != null && <span className="inline-flex text-lilac">{icon}</span>}
      </div>
      <div
        className={cx(
          'text-[24px] font-bold leading-[1.2] tabular-nums',
          display && 'font-display text-[32px] font-semibold',
        )}
      >
        {value}
      </div>
      {hint != null && <div className="text-[12px] text-faint">{hint}</div>}
    </div>
  )
}