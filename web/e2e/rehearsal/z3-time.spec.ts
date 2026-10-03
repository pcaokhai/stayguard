import { execFileSync } from "node:child_process";
import { backdate, cfg, expect, raiseAlerts, stack, test } from "./helpers";

// Time-based rules without waiting: the helper backdates one invoice in the rehearsal database and `stayguard jobs run` is run once.
// Rule: PAYMENT_PARTIAL after 15 minutes, PAYMENT_UNPAID or REFUND_PENDING 30 minutes after check-out, once each (docs/15).
test("TT-09 a short transfer raises one partial alert after 15 minutes, and only one", async ({
  api,
}) => {
  const r = await api.as("r1");
  const owner = await api.as("owner");
  const q = await api.toQr(r);
  await api.pay(q.note, Math.floor(q.amount / 2));
  await raiseAlerts("partial", q.note, 10);
  expect(
    await api.alertsFor(owner, "PAYMENT_PARTIAL", q.note),
    "10 minutes is too early",
  ).toHaveLength(0);
  await raiseAlerts("partial", q.note, 10); // now 20 minutes
  const first = await api.alertsFor(owner, "PAYMENT_PARTIAL", q.note);
  expect(first, "exactly one alert after 15 minutes").toHaveLength(1);
  expect(first[0].amount, "the amount still to pay").toBe(q.amount - Math.floor(q.amount / 2));
  await stack.jobsOnce();
  expect(
    await api.alertsFor(owner, "PAYMENT_PARTIAL", q.note),
    "a second run adds none",
  ).toHaveLength(1);
});

test("TT-20 a bill nobody paid raises one unpaid alert 30 minutes after check-out, and only one", async ({
  api,
}) => {
  const r = await api.as("r1");
  const owner = await api.as("owner");
  const q = await api.toQr(r);
  await raiseAlerts("unpaid", q.note, 20);
  expect(
    await api.alertsFor(owner, "PAYMENT_UNPAID", q.note),
    "20 minutes is too early",
  ).toHaveLength(0);
  await raiseAlerts("unpaid", q.note, 15); // now 35 minutes
  const first = await api.alertsFor(owner, "PAYMENT_UNPAID", q.note);
  expect(first, "exactly one alert after 30 minutes").toHaveLength(1);
  expect(first[0].amount).toBe(q.amount);
  await stack.jobsOnce();
  expect(
    await api.alertsFor(owner, "PAYMENT_UNPAID", q.note),
    "a second run adds none",
  ).toHaveLength(1);
});

test("TT-27 a deposit nobody gave back raises one refund alert 30 minutes after check-out, and no unpaid alert", async ({
  api,
}) => {
  const r = await api.as("r1");
  const owner = await api.as("owner");
  const stay = await api.checkIn(r, { deposit: 500_000 });
  const invoice = await api.checkout(r, stay.id);
  const bill = invoice.billCode as string;
  await raiseAlerts("unpaid", bill, 20);
  expect(
    await api.alertsFor(owner, "REFUND_PENDING", bill),
    "20 minutes is too early",
  ).toHaveLength(0);
  await raiseAlerts("unpaid", bill, 15);
  const first = await api.alertsFor(owner, "REFUND_PENDING", bill);
  expect(first, "exactly one refund alert").toHaveLength(1);
  expect(first[0].amount).toBe(invoice.quote.refundDue);
  expect(
    await api.alertsFor(owner, "PAYMENT_UNPAID", bill),
    "a refund is never an unpaid bill",
  ).toHaveLength(0);
  await stack.jobsOnce();
  expect(await api.alertsFor(owner, "REFUND_PENDING", bill), "a second run adds none").toHaveLength(
    1,
  );
});

test("BM-04 a webhook that arrives 6 minutes late is refused, a fresh delivery of the same transfer is accepted", async ({
  api,
}) => {
  const r = await api.as("r1");
  const q = await api.toQr(r);
  stack.stopApi(); // the bank cannot reach us
  const down = await api.deliver({ note: q.note, amount: q.amount }).then(
    (x) => x.status,
    () => "refused",
  );
  expect(down, "nothing answers while the API is down").not.toBe(200);
  stack.startApi();
  await stack.ready();
  // SEPAY_TIMESTAMP_TOLERANCE is 300 s by default: the signed timestamp is 360 s old when it finally arrives.
  const late = await api.deliver({ note: q.note, amount: q.amount, ageSeconds: 360 });
  expect(late.status, "outside the tolerance").toBe(401);
  expect((await api.payment(r, q.payment.id)).status, "nothing was paid by it").toBe("PENDING");
  expect(stack.logs("api")).toContain("timestamp outside tolerance");
  const near = await api.deliver({ note: q.note, amount: q.amount, ageSeconds: 240, id: late.id });
  expect(near.status, "inside the tolerance, the same transaction id").toBe(200);
  await api.untilPayment(r, q.payment.id, "PAID");
});

test("BM-06 SePay's own retry schedule after a missed delivery", async () => {
  // Needs SePay itself: how often and how long it retries a webhook we answered late or not at all cannot be rehearsed here.
  test.skip(
    true,
    "manual: SePay real retry behaviour (retry schedule and whether it re-signs each attempt) needs a SePay test transfer",
  );
  void backdate;
});

test("VH-08 the backdate helper refuses to run anywhere but the rehearsal stack", async () => {
  const run = (env: Record<string, string>) => {
    try {
      execFileSync("scripts/rehearsal-backdate.sh", ["unpaid", "PH1003A101", "5"], {
        cwd: cfg.root,
        encoding: "utf8",
        stdio: ["pipe", "pipe", "pipe"],
        env: { ...process.env, ...env },
      });
      return { status: 0, stderr: "" };
    } catch (e) {
      const x = e as { status: number; stderr: string };
      return { status: x.status, stderr: x.stderr };
    }
  };
  const notRehearsal = run({ RH_TENANT: "smoke" });
  expect(notRehearsal.status).toBe(1);
  expect(notRehearsal.stderr).toContain("REFUSED");
  const otherProject = run({ RH_TENANT: cfg.guesthouse, COMPOSE_PROJECT_NAME: "stayguard" });
  expect(otherProject.status).toBe(1);
  expect(otherProject.stderr).toContain("REFUSED");
});
