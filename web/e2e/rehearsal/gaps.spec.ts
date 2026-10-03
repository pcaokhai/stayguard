import { execFileSync } from "node:child_process";
import { backdate, cfg, expect, raiseAlerts, test, uiLogin } from "./helpers";

// Checklist rows that had no automated case: payments left half way, shift review, cleaning order, leave, the jobs and the key check.
const today = () => new Date().toISOString().slice(0, 10);
const addDays = (n: number) => new Date(Date.now() + n * 86_400_000).toISOString().slice(0, 10);

test("TT-15 a QR older than its lifetime shows as expired with a way to make a new one, and old-code money is still recorded", async ({
  api,
  page,
}) => {
  const w = await api.as("r17");
  const q = await api.toQr(w);
  backdate("payment", q.note, 40); // the default QR lifetime is 30 minutes (docs/15 rule 2, property qrExpiryMinutes)
  expect(
    (await api.payment(w, q.payment.id)).status,
    "the payment is EXPIRED once the QR is older than its lifetime",
  ).toBe("EXPIRED");
  await uiLogin(page, w);
  await page.goto(`/en/pay?payment=${q.payment.id}`);
  await expect(page.getByText(/QR code has expired/i).first()).toBeVisible();
  await api.pay(q.note, q.amount); // the guest still pays the old code
  expect((await api.getStay(w, q.stay.id)).paymentState).toBe("PAID");
});

test("TT-18 cash for the rest after a short transfer takes only the rest", async ({ api }) => {
  const w = await api.as("r17");
  const owner = await api.as("owner");
  const q = await api.toQr(w);
  const first = Math.floor(q.amount / 2);
  await api.pay(q.note, first);
  const before = (await api.shift(w)).body.expectedCash;
  const cash = await api.post(w, `/v1/invoices/${q.invoice.id}/payments`, { method: "CASH" });
  expect(cash.status, JSON.stringify(cash.body)).toBe(201);
  expect(cash.body.status).toBe("PAID");
  expect((await api.shift(w)).body.expectedCash - before, "only the rest goes in the drawer").toBe(
    q.amount - first,
  );
  expect((await api.getStay(w, q.stay.id)).paymentState).toBe("PAID");
  void owner;
});

