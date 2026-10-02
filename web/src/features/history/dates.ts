import { clockLocale } from "../../lib/time";

const p2 = (n: number) => String(n).padStart(2, "0");

// "2026-09-30" <-> local Date at noon (no timezone drift when shifting days).
export const toIso = (d: Date) => `${d.getFullYear()}-${p2(d.getMonth() + 1)}-${p2(d.getDate())}`;
export const fromIso = (iso: string) => new Date(`${iso}T12:00:00`);
export const shiftDay = (iso: string, by: number) => {
  const d = fromIso(iso);
  d.setDate(d.getDate() + by);
  return toIso(d);
};
export const today = () => toIso(new Date());

// "Tue 30 Sep 2026" / "Thứ 3, 30/09/2026"
export const longDate = (iso: string) =>
  new Intl.DateTimeFormat(clockLocale(), {
    weekday: "short",
    day: "2-digit",
    month: clockLocale() === "en-GB" ? "short" : "2-digit",
    year: "numeric",
  }).format(fromIso(iso));
