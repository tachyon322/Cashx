export type TrackerOptions = {
    refKey?: string;
    tokenKey?: string;
    maxAgeDays?: number;
};
export declare function captureAffiliateParams(opts?: TrackerOptions): {
    ref: string | null;
    clickToken: string | null;
};
/** Server-side: pull ref/click_token out of a raw Cookie header. */
export declare function parseAffiliateCookies(cookieHeader: string | null | undefined, opts?: TrackerOptions): {
    ref: string | null;
    clickToken: string | null;
};
