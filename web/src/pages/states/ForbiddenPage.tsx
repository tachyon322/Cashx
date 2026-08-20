import { ShieldAlert } from 'lucide-react'
import { useNavigate } from 'react-router-dom'

/** 403: у роли staff нет прав на раздел. */
export function ForbiddenPage() {
  const navigate = useNavigate()
  return (
    <div className="flex min-h-[calc(100vh-64px)] flex-col items-center justify-center bg-bg bg-hero-glow p-4">
      <div className="flex w-full max-w-[460px] flex-col items-center gap-4 rounded-lg border border-border bg-surface-1 p-4 text-center shadow-card">
        <div className="flex h-14 w-14 items-center justify-center rounded-[16px] bg-violet/14 text-lilac">
          <ShieldAlert size={28} />
        </div>
        <h1 className="font-display text-[20px] font-semibold">Нет доступа</h1>
        <p className="text-[14px] text-muted">
          У вашей роли недостаточно прав для просмотра этого раздела.
        </p>
        <div className="mt-1.5 flex gap-3">
          <button
            type="button"
            className="inline-flex cursor-pointer items-center justify-center rounded-md border border-border bg-transparent px-5 py-[9px] text-[13.5px] font-medium text-text transition-colors duration-150 hover:bg-surface-hover"
            onClick={() => void navigate(-1)}
          >
            Назад
          </button>
        </div>
      </div>
    </div>
  )
}