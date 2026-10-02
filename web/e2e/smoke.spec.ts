import { createHmac } from "node:crypto";
import { expect, type Page, test } from "@playwright/test";

// Money-path smoke test (docs/14 G1): a receptionist checks a guest in, adds extras, checks out and takes a bank transfer that
// arrives as a signed SePay webhook; the room is cleaned, the shift is closed, and the owner sees the revenue.
// Run it with `make smoke` (scripts/smoke.sh builds the stack, creates the test guesthouse and sets these variables).
const need = (name: string) => {
  const v = process.env[name];
  if (!v) throw new Error(`${name} is not set: run this test through "make smoke"`);
  return v;
};
// Plain "npm run test:e2e" (mock layer, no stack) skips this test instead of failing.
const configured = Boolean(process.env.SMOKE_HOOK_PATH);
const GUESTHOUSE = process.env.SMOKE_GUESTHOUSE ?? "";
const HOOK_PATH = process.env.SMOKE_HOOK_PATH ?? "";
const SECRET = process.env.SMOKE_SEPAY_SECRET ?? "";
const ACCOUNT_NO = process.env.SMOKE_ACCOUNT_NO ?? "";
const BASE = process.env.E2E_BASE_URL ?? "";
const NEW_PIN = "482916"; // not a run, not a repeated digit
const DEPOSIT = 100_000; // the check-in form's default deposit

// The front desk works on a phone: tiles are links to pages there, and a wide screen would open a side panel instead.
test.use({ viewport: { width: 390, height: 844 } });

async function signIn(page: Page, username: string, oneTimePin: string) {
  await page.goto("/en/sign-in");
  await page.locator('input[name="guesthouseCode"]').fill(GUESTHOUSE);
  await page.locator('input[name="username"]').fill(username);
  await page.locator("input").nth(2).click();
  await page.keyboard.type(oneTimePin);
  await page.getByRole("button", { name: "Sign in" }).click();
  // The PIN from the installer works once: everybody chooses their own at the first sign-in.
  await page.waitForURL(/set-pin/);
  await page.locator("input").nth(0).click();
  await page.keyboard.type(NEW_PIN);
  await page.locator("input").nth(1).click();
  await page.keyboard.type(NEW_PIN);
  await page.getByRole("button", { name: "Save PIN and continue" }).click();
}

const vnd = (text: string) => Number(text.replace(/[^\d]/g, ""));

// What SePay sends: sha256= and the hex HMAC-SHA256 of "{timestamp}.{raw body}", with the test secret set on the stack.
async function deliverTransfer(
  request: import("@playwright/test").APIRequestContext,
  note: string,
  amount: number,
) {
  const body = JSON.stringify({
    id: Date.now(),
    gateway: "Vietcombank",
    transactionDate: "2026-10-02 14:30:00",
    accountNumber: ACCOUNT_NO,
    subAccount: "",
    code: null,
    content: `${note} chuyen tien`,
    transferType: "in",
    description: "smoke test transfer",
    transferAmount: amount,
    accumulated: 0,
    referenceCode: "FT-SMOKE",
  });
  const timestamp = String(Math.floor(Date.now() / 1000));
  const signature =
    "sha256=" + createHmac("sha256", SECRET).update(`${timestamp}.${body}`).digest("hex");
  const res = await request.post(BASE + HOOK_PATH, {
    data: body,
    headers: {
      "content-type": "application/json",
      "x-sepay-signature": signature,
      "x-sepay-timestamp": timestamp,
    },
  });
  expect(res.status(), "the webhook must be accepted").toBe(200);
  expect(await res.json()).toEqual({ success: true });
}

