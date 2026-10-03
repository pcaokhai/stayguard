import type { components } from "@/api/generated/schema";
import { formatVnd } from "@/lib/money";
import { t, tf } from "@/lib/t";
import { clockOf } from "../format";

export type Tx = components["schemas"]["Transaction"];
export type Tab = "all" | "transfer" | "cash" | "needs";

export const isRefund = (x: Tx) => x.kind === "CASH_REFUND";

// The row time is when the bank money arrived; cash and refunds use `at`.
export const shownTime = (x: Tx) => clockOf(x.receivedAt ?? x.at);

// "gán lúc HH:MM" under the time, only when the transfer was linked later than it arrived.
export function settledNote(x: Tx): string | null {
  if (isRefund(x) || !x.receivedAt || !x.settledAt) return null;
  const when = clockOf(x.settledAt);
  return when === clockOf(x.receivedAt) ? null : tf("money.settledAt", { time: when });
}

// A refund is a negative amount shown with an explicit minus.
export const amountLine = (x: Tx) =>
  x.amount < 0 ? `−${formatVnd(-x.amount)}` : formatVnd(x.amount);

export const methodLine = (x: Tx) =>
  isRefund(x)
    ? t("money.cashRefund")
    : x.method === "TRANSFER"
      ? t("owner.transfer")
      : t("owner.cashShort");

export const needsAction = (x: Tx) =>
  x.reconciliation === "MISMATCH" || x.reconciliation === "UNMATCHED";

export const inTab = (x: Tx, tab: Tab) =>
  tab === "all" ||
  (tab === "transfer" && x.method === "TRANSFER" && !isRefund(x)) ||
  (tab === "cash" && (x.method === "CASH" || isRefund(x))) ||
  (tab === "needs" && needsAction(x));
