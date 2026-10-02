// Room codes a new building or floor will get: prefix + floor + two-digit number (E101, E102 ... E305).
// Display only; the server assigns the real codes.
export function buildingCodes(code: string, floors: number, perFloor: number): string[] {
  return Array.from({ length: floors }, (_, f) =>
    Array.from({ length: perFloor }, (_, n) => `${code}${f + 1}${String(n + 1).padStart(2, "0")}`),
  ).flat();
}

// "A401" + 6 -> A401 ... A406 (counts up the trailing number).
export function codeRange(start: string, count: number): string[] {
  const m = /^(.*?)(\d+)$/.exec(start);
  if (!m) return [];
  return Array.from(
    { length: count },
    (_, i) => `${m[1]}${String(Number(m[2]) + i).padStart(m[2].length, "0")}`,
  );
}

// "A107" to "A109" -> A107, A108, A109 (same prefix, ascending, at most 50).
export function between(from: string, to: string): string[] {
  const a = /^(.*?)(\d+)$/.exec(from);
  const b = /^(.*?)(\d+)$/.exec(to);
  if (!a || !b || a[1] !== b[1]) return [];
  const n = Number(b[2]) - Number(a[2]) + 1;
  return n >= 1 && n <= 50 ? codeRange(from, n) : [];
}

export const summarise = (codes: string[]) =>
  codes.length > 6 ? `${codes[0]}–${codes[codes.length - 1]}` : codes.join(", ");
