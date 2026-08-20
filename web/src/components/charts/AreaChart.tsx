import { useId, useLayoutEffect, useMemo, useRef, useState } from 'react'
import type { MouseEvent } from 'react'
import { EmptyState } from '../EmptyState'

export interface AreaPoint {
  date: string
  value: number
}

interface AreaChartProps {
  data: readonly AreaPoint[]
  color?: string
  height?: number
  formatValue?: (value: number) => string
  formatDate?: (date: string) => string
  emptyLabel?: string
}

const DEFAULT_COLOR = '#A855F7'
const PAD_TOP = 12
const PAD_RIGHT = 8
const PAD_BOTTOM = 24
const PAD_LEFT = 8
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

/** dd.MM — подписи оси X. */
function shortDate(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return ''
  const day = String(date.getDate()).padStart(2, '0')
  const month = String(date.getMonth() + 1).padStart(2, '0')
  return `${day}.${month}`
}

export function AreaChart({
  data,
  color = DEFAULT_COLOR,
  height = 220,
  formatValue,
  formatDate,
  emptyLabel = 'Нет данных за период',
}: AreaChartProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [width, setWidth] = useState(0)
  const [hoverIndex, setHoverIndex] = useState<number | null>(null)
  const gradientId = useId().replace(/[^a-zA-Z0-9]/g, '')

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

  const hasData = data.length > 0 && data.some((point) => point.value !== 0)

  const geometry = useMemo(() => {
    if (!hasData || width <= 0) return null
    const yMax = niceCeil(Math.max(...data.map((point) => point.value)))
    const innerWidth = Math.max(1, width - PAD_LEFT - PAD_RIGHT)
    const innerHeight = height - PAD_TOP - PAD_BOTTOM
    const n = data.length
    const x = (index: number) =>
      n === 1 ? PAD_LEFT + innerWidth / 2 : PAD_LEFT + (index / (n - 1)) * innerWidth
    const y = (value: number) => PAD_TOP + (1 - value / yMax) * innerHeight
    const points = data.map((point, index) => ({ x: x(index), y: y(point.value) }))
    const linePath = points.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x.toFixed(2)} ${p.y.toFixed(2)}`).join(' ')
    const areaPath = `${linePath} L ${points[points.length - 1].x.toFixed(2)} ${(PAD_TOP + innerHeight).toFixed(2)} L ${points[0].x.toFixed(2)} ${(PAD_TOP + innerHeight).toFixed(2)} Z`
    return { yMax, innerHeight, points, linePath, areaPath }
  }, [data, hasData, height, width])

  const onMove = (event: MouseEvent<SVGSVGElement>) => {
    if (!geometry) return
    const rect = event.currentTarget.getBoundingClientRect()
    const relX = event.clientX - rect.left
    let best = 0
    let bestDist = Number.POSITIVE_INFINITY
    geometry.points.forEach((point, index) => {
      const dist = Math.abs(point.x - relX)
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
  // пропускаем пустые даты (данные могут быть разреженными)
  const labelIndexes = Object.keys(wanted)
    .map(Number)
    .sort((a, b) => a - b)
    .filter((index) => data[index].date)

  const hovered = hoverIndex != null ? geometry.points[hoverIndex] : null
  const hoverPoint = hoverIndex != null ? data[hoverIndex] : null
  const tooltipLeft = hovered ? Math.min(Math.max(hovered.x, 70), Math.max(70, width - 70)) : 0

  return (
    <div className="relative w-full" ref={containerRef} style={{ height }}>
      <svg
        className="block overflow-visible"
        width={width}
        height={height}
        onMouseMove={onMove}
        onMouseLeave={() => setHoverIndex(null)}
      >
        <defs>
          <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor={color} stopOpacity="0.32" />
            <stop offset="100%" stopColor={color} stopOpacity="0" />
          </linearGradient>
        </defs>

        {/* сетка */}
        {Array.from({ length: GRID_LINES }, (_, i) => {
          const y = PAD_TOP + (geometry.innerHeight / (GRID_LINES - 1)) * i
          return (
            <line
              key={i}
              x1={PAD_LEFT}
              x2={width - PAD_RIGHT}
              y1={y}
              y2={y}
              stroke="rgba(202,174,255,.08)"
              strokeWidth="1"
            />
          )
        })}

        {/* подписи оси X */}
        {labelIndexes.map((index) => (
          <text
            key={index}
            x={geometry.points[index].x}
            y={height - 6}
            textAnchor="middle"
            className="fill-faint text-[10.5px]"
          >
            {formatDate ? formatDate(data[index].date) : shortDate(data[index].date)}
          </text>
        ))}

        {/* область + линия */}
        <path d={geometry.areaPath} fill={`url(#${gradientId})`} />
        <path
          d={geometry.linePath}
          fill="none"
          stroke={color}
          strokeWidth="2"
          strokeLinejoin="round"
          strokeLinecap="round"
          style={{ filter: `drop-shadow(0 0 6px ${color})` }}
        />

        {/* точки при наведении */}
        {hovered != null && (
          <>
            <line
              x1={hovered.x}
              x2={hovered.x}
              y1={PAD_TOP}
              y2={PAD_TOP + geometry.innerHeight}
              stroke="rgba(202,174,255,.35)"
              strokeWidth="1"
            />
            <circle cx={hovered.x} cy={hovered.y} r="4" fill={color} stroke="#05050D" strokeWidth="2" />
          </>
        )}
      </svg>

      {hovered != null && hoverPoint != null && (
        <div
          className="pointer-events-none absolute top-0 z-[5] flex min-w-[120px] -translate-x-1/2 flex-col gap-1 rounded-md border border-border bg-surface-2 px-3 py-2 shadow-card"
          style={{ left: tooltipLeft }}
        >
          <div className="text-[11px] text-faint">
            {formatDate ? formatDate(hoverPoint.date) : shortDate(hoverPoint.date)}
          </div>
          <div className="text-[13.5px] font-bold tabular-nums">
            {formatValue ? formatValue(hoverPoint.value) : hoverPoint.value}
          </div>
        </div>
      )}
    </div>
  )
}
