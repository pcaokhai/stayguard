import { beforeEach, describe, expect, test } from "vitest";
import { setLocale } from "@/lib/locale";
import {
  alertDetails,
  alertHref,
  alertKind,
  isResolved,
  kindLabel,
  resolvedLine,
  type Alert,
} from "./text";

const base = { id: "a1", createdAt: "2026-10-03T07:05:00Z", roomCode: "A101", stayId: "st1" };
const mk = (kind: string, extra: Partial<Alert> = {}): Alert =>
  ({ ...base, kind, ...extra }) as Alert;

describe.each(["vi", "en"] as const)("alert text in %s", (locale) => {
  beforeEach(() => setLocale(locale));
  const pick = (vi: string, en: string) => (locale === "vi" ? vi : en);

  test("PAYMENT_UNPAID with a balance reads as money still to collect", () => {
    const a = mk("PAYMENT_UNPAID", {
      amount: 80000,
      details: { billCode: "PH1A", balance: "80000" },
    });
    expect(alertDetails(a)).toContain(pick("Còn 80.000đ chưa thu", "₫80,000 still unpaid"));
    expect(alertKind(a)).toBe("PAYMENT_UNPAID");
  });

  test("PAYMENT_UNPAID that carries a refundDue is a refund owed to the guest, not an unpaid bill", () => {
    const a = mk("PAYMENT_UNPAID", {
      amount: 20000,
      details: { billCode: "PH1A", balance: "0", refundDue: "20000" },
    });
    expect(alertKind(a)).toBe("REFUND_PENDING");
    expect(alertDetails(a)).toContain(
      pick("Chờ hoàn 20.000đ cho khách", "₫20,000 to refund to the guest"),
    );
    expect(alertDetails(a)).not.toContain(pick("chưa thu", "still to collect"));
    expect(kindLabel(a)).toContain(pick("Chờ hoàn", "Refund"));
  });

  test("a REFUND_PENDING kind from the API reads the same way", () => {
    const a = mk("REFUND_PENDING", { amount: 20000 });
    expect(alertDetails(a)).toContain(
      pick("Chờ hoàn 20.000đ cho khách", "₫20,000 to refund to the guest"),
    );
  });

  test("SEPAY_UPDATED shows a sentence, never the raw ISO time", () => {
    const a = mk("SEPAY_UPDATED", {
      roomCode: null,
      stayId: null,
      details: { at: "2026-10-03T07:05:00Z" },
    });
    const text = alertDetails(a);
    expect(text).not.toMatch(/\d{4}-\d\d-\d\dT/);
    expect(text).toContain(
      pick("Người cài đặt đã cập nhật kết nối SePay", "The installer updated the SePay connection"),
    );
  });

  test("OVERPAID and PAYMENT_PARTIAL have their own lines and labels", () => {
    const over = mk("OVERPAID", { amount: 5000, details: { billCode: "PH1A", excess: "5000" } });
    expect(alertDetails(over)).toContain(pick("Thừa 5.000đ", "₫5,000 extra"));
    const part = mk("PAYMENT_PARTIAL", {
      amount: 30000,
      details: { billCode: "PH1A", received: "50000", remaining: "30000" },
    });
    expect(alertDetails(part)).toContain(pick("còn thiếu 30.000đ", "₫30,000 still to pay"));
    expect(kindLabel(over)).not.toContain("alerts.kind");
    expect(kindLabel(part)).not.toContain("alerts.kind");
  });
});

describe("where Xem goes", () => {
  test("a stay-related alert opens the owner's stay timeline, not the front-desk stay page", () => {
    expect(alertHref(mk("STAY_TIME_EDITED"))).toBe("/owner/stay?id=st1");
    expect(alertHref(mk("REFUND_PENDING"))).toBe("/owner/stay?id=st1");
    expect(alertHref(mk("PAYMENT_UNPAID"))).toBe("/owner/stay?id=st1");
  });
  test("alerts without a stay keep their own targets", () => {
    expect(alertHref(mk("CASH_SHORT", { stayId: null, shiftId: "sh1" }))).toBe(
      "/owner/shift?id=sh1",
    );
    expect(alertHref(mk("UNMATCHED_TRANSFER", { stayId: null, roomCode: null }))).toBe(
      "/owner/transactions",
    );
  });
});

describe.each(["vi", "en"] as const)("resolved alerts in %s", (locale) => {
  beforeEach(() => setLocale(locale));
  const pick = (vi: string, en: string) => (locale === "vi" ? vi : en);
  const at = new Date(2026, 9, 3, 14, 5).toISOString();

  test("an unresolved alert has no resolved line", () => {
    expect(resolvedLine(mk("PAYMENT_UNPAID", { resolvedAt: null, resolution: null }))).toBeNull();
    expect(isResolved(mk("PAYMENT_UNPAID", { resolvedAt: null }))).toBe(false);
  });

  test("a resolved alert says when and how", () => {
    const a = mk("PAYMENT_UNPAID", { resolvedAt: at, resolution: "PAID" });
    expect(isResolved(a)).toBe(true);
    expect(resolvedLine(a)).toBe(
      pick("Đã giải quyết lúc 14:05 · đã thu tiền", "Resolved at 14:05 · paid"),
    );
    expect(
      resolvedLine(mk("REFUND_PENDING", { resolvedAt: at, resolution: "REFUNDED" })),
    ).toContain(pick("đã hoàn tiền", "refunded"));
    expect(
      resolvedLine(mk("UNMATCHED_TRANSFER", { resolvedAt: at, resolution: "LINKED" })),
    ).toContain(pick("đã gán phiếu", "linked"));
  });

  test("a resolved alert without a known resolution still shows the time", () => {
    expect(resolvedLine(mk("OVERPAID", { resolvedAt: at, resolution: null }))).toBe(
      pick("Đã giải quyết lúc 14:05", "Resolved at 14:05"),
    );
  });
});
