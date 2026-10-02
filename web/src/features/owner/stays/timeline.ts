import type { components } from "@/api/generated/schema";
import { formatVnd } from "@/lib/money";
import { t, tf, type MessageKey } from "@/lib/t";
import { clockOf } from "../format";

export type TimelineEvent = components["schemas"]["StayTimelineEvent"];

const RENTAL: Record<string, MessageKey> = {
  HOURLY: "ownerStays.rentalHourly",
  OVERNIGHT: "ownerStays.rentalOvernight",
  DAILY: "ownerStays.rentalDaily",
};
const time = (v?: string) => (v && /^\d{4}-\d\d-\d\dT/.test(v) ? clockOf(v) : (v ?? ""));

// One line per event, from the stable kind plus its details (keys written by the stay-history query).
export function eventText(e: TimelineEvent): string {
  const d = e.details ?? {};
  switch (e.kind) {
    case "CHECKED_IN":
      return tf("ownerStays.ev.CHECKED_IN", {
        rental: d.rentalType ? t(RENTAL[d.rentalType] ?? "ownerStays.rentalHourly") : "",
      }).replace(/ · $/, "");
    case "CHECK_IN_EDITED":
      return tf(d.note ? "ownerStays.ev.CHECK_IN_EDITED_NOTE" : "ownerStays.ev.CHECK_IN_EDITED", {
        old: time(d.oldTime),
        new: time(d.newTime),
        note: d.note ?? "",
      });
    case "EXTRAS_ADDED":
      return tf("ownerStays.ev.EXTRAS_ADDED", { qty: d.quantity ?? "1", service: d.service ?? "" });
    case "MOVED":
      return tf("ownerStays.ev.MOVED", { from: d.from ?? "", to: d.to ?? "" });
    case "CHECKED_OUT":
      return tf("ownerStays.ev.CHECKED_OUT", { bill: d.billCode ?? "" });
    case "PAYMENT_RECEIVED":
      return tf("ownerStays.ev.PAYMENT_RECEIVED", {
        amount: formatVnd(Number(d.amount ?? 0)),
        method: d.method === "TRANSFER" ? t("owner.transfer") : t("owner.cashShort"),
      });
    case "PAYMENT_MISMATCH":
      return tf("ownerStays.ev.PAYMENT_MISMATCH", {
        received: formatVnd(Number(d.received ?? 0)),
        expected: formatVnd(Number(d.expected ?? 0)),
      });
    default:
      return t(`ownerStays.ev.${e.kind}` as MessageKey);
  }
}

export const dotTone = (k: TimelineEvent["kind"]) =>
  k === "CHECK_IN_EDITED" || k === "PAYMENT_MISMATCH"
    ? "bg-warn"
    : k === "PAYMENT_RECEIVED" || k === "LINKED_BY_OWNER"
      ? "bg-ok"
      : "bg-muted-foreground/60";
