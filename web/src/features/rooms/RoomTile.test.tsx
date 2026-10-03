import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, expect, test, vi } from "vitest";
import type { components } from "../../api/generated/schema";
import { setLocale } from "../../lib/locale";
import { RoomTile } from "./RoomTile";

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: () => {} }) }));

type Room = components["schemas"]["Room"];
type Pending = components["schemas"]["PendingPayment"];
const pending = (p: Partial<Pending>): Pending => ({
  paymentId: "pm1",
  total: 80000,
  deposit: 10000,
  received: 10000,
  remaining: 0,
  refundDue: 0,
  createdAt: "2026-10-03T08:00:00Z",
  ...p,
});
const room = (p: Pending): Room => ({
  id: "r1",
  code: "A103",
  buildingId: "b",
  floor: 1,
  unitType: { code: "STD", name: { vi: "Thường", en: "Standard" } },
  status: "OCCUPIED",
  activeStay: {
    id: "s1",
    rentalType: "HOURLY",
    checkInAt: "2026-10-03T06:00:00Z",
    guestName: "G",
    elapsedMinutes: 90,
    runningTotal: 80000,
    pendingPayment: p,
  },
});
const text = (p: Pending) =>
  renderToStaticMarkup(<RoomTile room={room(p)} readOnly={false} />)
    .replace(/<[^>]+>/g, " ")
    .replace(/\s+/g, " ");

afterEach(() => setLocale("vi"));

test("tile_remaining_refund_and_check_texts_vi_FU", () => {
  expect(text(pending({ remaining: 70000 }))).toContain("Chờ thanh toán");
  expect(text(pending({ remaining: 70000 }))).toContain("Còn thiếu 70.000đ");
  expect(text(pending({ refundDue: 20000 }))).toContain("Chờ hoàn 20.000đ");
  const none = text(pending({}));
  expect(none).toContain("Cần kiểm tra phiếu");
  expect(none).not.toMatch(/Còn thiếu\s*0/);
});

test("tile_remaining_refund_and_check_texts_en_FU", () => {
  setLocale("en");
  expect(text(pending({ remaining: 70000 }))).toContain("Still to pay ₫70,000");
  expect(text(pending({ refundDue: 20000 }))).toContain("Refund pending ₫20,000");
  const none = text(pending({}));
  expect(none).toContain("Check the bill");
  expect(none).not.toMatch(/Still to pay\s*₫0/);
});
