import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, expect, test, vi } from "vitest";
import type { components } from "../../api/generated/schema";
import { setLocale } from "../../lib/locale";
import { PaymentState } from "./PayView";
import { payRefetchInterval } from "./hooks";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: () => {}, replace: () => {}, back: () => {} }),
  useSearchParams: () => new URLSearchParams("room=r1"),
}));

type Payment = components["schemas"]["Payment"];
const pay = (p: Partial<Payment>): Payment => ({
  id: "pm1",
  invoiceId: "iv1",
  method: "TRANSFER",
  status: "EXPIRED",
  amount: 140000,
  remaining: 0,
  receivedAmount: null,
  paidAt: null,
  transactionId: null,
  qr: null,
  ...p,
});
const render = (p: Payment) =>
  renderToStaticMarkup(
    <QueryClientProvider client={new QueryClient()}>
      <PaymentState p={p} paid={false} />
    </QueryClientProvider>,
  );

afterEach(() => setLocale("vi"));

test("expired_without_qr_offers_a_new_qr_and_cash_vi_en_FU", () => {
  for (const [loc, newQr, cash] of [
    ["vi", "Tạo mã QR mới", "Thu tiền mặt"],
    ["en", "Make a new QR code", "Take cash"],
  ] as const) {
    setLocale(loc);
    const html = render(pay({}));
    expect(html, `${loc}: new QR`).toContain(newQr);
    expect(html, `${loc}: cash`).toContain(cash);
    expect(html).not.toContain("<img");
  }
});

test("pending_without_qr_is_treated_as_expired_never_blank_FU", () => {
  setLocale("en");
  const html = render(pay({ status: "PENDING", remaining: 140000 }));
  expect(html).toContain("Make a new QR code");
});

test("expired_after_a_partial_transfer_shows_what_arrived_not_the_full_bill_as_due_FU", () => {
  setLocale("en");
  const html = render(pay({ receivedAmount: 100000 }));
  expect(html).toContain("100,000");
  expect(html).not.toContain("140,000");
  expect(html).toContain("Make a new QR code");
});

test("polling_continues_while_expired_so_a_late_transfer_turns_it_paid_FU", () => {
  expect(payRefetchInterval("PENDING")).toBe(3000);
  expect(payRefetchInterval("EXPIRED")).toBe(3000);
  expect(payRefetchInterval("PAID")).toBe(false);
  expect(payRefetchInterval("MISMATCH")).toBe(false);
  expect(payRefetchInterval(undefined)).toBe(false);
});
