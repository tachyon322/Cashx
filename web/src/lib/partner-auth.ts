/** Партнёрский токен в cookie + localStorage, порт front/lib/partner-auth.ts */
export const PARTNER_TOKEN_COOKIE = 'partner_token'
const LS_KEY = 'partner_token'
const MAX_AGE = 30 * 24 * 60 * 60 // 30 дней

export function setPartnerTokenCookie(token: string): void {
  try {
    document.cookie = `${PARTNER_TOKEN_COOKIE}=${encodeURIComponent(token)}; path=/; max-age=${MAX_AGE}; samesite=lax`
    localStorage.setItem(LS_KEY, token)
  } catch {
    // ignore
  }
}

export function getPartnerToken(): string | null {
  try {
    const ls = localStorage.getItem(LS_KEY)
    if (ls) return ls
    const m = document.cookie.match(new RegExp(`(?:^|; )${PARTNER_TOKEN_COOKIE}=([^;]*)`))
    if (m) return decodeURIComponent(m[1])
  } catch {
    // ignore
  }
  return null
}

export function clearPartnerToken(): void {
  try {
    document.cookie = `${PARTNER_TOKEN_COOKIE}=; path=/; max-age=0`
    localStorage.removeItem(LS_KEY)
  } catch {
    // ignore
  }
}
