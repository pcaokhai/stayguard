import type { components } from "../api/generated/schema";

export type Session = components["schemas"]["Session"];

const KEY = "stayguard.session";
const hasStorage = () => typeof window !== "undefined";

export function saveSession(s: Session): void {
  if (hasStorage()) window.sessionStorage.setItem(KEY, JSON.stringify(s));
}

export function loadSession(): Session | null {
  if (!hasStorage()) return null;
  const raw = window.sessionStorage.getItem(KEY);
  return raw ? (JSON.parse(raw) as Session) : null;
}

export function clearSession(): void {
  if (hasStorage()) window.sessionStorage.removeItem(KEY);
}
