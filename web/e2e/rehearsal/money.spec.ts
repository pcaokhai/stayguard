import { expect, test } from "./helpers";

// Money paths (docs/15 rules 1, 2 and 17a). Payments arrive only as signed SePay webhooks. Amounts come from the API, never computed here.
test.describe.configure({ mode: "serial" });

test("TT-01 exact transfer pays the bill and the room goes to clean", async ({ api }) => {
  const r = await api.as("r1");
  const owner = await api.as("owner");
  const q = await api.toQr(r);
  expect(q.payment.status).toBe("PENDING");
  expect(q.note).toMatch(/^PH\d{4}A\d+/);
  await api.pay(q.note, q.amount);
  await api.untilPayment(r, q.payment.id, "PAID");
  const stay = await api.getStay(r, q.stay.id);
  expect(stay.paymentState).toBe("PAID");
  expect((await api.room(r, q.stay.roomCode)).status).toBe("TO_CLEAN");
  const tx = (await api.transactions(owner)).find((t) => t.billCode === q.note);
  expect(tx, "the transfer shows in transactions").toBeTruthy();
  expect(tx.reconciliation).toBe("MATCHED");
  expect(tx.amount).toBe(q.amount);
});
