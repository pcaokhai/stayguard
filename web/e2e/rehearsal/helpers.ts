import { createHmac, randomUUID } from "node:crypto";
import { execFileSync } from "node:child_process";
import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { expect, test as base, type APIRequestContext, type Page } from "@playwright/test";

// Shared by the rehearsal specs (`make rehearse-test`). Every call goes through Api, which keeps a log (secrets masked) that the
// reporter files as evidence. Sessions are API sign-ins; pages get the same session as the sg_session cookie.
export const cfg = {
  base: process.env.E2E_BASE_URL ?? "",
  guesthouse: process.env.RH_GUESTHOUSE ?? "",
  hook: process.env.RH_HOOK_PATH ?? "",
  secret: process.env.RH_SEPAY_SECRET ?? "",
  account: process.env.RH_ACCOUNT_NO ?? "",
  pinsFile: process.env.RH_PINS_FILE ?? "",
  root: process.env.RH_ROOT ?? "",
  envFile: process.env.RH_ENV_FILE ?? "deploy/.env.rehearse",
};
export const configured = Boolean(cfg.hook);
export const NEW_PIN = "482916"; // not a run, not a repeated digit
const SECRET_KEYS = /pin|token|idNumber|number|secret|authorization/i;

export type Who = { user: string; token: string; id: string; role: string };
export type Res = { status: number; body: any; headers: Record<string, string> };
type Opts = { key?: string; headers?: Record<string, string>; raw?: string };

// ---- sign-in state shared by all spec files (one worker, but a failed test restarts it) ----
const tokens = new Map<string, Who>();
const setFile = () => `${cfg.pinsFile}.set`;
const readJson = (p: string) => (existsSync(p) ? JSON.parse(readFileSync(p, "utf8")) : {});
export const chosenPin = (user: string): string | undefined => readJson(setFile())[user];
export const rememberPin = (user: string, pin: string) =>
  writeFileSync(setFile(), JSON.stringify({ ...readJson(setFile()), [user]: pin }));
export const forgetSession = (user: string) => tokens.delete(user);

const clip = (v: unknown) => (typeof v === "string" ? v : JSON.stringify(v ?? "")).slice(0, 600);
const mask = (v: unknown): unknown =>
  Array.isArray(v)
    ? v.map(mask)
    : v && typeof v === "object"
      ? Object.fromEntries(
          Object.entries(v).map(([k, x]) => [
            k,
            SECRET_KEYS.test(k) && typeof x === "string" ? "***" : mask(x),
          ]),
        )
      : v;

export class Api {
  readonly log: string[] = [];
  constructor(readonly request: APIRequestContext) {}

  async call(
    who: Who | null,
    method: string,
    path: string,
    body?: unknown,
    o: Opts = {},
  ): Promise<Res> {
    const headers: Record<string, string> = { ...(o.headers ?? {}) };
    if (who) headers.authorization = `Bearer ${who.token}`;
    if (method !== "GET" && method !== "HEAD") headers["idempotency-key"] = o.key ?? randomUUID();
    const res = await this.request.fetch(cfg.base + path, {
      method,
      headers:
        body === undefined && o.raw === undefined
          ? headers
          : { "content-type": "application/json", ...headers },
      data: o.raw ?? (body === undefined ? undefined : JSON.stringify(body)),
      failOnStatusCode: false,
    });
    const text = await res.text();
    let parsed: any = text;
    try {
      parsed = text ? JSON.parse(text) : null;
    } catch {
      /* not JSON */
    }
    this.log.push(
      `${method} ${path} ${body === undefined ? "" : clip(mask(body))}\n  -> ${res.status()} ${clip(mask(parsed))}`,
    );
    return { status: res.status(), body: parsed, headers: res.headers() };
  }
  get = (w: Who | null, p: string) => this.call(w, "GET", p);
  post = (w: Who | null, p: string, b?: unknown, o?: Opts) => this.call(w, "POST", p, b, o);
  put = (w: Who | null, p: string, b?: unknown, o?: Opts) => this.call(w, "PUT", p, b, o);
  patch = (w: Who | null, p: string, b?: unknown, o?: Opts) => this.call(w, "PATCH", p, b, o);

  /** Signs in with the PIN the person chose (or, the first time, the installer's one-time PIN, then chooses NEW_PIN). Cached. */
  async as(user: string): Promise<Who> {
    const cached = tokens.get(user);
    if (cached) return cached;
    const one = (readJson(cfg.pinsFile) as Record<string, string>)[user];
    const known = chosenPin(user);
    // 20 sign-ins per minute per address: wait out a 429 instead of failing a case for it.
    const signIn = async (pin: string): Promise<Res> => {
      for (let i = 0; ; i++) {
        const res = await this.post(null, "/v1/auth/sign-in", {
          guesthouseCode: cfg.guesthouse,
          username: user,
          pin,
        });
        if (res.status !== 429 || i >= 2) return res;
        await sleep(Number(res.headers["retry-after"] ?? 60) * 1000);
      }
    };
    let r = await signIn(known ?? one);
    if (r.status === 401 && !known) r = await signIn(NEW_PIN); // a hand run already chose it
    expect(r.status, `sign-in as ${user}`).toBe(200);
    if (r.body.mustChangePin) {
      const t = { user, token: r.body.accessToken, id: r.body.user.id, role: r.body.user.role };
      const c = await this.put(t, "/v1/me/pin", { currentPin: one, newPin: NEW_PIN });
      expect(c.status, `first PIN of ${user}`).toBe(204);
      rememberPin(user, NEW_PIN);
      // The session that changed the PIN stays valid (only other sessions end), which saves a second sign-in against the limit.
      const me = await this.get(t, "/v1/me");
      if (me.status !== 200) r = await signIn(NEW_PIN);
    }
    const who = {
      user,
      token: r.body.accessToken as string,
      id: r.body.user.id as string,
      role: r.body.user.role as string,
    };
    tokens.set(user, who);
    return who;
  }

