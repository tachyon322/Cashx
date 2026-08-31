/** Типы и хелперы партнёрки, порт front/lib/api.ts partnerApi */

export type AffiliateSourceType = 'link' | 'promo'

export interface SourceInput {
  name?: string
  type?: AffiliateSourceType
  code?: string
  registrationBonus?: number | null
  groupId?: string | null
  redirectId?: string | null
  domain?: string | null
  comment?: string | null
  isActive?: boolean
}

export interface SourceStatsAggregate {
  clicks: number
  uniqueClicks: number
  signups: number
  promos: number
  depositors: number
  depositsCount: number
  depositsSum: number
  income: number
  cr: number | null
  crPayment: number | null
}

export function buildAffiliateLink(code: string, domain: string | null, defaultDomain: string): string {
  const origin = domain ?? defaultDomain
  const base = origin ? origin.replace(/\/$/, '') : window.location.origin.replace(/\/$/, '')
  return `${base}/r/${code}`
}

export function normalizeCode(raw: string): string {
  return raw.trim().toUpperCase().replace(/[^A-Z0-9_]/g, '')
}

export const PROMO_BONUS_DEFAULT = 500
