import { beforeEach, describe, expect, test } from "vitest";
import { setLocale } from "@/lib/locale";
import { floatLine, handoverLine, shiftCsv, type Review } from "./labels";

const review = (extra: Partial<Review> = {}): Review =>
  ({
    shift: {
      id: "s1",
      userName: "Lan",
      openedAt: "2026-10-03T01:00:00Z",
      closedAt: "2026-10-03T08:00:00Z",
      expectedCash: 500000,
      cashIn: 300000,
    },
    countedCash: 450000,
    difference: -50000,
    cashPayments: [
      { roomCode: "A101", rentalType: "HOURLY", at: "2026-10-03T02:00:00Z", amount: 80000 },
    ],
    staffHistory: { shiftsWithDifference: 1, totalShort: 50000 },
    closedByName: "Lan",
    closedByRole: "RECEPTIONIST",
    floatLeft: 200000,
    ...extra,
  }) as Review;

describe.each(["vi", "en"] as const)("shift handover in %s", (locale) => {
  beforeEach(() => setLocale(locale));
  const pick = (vi: string, en: string) => (locale === "vi" ? vi : en);

  test("names who closed the shift with their role", () => {
    expect(handoverLine(review())).toBe(
      pick("Người bàn giao: Lan (Lễ tân)", "Closed by (handover): Lan (Front desk)"),
    );
  });
  test("an unknown role is left out rather than printed raw", () => {
    const line = handoverLine(review({ closedByRole: "SOMETHING" }));
    expect(line).toBe(pick("Người bàn giao: Lan", "Closed by (handover): Lan"));
  });
  test("an older shift without the fields falls back to the shift's person", () => {
    expect(handoverLine(review({ closedByName: undefined, closedByRole: undefined }))).toContain(
      "Lan",
    );
  });
  test("the float left is shown as money, or as not recorded when null", () => {
    expect(floatLine(review())).toBe(pick("Để lại két: 200.000đ", "Left in the drawer: ₫200,000"));
    expect(floatLine(review({ floatLeft: null }))).toBe(
      pick("Để lại két: không ghi", "Left in the drawer: not recorded"),
    );
  });
  test("the csv carries the handover person, role and float", () => {
    const csv = shiftCsv(review());
    expect(csv).toContain(
      pick('"Người bàn giao","Lan","Lễ tân"', '"Closed by (handover)","Lan","Front desk"'),
    );
    expect(csv).toContain(pick('"Để lại két","200000"', '"Left in the drawer","200000"'));
    expect(csv).not.toMatch(/[{}]|closedBy|RECEPTIONIST/);
  });
});
