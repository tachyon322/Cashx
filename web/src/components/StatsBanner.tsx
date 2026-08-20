import { ArrowDownRight, ArrowUpRight, Minus } from 'lucide-react'
import type { ReactNode } from 'react'
import { formatNumber, formatRubles } from '../lib/format'
import type { PeriodStats } from '../lib/stats'
import { cx } from '../lib/cx'

export function StatsBanner({
  stats,
  icon,
  className,
}: {
  stats: PeriodStats
  icon?: ReactNode
  className?: string
}) {
  return (
    <article
      className={cx(
        'relative flex flex-col gap-3 overflow-hidden rounded-xl border border-[rgba(168,85,247,0.32)] bg-[linear-gradient(135deg,#0d0c1c_0%,#15122a_100%)] p-4 shadow-card card-neon',
        className,
      )}
    >
      <div
        className="pointer-events-none absolute -right-14 -top-14 h-40 w-40 rounded-full bg-violet/15 blur-[80px]"
        aria-hidden
      />
      <div className="pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-violet-bright/25 to-transparent" aria-hidden />

      <div className="relative z-[1] flex flex-col gap-3">
        <header className="flex items-start justify-between gap-3">
          <div>
            <h3 className="text-[10.5px] font-semibold uppercase tracking-[0.08em] text-muted">
              {stats.title}
            </h3>
            <p className="mt-0.5 hidden text-[11px] text-faint md:block">{stats.subtitle}</p>
          </div>
          {icon && (
            <span className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md border border-[rgba(168,85,247,0.35)] bg-violet/10 text-violet-bright shadow-[0_0_12px_rgba(121,40,255,0.22)]">
              {icon}
            </span>
          )}
        </header>

        <div className="flex items-start justify-between gap-2">
          <span className="font-display text-[22px] font-bold leading-none tracking-[-0.015em] text-text tabular-nums">
            {formatRubles(stats.amountKopecks)}
          </span>
          <GrowthBadge percent={stats.growthPercent} />
        </div>
        {/* inline growth hint on mobile subtitle if needed */}
        <p className="text-[11px] text-faint md:hidden">{stats.subtitle}</p>

        <dl className="mt-1 grid grid-cols-3 gap-2 border-t border-[rgba(168,85,247,0.18)] pt-3">
          <FunnelStat value={stats.clicks} label="Переходы" />
          <FunnelStat value={stats.registrations} label="Регистрации" />
          <FunnelStat value={stats.deposits} label="Депозиты" />
        </dl>
      </div>
    </article>
  )
}

function FunnelStat({ value, label }: { value: number; label: string }) {
  return (
    <div>
      <dt className="sr-only">{label}</dt>
      <dd className="font-display text-[16px] font-bold leading-none text-text tabular-nums">{formatNumber(value)}</dd>
      <p className="mt-1 text-[10.5px] leading-tight text-faint">{label}</p>
    </div>
  )
}

function GrowthBadge({ percent }: { percent: number | null }) {
  if (percent == null) {
    return (
      <span className="inline-flex items-center gap-1 rounded-full bg-surface-hover px-2 py-1 text-[11px] font-medium text-faint">
        <Minus size={11} />
        —
      </span>
    )
  }
  const up = percent > 0
  const down = percent < 0
  return (
    <span
      className={cx(
        'inline-flex items-center gap-1 rounded-full px-2 py-1 text-[11px] font-bold tabular-nums',
        up && 'bg-success/12 text-success',
        down && 'bg-danger/12 text-danger',
        !up && !down && 'bg-surface-hover text-muted',
      )}
    >
      {up ? (
        <>
          <ArrowUpRight size={11} />+{percent}%
        </>
      ) : down ? (
        <>
          <ArrowDownRight size={11} />
          {percent}%
        </>
      ) : (
        <>
          <Minus size={11} />
          0%
        </>
      )}
    </span>
  )
}
