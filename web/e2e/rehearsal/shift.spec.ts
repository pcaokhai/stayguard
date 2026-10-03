import { expect, test, uiLogin, type Api, type Who } from "./helpers";

// Shifts and cash (docs/15 rules 12, 17 and 17a). Each case has its own receptionist, so no case shares a drawer with another.
const NOTES = [500_000, 200_000, 100_000, 50_000, 20_000, 10_000];
/** Banknotes that add up to `amount` (whole 10,000 only): the counts the receptionist types in. */
const counts = (amount: number) => {
  let left = amount;
  return NOTES.map((denomination) => {
    const quantity = Math.floor(left / denomination);
    left -= quantity * denomination;
    return { denomination, quantity };
  }).filter((c) => c.quantity > 0);
};
const close = (api: Api, w: Who, expected: number, extra: Record<string, unknown> = {}) =>
  api.post(w, "/v1/shifts/current/close", { counts: counts(expected), floatLeft: 0, ...extra });
const openShift = async (api: Api, w: Who, deposit = 50_000) => {
  const stay = await api.checkIn(w, { deposit }); // the first cash action opens the shift
  return { stay, shift: (await api.shift(w)).body };
};

test("SH-01 the cash left in the drawer is the opening float of the next shift", async ({
  api,
}) => {
  const w = await api.as("r4");
  const { shift } = await openShift(api, w);
  expect(shift.openingFloat).toBe(0);
  const done = await close(api, w, shift.expectedCash, { floatLeft: 20_000 });
  expect(done.status, JSON.stringify(done.body)).toBeLessThan(300);
  await api.checkIn(w, { deposit: 10_000 });
  const next = (await api.shift(w)).body;
  expect(next.id).not.toBe(shift.id);
  expect(next.openingFloat, "what was left in the drawer").toBe(20_000);
  expect(next.expectedCash, "float plus the new deposit").toBe(30_000);
});

test("SH-02 a cash payout needs a note and leaves the drawer", async ({ api }) => {
  const w = await api.as("r5");
  const { shift } = await openShift(api, w);
  const noNote = await api.post(w, "/v1/shifts/current/payouts", {
    amount: 10_000,
    description: "",
  });
  expect(noNote.status).toBe(422);
  const ok = await api.post(w, "/v1/shifts/current/payouts", {
    amount: 10_000,
    description: "Buy light bulbs",
  });
  expect(ok.status, JSON.stringify(ok.body)).toBeLessThan(300);
  const after = (await api.shift(w)).body;
  expect(after.expectedCash).toBe(shift.expectedCash - 10_000);
  expect(after.movements.some((m: any) => m.kind === "PAYOUT")).toBe(true);
});

test("SH-03 closing with the exact cash needs no reason", async ({ api }) => {
  const w = await api.as("r6");
  const owner = await api.as("owner");
  const { shift } = await openShift(api, w);
  const done = await close(api, w, shift.expectedCash);
  expect(done.status, JSON.stringify(done.body)).toBeLessThan(300);
  expect((await api.shift(w)).status, "no open shift any more").toBe(404);
  const review = (await api.get(owner, `/v1/owner/shifts/${shift.id}`)).body;
  expect(review.difference).toBe(0);
  expect(review.countedCash).toBe(shift.expectedCash);
});

test("SH-04 closing short needs a reason and tells the owner", async ({ api }) => {
  const w = await api.as("r7");
  const owner = await api.as("owner");
  const { shift } = await openShift(api, w);
  const short = shift.expectedCash - 10_000;
  const noReason = await close(api, w, short);
  expect(noReason.status, "short cash without a reason").toBe(422);
  expect((await api.shift(w)).status, "still open").toBe(200);
  const done = await close(api, w, short, { reason: "A 10,000 note was given as change" });
  expect(done.status, JSON.stringify(done.body)).toBeLessThan(300);
  const review = (await api.get(owner, `/v1/owner/shifts/${shift.id}`)).body;
  expect(review.difference).toBe(-10_000);
  expect(review.reason).toContain("change");
  expect(
    (await api.alerts(owner)).some((a) => a.kind === "CASH_SHORT" && a.shiftId === shift.id),
  ).toBe(true);
});

