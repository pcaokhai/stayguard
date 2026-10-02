import { getLocale } from "./locale";

const vi = new Intl.NumberFormat("vi-VN");
const en = new Intl.NumberFormat("en-US");

// Shows an amount exactly as the API returned it; never used to compute one.
export const vndNumber = (n: number) => (getLocale() === "en" ? en : vi).format(n);
// vi: 4.860.000đ, en: ₫4,860,000 (docs/16 boards).
export const formatVnd = (n: number) =>
  getLocale() === "en" ? `₫${vndNumber(n)}` : `${vndNumber(n)}đ`;
export const parseVnd = (s: string) => Number(s.replace(/\D/g, "") || 0);
