const vnd = new Intl.NumberFormat("vi-VN");

// Shows an amount exactly as the API returned it; never used to compute one.
export const vndNumber = (n: number) => vnd.format(n);
export const formatVnd = (n: number) => `${vndNumber(n)}đ`;
export const parseVnd = (s: string) => Number(s.replace(/\D/g, "") || 0);
