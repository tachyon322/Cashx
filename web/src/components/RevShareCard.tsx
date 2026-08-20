import { ArrowRight, HelpCircle } from 'lucide-react'
import { formatBps } from '../lib/format'

interface RevShareCardProps {
  revsharePercentBps?: number
  onDetails: () => void
}

export function RevShareCard({ revsharePercentBps, onDetails }: RevShareCardProps) {
  return (
    <aside className="relative flex flex-col justify-between gap-3 overflow-hidden rounded-xl border border-[rgba(168,85,247,0.35)] bg-[#0a0a1a] p-5 shadow-card card-neon @container">
      <div className="pointer-events-none absolute -right-16 -top-16 h-48 w-48 rounded-full bg-violet/20 blur-[90px]" aria-hidden />
      <div className="pointer-events-none absolute -bottom-12 -left-12 h-32 w-32 rounded-full bg-magenta/10 blur-[70px]" aria-hidden />
      <div className="pointer-events-none absolute inset-0 hero-grid opacity-[0.18]" aria-hidden />
      <div className="pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-violet-bright/30 to-transparent" aria-hidden />

      <div className="relative z-[1] flex flex-1 flex-col gap-2">
        <div className="flex items-center justify-between gap-2">
          <span className="text-[10.5px] font-semibold uppercase tracking-[0.08em] text-muted">
            Ваша комиссия
          </span>
          <span className="inline-flex h-5 w-5 items-center justify-center rounded-full border border-border text-faint">
            <HelpCircle size={11} />
          </span>
        </div>

        <div className="flex flex-1 items-center">
          {revsharePercentBps != null ? (
            <span className="font-display text-[18cqw] font-black leading-none tracking-[-0.03em] text-[#d8b4fe] neon-text tabular-nums">
              {formatBps(revsharePercentBps)}
            </span>
          ) : (
            <span className="h-[18cqw] w-[36cqw] animate-pulse rounded-md bg-surface-hover" aria-hidden />
          )}
        </div>

        <div className="flex items-center gap-2">
          <span className="text-[13px] font-semibold tracking-[0.06em] text-muted">RevShare</span>
          <span className="inline-flex h-5 w-5 items-center justify-center rounded-full border border-border text-faint">
            <HelpCircle size={10} />
          </span>
        </div>
      </div>

      <button
        type="button"
        onClick={onDetails}
        className="relative isolate z-[1] inline-flex w-full items-center justify-between overflow-hidden rounded-md border border-[rgba(168,85,247,0.32)] bg-[rgba(121,40,255,0.08)] px-4 py-2.5 text-[12px] font-semibold text-violet-bright transition-[background-color,border-color,color,box-shadow,transform] duration-150 hover:bg-[rgba(121,40,255,0.16)] hover:text-text active:btn-volume-pressed active:translate-y-px btn-volume-secondary btn-side-gradient"
      >
        <span className="relative z-[1]">Подробнее об условиях</span>
        <ArrowRight size={14} className="relative z-[1] shrink-0" />
      </button>
    </aside>
  )
}
