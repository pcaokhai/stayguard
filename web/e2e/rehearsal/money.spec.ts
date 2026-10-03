import { expect, test, uiLogin } from "./helpers";

// Money paths (docs/15 rules 1, 2 and 17a). Payments arrive only as signed SePay webhooks. Amounts come from the API, never computed here.

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

test("TT-02 transfer without the bill code is unmatched and alerts the owner", async ({ api }) => {
  const owner = await api.as("owner");
  await api.pay("", 12_345, { content: "chuyen tien khong ma" });
  const tx = (await api.transactions(owner)).find(
    (t) => t.amount === 12_345 && t.reconciliation === "UNMATCHED",
  );
  expect(tx, "an UNMATCHED transaction").toBeTruthy();
  expect(tx.paymentEventId, "the owner can link it").toBeTruthy();
  const alert = (await api.alerts(owner)).find(
    (a) => a.kind === "UNMATCHED_TRANSFER" && a.amount === 12_345,
  );
  expect(alert, "UNMATCHED_TRANSFER alert").toBeTruthy();
});

test("TT-03 owner links an unmatched transfer of the exact amount and the bill is paid", async ({
  api,
}) => {
  const r = await api.as("r1");
  const owner = await api.as("owner");
  const q = await api.toQr(r);
  await api.pay("", q.amount, { content: "no bill code here" });
  const tx = (await api.transactions(owner)).find(
    (t) => t.amount === q.amount && t.reconciliation === "UNMATCHED" && !t.billCode,
  );
  expect(tx?.paymentEventId, "the unmatched event").toBeTruthy();
  const link = await api.post(owner, `/v1/owner/payment-events/${tx.paymentEventId}/link`, {
    invoiceId: q.invoice.id,
  });
  expect(link.status, "link").toBe(200);
  await api.untilPayment(r, q.payment.id, "PAID");
  expect((await api.getStay(r, q.stay.id)).paymentState).toBe("PAID");
  expect(JSON.stringify(await api.audit(owner)), "the link is in the activity log").toMatch(
    /link/i,
  );
});

test("TT-04 linking money of a different amount is refused", async ({ api }) => {
  const r = await api.as("r1");
  const owner = await api.as("owner");
  const q = await api.toQr(r);
  await api.pay("", q.amount - 1_000, { content: "short and no code" });
  const tx = (await api.transactions(owner)).find(
    (t) => t.amount === q.amount - 1_000 && t.reconciliation === "UNMATCHED",
  );
  expect(tx?.paymentEventId).toBeTruthy();
  const link = await api.post(owner, `/v1/owner/payment-events/${tx.paymentEventId}/link`, {
    invoiceId: q.invoice.id,
  });
  expect(link.status, "a different amount must not be linked").toBe(409);
  expect(link.body.code).toBe("LINK_AMOUNT_MISMATCH");
  expect((await api.payment(r, q.payment.id)).status).toBe("PENDING");
});

test("TT-05 a transfer that is already linked cannot be linked again", async ({ api }) => {
  const r = await api.as("r1");
  const owner = await api.as("owner");
  const a = await api.toQr(r);
  const b = await api.toQr(r);
  const amount = a.amount;
  expect(b.amount, "both bills are the same").toBe(amount);
  await api.pay("", amount, { content: "once only" });
  const tx = (await api.transactions(owner)).find(
    (t) => t.amount === amount && t.reconciliation === "UNMATCHED" && !t.billCode,
  );
  expect(tx?.paymentEventId).toBeTruthy();
  expect(
    (
      await api.post(owner, `/v1/owner/payment-events/${tx.paymentEventId}/link`, {
        invoiceId: a.invoice.id,
      })
    ).status,
  ).toBe(200);
  const again = await api.post(owner, `/v1/owner/payment-events/${tx.paymentEventId}/link`, {
    invoiceId: b.invoice.id,
  });
  expect(again.status, "second link refused").toBe(409);
  expect((await api.payment(r, b.payment.id)).status).toBe("PENDING");
});

test("TT-06 a receptionist cannot link a transfer", async ({ api }) => {
  const r = await api.as("r1");
  const owner = await api.as("owner");
  const q = await api.toQr(r);
  await api.pay("", q.amount, { content: "front desk tries" });
  const tx = (await api.transactions(owner)).find(
    (t) => t.amount === q.amount && t.reconciliation === "UNMATCHED" && !t.billCode,
  );
  expect(tx?.paymentEventId).toBeTruthy();
  const link = await api.post(r, `/v1/owner/payment-events/${tx.paymentEventId}/link`, {
    invoiceId: q.invoice.id,
  });
  expect(link.status).toBe(403);
  expect((await api.payment(r, q.payment.id)).status).toBe("PENDING");
});

