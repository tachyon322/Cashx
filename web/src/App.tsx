import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createBrowserRouter, Navigate, Outlet, RouterProvider } from 'react-router-dom'
import type { ReactNode } from 'react'
import { AuthProvider, useAuth } from './auth/AuthContext'
import { AppLayout, PartnerLayout } from './components/layout/AppLayout'
import { AffiliateRefTracker } from './components/AffiliateRefTracker'
import { ToastProvider } from './components/Toast'
import { ClickBounce } from './pages/public/ClickBounce'
import { ForgotPage } from './pages/public/ForgotPage'
import { LoginPage } from './pages/public/LoginPage'
import { RegisterPage } from './pages/public/RegisterPage'
import { ResetPage } from './pages/public/ResetPage'
import { DashboardPage } from './pages/partner/DashboardPage'
import { OfferDetailPage } from './pages/partner/OfferDetailPage'
import { OffersPage } from './pages/partner/OffersPage'
import { PayoutsPage } from './pages/partner/PayoutsPage'
import { ProfilePage } from './pages/partner/ProfilePage'
import { ReferralsPage } from './pages/partner/ReferralsPage'
import { B2CReferralsPage } from './pages/partner/B2CReferralsPage'
import { LeaderboardPage } from './pages/partner/LeaderboardPage'
import { SettingsPage } from './pages/partner/SettingsPage'
import { AnnouncementsPage } from './pages/admin/AnnouncementsPage'
import { AuditPage } from './pages/admin/AuditPage'
import { BrandingPage } from './pages/admin/BrandingPage'
import { FinancePage } from './pages/admin/FinancePage'
import { IntegrationKeysPage } from './pages/admin/IntegrationKeysPage'
import { OffersAdminPage } from './pages/admin/OffersAdminPage'
import { PartnerDetailPage } from './pages/admin/PartnerDetailPage'
import { PartnersPage } from './pages/admin/PartnersPage'
import { StaffPage } from './pages/admin/StaffPage'
import { WithdrawalsPage } from './pages/admin/WithdrawalsPage'
import { BlockedPage, ForbiddenPage, PendingPage } from './pages/states'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // В этом деплое (общий/фоновый браузер + строгий рейт-лимитер auth)
      // refetch по фокусу окна превращает любой 429 в самоподдерживающийся
      // шторм запросов. Свежесть данных обеспечивается инвалидацией после
      // мутаций и refetchInterval (60s) уведомлений.
      refetchOnWindowFocus: false,
      refetchOnReconnect: false,
    },
  },
})

/* Splash на время загрузки /auth/me */
function SplashScreen() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-4 bg-bg bg-hero-glow">
      <div className="flex items-center gap-3 font-display text-[26px] font-bold">
        CashX
        <span className="h-[9px] w-[9px] rounded-full bg-violet shadow-[0_0_14px_rgba(168,85,247,0.9)]" />
      </div>
      <div className="h-7 w-7 animate-spin-slow rounded-full border-2 border-surface-2 border-t-violet-bright" />
    </div>
  )
}

/** Корневой гейт: splash только на первичной загрузке /auth/me (pending),
 *  в состоянии ошибки (429/500) показываем маршруты — иначе пользователь
 *  навсегда завис бы на splash при недоступном/рейт-лимиченном API. */
function AppGate() {
  const { isPending } = useAuth()
  if (isPending) return <SplashScreen />
  return (
    <>
      <AffiliateRefTracker />
      <Outlet />
    </>
  )
}

/** Публичные страницы доступны только неавторизованным. */
function PublicOnly() {
  const { user } = useAuth()
  if (!user) return <Outlet />
  const home = user.role === 'staff' ? '/admin/partners' : '/cabinet'
  return <Navigate to={home} replace />
}

/** Кабинет: наличие partner-профиля, approved && !blocked, иначе — состояние/редирект. */
function RequirePartner() {
  const { user } = useAuth()
  if (!user) return <Navigate to="/login" replace />
  if (!user.partner) {
    if (user.role === 'staff') return <Navigate to="/admin/partners" replace />
    return <PendingPage />
  }
  if (!user.is_approved) return <PendingPage />
  if (user.is_blocked) return <BlockedPage />
  return <Outlet />
}

/** Staff-зона: роли фильтруются (superadmin проходит всё). */
function RequireStaff({
  roles,
  children,
}: {
  roles?: readonly string[]
  children?: ReactNode
}) {
  const { user } = useAuth()
  if (!user) return <Navigate to="/login" replace />
  if (user.role !== 'staff') return <Navigate to="/cabinet" replace />
  const staffRoles = user.staff?.roles ?? []
  const allowed =
    !roles ||
    roles.length === 0 ||
    staffRoles.includes('superadmin') ||
    roles.some((role) => staffRoles.includes(role))
  if (!allowed) return <ForbiddenPage />
  return children ?? <Outlet />
}

