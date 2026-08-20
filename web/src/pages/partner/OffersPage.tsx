import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Coins, Gamepad2, Landmark, Rocket, Dices, Crown, Trophy, Check } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { ApiRequestError } from '../../api/client'
import { useJoinOffer, useOffers } from '../../api/queries'
import { Button } from '../../components/Button'
import { CopyButton } from '../../components/CopyButton'
import { EmptyState } from '../../components/EmptyState'
import { Modal } from '../../components/Modal'
import { Skeleton } from '../../components/Skeleton'
import { useToast } from '../../components/Toast'
import { formatBps } from '../../lib/format'
import { offerStatus } from '../../lib/status'
import { cx } from '../../lib/cx'

const STATUS_FILTERS: readonly { key: string; label: string }[] = [
  { key: 'all', label: 'Все' },
  { key: 'active', label: 'Активен' },
  { key: 'available', label: 'Доступен' },
  { key: 'pending', label: 'На модерации' },
  { key: 'coming_soon', label: 'Скоро' },
]

const CATEGORY_VISUALS: readonly {
  keywords: readonly string[]
  icon: LucideIcon
  tint: string
}[] = [
  { keywords: ['slot', 'spin', '777', 'слот'], icon: Dices, tint: 'violet' },
  { keywords: ['rocket', 'ракет', 'crash'], icon: Rocket, tint: 'violet' },
  { keywords: ['dice', 'кости', 'blockchain'], icon: Dices, tint: 'violet' },
  { keywords: ['king', 'bet', 'король', 'crown'], icon: Crown, tint: 'violet' },
  { keywords: ['goal', 'sport', 'футбол', 'win', 'goalwin'], icon: Trophy, tint: 'violet' },
  { keywords: ['crypto', 'coin', 'крипт', 'обмен', 'exchange'], icon: Coins, tint: 'violet' },
  { keywords: ['financ', 'bank', 'pay', 'финанс', 'банк', 'плат'], icon: Landmark, tint: 'violet' },
  { keywords: ['game', 'gambl', 'casino', 'игр', 'казин', 'гейм', 'litgame'], icon: Gamepad2, tint: 'violet' },
]

function offerVisual(category?: string | null, name?: string | null): { icon: LucideIcon } {
  const hay = `${category ?? ''} ${name ?? ''}`.toLowerCase()
  for (const v of CATEGORY_VISUALS) if (v.keywords.some((k) => hay.includes(k))) return { icon: v.icon }
  return { icon: Rocket }
}

function formatEpc(epc: number): string {
  return `${(epc / 100).toLocaleString('ru-RU', { maximumFractionDigits: 2 })} ₽`
}
function formatCr(cr: number): string {
  return `${cr.toLocaleString('ru-RU', { maximumFractionDigits: 2 })}%`
}

function getBadge(offer: any): { label: string; tone: 'violet' | 'success' | 'warning' | 'muted' | 'blue' } {
  const status = offer.status
  const cr = offer.cr ?? 0
  if (status === 'active') return { label: 'Активен', tone: 'violet' }
  if (status === 'available' && cr >= 2.5) return { label: 'Популярный', tone: 'warning' }
  if (status === 'available') return { label: 'Новый', tone: 'success' }
  if (status === 'pending' || status === 'coming_soon') return { label: 'Скоро', tone: 'muted' }
  return { label: offerStatus(status).label, tone: offerStatus(status).tone as any }
}

const BADGE_STYLES: Record<string, string> = {
  violet: 'border-violet/30 bg-violet/15 text-violet-bright',
  success: 'border-success/30 bg-success/12 text-success',
  warning: 'border-warning/30 bg-warning/12 text-warning',
  muted: 'border-white/10 bg-white/[0.06] text-faint',
  blue: 'border-blue/30 bg-blue/12 text-blue',
}

interface JoinSuccess {
  offerId: string
  trackingUrl: string
}

function OffersSkeleton() {
  return (
    <>
      <div className="flex flex-wrap gap-2">
        {STATUS_FILTERS.map((f) => (
          <Skeleton key={f.key} style={{ width: 96, height: 32 }} />
        ))}
      </div>
      <div className="grid items-start gap-4 grid-cols-[repeat(auto-fill,minmax(320px,1fr))]">
        {Array.from({ length: 6 }, (_, i) => (
          <Skeleton key={i} style={{ height: 340 }} />
        ))}
      </div>
    </>
  )
}

const CHIP_CLASSES =
  'h-[32px] cursor-pointer rounded-md border border-[rgba(168,85,247,0.22)] bg-transparent px-3.5 text-[13px] font-medium text-muted transition-colors duration-150 hover:bg-surface-hover hover:text-text'
const CHIP_ACTIVE = 'border-[rgba(168,85,247,0.45)] bg-violet/15 text-text shadow-[0_0_14px_rgba(121,40,255,0.18)]'

