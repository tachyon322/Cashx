const DEFAULTS = { refKey: "aff_ref", tokenKey: "click_token", maxAgeDays: 90 };
function readCookie(name) {
    const m = document.cookie.match(new RegExp(`(?:^|; )${name}=([^;]*)`));
    return m ? decodeURIComponent(m[1]) : null;
}
function writeCookie(name, value, days) {
    document.cookie = `${name}=${encodeURIComponent(value)}; path=/; max-age=${days * 86400}; samesite=lax`;
}
export function captureAffiliateParams(opts = {}) {
    const { refKey, tokenKey, maxAgeDays } = { ...DEFAULTS, ...opts };
    try {
        const url = new URL(window.location.href);
        let changed = false;
        for (const [param, key] of [["ref", refKey], ["click_token", tokenKey]]) {
            const value = url.searchParams.get(param);
            if (!value)
                continue;
            localStorage.setItem(key, value);
            writeCookie(key, value, maxAgeDays);
            url.searchParams.delete(param);
            changed = true;
        }
        if (changed) {
            const qs = url.searchParams.toString();
            window.history.replaceState({}, "", `${url.pathname}${qs ? `?${qs}` : ""}${url.hash}`);
        }
        else {
            // Re-hydrate localStorage from cookies (e.g. set by the /r/ route).
            for (const key of [refKey, tokenKey]) {
                if (!localStorage.getItem(key)) {
                    const c = readCookie(key);
                    if (c)
                        localStorage.setItem(key, c);
                }
            }
        }
        return { ref: localStorage.getItem(refKey), clickToken: localStorage.getItem(tokenKey) };
    }
    catch {
        return { ref: null, clickToken: null };
    }
}
/** Server-side: pull ref/click_token out of a raw Cookie header. */
export function parseAffiliateCookies(cookieHeader, opts = {}) {
    const { refKey, tokenKey } = { ...DEFAULTS, ...opts };
    const get = (name) => {
        const m = (cookieHeader ?? "").match(new RegExp(`(?:^|;\\s*)${name}=([^;]*)`));
        if (!m)
            return null;
        try {
            return decodeURIComponent(m[1]);
        }
        catch {
            return m[1];
        }
    };
    return { ref: get(refKey), clickToken: get(tokenKey) };
}
