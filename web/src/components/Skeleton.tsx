import type { CSSProperties } from 'react'
import { cx } from '../lib/cx'

export function Skeleton({ className = '', style }: { className?: string; style?: CSSProperties }) {
  return (
    <div
      className={cx('relative overflow-hidden rounded-md bg-surface-2 skeleton-shimmer', className)}
      style={style}
    />
  )
}