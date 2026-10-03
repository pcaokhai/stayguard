import { beforeEach, describe, expect, test } from "vitest";
import { AlertKind } from "@/mocks/generated/api";
import { setLocale } from "@/lib/locale";
import { alertDetails, kindLabel, type Alert } from "./text";

// What the API stores per kind (api/internal/app: alerts raised in auth, stock, rosters, maintenance, ...).
const DETAILS: Record<string, Record<string, string>> = {
  ACCOUNT_LOCKED: { lockedUntil: "2026-10-03T11:55:00Z" },
  DAMAGE_REPORTED: { ticket: "BT-12", category: "AIR_CONDITIONER", severity: "LOCK_ROOM" },
  LEAVE_REQUESTED: { from: "2026-10-05", to: "2026-10-06", kind: "SICK", request: "NEW" },
  STOCKTAKE_DIFFERENCE: { stocktake: "sk1", items: "2" },
  UNUSED_ROOM_REPORT: { note: "Giường đã nằm" },
  STAY_TIME_EDITED: { oldTime: "12:50", newTime: "13:10" },
  PAYMENT_MISMATCH: { received: "100000", expected: "150000", billCode: "PH1" },
  SEPAY_UPDATED: { at: "2026-10-03T07:05:00Z" },
};
const RAW = /LOCK_ROOM|STILL_RENTABLE|AIR_CONDITIONER|^SICK$|\bNEW\b|stocktake|sk1|[{}]/;

const mk = (kind: string, details?: Record<string, string>): Alert =>
  ({
    id: "a1",
    kind,
    createdAt: "2026-10-03T07:05:00Z",
    roomCode: "A101",
    amount: 50000,
    details,
  }) as Alert;

describe.each(["vi", "en"] as const)("every alert kind in %s", (locale) => {
  beforeEach(() => setLocale(locale));

  test.each(Object.values(AlertKind))("%s has a label and clean text", (kind) => {
    for (const details of [DETAILS[kind], {}, undefined]) {
      const a = mk(kind, details);
      expect(kindLabel(a)).not.toContain(kind);
      expect(kindLabel(a)).not.toBe(`alerts.kind.${kind}`);
      const text = alertDetails(a);
      expect(text).not.toContain(kind);
      expect(text).not.toMatch(RAW);
      expect(text).not.toMatch(/alerts\.text\./);
    }
  });

  test("a kind with details always says something", () => {
    for (const kind of Object.values(AlertKind))
      if (kind !== "UNUSED_ROOM_REPORT")
        expect(alertDetails(mk(kind, DETAILS[kind])).length, kind).toBeGreaterThan(0);
  });
});
