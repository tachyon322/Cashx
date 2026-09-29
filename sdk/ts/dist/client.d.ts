export type EventKind = "deposit" | "gate";
export type EventInput = {
    event_id: string;
    type: "registration.created" | "revenue.confirmed" | "revenue.reversed";
    occurred_at: string;
    external_user_id: string;
    click_token?: string;
    source_code?: string;
    external_payment_id?: string;
    amount_kopecks?: number;
    currency?: "RUB";
    kind?: EventKind;
};
export type EventSource = {
    code: string;
    type: string;
    is_promo: boolean;
    registration_bonus?: number;
};
export type ProcessResult = {
    status: "accepted" | "duplicate" | "ignored";
    reason?: string;
    source?: EventSource;
};
export type SourceInfo = {
    code: string;
    type: string;
    is_promo: boolean;
    is_active: boolean;
    access_active: boolean;
    registration_bonus?: number;
};
export type CashxConfig = {
    baseUrl: string;
    keyId: string;
    secret: string;
    /** Prefix for event_id / external_payment_id, e.g. "razdevator". */
    prefix: string;
    enabled?: boolean;
    timeoutMs?: number;
    /** Delays between transport retries; [] disables retries (use an outbox). */
    retryDelaysMs?: number[];
    fetch?: typeof fetch;
    now?: () => number;
};
export declare class CashxTransportError extends Error {
    readonly status?: number | undefined;
    constructor(message: string, status?: number | undefined);
}
export declare function signBody(secret: string, body: string, nowMs?: number): {
    ts: string;
    sig: string;
};
export type CashxClient = ReturnType<typeof createCashxClient>;
export declare function createCashxClient(cfg: CashxConfig): {
    sendEvent: (input: EventInput) => Promise<ProcessResult>;
    lookupSource: (code?: string, clickToken?: string) => Promise<SourceInfo | null>;
    events: {
        /** Returns null when there is neither a click token nor a source code. */
        attribution(userId: string, ref?: string, clickToken?: string, occurredAt?: Date): EventInput | null;
        commission(userId: string, paymentId: string, amountKopecks: number, occurredAt: Date, kind?: EventKind): EventInput;
        reversal(userId: string, paymentId: string, occurredAt?: Date): EventInput;
    };
    isEnabled: () => boolean;
};