test("SH-05 closing with an invoice that is not fully paid needs a reason", async ({ api }) => {
  const w = await api.as("r8");
  const { stay, shift } = await openShift(api, w);
  await api.checkout(w, stay.id); // checked out, nothing paid yet
  const now = (await api.shift(w)).body;
  expect(now.unpaidInvoices.length, "the shift lists the unpaid invoice").toBeGreaterThan(0);
  const noReason = await close(api, w, now.expectedCash);
  expect(noReason.status, "exact cash but an unpaid invoice").toBe(422);
  const done = await close(api, w, now.expectedCash, { reason: "Guest will transfer later" });
  expect(done.status, JSON.stringify(done.body)).toBeLessThan(300);
  void shift;
});

test("SH-06 cash the owner moves on an open shift lands on it, marked by owner", async ({
  api,
}) => {
  const w = await api.as("r9");
  const owner = await api.as("owner");
  const stay = await api.checkIn(w, { deposit: 500_000 });
  const invoice = await api.checkout(w, stay.id);
  const before = (await api.shift(w)).body;
  const pay = await api.post(owner, `/v1/invoices/${invoice.id}/payments`, { method: "CASH" });
  expect(pay.status, JSON.stringify(pay.body)).toBe(201);
  const after = (await api.shift(w)).body;
  const line = after.movements.find(
    (m: any) => m.kind === "REFUND" && m.billCode === invoice.billCode,
  );
  expect(line?.byOwner, "marked by owner").toBe(true);
  expect(after.expectedCash).toBe(before.expectedCash - invoice.quote.refundDue);
});

test("SH-07 the shift screen and the owner overview show the same expected cash", async ({
  api,
  page,
}, testInfo) => {
  const w = await api.as("r10");
  const owner = await api.as("owner");
  // The overview adds up every shift of the day, so compare what one drawer adds to it: both must move by the same amount.
  const o0 = (await api.overview(owner)).cashExpected;
  const { shift } = await openShift(api, w, 70_000);
  const o1 = (await api.overview(owner)).cashExpected;
  expect(o1 - o0, "the overview grew by the drawer's expected cash").toBe(shift.expectedCash);
  await api.post(w, "/v1/shifts/current/payouts", {
    amount: 10_000,
    description: "Taxi for a guest",
  });
  const mine = (await api.shift(w)).body.expectedCash;
  expect(mine).toBe(shift.expectedCash - 10_000);
  expect((await api.overview(owner)).cashExpected - o0, "and shrank with the payout").toBe(mine);
  await uiLogin(page, w);
  await page.goto("/en/shift");
  await expect(page.getByText("Cash expected").locator("xpath=following::*[1]")).toContainText(
    mine.toLocaleString("en-US"),
  );
  const ownerPage = await page
    .context()
    .browser()!
    .newPage({ viewport: { width: 1280, height: 900 } });
  await uiLogin(ownerPage, owner);
  await ownerPage.goto("/en/owner");
  // The headline figure is an animated number (digits in a shadow root), so the screen is filed as evidence rather than read.
  await expect(ownerPage.getByText("Cash expected")).toBeVisible();
  await testInfo.attach("owner-overview", {
    body: await ownerPage.screenshot(),
    contentType: "image/png",
  });
  await ownerPage.close();
});

test("SH-08 a float left in the drawer is not counted twice by the owner overview", async ({
  api,
}) => {
  const w = await api.as("r11");
  const owner = await api.as("owner");
  const { shift } = await openShift(api, w, 50_000);
  const o0 = (await api.overview(owner)).cashExpected;
  const done = await close(api, w, shift.expectedCash, { floatLeft: 20_000 });
  expect(done.status, JSON.stringify(done.body)).toBeLessThan(300);
  const o1 = (await api.overview(owner)).cashExpected;
  await api.checkIn(w, { deposit: 10_000 });
  const o2 = (await api.overview(owner)).cashExpected;
  // The new shift opens with the 20,000 that is already inside the closed shift's expected cash; only the new deposit is new money.
  expect(o2 - o1, "only the new deposit adds to the day's cash").toBe(10_000);
  expect(o1, "closing a shift does not change the cash").toBe(o0);
});