/** Catch-all: редирект по состоянию входа. */
function CatchAll() {
  const { user } = useAuth()
  if (!user) return <Navigate to="/login" replace />
  return <Navigate to={user.role === 'staff' ? '/admin/partners' : '/cabinet'} replace />
}

/*
 * Реальные страницы подключены (шаги 6–8): публичные, кабинет партнёра и
 * админ-зона используют свои компоненты; структура маршрутов не меняется.
 */
const router = createBrowserRouter([
  {
    element: (
      <AuthProvider>
        <AppGate />
      </AuthProvider>
    ),
    children: [
      { index: true, element: <CatchAll /> },

      /* --- Публичные (только для неавторизованных) --- */
      {
        element: <PublicOnly />,
        children: [
          { path: '/login', element: <LoginPage /> },
          { path: '/register', element: <RegisterPage /> },
          { path: '/invite/:code', element: <RegisterPage /> },
          { path: '/forgot', element: <ForgotPage /> },
          { path: '/reset', element: <ResetPage /> },
        ],
      },

      /* --- Bounce трекинг-ссылок: доступен всем (шаг 6) --- */
      { path: '/c/:code', element: <ClickBounce /> },
      { path: '/r/:code', element: <ClickBounce /> },
      { path: '/partner', element: <Navigate to="/cabinet/offers" replace /> },

      /* --- Кабинет партнёра --- */
      {
        element: <RequirePartner />,
        children: [
          {
            element: <PartnerLayout />,
            children: [
              { path: '/cabinet', element: <DashboardPage /> },
              { path: '/cabinet/offers', element: <OffersPage /> },
              {
                path: '/cabinet/offers/:offerId',
                element: <OfferDetailPage />,
              },
              { path: '/cabinet/payouts', element: <PayoutsPage /> },
              { path: '/cabinet/referrals', element: <ReferralsPage /> },
              { path: '/cabinet/b2c-referrals', element: <B2CReferralsPage /> },
              { path: '/cabinet/leaderboard', element: <LeaderboardPage /> },
              { path: '/cabinet/settings', element: <SettingsPage /> },
              { path: '/cabinet/profile', element: <ProfilePage /> },
            ],
          },
        ],
      },

      /* --- Админ-зона staff --- */
      {
        element: <RequireStaff />,
        children: [
          {
            element: <AppLayout />,
            children: [
              { path: '/admin', element: <Navigate to="/admin/partners" replace /> },
              {
                element: <RequireStaff roles={['project_manager', 'support']} />,
                children: [
                  { path: '/admin/partners', element: <PartnersPage /> },
                  {
                    path: '/admin/partners/:id',
                    element: <PartnerDetailPage />,
                  },
                  { path: '/admin/projects', element: <Navigate to="/admin/offers" replace /> },
                  { path: '/admin/offers', element: <OffersAdminPage /> },
                ],
              },
              {
                path: '/admin/integration-keys',
                element: (
                  <RequireStaff roles={['project_manager']}>
                    <IntegrationKeysPage />
                  </RequireStaff>
                ),
              },
              {
                path: '/admin/withdrawals',
                element: (
                  <RequireStaff roles={['finance', 'support']}>
                    <WithdrawalsPage />
                  </RequireStaff>
                ),
              },
              {
                path: '/admin/finance',
                element: (
                  <RequireStaff roles={['finance']}>
                    <FinancePage />
                  </RequireStaff>
                ),
              },
              {
                path: '/admin/announcements',
                element: (
                  <RequireStaff roles={['content_manager']}>
                    <AnnouncementsPage />
                  </RequireStaff>
                ),
              },
              {
                path: '/admin/branding',
                element: (
                  <RequireStaff roles={['content_manager']}>
                    <BrandingPage />
                  </RequireStaff>
                ),
              },
              {
                path: '/admin/staff',
                element: (
                  <RequireStaff roles={['superadmin']}>
                    <StaffPage />
                  </RequireStaff>
                ),
              },
              {
                path: '/admin/audit',
                element: <AuditPage />,
              },
            ],
          },
        ],
      },

      { path: '*', element: <CatchAll /> },
    ],
  },
])

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <RouterProvider router={router} />
      </ToastProvider>
    </QueryClientProvider>
  )
}