  // ---- the front-desk flow, by API ----
  buildingId?: string;
  async rooms(w: Who): Promise<any[]> {
    this.buildingId ??= (await this.get(w, "/v1/buildings")).body.items[0].id;
    return (await this.get(w, `/v1/buildings/${this.buildingId}/rooms`)).body.items;
  }
  async room(w: Who, code: string) {
    return (await this.rooms(w)).find((r) => r.code === code);
  }
  /** The first vacant room that no earlier case used (rooms are never reused within a run unless a case cleans them). */
  async vacantRoom(w: Who) {
    const r = (await this.rooms(w)).find((x) => x.status === "VACANT");
    expect(r, "a vacant room is left in the rehearsal building").toBeTruthy();
    return r;
  }
  async checkIn(
    w: Who,
    o: { room?: any; deposit?: number; rentalType?: string; name?: string; idNumber?: string } = {},
  ) {
    const room = o.room ?? (await this.vacantRoom(w));
    const r = await this.post(w, `/v1/rooms/${room.id}/stays`, {
      rentalType: o.rentalType ?? "HOURLY",
      guestName: o.name ?? "Rehearsal Guest",
      guestPhone: "0912345678",
      deposit: o.deposit ?? 10_000,
      ...(o.idNumber ? { idNumber: o.idNumber } : {}),
    });
    expect(r.status, "check-in").toBe(201);
    return r.body;
  }
  async addWater(w: Who, stayId: string, qty: number) {
    const r = await this.post(w, `/v1/stays/${stayId}/extras`, {
      items: [{ serviceCode: "WATER", quantity: qty }],
    });
    expect(r.status, "extras").toBeLessThan(300);
  }
  async checkout(w: Who, stayId: string) {
    const r = await this.post(w, `/v1/stays/${stayId}/checkout`);
    expect(r.status, "check-out").toBeLessThan(300);
    return r.body; // the invoice
  }
  /** Check in, optionally add water, check out and start a bank transfer: the QR the receptionist shows. */
  async toQr(w: Who, o: { deposit?: number; water?: number; room?: any } = {}) {
    const stay = await this.checkIn(w, { deposit: o.deposit, room: o.room });
    if (o.water) await this.addWater(w, stay.id, o.water);
    const invoice = await this.checkout(w, stay.id);
    const p = await this.post(w, `/v1/invoices/${invoice.id}/payments`, { method: "TRANSFER" });
    expect(p.status, "start transfer").toBe(201);
    return {
      stay,
      invoice,
      payment: p.body,
      note: invoice.billCode as string,
      amount: p.body.amount as number,
    };
  }
  async getStay(w: Who, id: string) {
    return (await this.get(w, `/v1/stays/${id}`)).body;
  }
  async payment(w: Who, id: string) {
    return (await this.get(w, `/v1/payments/${id}`)).body;
  }

  // ---- owner views ----
  async alerts(owner: Who): Promise<any[]> {
    return (await this.get(owner, "/v1/owner/alerts")).body.items;
  }
  async transactions(owner: Who, q = ""): Promise<any[]> {
    return (await this.get(owner, `/v1/owner/transactions${q}`)).body.items;
  }
  async overview(owner: Who) {
    return (await this.get(owner, "/v1/owner/overview")).body;
  }
  /** The activity log for yesterday to tomorrow (the API wants a date range). */
  async audit(owner: Who, q = ""): Promise<any[]> {
    const day = (d: number) => new Date(Date.now() + d * 86_400_000).toISOString().slice(0, 10);
    return (await this.get(owner, `/v1/owner/audit-logs?from=${day(-1)}&to=${day(1)}${q}`)).body
      .items;
  }
  async shift(w: Who) {
    return this.get(w, "/v1/shifts/current");
  }

