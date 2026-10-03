import { readFileSync } from "node:fs";
import { beforeEach, describe, expect, test } from "vitest";
import golden from "../../../../contracts/pricing/golden-cases.json";
import { setLocale } from "@/lib/locale";
import { billLineLabel } from "./labels";

// Every pricing line code the domain can emit (api/internal/domain/pricing: Code* constants), checked against
// the codes the golden pricing cases actually contain so a new code cannot slip in unlabelled.
const KNOWN = [
  "FIRST_HOUR",
  "EXTRA_HOUR",
  "OVERNIGHT",
  "DAILY",
  "EARLY_CHECKIN_HOUR",
  "LATE_CHECKOUT_HOUR",
  "EARLY_CHECKIN_FEE",
  "LATE_CHECKOUT_FEE",
];
const found = new Set<string>();
const walk = (o: unknown) => {
  if (Array.isArray(o)) o.forEach(walk);
  else if (o && typeof o === "object")
    for (const [k, v] of Object.entries(o)) {
      if (k === "code" && typeof v === "string" && /^[A-Z_]+$/.test(v)) found.add(v);
      else walk(v);
    }
};
walk(golden);

describe.each(["vi", "en"] as const)("bill line labels in %s", (locale) => {
  beforeEach(() => setLocale(locale));

  test("the golden cases only use codes we know", () => {
    expect([...found].filter((c) => !KNOWN.includes(c))).toEqual([]);
  });

  test.each(KNOWN)("%s reads as text, never the code or a key", (code) => {
    const text = billLineLabel(code);
    expect(text).not.toContain(code);
    expect(text).not.toMatch(/^bill\./);
    expect(text.length).toBeGreaterThan(2);
  });

  test("labels are distinct, so no line silently borrows another's name", () => {
    expect(new Set(KNOWN.map(billLineLabel)).size).toBe(KNOWN.length);
  });

  test("an unknown code gets a generic line, not the raw code or another code's name", () => {
    const text = billLineLabel("SOMETHING_NEW");
    expect(text).not.toContain("SOMETHING_NEW");
    expect(text).not.toBe(billLineLabel("DAILY"));
    expect(text).not.toMatch(/^bill\./);
  });
});

// Owner stay page, front-desk stay page, checkout and receipt all print lines through the one function.
test.each([
  "../owner/stays/StayView.tsx",
  "CheckedOutSummary.tsx",
  "CheckoutView.tsx",
  "../payment/ReceiptView.tsx",
])("%s labels bill lines with billLineLabel", (file) => {
  const src = readFileSync(new URL(file, import.meta.url), "utf8");
  expect(src).toContain("billLineLabel(");
  expect(src).not.toMatch(/\.line\.\$\{/);
});
