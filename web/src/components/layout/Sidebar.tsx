import { NavLink } from 'react-router-dom'
import type { LucideIcon } from 'lucide-react'
import {
  ArrowLeftRight,
  BadgePercent,
  CircleUserRound,
  Coins,
  FolderKanban,
  Headphones,
  KeyRound,
  LayoutDashboard,
  Megaphone,
  Palette,
  ScrollText,
  Tags,
  UserCog,
  Users,
  Wallet,
  X,
} from 'lucide-react'
import { useAuth } from '../../auth/AuthContext'
import { cx } from '../../lib/cx'

interface NavItem {
  to: string
  label: string
  icon: LucideIcon
  roles?: string[]
}

const CABINET_NAV: readonly NavItem[] = [
  { to: '/cabinet', label: 'Главная', icon: LayoutDashboard },
  { to: '/cabinet/offers', label: 'Офферы', icon: Tags },
  { to: '/cabinet/payouts', label: 'Выплаты', icon: Wallet },
  { to: '/cabinet/referrals', label: 'Рефералы', icon: Users },
  { to: '/cabinet/profile', label: 'Настройки', icon: CircleUserRound },
]

const ADMIN_NAV: readonly NavItem[] = [
  { to: '/admin/partners', label: 'Партнёры', icon: UserCog, roles: ['project_manager', 'support'] },
  { to: '/admin/projects', label: 'Проекты', icon: FolderKanban, roles: ['project_manager', 'support'] },
  { to: '/admin/offers', label: 'Офферы', icon: BadgePercent, roles: ['project_manager', 'support'] },
  { to: '/admin/integration-keys', label: 'Ключи', icon: KeyRound, roles: ['project_manager'] },
  { to: '/admin/withdrawals', label: 'Выводы', icon: ArrowLeftRight, roles: ['finance', 'support'] },
  { to: '/admin/finance', label: 'Финансы', icon: Coins, roles: ['finance'] },
  { to: '/admin/announcements', label: 'Анонсы', icon: Megaphone, roles: ['content_manager'] },
  { to: '/admin/branding', label: 'Брендинг', icon: Palette, roles: ['content_manager'] },
  { to: '/admin/audit', label: 'Аудит', icon: ScrollText },
]

function SupportCard() {
  return (
    <div className="rounded-xl border border-[rgba(168,85,247,0.18)] bg-[#0f0e1e] p-3">
      <div className="flex items-start justify-between">
        <div>
          <p className="text-[11px] font-bold uppercase tracking-[0.06em] text-violet-bright">Поддержка 24/7</p>
          <p className="mt-0.5 text-[11px] text-faint">Онлайн в чате</p>
        </div>
        <span className="inline-flex h-7 w-7 items-center justify-center rounded-full bg-white/[0.06] text-faint">
          <Headphones size={14} />
        </span>
      </div>
      <button className="relative isolate mt-3 inline-flex w-full items-center justify-center overflow-hidden rounded-md border border-white/10 bg-white/[0.06] px-3 py-2 text-[12px] font-semibold text-muted transition-[background-color,border-color,color,box-shadow,transform] duration-150 hover:bg-white/[0.10] hover:text-text active:btn-volume-pressed active:translate-y-px btn-volume-ghost btn-side-gradient">
        <span className="relative z-[1]">Написать</span>
      </button>
    </div>
  )
}

export function Sidebar({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { user } = useAuth()
  const staffRoles = user?.staff?.roles ?? []
  const canSee = (item: NavItem) =>
    !item.roles || item.roles.length === 0 || staffRoles.includes('superadmin') || item.roles.some((r) => staffRoles.includes(r))

  const groups: { title: string; items: NavItem[] }[] = []
  if (user?.partner) {
    groups.push({ title: 'Кабинет', items: [...CABINET_NAV] })
  }
  if (user?.role === 'staff') {
    groups.push({ title: 'Администрирование', items: ADMIN_NAV.filter(canSee) })
  }

  const showExtra = Boolean(user?.partner)

  return (
    <aside
      className={cx(
        'fixed inset-y-0 left-0 z-[60] flex w-[248px] flex-col border-r border-[rgba(168,85,247,0.18)] bg-[#080815] p-3 max-[899px]:transition-transform max-[899px]:duration-[220ms]',
        open
          ? 'max-[899px]:translate-x-0 max-[899px]:shadow-[24px_0_60px_rgba(0,0,0,0.5)]'
          : 'max-[899px]:-translate-x-full max-[899px]:shadow-none',
      )}
    >
      <div className="flex items-center justify-between px-2 pb-5 pt-0.5">
        <NavLink to="/" className="flex items-center gap-2 font-display text-[22px] font-bold tracking-[0.02em] text-text" onClick={onClose}>
          <span className="bg-gradient-to-r from-white to-[#d8b4fe] bg-clip-text text-transparent">Cashx</span>
          <span className="bg-gradient-to-r from-violet-bright to-violet bg-clip-text font-black text-transparent">Pay</span>
          <span className="h-2 w-2 rounded-full bg-violet shadow-[0_0_12px_rgba(168,85,247,0.8)]" />
        </NavLink>
        <button
          type="button"
          className="hidden h-[30px] w-[30px] cursor-pointer items-center justify-center rounded-md border-none bg-transparent text-muted transition-colors duration-150 hover:bg-surface-hover hover:text-text max-[899px]:inline-flex"
          onClick={onClose}
          aria-label="Закрыть меню"
        >
          <X size={18} />
        </button>
      </div>

      <div className="flex flex-1 flex-col gap-4 overflow-y-auto -mx-2 px-2">
        <nav className="flex flex-col gap-4">
          {groups.map((group) => (
            <div key={group.title} className="flex flex-col gap-1">
              <div className="px-2.5 pb-1.5 text-[10px] font-semibold uppercase leading-[1.4] tracking-[0.08em] text-faint">
                {group.title}
              </div>
              {group.items.map((item) => (
                <NavLink
                  key={item.to}
                  to={item.to}
                  end={item.to === '/cabinet'}
                  className={({ isActive }) =>
                    cx(
                      'flex items-center gap-3 rounded-lg border border-transparent px-2.5 py-2 text-[13px] font-medium text-muted transition-colors duration-150 hover:bg-white/[0.04] hover:text-text',
                      isActive && 'border-[rgba(168,85,247,0.35)] bg-[rgba(168,85,247,0.12)] text-text shadow-[0_0_18px_rgba(121,40,255,0.18)]',
                    )
                  }
                  onClick={onClose}
                >
                  <item.icon size={18} strokeWidth={1.75} className="shrink-0" />
                  <span>{item.label}</span>
                </NavLink>
              ))}
            </div>
          ))}
        </nav>

        {showExtra && (
          <div className="flex flex-col gap-3 pt-2">
            <SupportCard />
          </div>
        )}
      </div>
    </aside>
  )
}
