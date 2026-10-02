// Client-side mirror of the server PIN rules, so the checklist can tick live (the server decides).
export const PIN_LENGTH = 6;

export const isSixDigits = (pin: string) => /^[0-9]{6}$/.test(pin);

// Repeated digit (111111) or a run up or down (123456, 654321).
export function isWeakPin(pin: string): boolean {
  if (!isSixDigits(pin)) return false;
  const d = [...pin].map(Number);
  const step = d[1] - d[0];
  return Math.abs(step) <= 1 && d.every((x, i) => i === 0 || x - d[i - 1] === step);
}
