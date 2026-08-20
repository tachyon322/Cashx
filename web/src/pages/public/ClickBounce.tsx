import { useEffect } from 'react'
import { useParams } from 'react-router-dom'

/**
 * Bounce трекинг-ссылок (/c/:code): мгновенный редирект на redirect-сервис,
 * который записывает клик и уводит на destination (валидацию кода делает он).
 */
export function ClickBounce() {
  const { code } = useParams<{ code: string }>()

  useEffect(() => {
    const base = import.meta.env.VITE_REDIRECT_BASE ?? 'http://localhost:8081'
    window.location.replace(`${base}/c/${code}`)
  }, [code])

  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-4 bg-bg bg-hero-glow">
      <div
        className="h-7 w-7 animate-spin-slow rounded-full border-2 border-surface-2 border-t-violet-bright"
        aria-hidden
      />
      <p className="text-[14px] text-muted">Переходим...</p>
    </div>
  )
}