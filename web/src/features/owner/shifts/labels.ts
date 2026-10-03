import type { components } from "@/api/generated/schema";
import { formatVnd } from "@/lib/money";
import { t, tf, type MessageKey } from "@/lib/t";
import { clockOf, formatDayMonth, localDay } from "../format";
import { addDays } from "../range";

export type ClosedShift = components["schemas"]["ClosedShift"];
export type Review = components["schemas"]["ShiftReview"];

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

// Role label from the message files; an unknown role gives "" so a raw code is never shown.
const roleName = (role?: string) => {
  const key = `rooms.role${role}` as MessageKey;
  return role && t(key) !== key ? t(key) : "";
};

// "Người bàn giao" is the person who closed the shift (a shift is closed by its own person), with their role.
export function handoverLine(r: Review): string {
  const name = r.closedByName || r.shift.userName;
  const role = roleName(r.closedByRole);
  return role ? tf("shifts.handoverRole", { name, role }) : tf("shifts.handover", { name });
}

export const floatLine = (r: Review) =>
  r.floatLeft == null
    ? t("shifts.floatNone")
    : tf("shifts.floatLeft", { amount: formatVnd(r.floatLeft) });

const cell = (v: string | number) => `"${String(v).replace(/"/g, '""')}"`;

// One CSV of the review: the figures first, then the handover person and float, then the cash bills.
export function shiftCsv(r: Review): string {
  const name = r.closedByName || r.shift.userName;
  const role = roleName(r.closedByRole);
  const rows: (string | number)[][] = [
    [t("shifts.expected"), r.shift.expectedCash],
    [t("shifts.counted"), r.countedCash],
    [t("shifts.difference"), r.difference],
    [t("shifts.handoverCsv"), name, role],
    [t("shifts.floatCsv"), r.floatLeft ?? ""],
    [],
    [t("shifts.room"), t("shifts.type"), t("shifts.timeCol"), t("shifts.cash")],
    ...r.cashPayments.map((p) => [
      p.roomCode,
      t(`shifts.rental.${p.rentalType}` as MessageKey),
      clockOf(p.at),
      p.amount,
    ]),
  ];
  return rows.map((row) => row.map(cell).join(",")).join("\n");
}