test("TT-07 short transfer, then the remainder QR with the same bill code, top-up, paid", async ({
  api,
}) => {
  const r = await api.as("r1");
  const owner = await api.as("owner");
  const q = await api.toQr(r);
  const first = Math.floor(q.amount / 2);
  await api.pay(q.note, first);
  let p = await api.payment(r, q.payment.id);
  expect(p.status, "short money does not pay the bill").toBe("PENDING");
  expect(p.receivedAmount).toBe(first);
  expect(p.remaining).toBe(q.amount - first);
  const again = await api.post(r, `/v1/invoices/${q.invoice.id}/payments`, { method: "TRANSFER" });
  expect(again.status, "remainder QR").toBeLessThan(300);
  expect(again.body.qr.transferNote, "the same bill code").toBe(q.note);
  expect(again.body.qr.amount, "for the remainder only").toBe(q.amount - first);
  await api.pay(q.note, q.amount - first);
  await api.untilPayment(r, again.body.id, "PAID");
  expect((await api.getStay(r, q.stay.id)).paymentState).toBe("PAID");
  const mine = (await api.transactions(owner)).filter(
    (t) => t.billCode === q.note && t.method === "TRANSFER",
  );
  expect(mine.reduce((s, t) => s + t.amount, 0)).toBe(q.amount);
});

test("TT-08 overpaid transfer pays the bill and alerts the owner", async ({ api }) => {
  const r = await api.as("r1");
  const owner = await api.as("owner");
  const q = await api.toQr(r);
  await api.pay(q.note, q.amount + 5_000);
  await api.untilPayment(r, q.payment.id, "PAID");
  const alert = (await api.alerts(owner)).find(
    (a) => a.kind === "OVERPAID" && a.details?.billCode === q.note,
  );
  expect(alert, "OVERPAID alert for this bill").toBeTruthy();
});

test("TT-09 an outgoing transfer is ignored", async ({ api }) => {
  const r = await api.as("r1");
  const q = await api.toQr(r);
  const out = await api.deliver({ note: q.note, amount: q.amount, type: "out" });
  expect(out.status).toBe(200);
  expect((await api.payment(r, q.payment.id)).status, "money going out pays nothing").toBe(
    "PENDING",
  );
  expect((await api.getStay(r, q.stay.id)).paymentState).toBe("AWAITING_PAYMENT");
  // Money that left the account must not turn into money the owner can link to a bill.
  const owner = await api.as("owner");
  const row = (await api.transactions(owner)).find(
    (t) => t.transferNote?.includes(q.note) && t.paymentEventId,
  );
  if (row) {
    const link = await api.post(owner, `/v1/owner/payment-events/${row.paymentEventId}/link`, {
      invoiceId: q.invoice.id,
    });
    expect(link.status, "an outgoing transfer cannot be linked to a bill").toBeGreaterThanOrEqual(
      400,
    );
    expect((await api.payment(r, q.payment.id)).status).toBe("PENDING");
  }
});

test("TT-10 the same delivery twice is counted once", async ({ api }) => {
  const r = await api.as("r1");
  const owner = await api.as("owner");
  const q = await api.toQr(r);
  const first = await api.pay(q.note, q.amount);
  const second = await api.deliver({ note: q.note, amount: q.amount, id: first.id });
  expect(second.status, "SePay retries get the same answer").toBe(200);
  expect(second.body).toEqual({ success: true });
  await api.untilPayment(r, q.payment.id, "PAID");
  const mine = (await api.transactions(owner)).filter(
    (t) => t.billCode === q.note && t.method === "TRANSFER",
  );
  expect(mine, "one transaction").toHaveLength(1);
  expect(
    (await api.alerts(owner)).filter(
      (a) => a.kind === "OVERPAID" && a.details?.billCode === q.note,
    ),
  ).toHaveLength(0);
});

test("TT-11 deposit larger than the bill is refunded in cash and lands on the shift", async ({
  api,
}) => {
  const r = await api.as("r2"); // own drawer: nobody else's cash is on this shift
  const stay = await api.checkIn(r, { deposit: 500_000 });
  const invoice = await api.checkout(r, stay.id);
  const refund = invoice.quote.refundDue;
  expect(refund, "the deposit is more than the bill").toBeGreaterThan(0);
  expect((await api.getStay(r, stay.id)).paymentState).toBe("REFUND_PENDING");
  const before = (await api.shift(r)).body;
  const pay = await api.post(r, `/v1/invoices/${invoice.id}/payments`, { method: "CASH" });
  expect(pay.status).toBe(201);
  expect(pay.body.status).toBe("PAID");
  const after = (await api.shift(r)).body;
  expect(after.expectedCash, "the refund leaves the drawer").toBe(before.expectedCash - refund);
  const line = after.movements.find(
    (x: any) => x.kind === "REFUND" && x.billCode === invoice.billCode,
  );
  expect(line, "a REFUND line on the shift").toBeTruthy();
  expect(Math.abs(line.amount)).toBe(refund);
  expect((await api.getStay(r, stay.id)).paymentState).toBe("PAID");
});

