import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, expect, test } from "vitest";
import viMsg from "../../../messages/vi.json";
import type { components } from "../../api/generated/schema";
import { setLocale } from "../../lib/locale";
import { ShiftLine } from "./ShiftLine";

type Movement = components["schemas"]["ShiftMovement"];
const mv = (p: Partial<Movement>): Movement => ({
  at: "2026-10-03T08:05:00Z",
  kind: "DEPOSIT",
  amount: 100000,
  byOwner: false,
  roomCode: "A101",
  billCode: null,
  ...p,
});
const text = (html: string) => html.replace(/<[^>]+>/g, " ").replace(/\s+/g, " ");

afterEach(() => setLocale("vi"));

test("shift_line_lists_movements_with_signs_and_owner_tag_vi_FU", () => {
  const html = text(
    renderToStaticMarkup(
      <ShiftLine
        label="Chi trong ca"
        value="-30.000đ"
        defaultOpen
        movements={[
          mv({ kind: "REFUND", amount: -20000, roomCode: "A103" }),
          mv({ kind: "PAYOUT", amount: -10000, roomCode: null, byOwner: true }),
        ]}
      />,
    ),
  );
  expect(html).toContain(viMsg.shift.mv.REFUND);
  expect(html).toContain("A103");
  expect(html).toContain("−20.000đ");
  expect(html).toContain(viMsg.shift.mv.PAYOUT);
  expect(html).toContain(viMsg.shift.byOwner); // only the owner's movement carries the tag
  expect(html.match(new RegExp(viMsg.shift.byOwner, "g"))).toHaveLength(1);
});

test("shift_line_without_movements_is_plain_and_en_labels_FU", () => {
  setLocale("en");
  const plain = text(
    renderToStaticMarkup(<ShiftLine label="Opening float" value="₫0" movements={[]} />),
  );
  expect(plain).toContain("Opening float");
  expect(plain).not.toContain("Show details");
  const open = text(
    renderToStaticMarkup(
      <ShiftLine
        label="Cash taken"
        value="₫1"
        defaultOpen
        movements={[mv({ kind: "DEPOSIT", byOwner: true })]}
      />,
    ),
  );
  expect(open).toContain("Deposit");
  expect(open).toContain("by owner");
  expect(open).toContain("+₫100,000");
});
