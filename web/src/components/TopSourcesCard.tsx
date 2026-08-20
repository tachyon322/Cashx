import { Link } from 'react-router-dom'
import { Card } from './Card'
import { EmptyState } from './EmptyState'
import { formatRubles } from '../lib/format'

export interface TopSourceItem {
  rank: number
  name: string
  sub: string
  incomeKopecks: number
  color?: string
}

const RANK_STYLES: Record<number, string> = {
  1: 'border-amber-400/40 bg-amber-400/15 text-amber-300 shadow-[0_0_12px_rgba(251,191,36,0.25)]',
  2: 'border-violet-400/40 bg-violet-500/15 text-violet-300 shadow-[0_0_12px_rgba(168,85,247,0.25)]',
  3: 'border-orange-400/30 bg-orange-400/10 text-orange-300',
}

interface TopSourcesCardProps {
  items?: TopSourceItem[]
  isLoading?: boolean
  onViewAll?: () => void
}

export function TopSourcesCard({ items, isLoading = false, onViewAll }: TopSourcesCardProps) {
  return (
    <Card
      neon
      title={<span className="text-[12px] font-bold uppercase tracking-[0.08em]">Топ источников</span>}
      actions={
        onViewAll ? (
          <button
            type="button"
            onClick={onViewAll}
            className="rounded-md border border-[rgba(168,85,247,0.28)] bg-transparent px-3 py-1 text-[11px] font-semibold text-violet-bright hover:bg-violet/10"
          >
            Все источники
          </button>
        ) : (
          <Link
            to="/cabinet/offers"
            className="rounded-md border border-[rgba(168,85,247,0.28)] bg-transparent px-3 py-1 text-[11px] font-semibold text-violet-bright hover:bg-violet/10"
          >
            Все источники
          </Link>
        )
      }
      className="flex flex-col p-0 overflow-hidden"
    >
      {isLoading ? (
        <div className="flex flex-col divide-y divide-[rgba(168,85,247,0.12)]">
          {Array.from({ length: 5 }, (_, i) => (
            <div key={i} className="flex items-center gap-3 px-4 py-3">
              <span className="h-7 w-7 shrink-0 animate-pulse rounded-full bg-white/10" />
              <div className="min-w-0 flex-1 space-y-1.5">
                <div className="h-3 w-24 animate-pulse rounded bg-white/10" />
                <div className="h-2.5 w-32 animate-pulse rounded bg-white/5" />
              </div>
              <span className="h-3 w-16 animate-pulse rounded bg-white/10" />
            </div>
          ))}
        </div>
      ) : !items || items.length === 0 ? (
        <div className="p-4">
          <EmptyState title="Нет источников" hint="Создайте ссылку в офферах — статистика появится здесь" />
        </div>
      ) : (
        <div className="flex flex-col divide-y divide-[rgba(168,85,247,0.12)]">
          {items.map((it) => (
            <div key={it.rank} className="flex items-center gap-3 px-4 py-3 hover:bg-white/[0.02] transition-colors">
              <span
                className={`inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-full border text-[12px] font-bold ${
                  RANK_STYLES[it.rank] ?? 'border-white/10 bg-white/[0.06] text-muted'
                }`}
              >
                {it.rank}
              </span>
              <div className="min-w-0 flex-1">
                <p className="truncate text-[13px] font-bold leading-tight">{it.name}</p>
                <p className="truncate text-[11px] text-faint">{it.sub}</p>
              </div>
              <span className="shrink-0 text-[13px] font-bold tabular-nums">{formatRubles(it.incomeKopecks)}</span>
            </div>
          ))}
        </div>
      )}
    </Card>
  )
}
