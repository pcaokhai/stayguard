import { createHmac } from "node:crypto";
import { expect, type Page, test } from "@playwright/test";

// Partial transfer path: the bank reports 10,000 less than the bill, the screen shows the API's
// remaining amount, the QR is for that remainder, and a second signed webhook on the same bill code settles it.
// Run through `make smoke` (scripts/smoke.sh sets the variables); skipped otherwise.
const configured = Boolean(process.env.SMOKE_HOOK_PATH);
const env = (n: string) => process.env[n] ?? "";
const NEW_PIN = "482916";
const SHORT = 10_000;
test.use({ viewport: { width: 390, height: 844 } });

const vnd = (s: string) => Number(s.replace(/[^\d]/g, ""));

async function typePin(page: Page, nth: number, pin: string) {
  await page.locator("input").nth(nth).click();
  await page.keyboard.type(pin);
}

// Works whichever spec signed the receptionist in first; the chosen PIN goes first so wrong guesses do not add up to a lockout.
async function signIn(page: Page) {
  const attempt = async (pin: string) => {
    await page.goto("/en/sign-in");
    await page.locator('input[name="guesthouseCode"]').fill(env("SMOKE_GUESTHOUSE"));
    await page.locator('input[name="username"]').fill("linh");
    await typePin(page, 2, pin);
    await page.getByRole("button", { name: "Sign in" }).click();
  };
  await attempt(NEW_PIN);
  const next = await Promise.race([
    page.waitForURL(/\/en\/rooms/).then(() => "home"),
    page
      .getByText("is not right")
      .waitFor()
      .then(() => "first"),
  ]);
  if (next === "first") {
    await attempt(env("SMOKE_RECEPTIONIST_PIN"));
    await page.waitForURL(/set-pin/);
    await typePin(page, 0, NEW_PIN);
    await typePin(page, 1, NEW_PIN);
    await page.getByRole("button", { name: "Save PIN and continue" }).click();
    await page.waitForURL(/\/en\/rooms/);
  }
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
    description: "partial transfer test",
    transferAmount: amount,
    accumulated: 0,
    referenceCode: "FT-PARTIAL",
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

test("a transfer short by 10,000 shows the remainder, a second transfer on the same bill code settles it", async ({
  page,
  request,
}) => {
  test.skip(!configured, "needs the stack and test guesthouse that `make smoke` creates");
  test.setTimeout(120_000);
  await signIn(page);

  // Check in A102 and go to the QR screen (the deposit is below the bill, so a transfer is due).
  await page.getByRole("link", { name: /A102/ }).first().click();
  await page.locator('input[name="guestName"]').fill("Partial Guest");
  await page.locator('input[name="guestPhone"]').fill("0912345671");
  await page.locator('input[name="deposit"]').fill("10000");
  await page.getByRole("button", { name: "Confirm check-in" }).click();
  await expect(page).toHaveURL(/\/en\/stay\?id=/);
  await page.getByRole("link", { name: "Check out" }).click();
  await page.getByRole("button", { name: /Bank transfer/ }).click();
  await expect(page.getByText("Transfer note")).toBeVisible();
  const text = await page.locator("body").innerText();
  const note = text.match(/Transfer note\s+(\S+)/)?.[1] ?? "";
  const due = vnd(text.match(/₫[\d,]+/)?.[0] ?? "");
  expect(due).toBeGreaterThan(SHORT);

  // The bank reports less than the bill: the same payment stays PENDING and the screen shows what the bank
  // sent and what is still to pay (both from the API), with a QR for the remainder under the same bill code.
  await deliver(request, note, due - SHORT);
  await expect(
    page.getByText("Received so far").locator("xpath=following-sibling::*[1]"),
  ).toHaveText(`₫${(due - SHORT).toLocaleString("en-US")}`, { timeout: 20_000 });
  await expect(page.getByText("Still to pay").locator("xpath=following-sibling::*[1]")).toHaveText(
    "₫10,000",
  );
  const next = await page.locator("body").innerText();
  expect(vnd(next.match(/₫[\d,]+/)?.[0] ?? ""), "the QR is for the remainder").toBe(SHORT);
  expect(next.match(/Transfer note\s+(\S+)/)?.[1]).toBe(note);

  // The second signed webhook completes the bill and the screen turns Paid by itself.
  await deliver(request, note, SHORT);
  await expect(page).toHaveURL(/\/en\/paid\?payment=/, { timeout: 20_000 });
  await expect(page.getByText(/Room A102 is now marked To clean/)).toBeVisible();
});
