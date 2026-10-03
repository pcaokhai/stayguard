import createClient, { type Middleware } from "openapi-fetch";
import type { paths } from "../api/generated/schema";
import { lp } from "./locale";
import { clearSession } from "./session";

// One UUID per user action; reuse the same value when that action is retried (docs/10).
export const newIdempotencyKey = () => crypto.randomUUID();
export const idempotencyHeader = (key: string) => ({ "Idempotency-Key": key });

// Cookie sessions: the browser sends the HttpOnly `sg_session` cookie itself (same-origin credentials). A write that
// is authenticated by the cookie must say it comes from this app: `X-Requested-With: stayguard` (the API answers 403
// CSRF_REJECTED otherwise). A 401 clears the tab's hint and sends the person to sign-in, remembering the page.
const UNSAFE = new Set(["POST", "PUT", "PATCH", "DELETE"]);
const auth: Middleware = {
  onRequest({ request }) {
    if (UNSAFE.has(request.method)) request.headers.set("X-Requested-With", "stayguard");
    return request;
  },
  onResponse({ response }) {
    // A wrong PIN is a 401 from sign-in or change-PIN itself; the form shows it, no redirect.
    const ownError = /\/v1\/(auth\/sign-in|me\/pin)$/.test(response.url);
    // Sign-in itself and the bare language root decide for themselves (no loop, no "session ended" on a first visit).
    const here = typeof window === "undefined" ? "" : window.location.pathname.replace(/\/$/, "");
    const exempt = /\/(vi|en)(\/sign-in)?$/.test(here);
    if (response.status === 401 && typeof window !== "undefined" && !ownError && !exempt) {
      clearSession();
      // Only a 401 ends the session (a 403 is "no permission" and keeps it); say why and remember where to return.
      const back = encodeURIComponent(window.location.pathname + window.location.search);
      // eslint-disable-next-line @next/next/no-location-assign-relative-destination -- a full reload also drops every cached query
      window.location.assign(`${lp("/sign-in")}?reason=expired&next=${back}`);
    }
    return response;
  },
};

// The only door to /v1 (docs/10 §40): a typed client over the generated schema.
export const createApiClient = (baseUrl: string) => {
  const client = createClient<paths>({ baseUrl, credentials: "same-origin" });
  client.use(auth);
  return client;
};

// Same origin: the API container serves the static export and /v1 (docs/02 §8).
export const api = createApiClient("");