test("TT-12 leave the payment screen, reload, resume with the same bill code and no second payment", async ({
  api,
  page,
}) => {
  const r = await api.as("r3");
  const q = await api.toQr(r);
  await uiLogin(page, r);
  await page.goto(`/en/pay?payment=${q.payment.id}`);
  await expect(page.getByText("Transfer note")).toBeVisible();
  await expect(page.locator("body")).toContainText(q.note);
  await page.goto("/en/rooms"); // leave the payment screen
  await page.goto(`/en/stay?id=${q.stay.id}`);
  await page.getByRole("button", { name: "Continue payment" }).click();
  await expect(page).toHaveURL(new RegExp(`/en/pay\\?payment=${q.payment.id}`));
  await page.reload();
  await expect(page.locator("body")).toContainText(q.note);
  await expect(page).toHaveURL(new RegExp(q.payment.id));
  const stay = await api.getStay(r, q.stay.id);
  expect(stay.pendingPayment.paymentId, "still the one payment").toBe(q.payment.id);
  await api.pay(q.note, q.amount);
  await expect(page).toHaveURL(/\/en\/paid\?payment=/, { timeout: 20_000 });
});

test("TT-13 the reopened checkout shows the real deposit", async ({ api, page }) => {
  const r = await api.as("r3");
  const stay = await api.checkIn(r, { deposit: 250_000 });
  await api.checkout(r, stay.id);
  await uiLogin(page, r);
  await page.goto(`/en/checkout?stay=${stay.id}`);
  await expect(page.locator("body")).toContainText("250,000");
  await expect(page.locator("body")).not.toContainText("100,000");
  expect((await api.getStay(r, stay.id)).quote.depositPaid).toBe(250_000);
});

test("TT-14 a finished stay is read-only in the app and in the API", async ({ api, page }) => {
  const r = await api.as("r3");
  const q = await api.toQr(r);
  await api.pay(q.note, q.amount);
  await api.untilPayment(r, q.payment.id, "PAID");
  const id = q.stay.id;
  const other = await api.vacantRoom(r);
  const edits = [
    api.post(r, `/v1/stays/${id}/extras`, { items: [{ serviceCode: "WATER", quantity: 1 }] }),
    api.post(r, `/v1/stays/${id}/checkout`),
    api.post(r, `/v1/stays/${id}/check-in-time`, {
      newCheckInAt: new Date().toISOString(),
      reasonCode: "OTHER",
      note: "late fix",
    }),
    api.post(r, `/v1/stays/${id}/move`, { toRoomId: other.id, rentalType: "HOURLY" }),
  ];
  for (const res of await Promise.all(edits))
    expect(res.status, JSON.stringify(res.body)).toBe(409);
  await uiLogin(page, r);
  await page.goto(`/en/stay?id=${id}`);
  await expect(page.locator("body")).toContainText("Checked out");
  await expect(page.getByRole("link", { name: "Check out" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "+ Add extras" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Continue payment" })).toHaveCount(0);
});

test("TT-15 transactions show the bank time and a negative cash refund line", async ({ api }) => {
  const r = await api.as("r3");
  const owner = await api.as("owner");
  const q = await api.toQr(r);
  await api.pay(q.note, q.amount);
  const stay = await api.checkIn(r, { deposit: 500_000 });
  const invoice = await api.checkout(r, stay.id);
  await api.post(r, `/v1/invoices/${invoice.id}/payments`, { method: "CASH" });
  const rows = await api.transactions(owner);
  const refund = rows.find((t) => t.billCode === invoice.billCode && t.kind === "CASH_REFUND");
  expect(refund, "a CASH_REFUND line").toBeTruthy();
  expect(refund.amount, "negative").toBe(-invoice.quote.refundDue);
  expect(refund.settledAt).toBeTruthy();
  const bank = rows.find((t) => t.billCode === q.note && t.method === "TRANSFER");
  expect(bank.receivedAt, "the time the bank reported the money").toBeTruthy();
  expect(bank.settledAt).toBeTruthy();
});
