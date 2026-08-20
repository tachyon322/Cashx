import { ChevronLeft, ChevronRight } from 'lucide-react'

export const PAGE_SIZE = 50

interface PaginationProps {
  total: number
  offset: number
  limit?: number
  onChange: (offset: number) => void
}

/** Offset-пагинация: «Назад/Вперёд» + «стр. N из M». */
export function Pagination({ total, offset, limit = PAGE_SIZE, onChange }: PaginationProps) {
  const page = Math.floor(offset / limit) + 1
  const pages = Math.max(1, Math.ceil(total / limit))
  const canPrev = offset > 0
  const canNext = offset + limit < total

  return (
    <div className="flex items-center justify-center gap-4 py-1.5">
      <button
        type="button"
        className="inline-flex h-[32px] cursor-pointer items-center gap-2 rounded-md border border-border bg-transparent px-3.5 text-[13px] font-medium text-text transition-colors duration-150 enabled:hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-45"
        disabled={!canPrev}
        onClick={() => onChange(Math.max(0, offset - limit))}
      >
        <ChevronLeft size={16} />
        Назад
      </button>
      <span className="whitespace-nowrap text-[13px] text-faint">
        стр. {page} из {pages}
      </span>
      <button
        type="button"
        className="inline-flex h-[32px] cursor-pointer items-center gap-2 rounded-md border border-border bg-transparent px-3.5 text-[13px] font-medium text-text transition-colors duration-150 enabled:hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-45"
        disabled={!canNext}
        onClick={() => onChange(offset + limit)}
      >
        Вперёд
        <ChevronRight size={16} />
      </button>
    </div>
  )
}