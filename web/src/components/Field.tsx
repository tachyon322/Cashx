import { Children, cloneElement, isValidElement } from 'react'
import type { ReactNode } from 'react'
import { cx } from '../lib/cx'

interface FieldProps {
  label?: ReactNode
  hint?: ReactNode
  error?: string | null
  htmlFor?: string
  className?: string
  children: ReactNode
}

/** Обёртка контрола: label + сам контрол + ошибка/подсказка. */
export function Field({ label, hint, error, htmlFor, className = '', children }: FieldProps) {
  return (
    <div className={cx('flex flex-col gap-2', className)}>
      {label != null && (
        <label className="text-[12.5px] font-medium text-muted" htmlFor={htmlFor}>
          {label}
        </label>
      )}
      {Children.map(children, (child) =>
        isValidElement<{ 'aria-invalid'?: boolean }>(child)
          ? cloneElement(child, { 'aria-invalid': error != null ? true : undefined })
          : child,
      )}
      {error ? (
        <div className="text-[12px] text-danger" role="alert">
          {error}
        </div>
      ) : hint != null ? (
        <div className="text-[12px] text-faint">{hint}</div>
      ) : null}
    </div>
  )
}