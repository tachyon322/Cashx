import { Link } from 'react-router-dom'
import { useOffers } from '../../api/queries'
import { EmptyState } from '../../components/EmptyState'
import { Skeleton } from '../../components/Skeleton'

/** Хаб проектов — группировка офферов по project_id, готов к N проектам. */
export function OffersHub() {
  const offersQ = useOffers()

  if (offersQ.isLoading) {
    return (
      <div className="flex flex-col gap-4">
        <Skeleton style={{ height: 80 }} />
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 3 }, (_, i) => (
            <Skeleton key={i} style={{ height: 180 }} />
          ))}
        </div>
      </div>
    )
  }

  if (offersQ.isError) return <EmptyState title="Не удалось загрузить проекты" />
  const items = offersQ.data?.items ?? []
  if (items.length === 0) {
    return <EmptyState title="Проекты пока не добавлены" hint="Администратор ещё не создал проекты и офферы" />
  }

  const byProject = new Map<string, typeof items>()
  for (const o of items) {
    const pid = o.project_id ?? 'unknown'
    const arr = byProject.get(pid) ?? []
    arr.push(o)
    byProject.set(pid, arr)
  }

  return (
    <div className="flex flex-col gap-4">
      <div>
        <h2 className="font-display text-[22px] font-bold tracking-[-0.01em]">Проекты</h2>
        <p className="mt-1 text-[13px] text-muted">Выберите проект — каждый имеет свои офферы и ставку RevShare</p>
      </div>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {[...byProject.entries()].map(([projectId, offers]) => {
          const first = offers[0]
          return (
            <Link
              key={projectId}
              to="/cabinet/offers"
              className="group relative flex flex-col overflow-hidden rounded-xl border border-[rgba(168,85,247,0.22)] bg-[#0d0c1c] p-4 card-neon hover:border-[rgba(168,85,247,0.38)]"
            >
              <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_at_center,rgba(168,85,247,0.12),transparent_70%)]" />
              <div className="relative">
                <p className="text-[11px] font-semibold uppercase tracking-[0.06em] text-faint">{offers.length} офферов</p>
                <h3 className="mt-1 line-clamp-1 text-[16px] font-bold">{first.project_name ?? 'Проект'}</h3>
                <p className="mt-1 line-clamp-2 text-[12px] text-muted">{offers.map((o) => o.name).join(', ')}</p>
                <span className="mt-3 inline-flex text-[12px] font-semibold text-violet-bright group-hover:underline">К офферам →</span>
              </div>
            </Link>
          )
        })}
      </div>
    </div>
  )
}
