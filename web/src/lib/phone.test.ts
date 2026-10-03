import { expect, test } from "vitest";
import { isVnPhone } from "./phone";

test("vietnamese_phone_shapes_FU", () => {
  for (const ok of [
    "0912345678",
    "912345678",
    "+84912345678",
    "0901 234 567",
    "090-123-4567",
    "84912345678".slice(2),
    "02838123456",
  ])
    expect(isVnPhone(ok), ok).toBe(true);
  for (const bad of [
    "12345",
    "0912345",
    "091234567890123",
    "abcdefghij",
    "+85912345678",
    "0912 345 67a",
    "",
  ])
    expect(isVnPhone(bad), bad).toBe(false);
});
