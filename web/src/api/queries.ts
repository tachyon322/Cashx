import { useMemo } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, ApiRequestError, unwrap } from './client'
import type { components } from './schema'

/* ============================================================
   Обработка 401: регистрируется AuthProvider'ом; вызывается
   из защищённых хуков (не глобально в client.ts).
   ============================================================ */

let authSignOut: (() => void) | null = null

/** AuthProvider регистрирует свою signOut (без циклического импорта). */
export function setAuthSignOut(fn: (() => void) | null): void {
  authSignOut = fn
}

/** 401 на защищённом запросе → разлогинивание. */
export function handleAuthError(error: unknown): void {
  if (error instanceof ApiRequestError && error.status === 401) {
    authSignOut?.()
  }
}

/** Обёртка queryFn защищённых запросов: при 401 → signOut, ошибка пробрасывается дальше. */
async function guarded<T>(fn: () => Promise<T>): Promise<T> {
  try {
    return await fn()
  } catch (error) {
    handleAuthError(error)
    throw error
  }
}

/* ============================================================
   Параметры списков админки
   ============================================================ */

export interface AdminPartnersParams {
  search?: string
  status?: 'pending' | 'active' | 'blocked'
  limit?: number
  offset?: number
}

export interface AdminWithdrawalsParams {
  status?: string
  partner_id?: string
  limit?: number
  offset?: number
}

export interface AdminListParams {
  partner_id?: string
  from?: string
  to?: string
  limit?: number
  offset?: number
}

export interface AdminEarningsParams {
  partner_id?: string
  offer_id?: string
  from?: string
  to?: string
  limit?: number
  offset?: number
}

export interface AdminAuditParams {
  entity_type?: string
  entity_id?: string
  limit?: number
  offset?: number
}

/* ============================================================
   Запросы (GET)
   ============================================================ */

/** GET /auth/me — 401 означает «не вошёл» (null), остальные ошибки бросаем. */
export function useMe() {
  return useQuery({
    queryKey: ['me'],
    queryFn: async () => {
      const res = await api.GET('/auth/me')
      // 401 — не вошёл; 429 — временный rate-limit. Оба трактуем как null:
      // иначе error-состояние без данных зацикливает AppGate (mount → refetch → error).
      if (res.response.status === 401 || res.response.status === 429) return null
      if (res.error) {
        throw new ApiRequestError(res.response.status, 'Не удалось загрузить профиль')
      }
      return res.data.user ?? null
    },
    retry: false,
    retryOnMount: false,
    staleTime: 60_000,
  })
}

/** Псевдоним для страниц профиля. */
export function useProfile() {
  return useMe()
}

export function useSummary() {
  return useQuery({
    queryKey: ['summary'],
    queryFn: () => guarded(() => unwrap(api.GET('/cabinet/summary'))),
    retry: 1,
  })
}

export function useOffers() {
  return useQuery({
    queryKey: ['offers'],
    queryFn: () => guarded(() => unwrap(api.GET('/cabinet/offers'))),
    retry: 1,
  })
}

export function useOfferStats(offerId: string) {
  return useQuery({
    queryKey: ['offer-stats', offerId],
    queryFn: () =>
      guarded(() => unwrap(api.GET('/cabinet/offers/{offerId}', { params: { path: { offerId } } }))),
    enabled: offerId.length > 0,
    retry: 1,
  })
}

export function usePayoutConfig() {
  return useQuery({
    queryKey: ['payout-config'],
    queryFn: () => guarded(() => unwrap(api.GET('/cabinet/payouts/config'))),
    retry: 1,
  })
}

export function usePayouts() {
  return useQuery({
    queryKey: ['payouts'],
    queryFn: () => guarded(() => unwrap(api.GET('/cabinet/payouts'))),
    retry: 1,
  })
}

export function useReferrals() {
  return useQuery({
    queryKey: ['referrals'],
    queryFn: () => guarded(() => unwrap(api.GET('/cabinet/referrals'))),
    retry: 1,
  })
}

