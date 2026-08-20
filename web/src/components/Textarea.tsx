import type { TextareaHTMLAttributes } from 'react'
import { cx } from '../lib/cx'

export function Textarea({ className = '', ...rest }: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return (
    <textarea
      className={cx(
        'min-h-24 w-full resize-y rounded-md border border-border bg-surface-1 px-3 py-2.5 text-[14px] leading-[1.5] text-text transition-[border-color,box-shadow] duration-150 placeholder:text-faint focus:border-border-active focus:shadow-[0_0_0_3px_rgba(121,40,255,0.18)] focus:outline-none disabled:cursor-not-allowed disabled:opacity-55 aria-invalid:border-danger',
        className,
      )}
      {...rest}
    />
  )
}