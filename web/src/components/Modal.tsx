import { useEffect } from 'react'
import type { ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { X } from 'lucide-react'
import { cx } from '../lib/cx'

interface ModalProps {
  open: boolean
  onClose: () => void
  title?: ReactNode
  /** 560px для табличных модалок */
  wide?: boolean
  children: ReactNode
}

export function Modal({ open, onClose, title, wide = false, children }: ModalProps) {
  useEffect(() => {
    if (!open) return
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [open, onClose])

  if (!open) return null

  return createPortal(
    <div
      className="fixed inset-0 z-[100] flex items-center justify-center p-4"
      role="dialog"
      aria-modal="true"
      aria-label={typeof title === 'string' ? title : undefined}
    >
      <div
        className="absolute inset-0 bg-[rgba(5,5,13,0.7)] backdrop-blur-xs"
        onClick={onClose}
      />
      <div
        className={cx(
          'relative max-h-[calc(100vh-48px)] w-full max-w-[480px] overflow-y-auto rounded-lg border border-border bg-surface-2 p-4 shadow-card',
          wide && 'max-w-[560px]',
        )}
      >
        <div className="mb-4 flex items-center justify-between gap-3">
          <h3 className="text-[16px] font-semibold">{title}</h3>
          <button
            type="button"
            className="inline-flex h-[30px] w-[30px] cursor-pointer items-center justify-center rounded-md border-none bg-transparent text-muted transition-colors duration-150 hover:bg-surface-hover hover:text-text"
            onClick={onClose}
            aria-label="Закрыть"
          >
            <X size={18} />
          </button>
        </div>
        <div className="flex flex-col gap-4">{children}</div>
      </div>
    </div>,
    document.body,
  )
}