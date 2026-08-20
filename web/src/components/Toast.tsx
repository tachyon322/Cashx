import { createContext, useCallback, useContext, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { CheckCircle2, Info, XCircle } from 'lucide-react'
import { cx } from '../lib/cx'

type ToastVariant = 'success' | 'danger' | 'info'

interface ToastItem {
  id: number
  variant: ToastVariant
  message: string
}

export interface ToastApi {
  toast: (message: string, variant?: ToastVariant) => void
  success: (message: string) => void
  error: (message: string) => void
  info: (message: string) => void
}

const ToastContext = createContext<ToastApi | null>(null)

const AUTO_CLOSE_MS = 4000
const MAX_STACK = 5

const VARIANT_CLASSES: Record<ToastVariant, string> = {
  success: 'border-success/35 [&>svg]:shrink-0 [&>svg]:text-success',
  danger: 'border-danger/35 [&>svg]:shrink-0 [&>svg]:text-danger',
  info: '[&>svg]:shrink-0 [&>svg]:text-blue',
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([])
  const nextId = useRef(0)

  const dismiss = useCallback((id: number) => {
    setItems((prev) => prev.filter((item) => item.id !== id))
  }, [])

  const toast = useCallback(
    (message: string, variant: ToastVariant = 'info') => {
      const id = ++nextId.current
      setItems((prev) => [...prev.slice(-(MAX_STACK - 1)), { id, variant, message }])
      window.setTimeout(() => dismiss(id), AUTO_CLOSE_MS)
    },
    [dismiss],
  )

  const api = useMemo<ToastApi>(
    () => ({
      toast,
      success: (message: string) => toast(message, 'success'),
      error: (message: string) => toast(message, 'danger'),
      info: (message: string) => toast(message, 'info'),
    }),
    [toast],
  )

  return (
    <ToastContext.Provider value={api}>
      {children}
      {createPortal(
        <div className="fixed bottom-4 right-4 z-[200] flex max-w-[360px] flex-col gap-2" role="status" aria-live="polite">
          {items.map((item) => (
            <button
              key={item.id}
              type="button"
              className={cx(
                'flex cursor-pointer items-start gap-3 rounded-lg border border-border bg-surface-2 px-3.5 py-3 text-left text-[13.5px] leading-[1.45] text-text shadow-card animate-toast-in',
                VARIANT_CLASSES[item.variant],
              )}
              onClick={() => dismiss(item.id)}
              title="Закрыть"
            >
              {item.variant === 'success' && <CheckCircle2 size={16} />}
              {item.variant === 'danger' && <XCircle size={16} />}
              {item.variant === 'info' && <Info size={16} />}
              <span>{item.message}</span>
            </button>
          ))}
        </div>,
        document.body,
      )}
    </ToastContext.Provider>
  )
}

export function useToast(): ToastApi {
  const ctx = useContext(ToastContext)
  if (!ctx) {
    throw new Error('useToast должен использоваться внутри ToastProvider')
  }
  return ctx
}