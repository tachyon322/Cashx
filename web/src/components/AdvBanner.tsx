import type { ReactNode } from 'react'
import { cx } from '../lib/cx'

interface AdvBannerProps {
  title: ReactNode
  description?: ReactNode
  actions?: ReactNode
  media?: ReactNode
  className?: string
}

export function AdvBanner({ title, description, actions, media, className }: AdvBannerProps) {
  return (
    <section
      className={cx(
        'relative overflow-hidden rounded-xl border border-[rgba(168,85,247,0.35)] bg-[#0a0a1a] card-neon-strong',
        className,
      )}
    >
      <div className="pointer-events-none absolute inset-0 hero-grid opacity-[0.28]" aria-hidden />
      <div
        className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_at_50%_120%,rgba(121,40,255,0.32),transparent_62%)]"
        aria-hidden
      />
      <div className="pointer-events-none absolute inset-0 bg-hero-glow opacity-60" aria-hidden />
      <div
        className="pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-violet-bright/50 to-transparent"
        aria-hidden
      />
      <div className="pointer-events-none absolute inset-x-0 bottom-0 h-[58%] opacity-[0.22]" aria-hidden>
        <svg viewBox="0 0 1200 240" preserveAspectRatio="none" className="h-full w-full">
          <path
            d="M0 180 L40 180 L40 120 L60 120 L60 150 L80 150 L80 100 L100 100 L100 140 L120 140 L120 90 L140 90 L140 160 L160 160 L160 110 L180 110 L180 170 L200 170 L200 80 L220 80 L220 150 L260 150 L260 120 L280 120 L280 170 L320 170 L320 60 L340 60 L340 140 L360 140 L360 100 L380 100 L380 160 L400 160 L400 130 L420 130 L420 180 L480 180 L480 100 L500 100 L500 140 L520 140 L520 80 L540 80 L540 150 L560 150 L560 120 L580 120 L580 170 L620 170 L620 90 L640 90 L640 160 L680 160 L680 70 L700 70 L700 150 L720 150 L720 110 L740 110 L740 170 L780 170 L780 130 L800 130 L800 180 L860 180 L860 100 L880 100 L880 140 L900 140 L900 80 L920 80 L920 160 L960 160 L960 120 L980 120 L980 180 L1020 180 L1020 100 L1040 100 L1040 150 L1060 150 L1060 90 L1080 90 L1080 170 L1100 170 L1100 130 L1120 130 L1120 180 L1200 180 L1200 240 L0 240 Z"
            fill="rgba(168,85,247,0.18)"
          />
          <path
            d="M0 180 L40 180 L40 120 L60 120 L60 150 L80 150 L80 100 L100 100 L100 140 L120 140 L120 90 L140 90 L140 160 L160 160 L160 110 L180 110 L180 170 L200 170 L200 80 L220 80 L220 150 L260 150 L260 120 L280 120 L280 170 L320 170 L320 60 L340 60 L340 140 L360 140 L360 100 L380 100 L380 160 L400 160 L400 130 L420 130 L420 180 L480 180 L480 100 L500 100 L500 140 L520 140 L520 80 L540 80 L540 150 L560 150 L560 120 L580 120 L580 170 L620 170 L620 90 L640 90 L640 160 L680 160 L680 70 L700 70 L700 150 L720 150 L720 110 L740 110 L740 170 L780 170 L780 130 L800 130 L800 180 L860 180 L860 100 L880 100 L880 140 L900 140 L900 80 L920 80 L920 160 L960 160 L960 120 L980 120 L980 180 L1020 180 L1020 100 L1040 100 L1040 150 L1060 150 L1060 90 L1080 90 L1080 170 L1100 170 L1100 130 L1120 130 L1120 180 L1200 180"
            stroke="rgba(168,85,247,0.35)"
            strokeWidth="1"
            fill="none"
          />
        </svg>
      </div>
      <div
        className="pointer-events-none absolute inset-x-0 bottom-0 h-[42%] opacity-[0.22]"
        style={{
          background:
            'repeating-linear-gradient(90deg, transparent 0 36px, rgba(168,85,247,0.18) 36px 37px), repeating-linear-gradient(0deg, transparent 0 24px, rgba(168,85,247,0.12) 24px 25px)',
          maskImage: 'linear-gradient(to top, black 40%, transparent 100%)',
          transform: 'perspective(500px) rotateX(62deg)',
          transformOrigin: 'bottom',
        }}
        aria-hidden
      />

      <div className="relative z-[1] grid items-center gap-6 p-5 md:grid-cols-[1.05fr_0.95fr] md:p-6 lg:p-7">
        <div className="flex min-w-0 flex-col items-start gap-3">
          <h2 className="font-display text-[22px] font-bold leading-[1.12] tracking-[-0.015em] text-text md:text-[26px]">
            {title}
          </h2>
          {description != null && (
            <p className="max-w-[48ch] text-[13.5px] leading-[1.6] text-muted">{description}</p>
          )}
          {actions != null && <div className="mt-1 flex flex-wrap items-center gap-3">{actions}</div>}
        </div>
        {media != null ? (
          <div className="flex justify-center md:justify-end">{media}</div>
        ) : (
          <div className="hidden md:block" />
        )}
      </div>
    </section>
  )
}

export function BannerArt() {
  return <HeroNeonArt />
}

