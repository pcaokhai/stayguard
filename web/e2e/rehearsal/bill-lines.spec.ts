import { backdate, expect, rememberStay, test, type Api, type Who } from "./helpers";

// One stay for each kind of bill line, so ui-lint has real data to read: early check-in, late check-out, extras, the day rate, a refund due.
// Times come from the real clock (Vietnam time, UTC+7), never typed in, so the cases do not depend on when they run.
const HOUR = 3_600_000;
/** The latest moment at or before now whose Vietnam clock reads hh:mm, daysBack days earlier. */
function lastAt(hh: number, mm: number, daysBack = 0, now = Date.now()) {
  const d = new Date(now + 7 * HOUR);
  d.setUTCHours(hh, mm, 0, 0);
  let t = d.getTime() - 7 * HOUR;
  if (t > now) t -= 24 * HOUR;
  return t - daysBack * 24 * HOUR;
}

async function checkedOut(
  api: Api,
  w: Who,
  kind: string,
  o: { rentalType: string; deposit: number; checkInAt?: number; water?: number },
) {
  const stay = await api.checkIn(w, { rentalType: o.rentalType, deposit: o.deposit });
  if (o.checkInAt !== undefined) {
    const minutes = Math.round((Date.parse(stay.checkInAt) - o.checkInAt) / 60_000);
    backdate("checkin", stay.id, minutes);
  }
  if (o.water) await api.addWater(w, stay.id, o.water);
  const invoice = await api.checkout(w, stay.id);
  rememberStay({ kind, stayId: stay.id, invoiceId: invoice.id, user: w.user });
  return { stay, invoice, codes: (invoice.quote.lines as any[]).map((l) => l.code as string) };
}

test("BL-01 an overnight stay that checked in before the window has an early check-in line", async ({
  api,
}) => {
  const r = await checkedOut(api, await api.as("r9"), "early", {
    rentalType: "OVERNIGHT",
    deposit: 0,
    checkInAt: lastAt(19, 30),
  });
  expect(r.codes.join(), JSON.stringify(r.invoice.quote)).toMatch(/EARLY_CHECKIN/);
});

test("BL-02 an overnight stay that stayed past noon has a late check-out line", async ({ api }) => {
  const r = await checkedOut(api, await api.as("r9"), "late", {
    rentalType: "OVERNIGHT",
    deposit: 0,
    checkInAt: lastAt(22, 0, 2),
  });
  expect(r.codes.join(), JSON.stringify(r.invoice.quote)).toMatch(/LATE_CHECKOUT/);
});

test("BL-03 a stay with extras has extras on the bill", async ({ api }) => {
  const r = await checkedOut(api, await api.as("r9"), "extras", {
    rentalType: "HOURLY",
    deposit: 10_000,
    water: 3,
  });
  expect(r.invoice.quote.extrasAmount).toBe(30_000);
});

test("BL-04 a daily stay is billed at the day rate", async ({ api }) => {
  const r = await checkedOut(api, await api.as("r9"), "day", { rentalType: "DAILY", deposit: 0 });
  expect(r.codes).toContain("DAILY");
});

test("BL-05 a deposit above the bill leaves a refund due, paid back in cash", async ({ api }) => {
  const w = await api.as("r9");
  const r = await checkedOut(api, w, "refund", { rentalType: "HOURLY", deposit: 500_000 });
  expect(r.invoice.quote.refundDue).toBeGreaterThan(0);
  expect(
    (await api.post(w, `/v1/invoices/${r.invoice.id}/payments`, { method: "CASH" })).status,
  ).toBe(201);
});
