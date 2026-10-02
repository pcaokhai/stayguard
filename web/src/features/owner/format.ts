import { getLocale } from "@/lib/locale";
import { tf } from "@/lib/t";

const loc = () => (getLocale() === "en" ? "en-GB" : "vi-VN");

// "2026-09-30" or an ISO instant -> "30/09" (vi) or "30 Sep" (en).
export function formatDayMonth(value: string) {
  const d = new Date(value.length === 10 ? `${value}T12:00:00` : value);
  if (getLocale() === "en") return d.toLocaleDateString(loc(), { day: "numeric", month: "short" });
  return `${String(d.getDate()).padStart(2, "0")}/${String(d.getMonth() + 1).padStart(2, "0")}`;
}

// Local calendar day as yyyy-MM-dd (the audit-log range and its grouping).
export const localDay = (d: Date) =>
  `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
export const parseDay = (s: string) => new Date(`${s}T12:00:00`);

// 125 minutes -> "2 giờ 5 phút"; display only.
export function formatDuration(minutes: number) {
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  const parts = [];
  if (h) parts.push(tf("owner.hours", { h }));
  if (m || !h) parts.push(tf("owner.minutes", { m }));
  return parts.join(" ");
}

export const clockOf = (iso: string) =>
  new Date(iso).toLocaleTimeString(loc(), { hour: "2-digit", minute: "2-digit" });