test("receptionist money path from check-in to closed shift, and the owner sees the revenue", async ({
  page,
  request,
  browser,
}) => {
  test.skip(!configured, "needs the stack and test guesthouse that `make smoke` creates");
  test.setTimeout(180_000);

  // 1. Sign in as the receptionist.
  await signIn(page, "linh", need("SMOKE_RECEPTIONIST_PIN"));
  await expect(page).toHaveURL(/\/en\/rooms/);

  // 2. Check a guest into A101 (overnight, with the default deposit).
  await page.getByRole("link", { name: /A101/ }).first().click();
  await page.getByText("Overnight").first().click();
  await page.locator('input[name="guestName"]').fill("Smoke Guest");
  await page.locator('input[name="guestPhone"]').fill("0912345678");
  await page.getByRole("button", { name: "Confirm check-in" }).click();
  await expect(page).toHaveURL(/\/en\/stay\?id=/);
  await expect(page.getByText("Deposit paid")).toBeVisible();

  // 3. Two bottles of water as extras.
  await page.getByRole("button", { name: "+ Add extras" }).click();
  await page.getByRole("button", { name: "Increase" }).click();
  await page.getByRole("button", { name: "Increase" }).click();
  await page.getByRole("button", { name: "Add to room" }).click();
  await expect(page.getByText("Still water × 2")).toBeVisible();

  // 4. Check out and choose a bank transfer; the QR screen names the bill and the amount to pay.
  await page.getByRole("link", { name: "Check out" }).click();
  await page.getByRole("button", { name: /Bank transfer/ }).click();
  await expect(page).toHaveURL(/\/en\/pay\?payment=/);
  await expect(page.getByText("Transfer note")).toBeVisible();
  const text = await page.locator("body").innerText();
  const note = text.match(/Transfer note\s+(\S+)/)?.[1] ?? "";
  const amount = vnd(text.match(/₫[\d,]+/)?.[0] ?? "");
  expect(note, "the transfer note is the bill code").toMatch(/^PH\d{4}A101/);
  expect(amount, "balance after the deposit").toBeGreaterThan(0);
  await expect(page.getByText(/Waiting for payment/)).toBeVisible();

  // 5. The bank reports the transfer: a signed webhook. The screen turns Paid by itself (it polls every 3 seconds).
  await deliverTransfer(request, note, amount);
  await expect(page).toHaveURL(/\/en\/paid\?payment=/, { timeout: 20_000 });
  await expect(page.getByText(/Room A101 is now marked To clean/)).toBeVisible();

  // 6. Clean the room: it goes from To clean back to Vacant.
  await page.getByRole("link", { name: "Back to room map" }).click();
  await page.getByRole("link", { name: /A101/ }).first().click();
  await expect(page.getByText("Clean room")).toBeVisible();
  await page.getByRole("button", { name: /Cleaned/ }).click();
  await expect(page).toHaveURL(/\/en\/rooms$/); // the app leaves the page when the server has taken the change
  await expect(page.getByRole("link", { name: /A101.*Vacant/ })).toBeVisible();

  // 7. Close the shift: the first cash action (the deposit) opened it, so the drawer should hold exactly that deposit.
  await page.goto("/en/shift");
  await expect(page.getByText("Close shift").first()).toBeVisible();
  await expect(page.getByText("Cash expected").locator("xpath=following::*[1]")).toContainText(
    "100,000",
  );
  // Count the drawer: one 100,000 note.
  await page.getByRole("button", { name: "More ₫100,000" }).click();
  await expect(page.getByText(/Short|Over/)).toHaveCount(0);
  await page.getByRole("button", { name: /Close shift and send to owner/ }).click();
  await expect(page).toHaveURL(/\/en\/rooms/);
  await expect(page.getByText("Shift closed")).toBeVisible();

  // 8. The owner signs in on another device and sees the money: the building card shows what the bill was for (deposit plus
  // transfer), and the latest payments list the transfer. The headline figures roll digit by digit, so the card is the stable place to read.
  const ownerPage = await (
    await browser.newContext({ viewport: { width: 390, height: 844 } })
  ).newPage();
  await signIn(ownerPage, "owner", need("SMOKE_OWNER_PIN"));
  await ownerPage.goto("/en/owner");
  const revenue = `₫${(DEPOSIT + amount).toLocaleString("en-US")}`;
  await expect(ownerPage.locator("body")).toContainText(new RegExp(`${revenue}\\s*revenue today`), {
    timeout: 15_000,
  });
  await expect(ownerPage.locator("body")).toContainText(/Latest payments\s*A101 · Transfer/i);
});
