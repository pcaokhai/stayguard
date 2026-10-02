import { describe, expect, it } from "vitest";
import { addDays, mondayOf, weekDays } from "./week";

describe("roster weeks", () => {
  it("finds the Monday of any day", () => {
    expect(mondayOf("2026-10-03")).toBe("2026-09-28"); // Saturday
    expect(mondayOf("2026-09-28")).toBe("2026-09-28"); // already Monday
    expect(mondayOf("2026-10-04")).toBe("2026-09-28"); // Sunday belongs to the week before
  });
  it("lists seven days across a month boundary", () => {
    expect(weekDays("2026-09-28")[6]).toBe("2026-10-04");
    expect(addDays("2026-12-31", 1)).toBe("2027-01-01");
  });
});
