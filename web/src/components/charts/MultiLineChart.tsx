import { useLayoutEffect, useMemo, useRef, useState } from 'react'
import type { MouseEvent } from 'react'
import { EmptyState } from '../EmptyState'

export interface MultiLinePoint {
  /** Короткая подпись для оси X. */
  label: string
  /** Полная подпись для тултипа. */
  tooltipLabel: string
  /** Значения серий — в том же порядке, что и series. */
  values: readonly number[]
}

export interface MultiLineSeries {
  label: string
  color: string
  /**
   * Ось серии: 'left' — своя шкала слева (например, деньги),
   * 'right' — своя шкала справа (например, количество).
   * По умолчанию 'left'. Разные оси позволяют не прижимать
   * мелкие значения к полу из-за крупных на общей шкале.
   */
  axis?: 'left' | 'right'
}

interface MultiLineChartProps {
  data: readonly MultiLinePoint[]
  series: readonly MultiLineSeries[]
  height?: number
  formatValue?: (value: number, seriesIndex: number) => string
  emptyLabel?: string
}

const PAD_TOP = 12
const PAD_BOTTOM = 24
const PAD_LEFT = 52 // подписи левой оси Y (деньги)
const PAD_RIGHT_SINGLE = 8
const PAD_RIGHT_DUAL = 48 // подписи правой оси Y (количество)
const GRID_LINES = 4

/** Округление максимума вверх до «красивого» шага (1/2/2.5/5 × 10ⁿ). */
function niceCeil(value: number): number {
  if (value <= 0) return 0
  const exp = Math.floor(Math.log10(value))
  const base = Math.pow(10, exp)
  const frac = value / base
  let nice: number
  if (frac <= 1) nice = 1
  else if (frac <= 2) nice = 2
  else if (frac <= 2.5) nice = 2.5
  else if (frac <= 5) nice = 5
  else nice = 10
  return nice * base
}

/** Компактные рубли для подписей оси Y: «50 тыс ₽», «1,2 млн ₽». */
function compactRubles(kopecks: number): string {
  const rubles = kopecks / 100
  if (rubles >= 1_000_000) {
    return `${(rubles / 1_000_000).toLocaleString('ru-RU', { maximumFractionDigits: 1 })} млн ₽`
  }
  if (rubles >= 1_000) return `${Math.round(rubles / 1_000)} тыс ₽`
  return `${Math.round(rubles)} ₽`
}

/** Компактные штуки для правой оси: «1,2 тыс», «3,4 млн». */
function compactCount(value: number): string {
  if (value >= 1_000_000) {
    return `${(value / 1_000_000).toLocaleString('ru-RU', { maximumFractionDigits: 1 })} млн`
  }
  if (value >= 1_000) {
    return `${(value / 1_000).toLocaleString('ru-RU', { maximumFractionDigits: 1 })} тыс`
  }
  return `${Math.round(value)}`
}

