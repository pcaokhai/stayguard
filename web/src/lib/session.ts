import type { components } from "../api/generated/schema";

// Browser sessions are the HttpOnly cookie `sg_session` the API sets at sign-in: this app never sees or keeps the
// token. What is stored per tab is only a non-secret hint (who, which tenant) for the demo role switch and mock mode.
export type SessionHint = Pick<components["schemas"]["Session"], "tenantId" | "user">;

const KEY = "stayguard.session";
const hasStorage = () => typeof window !== "undefined";

export function saveSession(s: SessionHint & Record<string, unknown>): void {
  if (hasStorage())
    window.sessionStorage.setItem(KEY, JSON.stringify({ tenantId: s.tenantId, user: s.user }));
}

export function loadSession(): SessionHint | null {
  if (!hasStorage()) return null;
  const raw = window.sessionStorage.getItem(KEY);
  return raw ? (JSON.parse(raw) as SessionHint) : null;
}

export function clearSession(): void {
  if (hasStorage()) window.sessionStorage.removeItem(KEY);
}

// Set only by the demo role picker; the simulator button depends on it (real sign-in never sets it).
const DEMO_KEY = "stayguard.demo";
export const markDemo = () => hasStorage() && window.sessionStorage.setItem(DEMO_KEY, "1");
export const isDemo = () => hasStorage() && window.sessionStorage.getItem(DEMO_KEY) === "1";