export function useNotifications() {
  return useQuery({
    queryKey: ['notifications'],
    queryFn: () => guarded(() => unwrap(api.GET('/cabinet/notifications'))),
    refetchInterval: 60_000,
    retry: 1,
  })
}

export function useAdminPartners(params: AdminPartnersParams = {}) {
  return useQuery({
    queryKey: ['admin', 'partners', params],
    queryFn: () => guarded(() => unwrap(api.GET('/admin/partners', { params: { query: params } }))),
    retry: 1,
  })
}

export function useAdminPartner(id: string) {
  return useQuery({
    queryKey: ['admin', 'partner', id],
    queryFn: () => guarded(() => unwrap(api.GET('/admin/partners/{id}', { params: { path: { id } } }))),
    enabled: id.length > 0,
    retry: 1,
  })
}

export function useAdminProjects() {
  return useQuery({
    queryKey: ['admin', 'projects'],
    queryFn: () => guarded(() => unwrap(api.GET('/admin/projects'))),
    retry: 1,
  })
}

export function useAdminOffers(projectId?: string) {
  return useQuery({
    queryKey: ['admin', 'offers', projectId ?? null],
    queryFn: () =>
      guarded(() =>
        unwrap(
          api.GET('/admin/offers', {
            params: { query: projectId ? { project_id: projectId } : {} },
          }),
        ),
      ),
    retry: 1,
  })
}

export function useAdminIntegrationKeys(projectId: string) {
  return useQuery({
    queryKey: ['admin', 'integration-keys', projectId],
    queryFn: () =>
      guarded(() =>
        unwrap(
          api.GET('/admin/integration-keys', {
            params: { query: { project_id: projectId } },
          }),
        ),
      ),
    enabled: projectId.length > 0,
    retry: 1,
  })
}

export function useAdminWithdrawals(params: AdminWithdrawalsParams = {}) {
  return useQuery({
    queryKey: ['admin', 'withdrawals', params],
    queryFn: () => guarded(() => unwrap(api.GET('/admin/withdrawals', { params: { query: params } }))),
    retry: 1,
  })
}

export function useAdminFinanceRules() {
  return useQuery({
    queryKey: ['admin', 'finance', 'rules'],
    queryFn: () => guarded(() => unwrap(api.GET('/admin/finance/rules'))),
    retry: 1,
  })
}

export function useAdminFinanceLedger(params: AdminListParams = {}) {
  return useQuery({
    queryKey: ['admin', 'finance', 'ledger', params],
    queryFn: () =>
      guarded(() => unwrap(api.GET('/admin/finance/ledger', { params: { query: params } }))),
    retry: 1,
  })
}

export function useAdminFinanceEarnings(params: AdminEarningsParams = {}) {
  return useQuery({
    queryKey: ['admin', 'finance', 'earnings', params],
    queryFn: () =>
      guarded(() => unwrap(api.GET('/admin/finance/earnings', { params: { query: params } }))),
    retry: 1,
  })
}

export function useAdminAnnouncements() {
  return useQuery({
    queryKey: ['admin', 'announcements'],
    queryFn: () => guarded(() => unwrap(api.GET('/admin/announcements'))),
    retry: 1,
  })
}

export function useAdminBranding() {
  return useQuery({
    queryKey: ['admin', 'branding'],
    queryFn: () => guarded(() => unwrap(api.GET('/admin/platform/branding'))),
    retry: 1,
  })
}

export function useAdminAudit(params: AdminAuditParams = {}) {
  return useQuery({
    queryKey: ['admin', 'audit', params],
    queryFn: () => guarded(() => unwrap(api.GET('/admin/audit', { params: { query: params } }))),
    retry: 1,
  })
}

/* ============================================================
   Мутации (POST / PATCH / PUT / DELETE)
   ============================================================ */

/** Инвалидирует перечисленные query-ключи (массивы = префиксы) после успешной мутации. */
function useInvalidate(...keys: unknown[][]): () => void {
  const qc = useQueryClient()
  return () => {
    for (const key of keys) {
      void qc.invalidateQueries({ queryKey: key })
    }
  }
}

/* --- Кабинет --- */

