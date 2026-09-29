// HMAC client for the CashX integrations API (POST /events, GET /source).
// Pure transport: it signs and sends, throws when CashX is unreachable.
// Queueing/replay is the consumer's job (events are idempotent by event_id).
import { createHmac } from "node:crypto";
export class CashxTransportError extends Error {
    status;
    constructor(message, status) {
        super(message);
        this.status = status;
        this.name = "CashxTransportError";
    }
}
export function signBody(secret, body, nowMs = Date.now()) {
    const ts = Math.floor(nowMs / 1000).toString();
    const sig = createHmac("sha256", secret).update(ts + "." + body).digest("hex");
    return { ts, sig };
}
export function createCashxClient(cfg) {
    const baseUrl = cfg.baseUrl.replace(/\/$/, "");
    const doFetch = cfg.fetch ?? fetch;
    const retryDelays = cfg.retryDelaysMs ?? [500, 2000];
    const timeoutMs = cfg.timeoutMs ?? 6000;
    const enabled = () => cfg.enabled !== false && !!cfg.keyId && !!cfg.secret;
    function headers(body) {
        const { ts, sig } = signBody(cfg.secret, body, cfg.now?.());
        return {
            "Content-Type": "application/json",
            "X-CashX-Key": cfg.keyId,
            "X-CashX-Timestamp": ts,
            "X-CashX-Signature": sig,
        };
    }
    async function request(url, init) {
        const controller = new AbortController();
        const timer = setTimeout(() => controller.abort(), timeoutMs);
        try {
            return await doFetch(url, { ...init, signal: controller.signal });
        }
        finally {
            clearTimeout(timer);
        }
    }
    /**
     * Send one signed event. Retries transport errors and 5xx only; a 4xx is a
     * processed rejection and is returned as `ignored`. Throws
     * CashxTransportError when CashX stays unreachable.
     */
    async function sendEvent(input) {
        if (!enabled())
            return { status: "ignored", reason: "sync_disabled" };
        const body = JSON.stringify(input);
        const url = `${baseUrl}/api/v1/integrations/events`;
        let lastError = null;
        for (let attempt = 0; attempt <= retryDelays.length; attempt++) {
            if (attempt > 0)
                await new Promise((r) => setTimeout(r, retryDelays[attempt - 1]));
            let res;
            try {
                res = await request(url, { method: "POST", headers: headers(body), body });
            }
            catch (e) {
                lastError = new CashxTransportError(e instanceof Error ? e.message : String(e));
                continue;
            }
            const data = (await res.json().catch(() => ({})));
            if (res.status >= 500) {
                lastError = new CashxTransportError(`http_${res.status}`, res.status);
                continue;
            }
            if (!res.ok) {
                const msg = typeof data["message"] === "string" ? data["message"] : `http_${res.status}`;
                return { status: "ignored", reason: msg };
            }
            return {
                status: typeof data["status"] === "string" ? data["status"] : "accepted",
                reason: typeof data["reason"] === "string" ? data["reason"] : undefined,
                source: isEventSource(data["source"]) ? data["source"] : undefined,
            };
        }
        throw lastError ?? new CashxTransportError("unknown");
    }
    /** Resolve a tracking link / promo code inside the project scope; null if unknown. */
    async function lookupSource(code, clickToken) {
        if (!enabled())
            return null;
        const params = new URLSearchParams();
        const normalized = code?.trim().toUpperCase();
        if (normalized)
            params.set("code", normalized);
        const token = clickToken?.trim();
        if (token)
            params.set("click_token", token);
        if (!params.toString())
            return null;
        const res = await request(`${baseUrl}/api/v1/integrations/source?${params}`, {
            method: "GET",
            headers: headers(""),
        });
        if (res.status === 404)
            return null;
        if (!res.ok)
            throw new CashxTransportError(`source lookup ${res.status}`, res.status);
        return (await res.json());
    }
    const id = (kind, key) => `${cfg.prefix}-${kind}-${key}`;
    /** Builders: pure, so they can be stored in an outbox and sent later. */
    const events = {
        /** Returns null when there is neither a click token nor a source code. */
        attribution(userId, ref, clickToken, occurredAt = new Date()) {
            const token = clickToken?.trim() || undefined;
            const code = ref?.trim() || undefined;
            if (!token && !code)
                return null;
            return {
                event_id: id("signup", userId),
                type: "registration.created",
                occurred_at: occurredAt.toISOString(),
                external_user_id: userId,
                click_token: token,
                source_code: code,
            };
        },
        commission(userId, paymentId, amountKopecks, occurredAt, kind = "deposit") {
            return {
                event_id: id("payment", paymentId),
                type: "revenue.confirmed",
                occurred_at: occurredAt.toISOString(),
                external_user_id: userId,
                external_payment_id: id("payment", paymentId),
                amount_kopecks: Math.round(amountKopecks),
                currency: "RUB",
                kind,
            };
        },
        reversal(userId, paymentId, occurredAt = new Date()) {
            return {
                event_id: id("reverse", paymentId),
                type: "revenue.reversed",
                occurred_at: occurredAt.toISOString(),
                external_user_id: userId,
                external_payment_id: id("payment", paymentId),
            };
        },
    };
    return { sendEvent, lookupSource, events, isEnabled: enabled };
}
function isEventSource(v) {
    return typeof v === "object" && v !== null && typeof v.code === "string";
}
