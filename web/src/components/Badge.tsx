import type { ReactNode } from 'react'
import type { Tone } from '../lib/status'
import { cx } from '../lib/cx'

const TONE_CLASSES: Record<Tone, string> = {
  success: 'bg-success/12 text-success',
  violet: 'bg-violet/14 text-violet-bright',
  warning: 'bg-warning/12 text-warning',
  blue: 'bg-blue/12 text-blue',
  danger: 'bg-danger/12 text-danger',
  muted: 'bg-muted/12 text-muted',
}

export function Badge({
  tone = 'muted',
  children,
  className = '',
}: {
  tone?: Tone
  children: ReactNode
  className?: string
}) {
  return (
    <span
      className={cx(
        'inline-flex items-center gap-1 whitespace-nowrap rounded-full px-2 py-0.5 text-[10.5px] font-semibold leading-[1.6] tracking-[0.02em]',
        TONE_CLASSES[tone],
        className,
      )}
    >
      {children}
    </span>
  )
}