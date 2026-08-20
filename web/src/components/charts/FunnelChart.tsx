import { EmptyState } from '../EmptyState'

export interface FunnelItem {
  label: string
  value: number
  color?: string
}

interface FunnelChartProps {
  items: readonly FunnelItem[]
  formatValue?: (value: number) => string
}

export function FunnelChart({ items, formatValue }: FunnelChartProps) {
  if (items.length === 0) {
    return <EmptyState title="Нет данных за период" />
  }

  const max = Math.max(1, ...items.map((item) => item.value))

  return (
    <div className="flex flex-col gap-4">
      {items.map((item, index) => (
        <div key={index} className="grid grid-cols-[130px_1fr_auto] items-center gap-3">
          <span className="truncate text-[12px] text-muted">{item.label}</span>
          <div className="h-2.5 overflow-hidden rounded-full bg-surface-2">
            <div
              className="h-full rounded-full transition-[width] duration-300"
              style={{
                width: `${(item.value / max) * 100}%`,
                background: item.color ?? 'var(--cx-violet-bright)',
              }}
            />
          </div>
          <span className="whitespace-nowrap text-[13.5px] font-bold tabular-nums">
            {formatValue ? formatValue(item.value) : item.value}
          </span>
        </div>
      ))}
    </div>
  )
}