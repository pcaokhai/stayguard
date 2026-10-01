const vnd = new Intl.NumberFormat("vi-VN");

// Shows an amount exactly as the API returned it; never used to compute one.
export const formatVnd = (n: number) => `${vnd.format(n)}đ`;
export const parseVnd = (s: string) => Number(s.replace(/\D/g, "") || 0);
