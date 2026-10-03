import { expect, raiseAlerts, test, uiLogin } from "./helpers";

// Alerts resolve themselves when the issue is fixed, and "needs action" counts only what is still open (docs/15 rules 1 and 17a).
const open = (list: any[]) => list.filter((a) => !a.resolvedAt);

test("AL-01 a partial-payment alert resolves when the rest is paid", async ({ api }) => {
  const r = await api.as("r2");
  const owner = await api.as("owner");
  const q = await api.toQr(r);
  const half = Math.floor(q.amount / 2);
  await api.pay(q.note, half);
  await raiseAlerts("partial", q.note, 20);
  const before = await api.alertsFor(owner, "PAYMENT_PARTIAL", q.note);
  expect(open(before)).toHaveLength(1);
  const again = await api.post(r, `/v1/invoices/${q.invoice.id}/payments`, { method: "TRANSFER" });
  await api.pay(q.note, q.amount - half);
  await api.untilPayment(r, again.body.id, "PAID");
  const after = await api.alertsFor(owner, "PAYMENT_PARTIAL", q.note);
  expect(open(after), "no open partial alert").toHaveLength(0);
  expect(after[0].resolution).toBe("PAID");
  expect(after[0].resolvedAt).toBeTruthy();
});

test("AL-02 an unpaid alert resolves when the bill is paid", async ({ api }) => {
  const r = await api.as("r2");
  const owner = await api.as("owner");
  const q = await api.toQr(r);
  await raiseAlerts("unpaid", q.note, 40);
  expect(open(await api.alertsFor(owner, "PAYMENT_UNPAID", q.note))).toHaveLength(1);
  await api.pay(q.note, q.amount);
  await api.untilPayment(r, q.payment.id, "PAID");
  const after = await api.alertsFor(owner, "PAYMENT_UNPAID", q.note);
  expect(open(after)).toHaveLength(0);
  expect(after[0].resolution).toBe("PAID");
});

test("AL-03 a refund alert resolves when the deposit is given back", async ({ api }) => {
  const r = await api.as("r2");
  const owner = await api.as("owner");
  const stay = await api.checkIn(r, { deposit: 500_000 });
  const invoice = await api.checkout(r, stay.id);
  await raiseAlerts("unpaid", invoice.billCode, 40);
  expect(open(await api.alertsFor(owner, "REFUND_PENDING", invoice.billCode))).toHaveLength(1);
  expect(
    (await api.post(r, `/v1/invoices/${invoice.id}/payments`, { method: "CASH" })).status,
  ).toBe(201);
  const after = await api.alertsFor(owner, "REFUND_PENDING", invoice.billCode);
  expect(open(after)).toHaveLength(0);
  expect(after[0].resolution).toBe("REFUNDED");
});

// The count is drawn before the data arrives, so wait for it to show the value instead of reading it once.
async function counted(
  page: import("@playwright/test").Page,
  path: string,
  label: RegExp,
  want: number,
  why: string,
) {
  await page.goto(path);
  await expect
    .poll(
      async () =>
        Number(
          (
            await page
              .getByText(label)
              .first()
              .innerText()
              .catch(() => "0")
          ).match(/(\d+)\D*$/)![1],
        ),
      { message: why, timeout: 15_000 },
    )
    .toBe(want);
}

test("AL-04 'Cần xử lý' on the owner overview counts only open items", async ({ api, page }) => {
  const r = await api.as("r2");
  const owner = await api.as("owner");
  await uiLogin(page, owner);
  const label = /Cần xử lý ngay · \d+/;
  const c0 = (await api.overview(owner)).attention.length;
  await counted(page, "/vi/owner", label, c0, "the screen and the API agree");
  const stay = await api.checkIn(r, { deposit: 500_000 });
  const invoice = await api.checkout(r, stay.id);
  await raiseAlerts("unpaid", invoice.billCode, 40);
  await counted(page, "/vi/owner", label, c0 + 1, "one more open item");
  await api.post(r, `/v1/invoices/${invoice.id}/payments`, { method: "CASH" });
  await counted(page, "/vi/owner", label, c0, "fixed: back to what it was");
});

test("AL-05 'Cần xử lý' on Transactions counts only transfers still waiting for the owner", async ({
  api,
  page,
}) => {
  const r = await api.as("r2");
  const owner = await api.as("owner");
  await uiLogin(page, owner);
  const label = /Cần xử lý \(\d+\)/;
  const waiting = async () => (await api.transactions(owner, "?filter=NEEDS_ACTION")).length;
  const c0 = await waiting();
  await counted(page, "/vi/owner/transactions", label, c0, "the screen and the API agree");
  const q = await api.toQr(r);
  await api.pay("", q.amount, { content: "no bill code, AL-05" });
  await counted(page, "/vi/owner/transactions", label, c0 + 1, "an unmatched transfer is waiting");
  const tx = (await api.transactions(owner)).find(
    (t) => t.amount === q.amount && t.reconciliation === "UNMATCHED" && !t.billCode,
  );
  expect(
    (
      await api.post(owner, `/v1/owner/payment-events/${tx.paymentEventId}/link`, {
        invoiceId: q.invoice.id,
      })
    ).status,
  ).toBe(200);
  await counted(page, "/vi/owner/transactions", label, c0, "linked: nothing is waiting any more");
});
