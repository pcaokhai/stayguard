// Months travel as "YYYY-MM" (the API's month format); these helpers only step and label them.
export const thisMonth = () => {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
};

export function addMonths(month: string, n: number): string {
  const [y, m] = month.split("-").map(Number);
  const d = new Date(y, m - 1 + n, 1);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
}

// "2026-09" -> "9/2026"; the label never needs a locale.
export const monthLabel = (month: string) => `${Number(month.slice(5))}/${month.slice(0, 4)}`;
export const monthNumber = (month: string) => Number(month.slice(5));
export const isMonth = (s: string | null): s is string => !!s && /^\d{4}-(0[1-9]|1[0-2])$/.test(s);
