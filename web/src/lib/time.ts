import { getLocale } from "./locale";

// 24-hour clock in both languages ("14:32").
export const clockLocale = () => (getLocale() === "en" ? "en-GB" : "vi-VN");
export const formatClock = (iso: string) =>
  new Date(iso).toLocaleTimeString(clockLocale(), { hour: "2-digit", minute: "2-digit" });

// Both instants come from the server (stay.checkInAt, quote.asOf), never the browser clock.
export const minutesBetween = (fromIso: string, toIso: string) =>
  Math.max(0, Math.floor((Date.parse(toIso) - Date.parse(fromIso)) / 60_000));