export function MultiLineChart({
  data,
  series,
  height = 260,
  formatValue,
  emptyLabel = 'Нет данных за период',
}: MultiLineChartProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [width, setWidth] = useState(0)
  const [hoverIndex, setHoverIndex] = useState<number | null>(null)

  useLayoutEffect(() => {
    const el = containerRef.current
    if (!el) return
    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        setWidth(entry.contentRect.width)
      }
    })
    observer.observe(el)
    return () => observer.disconnect()
  }, [])

  const hasData = data.length > 0 && data.some((point) => point.values.some((value) => value !== 0))

  const hasRightAxis = series.some((s) => s.axis === 'right')
  const padRight = hasRightAxis ? PAD_RIGHT_DUAL : PAD_RIGHT_SINGLE

  const geometry = useMemo(() => {
    if (!hasData || width <= 0) return null
    // У каждой оси свой максимум, чтобы «Депозиты»/«Регистрация» (штуки)
    // не прижимались к полу из-за доходов (деньги) на той же шкале.
    const axisOf = (index: number) => series[index]?.axis ?? 'left'
    const leftValues = data.flatMap((point) => point.values.filter((_, i) => axisOf(i) === 'left'))
    const rightValues = data.flatMap((point) => point.values.filter((_, i) => axisOf(i) === 'right'))
    const yMaxLeft = niceCeil(Math.max(0, ...leftValues))
    const yMaxRight = niceCeil(Math.max(0, ...rightValues))
    const innerWidth = Math.max(1, width - PAD_LEFT - padRight)
    const innerHeight = height - PAD_TOP - PAD_BOTTOM
    const n = data.length
    const x = (index: number) =>
      n === 1 ? PAD_LEFT + innerWidth / 2 : PAD_LEFT + (index / (n - 1)) * innerWidth
    const yFor = (value: number, yMax: number): number =>
      yMax <= 0 ? PAD_TOP + innerHeight : PAD_TOP + (1 - value / yMax) * innerHeight
    const y = (value: number, seriesIndex: number): number =>
      yFor(value, axisOf(seriesIndex) === 'right' ? yMaxRight : yMaxLeft)
    return { yMaxLeft, yMaxRight, innerWidth, innerHeight, x, y }
  }, [data, hasData, height, width, series, padRight])

  const linePaths = useMemo(() => {
    if (!geometry) return []
    return series.map((s, seriesIndex) => {
      const d = data
        .map((point, index) => {
          const px = geometry.x(index)
          const py = geometry.y(point.values[seriesIndex] ?? 0, seriesIndex)
          return `${index === 0 ? 'M' : 'L'} ${px.toFixed(2)} ${py.toFixed(2)}`
        })
        .join(' ')
      return { color: s.color, d }
    })
  }, [data, geometry, series])

  const onMove = (event: MouseEvent<SVGSVGElement>) => {
    if (!geometry) return
    const rect = event.currentTarget.getBoundingClientRect()
    const relX = event.clientX - rect.left
    let best = 0
    let bestDist = Number.POSITIVE_INFINITY
    data.forEach((_, index) => {
      const dist = Math.abs(geometry.x(index) - relX)
      if (dist < bestDist) {
        bestDist = dist
        best = index
      }
    })
    setHoverIndex(best)
  }

  if (!hasData) {
    return (
      <div className="relative w-full" ref={containerRef} style={{ height }}>
        <EmptyState title={emptyLabel} />
      </div>
    )
  }

  // geometry == null только пока ширина ещё не измерена ResizeObserver'ом
  if (!geometry) {
    return <div className="relative w-full" ref={containerRef} style={{ height }} />
  }

  const labelEvery = Math.max(1, Math.ceil(data.length / 5))
  const wanted: Record<number, true> = { 0: true, [data.length - 1]: true }
  for (let i = 0; i < data.length; i += labelEvery) wanted[i] = true
  const labelIndexes = Object.keys(wanted)
    .map(Number)
    .sort((a, b) => a - b)
    .filter((index) => data[index].label)

  const hovered = hoverIndex != null ? data[hoverIndex] : null
  const hoverX = hoverIndex != null ? geometry.x(hoverIndex) : null
  const tooltipLeft = hoverX != null ? Math.min(Math.max(hoverX, 90), Math.max(90, width - 90)) : 0

  return (
    <div className="relative w-full" ref={containerRef} style={{ height: height + 28 }}>
      {/* легенда */}
      <div className="mb-2 flex flex-wrap items-center gap-x-4 gap-y-1.5">
        {series.map((s) => (
          <span key={s.label} className="flex items-center gap-1.5 text-[12px] text-muted">
            <span className="h-2 w-2 rounded-full" style={{ background: s.color }} />
            {s.label}
          </span>
        ))}
      </div>

      <svg
        className="block overflow-visible"
        width={width}
        height={height}
        onMouseMove={onMove}
        onMouseLeave={() => setHoverIndex(null)}
      >
        {/* сетка + подписи осей Y (слева деньги, справа количество) */}
        {Array.from({ length: GRID_LINES }, (_, i) => {
          const gy = PAD_TOP + (geometry.innerHeight / (GRID_LINES - 1)) * i
          const leftValue = geometry.yMaxLeft * (1 - i / (GRID_LINES - 1))
          const rightValue = geometry.yMaxRight * (1 - i / (GRID_LINES - 1))
          return (
            <g key={i}>
              <line
                x1={PAD_LEFT}
                x2={width - padRight}
                y1={gy}
                y2={gy}
                stroke="rgba(202,174,255,.08)"
                strokeWidth="1"
              />
              <text x={PAD_LEFT - 6} y={gy + 3.5} textAnchor="end" className="fill-faint text-[10.5px]">
                {compactRubles(leftValue)}
              </text>
              {hasRightAxis && (
                <text
                  x={width - padRight + 6}
                  y={gy + 3.5}
                  textAnchor="start"
                  className="fill-faint text-[10.5px]"
                >
                  {compactCount(rightValue)}
                </text>
              )}
            </g>
          )
        })}

        {/* подписи оси X */}
        {labelIndexes.map((index) => (
          <text
            key={index}
            x={geometry.x(index)}
            y={height - 6}
            textAnchor="middle"
            className="fill-faint text-[10.5px]"
          >
            {data[index].label}
          </text>
        ))}

        {/* линии серий */}
        {linePaths.map((path, seriesIndex) => (
          <path
            key={seriesIndex}
            d={path.d}
            fill="none"
            stroke={path.color}
            strokeWidth="2"
            strokeLinejoin="round"
            strokeLinecap="round"
            style={{ filter: `drop-shadow(0 0 6px ${path.color})` }}
          />
        ))}

        {/* перекрестие + точки при наведении */}
        {hovered != null && hoverX != null && (
          <>
            <line
              x1={hoverX}
              x2={hoverX}
              y1={PAD_TOP}
              y2={PAD_TOP + geometry.innerHeight}
              stroke="rgba(202,174,255,.35)"
              strokeWidth="1"
            />
            {series.map((s, seriesIndex) => {
              const value = hovered.values[seriesIndex] ?? 0
              return (
                <circle
                  key={s.label}
                  cx={hoverX}
                  cy={geometry.y(value, seriesIndex)}
                  r="4"
                  fill={s.color}
                  stroke="#05050D"
                  strokeWidth="2"
                />
              )
            })}
          </>
        )}
      </svg>

      {hovered != null && (
        <div
          className="pointer-events-none absolute top-0 z-[5] flex min-w-[170px] -translate-x-1/2 flex-col gap-1 rounded-md border border-border bg-surface-2 px-3 py-2 shadow-card"
          style={{ left: tooltipLeft }}
        >
          <div className="text-[11px] text-faint">{hovered.tooltipLabel}</div>
          {series.map((s, seriesIndex) => (
            <div
              key={s.label}
              className="grid grid-cols-[auto_1fr_auto] items-center gap-1.5 text-[12.5px]"
            >
              <span className="h-1.5 w-1.5 rounded-full" style={{ background: s.color }} />
              <span className="text-muted">{s.label}</span>
              <span className="font-semibold tabular-nums">
                {formatValue ? formatValue(hovered.values[seriesIndex] ?? 0, seriesIndex) : hovered.values[seriesIndex] ?? 0}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
