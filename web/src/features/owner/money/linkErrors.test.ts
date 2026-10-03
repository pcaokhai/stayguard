import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, describe, expect, test } from "vitest";
import { createApiClient } from "@/lib/api";
import { setLocale } from "@/lib/locale";
import { linkTransfer } from "./hooks";
import { linkErrorMessage } from "./linkErrors";

const server = setupServer();
beforeAll(() => server.listen({ onUnhandledFrame: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

// Built per call: openapi-fetch keeps the fetch that exists when the client is made, and msw patches it in beforeAll.
const client = () => createApiClient("http://localhost");
const answer = (status: number, code: string, traceId?: string) =>
  server.use(
    http.post("*/v1/owner/payment-events/:id/link", () =>
      HttpResponse.json({ type: "about:blank", title: "x", status, code, traceId }, { status }),
    ),
  );
const refused = async (status: number, code: string, traceId?: string) => {
  answer(status, code, traceId);
  const err = await linkTransfer({ eventId: "e1", invoiceId: "i1", key: "k" }, client()).then(
    () => null,
    (e: unknown) => e,
  );
  expect(err, "the link must be refused").not.toBeNull();
  return err;
};

// The server answers 409 for four different reasons; the text must follow the code, not the status.
const CASES: [string, number, string, string, string][] = [
  [
    "amount mismatch",
    409,
    "LINK_AMOUNT_MISMATCH",
    "Số tiền không khớp phiếu này",
    "does not match",
  ],
  ["event already linked", 409, "EVENT_NOT_LINKABLE", "Khoản này đã được gán", "already linked"],
  [
    "event already linked (alias)",
    409,
    "EVENT_ALREADY_LINKED",
    "Khoản này đã được gán",
    "already linked",
  ],
  ["invoice already paid", 409, "INVOICE_NOT_OPEN", "Phiếu này đã thanh toán", "already paid"],
  [
    "invoice already paid (alias)",
    409,
    "INVOICE_ALREADY_PAID",
    "Phiếu này đã thanh toán",
    "already paid",
  ],
  ["not allowed", 403, "ROLE_FORBIDDEN", "không có quyền", "not allowed"],
  ["not found", 404, "NOT_FOUND", "Không tìm thấy", "could not find"],
  ["server error", 500, "INTERNAL", "Máy chủ", "server had a problem"],
];

describe.each([
  ["vi", 3],
  ["en", 4],
] as const)("link errors in %s", (locale, col) => {
  test.each(CASES)("%s", async (_name, status, code, ...texts) => {
    setLocale(locale);
    expect(linkErrorMessage(await refused(status, code))).toContain(texts[col - 3]);
  });

  test("an unknown code shows the generic line with the trace id, whatever the status", async () => {
    setLocale(locale);
    const msg = linkErrorMessage(await refused(409, "SOMETHING_NEW", "trace-123"));
    expect(msg).toContain("trace-123");
    expect(msg).not.toContain("already linked");
    expect(msg).not.toContain("đã được gán");
  });

  test("an unknown code without a trace id still gives a readable line", async () => {
    setLocale(locale);
    expect(linkErrorMessage(await refused(502, "WEIRD"))).toContain("—");
  });
});

test("the same 409 status gives different texts for different codes", async () => {
  setLocale("vi");
  const texts = new Set<string>();
  for (const code of ["LINK_AMOUNT_MISMATCH", "EVENT_NOT_LINKABLE", "INVOICE_NOT_OPEN"])
    texts.add(linkErrorMessage(await refused(409, code)));
  expect(texts.size).toBe(3);
});

test("a network failure says so instead of blaming the transfer", () => {
  setLocale("vi");
  expect(linkErrorMessage(new TypeError("Failed to fetch"))).toContain("Không kết nối được");
});
