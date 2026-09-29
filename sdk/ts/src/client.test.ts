import { describe, expect, test } from "bun:test";
import { createHmac } from "node:crypto";
import { createCashxClient, signBody, CashxTransportError } from "./client.js";
import { parseAffiliateCookies } from "./tracker.js";

const base = { baseUrl: "http://cashx", keyId: "k1", secret: "s3cret", prefix: "razdevator" };

function res(status: number, json: unknown = {}) {
  return new Response(JSON.stringify(json), { status });
}

describe("signBody", () => {
  test("hex HMAC-SHA256 of `${ts}.${body}`", () => {
    const { ts, sig } = signBody("s3cret", '{"a":1}', 1_700_000_000_000);
    expect(ts).toBe("1700000000");
    expect(sig).toBe(createHmac("sha256", "s3cret").update('1700000000.{"a":1}').digest("hex"));
  });
  test("GET signs an empty body", () => {
    expect(signBody("s", "", 5000).sig).toBe(createHmac("sha256", "s").update("5.").digest("hex"));
  });
});

describe("sendEvent", () => {
  test("sends signed headers and exact body", async () => {
    let seen: { url: string; init: RequestInit } | undefined;
    const c = createCashxClient({
      ...base,
      now: () => 1_700_000_000_000,
      fetch: (async (url: string, init: RequestInit) => {
        seen = { url, init };
        return res(202, { status: "accepted" });
      }) as unknown as typeof fetch,
    });
    const ev = c.events.attribution("u1", "abc")!;
    const out = await c.sendEvent(ev);
    expect(out.status).toBe("accepted");
    expect(seen!.url).toBe("http://cashx/api/v1/integrations/events");
    const h = seen!.init.headers as Record<string, string>;
    expect(h["X-CashX-Key"]).toBe("k1");
    expect(h["X-CashX-Signature"]).toBe(signBody("s3cret", seen!.init.body as string, 1_700_000_000_000).sig);
  });

  test("retries 5xx then succeeds", async () => {
    let n = 0;
    const c = createCashxClient({
      ...base,
      retryDelaysMs: [1, 1],
      fetch: (async () => (++n < 3 ? res(503) : res(202, { status: "duplicate" }))) as unknown as typeof fetch,
    });
    expect((await c.sendEvent(c.events.reversal("u", "p"))).status).toBe("duplicate");
    expect(n).toBe(3);
  });

  test("4xx is ignored without retry", async () => {
    let n = 0;
    const c = createCashxClient({
      ...base,
      fetch: (async () => (n++, res(400, { message: "invalid_payload" }))) as unknown as typeof fetch,
    });
    expect(await c.sendEvent(c.events.reversal("u", "p"))).toEqual({ status: "ignored", reason: "invalid_payload" });
    expect(n).toBe(1);
  });

  test("throws CashxTransportError when unreachable", async () => {
    const c = createCashxClient({
      ...base,
      retryDelaysMs: [1],
      fetch: (async () => {
        throw new Error("ECONNREFUSED");
      }) as unknown as typeof fetch,
    });
    await expect(c.sendEvent(c.events.reversal("u", "p"))).rejects.toBeInstanceOf(CashxTransportError);
  });

  test("disabled/no keys is a no-op", async () => {
    const c = createCashxClient({ ...base, secret: "" });
    expect((await c.sendEvent(c.events.reversal("u", "p"))).reason).toBe("sync_disabled");
  });
});

describe("event builders", () => {
  const c = createCashxClient(base);
  test("attribution needs ref or token", () => {
    expect(c.events.attribution("u")).toBeNull();
    expect(c.events.attribution("u", " ", "")).toBeNull();
    expect(c.events.attribution("u", "X", "T")).toMatchObject({ event_id: "razdevator-signup-u", source_code: "X", click_token: "T" });
  });
  test("commission uses prefixed ids and integer kopecks", () => {
    expect(c.events.commission("u", "p1", 1000.4, new Date(0))).toMatchObject({
      event_id: "razdevator-payment-p1",
      external_payment_id: "razdevator-payment-p1",
      amount_kopecks: 1000,
      currency: "RUB",
      kind: "deposit",
    });
  });
});

describe("parseAffiliateCookies", () => {
  test("reads and decodes", () => {
    expect(parseAffiliateCookies("a=1; aff_ref=AB%20C; click_token=tok")).toEqual({ ref: "AB C", clickToken: "tok" });
    expect(parseAffiliateCookies(null)).toEqual({ ref: null, clickToken: null });
  });
});
