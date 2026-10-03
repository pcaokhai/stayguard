// Vietnamese number shape: 9 to 11 digits, optionally written with a leading 0 or +84, spaces, dots or dashes.
export function isVnPhone(input: string): boolean {
  const s = input.replace(/[\s.-]/g, "");
  if (!/^\+?\d+$/.test(s)) return false;
  if (s.startsWith("+") && !s.startsWith("+84")) return false;
  const digits = s.replace(/\D/g, "").length;
  return digits >= 9 && digits <= 11;
}
