import { useEffect, useRef, useState } from 'react'
import { ChevronDown, HelpCircle, LogOut, Menu } from 'lucide-react'
import { useLocation } from 'react-router-dom'
import { useAuth } from '../../auth/AuthContext'
import { NotificationsBell } from '../NotificationsBell'

const TITLES: readonly (readonly [prefix: string, title: string])[] = [
  ['/cabinet/offers/', 'Статистика оффера'],
  ['/cabinet/offers', 'Офферы'],
  ['/cabinet/payouts', 'Выплаты'],
  ['/cabinet/referrals', 'Рефералы'],
  ['/cabinet/profile', 'Профиль'],
  ['/cabinet', 'Дашборд'],
  ['/admin/partners/', 'Карточка партнёра'],
  ['/admin/partners', 'Партнёры'],
  ['/admin/projects', 'Проекты'],
  ['/admin/offers', 'Офферы'],
  ['/admin/integration-keys', 'Интеграционные ключи'],
  ['/admin/withdrawals', 'Выводы'],
  ['/admin/finance', 'Финансы'],
  ['/admin/announcements', 'Анонсы'],
  ['/admin/branding', 'Брендинг'],
  ['/admin/audit', 'Аудит'],
]

function usePageTitle(): string {
  const { pathname } = useLocation()
  const hit = TITLES.find(([prefix]) => pathname.startsWith(prefix))
  return hit?.[1] ?? 'CashX'
}

function UserChip() {
  const { user, signOut } = useAuth()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const onDocClick = (event: MouseEvent) => {
      if (ref.current && !ref.current.contains(event.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDocClick)
    return () => document.removeEventListener('mousedown', onDocClick)
  }, [open])

  if (!user) return null

  const roleLabel =
    user.role === 'staff'
      ? user.staff?.roles?.includes('superadmin')
        ? 'Супер-админ'
        : 'Администратор'
      : 'Партнёр'
  const initials = (user.name ?? user.email ?? '?')
    .split(/\s+/)
    .slice(0, 2)
    .map((word) => word[0]?.toUpperCase() ?? '')
    .join('')

  const shortId = user.id ? user.id.slice(0, 5).toUpperCase() : '—'

  return (
    <div className="relative" ref={ref}>
      <button
        type="button"
        className="flex cursor-pointer items-center gap-2.5 rounded-full border border-border bg-surface-1 px-2 py-1 text-text transition-colors duration-150 hover:bg-surface-hover"
        onClick={() => setOpen((value) => !value)}
        aria-haspopup="menu"
        aria-expanded={open}
      >
        <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-violet text-[12px] font-bold text-white">
          {initials}
        </span>
        <span className="hidden flex-col items-start leading-none md:flex">
          <span className="whitespace-nowrap text-[12px] font-semibold">{user.name ?? user.email}</span>
          <span className="text-[10px] text-faint">ID: {shortId}</span>
        </span>
        <span className="hidden whitespace-nowrap rounded-full bg-violet/15 px-2 py-0.5 text-[10px] font-semibold tracking-[0.04em] text-lilac max-[899px]:hidden xl:inline">
          {roleLabel}
        </span>
        <ChevronDown size={14} className="text-muted" />
      </button>
      {open && (
        <div
          className="absolute right-0 top-[calc(100%+8px)] z-50 min-w-[200px] rounded-lg border border-border bg-surface-2 p-1.5 shadow-card"
          role="menu"
        >
          <button
            type="button"
            className="flex w-full cursor-pointer items-center gap-2 rounded-md border-none bg-none px-2.5 py-[9px] text-[13px] font-medium text-danger transition-colors duration-150 hover:bg-danger/10"
            role="menuitem"
            onClick={() => {
              setOpen(false)
              void signOut()
            }}
          >
            <LogOut size={15} />
            Выйти
          </button>
        </div>
      )}
    </div>
  )
}

export function Topbar({ onMenu }: { onMenu: () => void }) {
  const { user, isLoading } = useAuth()
  const hasPartnerCabinet = !isLoading && user?.partner != null

  return (
    <header className="sticky top-0 z-40 flex h-16 items-center justify-between gap-3 border-b border-[rgba(168,85,247,0.18)] bg-[rgba(8,8,21,0.85)] px-4 backdrop-blur-md">
      <div className="flex min-w-0 items-center gap-3">
        <button
          type="button"
          className="hidden h-[34px] w-[34px] cursor-pointer items-center justify-center rounded-md border border-border bg-surface-1 text-text transition-colors duration-150 hover:bg-surface-hover max-[899px]:inline-flex"
          onClick={onMenu}
          aria-label="Открыть меню"
        >
          <Menu size={20} />
        </button>
        <h1 className="hidden truncate text-[13px] font-semibold uppercase tracking-[0.06em] text-faint md:block">
          Партнёрская программа
        </h1>
        <span className="truncate text-[16px] font-semibold text-text md:hidden">{usePageTitle()}</span>
      </div>
      <div className="flex items-center gap-2.5">
        {hasPartnerCabinet && <NotificationsBell />}
        <button
          type="button"
          className="hidden h-8 w-8 items-center justify-center rounded-md border border-border bg-surface-1 text-muted hover:bg-surface-hover hover:text-text md:inline-flex"
          aria-label="Помощь"
        >
          <HelpCircle size={16} />
        </button>
        <UserChip />
      </div>
    </header>
  )
}
