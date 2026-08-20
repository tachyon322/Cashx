import { useState } from 'react'
import { Check, Copy } from 'lucide-react'
import { cx } from '../lib/cx'

const FEEDBACK_MS = 1500

/** Копирование в буфер: navigator.clipboard → fallback execCommand. */
export function CopyButton({ value, label = 'Копировать' }: { value: string; label?: string }) {
  const [copied, setCopied] = useState(false)

  const copy = async () => {
    let ok = false
    try {
      await navigator.clipboard.writeText(value)
      ok = true
    } catch {
      ok = fallbackCopy(value)
    }
    if (ok) {
      setCopied(true)
      window.setTimeout(() => setCopied(false), FEEDBACK_MS)
    }
  }

  return (
    <button
      type="button"
      className={cx(
        'inline-flex h-[30px] cursor-pointer items-center gap-2 rounded-md border border-border bg-surface-1 px-3 text-[12.5px] font-medium text-muted transition-[background-color,border-color,color] duration-150 hover:bg-surface-hover hover:text-text',
        copied && 'border-success/40 bg-success/8 text-success',
      )}
      onClick={() => void copy()}
      title={label}
      aria-label={label}
    >
      {copied ? <Check size={14} /> : <Copy size={14} />}
      <span>{copied ? 'Скопировано' : label}</span>
    </button>
  )
}

/** Скрытый textarea + document.execCommand('copy') для старых браузеров/небезопасного контекста. */
function fallbackCopy(value: string): boolean {
  try {
    const textarea = document.createElement('textarea')
    textarea.value = value
    textarea.style.position = 'fixed'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)
    textarea.select()
    const ok = document.execCommand('copy')
    document.body.removeChild(textarea)
    return ok
  } catch {
    return false
  }
}