export function OffersPage() {
  const toast = useToast()
  const navigate = useNavigate()
  const offersQuery = useOffers()
  const join = useJoinOffer()
  const [filter, setFilter] = useState<string>('all')
  const [joiningId, setJoiningId] = useState<string | null>(null)
  const [joinSuccess, setJoinSuccess] = useState<JoinSuccess | null>(null)

  const items = offersQuery.data?.items ?? []
  const filtered = filter === 'all' ? items : items.filter((o) => o.status === filter)

  const handleJoin = (offerId: string) => {
    if (join.isPending) return
    setJoiningId(offerId)
    join.mutate(offerId, {
      onSuccess: (data) => setJoinSuccess({ offerId, trackingUrl: data.tracking_url ?? '' }),
      onError: (error) => {
        if (error instanceof ApiRequestError && error.status === 409) {
          toast.error('Вы уже подключили этот оффер')
          void offersQuery.refetch()
        } else toast.error(error instanceof Error ? error.message : 'Не удалось подключить оффер')
      },
      onSettled: () => setJoiningId(null),
    })
  }
  const goToStats = () => {
    if (!joinSuccess) return
    void navigator.clipboard.writeText(joinSuccess.trackingUrl).catch(() => undefined)
    navigate(`/cabinet/offers/${joinSuccess.offerId}`)
  }

  if (offersQuery.isLoading) return <OffersSkeleton />
  if (offersQuery.error)
    return <EmptyState title="Не удалось загрузить офферы" hint="Попробуйте обновить страницу через несколько секунд" />

  return (
    <>
      <div className="flex flex-col gap-4">
        <div>
          <h2 className="font-display text-[22px] font-bold tracking-[-0.01em] text-text">Выберите проект</h2>
          <p className="mt-1 text-[13px] text-muted">Выберите проект для работы — каждый имеет свою ставку RevShare и воронку</p>
        </div>

        <div className="flex flex-wrap gap-2" role="group" aria-label="Фильтр по статусу">
          {STATUS_FILTERS.map((chip) => (
            <button
              key={chip.key}
              type="button"
              className={cx(CHIP_CLASSES, filter === chip.key && CHIP_ACTIVE)}
              onClick={() => setFilter(chip.key)}
              aria-pressed={filter === chip.key}
            >
              {chip.label}
            </button>
          ))}
        </div>
      </div>

      {filtered.length === 0 ? (
        <EmptyState
          title={items.length === 0 ? 'Офферы пока не добавлены' : 'Нет офферов с таким статусом'}
          hint="Новые офферы появляются по мере подключения проектов"
        />
      ) : (
        <div className="grid items-start gap-4 grid-cols-[repeat(auto-fill,minmax(300px,1fr))] xl:grid-cols-3">
          {filtered.map((offer) => {
            const visual = offerVisual(offer.category, offer.name)
            const VisualIcon = visual.icon
            const joined = offer.my_rate_bps != null || offer.my_tracking_url != null
            const badge = getBadge(offer)
            const isComing = offer.status === 'coming_soon' || offer.status === 'pending'
            const metricsEpc = offer.epc != null ? formatEpc(offer.epc) : null
            const metricsCr = offer.cr != null ? formatCr(offer.cr) : null

            return (
              <div
                key={offer.offer_id ?? ''}
                className="group relative flex flex-col overflow-hidden rounded-xl border border-[rgba(168,85,247,0.22)] bg-[#0d0c1c] p-0 shadow-card card-neon transition-[border-color,box-shadow] duration-200 hover:border-[rgba(168,85,247,0.38)] hover:shadow-[0_0_0_1px_rgba(168,85,247,0.28),0_0_28px_rgba(121,40,255,0.16)]"
              >
                {/* badge */}
                <div className="pointer-events-none absolute left-3 top-3 z-10">
                  <span
                    className={cx(
                      'inline-flex items-center rounded-full border px-2.5 py-1 text-[10px] font-bold uppercase tracking-[0.06em]',
                      BADGE_STYLES[badge.tone] ?? BADGE_STYLES.muted,
                    )}
                  >
                    {badge.label}
                  </span>
                </div>

                {/* visual */}
                <div className="relative flex h-[148px] shrink-0 items-center justify-center overflow-hidden">
                  <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_at_center,rgba(168,85,247,0.22),transparent_68%)]" aria-hidden />
                  <div className="pointer-events-none absolute inset-0 hero-grid opacity-[0.12]" aria-hidden />
                  {offer.project_logo_url ? (
                    <img
                      className="relative z-[1] max-h-[56px] max-w-[56%] object-contain drop-shadow-[0_0_18px_rgba(168,85,247,0.45)]"
                      src={offer.project_logo_url}
                      alt={offer.project_name ?? ''}
                    />
                  ) : (
                    <div className="relative z-[1] flex flex-col items-center gap-3">
                      <div className="flex h-[88px] w-[88px] items-center justify-center rounded-2xl border border-[rgba(168,85,247,0.45)] bg-[linear-gradient(135deg,rgba(30,16,60,0.95),rgba(60,20,120,0.95))] shadow-[0_0_28px_rgba(121,40,255,0.35),inset_0_1px_0_rgba(255,255,255,0.08)]">
                        <VisualIcon size={42} strokeWidth={1.6} className="text-[#e9d5ff] drop-shadow-[0_0_12px_rgba(168,85,247,0.7)]" />
                      </div>
                    </div>
                  )}
                  {/* inner highlight */}
                  <div className="pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-violet-bright/25 to-transparent" />
                </div>

                <div className="flex flex-1 flex-col gap-3 p-4">
                  <div>
                    <h3 className="text-[16px] font-bold leading-[1.2] tracking-[-0.01em]">{offer.name}</h3>
                    {offer.description && (
                      <p className="mt-1 line-clamp-2 text-[12.5px] leading-[1.5] text-muted">{offer.description}</p>
                    )}
                    {offer.project_name && (
                      <span className="mt-1 inline-block text-[11px] font-medium text-faint">{offer.project_name}</span>
                    )}
                  </div>

                  <div className="grid grid-cols-2 gap-3 border-t border-[rgba(168,85,247,0.12)] pt-3">
                    <div>
                      <p className="text-[10px] font-semibold uppercase tracking-[0.06em] text-faint">EPC</p>
                      <p className="mt-1 font-display text-[15px] font-bold leading-none tabular-nums">
                        {metricsEpc ?? '—'}
                      </p>
                    </div>
                    <div>
                      <p className="text-[10px] font-semibold uppercase tracking-[0.06em] text-faint">CR</p>
                      <p className="mt-1 font-display text-[15px] font-bold leading-none tabular-nums">
                        {metricsCr ?? '—'}
                      </p>
                    </div>
                  </div>

                  <div className="flex items-center justify-between">
                    <span className="text-[11px] font-medium text-muted">
                      Комиссия{' '}
                      <span className="font-bold text-text">
                        до {offer.my_rate_bps != null ? formatBps(offer.my_rate_bps) : offer.project_name ? '40% RevShare' : formatBps(4000)} RevShare
                      </span>
                    </span>
                  </div>

                  <div className="mt-auto pt-1">
                    {joined ? (
                      <Link
                        to={`/cabinet/offers/${offer.offer_id ?? ''}`}
                        className="relative isolate inline-flex h-[40px] w-full items-center justify-center gap-2 overflow-hidden rounded-md border border-violet/30 bg-violet px-4 text-[13px] font-bold text-white transition-[background-color,border-color,color,box-shadow,transform] duration-150 active:btn-volume-pressed active:translate-y-px btn-volume-primary btn-side-gradient"
                      >
                        <span className="relative z-[1] inline-flex items-center gap-2">Выбран <Check size={16} strokeWidth={2.5} /></span>
                      </Link>
                    ) : isComing ? (
                      <button
                        type="button"
                        disabled
                        className="inline-flex h-[40px] w-full cursor-not-allowed items-center justify-center rounded-md border border-white/10 bg-white/[0.04] px-4 text-[13px] font-semibold text-faint"
                      >
                        Скоро
                      </button>
                    ) : (
                      <Button
                        variant="secondary"
                        className="h-[40px] w-full rounded-md border-[rgba(168,85,247,0.32)] bg-transparent text-[13px] font-bold text-violet-bright hover:bg-violet/10 hover:text-text"
                        loading={joiningId === offer.offer_id}
                        onClick={() => handleJoin(offer.offer_id ?? '')}
                      >
                        Выбрать проект
                      </Button>
                    )}
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}

      <Modal open={joinSuccess != null} onClose={() => setJoinSuccess(null)} title="Оффер подключён">
        <div className="flex flex-col gap-4">
          <p className="text-[13.5px] text-muted">Ваша персональная трекинг-ссылка — используйте её в рекламных кампаниях:</p>
          <div className="flex min-w-0 items-center gap-3 rounded-lg border border-[rgba(168,85,247,0.28)] bg-surface-1 p-4 card-neon">
            <span className="min-w-0 flex-1 truncate font-mono text-[12.5px] text-text" title={joinSuccess?.trackingUrl}>
              {joinSuccess?.trackingUrl}
            </span>
            <CopyButton value={joinSuccess?.trackingUrl ?? ''} label="Копировать" />
          </div>
          <Button variant="primary" className="mt-1 rounded-md" onClick={goToStats}>
            Скопировать и перейти к статистике
          </Button>
          <span className="text-center text-[12px] text-faint">
            Это основная ссылка. Дополнительные ссылки по источникам создаются на странице статистики оффера
          </span>
        </div>
      </Modal>
    </>
  )
}
