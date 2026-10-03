import { createHmac } from "node:crypto";
import { expect, type Page, test } from "@playwright/test";

// A stay that was checked out and sent to payment but left unpaid: the room map says "Awaiting payment", the stay
// has no Check out button, and "Continue payment" reopens the payment for the same bill code. Run through `make smoke`.
const configured = Boolean(process.env.SMOKE_HOOK_PATH);
const env = (n: string) => process.env[n] ?? "";
const NEW_PIN = "482916";
const vnd = (s: string) => Number(s.replace(/[^\d]/g, ""));

async function typePin(page: Page, nth: number, pin: string) {
  await page.locator("input").nth(nth).click();
  await page.keyboard.type(pin);
}
// Works whichever spec signed the user in first: the one-time PIN only works once.
async function signIn(page: Page, user: "linh" | "owner" = "linh") {
  const oneTime = env(user === "linh" ? "SMOKE_RECEPTIONIST_PIN" : "SMOKE_OWNER_PIN");
  const home = user === "linh" ? /\/en\/rooms/ : /\/en\/owner/;
  const attempt = async (pin: string) => {
    await page.goto("/en/sign-in");
    await page.locator('input[name="guesthouseCode"]').fill(env("SMOKE_GUESTHOUSE"));
    await page.locator('input[name="username"]').fill(user);
    await typePin(page, 2, pin);
    await page.getByRole("button", { name: "Sign in" }).click();
  };
  await attempt(oneTime);
  const next = await Promise.race([
    page.waitForURL(/set-pin/).then(() => "set"),
    page.waitForURL(home).then(() => "home"),
    page
      .getByText("is not right")
      .waitFor()
      .then(() => "wrong"),
  ]);
  if (next === "set") {
    await typePin(page, 0, NEW_PIN);
    await typePin(page, 1, NEW_PIN);
    await page.getByRole("button", { name: "Save PIN and continue" }).click();
  } else if (next === "wrong") await attempt(NEW_PIN);
  await page.waitForURL(home);
}
async function deliver(
  request: import("@playwright/test").APIRequestContext,
  note: string,
  amount: number,
) {
  const body = JSON.stringify({
    id: Date.now() + Math.floor(Math.random() * 1000),
    gateway: "Vietcombank",
    transactionDate: "2026-10-02 14:30:00",
    accountNumber: env("SMOKE_ACCOUNT_NO"),
    subAccount: "",
    code: null,
    content: `${note} chuyen tien`,
    transferType: "in",
    description: "awaiting payment test",
    transferAmount: amount,
    accumulated: 0,
    referenceCode: "FT-AWAIT",
  });
  const timestamp = String(Math.floor(Date.now() / 1000));
  const signature =
    "sha256=" +
    createHmac("sha256", env("SMOKE_SEPAY_SECRET")).update(`${timestamp}.${body}`).digest("hex");
  const res = await request.post(env("E2E_BASE_URL") + env("SMOKE_HOOK_PATH"), {
    data: body,
    headers: {
      "content-type": "application/json",
      "x-sepay-signature": signature,
      "x-sepay-timestamp": timestamp,
    },
  });
  expect(res.status(), "the webhook must be accepted").toBe(200);
}
async function checkIn(page: Page, room: RegExp, name: string, phone: string) {
  await page.getByRole("link", { name: room }).first().click();
  await page.locator('input[name="guestName"]').fill(name);
  await page.locator('input[name="guestPhone"]').fill(phone);
  await page.locator('input[name="deposit"]').fill("10000");
  await page.getByRole("button", { name: "Confirm check-in" }).click();
  await expect(page).toHaveURL(/\/en\/stay\?id=/);
}

// The owner's map must say the same: awaiting payment, never the maintenance text, no Check out.
async function ownerSees(browser: import("@playwright/test").Browser, room: RegExp, width: number) {
  const ctx = await browser.newContext({ viewport: { width, height: 900 } });
  const owner = await ctx.newPage();
  await signIn(owner, "owner");
  await owner.goto("/en/owner/rooms");
  const tile = owner.locator("li", { hasText: room }).first();
  await expect(tile).toContainText("Awaiting payment");
  if (width >= 1024) {
    await tile.getByRole("button").click();
    const panel = owner.locator("aside");
    await expect(panel).toContainText("Awaiting payment");
    await expect(panel).toContainText("Still to pay");
    await expect(panel).not.toContainText("maintenance");
    await expect(panel.getByRole("link", { name: "Check out" })).toHaveCount(0);
    await expect(panel.getByRole("button", { name: "Continue payment" })).toBeVisible();
  } else {
    await tile.getByRole("link").click();
    await expect(owner).toHaveURL(/\/en\/stay\?id=/);
    await expect(owner.getByRole("link", { name: "Check out" })).toHaveCount(0);
    await expect(owner.getByRole("button", { name: "Continue payment" })).toBeVisible();
    await expect(owner.locator("body")).not.toContainText("maintenance");
  }
  await ctx.close();
}

