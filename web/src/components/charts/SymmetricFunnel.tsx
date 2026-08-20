import { EmptyState } from '../EmptyState'

export interface SymmetricFunnelItem {
  label: string
  value: number
  color: string
  formatValue?: (value: number) => string
  hint?: string
}

interface SymmetricFunnelProps {
  items: readonly SymmetricFunnelItem[]
  emptyLabel?: string
}

const ICONS: Record<string, string> = {
  Переходы: '◉',
  Регистрации: '◎',
  Депозиты: '⬣',
  Доход: '₿',
}

export function SymmetricFunnel({ items, emptyLabel = 'Нет данных за период' }: SymmetricFunnelProps) {
  const hasData = items.length > 0 && items.some((i) => i.value > 0)
  if (!hasData) return <div className="flex flex-1 flex-col justify-center"><EmptyState title={emptyLabel} /></div>

  // Log scale to compress money vs counts
  const log10 = (v: number) => Math.log10(1 + v)
  const max = Math.max(1, ...items.map((i) => log10(i.value)))
  // widths 38%..100%
  const widthPct = (v: number) => 38 + (log10(v) / max) * 62

  // Dor own colors for neon per level (override incoming but keep hint)
  const levelStyles = [
    { border: 'rgba(168,85,247,0.45)', bg: 'linear-gradient(135deg, rgba(168,85,247,0.22), rgba(96,40,200,0.10))', iconBg: 'rgba(168,85,247,0.18)', iconColor: '#d8b4fe' },
    { border: 'rgba(86,183,255,0.45)', bg: 'linear-gradient(135deg, rgba(56,140,255,0.18), rgba(30,80,160,0.10))', iconBg: 'rgba(86,183,255,0.18)', iconColor: '#7ec8ff' },
    { border: 'rgba(255,197,87,0.45)', bg: 'linear-gradient(135deg, rgba(255,180,40,0.18), rgba(140,90,10,0.10))', iconBg: 'rgba(255,197,87,0.18)', iconColor: '#ffd37a' },
    { border: 'rgba(47,227,154,0.50)', bg: 'linear-gradient(135deg, rgba(47,227,154,0.20), rgba(16,120,80,0.12))', iconBg: 'rgba(47,227,154,0.20)', iconColor: '#2fe39a' },
  ]

  return (
    <div className="flex flex-col gap-[10px] py-1">
      {items.map((item, idx) => {
        const pct = widthPct(item.value)
        const st = levelStyles[idx] ?? levelStyles[0]
        const isIncome = idx === items.length - 1
        return (
          <div key={item.label} className="flex justify-center">
            <div
              className="relative flex h-[56px] items-center justify-between gap-3 rounded-xl border px-3.5 backdrop-blur transition-[width] duration-300"
              style={{
                width: `${pct}%`,
                borderColor: st.border,
                background: st.bg,
                boxShadow: `0 0 18px ${st.border.replace('0.45', '0.22')}, 0 0 36px ${st.border.replace('0.45', '0.10')}`,
              }}
            >
              {/* left */}
              <div className="flex min-w-0 items-center gap-3">
                <span
                  className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-full border text-[13px]"
                  style={{ background: st.iconBg, borderColor: st.border, color: st.iconColor }}
                >
                  {ICONS[item.label] ?? '•'}
                </span>
                <div className="flex min-w-0 flex-col leading-tight">
                  <span className="truncate text-[12px] font-semibold text-text">{item.label}</span>
                  <span className="text-[11px] font-bold tabular-nums text-text/90">
                    {item.formatValue ? item.formatValue(item.value) : item.value.toLocaleString('ru-RU')}
                  </span>
                </div>
              </div>
              {/* right hint */}
              {item.hint ? (
                <span className="shrink-0 rounded-full bg-black/20 px-2 py-1 text-[11px] font-bold tabular-nums text-white/80">
                  ↓ {item.hint.split(' ')[0]}
                </span>
              ) : isIncome ? (
                <span className="hidden text-[10px] font-bold uppercase tracking-widest text-white/60 md:block">Доход</span>
              ) : null}

              {/* inner highlight */}
              <div className="pointer-events-none absolute inset-x-3 top-0 h-px bg-gradient-to-r from-transparent via-white/15 to-transparent" />
            </div>
          </div>
        )
      })}
    </div>
  )
}
