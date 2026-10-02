import type { components } from "@/api/generated/schema";
import { formatVnd } from "@/lib/money";
import { t, tf, type MessageKey } from "@/lib/t";
import { clockOf } from "../format";

export type Alert = components["schemas"]["Alert"];

const time = (v?: string) => (v && /^\d{4}-\d\d-\d\dT/.test(v) ? clockOf(v) : (v ?? ""));
const amount = (n?: number | null) => (n == null ? "" : formatVnd(n));
const money = (v?: string) => (v ? formatVnd(Number(v)) : "");

// Display text for an alert row, built from the kind-specific details the API stores.
export function alertDetails(a: Alert): string {
  const d = a.details ?? {};
  const reason = d.reason ? tf("alerts.text.reason", { reason: d.reason }) : "";
  switch (a.kind) {
    case "PAYMENT_MISMATCH":
      return tf("alerts.text.PAYMENT_MISMATCH", {
        got: amount(a.amount) || money(d.received),
        expected: money(d.expected),
        bill: d.billCode ?? "",
      });
    case "OVERPAID":
      return tf("alerts.text.OVERPAID", { got: amount(a.amount), bill: d.billCode ?? "" });
    case "UNMATCHED_TRANSFER":
      return tf("alerts.text.UNMATCHED_TRANSFER", {
        got: amount(a.amount),
        note: d.transferNote ?? "",
      });
    case "STAY_TIME_EDITED":
      return (
        tf("alerts.text.STAY_TIME_EDITED", { old: time(d.oldTime), new: time(d.newTime) }) + reason
      );
    case "CASH_SHORT":
    case "CASH_OVER":
      return tf(`alerts.text.${a.kind}` as MessageKey, { got: amount(a.amount) }) + reason;
    case "UNUSED_ROOM_REPORT":
      return d.note ?? "";
    case "ACCOUNT_LOCKED":
      return tf("alerts.text.ACCOUNT_LOCKED", { until: time(d.lockedUntil) });
    default:
      return Object.values(d).join(" · ");
  }
}

export const kindLabel = (k: Alert["kind"]) => t(`alerts.kind.${k}` as MessageKey);
export const roomOrShift = (a: Alert) => a.roomCode || a.details?.shiftName || "—";

// Where "Open" goes. The money and shift pages arrive with L-W7.
export function alertHref(a: Alert): string {
  if (a.stayId) return `/stay?id=${encodeURIComponent(a.stayId)}`;
  if (a.shiftId) return `/owner/shift?id=${encodeURIComponent(a.shiftId)}`;
  if (a.kind === "PAYMENT_MISMATCH" || a.kind === "UNMATCHED_TRANSFER" || a.kind === "OVERPAID")
    return "/owner/transactions";
  if (a.roomCode) return `/owner/rooms?room=${encodeURIComponent(a.roomCode)}`;
  return "/owner/rooms";
}

export const FILTERS = {
  money: ["PAYMENT_MISMATCH", "OVERPAID", "UNMATCHED_TRANSFER", "CASH_SHORT", "CASH_OVER"],
  stay: ["STAY_TIME_EDITED"],
  hk: ["UNUSED_ROOM_REPORT", "DAMAGE_REPORTED"],
} as const;
