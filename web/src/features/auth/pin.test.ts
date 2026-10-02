import { expect, test } from "vitest";
import { isSixDigits, isWeakPin } from "./pin";

test("weak_pin_run_and_repeat_L-W1", () => {
  for (const pin of ["123456", "654321", "111111", "000000"]) expect(isWeakPin(pin)).toBe(true);
  for (const pin of ["482915", "123457", "121212"]) expect(isWeakPin(pin)).toBe(false);
});

test("six_digits_only_L-W1", () => {
  expect(isSixDigits("482915")).toBe(true);
  for (const pin of ["48291", "4829150", "48291a", ""]) expect(isSixDigits(pin)).toBe(false);
});