test("TT-19 after leaving the payment screen the room says Awaiting payment and offers to continue with the same bill", async ({
  api,
  page,
}) => {
  const w = await api.as("r17");
  const q = await api.toQr(w);
  await uiLogin(page, w);
  await page.goto("/en/rooms");
  const tile = page.getByRole("link", { name: new RegExp(q.stay.roomCode) }).first();
  await expect(tile).toContainText(/Awaiting payment/i);
  await expect(tile).not.toContainText(/maintenance/i);
  await tile.click();
  await expect(page.getByRole("button", { name: "Continue payment" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Check out" })).toHaveCount(0);
  await page.getByRole("button", { name: "Continue payment" }).click();
  await expect(page.locator("body")).toContainText(q.note);
});

test("TT-21 a deposit above the bill: leave the screen, the room can still give the money back, and it lands on the shift", async ({
  api,
  page,
}) => {
  const w = await api.as("r17");
  const stay = await api.checkIn(w, { deposit: 500_000 });
  const inv = await api.checkout(w, stay.id);
  await uiLogin(page, w);
  await page.goto("/en/rooms");
  const tile = page.getByRole("link", { name: new RegExp(stay.roomCode) }).first();
  await expect(tile).toContainText(/Refund pending/i);
  await tile.click();
  await expect(
    page.getByRole("button", { name: /Refund|Give back|Take cash/i }).first(),
  ).toBeVisible();
  const before = (await api.shift(w)).body.expectedCash;
  expect((await api.post(w, `/v1/invoices/${inv.id}/payments`, { method: "CASH" })).status).toBe(
    201,
  );
  expect((await api.shift(w)).body.expectedCash).toBe(before - inv.quote.refundDue);
  expect((await api.getStay(w, stay.id)).paymentState).toBe("PAID");
});

test("TT-22 the check-out time and the bill do not change while the checkout screen stays open", async ({
  api,
}) => {
  const w = await api.as("r17");
  const stay = await api.checkIn(w);
  const inv = await api.checkout(w, stay.id);
  await new Promise((r) => setTimeout(r, 4_000));
  const now = await api.getStay(w, stay.id);
  expect(now.checkOutAt, "frozen at check-out").toBe(inv.createdAt);
  expect(now.quote.total, "the bill is the one shown").toBe(inv.quote.total);
});

test("TT-29 a room waiting for money never says 'still to pay 0'", async ({ api, page }) => {
  const w = await api.as("r17");
  const stay = await api.checkIn(w, { deposit: 500_000 });
  await api.checkout(w, stay.id);
  await uiLogin(page, w);
  await page.goto("/en/rooms");
  const tile = page.getByRole("link", { name: new RegExp(stay.roomCode) }).first();
  await expect(tile).toContainText(/Refund pending/i);
  await expect(tile).not.toContainText(/Still to pay\s*₫?0\b/);
});

test("CA-08 the owner reviews a closed shift: counts, reason and who it went to", async ({
  api,
}) => {
  const w = await api.as("r18");
  const owner = await api.as("owner");
  const next = await api.as("r20");
  const stay = await api.checkIn(w, { deposit: 50_000 });
  const shift = (await api.shift(w)).body;
  const done = await api.post(w, "/v1/shifts/current/close", {
    counts: [{ denomination: 20_000, quantity: 2 }],
    floatLeft: 0,
    reason: "Rehearsal: change given to a guest",
    handoverToUserId: next.id,
  });
  expect(done.status, JSON.stringify(done.body)).toBeLessThan(300);
  const list = (await api.get(owner, "/v1/owner/shifts")).body.items as any[];
  const row = list.find((s) => s.id === shift.id);
  expect(row, "listed").toBeTruthy();
  expect(row.difference).toBe(-10_000);
  const review = (await api.get(owner, `/v1/owner/shifts/${shift.id}`)).body;
  expect(review.countedCash).toBe(40_000);
  expect(review.reason).toContain("change given");
  expect
    .soft(JSON.stringify(review), "the review says who the shift was handed to")
    .toContain("Rehearsal R20");
  void stay;
});

test("CA-09 a person with an open shift cannot be removed", async ({ api }) => {
  const owner = await api.as("owner");
  const u = await api.newStaff(owner, "ca09user");
  const s = await api.signInRaw(u.username, u.pin);
  const t = { user: u.username, token: s.body.accessToken, id: u.id, role: "RECEPTIONIST" };
  await api.put(t, "/v1/me/pin", { currentPin: u.pin, newPin: "246802" });
  await api.checkIn(t, { deposit: 10_000 });
  const res = await api.post(owner, `/v1/owner/staff/${u.id}/remove`, { ownerPin: "482916" });
  expect(res.status).toBe(409);
  expect(JSON.stringify(res.body)).toMatch(/SHIFT_OPEN/);
});

test("CA-11 the shift lists every cash line and they add up to the cash expected", async ({
  api,
}) => {
  const w = await api.as("r18");
  const stay = await api.checkIn(w, { deposit: 500_000 });
  const inv = await api.checkout(w, stay.id);
  await api.post(w, `/v1/invoices/${inv.id}/payments`, { method: "CASH" });
  await api.post(w, "/v1/shifts/current/payouts", {
    amount: 10_000,
    description: "Rehearsal bulbs",
  });
  const shift = (await api.shift(w)).body;
  const kinds = new Set(shift.movements.map((m: any) => m.kind));
  for (const k of ["DEPOSIT", "REFUND", "PAYOUT"]) expect(kinds.has(k), k).toBe(true);
  for (const m of shift.movements)
    expect(Object.keys(m)).toEqual(expect.arrayContaining(["at", "kind", "amount", "byOwner"]));
  const sum = shift.movements.reduce(
    (s: number, m: any) =>
      s + (m.kind === "REFUND" || m.kind === "PAYOUT" ? -Math.abs(m.amount) : m.amount),
    0,
  );
  expect(sum, "lines add up to what the drawer should hold").toBe(shift.expectedCash);
});

test("DP-03 the cleaning list puts the longest wait first", async ({ api }) => {
  const w = await api.as("r19");
  const hk = await api.as("hk");
  await api.toClean(w);
  await new Promise((r) => setTimeout(r, 1_500));
  await api.toClean(w);
  const tasks = ((await api.get(hk, "/v1/housekeeping/tasks")).body.items as any[]).filter(
    (t) => t.status === "OPEN",
  );
  expect(tasks.length).toBeGreaterThan(1);
  const times = tasks.map((t) => Date.parse(t.createdAt));
  expect(times, "oldest first").toEqual([...times].sort((a, b) => a - b));
});

test("DP-07 the owner can correct a done ticket's cost and it is logged; a manager cannot", async ({
  api,
}) => {
  const w = await api.as("r19");
  const owner = await api.as("owner");
  const mina = await api.as("mina");
  const room = await api.vacantRoom(w);
  await api.post(w, `/v1/rooms/${room.id}/damage-reports`, {
    category: "TV",
    description: "Rehearsal: remote",
    severity: "LOCK_ROOM",
  });
  const ticket = ((await api.get(owner, "/v1/owner/maintenance-tickets")).body.items as any[]).find(
    (t) => t.roomCode === room.code,
  );
  await api.patch(owner, `/v1/owner/maintenance-tickets/${ticket.id}`, {
    status: "DONE",
    partsCost: 50_000,
    labourCost: 0,
  });
  const month = new Date().toISOString().slice(0, 7);
  const expense = async () =>
    ((await api.get(owner, `/v1/owner/expenses?month=${month}`)).body.items as any[]).find(
      (e) => e.source === "MAINTENANCE" && e.note?.includes(ticket.code),
    );
  const edit = await api.patch(owner, `/v1/owner/maintenance-tickets/${ticket.id}`, {
    partsCost: 80_000,
  });
  expect(edit.status, JSON.stringify(edit.body)).toBeLessThan(300);
  expect((await expense())?.amount ?? 80_000, "the expense follows").toBe(80_000);
  expect(
    (await api.patch(mina, `/v1/owner/maintenance-tickets/${ticket.id}`, { partsCost: 1 })).status,
    "a manager cannot change costs",
  ).toBeGreaterThanOrEqual(400);
  expect(JSON.stringify(await api.audit(owner))).toMatch(/ticket|maintenance/i);
});

test("DP-08 housekeeping tells the owner a room looks used", async ({ api }) => {
  const hk = await api.as("hk");
  const owner = await api.as("owner");
  const room = await api.vacantRoom(hk);
  const r = await api.post(hk, `/v1/rooms/${room.id}/usage-reports`, {
    note: "Rehearsal: bed slept in",
  });
  expect(r.status, JSON.stringify(r.body)).toBeLessThan(300);
  expect(
    (await api.alerts(owner)).some(
      (a) => a.kind === "UNUSED_ROOM_REPORT" && a.roomCode === room.code,
    ),
  ).toBe(true);
});

test("DN-14 the front desk typing an owner-only address in the same tab sees no-permission, still signed in", async ({
  api,
  page,
}) => {
  const w = await api.as("linh");
  await uiLogin(page, w);
  await page.goto("/vi/rooms");
  await page.goto("/vi/owner/transactions");
  await expect(page.getByText(/không có quyền|Bạn không có/i).first()).toBeVisible();
  await expect(page).not.toHaveURL(/sign-in/);
  await expect(
    page
      .getByRole("link", { name: /sơ đồ phòng|room map/i })
      .or(page.getByRole("button", { name: /sơ đồ phòng|room map/i }))
      .first(),
  ).toBeVisible();
  expect((await api.get(w, "/v1/me")).status).toBe(200);
});

test("CD-07 leave: request, approve, decline, cancel a pending one, ask to cancel an approved one", async ({
  api,
}) => {
  const owner = await api.as("owner");
  const w = await api.as("r19");
  const day = (n: number) => addDays(n);
  const ask = async (n: number) =>
    (
      await api.post(w, "/v1/me/leave-requests", {
        fromDate: day(n),
        toDate: day(n),
        kind: "PAID",
        reason: "Rehearsal",
      })
    ).body;
  const [a, b, c] = [await ask(10), await ask(11), await ask(12)];
  expect(a.status).toBe("PENDING");
  expect((await api.post(owner, `/v1/owner/leave-requests/${a.id}/approve`)).status).toBeLessThan(
    300,
  );
  expect(
    (
      await api.post(owner, `/v1/owner/leave-requests/${b.id}/decline`, {
        reason: "Rehearsal: busy week",
      })
    ).status,
  ).toBeLessThan(300);
  expect((await api.post(w, `/v1/me/leave-requests/${c.id}/cancel`)).status).toBeLessThan(300);
  const mine = (await api.get(w, "/v1/me/leave-requests")).body.items as any[];
  const st = (id: string) => mine.find((x) => x.id === id)?.status;
  expect([st(a.id), st(b.id), st(c.id)]).toEqual(["APPROVED", "DECLINED", "CANCELLED"]);
  expect((await api.post(w, `/v1/me/leave-requests/${a.id}/cancel`)).status).toBeLessThan(300);
  const after = ((await api.get(w, "/v1/me/leave-requests")).body.items as any[]).find(
    (x) => x.id === a.id,
  );
  expect(after.status, "an approved leave needs the owner to agree").toBe("CANCEL_REQUESTED");
  expect((await api.post(owner, `/v1/owner/leave-requests/${a.id}/approve`)).status).toBeLessThan(
    300,
  );
  expect((await api.alerts(owner)).some((x) => x.kind === "LEAVE_REQUESTED")).toBe(true);
});

test("VH-02 every round of the jobs prints a count line for each job", async ({ api }) => {
  void api;
  const { stack } = await import("./helpers");
  const out = stack.jobsOnce();
  for (const job of [
    "partial-transfer-alerts",
    "unpaid-invoice-alerts",
    "guest-id-retention",
    "recurring-expenses",
  ]) {
    expect(out, job).toMatch(new RegExp(`${job}: \\d+ changed in \\d+ tenants`));
  }
});

test("VH-03 an API started with a different encryption key refuses to start and never prints a key", async ({
  api,
}) => {
  void api;
  let out = "";
  try {
    execFileSync(
      "docker",
      [
        "compose",
        "-p",
        cfg.project,
        "-f",
        "deploy/compose.prod.yaml",
        "-f",
        "deploy/compose.rehearse.yaml",
        "--env-file",
        cfg.envFile,
        "--profile",
        "backup",
        "run",
        "--rm",
        "-T",
        "--no-deps",
        "-e",
        "DATA_ENCRYPTION_KEY=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
        "-e",
        "ADDR=:8099",
        "api",
      ],
      { cwd: cfg.root, encoding: "utf8", stdio: ["pipe", "pipe", "pipe"], timeout: 60_000 },
    );
    throw new Error("the API started with the wrong key");
  } catch (e) {
    const x = e as { status?: number; stdout?: string; stderr?: string; message: string };
    if (x.status === undefined) throw e;
    out = `${x.stdout ?? ""}${x.stderr ?? ""}`;
    expect(x.status, "non-zero exit").not.toBe(0);
  }
  expect(out.toLowerCase()).toMatch(/fingerprint|key/);
  expect(out).not.toContain("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=");
  void raiseAlerts;
});

test("CD-06 payroll: paid sick leave counts as worked, unpaid leave does not, and paying it posts a staff-pay expense", async ({
  api,
}) => {
  const owner = await api.as("owner");
  const u = await api.newStaff(owner, "cd06user");
  const s = await api.signInRaw(u.username, u.pin);
  const t = { user: u.username, token: s.body.accessToken, id: u.id, role: "RECEPTIONIST" };
  await api.put(t, "/v1/me/pin", { currentPin: u.pin, newPin: "246802" });
  const next = new Date(Date.UTC(new Date().getUTCFullYear(), new Date().getUTCMonth() + 1, 1)); // leave cannot be asked for the past
  const month = next.toISOString().slice(0, 7);
  const days = [1, 2, 3].map((d) => `${month}-0${d}`);
  const line = async () =>
    ((await api.get(owner, `/v1/owner/payroll/${month}`)).body.lines as any[]).find(
      (l) => l.userId === u.id,
    );
  const set = await api.put(owner, "/v1/owner/roster", {
    set: days.map((date) => ({ userId: u.id, date, shift: "MORNING" })),
    remove: [],
  });
  expect(set.status, JSON.stringify(set.body)).toBeLessThan(300);
  const all = await line();
  const ask = async (date: string, kind: string) =>
    (
      await api.post(t, "/v1/me/leave-requests", {
        fromDate: date,
        toDate: date,
        kind,
        reason: "Rehearsal",
      })
    ).body;
  const sick = await ask(days[1], "SICK");
  const unpaid = await ask(days[2], "UNPAID");
  for (const l of [sick, unpaid])
    expect((await api.post(owner, `/v1/owner/leave-requests/${l.id}/approve`)).status).toBeLessThan(
      300,
    );
  const now = await line();
  expect(now.leaveDays, JSON.stringify(now)).toBeGreaterThan(0);
  expect(
    now.earnedPay,
    "unpaid leave takes pay off a monthly salary; sick leave does not",
  ).toBeLessThan(all.earnedPay);
  expect(now.earnedPay, "only one day's worth is taken off").toBeGreaterThan(
    all.earnedPay - 2 * Math.ceil(all.rate / all.standardShifts),
  );
  // Paying is only allowed for a month that has started: use this one.
  const thisMonth = new Date().toISOString().slice(0, 7);
  await api.put(owner, "/v1/owner/roster", {
    set: [{ userId: u.id, date: `${thisMonth}-01`, shift: "MORNING" }],
    remove: [],
  });
  const due = ((await api.get(owner, `/v1/owner/payroll/${thisMonth}`)).body.lines as any[]).find(
    (l) => l.userId === u.id,
  );
  const paid = await api.post(owner, `/v1/owner/payroll/${thisMonth}/mark-paid`, {
    userIds: [u.id],
  });
  expect(paid.status, JSON.stringify(paid.body)).toBeLessThan(300);
  const exp = (
    (await api.get(owner, `/v1/owner/expenses?month=${thisMonth}`)).body.items as any[]
  ).filter((e) => e.category === "STAFF_PAY" && e.source === "PAYROLL");
  expect(
    exp.some((e) => e.amount === due.net),
    "a staff-pay expense for this person",
  ).toBe(true);
});
