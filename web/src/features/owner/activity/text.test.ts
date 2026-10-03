import actions from "../../../../../contracts/audit-actions.json";
import en from "../../../../messages/en.json";
import vi from "../../../../messages/vi.json";
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
    expect(actionText(mk("GUEST_ID_REVEALED"))).toBe(
      pick("Xem số giấy tờ khách", "Revealed guest ID number"),
    );
  });

  test("never shows a raw placeholder for any known template", () => {
    for (const a of ["stay.moved", "shift.closed", "payment.linked"])
      expect(actionText(mk(a))).not.toMatch(/[{}]/);
  });

  test("empty actor shows the system name in the list and the CSV", () => {
    const sys = pick("Hệ thống", "System");
    expect(actorOf(mk("stay.moved", {}, ""))).toBe(sys);
    expect(actorOf(mk("stay.moved"))).toBe("Lan");
    expect(csvLine(mk("stay.checked_out", {}, ""))).toContain(`"${sys}"`);
  });
});

describe("every action in contracts/audit-actions.json", () => {
  const sets = { vi: vi.activity.act, en: en.activity.act } as Record<
    string,
    Record<string, string>
  >;
  const entries = Object.entries(actions) as [string, { details: string[] }][];

  test.each(entries)("%s has vi and en templates using only its details", (code, spec) => {
    for (const [locale, acts] of Object.entries(sets)) {
      const template = acts[code.replace(/\./g, "_")];
      expect(template, `${locale} template for ${code}`).toBeTruthy();
      const used = [...template.matchAll(/\{(\w+)\}/g)].map((m) => m[1]);
      expect(used.filter((k) => !spec.details.includes(k))).toEqual([]);
    }
  });

  test.each(["vi", "en"] as const)("never shows a raw code or placeholder in %s", (locale) => {
    setLocale(locale);
    for (const [code, spec] of entries) {
      const full = Object.fromEntries(spec.details.map((k) => [k, "1000"]));
      for (const details of [{}, full]) {
        const text = actionText(mk(code, details));
        expect(text).not.toContain(code);
        expect(text).not.toMatch(/[{}]/);
      }
    }
  });
});
