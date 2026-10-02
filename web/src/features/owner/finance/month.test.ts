import { describe, expect, it } from "vitest";
import { addMonths, isMonth, monthLabel } from "./month";

describe("month helpers", () => {
  it("steps across a year boundary", () => {
    expect(addMonths("2026-12", 1)).toBe("2027-01");
    expect(addMonths("2026-01", -1)).toBe("2025-12");
  });
  it("labels and validates", () => {
    expect(monthLabel("2026-09")).toBe("9/2026");
    expect(isMonth("2026-13")).toBe(false);
    expect(isMonth("2026-09")).toBe(true);
  });
});
