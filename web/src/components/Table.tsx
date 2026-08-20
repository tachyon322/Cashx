import type { ReactNode } from 'react'
import { EmptyState } from './EmptyState'
import { cx } from '../lib/cx'

export interface TableColumn<T> {
  key: string
  header: ReactNode
  render: (row: T) => ReactNode
  align?: 'left' | 'right'
}

interface TableProps<T> {
  columns: readonly TableColumn<T>[]
  rows: readonly T[]
  rowKey: (row: T) => string
  onRowClick?: (row: T) => void
  compact?: boolean
  empty?: ReactNode
  emptyTitle?: string
  emptyHint?: string
}

export function Table<T>({
  columns,
  rows,
  rowKey,
  onRowClick,
  compact = false,
  empty,
  emptyTitle = 'Нет данных',
  emptyHint,
}: TableProps<T>) {
  if (rows.length === 0) {
    return empty ?? <EmptyState title={emptyTitle} hint={emptyHint} />
  }

  const cellPadding = compact ? 'px-2.5 py-2' : 'px-3 py-3'
  const headPadding = compact ? 'px-2.5 py-2' : 'px-3 py-2.5'

  return (
    <div className="w-full overflow-x-auto">
      <table className={cx('w-full border-collapse text-[13.5px]', compact && 'text-[12.5px] leading-[1.35]')}>
        <thead>
          <tr>
            {columns.map((column) => (
              <th
                key={column.key}
                className={cx(
                  'whitespace-nowrap border-b border-border text-left text-[10.5px] font-semibold uppercase leading-[1.4] tracking-[0.08em] text-faint',
                  headPadding,
                  column.align === 'right' && 'text-right',
                )}
              >
                {column.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr
              key={rowKey(row)}
              className={cx(
                'last:[&>td]:border-b-0 hover:bg-surface-hover',
                compact ? '[&>th]:px-2.5 [&>td]:px-2.5' : '',
                onRowClick && 'cursor-pointer',
              )}
              onClick={onRowClick ? () => onRowClick(row) : undefined}
            >
              {columns.map((column) => (
                <td
                  key={column.key}
                  className={cx(
                    'border-b border-border align-middle',
                    cellPadding,
                    column.align === 'right' && 'text-right',
                  )}
                >
                  {column.render(row)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}