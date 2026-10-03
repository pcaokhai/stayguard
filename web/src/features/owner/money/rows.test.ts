import { beforeEach, describe, expect, test } from "vitest";
import { setLocale } from "@/lib/locale";
import { amountLine, inTab, methodLine, settledNote, shownTime, type Tx } from "./rows";

// Times are built from local clock parts so the test does not depend on the machine's time zone.
const at = (h: number, m: number) => new Date(2026, 9, 3, h, m).toISOString();
const tx = (extra: Partial<Tx>): Tx =>
  ({
    id: "t1",
    at: at(14, 1),
    amount: 30000,
    method: "TRANSFER",
    reconciliation: "MATCHED",
    kind: "PAYMENT",
    ...extra,
  }) as Tx;

describe.each(["vi", "en"] as const)("transaction rows in %s", (locale) => {
  beforeEach(() => setLocale(locale));
  const pick = (vi: string, en: string) => (locale === "vi" ? vi : en);

  test("the row time is when the bank money arrived", () => {
    expect(shownTime(tx({ receivedAt: at(13, 20), settledAt: at(14, 5) }))).toBe("13:20");
    expect(shownTime(tx({ receivedAt: null, at: at(9, 5) }))).toBe("09:05");
  });

  test('"gán lúc" appears only when the settle time differs from the bank time', () => {
    expect(settledNote(tx({ receivedAt: at(13, 20), settledAt: at(14, 5) }))).toBe(
      pick("gán lúc 14:05", "linked at 14:05"),
    );
    expect(settledNote(tx({ receivedAt: at(13, 20), settledAt: at(13, 20) }))).toBeNull();
    expect(settledNote(tx({ receivedAt: at(13, 20), settledAt: null }))).toBeNull();
    expect(settledNote(tx({ method: "CASH", receivedAt: null, settledAt: at(9, 5) }))).toBeNull();
  });

  test("a cash refund is a negative line, never a plain cash amount", () => {
    const refund = tx({
      method: "CASH",
      kind: "CASH_REFUND",
      amount: -20000,
      reconciliation: "CASH",
    });
    expect(amountLine(refund)).toBe(pick("−20.000đ", "−₫20,000"));
    expect(methodLine(refund)).toBe(pick("Hoàn tiền mặt", "Cash refund"));
    expect(amountLine(refund)).not.toBe(pick("0đ", "₫0"));
    expect(amountLine(tx({ amount: 30000 }))).toBe(pick("30.000đ", "₫30,000"));
    expect(methodLine(tx({ method: "CASH", reconciliation: "CASH" }))).toBe(
      pick("Tiền mặt", "Cash"),
    );
  });

  test("a cash refund counts under the Cash tab, not Transfer or Needs action", () => {
    const refund = tx({
      method: "CASH",
      kind: "CASH_REFUND",
      amount: -20000,
      reconciliation: "CASH",
    });
    expect(inTab(refund, "cash")).toBe(true);
    expect(inTab(refund, "transfer")).toBe(false);
    expect(inTab(refund, "needs")).toBe(false);
    expect(inTab(tx({ reconciliation: "UNMATCHED" }), "needs")).toBe(true);
  });
});