export function HeroNeonArt() {
  return (
    <div className="relative h-[220px] w-full max-w-[420px] select-none md:h-[240px]">
      <div
        className="absolute left-1/2 top-[52%] h-[140px] w-[260px] -translate-x-1/2 rounded-full bg-[radial-gradient(ellipse_at_center,rgba(168,85,247,0.55),transparent_68%)] blur-[18px]"
        aria-hidden
      />
      <div className="absolute bottom-[18%] left-1/2 h-[86px] w-[300px] -translate-x-1/2" aria-hidden>
        <div
          className="absolute inset-x-[8%] top-0 h-[46%] rounded-[14px] border border-[rgba(168,85,247,0.55)] bg-[linear-gradient(180deg,rgba(30,16,60,0.95),rgba(10,10,26,0.98))] shadow-[0_0_22px_rgba(121,40,255,0.35)]"
          style={{ transform: 'perspective(320px) rotateX(18deg)' }}
        >
          <div className="absolute inset-[10%] rounded-[10px] border border-white/5 opacity-60" />
          <div className="absolute inset-x-0 top-1/2 h-px bg-gradient-to-r from-transparent via-violet-bright/40 to-transparent" />
        </div>
        <div
          className="absolute inset-x-[10%] bottom-0 top-[28%] rounded-b-[16px] bg-[linear-gradient(180deg,rgba(36,20,72,0.9),rgba(16,12,36,1))] opacity-90"
          style={{ transform: 'perspective(320px) rotateX(18deg) translateZ(-6px)' }}
        />
        <div
          className="absolute inset-x-[10%] top-[6%] h-[36%] opacity-[0.22]"
          style={{
            backgroundImage:
              'linear-gradient(rgba(168,85,247,0.5) 1px, transparent 1px), linear-gradient(90deg, rgba(168,85,247,0.5) 1px, transparent 1px)',
            backgroundSize: '18px 18px',
            transform: 'perspective(320px) rotateX(18deg)',
          }}
        />
      </div>

      <div className="absolute bottom-[34%] left-[38%] flex flex-col items-center" aria-hidden>
        {Array.from({ length: 5 }).map((_, i) => (
          <div
            key={i}
            className="relative h-[14px] w-[64px] -mt-[3px]"
            style={{ zIndex: 5 - i, transform: `translateY(${i * -0.5}px)` }}
          >
            <div className="absolute inset-x-0 top-0 h-[9px] rounded-[50%] border border-[rgba(200,170,255,0.5)] bg-[radial-gradient(ellipse_at_30%_30%,#e9d5ff,#a855f7_45%,#581c87)] shadow-[0_0_10px_rgba(168,85,247,0.6)]" />
            <div className="absolute inset-x-[2px] top-[5px] h-[7px] rounded-b-[50%] bg-[linear-gradient(180deg,#4a1a8a,#1a0f2e)] border-x border-b border-[rgba(168,85,247,0.4)]" />
            <div className="absolute left-[18%] right-[18%] top-[2px] h-[1px] rounded-full bg-white/35 blur-[0.5px]" />
          </div>
        ))}
        <div className="absolute -top-1 left-1/2 h-[18px] w-[42px] -translate-x-1/2 rounded-[50%] bg-[radial-gradient(ellipse_at_center,rgba(255,255,255,0.9),transparent_60%)] opacity-70 blur-[2px]" />
      </div>

      <div
        className="absolute bottom-[30%] right-[8%] h-[118px] w-[168px] rotate-[-8deg]"
        style={{ transform: 'perspective(600px) rotateY(-14deg) rotateX(10deg) rotateZ(-6deg)' }}
        aria-hidden
      >
        <div className="absolute bottom-0 inset-x-0 h-[14px] rounded-[4px] bg-[linear-gradient(180deg,#2a1b4a,#0f0f1f)] border border-[rgba(168,85,247,0.45)] shadow-[0_8px_22px_rgba(0,0,0,0.5)]" />
        <div className="absolute inset-x-[6%] bottom-[12%] top-[6%] rounded-[10px] border border-[rgba(168,85,247,0.6)] bg-[linear-gradient(135deg,#0f0b1e_0%,#1a1040_55%,#2a1a6a_100%)] p-[2px] shadow-[0_0_28px_rgba(121,40,255,0.45)]">
          <div className="flex h-full w-full flex-col items-center justify-center rounded-[8px] bg-[radial-gradient(ellipse_at_center,rgba(168,85,247,0.22),transparent_70%)] p-2">
            <span className="font-display text-[38px] font-black leading-none tracking-[-0.03em] text-[#d8b4fe] neon-text">40%</span>
            <span className="mt-0.5 text-[9px] font-bold uppercase tracking-[0.22em] text-violet-bright/90">
              RevShare
            </span>
            <div className="pointer-events-none absolute inset-[6px] rounded-[7px] border border-white/10" />
          </div>
          <div className="pointer-events-none absolute -inset-3 -z-10 rounded-[14px] bg-[radial-gradient(ellipse_at_center,rgba(168,85,247,0.35),transparent_65%)] blur-[12px]" />
        </div>
        <div className="absolute bottom-[12%] left-1/2 h-[4px] w-[60%] -translate-x-1/2 rounded-full bg-[#1a1440] border border-white/10" />
      </div>
    </div>
  )
}
