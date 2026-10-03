import { beforeEach, describe, expect, test } from "vitest";
import { setLocale } from "@/lib/locale";
import { actionText, actorOf, csvLine, type AuditEntry } from "./text";

const mk = (action: string, details: Record<string, string> = {}, actorName = "Lan"): AuditEntry =>
  ({ at: "2026-10-03T07:05:00Z", actorName, category: "STAY_TIME", action, details }) as AuditEntry;

describe.each(["vi", "en"] as const)("activity text in %s", (locale) => {
  beforeEach(() => setLocale(locale));
  const pick = (vi: string, en: string) => (locale === "vi" ? vi : en);

  test("fills placeholders", () => {
    expect(actionText(mk("stay.checked_out", { room: "A101" }))).toBe(
      pick("Trả phòng A101", "Checked out A101"),
    );
  });

  test("drops an unresolved placeholder with its separator", () => {
    expect(actionText(mk("stay.checked_out"))).toBe(pick("Trả phòng", "Checked out"));
    expect(actionText(mk("stay.check_in_edited", { room: "A101" }))).toBe(
      pick("Sửa giờ vào A101", "Edited A101 check-in"),
    );
    expect(actionText(mk("stay.moved", { from: "A101" }))).toBe(
      pick("Chuyển phòng A101", "Moved A101"),
    );
    expect(actionText(mk("rate.updated"))).not.toMatch(/[{}]/);
  });

  test("never shows a raw placeholder for any known template", () => {
    for (const a of ["stay.moved", "shift.closed", "staff.access_changed", "transfer.linked"])
      expect(actionText(mk(a))).not.toMatch(/[{}]/);
  });

  test("empty actor shows the system name in the list and the CSV", () => {
    const sys = pick("Hệ thống", "System");
    expect(actorOf(mk("stay.moved", {}, ""))).toBe(sys);
    expect(actorOf(mk("stay.moved"))).toBe("Lan");
    expect(csvLine(mk("stay.checked_out", {}, ""))).toContain(`"${sys}"`);
  });
});
