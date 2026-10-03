import type { Page } from "@playwright/test";
import { expect, test, uiLogin } from "./helpers";

// Portfolio-demo checks, screen level: tablet, desktop, reduced motion, English, offline (GD) and the owner's alert and room screens (TD).
const noPageScroll = (page: Page) =>
  page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1);

test("GD-03 on a tablet (834 px) the icon rail is on the left and the page does not scroll sideways", async ({
  api,
  browser,
}, testInfo) => {
  const w = await api.as("linh");
  const ctx = await browser.newContext({ viewport: { width: 834, height: 1194 } });
  const page = await ctx.newPage();
  await uiLogin(page, w);
  for (const route of ["/en/rooms", "/en/stays", "/en/shift", "/en/account"]) {
    await page.goto(route);
    await page.waitForTimeout(1_000);
    expect(await noPageScroll(page), `${route} scrolls sideways`).toBe(true);
  }
  await page.goto("/en/rooms");
  await page.waitForTimeout(1_000);
  await testInfo.attach("tablet-rooms", {
    body: await page.screenshot(),
    contentType: "image/png",
  });
  const rail = await page.locator("nav, aside").evaluateAll((els) =>
    els
      .map((e) => e.getBoundingClientRect())
      .filter((r) => r.height > 300 && r.width < 200)
      .map((r) => ({ x: r.x, w: r.width })),
  );
  expect(rail.length, "a tall narrow rail").toBeGreaterThan(0);
  expect(rail[0].x, "on the left").toBeLessThan(10);
  await ctx.close();
});

test("GD-04 on a desktop (1280 px) the sidebar has groups and wide tables scroll inside their card", async ({
  api,
  browser,
}) => {
  const owner = await api.as("owner");
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await ctx.newPage();
  await uiLogin(page, owner);
  await page.goto("/en/owner/payroll");
  await page.waitForTimeout(1_500);
  for (const group of ["Monitor", "Finance", "Operations", "People", "Settings"])
    await expect(page.getByText(group, { exact: true }).first()).toBeVisible();
  expect(await noPageScroll(page), "the page itself does not scroll sideways").toBe(true);
  for (const route of ["/en/owner/transactions", "/en/owner/stays", "/en/owner/roster"]) {
    await page.goto(route);
    await page.waitForTimeout(1_000);
    expect(await noPageScroll(page), route).toBe(true);
  }
  await ctx.close();
});

test("GD-05 with reduced motion the pay and paid screens still show everything", async ({
  api,
  browser,
}) => {
  const w = await api.as("r15");
  const q = await api.toQr(w);
  const ctx = await browser.newContext({
    viewport: { width: 390, height: 844 },
    reducedMotion: "reduce",
  });
  const page = await ctx.newPage();
  await uiLogin(page, w);
  await page.goto(`/en/pay?payment=${q.payment.id}`);
  await expect(page.getByText("Transfer note")).toBeVisible();
  await expect(page.locator("body")).toContainText(q.note);
  expect(await page.evaluate(() => matchMedia("(prefers-reduced-motion: reduce)").matches)).toBe(
    true,
  );
  await api.pay(q.note, q.amount);
  await expect(page).toHaveURL(/\/en\/paid\?payment=/, { timeout: 20_000 });
  await expect(page.getByText("Paid", { exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: "Back to room map" })).toBeVisible();
  await ctx.close();
});

test("GD-06 English: switching the language leaves no Vietnamese or translation keys on the main screens", async ({
  api,
  browser,
}) => {
  const owner = await api.as("owner");
  expect((await api.put(owner, "/v1/me/locale", { locale: "en" })).status).toBeLessThan(300);
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await ctx.newPage();
  await uiLogin(page, owner);
  for (const route of [
    "/en/owner",
    "/en/owner/rooms",
    "/en/owner/alerts",
    "/en/owner/transactions",
    "/en/owner/items",
    "/en/owner/staff",
    "/en/account",
  ]) {
    await page.goto(route);
    await page.waitForTimeout(1_200);
    const text = (await page.evaluate(() => document.body.innerText)).replace(/Tiếng Việt/g, ""); // the language's own name
    expect(text, `${route}: Vietnamese text`).not.toMatch(
      /[ăâđêôơưạảấầẩẫậắằẳẵặẹẻẽếềểễệỉịọỏốồổỗộớờởỡợụủứừửữựỳỵỷỹ]/i,
    );
    expect(text, `${route}: a translation key`).not.toMatch(
      /\b(rooms|owner|alerts|stay|shift)\.[a-z][A-Za-z]+\b/,
    );
  }
  await ctx.close();
});

