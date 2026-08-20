import type { SelectHTMLAttributes } from 'react'
import { cx } from '../lib/cx'

export function Select({ className = '', ...rest }: SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select
      className={cx(
        'h-10 w-full cursor-pointer rounded-md border border-border px-3 pr-[34px] text-[14px] text-text transition-[border-color,box-shadow] duration-150 focus:border-border-active focus:shadow-[0_0_0_3px_rgba(121,40,255,0.18)] focus:outline-none disabled:cursor-not-allowed disabled:opacity-55 aria-invalid:border-danger select-arrow',
        className,
      )}
      {...rest}
    />
  )
}