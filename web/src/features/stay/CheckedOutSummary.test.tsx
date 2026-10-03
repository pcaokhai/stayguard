import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, expect, test, vi } from "vitest";
import en from "../../../messages/en.json";
import viMsg from "../../../messages/vi.json";
import type { components } from "../../api/generated/schema";
import { setLocale } from "../../lib/locale";
import { CheckedOutSummary } from "./CheckedOutSummary";

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: () => {} }) }));

type Stay = components["schemas"]["Stay"];
const stay = (pendingPayment: Stay["pendingPayment"]): Stay => ({
  id: "s1",
  invoiceId: "iv1",
  billCode: "PH1003A103",
  roomId: "r1",
  roomCode: "A103",
  rentalType: "HOURLY",
  status: "CHECKED_OUT",
  checkInAt: "2026-10-03T06:00:00Z",
  checkOutAt: "2026-10-03T08:35:00Z",
  guestName: "G",
  guestPhone: "0912345678",
  deposit: 10000,
  extras: [],
  pricingVersion: 1,
  pendingPayment,
  quote: {
    asOf: "2026-10-03T08:35:00Z",
    stayAmount: 80000,
    extrasAmount: 0,
    total: 80000,
    depositPaid: 10000,
    balanceDue: 0,
    refundDue: 0,
    capped: false,
    lines: [{ code: "FIRST_HOUR", quantity: 1, unitAmount: 80000, amount: 80000 }],
  },
});
const render = (s: Stay) =>
  renderToStaticMarkup(
    <QueryClientProvider client={new QueryClient()}>
      <CheckedOutSummary stay={s} />
    </QueryClientProvider>,
  );
const pending = (p: Partial<NonNullable<Stay["pendingPayment"]>>) => ({
  paymentId: "pm1",
  total: 80000,
  deposit: 10000,
  received: 10000,
  remaining: 70000,
  refundDue: 0,
  createdAt: "2026-10-03T08:35:00Z",
  ...p,
});
const FORBIDDEN_VI = [
  viMsg.stay.checkout,
  viMsg.stay.addService,
  viMsg.stay.editTime,
  viMsg.stay.moveRoom,
  viMsg.stay.staying,
  viMsg.stay.runningTotal,
  viMsg.stay.balanceDue,
];

afterEach(() => setLocale("vi"));

test("checked_out_stay_is_read_only_paid_unpaid_refund_vi_FU", () => {
  const paid = render(stay(null));
  expect(paid).toContain("Đã trả phòng lúc");
  expect(paid).toContain(viMsg.pay.paid);
  expect(paid).toContain(viMsg.stay.viewReceipt);
  const unpaid = render(stay(pending({})));
  expect(unpaid).toContain("Tiếp tục thanh toán");
  expect(unpaid).toContain("70.000đ");
  const refund = render(stay(pending({ remaining: 0, refundDue: 20000, paymentId: null })));
  expect(refund).toContain("Hoàn lại khách 20.000đ (tiền mặt)");
  for (const html of [paid, unpaid, refund])
    for (const text of FORBIDDEN_VI) expect(html, text).not.toContain(`>${text}<`);
});

test("checked_out_stay_is_read_only_en_FU", () => {
  setLocale("en");
  const html = render(stay(pending({})));
  expect(html).toContain("Checked out at");
  expect(html).toContain("Continue payment");
  for (const text of [
    en.stay.checkout,
    en.stay.addService,
    en.stay.editTime,
    en.stay.moveRoom,
    en.stay.staying,
    en.stay.runningTotal,
  ])
    expect(html, text).not.toContain(`>${text}<`);
});