// Leaves the room vacant so later specs can use it.
async function clean(page: Page, room: RegExp) {
  await page.goto("/en/rooms");
  await page.getByRole("link", { name: room }).first().click();
  await page.getByRole("button", { name: /Cleaned/ }).click();
  await expect(page).toHaveURL(/\/en\/rooms$/);
}

test.describe.configure({ mode: "serial" });

test("transfer chosen, left unpaid: awaiting payment on the map, resume the same bill, signed webhook, Paid", async ({
  page,
  request,
  browser,
}) => {
  test.skip(!configured, "needs the stack and test guesthouse that `make smoke` creates");
  test.setTimeout(150_000);
  await page.setViewportSize({ width: 390, height: 844 });
  await signIn(page);
  await checkIn(page, /A103/, "Awaiting Guest", "0912345676");
  await page.getByRole("link", { name: "Check out" }).click();
  await page.getByRole("button", { name: /Bank transfer/ }).click();
  await expect(page.getByText("Transfer note")).toBeVisible();
  const note = (await page.locator("body").innerText()).match(/Transfer note\s+(\S+)/)?.[1] ?? "";

  await ownerSees(browser, /A103/, 390); // owner, phone

  // go to another room and come back
  await page.goto("/en/rooms");
  await page.getByRole("link", { name: /A101/ }).first().click();
  await expect(page).toHaveURL(/\/en\/checkin/);
  await page.goBack();
  const tile = page.locator("li", { hasText: /A103/ }).first();
  await expect(tile).toContainText("Awaiting payment");
  await expect(tile).not.toContainText("Occupied");

  // the stay has no Check out, only Continue payment on the same bill code
  await tile.getByRole("link").click();
  await expect(page).toHaveURL(/\/en\/stay\?id=/);
  await expect(page.getByRole("link", { name: "Check out" })).toHaveCount(0);
  await page.getByRole("button", { name: "Continue payment" }).click();
  await expect(page.getByText("Transfer note")).toBeVisible();
  const text = await page.locator("body").innerText();
  expect(text.match(/Transfer note\s+(\S+)/)?.[1], "the same bill code").toBe(note);
  const amount = vnd(text.match(/₫[\d,]+/)?.[0] ?? "");
  expect(amount).toBeGreaterThan(0);

  await deliver(request, note, amount);
  await expect(page).toHaveURL(/\/en\/paid\?payment=/, { timeout: 20_000 });
  await clean(page, /A103/);
});

test("checked out without choosing a method, desktop panel: Continue payment creates the payment and settles by webhook", async ({
  page,
  request,
  browser,
}) => {
  test.skip(!configured, "needs the stack and test guesthouse that `make smoke` creates");
  test.setTimeout(150_000);
  await page.setViewportSize({ width: 390, height: 844 });
  await signIn(page);
  await checkIn(page, /A102/, "No Method Guest", "0912345677");
  await page.getByRole("link", { name: "Check out" }).click();
  await expect(page.getByText("Guest pays by")).toBeVisible(); // leave without choosing

  await ownerSees(browser, /A102/, 1280); // owner, desktop

  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto("/en/rooms");
  const tile = page.locator("li", { hasText: /A102/ }).first();
  await expect(tile).toContainText("Awaiting payment");
  await tile.getByRole("button").click();
  const panel = page.locator("aside");
  await expect(panel).toContainText("Awaiting payment");
  await expect(panel).toContainText("Still to pay");
  await expect(panel.getByRole("link", { name: "Check out" })).toHaveCount(0);
  await panel.getByRole("button", { name: "Continue payment" }).click();
  await expect(page.getByText("Transfer note")).toBeVisible();
  const text = await page.locator("body").innerText();
  const note = text.match(/Transfer note\s+(\S+)/)?.[1] ?? "";
  expect(note).toMatch(/^PH\d{4}A102/);
  await deliver(request, note, vnd(text.match(/₫[\d,]+/)?.[0] ?? ""));
  await expect(page).toHaveURL(/\/en\/paid\?payment=/, { timeout: 20_000 });
  await page.setViewportSize({ width: 390, height: 844 });
  await clean(page, /A102/);
});
