export const formatClock = (iso: string) =>
  new Date(iso).toLocaleTimeString("vi-VN", { hour: "2-digit", minute: "2-digit" });

// Both instants come from the server (stay.checkInAt, quote.asOf), never the browser clock.
export const minutesBetween = (fromIso: string, toIso: string) =>
  Math.max(0, Math.floor((Date.parse(toIso) - Date.parse(fromIso)) / 60_000));
