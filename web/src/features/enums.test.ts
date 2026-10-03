import { describe, expect, test } from "vitest";
import * as api from "@/mocks/generated/api";
import en from "../../messages/en.json";
import vi from "../../messages/vi.json";
import { STATUS } from "./rooms/status";
import { ApiProblem, messageFor } from "./problem/problem";
import { setLocale } from "@/lib/locale";

const lookup = (m: unknown, key: string) =>
  key.split(".").reduce<unknown>((o, k) => (o as Record<string, unknown>)?.[k], m);

// Enum (from the generated client) -> message key prefix the UI builds with `${prefix}.${value}`.
const ENUMS: [string, Record<string, string>, string][] = [
  ["StayListItemState", api.StayListItemState, "ownerStays.state"],
  ["TicketStatus", api.TicketStatus, "maint.st"],
  ["LeaveStatus", api.LeaveStatus, "leave"],
  ["LeaveKind", api.LeaveKind, "leave"],
  ["StaffStatus", api.StaffStatus, "staff.status"],
  ["DamageCategory", api.DamageCategory, "damage"],
  ["AlertResolution", api.AlertResolution, "alerts.resolution"],
  ["TransactionReconciliation", api.TransactionReconciliation, "money"],
  ["ExpenseCategory", api.ExpenseCategory, "expense.cat"],
  ["ExpenseSource", api.ExpenseSource, "expense.src"],
  ["RentalType", api.RentalType, "report.rental"],
  ["PaymentMethod", api.PaymentMethod, "report.method"],
];

describe.each(ENUMS)("%s", (_name, values, prefix) => {
  test.each(Object.values(values))("%s has vi and en text, not the raw code", (v) => {
    for (const messages of [vi, en]) {
      const text = lookup(messages, `${prefix}.${v}`);
      expect(text, `${prefix}.${v}`).toBeTypeOf("string");
      if (v !== "TV") expect(text).not.toBe(v); // "TV" reads the same in both languages
    }
  });
});

describe("RoomStatus", () => {
  test.each(Object.values(api.RoomStatus))("%s has a label in vi and en", (s) => {
    const key = STATUS[s as keyof typeof STATUS].label;
    for (const messages of [vi, en]) expect(lookup(messages, key), key).toBeTypeOf("string");
  });
});

describe("problem codes", () => {
  test.each(["vi", "en"] as const)("an unknown code never shows in %s", (locale) => {
    setLocale(locale);
    const text = messageFor(new ApiProblem("op", 409, "SOME_NEW_CODE", "tr1"));
    expect(text).not.toContain("SOME_NEW_CODE");
    expect(text).toContain("tr1");
  });
  test("every code the screens map has a message", () => {
    setLocale("vi");
    for (const code of [
      "ROLE_FORBIDDEN",
      "NOT_FOUND",
      "INTERNAL",
      "RATE_LIMITED",
      "IDEMPOTENCY_KEY_REUSED",
    ])
      expect(messageFor(new ApiProblem("op", 400, code)), code).not.toMatch(/problem\.|[A-Z_]{6,}/);
  });
});
