import type { components } from "@/api/generated/schema";
import { formatVnd } from "@/lib/money";
import { t, tf, type MessageKey } from "@/lib/t";
import { clockOf, formatDayMonth } from "../format";

export type Alert = components["schemas"]["Alert"];

const time = (v?: string) => (v && /^\d{4}-\d\d-\d\dT/.test(v) ? clockOf(v) : (v ?? ""));
const amount = (n?: number | null) => (n == null ? "" : formatVnd(n));
// Label for an enum value, or "" when the message files have none: raw codes never reach the screen.
const labelOf = (prefix: string, value?: string) => {
  const key = `${prefix}.${value}`;
  return value && t(key as MessageKey) !== key ? t(key as MessageKey) : "";
};
const SEVERITY: Record<string, string> = { LOCK_ROOM: "lock", STILL_RENTABLE: "rentable" };
const money = (v?: string) => (v ? formatVnd(Number(v)) : "");

// The API sends a deposit refund the desk still owes as PAYMENT_UNPAID with details.refundDue; it is a
// refund owed to the guest, not an unpaid bill, so it gets its own kind (REFUND_PENDING may arrive as such too).
export const alertKind = (a: Alert): string =>
  (a.kind as string) === "PAYMENT_UNPAID" && a.details?.refundDue
    ? "REFUND_PENDING"
    : (a.kind as string);

// Display text for an alert row, built from the kind-specific details the API stores.
export function alertDetails(a: Alert): string {
  const d = a.details ?? {};
  const reason = d.reason ? tf("alerts.text.reason", { reason: d.reason }) : "";
  switch (alertKind(a)) {
    case "REFUND_PENDING":
      return tf("alerts.text.REFUND_PENDING", { amount: amount(a.amount) || money(d.refundDue) });
    case "SEPAY_UPDATED": {
      const at = d.at && /^\d{4}-\d\d-\d\dT/.test(d.at) ? d.at : a.createdAt;
      return tf("alerts.text.SEPAY_UPDATED", { when: `${formatDayMonth(at)} ${clockOf(at)}` });
    }
    case "PAYMENT_MISMATCH":
      return tf("alerts.text.PAYMENT_MISMATCH", {
        got: amount(a.amount) || money(d.received),
        expected: money(d.expected),
        bill: d.billCode ?? "",
      });
    case "PAYMENT_PARTIAL":
      return tf("alerts.text.PAYMENT_PARTIAL", {
        got: money(d.received),
        left: amount(a.amount),
        bill: d.billCode ?? "",
      });
    case "PAYMENT_UNPAID":
      return tf("alerts.text.PAYMENT_UNPAID", { left: amount(a.amount), bill: d.billCode ?? "" });
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
      return d.note || t("alerts.text.UNUSED_ROOM_REPORT");
    case "DAMAGE_REPORTED":
      return tf("alerts.text.DAMAGE_REPORTED", {
        what: [labelOf("damage", d.category), labelOf("damage", SEVERITY[d.severity ?? ""])]
          .filter(Boolean)
          .join(" · "),
      });
    case "LEAVE_REQUESTED":
      return tf("alerts.text.LEAVE_REQUESTED", {
        kind: labelOf("leave", d.kind).toLowerCase(),
        from: d.from ? formatDayMonth(d.from) : "",
        to: d.to ? formatDayMonth(d.to) : "",
      });
    case "STOCKTAKE_DIFFERENCE":
      return tf("alerts.text.STOCKTAKE_DIFFERENCE", {
        items: d.items ?? "",
        amount: amount(a.amount),
      });
    case "ACCOUNT_LOCKED":
      return tf("alerts.text.ACCOUNT_LOCKED", { until: time(d.lockedUntil) });
    default:
      return ""; // an unknown kind shows no detail rather than raw enum values
  }
}

export const kindLabel = (a: Alert) => t(`alerts.kind.${alertKind(a)}` as MessageKey);
export const isResolved = (a: Alert) => !!a.resolvedAt;

// "Đã giải quyết lúc HH:MM · đã thu tiền": a resolved alert stays in the list, muted, with how it ended.
export function resolvedLine(a: Alert): string | null {
  if (!a.resolvedAt) return null;
  const time = clockOf(a.resolvedAt);
  return a.resolution
    ? tf("alerts.resolvedBy", { time, how: t(`alerts.resolution.${a.resolution}` as MessageKey) })
    : tf("alerts.resolved", { time });
}

export const roomOrShift = (a: Alert) => a.roomCode || a.details?.shiftName || "—";

// Where "Open" goes: a stay-related alert opens the owner's stay timeline, never the front-desk stay page.
export function alertHref(a: Alert): string {
  if (a.stayId) return `/owner/stay?id=${encodeURIComponent(a.stayId)}`;
  if (a.shiftId) return `/owner/shift?id=${encodeURIComponent(a.shiftId)}`;
  if (["PAYMENT_MISMATCH", "UNMATCHED_TRANSFER", "OVERPAID", "PAYMENT_PARTIAL"].includes(a.kind))
    return "/owner/transactions";
  if (a.roomCode) return `/owner/rooms?room=${encodeURIComponent(a.roomCode)}`;
  return "/owner/rooms";
}

export const FILTERS = {
  money: [
    "PAYMENT_MISMATCH",
    "PAYMENT_PARTIAL",
    "PAYMENT_UNPAID",
    "REFUND_PENDING",
    "OVERPAID",
    "UNMATCHED_TRANSFER",
    "CASH_SHORT",
    "CASH_OVER",
  ],
  stay: ["STAY_TIME_EDITED"],
  hk: ["UNUSED_ROOM_REPORT", "DAMAGE_REPORTED"],
} as const;