  // ---- the bank, as SePay sends it ----
  /** A signed SePay webhook: sha256= and the hex HMAC-SHA256 of "{timestamp}.{raw body}" with the tenant's secret. */
  async deliver(o: {
    note?: string;
    content?: string;
    amount: number;
    id?: number;
    type?: "in" | "out";
    accountNumber?: string;
    ageSeconds?: number;
    secret?: string;
  }): Promise<{ status: number; body: any; id: number }> {
    const id = o.id ?? Date.now() * 1000 + Math.floor(Math.random() * 1000);
    const body = JSON.stringify({
      id,
      gateway: "Vietcombank",
      transactionDate: new Date(Date.now() + 7 * 3600_000)
        .toISOString()
        .slice(0, 19)
        .replace("T", " "),
      accountNumber: o.accountNumber ?? cfg.account,
      subAccount: "", // the virtual-account field SePay always sends
      code: null,
      content: o.content ?? `${o.note ?? ""} chuyen tien`,
      transferType: o.type ?? "in",
      description: "rehearsal transfer",
      transferAmount: o.amount,
      accumulated: 0,
      referenceCode: `FT${id}`,
    });
    const timestamp = String(Math.floor(Date.now() / 1000) - (o.ageSeconds ?? 0));
    const signature =
      "sha256=" +
      createHmac("sha256", o.secret ?? cfg.secret)
        .update(`${timestamp}.${body}`)
        .digest("hex");
    const res = await this.request.post(cfg.base + cfg.hook, {
      data: body,
      headers: {
        "content-type": "application/json",
        "x-sepay-signature": signature,
        "x-sepay-timestamp": timestamp,
      },
      failOnStatusCode: false,
    });
    const text = await res.text();
    this.log.push(
      `WEBHOOK ${o.type ?? "in"} ${o.amount} "${(o.note ?? o.content ?? "").slice(0, 40)}" id=${id} age=${o.ageSeconds ?? 0}s\n  -> ${res.status()} ${text.slice(0, 200)}`,
    );
    let parsed: any = text;
    try {
      parsed = JSON.parse(text);
    } catch {
      /* not JSON */
    }
    return { status: res.status(), body: parsed, id };
  }
  /** Delivers an incoming transfer and expects SePay's {"success": true}. */
  async pay(
    note: string,
    amount: number,
    extra: Parameters<Api["deliver"]>[0] extends infer T ? Partial<T> : never = {},
  ) {
    const r = await this.deliver({ note, amount, ...extra });
    expect(r.status, "the webhook must be accepted").toBe(200);
    expect(r.body).toEqual({ success: true });
    return r;
  }
  /** Waits for the payment to reach a status (the screen polls every 3 s; the API settles at once). */
  async untilPayment(w: Who, id: string, status: string) {
    await expect
      .poll(async () => (await this.payment(w, id)).status, { timeout: 15_000 })
      .toBe(status);
  }
}

// `test` here gives every spec the Api (with its log filed as evidence) and skips outside `make rehearse-test`.
export const test = base.extend<{ api: Api }>({
  api: async ({ request }, use, testInfo) => {
    testInfo.skip(!configured, "needs the stack and guesthouse that `make rehearse-test` creates");
    const api = new Api(request);
    // eslint-disable-next-line react-hooks/rules-of-hooks -- Playwright's fixture callback, not a React hook
    await use(api);
    if (api.log.length)
      await testInfo.attach("api.log", { body: api.log.join("\n"), contentType: "text/plain" });
  },
});
export { expect };

/** Gives a page the person's session, the way the browser has it after sign-in (HttpOnly cookie, no token in the page). */
export async function uiLogin(page: Page, who: Who) {
  await page.context().addCookies([{ name: "sg_session", value: who.token, url: cfg.base }]);
}
export const vnd = (text: string) => Number(text.replace(/[^\d]/g, ""));
export const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

// ---- the rehearse stack, for the cases that restart it, read its logs or move time ----
const compose = (args: string[], input?: string) =>
  execFileSync(
    "docker",
    [
      "compose",
      "-p",
      "stayguard-rehearse",
      "-f",
      "deploy/compose.prod.yaml",
      "-f",
      "deploy/compose.rehearse.yaml",
      "--env-file",
      cfg.envFile,
      "--profile",
      "backup",
      ...args,
    ],
    { cwd: cfg.root, input, encoding: "utf8", maxBuffer: 64 * 1024 * 1024, timeout: 180_000 },
  );
export const stack = {
  stopApi: () => compose(["stop", "api"]),
  startApi: () => compose(["start", "api"]),
  restartApi: () => compose(["restart", "api"]),
  logs: (service = "api") => compose(["logs", "--no-color", service]),
  /** `stayguard jobs run`, once. Output is counts only. */
  jobsOnce: () => compose(["run", "--rm", "-T", "api", "jobs", "run"]),
  async ready() {
    await expect
      .poll(async () => (await fetch(`${cfg.base}/readyz`).catch(() => ({ status: 0 }))).status, {
        timeout: 90_000,
      })
      .toBe(200);
  },
};
/** scripts/rehearsal-backdate.sh: moves the time of one invoice back in the rehearsal database only (it refuses elsewhere). */
export function backdate(kind: "partial" | "unpaid", billCode: string, minutes: number) {
  return execFileSync("scripts/rehearsal-backdate.sh", [kind, billCode, String(minutes)], {
    cwd: cfg.root,
    encoding: "utf8",
    env: { ...process.env, RH_TENANT: cfg.guesthouse },
  }).trim();
}
