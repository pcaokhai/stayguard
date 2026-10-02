// Weeks run Monday to Sunday; dates travel as "YYYY-MM-DD".
const iso = (d: Date) =>
  `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
const parse = (s: string) => new Date(`${s}T12:00:00`);

export function mondayOf(date: string): string {
  const d = parse(date);
  const back = (d.getDay() + 6) % 7;
  d.setDate(d.getDate() - back);
  return iso(d);
}

export function addDays(date: string, n: number): string {
  const d = parse(date);
  d.setDate(d.getDate() + n);
  return iso(d);
}

export const weekDays = (monday: string) => Array.from({ length: 7 }, (_, i) => addDays(monday, i));
export const today = () => iso(new Date());
export const isDate = (s: string | null): s is string => !!s && /^\d{4}-\d\d-\d\d$/.test(s);
// "2026-10-03" -> "03/10"
export const dm = (date: string) => `${date.slice(8)}/${date.slice(5, 7)}`;
