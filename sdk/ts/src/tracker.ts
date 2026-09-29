// Browser-side capture of ?ref and ?click_token (set by the CashX redirect):
// persisted to localStorage + first-party cookies so the server can read them
// at signup, then stripped from the URL. Framework-free; call from an effect.
export type TrackerOptions = {
  refKey?: string;
  tokenKey?: string;
  maxAgeDays?: number;
};

const DEFAULTS = { refKey: "aff_ref", tokenKey: "click_token", maxAgeDays: 90 };

function readCookie(name: string): string | null {
  const m = document.cookie.match(new RegExp(`(?:^|; )${name}=([^;]*)`));
  return m ? decodeURIComponent(m[1]) : null;
}

function writeCookie(name: string, value: string, days: number): void {
  document.cookie = `${name}=${encodeURIComponent(value)}; path=/; max-age=${days * 86400}; samesite=lax`;
}

export function captureAffiliateParams(opts: TrackerOptions = {}): { ref: string | null; clickToken: string | null } {
  const { refKey, tokenKey, maxAgeDays } = { ...DEFAULTS, ...opts };
  try {
    const url = new URL(window.location.href);
    let changed = false;
    for (const [param, key] of [["ref", refKey], ["click_token", tokenKey]] as const) {
      const value = url.searchParams.get(param);
      if (!value) continue;
      localStorage.setItem(key, value);
      writeCookie(key, value, maxAgeDays);
      url.searchParams.delete(param);
      changed = true;
    }
    if (changed) {
      const qs = url.searchParams.toString();
      window.history.replaceState({}, "", `${url.pathname}${qs ? `?${qs}` : ""}${url.hash}`);
    } else {
      // Re-hydrate localStorage from cookies (e.g. set by the /r/ route).
      for (const key of [refKey, tokenKey]) {
        if (!localStorage.getItem(key)) {
          const c = readCookie(key);
          if (c) localStorage.setItem(key, c);
        }
      }
    }
    return { ref: localStorage.getItem(refKey), clickToken: localStorage.getItem(tokenKey) };
  } catch {
    return { ref: null, clickToken: null };
  }
}

/** Server-side: pull ref/click_token out of a raw Cookie header. */
export function parseAffiliateCookies(
  cookieHeader: string | null | undefined,
  opts: TrackerOptions = {},
): { ref: string | null; clickToken: string | null } {
  const { refKey, tokenKey } = { ...DEFAULTS, ...opts };
  const get = (name: string) => {
    const m = (cookieHeader ?? "").match(new RegExp(`(?:^|;\\s*)${name}=([^;]*)`));
    if (!m) return null;
    try {
      return decodeURIComponent(m[1]);
    } catch {
      return m[1];
    }
  };
  return { ref: get(refKey), clickToken: get(tokenKey) };
}