export function useJoinOffer() {
  const invalidate = useInvalidate(['offers'], ['summary'])
  return useMutation({
    mutationFn: (offerId: string) =>
      unwrap(api.POST('/cabinet/offers/{offerId}/join', { params: { path: { offerId } } })),
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

/* --- Источники трафика и потоки --- */

export function useSources(offerId: string) {
  return useQuery({
    queryKey: ['sources', offerId],
    queryFn: () =>
      guarded(() =>
        unwrap(
          api.GET('/cabinet/offers/{offerId}/sources', { params: { path: { offerId } } }),
        ),
      ),
    enabled: offerId.length > 0,
    retry: 1,
  })
}

export function useSourceGroups() {
  return useQuery({
    queryKey: ['source-groups'],
    queryFn: () => guarded(() => unwrap(api.GET('/cabinet/source-groups'))),
    retry: 1,
  })
}

/** Все источники партнёра по всем офферам — агрегат для дашборда. */
export function useAllSources() {
  const offersQuery = useOffers()
  const offerIds = useMemo(() => {
    const items = offersQuery.data?.items ?? []
    return items.map((o) => o.offer_id ?? '').filter((v): v is string => v.length > 0)
  }, [offersQuery.data])

  return useQuery({
    queryKey: ['all-sources', offerIds],
    queryFn: () =>
      guarded(async () => {
        const results = await Promise.all(
          offerIds.map((offerId) =>
            unwrap(api.GET('/cabinet/offers/{offerId}/sources', { params: { path: { offerId } } })),
          ),
        )
        return results.flatMap((r) => r.items ?? []) as components['schemas']['Source'][]
      }),
    enabled: offerIds.length > 0,
    retry: 1,
  })
}

export function useCreateSource(offerId: string) {
  const invalidate = useInvalidate(['sources', offerId], ['all-sources'], ['offer-stats', offerId], ['offers'], ['summary'])
  return useMutation({
    mutationFn: (input: components['schemas']['SourceInput']) =>
      unwrap(api.POST('/cabinet/offers/{offerId}/sources', { params: { path: { offerId } }, body: input })),
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

export function useUpdateSource(offerId: string) {
  const invalidate = useInvalidate(['sources', offerId], ['all-sources'], ['offer-stats', offerId], ['offers'], ['summary'])
  return useMutation({
    mutationFn: (input: components['schemas']['SourceUpdate'] & { id: string }) => {
      const { id, ...body } = input
      return unwrap(
        api.PATCH('/cabinet/offers/{offerId}/sources/{sourceId}', {
          params: { path: { offerId, sourceId: id } },
          body,
        }),
      )
    },
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

export function useDeleteSource(offerId: string) {
  const invalidate = useInvalidate(['sources', offerId], ['all-sources'], ['offer-stats', offerId], ['offers'], ['summary'])
  return useMutation({
    mutationFn: (sourceId: string) =>
      unwrap(
        api.DELETE('/cabinet/offers/{offerId}/sources/{sourceId}', {
          params: { path: { offerId, sourceId } },
        }),
      ),
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

export function useCreateGroup() {
  const invalidate = useInvalidate(['source-groups'])
  return useMutation({
    mutationFn: (input: components['schemas']['SourceGroupInput']) =>
      unwrap(api.POST('/cabinet/source-groups', { body: input })),
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

export function useUpdateGroup() {
  const invalidate = useInvalidate(['source-groups'], ['sources'])
  return useMutation({
    mutationFn: (input: components['schemas']['SourceGroupInput'] & { id: string }) => {
      const { id, ...body } = input
      return unwrap(
        api.PATCH('/cabinet/source-groups/{groupId}', { params: { path: { groupId: id } }, body }),
      )
    },
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

export function useDeleteGroup() {
  const invalidate = useInvalidate(['source-groups'], ['sources'])
  return useMutation({
    mutationFn: (groupId: string) =>
      unwrap(api.DELETE('/cabinet/source-groups/{groupId}', { params: { path: { groupId } } })),
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

export interface PayoutRequestInput {
  method: 'usdt' | 'sbp'
  amount_kopecks: number
  requisites: string
  bank?: string
}

export function useRequestPayout() {
  const invalidate = useInvalidate(['payouts'], ['summary'])
  return useMutation({
    mutationFn: (input: PayoutRequestInput) =>
      unwrap(api.POST('/cabinet/payouts/requests', { body: input })),
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

export function useCancelPayout() {
  const invalidate = useInvalidate(['payouts'], ['summary'])
  return useMutation({
    mutationFn: (id: string) =>
      unwrap(api.POST('/cabinet/payouts/requests/{id}/cancel', { params: { path: { id } } })),
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

export interface ProfileInput {
  name?: string
  telegram_user_id?: number
}

export function useUpdateProfile() {
  const invalidate = useInvalidate(['me'], ['summary'])
  return useMutation({
    mutationFn: (input: ProfileInput) => unwrap(api.PUT('/cabinet/profile', { body: input })),
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

export function useNotificationsReadAll() {
  const invalidate = useInvalidate(['notifications'])
  return useMutation({
    mutationFn: () => unwrap(api.POST('/cabinet/notifications/read-all')),
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

export interface NotificationReadInput {
  type: 'announcement' | 'personal'
  id: string
}

export function useNotificationRead() {
  const invalidate = useInvalidate(['notifications'])
  return useMutation({
    mutationFn: (input: NotificationReadInput) =>
      unwrap(
        api.POST('/cabinet/notifications/{type}/{id}/read', {
          params: { path: { type: input.type, id: input.id } },
        }),
      ),
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

/* --- Админ: партнёры --- */

export interface AdminPartnerCreateInput {
  name: string
  email: string
  password: string
  commission_bps?: number
}

export function useAdminPartnerCreate() {
  const invalidate = useInvalidate(['admin', 'partners'])
  return useMutation({
    mutationFn: (input: AdminPartnerCreateInput) =>
      unwrap(api.POST('/admin/partners', { body: input })),
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

export interface AdminPartnerUpdateInput {
  id: string
  name?: string
  email?: string
  password?: string
  is_approved?: boolean
  is_blocked?: boolean
  revshare_percent_bps?: number
}

export function useAdminPartnerUpdate() {
  const invalidate = useInvalidate(['admin', 'partners'], ['admin', 'partner'], ['me'])
  return useMutation({
    mutationFn: (input: AdminPartnerUpdateInput) => {
      const { id, ...body } = input
      return unwrap(api.PATCH('/admin/partners/{id}', { params: { path: { id } }, body }))
    },
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

export interface AdminPartnerRateInput {
  id: string
  offer_id: string
  rate_bps: number
}

export function useAdminPartnerRate() {
  const invalidate = useInvalidate(['admin', 'partners'], ['admin', 'partner'], ['offers'])
  return useMutation({
    mutationFn: (input: AdminPartnerRateInput) =>
      unwrap(
        api.POST('/admin/partners/{id}/rate', {
          params: { path: { id: input.id } },
          body: { offer_id: input.offer_id, rate_bps: input.rate_bps },
        }),
      ),
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

/* --- Админ: проекты --- */

export interface AdminProjectInput {
  slug: string
  name: string
  description?: string
  destination_url: string
  is_active?: boolean
}

export function useAdminProjectCreate() {
  const invalidate = useInvalidate(['admin', 'projects'])
  return useMutation({
    mutationFn: (input: AdminProjectInput) =>
      unwrap(api.POST('/admin/projects', { body: input })),
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

export interface AdminProjectUpdateInput {
  id: string
  name?: string
  description?: string
  destination_url?: string
  is_active?: boolean
}

export function useAdminProjectUpdate() {
  const invalidate = useInvalidate(['admin', 'projects'])
  return useMutation({
    mutationFn: (input: AdminProjectUpdateInput) => {
      const { id, ...body } = input
      return unwrap(api.PATCH('/admin/projects/{id}', { params: { path: { id } }, body }))
    },
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

/* --- Админ: офферы --- */

export interface AdminOfferCreateInput {
  project_id: string
  name: string
  category?: string
  description?: string
  destination_url?: string
  status?: 'active' | 'available' | 'pending' | 'coming_soon'
  rate_bps?: number
}

export function useAdminOfferCreate() {
  const invalidate = useInvalidate(['admin', 'offers'], ['offers'])
  return useMutation({
    mutationFn: (input: AdminOfferCreateInput) => {
      // RevShare теперь per-partner, offer rate deprecated — шлём дефолт для совместимости с текущим бэком
      const body = { rate_bps: 4000, ...input } as AdminOfferCreateInput & { rate_bps: number }
      return unwrap(api.POST('/admin/offers', { body: body as any }))
    },
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

export interface AdminOfferUpdateInput {
  id: string
  name?: string
  category?: string
  description?: string
  destination_url?: string
  status?: 'active' | 'available' | 'pending' | 'coming_soon'
}

export function useAdminOfferUpdate() {
  const invalidate = useInvalidate(['admin', 'offers'], ['offers'])
  return useMutation({
    mutationFn: (input: AdminOfferUpdateInput) => {
      const { id, ...body } = input
      return unwrap(api.PATCH('/admin/offers/{id}', { params: { path: { id } }, body }))
    },
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

export interface AdminOfferTermsInput {
  id: string
  rate_bps: number
}

export function useAdminOfferTerms() {
  const invalidate = useInvalidate(['admin', 'offers'], ['offers'])
  return useMutation({
    mutationFn: (input: AdminOfferTermsInput) =>
      unwrap(
        api.POST('/admin/offers/{id}/terms', {
          params: { path: { id: input.id } },
          body: { rate_bps: input.rate_bps },
        }),
      ),
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

/* --- Админ: интеграционные ключи --- */

export function useAdminIntegrationKeyCreate() {
  const invalidate = useInvalidate(['admin', 'integration-keys'])
  return useMutation({
    mutationFn: (project_id: string) =>
      unwrap(api.POST('/admin/integration-keys', { body: { project_id } })),
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

export function useAdminIntegrationKeyRotate() {
  const invalidate = useInvalidate(['admin', 'integration-keys'])
  return useMutation({
    mutationFn: (keyId: string) =>
      unwrap(api.POST('/admin/integration-keys/{keyId}/rotate', { params: { path: { keyId } } })),
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

export function useAdminIntegrationKeyDeactivate() {
  const invalidate = useInvalidate(['admin', 'integration-keys'])
  return useMutation({
    mutationFn: (keyId: string) =>
      unwrap(
        api.POST('/admin/integration-keys/{keyId}/deactivate', { params: { path: { keyId } } }),
      ),
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

/* --- Админ: выводы --- */

export interface AdminWithdrawalDecideInput {
  id: string
  decision: 'approved' | 'rejected'
  comment?: string
}

export function useAdminWithdrawalDecide() {
  const invalidate = useInvalidate(
    ['admin', 'withdrawals'],
    ['admin', 'partner'],
    ['payouts'],
    ['summary'],
  )
  return useMutation({
    mutationFn: (input: AdminWithdrawalDecideInput) =>
      unwrap(
        api.POST('/admin/withdrawals/{id}/decide', {
          params: { path: { id: input.id } },
          body: { decision: input.decision, comment: input.comment },
        }),
      ),
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

export interface AdminWithdrawalPayInput {
  id: string
  external_tx_id?: string
  comment?: string
}

export function useAdminWithdrawalPay() {
  const invalidate = useInvalidate(
    ['admin', 'withdrawals'],
    ['admin', 'partner'],
    ['payouts'],
    ['summary'],
  )
  return useMutation({
    mutationFn: (input: AdminWithdrawalPayInput) =>
      unwrap(
        api.POST('/admin/withdrawals/{id}/pay', {
          params: { path: { id: input.id } },
          body: { external_tx_id: input.external_tx_id, comment: input.comment },
        }),
      ),
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

/* --- Админ: финансы --- */

export interface AdminFinanceRulesInput {
  min_withdraw_kopecks?: number
  usdt_rate?: number
  sbp_fee_flat_kopecks?: number
  sbp_fee_percent_bps?: number
}

export function useAdminFinanceRulesPut() {
  const invalidate = useInvalidate(['admin', 'finance', 'rules'], ['payout-config'])
  return useMutation({
    mutationFn: (input: AdminFinanceRulesInput) =>
      unwrap(api.PUT('/admin/finance/rules', { body: input })),
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

/* --- Админ: анонсы --- */

export interface AdminAnnouncementInput {
  title?: string
  body?: string
  audience?: 'all' | 'partners' | 'staff' | 'specific_partner'
  partner_ids?: string[]
}

export function useAdminAnnouncementCreate() {
  const invalidate = useInvalidate(['admin', 'announcements'], ['notifications'])
  return useMutation({
    mutationFn: (input: AdminAnnouncementInput) =>
      unwrap(api.POST('/admin/announcements', { body: input })),
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

export interface AdminAnnouncementUpdateInput extends AdminAnnouncementInput {
  id: string
}

export function useAdminAnnouncementUpdate() {
  const invalidate = useInvalidate(['admin', 'announcements'], ['notifications'])
  return useMutation({
    mutationFn: (input: AdminAnnouncementUpdateInput) => {
      const { id, ...body } = input
      return unwrap(api.PATCH('/admin/announcements/{id}', { params: { path: { id } }, body }))
    },
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

export function useAdminAnnouncementDelete() {
  const invalidate = useInvalidate(['admin', 'announcements'], ['notifications'])
  return useMutation({
    mutationFn: (id: string) =>
      unwrap(api.DELETE('/admin/announcements/{id}', { params: { path: { id } } })),
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

/* --- Админ: команда / стафф --- */

export interface StaffMember {
  id: string
  email: string
  name: string
  is_active: boolean
  roles: string[]
  created_at: string
  updated_at: string
}

export function useAdminStaff(search?: string) {
  return useQuery({
    queryKey: ['admin', 'staff', search ?? ''],
    queryFn: () =>
      guarded(async () => {
        const qs = search ? `?search=${encodeURIComponent(search)}` : ''
        const res = await fetch(`/api/v1/admin/staff${qs}`, { credentials: 'include' })
        if (!res.ok) {
          const body = await res.json().catch(() => ({}))
          throw new ApiRequestError(res.status, (body as { message?: string }).message ?? `Ошибка ${res.status}`)
        }
        return (await res.json()) as { total: number; items: StaffMember[] }
      }),
    retry: 1,
  })
}

export function useAdminStaffCreate() {
  const invalidate = useInvalidate(['admin', 'staff'])
  return useMutation({
    mutationFn: async (input: { name: string; email: string; password: string; roles: string[] }) => {
      const res = await fetch('/api/v1/admin/staff', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(input),
      })
      if (!res.ok) {
        const body = await res.json().catch(() => ({}))
        throw new ApiRequestError(res.status, (body as { message?: string }).message ?? `Ошибка ${res.status}`)
      }
      return (await res.json()) as StaffMember
    },
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

export function useAdminStaffUpdate() {
  const invalidate = useInvalidate(['admin', 'staff'])
  return useMutation({
    mutationFn: async (input: { id: string; name?: string; email?: string; password?: string; is_active?: boolean; roles?: string[] }) => {
      const { id, ...body } = input
      const res = await fetch(`/api/v1/admin/staff/${encodeURIComponent(id)}`, {
        method: 'PATCH',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      if (!res.ok) {
        const bodyErr = await res.json().catch(() => ({}))
        throw new ApiRequestError(res.status, (bodyErr as { message?: string }).message ?? `Ошибка ${res.status}`)
      }
      return (await res.json()) as StaffMember
    },
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}

/* --- Админ: брендинг --- */

export interface AdminBrandingInput {
  name?: string
  telegram_url?: string
  avatar_url?: string | null
}

export function useAdminBrandingPut() {
  const invalidate = useInvalidate(['admin', 'branding'])
  return useMutation({
    mutationFn: (input: AdminBrandingInput) =>
      unwrap(api.PUT('/admin/platform/branding', { body: input })),
    onSuccess: invalidate,
    onError: handleAuthError,
  })
}
