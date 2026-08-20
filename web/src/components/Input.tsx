import type { InputHTMLAttributes } from 'react'
import { cx } from '../lib/cx'

export function Input({ className = '', ...rest }: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      className={cx(
        'h-10 w-full rounded-md border border-border bg-surface-1 px-3 text-[14px] text-text transition-[border-color,box-shadow] duration-150 placeholder:text-faint focus:border-border-active focus:shadow-[0_0_0_3px_rgba(121,40,255,0.18)] focus:outline-none disabled:cursor-not-allowed disabled:opacity-55 aria-invalid:border-danger',
        className,
      )}
      {...rest}
    />
  )
}