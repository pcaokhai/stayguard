import type { components } from "@/api/generated/schema";
import { formatVnd } from "@/lib/money";
import { t, tf, type MessageKey } from "@/lib/t";
import { clockOf, formatDayMonth, localDay } from "../format";
import { addDays } from "../range";

export type ClosedShift = components["schemas"]["ClosedShift"];

const NAME: Record<string, MessageKey> = {
  MORNING: "shifts.morning",
  AFTERNOON: "shifts.afternoon",
  NIGHT: "shifts.night",
};
export const shiftName = (s: Pick<ClosedShift, "shift">) =>
  t(NAME[s.shift ?? ""] ?? "shifts.generic");

// "Hôm nay", "Hôm qua" or 30/09, from the day the shift closed.
export function dayLabel(iso: string) {
  const d = localDay(new Date(iso));
  const now = new Date();
  if (d === localDay(now)) return t("shifts.today");
  if (d === localDay(addDays(now, -1))) return t("shifts.yesterday");
  return formatDayMonth(iso);
}

export const timeRange = (s: ClosedShift) => `${clockOf(s.openedAt)}–${clockOf(s.closedAt)}`;

export const differenceText = (n: number) =>
  n < 0
    ? tf("shifts.short", { amount: formatVnd(-n) })
    : n > 0
      ? tf("shifts.over", { amount: formatVnd(n) })
      : t("shifts.ok");

export const differenceTone = (n: number) =>
  n < 0
    ? "border-destructive/30 bg-destructive/10 text-destructive"
    : n > 0
      ? "border-warn-line bg-warn-bg text-warn-ink"
      : "border-ok-line bg-ok-bg text-ok";
