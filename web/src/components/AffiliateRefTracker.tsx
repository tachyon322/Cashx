import { useEffect } from 'react'

/** Трекинг ?ref + click_token → localStorage + cookie 90д + чистка URL. Порт kazik AffiliateRefTracker.tsx */
export function AffiliateRefTracker() {
  useEffect(() => {
    try {
      const url = new URL(window.location.href)
      const ref = url.searchParams.get('ref')
      const ct = url.searchParams.get('click_token')

      let changed = false

      if (ref) {
        localStorage.setItem('aff_ref', ref)
        document.cookie = `aff_ref=${encodeURIComponent(ref)}; path=/; max-age=${90 * 86400}; samesite=lax`
        url.searchParams.delete('ref')
        changed = true
      }
      if (ct) {
        localStorage.setItem('click_token', ct)
        document.cookie = `click_token=${encodeURIComponent(ct)}; path=/; max-age=${90 * 86400}; samesite=lax`
        url.searchParams.delete('click_token')
        changed = true
      }

      if (changed) {
        const qs = url.searchParams.toString()
        const next = `${url.pathname}${qs ? `?${qs}` : ''}${url.hash}`
        window.history.replaceState({}, '', next)
      } else {
        // гидратация из cookies если localStorage пуст
        if (!localStorage.getItem('aff_ref')) {
          const m = document.cookie.match(/(?:^|; )aff_ref=([^;]*)/)
          if (m) localStorage.setItem('aff_ref', decodeURIComponent(m[1]))
        }
        if (!localStorage.getItem('click_token')) {
          const m = document.cookie.match(/(?:^|; )click_token=([^;]*)/)
          if (m) localStorage.setItem('click_token', decodeURIComponent(m[1]))
        }
      }

      // Попытка атрибуции B2C если есть ref/click_token и партнёр авторизован (кука сессии есть)
      const storedRef = localStorage.getItem('aff_ref')
      const storedCt = localStorage.getItem('click_token')
      if (storedRef || storedCt) {
        // fire-and-forget, ignoring 401
        fetch('/api/v1/cabinet/attrib', {
          method: 'POST',
          credentials: 'include',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ ref: storedRef ?? '', click_token: storedCt ?? '' }),
        }).catch(() => undefined)
      }
    } catch {
      // ignore
    }
  }, [])

  return null
}
