import type { HTMLAttributes, ReactNode } from 'react'
import { cx } from '../lib/cx'

interface CardProps extends Omit<HTMLAttributes<HTMLDivElement>, 'title'> {
  title?: ReactNode
  subtitle?: ReactNode
  actions?: ReactNode
  clickable?: boolean
  /** неоновая рамка как на референсе CashxPay */
  neon?: boolean
  /** сильная неоновая рамка для геро-блоков */
  neonStrong?: boolean
}

export function Card({
  title,
  subtitle,
  actions,
  clickable = false,
  neon = false,
  neonStrong = false,
  className = '',
  children,
  ...rest
}: CardProps) {
  return (
    <div
      className={cx(
        'rounded-lg border border-border bg-surface-1 p-4 shadow-card',
        neon && 'card-neon',
        neonStrong && 'card-neon-strong',
        clickable &&
          'cursor-pointer transition-[border-color,transform] duration-150 hover:-translate-y-0.5 hover:border-border-active',
        className,
      )}
      {...rest}
    >
      {(title != null || actions != null) && (
        <div className="mb-4 flex items-start justify-between gap-3">
          <div>
            {title != null && <h3 className="text-[16px] font-semibold">{title}</h3>}
            {subtitle != null && <p className="mt-0.5 text-[13px] text-muted">{subtitle}</p>}
          </div>
          {actions != null && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
        </div>
      )}
      <div className="flex flex-col gap-4">{children}</div>
    </div>
  )
}