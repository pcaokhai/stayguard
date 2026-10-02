import createClient, { type Middleware } from "openapi-fetch";
import type { paths } from "../api/generated/schema";
import { lp } from "./locale";
import { clearSession, loadSession } from "./session";

// One UUID per user action; reuse the same value when that action is retried (docs/10).
export const newIdempotencyKey = () => crypto.randomUUID();
export const idempotencyHeader = (key: string) => ({ "Idempotency-Key": key });

// Adds the bearer token; a 401 clears the session and sends the user back to the role picker.
const auth: Middleware = {
  onRequest({ request }) {
    const token = loadSession()?.accessToken;
    if (token) request.headers.set("Authorization", `Bearer ${token}`);
    return request;
  },
  onResponse({ response }) {
    if (response.status === 401 && typeof window !== "undefined") {
      clearSession();
      window.location.assign(lp("/") + "/");
    }
    return response;
  },
};

// The only door to /v1 (docs/10 §40): a typed client over the generated schema.
export const createApiClient = (baseUrl: string) => {
  const client = createClient<paths>({ baseUrl });
  client.use(auth);
  return client;
};

// Same origin: the API container serves the static export and /v1 (docs/02 §8).
export const api = createApiClient("");
