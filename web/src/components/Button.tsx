import type { ButtonHTMLAttributes } from 'react'
import { cx } from '../lib/cx'

type Variant = 'primary' | 'secondary' | 'ghost' | 'danger'
type Size = 'sm' | 'md' | 'lg'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  size?: Size
  loading?: boolean
}

const VARIANT_CLASSES: Record<Variant, string> = {
  primary: 'bg-violet text-white hover:bg-violet-bright btn-volume-primary btn-side-gradient',
  secondary: 'border-border bg-transparent text-text hover:bg-surface-hover btn-volume-secondary btn-side-gradient',
  ghost: 'bg-transparent text-muted hover:bg-surface-hover hover:text-text btn-volume-ghost btn-side-gradient',
  danger: 'border-danger/35 bg-transparent text-danger hover:bg-danger/10 btn-volume-danger btn-side-gradient-danger',
}

const SIZE_CLASSES: Record<Size, string> = {
  sm: 'h-[30px] px-3 text-[12.5px]',
  md: 'h-[38px] px-[18px] text-[13.5px]',
  lg: 'h-[46px] px-[26px] text-[15px]',
}

const SPINNER_CLASSES: Record<Variant, string> = {
  primary: 'border-white/35 border-t-white',
  secondary: 'border-muted/35 border-t-muted',
  ghost: 'border-muted/35 border-t-muted',
  danger: 'border-muted/35 border-t-muted',
}

export function Button({
  variant = 'primary',
  size = 'md',
  loading = false,
  disabled,
  className = '',
  children,
  ...rest
}: ButtonProps) {
  return (
    <button
      type="button"
      className={cx(
        'relative isolate inline-flex select-none cursor-pointer items-center justify-center gap-2 whitespace-nowrap rounded-md border border-transparent font-semibold overflow-hidden transition-[background-color,border-color,color,filter,opacity,box-shadow,transform] duration-150 active:btn-volume-pressed active:translate-y-px disabled:cursor-not-allowed disabled:opacity-50 disabled:shadow-none',
        VARIANT_CLASSES[variant],
        SIZE_CLASSES[size],
        className,
      )}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      {...rest}
    >
      {loading && (
        <span
          className={cx(
            'relative z-[1] h-[14px] w-[14px] shrink-0 rounded-full border-2 animate-[cx-spin_0.8s_linear_infinite]',
            SPINNER_CLASSES[variant],
          )}
          aria-hidden
        />
      )}
      <span className="relative z-[1] inline-flex items-center justify-center gap-2">{children}</span>
    </button>
  )
}