test("GD-07 offline on the payment screen shows it, and coming back creates no second payment", async ({
  api,
  browser,
}) => {
  const w = await api.as("r15");
  const q = await api.toQr(w);
  const ctx = await browser.newContext({ viewport: { width: 390, height: 844 } });
  const page = await ctx.newPage();
  await uiLogin(page, w);
  await page.goto(`/en/pay?payment=${q.payment.id}`);
  await expect(page.getByText("Transfer note")).toBeVisible();
  await ctx.setOffline(true);
  await page.waitForTimeout(7_000); // the screen polls every 3 s: two misses
  await expect
    .soft(
      page.getByText(/no internet|offline|connection/i).first(),
      "the screen says there is no connection",
    )
    .toBeVisible();
  await ctx.setOffline(false);
  await page.waitForTimeout(4_000);
  await expect(page.locator("body")).toContainText(q.note);
  expect((await api.getStay(w, q.stay.id)).pendingPayment.paymentId, "still the one payment").toBe(
    q.payment.id,
  );
  await api.pay(q.note, q.amount);
  await expect(page).toHaveURL(/\/en\/paid\?payment=/, { timeout: 20_000 });
  await ctx.close();
});

test("TD-05 the owner's room map has building chips and no shift actions", async ({
  api,
  browser,
}) => {
  const owner = await api.as("owner");
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await ctx.newPage();
  await uiLogin(page, owner);
  await page.goto("/en/owner/rooms");
  await expect(page.getByText(/Building A/).first()).toBeVisible();
  const actions = await page.getByRole("button").or(page.getByRole("link")).allInnerTexts();
  expect(actions.join("|"), "no shift action to press").not.toMatch(/End shift|Payout|Close shift/);
  await ctx.close();
});

test("TD-07 the View button on a stay alert opens the owner's stay timeline", async ({
  api,
  browser,
}) => {
  const w = await api.as("r15");
  const owner = await api.as("owner");
  const stay = await api.checkIn(w);
  await api.post(w, `/v1/stays/${stay.id}/check-in-time`, {
    newCheckInAt: new Date(Date.parse(stay.checkInAt) - 600_000).toISOString(),
    reasonCode: "WRONG_TIME",
    note: "TD-07 rehearsal",
  });
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await ctx.newPage();
  await uiLogin(page, owner);
  await page.goto("/en/owner/alerts");
  const row = page.locator("li, tr").filter({ hasText: stay.roomCode }).first();
  await row
    .getByRole("link", { name: /Open|View/ })
    .or(row.getByRole("button", { name: /Open|View/ }))
    .first()
    .click();
  await expect(page).toHaveURL(/\/en\/owner\/stay\?id=/);
  await ctx.close();
});

test("TD-08 the SePay update alert reads as a sentence, not as a raw timestamp", async ({
  api,
  browser,
}) => {
  const owner = await api.as("owner");
  expect((await api.alerts(owner)).some((a) => a.kind === "SEPAY_UPDATED")).toBe(true);
  for (const locale of ["en", "vi"]) {
    const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
    const page = await ctx.newPage();
    await uiLogin(page, owner);
    await page.goto(`/${locale}/owner/alerts`);
    await page.waitForTimeout(1_500);
    const text = await page.evaluate(() => document.body.innerText);
    expect(text, locale).not.toMatch(/\d{4}-\d{2}-\d{2}T\d{2}:\d{2}/);
    expect(text, locale).not.toMatch(/SEPAY_UPDATED|SECRET_SET/);
    await ctx.close();
  }
});
